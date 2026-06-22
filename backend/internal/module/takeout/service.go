package takeout

import (
	"context"
	"strings"
	"time"
)

type MerchantService struct {
	repository MerchantRepository
	now        func() time.Time
}

func NewMerchantService(repository MerchantRepository, now func() time.Time) (*MerchantService, error) {
	if repository == nil {
		return nil, ErrInvalidRecord
	}
	if now == nil {
		now = time.Now
	}
	return &MerchantService{repository: repository, now: now}, nil
}

type CreateMerchantInput struct {
	OwnerUserID       uint64
	Name              string
	ContactPhone      string
	Location          string
	BusinessHours     string
	LogoAssetPublicID *string
}

func (s *MerchantService) CreateMerchant(ctx context.Context, input CreateMerchantInput) (Merchant, error) {
	publicID, err := NewULID()
	if err != nil {
		return Merchant{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.ContactPhone = strings.TrimSpace(input.ContactPhone)
	input.Location = strings.TrimSpace(input.Location)
	input.BusinessHours = strings.TrimSpace(input.BusinessHours)
	if input.LogoAssetPublicID != nil {
		logo := strings.TrimSpace(*input.LogoAssetPublicID)
		input.LogoAssetPublicID = &logo
	}
	now := s.now().UTC()
	value := Merchant{
		MerchantPublicID: publicID, OwnerUserID: input.OwnerUserID, Name: input.Name,
		ContactPhone: input.ContactPhone, Location: input.Location, BusinessHours: input.BusinessHours,
		LogoAssetPublicID: input.LogoAssetPublicID, Status: MerchantPendingReview,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := value.ValidateNew(); err != nil {
		return Merchant{}, err
	}
	if err := s.repository.CreateMerchant(ctx, &value); err != nil {
		return Merchant{}, err
	}
	return value, nil
}

func (s *MerchantService) Approve(ctx context.Context, publicID string, expectedRowVersion uint64) (Merchant, error) {
	return s.transition(ctx, publicID, nil, expectedRowVersion, MerchantPendingReview, MerchantActive)
}

func (s *MerchantService) Reject(ctx context.Context, publicID string, expectedRowVersion uint64) (Merchant, error) {
	return s.transition(ctx, publicID, nil, expectedRowVersion, MerchantPendingReview, MerchantRejected)
}

func (s *MerchantService) OpenForBusiness(ctx context.Context, publicID string, ownerUserID, expectedRowVersion uint64) (Merchant, error) {
	return s.transition(ctx, publicID, &ownerUserID, expectedRowVersion, MerchantResting, MerchantActive)
}

func (s *MerchantService) Rest(ctx context.Context, publicID string, ownerUserID, expectedRowVersion uint64) (Merchant, error) {
	return s.transition(ctx, publicID, &ownerUserID, expectedRowVersion, MerchantActive, MerchantResting)
}

func (s *MerchantService) Suspend(ctx context.Context, publicID string, expectedStatus MerchantStatus, expectedRowVersion uint64) (Merchant, error) {
	return s.transition(ctx, publicID, nil, expectedRowVersion, expectedStatus, MerchantSuspended)
}

func (s *MerchantService) Unsuspend(ctx context.Context, publicID string, expectedRowVersion uint64) (Merchant, error) {
	return s.transition(ctx, publicID, nil, expectedRowVersion, MerchantSuspended, MerchantActive)
}

func (s *MerchantService) FindMerchant(ctx context.Context, publicID string) (Merchant, error) {
	if !ValidULID(publicID) {
		return Merchant{}, ErrInvalidULID
	}
	return s.repository.FindMerchant(ctx, publicID)
}

func (s *MerchantService) transition(
	ctx context.Context,
	publicID string,
	ownerUserID *uint64,
	expectedRowVersion uint64,
	from MerchantStatus,
	to MerchantStatus,
) (Merchant, error) {
	if !ValidULID(publicID) {
		return Merchant{}, ErrInvalidULID
	}
	if expectedRowVersion == 0 || !from.Valid() || !to.Valid() {
		return Merchant{}, ErrInvalidRecord
	}
	if !CanTransitionMerchant(from, to) {
		return Merchant{}, ErrInvalidTransition
	}
	if ownerUserID != nil && *ownerUserID == 0 {
		return Merchant{}, ErrInvalidRecord
	}
	return s.repository.TransitionMerchantStatus(ctx, publicID, ownerUserID, expectedRowVersion, from, to, s.now().UTC())
}
