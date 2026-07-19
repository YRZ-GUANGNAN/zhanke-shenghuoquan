package identityapplication

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
)

var fixedTime = time.Date(2026, 7, 19, 8, 30, 0, 0, time.UTC)

func fixedClock() time.Time { return fixedTime }

type memoryRepository struct {
	nextID       uint64
	applications map[string]Application
	events       []ApplicationEvent
	saveErr      error
	steps        *[]string
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{nextID: 1, applications: make(map[string]Application)}
}

func currentKey(userID uint64, role Role) string {
	return fmt.Sprintf("%s:%d", role, userID)
}

func (r *memoryRepository) FindCurrent(_ context.Context, userID uint64, role Role) (Application, bool, error) {
	value, ok := r.applications[currentKey(userID, role)]
	return cloneApplication(value), ok, nil
}

func (r *memoryRepository) ListCurrent(_ context.Context, userID uint64) ([]Application, error) {
	values := make([]Application, 0, 2)
	for _, role := range []Role{RoleRider, RoleMerchant} {
		if value, ok := r.applications[currentKey(userID, role)]; ok {
			values = append(values, cloneApplication(value))
		}
	}
	return values, nil
}

func (r *memoryRepository) SaveSubmission(_ context.Context, value *Application, assetIDs []string, eventType EventType, expectedRowVersion uint64) error {
	if r.steps != nil {
		*r.steps = append(*r.steps, "save")
	}
	if r.saveErr != nil {
		return r.saveErr
	}
	key := currentKey(value.UserID, value.Role)
	current, exists := r.applications[key]
	if !exists {
		if expectedRowVersion != 0 {
			return ErrVersionConflict
		}
		value.ID = r.nextID
		r.nextID++
	} else {
		if current.Status == StatusPending {
			return ErrApplicationPending
		}
		if current.Status == StatusApproved {
			return ErrApplicationApproved
		}
		if current.RowVersion != expectedRowVersion {
			return ErrVersionConflict
		}
	}
	value.Assets = make([]ApplicationAsset, len(assetIDs))
	for index, assetID := range assetIDs {
		value.Assets[index] = ApplicationAsset{
			ID:            uint64(index + 1),
			ApplicationID: value.ID,
			AssetPublicID: assetID,
			SortOrder:     uint16(index),
			CreatedAt:     value.UpdatedAt,
		}
	}
	r.applications[key] = cloneApplication(*value)
	r.events = append(r.events, ApplicationEvent{
		ID:            uint64(len(r.events) + 1),
		ApplicationID: value.ID,
		ActorUserID:   value.UserID,
		EventType:     eventType,
		Status:        value.Status,
		CreatedAt:     value.UpdatedAt,
	})
	return nil
}

func (r *memoryRepository) ListAdmin(context.Context, AdminListFilter) ([]Application, int64, error) {
	return nil, 0, nil
}

func (r *memoryRepository) FindDetail(context.Context, string) (ApplicationDetail, error) {
	return ApplicationDetail{}, ErrApplicationNotFound
}

func (r *memoryRepository) reject(userID uint64, role Role, reviewerID uint64, reason string) {
	key := currentKey(userID, role)
	value := r.applications[key]
	value.Status = StatusRejected
	value.ReviewReason = reason
	value.ReviewerUserID = &reviewerID
	reviewedAt := fixedTime.Add(time.Hour)
	value.ReviewedAt = &reviewedAt
	value.RowVersion++
	value.UpdatedAt = reviewedAt
	r.applications[key] = value
}

func (r *memoryRepository) approve(userID uint64, role Role) {
	key := currentKey(userID, role)
	value := r.applications[key]
	value.Status = StatusApproved
	value.RowVersion++
	r.applications[key] = value
}

func cloneApplication(value Application) Application {
	value.Assets = append([]ApplicationAsset(nil), value.Assets...)
	return value
}

