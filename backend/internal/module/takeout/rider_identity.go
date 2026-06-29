package takeout

import (
	"context"
	"errors"
	"strings"
	"time"
)

// RiderIdentityRepository is deliberately limited to shared rider identity.
// It has no order, fulfillment, payment, catalog, or legacy runtime capability.
type RiderIdentityRepository interface {
	CreateRider(context.Context, *Rider) error
	FindRider(context.Context, uint64) (Rider, error)
}

type RiderIdentityService struct {
	repository RiderIdentityRepository
	now        func() time.Time
}

func NewRiderIdentityService(repository RiderIdentityRepository, now func() time.Time) (*RiderIdentityService, error) {
	if repository == nil {
		return nil, ErrInvalidRecord
	}
	if now == nil {
		now = time.Now
	}
	return &RiderIdentityService{repository: repository, now: now}, nil
}

func (s *RiderIdentityService) Find(ctx context.Context, riderUserID uint64) (Rider, error) {
	if riderUserID == 0 {
		return Rider{}, ErrInvalidRecord
	}
	return s.repository.FindRider(ctx, riderUserID)
}

func (s *RiderIdentityService) RequireActive(ctx context.Context, riderUserID uint64) error {
	value, err := s.Find(ctx, riderUserID)
	if err != nil {
		return err
	}
	if value.Status != RiderActive {
		return ErrRiderUnavailable
	}
	return nil
}

func (s *RiderIdentityService) RegisterApproved(
	ctx context.Context,
	riderUserID uint64,
	realName, phone string,
) (Rider, error) {
	if riderUserID == 0 {
		return Rider{}, ErrInvalidRecord
	}
	if existing, err := s.repository.FindRider(ctx, riderUserID); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrNotFound) {
		return Rider{}, err
	}
	now := s.now().UTC()
	value := Rider{
		RiderUserID: riderUserID,
		Status:      RiderActive,
		RealName:    strings.TrimSpace(realName),
		Phone:       strings.TrimSpace(phone),
		RowVersion:  1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if value.RealName == "" || value.Phone == "" {
		return Rider{}, ErrInvalidRecord
	}
	if err := value.ValidateNew(); err != nil {
		return Rider{}, err
	}
	if err := s.repository.CreateRider(ctx, &value); err != nil {
		if existing, findErr := s.repository.FindRider(ctx, riderUserID); findErr == nil {
			return existing, nil
		}
		return Rider{}, err
	}
	return value, nil
}
