package studentverification

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

var testNow = func() time.Time { return time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC) }

func testULID(t *testing.T) string {
	t.Helper()
	value, err := newULIDAt(testNow(), rand.Reader)
	if err != nil {
		t.Fatalf("generate test ulid: %v", err)
	}
	return value
}

type memoryRepo struct {
	value  *Application
	events []EventType
}

func (r *memoryRepo) snapshot() Application {
	if r.value == nil {
		return Application{}
	}
	clone := *r.value
	clone.Assets = append([]ApplicationAsset(nil), r.value.Assets...)
	return clone
}

func (r *memoryRepo) FindCurrent(_ context.Context, userID uint64) (Application, bool, error) {
	if r.value == nil || r.value.UserID != userID {
		return Application{}, false, nil
	}
	return r.snapshot(), true, nil
}

func (r *memoryRepo) SaveSubmission(_ context.Context, value *Application, assetIDs []string, eventType EventType, expectedRowVersion uint64, commit SubmissionCommit) error {
	if r.value != nil {
		if r.value.RowVersion != expectedRowVersion {
			return ErrVersionConflict
		}
		if r.value.Status != StatusRejected && r.value.Status != StatusCancelled {
			return ErrVersionConflict
		}
		if eventType != EventResubmitted {
			return ErrInvalidInput
		}
	} else if eventType != EventSubmitted {
		return ErrInvalidInput
	}
	if err := commit(context.Background(), nil); err != nil {
		return err
	}
	stored := *value
	stored.Assets = make([]ApplicationAsset, len(assetIDs))
	for index, assetID := range assetIDs {
		stored.Assets[index] = ApplicationAsset{VerificationID: stored.ID, AssetPublicID: assetID, SortOrder: uint16(index)}
	}
	if stored.ID == 0 {
		stored.ID = 1
	}
	*value = stored
	copied := stored
	r.value = &copied
	r.events = append(r.events, eventType)
	return nil
}

func (r *memoryRepo) ListAdmin(_ context.Context, _ AdminListFilter) ([]Application, int64, error) {
	if r.value == nil {
		return nil, 0, nil
	}
	return []Application{r.snapshot()}, 1, nil
}

func (r *memoryRepo) FindDetail(_ context.Context, publicID string) (VerificationDetail, error) {
	if r.value == nil || r.value.ApplicationPublicID != publicID {
		return VerificationDetail{}, ErrApplicationNotFound
	}
	value := r.snapshot()
	return VerificationDetail{VerificationView: toVerificationView(value)}, nil
}

func (r *memoryRepo) decide(input DecisionInput, status Status) (Application, error) {
	if r.value == nil || r.value.ApplicationPublicID != input.ApplicationPublicID {
		return Application{}, ErrApplicationNotFound
	}
	if r.value.Status != StatusPending || r.value.RowVersion != input.ExpectedRowVersion {
		return Application{}, ErrDecisionConflict
	}
	r.value.Status = status
	r.value.ReviewReason = input.Reason
	reviewerID := input.ReviewerAdminID
	r.value.ReviewerAdminID = &reviewerID
	reviewedAt := input.DecidedAt
	r.value.ReviewedAt = &reviewedAt
	r.value.RowVersion++
	r.value.UpdatedAt = input.DecidedAt
	r.events = append(r.events, EventRejected)
	if status == StatusApproved {
		r.events[len(r.events)-1] = EventApproved
	}
	return r.snapshot(), nil
}

func (r *memoryRepo) Approve(_ context.Context, input DecisionInput) (Application, error) {
	return r.decide(input, StatusApproved)
}

func (r *memoryRepo) Reject(_ context.Context, input DecisionInput) (Application, error) {
	if input.Reason == "" {
		return Application{}, ErrInvalidInput
	}
	return r.decide(input, StatusRejected)
}

func (r *memoryRepo) RequestCancel(_ context.Context, userID uint64, expectedRowVersion uint64, requestedAt time.Time) (Application, error) {
	if r.value == nil || r.value.UserID != userID {
		return Application{}, ErrApplicationNotFound
	}
	if r.value.Status != StatusApproved {
		return Application{}, ErrCancelNotAllowed
	}
	if r.value.CancelRequestedAt != nil {
		return r.snapshot(), nil
	}
	if r.value.RowVersion != expectedRowVersion {
		return Application{}, ErrDecisionConflict
	}
	requested := requestedAt
	r.value.CancelRequestedAt = &requested
	r.value.RowVersion++
	r.events = append(r.events, EventCancelRequested)
	return r.snapshot(), nil
}

