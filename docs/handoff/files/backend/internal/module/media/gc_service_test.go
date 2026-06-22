package media

import (
	"context"
	"errors"
	"testing"
	"time"
)

type gcRepositoryStub struct {
	delivery GCDelivery
	found    bool
	execErr  error
	failTo   GCStatus
	deleted  int
}

func (r *gcRepositoryStub) PlanGCJob(context.Context, string, GCObjectStage, time.Time, GCPolicy) (GCJob, bool, error) {
	return GCJob{}, false, nil
}
func (r *gcRepositoryStub) ClaimNextGCJob(context.Context, time.Time, time.Duration) (GCDelivery, bool, error) {
	if !r.found {
		return GCDelivery{}, false, nil
	}
	r.found = false
	return r.delivery, true, nil
}
func (r *gcRepositoryStub) ExecuteGCDeletion(_ context.Context, _ GCDelivery, _ time.Time, _ GCPolicy, deleteFn func(context.Context, string, string) (GCDeleteResult, error)) (GCJob, error) {
	if r.execErr != nil {
		return GCJob{}, r.execErr
	}
	if _, err := deleteFn(context.Background(), r.delivery.Job.ObjectKey, r.delivery.Job.ObjectVersionID); err != nil {
		return GCJob{}, err
	}
	return r.delivery.Job, nil
}
func (r *gcRepositoryStub) FailGCJob(context.Context, GCDelivery, string, time.Time, uint32, bool, time.Time) (GCStatus, error) {
	return r.failTo, nil
}
func (r *gcRepositoryStub) PurgeTerminalAuxiliary(context.Context, time.Time, time.Time, time.Duration, int) (GCAuxiliaryPurgeResult, error) {
	return GCAuxiliaryPurgeResult{}, nil
}

type gcStoreStub struct {
	key, version string
	err          error
	calls        int
}

func (s *gcStoreStub) DeleteVersion(_ context.Context, key, version string) (GCDeleteResult, error) {
	s.calls++
	s.key, s.version = key, version
	return GCDeleteResult{}, s.err
}

func testGCPolicy() GCPolicy {
	return GCPolicy{
		StagingRetention: time.Hour, RejectedRetention: time.Hour, ApprovedRetention: 24 * time.Hour,
		AuxiliaryRetention: 24 * time.Hour, LeaseTTL: time.Minute, RetryDelay: time.Minute, MaxAttempts: 3,
	}
}

func TestMediaGCServiceUsesExactVersion(t *testing.T) {
	jobID, _ := NewULID()
	repository := &gcRepositoryStub{found: true, delivery: GCDelivery{
		Job: GCJob{JobID: jobID, ObjectKey: "final/a.webp", ObjectVersionID: "fixed-v1", Status: GCProcessing}, LeaseToken: "lease",
	}}
	store := &gcStoreStub{}
	service, err := NewMediaGCService(repository, store, testGCPolicy(), func() time.Time { return time.Unix(100, 0).UTC() })
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.RunBatch(context.Background(), 1)
	if err != nil || result.Succeeded != 1 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if store.calls != 1 || store.key != "final/a.webp" || store.version != "fixed-v1" {
		t.Fatalf("store call key=%q version=%q calls=%d", store.key, store.version, store.calls)
	}
}

func TestMediaGCServiceFailClosedAndRetries(t *testing.T) {
	jobID, _ := NewULID()
	repository := &gcRepositoryStub{
		found: true, execErr: ErrGCReferenceExists, failTo: GCRetryWait,
		delivery: GCDelivery{Job: GCJob{JobID: jobID, Status: GCProcessing, AttemptCount: 1}, LeaseToken: "lease"},
	}
	service, _ := NewMediaGCService(repository, &gcStoreStub{}, testGCPolicy(), time.Now)
	result, err := service.RunBatch(context.Background(), 1)
	if err != nil || result.Retried != 1 || result.Succeeded != 0 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestTencentGCStoreRejectsUnversionedDelete(t *testing.T) {
	store := &TencentGCStore{}
	if _, err := store.DeleteVersion(context.Background(), "final/a.webp", ""); !errors.Is(err, ErrGCFixedIdentity) {
		t.Fatalf("unversioned delete returned %v", err)
	}
}
