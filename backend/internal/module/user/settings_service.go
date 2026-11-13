package user

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrSettingsUnavailable       = errors.New("user settings are unavailable")
	ErrPhoneBindingUnavailable   = errors.New("wechat phone binding is not configured")
	ErrInvalidPhone              = errors.New("invalid phone number")
	ErrPhoneTaken                = errors.New("phone number is already bound to another account")
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
	return settingsView(settings, user.OpenID != "", user.Phone, count), nil
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
	return settingsView(settings, user.OpenID != "", user.Phone, count), err
}

func settingsView(value UserSettings, wechatBound bool, phone *string, count int64) SettingsView {
	view := SettingsView{
		// Expose only the binding state, never the OpenID itself.
		WechatBound: wechatBound,
		OrderNotice: value.OrderNotice, SocialNotice: value.SocialNotice,
		QuietHours: value.QuietHours, Personalized: value.Personalized, BlacklistCount: count,
	}
	if phone != nil && *phone != "" {
		view.PhoneBound = true
		view.MaskedPhone = maskPhone(*phone)
	}
	return view
}

// takeOver 为 true 时表示用户已确认该手机号属于本人，可以把绑定从旧账号转移到当前账号。
func (s *Service) BindPhone(ctx context.Context, userID uint, code string, takeOver bool) (SettingsView, error) {
	phone, repo, err := s.resolvePhoneInput(ctx, code)
	if err != nil {
		return SettingsView{}, err
	}
	owner, err := repo.FindByPhone(ctx, phone)
	switch {
	case err == nil && owner.ID == userID && owner.Phone != nil && *owner.Phone == phone:
		return s.Settings(ctx, userID)
	case err == nil && !takeOver:
		return SettingsView{}, ErrPhoneTaken
	case err == nil && takeOver:
		// 只有持有该手机号的微信才能走到这里，转出方是同一号码的旧绑定。
		if detachErr := repo.DetachPhone(ctx, phone, owner.ID); detachErr != nil && !errors.Is(detachErr, ErrNotFound) {
			return SettingsView{}, detachErr
		}
	case err != nil && !errors.Is(err, ErrNotFound):
		return SettingsView{}, err
	}
	if err := repo.UpdatePhone(ctx, userID, phone); err != nil {
		return SettingsView{}, err
	}
	return s.Settings(ctx, userID)
}

func (s *Service) UnbindPhone(ctx context.Context, userID uint) (SettingsView, error) {
	repo, ok := s.repo.(settingsRepository)
	if !ok {
		return SettingsView{}, ErrSettingsUnavailable
	}
	if err := repo.ClearPhone(ctx, userID); err != nil {
		return SettingsView{}, err
	}
	return s.Settings(ctx, userID)
}

func (s *Service) resolvePhoneInput(ctx context.Context, code string) (string, settingsRepository, error) {
	repo, ok := s.repo.(settingsRepository)
	if !ok {
		return "", nil, ErrSettingsUnavailable
	}
	if s.phoneResolver == nil {
		return "", nil, ErrPhoneBindingUnavailable
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return "", nil, ErrInvalidPhone
	}
	phone, err := s.phoneResolver.ResolvePhone(ctx, code)
	if err != nil {
		return "", nil, fmt.Errorf("resolve wechat phone: %w", err)
	}
	phone = strings.TrimPrefix(strings.TrimSpace(phone), "+86")
	if !phonePattern.MatchString(phone) {
		return "", nil, ErrInvalidPhone
	}
	return phone, repo, nil
}

var phonePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

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

// MyFeedbackView 只暴露用户需要看的内容，不返回处理人身份。
type MyFeedbackView struct {
	ID        string     `json:"id"`
	Category  string     `json:"category"`
	Content   string     `json:"content"`
	Status    string     `json:"status"`
	Reply     string     `json:"reply"`
	CreatedAt time.Time  `json:"createdAt"`
	HandledAt *time.Time `json:"handledAt,omitempty"`
}

// 用户查看自己提交的反馈与处理结果。
func (s *Service) MyFeedbacks(ctx context.Context, userID uint, page, size int) ([]MyFeedbackView, int64, error) {
	repo, ok := s.repo.(feedbackRepository)
	if !ok {
		return nil, 0, ErrSettingsUnavailable
	}
	values, total, err := repo.ListFeedbacksByUser(ctx, userID, normalizePage(page), normalizeSize(size))
	if err != nil {
		return nil, 0, err
	}
	views := make([]MyFeedbackView, 0, len(values))
	for _, value := range values {
		views = append(views, MyFeedbackView{
			ID: value.FeedbackPublicID, Category: value.Category, Content: value.Content,
			Status: value.Status, Reply: value.AdminNote, CreatedAt: value.CreatedAt, HandledAt: value.HandledAt,
		})
	}
	return views, total, nil
}

func (s *Service) AdminFeedbacks(ctx context.Context, page, size int) ([]Feedback, int64, error) {
	repo, ok := s.repo.(feedbackRepository)
	if !ok {
		return nil, 0, ErrSettingsUnavailable
	}
	return repo.ListFeedbacks(ctx, normalizePage(page), normalizeSize(size))
}

var feedbackStatuses = map[string]bool{"open": true, "in_progress": true, "resolved": true, "ignored": true}

// 管理员处理用户反馈：记录状态、处理人、时间和说明，供后续追溯。
func (s *Service) AdminUpdateFeedback(ctx context.Context, actorID uint, publicID, status, note string) error {
	if !feedbackStatuses[status] {
		return errors.New("invalid feedback status")
	}
	if utf8.RuneCountInString(note) > 500 {
		return errors.New("feedback note is too long")
	}
	actor, err := s.repo.FindByID(ctx, actorID)
	if err != nil {
		return err
	}
	if actor.Status != StatusActive || (actor.Role != RoleAdmin && actor.Role != RoleSuperAdmin) {
		return ErrForbidden
	}
	repo, ok := s.repo.(feedbackRepository)
	if !ok {
		return ErrSettingsUnavailable
	}
	return repo.UpdateFeedbackHandling(ctx, publicID, status, strings.TrimSpace(note), actorID, time.Now().UTC())
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