func (r *memoryRepo) ApproveCancel(_ context.Context, input DecisionInput) (Application, error) {
	if r.value == nil || r.value.ApplicationPublicID != input.ApplicationPublicID {
		return Application{}, ErrApplicationNotFound
	}
	if r.value.Status != StatusApproved || r.value.CancelRequestedAt == nil || r.value.RowVersion != input.ExpectedRowVersion {
		return Application{}, ErrDecisionConflict
	}
	r.value.Status = StatusCancelled
	r.value.ReviewReason = input.Reason
	reviewerID := input.ReviewerAdminID
	r.value.ReviewerAdminID = &reviewerID
	reviewedAt := input.DecidedAt
	r.value.ReviewedAt = &reviewedAt
	r.value.RowVersion++
	r.events = append(r.events, EventCancelApproved)
	return r.snapshot(), nil
}

type stubMedia struct{}

func (stubMedia) Reserve(context.Context, uint64, []string) (MediaReservation, error) {
	return MediaReservation{Token: "m10_" + mustULID(), AssetPublicIDs: []string{mustULID()}, InternalAssetIDs: []uint64{1}}, nil
}

func (stubMedia) ReserveForApplication(_ context.Context, _ uint64, publicID string, assetPublicIDs []string) (MediaReservation, error) {
	return MediaReservation{Token: "m10_" + mustULID(), AssetPublicIDs: append([]string(nil), assetPublicIDs...), InternalAssetIDs: []uint64{1}}, nil
}

func (stubMedia) Commit(context.Context, uint64, MediaReservation, string) error { return nil }
func (stubMedia) EnsureCommitted(context.Context, uint64, MediaReservation, string) error {
	return nil
}
func (stubMedia) EnsureCommittedInTransaction(context.Context, *gorm.DB, uint64, MediaReservation, string) error {
	return nil
}
func (stubMedia) Cancel(context.Context, uint64, MediaReservation) error { return nil }
func (stubMedia) ResolveAdminProofs(_ context.Context, _ string, assetPublicIDs []string) ([]ProofAssetView, error) {
	proofs := make([]ProofAssetView, len(assetPublicIDs))
	for index, assetID := range assetPublicIDs {
		proofs[index] = ProofAssetView{AssetID: assetID, URL: "https://example/proof", ExpiresAt: testNow()}
	}
	return proofs, nil
}

func mustULID() string {
	value, err := newULIDAt(time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC), rand.Reader)
	if err != nil {
		panic(err)
	}
	return value
}

func newServiceForTest() (*Service, *memoryRepo) {
	repo := &memoryRepo{}
	return NewService(repo, stubMedia{}, testNow), repo
}

