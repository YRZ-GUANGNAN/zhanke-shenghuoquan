package user

import (
	"context"
	"errors"
	"testing"
	"time"
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
	for id, other := range r.users {
		if id != userID && other.Phone != nil && *other.Phone == phone {
			return ErrPhoneTaken
		}
	}
	value.Phone = &phone
	r.users[userID] = value
	return nil
}

func (r *domainARepositoryStub) ClearPhone(_ context.Context, userID uint) error {
	value, ok := r.users[userID]
	if !ok {
		return ErrNotFound
	}
	value.Phone = nil
	r.users[userID] = value
	return nil
}

func (r *domainARepositoryStub) FindByPhone(_ context.Context, phone string) (User, error) {
	for _, value := range r.users {
		if value.Phone != nil && *value.Phone == phone {
			return value, nil
		}
	}
	return User{}, ErrNotFound
}

func (r *domainARepositoryStub) DetachPhone(_ context.Context, phone string, expectedOwner uint) error {
	value, ok := r.users[expectedOwner]
	if !ok || value.Phone == nil || *value.Phone != phone {
		return ErrNotFound
	}
	value.Phone = nil
	r.users[expectedOwner] = value
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

func (r *domainARepositoryStub) ListFeedbacksByUser(_ context.Context, userID uint, _, _ int) ([]Feedback, int64, error) {
	values := make([]Feedback, 0, len(r.feedbacks))
	for _, value := range r.feedbacks {
		if value.UserID == userID {
			values = append(values, value)
		}
	}
	return values, int64(len(values)), nil
}

func (r *domainARepositoryStub) UpdateFeedbackHandling(_ context.Context, publicID, status, note string, actorID uint, now time.Time) error {
	for index, value := range r.feedbacks {
		if value.FeedbackPublicID != publicID {
			continue
		}
		r.feedbacks[index].Status = status
		r.feedbacks[index].AdminNote = note
		r.feedbacks[index].HandledBy = &actorID
		r.feedbacks[index].HandledAt = &now
		return nil
	}
	return ErrNotFound
}

func (r *domainARepositoryStub) DeleteFeedback(_ context.Context, userID, feedbackID uint) error {
	for index, value := range r.feedbacks {
		if value.ID == feedbackID && value.UserID == userID {
			r.feedbacks = append(r.feedbacks[:index], r.feedbacks[index+1:]...)
		}
	}
	return nil
}

func (r *domainARepositoryStub) ListFeedbacks(_ context.Context, _, _ int, status string) ([]Feedback, int64, error) {
	values := make([]Feedback, 0, len(r.feedbacks))
	for _, value := range r.feedbacks {
		if status == "" || value.Status == status {
			values = append(values, value)
		}
	}
	return values, int64(len(values)), nil
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
	settings, err = service.BindPhone(context.Background(), 1, "wechat-code", false)
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

func TestBindPhoneDetectsConflictAndSupportsTakeOver(t *testing.T) {
	repo := newDomainARepositoryStub(
		User{ID: 1, OpenID: "openid-a", Role: RoleUser, Status: StatusActive},
		User{ID: 2, OpenID: "openid-b", Role: RoleUser, Status: StatusActive},
	)
	service := NewService(repo, testSecret, "", "", true, WithWechatPhoneResolver(phoneResolverStub{phone: "+8613800138000"}))
	ctx := context.Background()

	if _, err := service.BindPhone(ctx, 1, "code", false); err != nil {
		t.Fatalf("first bind failed: %v", err)
	}
	if _, err := service.BindPhone(ctx, 2, "code", false); !errors.Is(err, ErrPhoneTaken) {
		t.Fatalf("second account must be rejected with ErrPhoneTaken, got %v", err)
	}
	// 重复绑定同一号码不应被视为冲突。
	if _, err := service.BindPhone(ctx, 1, "code", false); err != nil {
		t.Fatalf("re-binding same phone must be a no-op: %v", err)
	}

	settings, err := service.BindPhone(ctx, 2, "code", true)
	if err != nil {
		t.Fatalf("take over failed: %v", err)
	}
	if !settings.PhoneBound || settings.MaskedPhone != "138****8000" {
		t.Fatalf("take over must bind phone to the caller: %+v", settings)
	}
	previous, _ := service.Settings(ctx, 1)
	if previous.PhoneBound {
		t.Fatalf("previous owner must lose the phone binding: %+v", previous)
	}
}

func TestUnbindPhoneReleasesNumber(t *testing.T) {
	repo := newDomainARepositoryStub(
		User{ID: 1, OpenID: "openid-a", Role: RoleUser, Status: StatusActive},
		User{ID: 2, OpenID: "openid-b", Role: RoleUser, Status: StatusActive},
	)
	service := NewService(repo, testSecret, "", "", true, WithWechatPhoneResolver(phoneResolverStub{phone: "+8613800138000"}))
	ctx := context.Background()
	if _, err := service.BindPhone(ctx, 1, "code", false); err != nil {
		t.Fatal(err)
	}
	settings, err := service.UnbindPhone(ctx, 1)
	if err != nil || settings.PhoneBound {
		t.Fatalf("unbind failed: %+v err=%v", settings, err)
	}
	if _, err := service.BindPhone(ctx, 2, "code", false); err != nil {
		t.Fatalf("released number must be bindable: %v", err)
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
