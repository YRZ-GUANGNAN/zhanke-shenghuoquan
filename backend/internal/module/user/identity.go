package user

import (
	"context"
	"errors"
)

type Identity string

const (
	IdentityStudent  Identity = "student"
	IdentityRider    Identity = "rider"
	IdentityMerchant Identity = "merchant"
)

var (
	ErrInvalidIdentity     = errors.New("invalid identity")
	ErrIdentityUnavailable = errors.New("identity is unavailable")
)

func (i Identity) Valid() bool {
	switch i {
	case IdentityStudent, IdentityRider, IdentityMerchant:
		return true
	default:
		return false
	}
}

func normalizeIdentity(value Identity) Identity {
	if value.Valid() {
		return value
	}
	return IdentityStudent
}

type IdentityAvailability struct {
	Identity  Identity `json:"identity"`
	Available bool     `json:"available"`
	Reason    string   `json:"reason,omitempty"`
	Status    string   `json:"status,omitempty"`
}

type IdentitySummary struct {
	CurrentIdentity Identity               `json:"currentIdentity"`
	Identities      []IdentityAvailability `json:"identities"`
}

type IdentityEligibilityResult struct {
	Available bool
	Reason    string
	Status    string
}

type IdentityEligibility interface {
	RiderEligible(context.Context, uint64) (IdentityEligibilityResult, error)
	MerchantEligible(context.Context, uint64) (IdentityEligibilityResult, error)
}

func (s *Service) IdentitySummary(ctx context.Context, id uint) (IdentitySummary, error) {
	value, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return IdentitySummary{}, err
	}
	identities, err := s.identityAvailabilities(ctx, uint64(value.ID))
	if err != nil {
		return IdentitySummary{}, err
	}
	current := normalizeIdentity(value.CurrentIdentity)
	if !identityAvailable(identities, current) {
		current = IdentityStudent
	}
	return IdentitySummary{CurrentIdentity: current, Identities: identities}, nil
}

func (s *Service) SwitchIdentity(ctx context.Context, id uint, target Identity) (IdentitySummary, error) {
	if !target.Valid() {
		return IdentitySummary{}, ErrInvalidIdentity
	}
	value, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return IdentitySummary{}, err
	}
	identities, err := s.identityAvailabilities(ctx, uint64(id))
	if err != nil {
		return IdentitySummary{}, err
	}
	if !identityAvailable(identities, target) {
		return IdentitySummary{}, ErrIdentityUnavailable
	}
	if normalizeIdentity(value.CurrentIdentity) == target {
		return IdentitySummary{CurrentIdentity: target, Identities: identities}, nil
	}
	if err := s.repo.UpdateCurrentIdentity(ctx, id, target); err != nil {
		return IdentitySummary{}, err
	}
	return IdentitySummary{CurrentIdentity: target, Identities: identities}, nil
}

func (s *Service) identityAvailabilities(ctx context.Context, userID uint64) ([]IdentityAvailability, error) {
	rider := IdentityAvailability{
		Identity: IdentityRider, Available: false,
		Reason: "rider qualification required", Status: "missing",
	}
	merchant := IdentityAvailability{
		Identity: IdentityMerchant, Available: false,
		Reason: "merchant qualification required", Status: "missing",
	}
	if s.identityEligibility != nil {
		result, err := s.identityEligibility.RiderEligible(ctx, userID)
		if err != nil {
			return nil, err
		}
		rider.Available, rider.Reason, rider.Status = result.Available, result.Reason, result.Status
		result, err = s.identityEligibility.MerchantEligible(ctx, userID)
		if err != nil {
			return nil, err
		}
		merchant.Available, merchant.Reason, merchant.Status = result.Available, result.Reason, result.Status
	}
	return []IdentityAvailability{
		{Identity: IdentityStudent, Available: true, Status: "active"},
		rider,
		merchant,
	}, nil
}

func identityAvailable(values []IdentityAvailability, identity Identity) bool {
	for _, value := range values {
		if value.Identity == identity {
			return value.Available
		}
	}
	return false
}
