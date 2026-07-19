package main

import (
	"context"
	"errors"

	"campus-circle/backend/internal/module/identityapplication"
	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/user"
)

type riderIdentityFinder interface {
	Find(context.Context, uint64) (takeout.Rider, error)
}

type deliveryIdentityEligibility struct {
	enabled bool
	riders  riderIdentityFinder
	grants  identityapplication.GrantFinder
}

func newDeliveryIdentityEligibility(enabled bool, riders riderIdentityFinder, grants identityapplication.GrantFinder) user.IdentityEligibility {
	return &deliveryIdentityEligibility{enabled: enabled, riders: riders, grants: grants}
}

func (e *deliveryIdentityEligibility) RiderEligible(ctx context.Context, userID uint64) (user.IdentityEligibilityResult, error) {
	if e == nil || !e.enabled {
		return user.IdentityEligibilityResult{
			Available: false, Reason: "delivery is disabled", Status: "disabled",
		}, nil
	}
	grant, available, err := e.activeGrant(ctx, userID, identityapplication.RoleRider)
	if err != nil || !available {
		return grant, err
	}
	if e.riders == nil {
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

func (e *deliveryIdentityEligibility) MerchantEligible(ctx context.Context, userID uint64) (user.IdentityEligibilityResult, error) {
	if e == nil || !e.enabled {
		return user.IdentityEligibilityResult{
			Available: false, Reason: "delivery is disabled", Status: "disabled",
		}, nil
	}
	result, _, err := e.activeGrant(ctx, userID, identityapplication.RoleMerchant)
	return result, err
}

func (e *deliveryIdentityEligibility) activeGrant(ctx context.Context, userID uint64, role identityapplication.Role) (user.IdentityEligibilityResult, bool, error) {
	missing := user.IdentityEligibilityResult{
		Available: false, Reason: string(role) + " approval required", Status: "missing",
	}
	if userID == 0 || e.grants == nil {
		return missing, false, nil
	}
	grant, found, err := e.grants.FindGrant(ctx, userID, role)
	if err != nil {
		return user.IdentityEligibilityResult{}, false, err
	}
	if !found {
		return missing, false, nil
	}
	switch grant.Status {
	case identityapplication.GrantActive:
		return user.IdentityEligibilityResult{Available: true, Status: string(grant.Status)}, true, nil
	case identityapplication.GrantSuspended:
		return user.IdentityEligibilityResult{
			Available: false, Reason: string(role) + " approval is suspended", Status: string(grant.Status),
		}, false, nil
	default:
		return user.IdentityEligibilityResult{
			Available: false, Reason: string(role) + " approval is unavailable", Status: string(grant.Status),
		}, false, nil
	}
}

var _ user.IdentityEligibility = (*deliveryIdentityEligibility)(nil)
