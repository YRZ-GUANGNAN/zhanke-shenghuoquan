package main

import (
	"context"
	"errors"

	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/user"
)

type riderIdentityFinder interface {
	Find(context.Context, uint64) (takeout.Rider, error)
}

type deliveryIdentityEligibility struct {
	enabled bool
	riders  riderIdentityFinder
}

func newDeliveryIdentityEligibility(enabled bool, riders riderIdentityFinder) user.IdentityEligibility {
	return &deliveryIdentityEligibility{enabled: enabled, riders: riders}
}

func (e *deliveryIdentityEligibility) RiderEligible(ctx context.Context, userID uint64) (user.IdentityEligibilityResult, error) {
	if e == nil || !e.enabled {
		return user.IdentityEligibilityResult{
			Available: false, Reason: "delivery is disabled", Status: "disabled",
		}, nil
	}
	if e.riders == nil || userID == 0 {
		return user.IdentityEligibilityResult{
			Available: false, Reason: "rider qualification required", Status: "missing",
		}, nil
	}
	value, err := e.riders.Find(ctx, userID)
	switch {
	case err == nil && value.Status == takeout.RiderActive:
		return user.IdentityEligibilityResult{Available: true, Status: string(value.Status)}, nil
	case err == nil:
		return user.IdentityEligibilityResult{
			Available: false, Reason: "rider must be active", Status: string(value.Status),
		}, nil
	case errors.Is(err, takeout.ErrNotFound):
		return user.IdentityEligibilityResult{
			Available: false, Reason: "rider qualification required", Status: "missing",
		}, nil
	default:
		return user.IdentityEligibilityResult{}, err
	}
}

func (e *deliveryIdentityEligibility) MerchantEligible(_ context.Context, userID uint64) (user.IdentityEligibilityResult, error) {
	if e == nil || !e.enabled {
		return user.IdentityEligibilityResult{
			Available: false, Reason: "delivery is disabled", Status: "disabled",
		}, nil
	}
	if userID == 0 {
		return user.IdentityEligibilityResult{
			Available: false, Reason: "authentication required", Status: "missing",
		}, nil
	}
	return user.IdentityEligibilityResult{Available: true, Status: "active"}, nil
}

var _ user.IdentityEligibility = (*deliveryIdentityEligibility)(nil)
