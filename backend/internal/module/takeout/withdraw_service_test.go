package takeout

import (
	"context"
	"errors"
	"testing"
	"time"
)

type withdrawRepositoryStub struct {
	value WithdrawRequest
}

func (r *withdrawRepositoryStub) CreateWithdrawRequest(_ context.Context, value *WithdrawRequest) error {
	value.ID = 1
	r.value = *value
	return nil
}

func (r *withdrawRepositoryStub) FindWithdrawByID(_ context.Context, publicID string) (WithdrawRequest, error) {
	if r.value.WithdrawPublicID != publicID {
		return WithdrawRequest{}, ErrNotFound
	}
	return r.value, nil
}

func (r *withdrawRepositoryStub) ListWithdrawRequests(_ context.Context, status WithdrawStatus, _, _ int) ([]WithdrawRequest, int64, error) {
	if r.value.ID == 0 || (status != "" && r.value.Status != status) {
		return nil, 0, nil
	}
	return []WithdrawRequest{r.value}, 1, nil
}

func (r *withdrawRepositoryStub) MarkWithdrawPaid(_ context.Context, publicID string, expectedRowVersion uint64, at time.Time) (WithdrawRequest, error) {
	if r.value.WithdrawPublicID != publicID {
		return WithdrawRequest{}, ErrNotFound
	}
	if r.value.Status == WithdrawPaid {
		return r.value, nil
	}
	if expectedRowVersion != r.value.RowVersion {
		return WithdrawRequest{}, ErrConcurrentUpdate
	}
	r.value.Status, r.value.PaidAt, r.value.UpdatedAt = WithdrawPaid, &at, at
	r.value.RowVersion++
	return r.value, nil
}

func (r *withdrawRepositoryStub) RejectWithdraw(_ context.Context, publicID string, expectedRowVersion uint64, note string, at time.Time) (WithdrawRequest, error) {
	if r.value.WithdrawPublicID != publicID {
		return WithdrawRequest{}, ErrNotFound
	}
	if expectedRowVersion != r.value.RowVersion {
		return WithdrawRequest{}, ErrConcurrentUpdate
	}
	r.value.Status, r.value.Note, r.value.UpdatedAt = WithdrawRejected, note, at
	r.value.RowVersion++
	return r.value, nil
}

func TestWithdrawServiceValidationCASAndQueries(t *testing.T) {
	now := time.Date(2026, 6, 22, 20, 0, 0, 0, time.UTC)
	repository := &withdrawRepositoryStub{}
	service, err := NewWithdrawService(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateWithdrawRequest(context.Background(), 0, 100); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("zero rider accepted: %v", err)
	}
	value, err := service.CreateWithdrawRequest(context.Background(), 880001, 1234)
	if err != nil || value.Status != WithdrawPending || value.AmountCents != 1234 || value.RowVersion != 1 {
		t.Fatalf("create=%#v err=%v", value, err)
	}
	if _, err := service.MarkWithdrawPaid(context.Background(), value.WithdrawPublicID, 2); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale CAS returned %v", err)
	}
	value, err = service.MarkWithdrawPaid(context.Background(), value.WithdrawPublicID, 1)
	if err != nil || value.Status != WithdrawPaid || value.PaidAt == nil || value.RowVersion != 2 {
		t.Fatalf("paid=%#v err=%v", value, err)
	}
	repeated, err := service.MarkWithdrawPaid(context.Background(), value.WithdrawPublicID, 1)
	if err != nil || repeated.RowVersion != 2 {
		t.Fatalf("repeated paid=%#v err=%v", repeated, err)
	}
	values, total, err := service.ListWithdrawRequests(context.Background(), WithdrawPaid, 0, 999)
	if err != nil || total != 1 || len(values) != 1 {
		t.Fatalf("list=%#v total=%d err=%v", values, total, err)
	}
	if _, _, err := service.ListWithdrawRequests(context.Background(), WithdrawStatus("invalid"), 1, 20); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("invalid status returned %v", err)
	}
}

func TestWithdrawRejectRequiresReason(t *testing.T) {
	now := time.Date(2026, 6, 22, 20, 0, 0, 0, time.UTC)
	repository := &withdrawRepositoryStub{}
	service, _ := NewWithdrawService(repository, func() time.Time { return now })
	value, _ := service.CreateWithdrawRequest(context.Background(), 880002, 500)
	if _, err := service.RejectWithdraw(context.Background(), value.WithdrawPublicID, 1, " "); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("empty reason returned %v", err)
	}
	value, err := service.RejectWithdraw(context.Background(), value.WithdrawPublicID, 1, "identity mismatch")
	if err != nil || value.Status != WithdrawRejected || value.Note != "identity mismatch" {
		t.Fatalf("reject=%#v err=%v", value, err)
	}
}
