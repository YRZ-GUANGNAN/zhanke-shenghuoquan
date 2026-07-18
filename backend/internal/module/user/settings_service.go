package user

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	ErrSettingsUnavailable       = errors.New("user settings are unavailable")
	ErrPhoneBindingUnavailable   = errors.New("wechat phone binding is not configured")
	ErrInvalidPhone              = errors.New("invalid phone number")
	ErrInvalidBlock              = errors.New("cannot block this user")
	ErrFeedbackUnavailable       = errors.New("feedback media is unavailable")
	ErrInvalidFeedback           = errors.New("invalid feedback")
	ErrAccountClosureUnavailable = errors.New("account closure is unavailable")
)

type WechatPhoneResolver interface {
	ResolvePhone(context.Context, string) (string, error)
}

type FeedbackMediaReservation struct {
	Token    string
	AssetIDs []string
}

type FeedbackMedia interface {
	ReserveFeedback(context.Context, uint, []string) (FeedbackMediaReservation, error)
	CommitFeedback(context.Context, uint, FeedbackMediaReservation, string) error
	CancelFeedback(context.Context, uint, FeedbackMediaReservation) error
}

type AccountClosureBlocker struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type AccountClosureCoordinator interface {
	Close(context.Context, uint) ([]AccountClosureBlocker, error)
}

type AccountClosureBlockedError struct {
	Blockers []AccountClosureBlocker
}

func (e *AccountClosureBlockedError) Error() string { return "account closure is blocked" }

func (s *Service) Settings(ctx context.Context, userID uint) (SettingsView, error) {
	repo, ok := s.repo.(settingsRepository)
	if !ok {
		return SettingsView{}, ErrSettingsUnavailable
	}
	settings, err := repo.GetOrCreateSettings(ctx, userID)
	if err != nil {
		return SettingsView{}, err
	}
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return SettingsView{}, err
	}
	count := int64(0)
	if blocks, ok := s.repo.(blocksRepository); ok {
		count, err = blocks.CountBlocks(ctx, userID)
		if err != nil {
			return SettingsView{}, err
		}
	}
	return settingsView(settings, user.Phone, count), nil
}

func (s *Service) UpdateSettings(ctx context.Context, userID uint, input UserSettings) (SettingsView, error) {
	repo, ok := s.repo.(settingsRepository)
	if !ok {
		return SettingsView{}, ErrSettingsUnavailable
	}
	settings, err := repo.UpdateSettings(ctx, userID, input)
	if err != nil {
		return SettingsView{}, err
	}
	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return SettingsView{}, err
	}
	count := int64(0)
	if blocks, ok := s.repo.(blocksRepository); ok {
		count, err = blocks.CountBlocks(ctx, userID)
	}
	return settingsView(settings, user.Phone, count), err
}

func settingsView(value UserSettings, phone *string, count int64) SettingsView {
	view := SettingsView{
		OrderNotice: value.OrderNotice, SocialNotice: value.SocialNotice,
		QuietHours: value.QuietHours, Personalized: value.Personalized, BlacklistCount: count,
	}
	if phone != nil && *phone != "" {
		view.PhoneBound = true
		view.MaskedPhone = maskPhone(*phone)
	}
	return view
}

func (s *Service) BindPhone(ctx context.Context, userID uint, code string) (SettingsView, error) {
	if s.phoneResolver == nil {
		return SettingsView{}, ErrPhoneBindingUnavailable
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return SettingsView{}, ErrInvalidPhone
	}
	phone, err := s.phoneResolver.ResolvePhone(ctx, code)
	if err != nil {
		return SettingsView{}, fmt.Errorf("resolve wechat phone: %w", err)
	}
	phone = strings.TrimPrefix(strings.TrimSpace(phone), "+86")
	if !regexp.MustCompile(`^1[3-9][0-9]{9}$`).MatchString(phone) {
		return SettingsView{}, ErrInvalidPhone
	}
	repo, ok := s.repo.(settingsRepository)
	if !ok {
		return SettingsView{}, ErrSettingsUnavailable
	}
	if err := repo.UpdatePhone(ctx, userID, phone); err != nil {
		return SettingsView{}, err
	}
	return s.Settings(ctx, userID)
}

func maskPhone(value string) string {
	if len(value) != 11 {
		return "已绑定"
	}
	return value[:3] + "****" + value[7:]
}

