package user

import (
	"context"
	"errors"
	"testing"
)

type domainARepositoryStub struct {
	*memoryRepository
	settings  map[uint]UserSettings
	blocks    map[uint]map[uint]bool
	feedbacks []Feedback
}

func newDomainARepositoryStub(values ...User) *domainARepositoryStub {
	return &domainARepositoryStub{
		memoryRepository: newMemoryRepository(values...), settings: map[uint]UserSettings{}, blocks: map[uint]map[uint]bool{},
	}
}

func (r *domainARepositoryStub) GetOrCreateSettings(_ context.Context, userID uint) (UserSettings, error) {
	value, ok := r.settings[userID]
	if !ok {
		value = defaultUserSettings(userID)
		r.settings[userID] = value
	}
	return value, nil
}

func (r *domainARepositoryStub) UpdateSettings(_ context.Context, userID uint, value UserSettings) (UserSettings, error) {
	value.UserID = userID
	r.settings[userID] = value
	return value, nil
}

func (r *domainARepositoryStub) UpdatePhone(_ context.Context, userID uint, phone string) error {
	value, ok := r.users[userID]
	if !ok {
		return ErrNotFound
	}
	value.Phone = &phone
	r.users[userID] = value
	return nil
}

func (r *domainARepositoryStub) ListBlocks(_ context.Context, userID uint) ([]BlockedUserView, error) {
	values := []BlockedUserView{}
	for blockedID := range r.blocks[userID] {
		profile := r.users[blockedID].Profile()
		values = append(values, BlockedUserView{UserID: blockedID, Nickname: profile.Nickname, Avatar: profile.Avatar})
	}
	return values, nil
}

func (r *domainARepositoryStub) AddBlock(_ context.Context, userID, blockedUserID uint) error {
	if _, ok := r.users[blockedUserID]; !ok {
		return ErrNotFound
	}
	if r.blocks[userID] == nil {
		r.blocks[userID] = map[uint]bool{}
	}
	r.blocks[userID][blockedUserID] = true
	return nil
}

func (r *domainARepositoryStub) RemoveBlock(_ context.Context, userID, blockedUserID uint) error {
	delete(r.blocks[userID], blockedUserID)
	return nil
}

func (r *domainARepositoryStub) CountBlocks(_ context.Context, userID uint) (int64, error) {
	return int64(len(r.blocks[userID])), nil
}

func (r *domainARepositoryStub) CreateFeedback(_ context.Context, value *Feedback, assetIDs []string) error {
	value.ID = uint(len(r.feedbacks) + 1)
	r.feedbacks = append(r.feedbacks, *value)
	return nil
}

func (r *domainARepositoryStub) DeleteFeedback(_ context.Context, userID, feedbackID uint) error {
	for index, value := range r.feedbacks {
		if value.ID == feedbackID && value.UserID == userID {
			r.feedbacks = append(r.feedbacks[:index], r.feedbacks[index+1:]...)
		}
	}
	return nil
}

func (r *domainARepositoryStub) ListFeedbacks(_ context.Context, _, _ int) ([]Feedback, int64, error) {
	return append([]Feedback(nil), r.feedbacks...), int64(len(r.feedbacks)), nil
}

type phoneResolverStub struct{ phone string }

func (s phoneResolverStub) ResolvePhone(context.Context, string) (string, error) { return s.phone, nil }

type feedbackMediaStub struct {
	reserved, committed, cancelled bool
}

func (s *feedbackMediaStub) ReserveFeedback(_ context.Context, _ uint, ids []string) (FeedbackMediaReservation, error) {
	s.reserved = true
	return FeedbackMediaReservation{Token: "reservation", AssetIDs: append([]string(nil), ids...)}, nil
}
func (s *feedbackMediaStub) CommitFeedback(context.Context, uint, FeedbackMediaReservation, string) error {
	s.committed = true
	return nil
}
func (s *feedbackMediaStub) CancelFeedback(context.Context, uint, FeedbackMediaReservation) error {
	s.cancelled = true
	return nil
}

type closureStub struct{ blockers []AccountClosureBlocker }

func (s closureStub) Close(context.Context, uint) ([]AccountClosureBlocker, error) {
	return s.blockers, nil
}

func TestSettingsPhoneBlocksAndFeedbackContracts(t *testing.T) {
	repo := newDomainARepositoryStub(
		User{ID: 1, Nickname: "甲", Role: RoleUser, Status: StatusActive},
		User{ID: 2, Nickname: "乙", Role: RoleUser, Status: StatusActive},
	)
	service := NewService(repo, testSecret, "", "", true, WithWechatPhoneResolver(phoneResolverStub{phone: "+8613800138000"}))
	media := &feedbackMediaStub{}
	service.SetFeedbackMedia(media)

	settings, err := service.Settings(context.Background(), 1)
	if err != nil || !settings.OrderNotice || !settings.SocialNotice || !settings.Personalized || settings.QuietHours {
		t.Fatalf("unexpected default settings: %+v err=%v", settings, err)
	}
	settings, err = service.BindPhone(context.Background(), 1, "wechat-code")
	if err != nil || !settings.PhoneBound || settings.MaskedPhone != "138****8000" {
		t.Fatalf("unexpected phone view: %+v err=%v", settings, err)
	}
	if err := service.AddBlock(context.Background(), 1, 1); !errors.Is(err, ErrInvalidBlock) {
		t.Fatalf("self block accepted: %v", err)
	}
	if err := service.AddBlock(context.Background(), 1, 2); err != nil {
		t.Fatal(err)
	}
	settings, _ = service.Settings(context.Background(), 1)
	if settings.BlacklistCount != 1 {
		t.Fatalf("blacklist count=%d want=1", settings.BlacklistCount)
	}

	feedback, err := service.SubmitFeedback(context.Background(), 1, "bug", "页面按钮无法点击", []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"})
	if err != nil || feedback.FeedbackPublicID == "" || !media.reserved || !media.committed || media.cancelled {
		t.Fatalf("feedback=%+v media=%+v err=%v", feedback, media, err)
	}
}

func TestCloseAccountReturnsStructuredBlockers(t *testing.T) {
	repo := newDomainARepositoryStub(User{ID: 1, Role: RoleUser, Status: StatusActive})
	service := NewService(repo, testSecret, "", "", true)
	service.SetAccountClosureCoordinator(closureStub{blockers: []AccountClosureBlocker{{Code: "delivery_order", Message: "仍有未完成的配送订单"}}})
	err := service.CloseAccount(context.Background(), 1)
	var blocked *AccountClosureBlockedError
	if !errors.As(err, &blocked) || len(blocked.Blockers) != 1 || blocked.Blockers[0].Code != "delivery_order" {
		t.Fatalf("unexpected close error: %#v", err)
	}
}
