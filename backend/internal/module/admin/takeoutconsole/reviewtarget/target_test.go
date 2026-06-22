package reviewtarget

import (
	"context"
	"errors"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/takeout"
)

type merchantStub struct {
	value    takeout.Merchant
	findErr  error
	applyErr error
	approved int
	rejected int
}

func (s *merchantStub) FindMerchantByInternalID(context.Context, uint64) (takeout.Merchant, error) {
	return s.value, s.findErr
}
func (s *merchantStub) Approve(context.Context, string, uint64) (takeout.Merchant, error) {
	s.approved++
	return s.value, s.applyErr
}
func (s *merchantStub) Reject(context.Context, string, uint64) (takeout.Merchant, error) {
	s.rejected++
	return s.value, s.applyErr
}

type catalogStub struct {
	value   takeout.Dish
	findErr error
	putErr  error
	put     int
}

func (s *catalogStub) FindDishByInternalID(context.Context, uint64) (takeout.Dish, error) {
	return s.value, s.findErr
}
func (s *catalogStub) PutOnShelf(context.Context, string, string, takeout.DishStatus, uint64) (takeout.Dish, error) {
	s.put++
	return s.value, s.putErr
}

func TestMerchantTargetMapsReviewDecisionsToExistingCASService(t *testing.T) {
	for _, test := range []struct {
		name     string
		status   review.Status
		approved int
		rejected int
	}{
		{name: "reviewing is a guarded no-op", status: review.StatusReviewing},
		{name: "approved", status: review.StatusApproved, approved: 1},
		{name: "rejected", status: review.StatusRejected, rejected: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &merchantStub{value: takeout.Merchant{
				ID: 7, MerchantPublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
				Status: takeout.MerchantPendingReview, RowVersion: 3,
			}}
			target := MerchantTarget{service: service}
			if err := target.SetReviewStatus(context.Background(), 7, test.status); err != nil {
				t.Fatal(err)
			}
			if service.approved != test.approved || service.rejected != test.rejected {
				t.Fatalf("approve=%d reject=%d", service.approved, service.rejected)
			}
		})
	}
}

func TestDishTargetApprovalUsesCASServiceAndRejectionKeepsDraft(t *testing.T) {
	service := &catalogStub{value: takeout.Dish{
		ID: 8, DishPublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAW",
		MerchantPublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		Status:           takeout.DishDraft, RowVersion: 4,
	}}
	target := DishTarget{service: service}
	if err := target.SetReviewStatus(context.Background(), 8, review.StatusRejected); err != nil {
		t.Fatal(err)
	}
	if service.put != 0 {
		t.Fatal("rejected dish must remain draft without a transition")
	}
	if err := target.SetReviewStatus(context.Background(), 8, review.StatusApproved); err != nil {
		t.Fatal(err)
	}
	if service.put != 1 {
		t.Fatalf("put on shelf calls=%d", service.put)
	}
}

func TestTargetsFailClosed(t *testing.T) {
	merchant := &merchantStub{value: takeout.Merchant{Status: takeout.MerchantActive}}
	if err := (MerchantTarget{service: merchant}).SetReviewStatus(context.Background(), 1, review.StatusReviewing); !errors.Is(err, takeout.ErrInvalidTransition) {
		t.Fatalf("reviewing active merchant error=%v", err)
	}
	dish := &catalogStub{value: takeout.Dish{Status: takeout.DishOnShelf}}
	if err := (DishTarget{service: dish}).SetReviewStatus(context.Background(), 1, review.StatusRejected); !errors.Is(err, takeout.ErrInvalidTransition) {
		t.Fatalf("reject on-shelf dish error=%v", err)
	}
	if err := (DishTarget{service: dish}).SetReviewStatus(context.Background(), 1, review.StatusSubmitted); !errors.Is(err, ErrUnsupportedReviewStatus) {
		t.Fatalf("unsupported status error=%v", err)
	}
}

func TestTargetErrorsRemainObservable(t *testing.T) {
	merchant := &merchantStub{
		value:    takeout.Merchant{Status: takeout.MerchantPendingReview},
		applyErr: takeout.ErrConcurrentUpdate,
	}
	err := (MerchantTarget{service: merchant}).SetReviewStatus(context.Background(), 1, review.StatusApproved)
	if !errors.Is(err, takeout.ErrConcurrentUpdate) {
		t.Fatalf("CAS error was hidden: %v", err)
	}
}
