package identityapplication

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
	"gorm.io/gorm"
)

var fixedTime = time.Date(2026, 7, 19, 8, 30, 0, 0, time.UTC)

func fixedClock() time.Time { return fixedTime }

type memoryRepository struct {
	nextID       uint64
	applications map[string]Application
	grants       map[string]IdentityGrant
	events       []ApplicationEvent
	saveErr      error
	steps        *[]string
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{nextID: 1, applications: make(map[string]Application), grants: make(map[string]IdentityGrant)}
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

func (r *memoryRepository) SaveSubmission(ctx context.Context, value *Application, assetIDs []string, eventType EventType, expectedRowVersion uint64, commit SubmissionCommit) error {
	if r.steps != nil {
		*r.steps = append(*r.steps, "save")
	}
	if r.saveErr != nil {
		return r.saveErr
	}
	if commit == nil {
		return ErrInvalidInput
	}
	originalValue := cloneApplication(*value)
	originalNextID := r.nextID
	originalApplications := cloneApplicationMap(r.applications)
	originalEvents := append([]ApplicationEvent(nil), r.events...)
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
	if err := commit(ctx, nil); err != nil {
		r.nextID = originalNextID
		r.applications = originalApplications
		r.events = originalEvents
		*value = originalValue
		return err
	}
	return nil
}

func (r *memoryRepository) ListAdmin(context.Context, AdminListFilter) ([]Application, int64, error) {
	return nil, 0, nil
}

func (r *memoryRepository) FindDetail(_ context.Context, applicationPublicID string) (ApplicationDetail, error) {
	var value Application
	found := false
	for _, candidate := range r.applications {
		if candidate.ApplicationPublicID == applicationPublicID {
			value, found = cloneApplication(candidate), true
			break
		}
	}
	if !found {
		return ApplicationDetail{}, ErrApplicationNotFound
	}
	events := make([]ApplicationEventView, 0)
	for _, event := range r.events {
		if event.ApplicationID != value.ID {
			continue
		}
		events = append(events, ApplicationEventView{
			EventPublicID: event.EventPublicID,
			ActorUserID:   strconv.FormatUint(event.ActorUserID, 10),
			EventType:     event.EventType,
			Status:        event.Status,
			Reason:        event.Reason,
			CreatedAt:     event.CreatedAt,
		})
	}
	return ApplicationDetail{ApplicationView: toApplicationView(value), Events: events}, nil
}

func (r *memoryRepository) Approve(ctx context.Context, input DecisionInput, commit ApprovalCommit) (Application, error) {
	if commit == nil {
		return Application{}, ErrInvalidInput
	}
	return r.decide(ctx, input, StatusApproved, EventApproved, commit)
}

func (r *memoryRepository) Reject(ctx context.Context, input DecisionInput) (Application, error) {
	return r.decide(ctx, input, StatusRejected, EventRejected, nil)
}

func (r *memoryRepository) decide(ctx context.Context, input DecisionInput, status Status, eventType EventType, commit ApprovalCommit) (Application, error) {
	var key string
	var value Application
	found := false
	for candidateKey, candidate := range r.applications {
		if candidate.ApplicationPublicID == input.ApplicationPublicID {
			key, value, found = candidateKey, cloneApplication(candidate), true
			break
		}
	}
	if !found {
		return Application{}, ErrApplicationNotFound
	}
	if value.UserID == input.ReviewerUserID {
		return Application{}, ErrSelfReview
	}
	if value.Status != StatusPending || value.RowVersion != input.ExpectedRowVersion {
		return Application{}, ErrDecisionConflict
	}
	originalApplications := cloneApplicationMap(r.applications)
	originalGrants := cloneGrantMap(r.grants)
	originalEvents := append([]ApplicationEvent(nil), r.events...)
	if status == StatusApproved {
		sourceID := value.ID
		grant := IdentityGrant{
			UserID: value.UserID, Role: value.Role, Status: GrantActive,
			SourceApplicationID: &sourceID, GrantedAt: input.DecidedAt,
			RowVersion: 1, CreatedAt: input.DecidedAt, UpdatedAt: input.DecidedAt,
		}
		if current, ok := r.grants[key]; ok {
			grant.ID = current.ID
			grant.CreatedAt = current.CreatedAt
			grant.RowVersion = current.RowVersion + 1
		}
		r.grants[key] = grant
		if err := commit(ctx, nil, ApprovedIdentity{UserID: value.UserID, Role: value.Role, RealName: value.RealName, Phone: value.Phone}); err != nil {
			r.applications, r.grants, r.events = originalApplications, originalGrants, originalEvents
			return Application{}, err
		}
	}
	value.Status = status
	value.ReviewReason = input.Reason
	reviewerID := input.ReviewerUserID
	value.ReviewerUserID = &reviewerID
	reviewedAt := input.DecidedAt
	value.ReviewedAt = &reviewedAt
	value.RowVersion++
	value.UpdatedAt = input.DecidedAt
	r.applications[key] = cloneApplication(value)
	r.events = append(r.events, ApplicationEvent{
		ID: uint64(len(r.events) + 1), ApplicationID: value.ID,
		ActorUserID: input.ReviewerUserID, EventType: eventType,
		Status: status, Reason: input.Reason, CreatedAt: input.DecidedAt,
	})
	return value, nil
}

func (r *memoryRepository) hasGrant(userID uint64, role Role) bool {
	_, ok := r.grants[currentKey(userID, role)]
	return ok
}

func (r *memoryRepository) grant(userID uint64, role Role) (IdentityGrant, bool) {
	value, ok := r.grants[currentKey(userID, role)]
	return cloneGrant(value), ok
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

func cloneApplicationMap(values map[string]Application) map[string]Application {
	cloned := make(map[string]Application, len(values))
	for key, value := range values {
		cloned[key] = cloneApplication(value)
	}
	return cloned
}

func cloneGrant(value IdentityGrant) IdentityGrant {
	if value.SourceApplicationID != nil {
		sourceID := *value.SourceApplicationID
		value.SourceApplicationID = &sourceID
	}
	return value
}

func cloneGrantMap(values map[string]IdentityGrant) map[string]IdentityGrant {
	cloned := make(map[string]IdentityGrant, len(values))
	for key, value := range values {
		cloned[key] = cloneGrant(value)
	}
	return cloned
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
	commit := func(context.Context, *gorm.DB) error { return nil }
	valid := Application{UserID: 41, Role: RoleRider, Status: StatusPending, RowVersion: 1}
	if err := repository.SaveSubmission(context.Background(), &valid, []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"}, EventSubmitted, 0, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("submission accepted nil transactional media commit: %v", err)
	}
	created := Application{UserID: 41, Role: RoleRider, Status: StatusPending, RowVersion: 1}
	if err := repository.SaveSubmission(context.Background(), &created, []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"}, EventApproved, 0, commit); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("new application accepted approved event: %v", err)
	}
	existing := Application{ID: 7, UserID: 41, Role: RoleRider, Status: StatusPending, RowVersion: 3}
	if err := repository.SaveSubmission(context.Background(), &existing, []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"}, EventSubmitted, 2, commit); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("existing application accepted submitted event: %v", err)
	}
	if err := repository.SaveSubmission(context.Background(), &existing, []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"}, EventRejected, 2, commit); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("existing application accepted rejected event: %v", err)
	}
}

type submissionMediaStub struct {
	reservation   MediaReservation
	reserveErr    error
	ensureErr     error
	cancelErr     error
	steps         *[]string
	cancelled     bool
	cancelCtxErr  error
	existingID    string
	assetIDs      map[string]uint64
	liveIDs       []uint64
	revokedIDs    []uint64
	resolveAppID  string
	resolveIDs    []string
	resolveProofs []ProofAssetView
	resolveErr    error
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
	if len(value.InternalAssetIDs) == 0 {
		if m.assetIDs == nil {
			m.assetIDs = make(map[string]uint64)
		}
		value.InternalAssetIDs = make([]uint64, len(assetIDs))
		for index, assetID := range assetIDs {
			internalID, exists := m.assetIDs[assetID]
			if !exists {
				internalID = uint64(101 + len(m.assetIDs))
				m.assetIDs[assetID] = internalID
			}
			value.InternalAssetIDs[index] = internalID
		}
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
	panic("Submit must use EnsureCommittedInTransaction")
}

func (m *submissionMediaStub) EnsureCommittedInTransaction(_ context.Context, _ *gorm.DB, _ uint64, reservation MediaReservation, _ string) error {
	if m.steps != nil {
		*m.steps = append(*m.steps, "ensure")
	}
	if m.ensureErr != nil {
		return m.ensureErr
	}
	keep := make(map[uint64]struct{}, len(reservation.InternalAssetIDs))
	for _, internalID := range reservation.InternalAssetIDs {
		keep[internalID] = struct{}{}
	}
	for _, internalID := range m.liveIDs {
		if _, retained := keep[internalID]; !retained {
			m.revokedIDs = append(m.revokedIDs, internalID)
		}
	}
	m.liveIDs = append([]uint64(nil), reservation.InternalAssetIDs...)
	return nil
}

func (m *submissionMediaStub) Cancel(ctx context.Context, _ uint64, _ MediaReservation) error {
	if m.steps != nil {
		*m.steps = append(*m.steps, "cancel")
	}
	m.cancelled = true
	m.cancelCtxErr = ctx.Err()
	return m.cancelErr
}

func (m *submissionMediaStub) ResolveAdminProofs(_ context.Context, applicationPublicID string, assetIDs []string) ([]ProofAssetView, error) {
	m.resolveAppID = applicationPublicID
	m.resolveIDs = append([]string(nil), assetIDs...)
	return append([]ProofAssetView(nil), m.resolveProofs...), m.resolveErr
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

func TestSubmitMediaLifecycleCancelsReservationAfterRequestContextCancellation(t *testing.T) {
	repo := newMemoryRepository()
	repo.saveErr = context.Canceled
	media := &submissionMediaStub{}
	service := NewService(repo, media, fixedClock)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := service.Submit(ctx, 41, RoleRider, validRiderInput()); !errors.Is(err, context.Canceled) {
		t.Fatalf("submit err=%v", err)
	}
	if !media.cancelled || media.cancelCtxErr != nil {
		t.Fatalf("cleanup reused cancelled request context: cancelled=%v contextErr=%v", media.cancelled, media.cancelCtxErr)
	}
}

func TestSubmitMediaLifecycleCommitConflictRollsBackSubmission(t *testing.T) {
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
	if current, found, findErr := repo.FindCurrent(context.Background(), 41, RoleRider); findErr != nil || found || current.ID != 0 {
		t.Fatalf("failed media commit left application: found=%v current=%+v err=%v", found, current, findErr)
	}
	if len(repo.events) != 0 {
		t.Fatalf("failed media commit left events: %+v", repo.events)
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

func TestSubmitResubmissionReplacesLiveProofSetWithoutAccumulation(t *testing.T) {
	const (
		assetA = "01J00000000000000000000011"
		assetB = "01J00000000000000000000012"
		assetC = "01J00000000000000000000013"
	)
	repo := newMemoryRepository()
	media := &submissionMediaStub{}
	service := NewService(repo, media, fixedClock)
	input := validRiderInput()
	input.AssetPublicIDs = []string{assetA, assetB}
	first, err := service.Submit(context.Background(), 41, RoleRider, input)
	if err != nil {
		t.Fatal(err)
	}

	repo.reject(41, RoleRider, 7, "换一张")
	input.AssetPublicIDs = []string{assetB, assetC}
	input.ExpectedRowVersion = 2
	if _, err := service.Submit(context.Background(), 41, RoleRider, input); err != nil {
		t.Fatal(err)
	}
	wantBC := []uint64{media.assetIDs[assetB], media.assetIDs[assetC]}
	if !reflect.DeepEqual(media.liveIDs, wantBC) || !reflect.DeepEqual(media.revokedIDs, []uint64{media.assetIDs[assetA]}) {
		t.Fatalf("A+B -> B+C live=%v revoked=%v ids=%v", media.liveIDs, media.revokedIDs, media.assetIDs)
	}
	if media.existingID != first.ApplicationPublicID {
		t.Fatalf("existing reference=%q want=%q", media.existingID, first.ApplicationPublicID)
	}

	repo.reject(41, RoleRider, 7, "再换一张")
	input.AssetPublicIDs = []string{assetC}
	input.ExpectedRowVersion = 4
	if _, err := service.Submit(context.Background(), 41, RoleRider, input); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(media.liveIDs, []uint64{media.assetIDs[assetC]}) ||
		!reflect.DeepEqual(media.revokedIDs, []uint64{media.assetIDs[assetA], media.assetIDs[assetB]}) {
		t.Fatalf("third submission accumulated live references: live=%v revoked=%v", media.liveIDs, media.revokedIDs)
	}
	if len(repo.events) != 3 {
		t.Fatalf("review audit events were removed: %+v", repo.events)
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

type approvalProvisionerStub struct {
	calls []ApprovedIdentity
	err   error
}

func (p *approvalProvisionerStub) ProvisionApproved(_ context.Context, _ *gorm.DB, value ApprovedIdentity) error {
	p.calls = append(p.calls, value)
	return p.err
}

func TestApproveForbidsSelfReviewAndProvisionFailureIsAtomic(t *testing.T) {
	repo := newMemoryRepository()
	provisioner := &approvalProvisionerStub{}
	service := NewService(repo, &submissionMediaStub{}, fixedClock, provisioner)
	pending, err := service.Submit(context.Background(), 41, RoleRider, validRiderInput())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.Approve(context.Background(), 41, pending.ApplicationPublicID, pending.RowVersion); !errors.Is(err, ErrSelfReview) {
		t.Fatalf("self review err=%v", err)
	}
	current, _, _ := repo.FindCurrent(context.Background(), 41, RoleRider)
	if current.Status != StatusPending || current.RowVersion != 1 {
		t.Fatalf("self review mutated application: %+v", current)
	}

	provisioner.err = errors.New("rider provisioning failed")
	if _, err := service.Approve(context.Background(), 7, pending.ApplicationPublicID, pending.RowVersion); err == nil {
		t.Fatal("provision failure was accepted")
	}
	current, _, _ = repo.FindCurrent(context.Background(), 41, RoleRider)
	if current.Status != StatusPending || current.RowVersion != 1 || repo.hasGrant(41, RoleRider) {
		t.Fatalf("failed approval left partial state: app=%+v grants=%+v", current, repo.grants)
	}
	if len(repo.events) != 1 || repo.events[0].EventType != EventSubmitted {
		t.Fatalf("failed approval left event: %+v", repo.events)
	}
}

func TestApproveRiderAndMerchantWritesGrantAndImmutableEvent(t *testing.T) {
	repo := newMemoryRepository()
	provisioner := &approvalProvisionerStub{}
	service := NewService(repo, &submissionMediaStub{}, fixedClock, provisioner)
	rider, err := service.Submit(context.Background(), 41, RoleRider, validRiderInput())
	if err != nil {
		t.Fatal(err)
	}
	merchantInput := SubmitInput{
		RealName: "负责人", Phone: "13900139000", MerchantName: "校园店", MerchantAddress: "一号楼",
		AssetPublicIDs: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAW"},
	}
	merchant, err := service.Submit(context.Background(), 42, RoleMerchant, merchantInput)
	if err != nil {
		t.Fatal(err)
	}

	approvedRider, err := service.Approve(context.Background(), 7, rider.ApplicationPublicID, rider.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	if approvedRider.Status != StatusApproved || approvedRider.RowVersion != 2 || approvedRider.ReviewerUserID == nil || *approvedRider.ReviewerUserID != "7" {
		t.Fatalf("approved rider=%+v", approvedRider)
	}
	if len(provisioner.calls) != 1 || provisioner.calls[0].UserID != 41 || provisioner.calls[0].Role != RoleRider {
		t.Fatalf("provision calls=%+v", provisioner.calls)
	}
	if grant, ok := repo.grant(41, RoleRider); !ok || grant.Status != GrantActive || grant.SourceApplicationID == nil || *grant.SourceApplicationID != rider.ID {
		t.Fatalf("rider grant=%+v found=%v", grant, ok)
	}

	approvedMerchant, err := service.Approve(context.Background(), 7, merchant.ApplicationPublicID, merchant.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	if approvedMerchant.Status != StatusApproved || len(provisioner.calls) != 1 {
		t.Fatalf("merchant=%+v provision calls=%+v", approvedMerchant, provisioner.calls)
	}
	if grant, ok := repo.grant(42, RoleMerchant); !ok || grant.Status != GrantActive || grant.SourceApplicationID == nil || *grant.SourceApplicationID != merchant.ID {
		t.Fatalf("merchant grant=%+v found=%v", grant, ok)
	}
	if len(repo.events) != 4 || repo.events[2].EventType != EventApproved || repo.events[2].ActorUserID != 7 || repo.events[3].EventType != EventApproved {
		t.Fatalf("decision events=%+v", repo.events)
	}
}

func TestRejectRequiresReasonAndDecisionConflictsAreRecognizable(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(repo, &submissionMediaStub{}, fixedClock, &approvalProvisionerStub{})
	pending, err := service.Submit(context.Background(), 41, RoleRider, validRiderInput())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.Reject(context.Background(), 7, pending.ApplicationPublicID, pending.RowVersion, "  "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("blank reason err=%v", err)
	}
	if _, err := service.Reject(context.Background(), 7, pending.ApplicationPublicID, pending.RowVersion, repeatRune('拒', 501)); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("oversized reason err=%v", err)
	}
	if _, err := service.Reject(context.Background(), 7, pending.ApplicationPublicID, pending.RowVersion+1, "资料不清晰"); !errors.Is(err, ErrDecisionConflict) {
		t.Fatalf("stale reject err=%v", err)
	}
	rejected, err := service.Reject(context.Background(), 7, pending.ApplicationPublicID, pending.RowVersion, " 资料不清晰 ")
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != StatusRejected || rejected.ReviewReason != "资料不清晰" || rejected.RowVersion != 2 || rejected.ReviewerUserID == nil || *rejected.ReviewerUserID != "7" {
		t.Fatalf("rejected=%+v", rejected)
	}
	if _, err := service.Reject(context.Background(), 8, pending.ApplicationPublicID, pending.RowVersion, "重复处理"); !errors.Is(err, ErrDecisionConflict) {
		t.Fatalf("repeated reject err=%v", err)
	}
	if _, err := service.Approve(context.Background(), 8, pending.ApplicationPublicID, pending.RowVersion); !errors.Is(err, ErrDecisionConflict) {
		t.Fatalf("approve after reject err=%v", err)
	}
	if repo.hasGrant(41, RoleRider) {
		t.Fatalf("reject wrote grant: %+v", repo.grants)
	}
}

func TestAdminDetailResolvesExactPrivateProofsWhileListDoesNotSign(t *testing.T) {
	repo := newMemoryRepository()
	media := &submissionMediaStub{resolveProofs: []ProofAssetView{{AssetID: proofAssetID1, URL: "https://private.example/signed", ExpiresAt: fixedTime.Add(5 * time.Minute)}}}
	service := NewService(repo, media, fixedClock, &approvalProvisionerStub{})
	pending, err := service.Submit(context.Background(), 41, RoleRider, validRiderInput())
	if err != nil {
		t.Fatal(err)
	}

	list, err := service.ListAdmin(context.Background(), AdminListFilter{Role: RoleRider, Status: StatusPending, Page: 1, PageSize: 20})
	if err != nil || list.Total != 0 {
		// The memory list implementation is intentionally empty; this assertion
		// only proves list never asks the private-media adapter to sign.
		t.Fatalf("list=%+v err=%v", list, err)
	}
	if media.resolveAppID != "" || len(media.resolveIDs) != 0 {
		t.Fatalf("admin list signed private proofs: app=%s ids=%v", media.resolveAppID, media.resolveIDs)
	}

	detail, err := service.GetAdmin(context.Background(), pending.ApplicationPublicID)
	if err != nil {
		t.Fatal(err)
	}
	if media.resolveAppID != pending.ApplicationPublicID || !reflect.DeepEqual(media.resolveIDs, pending.AssetPublicIDs) {
		t.Fatalf("resolve app=%q ids=%v want app=%q ids=%v", media.resolveAppID, media.resolveIDs, pending.ApplicationPublicID, pending.AssetPublicIDs)
	}
	if len(detail.Proofs) != 1 || detail.Proofs[0].URL != "https://private.example/signed" {
		t.Fatalf("detail proofs=%+v", detail.Proofs)
	}
}
