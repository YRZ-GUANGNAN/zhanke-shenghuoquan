package media

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type ReferenceBindingService struct {
	repository ReferenceBindingRepository
	now        func() time.Time
}

func NewReferenceBindingService(repository ReferenceBindingRepository, now func() time.Time) (*ReferenceBindingService, error) {
	if repository == nil {
		return nil, ErrInvalidRecord
	}
	if now == nil {
		now = time.Now
	}
	return &ReferenceBindingService{repository: repository, now: now}, nil
}

func (s *ReferenceBindingService) Reserve(ctx context.Context, input BindingReserveInput) (BindingReservation, error) {
	if err := validateBindingReserve(input, s.now().UTC()); err != nil {
		return BindingReservation{}, err
	}
	references, err := s.repository.ReserveBinding(ctx, input)
	if err != nil {
		return BindingReservation{}, err
	}
	if len(references) != len(input.OrderedAssetPublicIDs) {
		return BindingReservation{}, ErrBindingMismatch
	}
	byPublicID := make(map[string]BindingAsset, len(references))
	for _, reference := range references {
		if reference.InternalID == 0 || !ValidULID(reference.AssetPublicID) {
			return BindingReservation{}, ErrBindingMismatch
		}
		if _, exists := byPublicID[reference.AssetPublicID]; exists {
			return BindingReservation{}, ErrBindingMismatch
		}
		byPublicID[reference.AssetPublicID] = reference
	}
	ordered := make([]BindingAsset, 0, len(input.OrderedAssetPublicIDs))
	for index, publicID := range input.OrderedAssetPublicIDs {
		reference, exists := byPublicID[publicID]
		if !exists {
			return BindingReservation{}, ErrBindingMismatch
		}
		reference.Slot = uint16(index)
		ordered = append(ordered, reference)
	}
	return BindingReservation{Token: input.Token, References: ordered}, nil
}

func (s *ReferenceBindingService) Commit(ctx context.Context, input BindingCommitInput) error {
	if err := validateBindingCommit(input); err != nil {
		return err
	}
	return s.repository.CommitBinding(ctx, input)
}

func (s *ReferenceBindingService) EnsureCommitted(ctx context.Context, input BindingCommitInput) error {
	if err := validateBindingCommit(input); err != nil {
		return err
	}
	commitErr := s.repository.CommitBinding(ctx, input)
	if commitErr == nil {
		return nil
	}
	exact, readErr := s.repository.ExactCommittedBinding(ctx, input)
	if readErr != nil {
		return errors.Join(commitErr, readErr)
	}
	if exact {
		return nil
	}
	if errors.Is(commitErr, ErrBindingConflict) || errors.Is(commitErr, ErrBindingMismatch) {
		return ErrBindingMismatch
	}
	return commitErr
}

func (s *ReferenceBindingService) Cancel(ctx context.Context, ownerID uint64, token string) error {
	if ownerID == 0 || !validModule10BindingToken(token) {
		return ErrBindingInvalidAsset
	}
	return s.repository.CancelBinding(ctx, ownerID, token)
}

func (s *ReferenceBindingService) Publish(ctx context.Context, referenceType Purpose, referenceID string) error {
	if !referenceType.Valid() || strings.TrimSpace(referenceID) == "" || len(referenceID) > 64 {
		return ErrBindingInvalidAsset
	}
	return s.repository.PublishBinding(ctx, referenceType, strings.TrimSpace(referenceID))
}

func (s *ReferenceBindingService) Revoke(ctx context.Context, referenceType Purpose, referenceID string, keep []uint64) error {
	if !referenceType.Valid() || strings.TrimSpace(referenceID) == "" || len(referenceID) > 64 || hasDuplicateInternalIDs(keep) {
		return ErrBindingInvalidAsset
	}
	fence, err := NewULID()
	if err != nil {
		return err
	}
	return s.repository.RevokeBinding(ctx, referenceType, strings.TrimSpace(referenceID), append([]uint64(nil), keep...), fence)
}

func validateBindingReserve(input BindingReserveInput, now time.Time) error {
	if input.OwnerID == 0 || !input.Purpose.Valid() || !validModule10BindingToken(input.Token) ||
		!input.ExpiresAt.After(now) || len(input.OrderedAssetPublicIDs) == 0 ||
		len(input.OrderedAssetPublicIDs) > maxReferenceBindingAssets || len(input.ExistingReferenceID) > 64 {
		return ErrBindingInvalidAsset
	}
	seen := make(map[string]struct{}, len(input.OrderedAssetPublicIDs))
	for _, publicID := range input.OrderedAssetPublicIDs {
		if !validBindingLookupID(publicID) {
			return ErrBindingInvalidAsset
		}
		if _, exists := seen[publicID]; exists {
			return ErrBindingInvalidAsset
		}
		seen[publicID] = struct{}{}
	}
	return nil
}

func validBindingLookupID(value string) bool {
	if ValidULID(value) {
		return true
	}
	if len(value) != 32 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 16
}

func validateBindingCommit(input BindingCommitInput) error {
	if input.OwnerID == 0 || !validModule10BindingToken(input.Token) || !input.ReferenceType.Valid() ||
		strings.TrimSpace(input.ReferenceID) == "" || len(input.ReferenceID) > 64 ||
		len(input.OrderedAssetPublicIDs) == 0 || len(input.OrderedAssetPublicIDs) > maxReferenceBindingAssets {
		return ErrBindingInvalidAsset
	}
	seen := make(map[string]struct{}, len(input.OrderedAssetPublicIDs))
	for _, publicID := range input.OrderedAssetPublicIDs {
		if !ValidULID(publicID) {
			return ErrBindingInvalidAsset
		}
		if _, exists := seen[publicID]; exists {
			return ErrBindingInvalidAsset
		}
		seen[publicID] = struct{}{}
	}
	return nil
}

func validModule10BindingToken(value string) bool {
	return strings.HasPrefix(value, module10BindingTokenPrefix) &&
		len(value) == len(module10BindingTokenPrefix)+26 && ValidULID(strings.TrimPrefix(value, module10BindingTokenPrefix))
}

func hasDuplicateInternalIDs(values []uint64) bool {
	seen := make(map[uint64]struct{}, len(values))
	for _, value := range values {
		if value == 0 {
			return true
		}
		if _, exists := seen[value]; exists {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}
