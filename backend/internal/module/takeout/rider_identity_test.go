package takeout

import (
	"context"
	"errors"
	"testing"
	"time"
)

type riderIdentityRepositoryStub struct {
	value   Rider
	findErr error
	created *Rider
}

func (r *riderIdentityRepositoryStub) CreateRider(_ context.Context, value *Rider) error {
	copy := *value
	r.created = &copy
	r.value = copy
	r.findErr = nil
	return nil
}

func (r *riderIdentityRepositoryStub) FindRider(context.Context, uint64) (Rider, error) {
	return r.value, r.findErr
}

func TestRiderIdentityServiceHasNoOrderDependency(t *testing.T) {
	now := time.Date(2026, 6, 30, 1, 0, 0, 0, time.UTC)
	repository := &riderIdentityRepositoryStub{findErr: ErrNotFound}
	service, err := NewRiderIdentityService(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	value, err := service.RegisterApproved(context.Background(), 701, " Rider ", " 13800000000 ")
	if err != nil {
		t.Fatal(err)
	}
	if repository.created == nil || value.Status != RiderActive || value.RiderUserID != 701 ||
		value.RealName != "Rider" || value.Phone != "13800000000" || value.RowVersion != 1 {
		t.Fatalf("registered rider=%#v", value)
	}
	if err := service.RequireActive(context.Background(), 701); err != nil {
		t.Fatal(err)
	}
	repository.value.Status = RiderResting
	if err := service.RequireActive(context.Background(), 701); !errors.Is(err, ErrRiderUnavailable) {
		t.Fatalf("resting rider returned %v", err)
	}
}

func TestRiderIdentityRegistrationIsIdempotentAndValidated(t *testing.T) {
	existing := Rider{RiderUserID: 702, Status: RiderSuspended, RowVersion: 3}
	repository := &riderIdentityRepositoryStub{value: existing}
	service, _ := NewRiderIdentityService(repository, time.Now)
	value, err := service.RegisterApproved(context.Background(), 702, "Different", "13900000000")
	if err != nil || value.Status != RiderSuspended || repository.created != nil {
		t.Fatalf("idempotent registration=%#v err=%v", value, err)
	}
	repository.findErr = ErrNotFound
	if _, err := service.RegisterApproved(context.Background(), 703, "", ""); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("empty approved identity returned %v", err)
	}
}