func TestSubmitCreatesPendingVerification(t *testing.T) {
	service, repo := newServiceForTest()
	view, err := service.Submit(context.Background(), 7, SubmitInput{
		RealName: "张三", StudentNo: "20260001", College: "计算机学院", AssetPublicIDs: []string{mustULID()},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if view.Status != StatusPending || view.RealName != "张三" || view.StudentNo != "20260001" {
		t.Fatalf("unexpected view: %+v", view)
	}
	if len(repo.events) != 1 || repo.events[0] != EventSubmitted {
		t.Fatalf("unexpected events: %v", repo.events)
	}
}

func TestSubmitWhilePendingIsRejected(t *testing.T) {
	service, _ := newServiceForTest()
	input := SubmitInput{RealName: "张三", StudentNo: "20260001", AssetPublicIDs: []string{mustULID()}}
	if _, err := service.Submit(context.Background(), 7, input); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if _, err := service.Submit(context.Background(), 7, input); !errors.Is(err, ErrApplicationPending) {
		t.Fatalf("expected ErrApplicationPending, got %v", err)
	}
}

func TestApproveLocksResubmission(t *testing.T) {
	service, _ := newServiceForTest()
	view, err := service.Submit(context.Background(), 7, SubmitInput{
		RealName: "张三", StudentNo: "20260001", AssetPublicIDs: []string{mustULID()},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	approved, err := service.Approve(context.Background(), 99, view.ApplicationPublicID, view.RowVersion)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != StatusApproved {
		t.Fatalf("expected approved, got %s", approved.Status)
	}
	if _, err := service.Submit(context.Background(), 7, SubmitInput{
		RealName: "张三", StudentNo: "20260001", AssetPublicIDs: []string{mustULID()},
	}); !errors.Is(err, ErrApplicationApproved) {
		t.Fatalf("expected ErrApplicationApproved, got %v", err)
	}
}

func TestRejectRequiresReasonAndAllowsResubmit(t *testing.T) {
	service, repo := newServiceForTest()
	view, err := service.Submit(context.Background(), 7, SubmitInput{
		RealName: "张三", StudentNo: "20260001", AssetPublicIDs: []string{mustULID()},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := service.Reject(context.Background(), 99, view.ApplicationPublicID, view.RowVersion, " "); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input for empty reason, got %v", err)
	}
	rejected, err := service.Reject(context.Background(), 99, view.ApplicationPublicID, view.RowVersion, "学号不清晰")
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if rejected.Status != StatusRejected || rejected.ReviewReason != "学号不清晰" {
		t.Fatalf("unexpected rejected view: %+v", rejected)
	}
	resubmitted, err := service.Submit(context.Background(), 7, SubmitInput{
		RealName: "张三", StudentNo: "20260001", College: "计算机学院",
		AssetPublicIDs: []string{mustULID()}, ExpectedRowVersion: rejected.RowVersion,
	})
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if resubmitted.Status != StatusPending || resubmitted.ReviewReason != "" {
		t.Fatalf("unexpected resubmitted view: %+v", resubmitted)
	}
	if repo.events[len(repo.events)-1] != EventResubmitted {
		t.Fatalf("expected resubmitted event, got %v", repo.events)
	}
}

func TestCancelRequestFlow(t *testing.T) {
	service, repo := newServiceForTest()
	view, err := service.Submit(context.Background(), 7, SubmitInput{
		RealName: "张三", StudentNo: "20260001", AssetPublicIDs: []string{mustULID()},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := service.RequestCancel(context.Background(), 7, view.RowVersion); !errors.Is(err, ErrCancelNotAllowed) {
		t.Fatalf("expected ErrCancelNotAllowed on pending, got %v", err)
	}
	approved, err := service.Approve(context.Background(), 99, view.ApplicationPublicID, view.RowVersion)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	requested, err := service.RequestCancel(context.Background(), 7, approved.RowVersion)
	if err != nil {
		t.Fatalf("request cancel: %v", err)
	}
	if !requested.CancelRequested {
		t.Fatal("expected cancelRequested=true")
	}
	repeated, err := service.RequestCancel(context.Background(), 7, requested.RowVersion+100)
	if err != nil {
		t.Fatalf("repeat request cancel should be idempotent: %v", err)
	}
	if repeated.RowVersion != requested.RowVersion {
		t.Fatalf("idempotent repeat must not bump version, got %d -> %d", requested.RowVersion, repeated.RowVersion)
	}
	cancelled, err := service.ApproveCancel(context.Background(), 99, requested.ApplicationPublicID, requested.RowVersion, "已解除绑定")
	if err != nil {
		t.Fatalf("approve cancel: %v", err)
	}
	if cancelled.Status != StatusCancelled || cancelled.CancelRequested != true {
		t.Fatalf("unexpected cancelled view: %+v", cancelled)
	}
	resubmitted, err := service.Submit(context.Background(), 7, SubmitInput{
		RealName: "张三", StudentNo: "20260001", AssetPublicIDs: []string{mustULID()}, ExpectedRowVersion: cancelled.RowVersion,
	})
	if err != nil {
		t.Fatalf("resubmit after cancel: %v", err)
	}
	if resubmitted.Status != StatusPending {
		t.Fatalf("expected pending after resubmit, got %s", resubmitted.Status)
	}
	if repo.events[len(repo.events)-1] != EventResubmitted {
		t.Fatalf("expected final resubmitted event, got %v", repo.events)
	}
}

// 后台管理员账号与小程序用户是两个独立 id 空间；同号不再构成“自审”。
func TestDecisionUsesIndependentAdminIdentity(t *testing.T) {
	service, _ := newServiceForTest()
	view, err := service.Submit(context.Background(), 7, SubmitInput{
		RealName: "张三", StudentNo: "20260001", AssetPublicIDs: []string{mustULID()},
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	approved, err := service.Approve(context.Background(), 7, view.ApplicationPublicID, view.RowVersion)
	if err != nil {
		t.Fatalf("admin decision rejected: %v", err)
	}
	if approved.Status != StatusApproved {
		t.Fatalf("approved=%+v", approved)
	}
}

func TestSubmitValidation(t *testing.T) {
	service, _ := newServiceForTest()
	cases := map[string]SubmitInput{
		"missing name":    {StudentNo: "2026", AssetPublicIDs: []string{mustULID()}},
		"missing number":  {RealName: "张三", AssetPublicIDs: []string{mustULID()}},
		"no proof":        {RealName: "张三", StudentNo: "2026"},
		"invalid asset":   {RealName: "张三", StudentNo: "2026", AssetPublicIDs: []string{"not-a-ulid"}},
		"duplicate asset": {RealName: "张三", StudentNo: "2026", AssetPublicIDs: []string{mustULID(), mustULID()}},
	}
	for name, input := range cases {
		if name == "duplicate asset" {
			assetID := mustULID()
			input.AssetPublicIDs = []string{assetID, assetID}
		}
		if _, err := service.Submit(context.Background(), 7, input); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s: expected invalid input, got %v", name, err)
		}
	}
}