func (s *Service) Blocks(ctx context.Context, userID uint) ([]BlockedUserView, error) {
	repo, ok := s.repo.(blocksRepository)
	if !ok {
		return nil, ErrSettingsUnavailable
	}
	return repo.ListBlocks(ctx, userID)
}

func (s *Service) AddBlock(ctx context.Context, userID, blockedUserID uint) error {
	if userID == 0 || blockedUserID == 0 || userID == blockedUserID {
		return ErrInvalidBlock
	}
	repo, ok := s.repo.(blocksRepository)
	if !ok {
		return ErrSettingsUnavailable
	}
	return repo.AddBlock(ctx, userID, blockedUserID)
}

func (s *Service) RemoveBlock(ctx context.Context, userID, blockedUserID uint) error {
	repo, ok := s.repo.(blocksRepository)
	if !ok {
		return ErrSettingsUnavailable
	}
	return repo.RemoveBlock(ctx, userID, blockedUserID)
}

func (s *Service) SubmitFeedback(ctx context.Context, userID uint, category, content string, assetIDs []string) (Feedback, error) {
	category, content = strings.TrimSpace(category), strings.TrimSpace(content)
	if !validFeedbackCategory(category) || utf8.RuneCountInString(content) < 5 || utf8.RuneCountInString(content) > 1000 || len(assetIDs) > 3 {
		return Feedback{}, ErrInvalidFeedback
	}
	repo, ok := s.repo.(feedbackRepository)
	if !ok {
		return Feedback{}, ErrSettingsUnavailable
	}
	reservation := FeedbackMediaReservation{}
	var err error
	if len(assetIDs) > 0 {
		if s.feedbackMedia == nil {
			return Feedback{}, ErrFeedbackUnavailable
		}
		reservation, err = s.feedbackMedia.ReserveFeedback(ctx, userID, assetIDs)
		if err != nil {
			return Feedback{}, err
		}
	}
	publicID, err := newFeedbackPublicID()
	if err != nil {
		_ = s.cancelFeedbackMedia(ctx, userID, reservation)
		return Feedback{}, err
	}
	value := Feedback{FeedbackPublicID: publicID, UserID: userID, Category: category, Content: content, Status: "open"}
	if err := repo.CreateFeedback(ctx, &value, assetIDs); err != nil {
		_ = s.cancelFeedbackMedia(ctx, userID, reservation)
		return Feedback{}, err
	}
	if len(assetIDs) > 0 {
		if err := s.feedbackMedia.CommitFeedback(ctx, userID, reservation, publicID); err != nil {
			_ = repo.DeleteFeedback(ctx, userID, value.ID)
			_ = s.cancelFeedbackMedia(ctx, userID, reservation)
			return Feedback{}, err
		}
	}
	value.Images = make([]FeedbackImage, 0, len(assetIDs))
	for index, assetID := range assetIDs {
		value.Images = append(value.Images, FeedbackImage{FeedbackID: value.ID, AssetPublicID: assetID, Slot: uint16(index)})
	}
	return value, nil
}

func (s *Service) cancelFeedbackMedia(ctx context.Context, userID uint, value FeedbackMediaReservation) error {
	if s.feedbackMedia == nil || value.Token == "" {
		return nil
	}
	return s.feedbackMedia.CancelFeedback(ctx, userID, value)
}

func validFeedbackCategory(value string) bool {
	switch value {
	case "bug", "suggestion", "report", "other":
		return true
	default:
		return false
	}
}

func (s *Service) AdminFeedbacks(ctx context.Context, page, size int) ([]Feedback, int64, error) {
	repo, ok := s.repo.(feedbackRepository)
	if !ok {
		return nil, 0, ErrSettingsUnavailable
	}
	return repo.ListFeedbacks(ctx, normalizePage(page), normalizeSize(size))
}

func (s *Service) CloseAccount(ctx context.Context, userID uint) error {
	value, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if value.Role != RoleUser {
		return &AccountClosureBlockedError{Blockers: []AccountClosureBlocker{{Code: "admin_role", Message: "管理员账号需先移交权限"}}}
	}
	if s.accountClosure == nil {
		return ErrAccountClosureUnavailable
	}
	blockers, err := s.accountClosure.Close(ctx, userID)
	if err != nil {
		return err
	}
	if len(blockers) > 0 {
		return &AccountClosureBlockedError{Blockers: blockers}
	}
	return nil
}
