package main

import (
	"context"
	"errors"
	"testing"

	"campus-circle/backend/internal/module/takeout"
)

type stubRiderIdentityFinder struct {
	value takeout.Rider
	err   error
}

func (s stubRiderIdentityFinder) Find(context.Context, uint64) (takeout.Rider, error) {
	return s.value, s.err
}

func TestDeliveryIdentityEligibilityFailsClosedWhenDisabled(t *testing.T) {
	eligibility := newDeliveryIdentityEligibility(false, nil)
	for name, check := range map[string]func(context.Context, uint64) (bool, string, error){
		"rider": func(ctx context.Context, id uint64) (bool, string, error) {
			value, err := eligibility.RiderEligible(ctx, id)
			return value.Available, value.Status, err
		},
		"merchant": func(ctx context.Context, id uint64) (bool, string, error) {
			value, err := eligibility.MerchantEligible(ctx, id)
			return value.Available, value.Status, err
		},
	} {
		t.Run(name, func(t *testing.T) {
			available, status, err := check(context.Background(), 42)
			if err != nil || available || status != "disabled" {
				t.Fatalf("available=%v status=%q err=%v", available, status, err)
			}
		})
	}
}

func TestDeliveryIdentityEligibilityUsesCurrentDeliveryPolicy(t *testing.T) {
	merchant := newDeliveryIdentityEligibility(true, nil)
	merchantResult, err := merchant.MerchantEligible(context.Background(), 42)
	if err != nil || !merchantResult.Available || merchantResult.Status != "active" {
		t.Fatalf("merchant=%+v err=%v", merchantResult, err)
	}

	for name, finder := range map[string]stubRiderIdentityFinder{
		"active":  {value: takeout.Rider{Status: takeout.RiderActive}},
		"resting": {value: takeout.Rider{Status: takeout.RiderResting}},
		"missing": {err: takeout.ErrNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			result, findErr := newDeliveryIdentityEligibility(true, finder).RiderEligible(context.Background(), 42)
			if findErr != nil {
				t.Fatal(findErr)
			}
			if name == "active" && !result.Available {
				t.Fatalf("active rider unavailable: %+v", result)
			}
			if name != "active" && result.Available {
				t.Fatalf("%s rider available: %+v", name, result)
			}
		})
	}
}

func TestDeliveryIdentityEligibilityPropagatesRepositoryFailure(t *testing.T) {
	want := errors.New("database unavailable")
	_, err := newDeliveryIdentityEligibility(true, stubRiderIdentityFinder{err: want}).RiderEligible(context.Background(), 42)
	if !errors.Is(err, want) {
		t.Fatalf("err=%v want=%v", err, want)
	}
}
