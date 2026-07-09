package main

import (
	"context"
	"errors"

	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/user"
)

type takeoutIdentityEligibility struct {
	riders    *takeout.RiderIdentityService
	merchants *takeout.MerchantIdentityService
}

func newTakeoutIdentityEligibility(
	riders *takeout.RiderIdentityService,
	merchants *takeout.MerchantIdentityService,
) user.IdentityEligibility {
	return &takeoutIdentityEligibility{riders: riders, merchants: merchants}
}

func (e *takeoutIdentityEligibility) RiderEligible(ctx context.Context, userID uint64) (user.IdentityEligibilityResult, error) {
	if e == nil || e.riders == nil || userID == 0 {
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

func (e *takeoutIdentityEligibility) MerchantEligible(ctx context.Context, userID uint64) (user.IdentityEligibilityResult, error) {
	if e == nil || e.merchants == nil || userID == 0 {
		return user.IdentityEligibilityResult{
			Available: false, Reason: "merchant qualification required", Status: "missing",
		}, nil
	}
	value, err := e.merchants.FindByOwnerUserID(ctx, userID)
	switch {
	case err == nil && value.Status == takeout.MerchantActive:
		return user.IdentityEligibilityResult{Available: true, Status: string(value.Status)}, nil
	case err == nil:
		return user.IdentityEligibilityResult{
			Available: false, Reason: "merchant must be active", Status: string(value.Status),
		}, nil
	case errors.Is(err, takeout.ErrNotFound):
		return user.IdentityEligibilityResult{
			Available: false, Reason: "merchant qualification required", Status: "missing",
		}, nil
	default:
		return user.IdentityEligibilityResult{}, err
	}
}

var _ user.IdentityEligibility = (*takeoutIdentityEligibility)(nil)