func TestSubmitRejectAndResubmit(t *testing.T) {
	repo := newMemoryRepository()
	media := &submissionMediaStub{}
	service := NewService(repo, media, fixedClock)

	first, err := service.Submit(context.Background(), 41, RoleRider, SubmitInput{
		RealName:       "  张同学  ",
		Phone:          " 13800138000 ",
		AssetPublicIDs: []string{" 01ARZ3NDEKTSV4RRFFQ69G5FAV "},
	})
	if err != nil || first.Status != StatusPending || first.RowVersion != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	if first.RealName != "张同学" || first.Phone != "13800138000" || len(first.AssetPublicIDs) != 1 || first.AssetPublicIDs[0] != "01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Fatalf("submission was not trimmed: %+v", first)
	}
	if first.ID == 0 || !validULID(first.ApplicationPublicID) {
		t.Fatalf("invalid identity application ids: %+v", first)
	}
	if len(repo.events) != 1 || repo.events[0].EventType != EventSubmitted {
		t.Fatalf("first event=%+v", repo.events)
	}

	if _, err := service.Submit(context.Background(), 41, RoleRider, SubmitInput{
		RealName: "重复", Phone: "13800138000", AssetPublicIDs: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"}, ExpectedRowVersion: 1,
	}); !errors.Is(err, ErrApplicationPending) {
		t.Fatalf("pending resubmit err=%v", err)
	}

	repo.reject(41, RoleRider, 7, "资料不清晰")
	second, err := service.Submit(context.Background(), 41, RoleRider, SubmitInput{
		RealName: " 张同学 ", Phone: "13800138000", AssetPublicIDs: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"}, ExpectedRowVersion: 2,
	})
	if err != nil || second.Status != StatusPending || second.ReviewReason != "" || second.ReviewerUserID != nil || second.ReviewedAt != nil || second.RowVersion != 3 {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	if len(repo.events) != 2 || repo.events[1].EventType != EventResubmitted {
		t.Fatalf("resubmit event=%+v", repo.events)
	}
	if media.existingID != first.ApplicationPublicID {
		t.Fatalf("resubmit existing reference=%q want=%q", media.existingID, first.ApplicationPublicID)
	}
}

func TestSubmitForbidsApprovedApplication(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(repo, &submissionMediaStub{}, fixedClock)
	if _, err := service.Submit(context.Background(), 41, RoleRider, validRiderInput()); err != nil {
		t.Fatal(err)
	}
	repo.approve(41, RoleRider)
	input := validRiderInput()
	input.ExpectedRowVersion = 2
	if _, err := service.Submit(context.Background(), 41, RoleRider, input); !errors.Is(err, ErrApplicationApproved) {
		t.Fatalf("approved resubmit err=%v", err)
	}
}

func TestSubmitTerminalStatePrecedesPayloadValidation(t *testing.T) {
	t.Run("pending", func(t *testing.T) {
		repo := newMemoryRepository()
		service := NewService(repo, &submissionMediaStub{}, fixedClock)
		if _, err := service.Submit(context.Background(), 41, RoleRider, validRiderInput()); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Submit(context.Background(), 41, RoleRider, SubmitInput{
			RealName: "重复", Phone: "13800138000", ExpectedRowVersion: 1,
		}); !errors.Is(err, ErrApplicationPending) {
			t.Fatalf("pending state must precede missing asset validation: %v", err)
		}
	})

	t.Run("approved", func(t *testing.T) {
		repo := newMemoryRepository()
		service := NewService(repo, &submissionMediaStub{}, fixedClock)
		if _, err := service.Submit(context.Background(), 41, RoleRider, validRiderInput()); err != nil {
			t.Fatal(err)
		}
		repo.approve(41, RoleRider)
		if _, err := service.Submit(context.Background(), 41, RoleRider, SubmitInput{
			RealName: "重复", Phone: "13800138000", ExpectedRowVersion: 2,
		}); !errors.Is(err, ErrApplicationApproved) {
			t.Fatalf("approved state must precede missing asset validation: %v", err)
		}
	})
}

func TestSubmitRequiresMatchingRejectedVersion(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(repo, &submissionMediaStub{}, fixedClock)
	if _, err := service.Submit(context.Background(), 41, RoleRider, validRiderInput()); err != nil {
		t.Fatal(err)
	}
	repo.reject(41, RoleRider, 7, "资料不清晰")
	input := validRiderInput()
	input.ExpectedRowVersion = 1
	if _, err := service.Submit(context.Background(), 41, RoleRider, input); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("version conflict err=%v", err)
	}
	input.ExpectedRowVersion = 2
	if _, err := service.Submit(context.Background(), 41, RoleRider, input); err != nil {
		t.Fatalf("matching version rejected: %v", err)
	}
}

