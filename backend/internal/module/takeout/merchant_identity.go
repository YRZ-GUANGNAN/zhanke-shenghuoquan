package takeout

import (
	"context"
	"time"
)

type MerchantIdentityRepository interface {
	FindMerchantByOwnerUserID(context.Context, uint64) (Merchant, error)
}

type MerchantIdentityService struct {
	repository MerchantIdentityRepository
}

func NewMerchantIdentityService(repository MerchantIdentityRepository) (*MerchantIdentityService, error) {
	if repository == nil {
		return nil, ErrInvalidRecord
	}
	return &MerchantIdentityService{repository: repository}, nil
}

func (s *MerchantIdentityService) FindByOwnerUserID(ctx context.Context, ownerUserID uint64) (Merchant, error) {
	if ownerUserID == 0 {
		return Merchant{}, ErrInvalidRecord
	}
	return s.repository.FindMerchantByOwnerUserID(ctx, ownerUserID)
}

func merchantBlocksNewOnboarding(value Merchant) bool {
	switch value.Status {
	case MerchantPendingReview, MerchantActive, MerchantResting, MerchantSuspended:
		return true
	default:
		return false
	}
}

func merchantIsActiveForOwnerActions(value Merchant, now time.Time) bool {
	_ = now
	return value.Status == MerchantActive
}

var _ MerchantIdentityRepository = (*GormRepository)(nil)
