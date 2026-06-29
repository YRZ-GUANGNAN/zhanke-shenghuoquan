package deliveryrider

import (
	"context"
	"errors"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/takeout"
)

type IdentityAdapter struct {
	service *takeout.RiderIdentityService
}

func NewIdentityAdapter(service *takeout.RiderIdentityService) (*IdentityAdapter, error) {
	if service == nil {
		return nil, delivery.ErrInvalidRecord
	}
	return &IdentityAdapter{service: service}, nil
}

func (a *IdentityAdapter) RequireActive(ctx context.Context, riderUserID uint64) error {
	err := a.service.RequireActive(ctx, riderUserID)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, takeout.ErrNotFound):
		return delivery.ErrRiderNotRegistered
	case errors.Is(err, takeout.ErrRiderUnavailable):
		return delivery.ErrRiderUnavailable
	default:
		return err
	}
}

func (a *IdentityAdapter) Find(ctx context.Context, riderUserID uint64) (takeout.Rider, error) {
	return a.service.Find(ctx, riderUserID)
}

func (a *IdentityAdapter) RegisterApproved(
	ctx context.Context,
	riderUserID uint64,
	realName, phone string,
) (takeout.Rider, error) {
	return a.service.RegisterApproved(ctx, riderUserID, realName, phone)
}

var _ delivery.RiderEligibility = (*IdentityAdapter)(nil)