func TestSubmitRejectsVersionOnFirstSubmission(t *testing.T) {
	service := NewService(newMemoryRepository(), &submissionMediaStub{}, fixedClock)
	input := validRiderInput()
	input.ExpectedRowVersion = 1
	if _, err := service.Submit(context.Background(), 41, RoleRider, input); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("first submission version err=%v", err)
	}
}

func TestValidationRejectsInvalidRoleAndUser(t *testing.T) {
	service := NewService(newMemoryRepository(), &submissionMediaStub{}, fixedClock)
	if _, err := service.Submit(context.Background(), 41, Role("student"), validRiderInput()); !errors.Is(err, ErrInvalidInput) || !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("invalid role err=%v", err)
	}
	if _, err := service.Submit(context.Background(), 0, RoleRider, validRiderInput()); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("zero user err=%v", err)
	}
}

func TestValidationRequiresMerchantFields(t *testing.T) {
	service := NewService(newMemoryRepository(), &submissionMediaStub{}, fixedClock)
	base := SubmitInput{
		RealName: "负责人", Phone: "13800138000", MerchantName: "校内店", MerchantAddress: "一号楼",
		AssetPublicIDs: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"},
	}
	tests := []struct {
		name   string
		mutate func(*SubmitInput)
	}{
		{name: "missing name", mutate: func(input *SubmitInput) { input.MerchantName = "  " }},
		{name: "missing address", mutate: func(input *SubmitInput) { input.MerchantAddress = "\t" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := base
			test.mutate(&input)
			if _, err := service.Submit(context.Background(), 41, RoleMerchant, input); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err=%v", err)
			}
		})
	}
	view, err := service.Submit(context.Background(), 41, RoleMerchant, SubmitInput{
		RealName: " 负责人 ", Phone: " 13800138000 ", MerchantName: " 校内店 ", MerchantAddress: " 一号楼 ",
		AssetPublicIDs: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"},
	})
	if err != nil || view.RealName != "负责人" || view.MerchantName != "校内店" || view.MerchantAddress != "一号楼" {
		t.Fatalf("trimmed merchant=%+v err=%v", view, err)
	}
}

func TestValidationRejectsInvalidPhone(t *testing.T) {
	service := NewService(newMemoryRepository(), &submissionMediaStub{}, fixedClock)
	for _, phone := range []string{"", "12800138000", "1380013800", "138001380000", "1380013800a", "+8613800138000"} {
		input := validRiderInput()
		input.Phone = phone
		if _, err := service.Submit(context.Background(), 41, RoleRider, input); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("phone %q err=%v", phone, err)
		}
	}
}

func TestValidationProofCountDuplicateAndULID(t *testing.T) {
	service := NewService(newMemoryRepository(), &submissionMediaStub{}, fixedClock)
	validIDs := []string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAV",
		"01ARZ3NDEKTSV4RRFFQ69G5FAW",
		"01ARZ3NDEKTSV4RRFFQ69G5FAX",
		"01ARZ3NDEKTSV4RRFFQ69G5FAY",
		"01ARZ3NDEKTSV4RRFFQ69G5FAZ",
		"01ARZ3NDEKTSV4RRFFQ69G5FB0",
	}
	tests := []struct {
		name string
		ids  []string
	}{
		{name: "none", ids: nil},
		{name: "seven", ids: append(append([]string{}, validIDs...), "01ARZ3NDEKTSV4RRFFQ69G5FB1")},
		{name: "duplicate after trim", ids: []string{validIDs[0], " " + validIDs[0] + " "}},
		{name: "invalid length", ids: []string{"not-a-ulid"}},
		{name: "invalid alphabet", ids: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAI"}},
		{name: "lowercase", ids: []string{"01arz3ndektsv4rrffq69g5fav"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validRiderInput()
			input.AssetPublicIDs = test.ids
			if _, err := service.Submit(context.Background(), 41, RoleRider, input); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err=%v", err)
			}
		})
	}
	input := validRiderInput()
	input.AssetPublicIDs = validIDs
	if _, err := service.Submit(context.Background(), 42, RoleRider, input); err != nil {
		t.Fatalf("six proofs rejected: %v", err)
	}
}

