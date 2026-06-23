package media

import (
	"context"
	"time"
)

// Service owns the Module 2 core Media Domain invariants. Module 3 upload-ticket
// orchestration is isolated in UploadTicketService.
type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

type NewAsset struct {
	OwnerID         uint64
	Purpose         Purpose
	IdempotencyKey  string
	UploadExpiresAt *time.Time
}

func (s *Service) CreateAssetRecord(ctx context.Context, input NewAsset) (Asset, error) {
	publicID, err := NewULID()
	if err != nil {
		return Asset{}, err
	}
	now := time.Now().UTC()
	value := Asset{
		AssetPublicID: publicID, OwnerID: input.OwnerID, Purpose: input.Purpose,
		StorageStage: StorageStageStaging, ReviewStatus: ReviewPending,
		TechnicalStatus: TechnicalPendingUpload, ProjectedDecision: ReviewPending,
		IdempotencyKey: input.IdempotencyKey, UploadExpiresAt: input.UploadExpiresAt,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := value.ValidateNew(); err != nil {
		return Asset{}, err
	}
	if err := s.repository.CreateAsset(ctx, &value); err != nil {
		return Asset{}, err
	}
	return value, nil
}

func (s *Service) FindAsset(ctx context.Context, publicID string) (Asset, error) {
	if !ValidULID(publicID) {
		return Asset{}, ErrInvalidULID
	}
	return s.repository.FindAsset(ctx, publicID)
}

func (s *Service) TransitionTechnical(ctx context.Context, id, version uint64, from, to TechnicalStatus) error {
	if !CanTransitionTechnical(from, to) {
		return ErrInvalidTransition
	}
	return s.repository.CASAssetTechnicalStatus(ctx, id, version, from, to)
}

func (s *Service) TransitionStorage(ctx context.Context, id, version uint64, from, to StorageStage) error {
	if !CanTransitionStorage(from, to) {
		return ErrInvalidTransition
	}
	return s.repository.CASAssetStorageStage(ctx, id, version, from, to)
}

func (s *Service) CreateReferenceRecord(ctx context.Context, value AssetReference) (AssetReference, error) {
	if value.AssetID == 0 || !value.ReferenceType.Valid() || value.ReferenceID == "" || !ValidULID(value.OperationID) || value.Status != ReferencePending || value.RowVersion != 1 {
		return AssetReference{}, ErrInvalidRecord
	}
	now := time.Now().UTC()
	value.CreatedAt, value.UpdatedAt = now, now
	if value.ParentVisibility == "" {
		value.ParentVisibility = ParentHidden
	}
	if err := s.repository.CreateReference(ctx, &value); err != nil {
		return AssetReference{}, err
	}
	return value, nil
}

func (s *Service) TransitionReference(ctx context.Context, id, version uint64, from, to ReferenceStatus) error {
	if !CanTransitionReference(from, to) {
		return ErrInvalidTransition
	}
	return s.repository.CASReferenceStatus(ctx, id, version, from, to)
}

func (s *Service) TransitionReferenceVisibility(ctx context.Context, id, version uint64, from, to ParentVisibility) error {
	if from != ParentHidden || to != ParentVisible {
		return ErrInvalidTransition
	}
	value, err := s.repository.FindReference(ctx, id)
	if err != nil {
		return err
	}
	if value.ParentVisibility == to {
		return nil
	}
	return s.repository.CASParentVisibility(ctx, id, version, from, to)
}

func (s *Service) PublishReference(ctx context.Context, assetPublicID string, referenceType Purpose, referenceID string, slot uint16) error {
	if !ValidULID(assetPublicID) {
		return ErrInvalidULID
	}
	if !referenceType.Valid() || referenceID == "" {
		return ErrInvalidRecord
	}
	asset, err := s.repository.FindAsset(ctx, assetPublicID)
	if err != nil {
		return err
	}
	reference, err := s.repository.FindReferenceByCoordinates(ctx, asset.ID, referenceType, referenceID, slot)
	if err != nil {
		return err
	}
	switch reference.Status {
	case ReferencePending:
		if err := s.TransitionReference(ctx, reference.ID, reference.RowVersion, ReferencePending, ReferenceActive); err != nil {
			return err
		}
		reference, err = s.repository.FindReference(ctx, reference.ID)
		if err != nil {
			return err
		}
	case ReferenceActive:
	default:
		return ErrInvalidTransition
	}
	switch reference.ParentVisibility {
	case ParentHidden:
		return s.TransitionReferenceVisibility(ctx, reference.ID, reference.RowVersion, ParentHidden, ParentVisible)
	case ParentVisible:
		return nil
	default:
		return ErrInvalidTransition
	}
}

func (s *Service) RebuildActiveRefCount(ctx context.Context, assetID uint64) (uint64, error) {
	if assetID == 0 {
		return 0, ErrInvalidRecord
	}
	return s.repository.RebuildActiveRefCount(ctx, assetID)
}