func TestValidationRejectsBlankOrOversizedNames(t *testing.T) {
	service := NewService(newMemoryRepository(), &submissionMediaStub{}, fixedClock)
	input := validRiderInput()
	input.RealName = " "
	if _, err := service.Submit(context.Background(), 41, RoleRider, input); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank real name err=%v", err)
	}
	input = validRiderInput()
	input.RealName = repeatRune('名', 65)
	if _, err := service.Submit(context.Background(), 41, RoleRider, input); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("long real name err=%v", err)
	}
	merchant := SubmitInput{
		RealName: "负责人", Phone: "13800138000", MerchantName: repeatRune('店', 121), MerchantAddress: "一号楼",
		AssetPublicIDs: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"},
	}
	if _, err := service.Submit(context.Background(), 41, RoleMerchant, merchant); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("long merchant name err=%v", err)
	}
	merchant.MerchantName = "校内店"
	merchant.MerchantAddress = repeatRune('址', 241)
	if _, err := service.Submit(context.Background(), 41, RoleMerchant, merchant); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("long merchant address err=%v", err)
	}
}

func TestApplicationViewSerializesUserIDsAsStrings(t *testing.T) {
	const largeUserID = uint64(9007199254740993)
	reviewerID := largeUserID + 1
	wantUserID := strconv.FormatUint(largeUserID, 10)
	wantReviewerID := strconv.FormatUint(reviewerID, 10)
	view := toApplicationView(Application{UserID: largeUserID, ReviewerUserID: &reviewerID})
	if view.UserID != wantUserID || view.ReviewerUserID == nil || *view.ReviewerUserID != wantReviewerID {
		t.Fatalf("view ids=%v/%v", view.UserID, view.ReviewerUserID)
	}
	event := ApplicationEventView{ActorUserID: wantUserID}

	payload, err := json.Marshal(struct {
		Application ApplicationView      `json:"application"`
		Event       ApplicationEventView `json:"event"`
	}{Application: view, Event: event})
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Application struct {
			UserID         string  `json:"userId"`
			ReviewerUserID *string `json:"reviewerUserId"`
		} `json:"application"`
		Event struct {
			ActorUserID string `json:"actorUserId"`
		} `json:"event"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("user ids were not encoded as JSON strings: %s: %v", payload, err)
	}
	if decoded.Application.UserID != wantUserID || decoded.Application.ReviewerUserID == nil ||
		*decoded.Application.ReviewerUserID != wantReviewerID || decoded.Event.ActorUserID != wantUserID {
		t.Fatalf("decoded=%+v payload=%s", decoded, payload)
	}
}

func TestSaveSubmissionRejectsWrongEventType(t *testing.T) {
	repository := NewRepository(nil)
	created := Application{UserID: 41, Role: RoleRider, Status: StatusPending, RowVersion: 1}
	if err := repository.SaveSubmission(context.Background(), &created, []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"}, EventApproved, 0); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("new application accepted approved event: %v", err)
	}
	existing := Application{ID: 7, UserID: 41, Role: RoleRider, Status: StatusPending, RowVersion: 3}
	if err := repository.SaveSubmission(context.Background(), &existing, []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"}, EventSubmitted, 2); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("existing application accepted submitted event: %v", err)
	}
	if err := repository.SaveSubmission(context.Background(), &existing, []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"}, EventRejected, 2); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("existing application accepted rejected event: %v", err)
	}
}

type submissionMediaStub struct {
	reservation MediaReservation
	reserveErr  error
	ensureErr   error
	cancelErr   error
	steps       *[]string
	cancelled   bool
	existingID  string
}

func (m *submissionMediaStub) Reserve(_ context.Context, _ uint64, assetIDs []string) (MediaReservation, error) {
	if m.steps != nil {
		*m.steps = append(*m.steps, "reserve")
	}
	if m.reserveErr != nil {
		return MediaReservation{}, m.reserveErr
	}
	value := m.reservation
	if value.Token == "" {
		value.Token = "m10_01J00000000000000000000005"
	}
	if len(value.AssetPublicIDs) == 0 {
		value.AssetPublicIDs = append([]string(nil), assetIDs...)
	}
	return value, nil
}

func (m *submissionMediaStub) ReserveForApplication(ctx context.Context, ownerID uint64, applicationPublicID string, assetIDs []string) (MediaReservation, error) {
	m.existingID = applicationPublicID
	return m.Reserve(ctx, ownerID, assetIDs)
}

func (m *submissionMediaStub) Commit(context.Context, uint64, MediaReservation, string) error {
	panic("Submit must use EnsureCommitted")
}

func (m *submissionMediaStub) EnsureCommitted(_ context.Context, _ uint64, _ MediaReservation, _ string) error {
	if m.steps != nil {
		*m.steps = append(*m.steps, "ensure")
	}
	return m.ensureErr
}

func (m *submissionMediaStub) Cancel(context.Context, uint64, MediaReservation) error {
	if m.steps != nil {
		*m.steps = append(*m.steps, "cancel")
	}
	m.cancelled = true
	return m.cancelErr
}

func (m *submissionMediaStub) ResolveAdminProofs(context.Context, string, []string) ([]ProofAssetView, error) {
	return nil, nil
}

func TestSubmitMediaLifecycleReserveSaveEnsureCommitted(t *testing.T) {
	steps := make([]string, 0, 3)
	repo := newMemoryRepository()
	repo.steps = &steps
	media := &submissionMediaStub{steps: &steps}
	service := NewService(repo, media, fixedClock)

	value, err := service.Submit(context.Background(), 41, RoleRider, validRiderInput())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(steps, ",") != "reserve,save,ensure" || media.cancelled || value.Status != StatusPending {
		t.Fatalf("steps=%v cancelled=%v value=%+v", steps, media.cancelled, value)
	}
}

func TestSubmitMediaLifecycleCancelsReservationWhenSaveFails(t *testing.T) {
	steps := make([]string, 0, 3)
	repo := newMemoryRepository()
	repo.steps, repo.saveErr = &steps, ErrVersionConflict
	media := &submissionMediaStub{steps: &steps}
	service := NewService(repo, media, fixedClock)

	if _, err := service.Submit(context.Background(), 41, RoleRider, validRiderInput()); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("err=%v", err)
	}
	if strings.Join(steps, ",") != "reserve,save,cancel" || !media.cancelled {
		t.Fatalf("steps=%v cancelled=%v", steps, media.cancelled)
	}
}

func TestSubmitMediaLifecycleCommitConflictDoesNotReportSuccess(t *testing.T) {
	steps := make([]string, 0, 4)
	repo := newMemoryRepository()
	repo.steps = &steps
	media := &submissionMediaStub{steps: &steps, ensureErr: ErrProofMediaConflict}
	service := NewService(repo, media, fixedClock)

	value, err := service.Submit(context.Background(), 41, RoleRider, validRiderInput())
	if !errors.Is(err, ErrProofMediaConflict) || value.ApplicationPublicID != "" {
		t.Fatalf("value=%+v err=%v", value, err)
	}
	if strings.Join(steps, ",") != "reserve,save,ensure,cancel" || !media.cancelled {
		t.Fatalf("steps=%v cancelled=%v", steps, media.cancelled)
	}
	// The application transaction is durable before the cross-domain binding.
	// The caller gets a conflict (never false success), and the pending row is
	// retained for an operator-safe reconciliation instead of being deleted.
	if current, found, findErr := repo.FindCurrent(context.Background(), 41, RoleRider); findErr != nil || !found || current.Status != StatusPending {
		t.Fatalf("saved boundary found=%v current=%+v err=%v", found, current, findErr)
	}
}

func TestSubmitMediaLifecycleMapsOwnershipConflictAndSkipsSave(t *testing.T) {
	steps := make([]string, 0, 1)
	repo := newMemoryRepository()
	repo.steps = &steps
	media := &submissionMediaStub{steps: &steps, reserveErr: coremedia.ErrBindingInvalidAsset}
	service := NewService(repo, media, fixedClock)

	if _, err := service.Submit(context.Background(), 41, RoleRider, validRiderInput()); !errors.Is(err, ErrProofMediaConflict) {
		t.Fatalf("err=%v", err)
	}
	if strings.Join(steps, ",") != "reserve" {
		t.Fatalf("steps=%v", steps)
	}
}

func validRiderInput() SubmitInput {
	return SubmitInput{
		RealName:       "张同学",
		Phone:          "13800138000",
		AssetPublicIDs: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"},
	}
}

func repeatRune(value rune, count int) string {
	runes := make([]rune, count)
	for index := range runes {
		runes[index] = value
	}
	return string(runes)
}
