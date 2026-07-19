package identityapplication

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
	"gorm.io/gorm"
)

const (
	identityMediaReservationTTL = 15 * time.Minute
	identityProofURLTTL         = 5 * time.Minute
	maxIdentityProofAssets      = 6
)

var (
	ErrProofMediaUnavailable = errors.New("identity application proof media unavailable")
	ErrProofMediaConflict    = errors.New("identity application proof media conflict")
)

type MediaReservation struct {
	Token            string
	AssetPublicIDs   []string
	InternalAssetIDs []uint64
}

// ApplicationMedia is the private media boundary used by identity applications.
// Admin proof resolution requires the current application ID so an asset ID can
// never be signed without proving its exact application binding.
type ApplicationMedia interface {
	Reserve(context.Context, uint64, []string) (MediaReservation, error)
	ReserveForApplication(context.Context, uint64, string, []string) (MediaReservation, error)
	Commit(context.Context, uint64, MediaReservation, string) error
	EnsureCommitted(context.Context, uint64, MediaReservation, string) error
	EnsureCommittedInTransaction(context.Context, *gorm.DB, uint64, MediaReservation, string) error
	Cancel(context.Context, uint64, MediaReservation) error
	ResolveAdminProofs(context.Context, string, []string) ([]ProofAssetView, error)
}

type referenceBindingGateway interface {
	Reserve(context.Context, coremedia.BindingReserveInput) (coremedia.BindingReservation, error)
	Commit(context.Context, coremedia.BindingCommitInput) error
	EnsureCommitted(context.Context, coremedia.BindingCommitInput) error
	Cancel(context.Context, uint64, string) error
}

type proofMediaRepository interface {
	FindAsset(context.Context, string) (coremedia.Asset, error)
	FindReferenceByCoordinates(context.Context, uint64, coremedia.Purpose, string, uint16) (coremedia.AssetReference, error)
	FindGatewayAssetSnapshots(context.Context, string) ([]coremedia.GatewaySnapshot, error)
}

type CoreApplicationMedia struct {
	bindings   referenceBindingGateway
	repository proofMediaRepository
	signer     coremedia.DisplayURLSigner
	now        func() time.Time
}

func NewApplicationMedia(bindings referenceBindingGateway, repository proofMediaRepository, signer coremedia.DisplayURLSigner, now func() time.Time) (*CoreApplicationMedia, error) {
	if bindings == nil || repository == nil || signer == nil {
		return nil, ErrProofMediaUnavailable
	}
	if now == nil {
		now = time.Now
	}
	return &CoreApplicationMedia{bindings: bindings, repository: repository, signer: signer, now: now}, nil
}

func (m *CoreApplicationMedia) Reserve(ctx context.Context, ownerID uint64, assetPublicIDs []string) (MediaReservation, error) {
	return m.reserve(ctx, ownerID, "", assetPublicIDs)
}

func (m *CoreApplicationMedia) ReserveForApplication(ctx context.Context, ownerID uint64, applicationPublicID string, assetPublicIDs []string) (MediaReservation, error) {
	if !validULID(applicationPublicID) {
		return MediaReservation{}, ErrProofMediaUnavailable
	}
	return m.reserve(ctx, ownerID, applicationPublicID, assetPublicIDs)
}

func (m *CoreApplicationMedia) reserve(ctx context.Context, ownerID uint64, existingReferenceID string, assetPublicIDs []string) (MediaReservation, error) {
	ordered, err := validateProofAssetIDs(assetPublicIDs)
	if err != nil || ownerID == 0 {
		return MediaReservation{}, ErrProofMediaUnavailable
	}
	operationID, err := coremedia.NewULID()
	if err != nil {
		return MediaReservation{}, err
	}
	token := "m10_" + operationID
	value, err := m.bindings.Reserve(ctx, coremedia.BindingReserveInput{
		OwnerID:               ownerID,
		Purpose:               coremedia.PurposeIdentityApplication,
		OrderedAssetPublicIDs: ordered,
		Token:                 token,
		ExpiresAt:             m.now().UTC().Add(identityMediaReservationTTL),
		ExistingReferenceID:   existingReferenceID,
	})
	if err != nil {
		return MediaReservation{}, err
	}
	if value.Token != token || len(value.References) != len(ordered) {
		_ = m.bindings.Cancel(ctx, ownerID, token)
		return MediaReservation{}, errors.Join(ErrProofMediaUnavailable, coremedia.ErrBindingMismatch)
	}
	internalAssetIDs := make([]uint64, len(value.References))
	seenInternalIDs := make(map[uint64]struct{}, len(value.References))
	for index, reference := range value.References {
		if reference.InternalID == 0 || reference.AssetPublicID != ordered[index] || reference.Slot != uint16(index) {
			_ = m.bindings.Cancel(ctx, ownerID, token)
			return MediaReservation{}, errors.Join(ErrProofMediaUnavailable, coremedia.ErrBindingMismatch)
		}
		if _, duplicate := seenInternalIDs[reference.InternalID]; duplicate {
			_ = m.bindings.Cancel(ctx, ownerID, token)
			return MediaReservation{}, errors.Join(ErrProofMediaUnavailable, coremedia.ErrBindingMismatch)
		}
		seenInternalIDs[reference.InternalID] = struct{}{}
		internalAssetIDs[index] = reference.InternalID
	}
	return MediaReservation{Token: token, AssetPublicIDs: ordered, InternalAssetIDs: internalAssetIDs}, nil
}

func (m *CoreApplicationMedia) Commit(ctx context.Context, ownerID uint64, reservation MediaReservation, applicationPublicID string) error {
	input, err := proofBindingCommitInput(ownerID, reservation, applicationPublicID)
	if err != nil {
		return err
	}
	return m.bindings.Commit(ctx, input)
}

func (m *CoreApplicationMedia) EnsureCommitted(ctx context.Context, ownerID uint64, reservation MediaReservation, applicationPublicID string) error {
	input, err := proofBindingCommitInput(ownerID, reservation, applicationPublicID)
	if err != nil {
		return err
	}
	return m.bindings.EnsureCommitted(ctx, input)
}

func (m *CoreApplicationMedia) EnsureCommittedInTransaction(ctx context.Context, tx *gorm.DB, ownerID uint64, reservation MediaReservation, applicationPublicID string) error {
	if tx == nil {
		return ErrProofMediaUnavailable
	}
	input, err := proofBindingCommitInput(ownerID, reservation, applicationPublicID)
	if err != nil {
		return err
	}
	keep, err := validateReservationInternalIDs(reservation.InternalAssetIDs, len(input.OrderedAssetPublicIDs))
	if err != nil {
		return err
	}
	repository := coremedia.NewReferenceBindingRepository(tx)
	bindings, err := coremedia.NewReferenceBindingService(repository, m.now)
	if err != nil {
		return err
	}
	if err := bindings.EnsureCommitted(ctx, input); err != nil {
		return err
	}
	if err := bindings.Revoke(ctx, coremedia.PurposeIdentityApplication, applicationPublicID, keep); err != nil {
		return err
	}
	exact, err := repository.ExactCommittedBinding(ctx, input)
	if err != nil {
		return err
	}
	if !exact {
		return coremedia.ErrBindingMismatch
	}
	return assertExactLiveProofReferences(ctx, tx, applicationPublicID, keep)
}

func (m *CoreApplicationMedia) Cancel(ctx context.Context, ownerID uint64, reservation MediaReservation) error {
	if ownerID == 0 || !validMediaReservationToken(reservation.Token) {
		return ErrProofMediaUnavailable
	}
	return m.bindings.Cancel(ctx, ownerID, reservation.Token)
}

func (m *CoreApplicationMedia) ResolveAdminProofs(ctx context.Context, applicationPublicID string, assetPublicIDs []string) ([]ProofAssetView, error) {
	if !validULID(applicationPublicID) {
		return nil, ErrProofMediaUnavailable
	}
	ordered, err := validateProofAssetIDs(assetPublicIDs)
	if err != nil {
		return nil, ErrProofMediaUnavailable
	}

	type finalObject struct {
		assetID string
		key     string
		version string
	}
	objects := make([]finalObject, 0, len(ordered))
	for index, assetPublicID := range ordered {
		asset, findErr := m.repository.FindAsset(ctx, assetPublicID)
		if findErr != nil || !validAdminProofAsset(asset, assetPublicID) {
			return nil, proofUnavailable(findErr)
		}
		snapshots, findErr := m.repository.FindGatewayAssetSnapshots(ctx, assetPublicID)
		if findErr != nil || len(snapshots) == 0 || !validAdminProofSnapshot(snapshots[0], assetPublicID) {
			return nil, proofUnavailable(findErr)
		}
		reference, findErr := m.repository.FindReferenceByCoordinates(
			ctx, asset.ID, coremedia.PurposeIdentityApplication, applicationPublicID, uint16(index),
		)
		if findErr != nil || !validAdminProofReference(reference, asset.ID, applicationPublicID, uint16(index)) {
			return nil, proofUnavailable(findErr)
		}
		objects = append(objects, finalObject{
			assetID: assetPublicID,
			key:     strings.TrimSpace(*asset.FinalKey),
			version: strings.TrimSpace(*asset.FinalVersionID),
		})
	}

	expiresAt := m.now().UTC().Add(identityProofURLTTL)
	proofs := make([]ProofAssetView, 0, len(objects))
	for _, object := range objects {
		url, signErr := m.signer.SignedGetURL(ctx, object.key, object.version, identityProofURLTTL)
		if signErr != nil || strings.TrimSpace(url) == "" {
			return nil, proofUnavailable(signErr)
		}
		proofs = append(proofs, ProofAssetView{AssetID: object.assetID, URL: url, ExpiresAt: expiresAt})
	}
	return proofs, nil
}

func validateProofAssetIDs(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > maxIdentityProofAssets {
		return nil, ErrProofMediaUnavailable
	}
	ordered := make([]string, len(values))
	seen := make(map[string]struct{}, len(values))
	for index, value := range values {
		value = strings.TrimSpace(value)
		if !validULID(value) {
			return nil, ErrProofMediaUnavailable
		}
		if _, duplicate := seen[value]; duplicate {
			return nil, ErrProofMediaUnavailable
		}
		seen[value] = struct{}{}
		ordered[index] = value
	}
	return ordered, nil
}

func proofBindingCommitInput(ownerID uint64, reservation MediaReservation, applicationPublicID string) (coremedia.BindingCommitInput, error) {
	ordered, err := validateProofAssetIDs(reservation.AssetPublicIDs)
	if err != nil || ownerID == 0 || !validMediaReservationToken(reservation.Token) || !validULID(applicationPublicID) {
		return coremedia.BindingCommitInput{}, ErrProofMediaUnavailable
	}
	return coremedia.BindingCommitInput{
		OwnerID:               ownerID,
		Token:                 reservation.Token,
		ReferenceType:         coremedia.PurposeIdentityApplication,
		ReferenceID:           applicationPublicID,
		OrderedAssetPublicIDs: ordered,
	}, nil
}

func validateReservationInternalIDs(values []uint64, expected int) ([]uint64, error) {
	if expected <= 0 || len(values) != expected {
		return nil, ErrProofMediaUnavailable
	}
	ordered := append([]uint64(nil), values...)
	seen := make(map[uint64]struct{}, len(ordered))
	for _, value := range ordered {
		if value == 0 {
			return nil, ErrProofMediaUnavailable
		}
		if _, duplicate := seen[value]; duplicate {
			return nil, ErrProofMediaUnavailable
		}
		seen[value] = struct{}{}
	}
	return ordered, nil
}

func assertExactLiveProofReferences(ctx context.Context, tx *gorm.DB, applicationPublicID string, internalAssetIDs []uint64) error {
	var references []coremedia.AssetReference
	if err := tx.WithContext(ctx).
		Where("reference_type = ? AND reference_id = ? AND status IN ?", coremedia.PurposeIdentityApplication, applicationPublicID,
			[]coremedia.ReferenceStatus{coremedia.ReferencePending, coremedia.ReferenceActive}).
		Order("slot ASC, id ASC").Find(&references).Error; err != nil {
		return err
	}
	if len(references) != len(internalAssetIDs) {
		return coremedia.ErrBindingMismatch
	}
	for index, reference := range references {
		if reference.AssetID != internalAssetIDs[index] || reference.Slot != uint16(index) ||
			reference.ReferenceType != coremedia.PurposeIdentityApplication || reference.ReferenceID != applicationPublicID ||
			reference.RevocationFence != nil || reference.ParentVisibility != coremedia.ParentHidden ||
			(reference.Status != coremedia.ReferencePending && reference.Status != coremedia.ReferenceActive) {
			return coremedia.ErrBindingMismatch
		}
	}
	return nil
}

func mapProofSubmissionMediaError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrProofMediaConflict) || errors.Is(err, coremedia.ErrBindingInvalidAsset) ||
		errors.Is(err, coremedia.ErrBindingConflict) || errors.Is(err, coremedia.ErrBindingMismatch) ||
		errors.Is(err, coremedia.ErrBindingLegacySet) {
		return errors.Join(ErrProofMediaConflict, err)
	}
	return err
}

func validMediaReservationToken(token string) bool {
	return strings.HasPrefix(token, "m10_") && validULID(strings.TrimPrefix(token, "m10_"))
}

func validAdminProofAsset(asset coremedia.Asset, requestedPublicID string) bool {
	return asset.ID != 0 && asset.AssetPublicID == requestedPublicID &&
		asset.Purpose == coremedia.PurposeIdentityApplication &&
		asset.StorageStage == coremedia.StorageStageFinal &&
		asset.TechnicalStatus == coremedia.TechnicalFinalReady &&
		asset.FinalKey != nil && strings.TrimSpace(*asset.FinalKey) != "" &&
		asset.FinalVersionID != nil && strings.TrimSpace(*asset.FinalVersionID) != "" &&
		asset.DeleteRequestedAt == nil && asset.DeletedAt == nil
}

func validAdminProofSnapshot(snapshot coremedia.GatewaySnapshot, requestedPublicID string) bool {
	return snapshot.AssetPublicID == requestedPublicID &&
		snapshot.Purpose == coremedia.PurposeIdentityApplication &&
		snapshot.StorageStage == coremedia.StorageStageFinal &&
		snapshot.TechnicalStatus == coremedia.TechnicalFinalReady &&
		snapshot.FinalIdentityReady && !snapshot.DeleteRequested &&
		snapshot.ProjectionStatus == coremedia.ProjectionApproved
}

func validAdminProofReference(reference coremedia.AssetReference, assetID uint64, applicationPublicID string, slot uint16) bool {
	return reference.ID != 0 && reference.AssetID == assetID &&
		reference.ReferenceType == coremedia.PurposeIdentityApplication &&
		reference.ReferenceID == applicationPublicID && reference.Slot == slot &&
		(reference.Status == coremedia.ReferencePending || reference.Status == coremedia.ReferenceActive) &&
		reference.ParentVisibility == coremedia.ParentHidden && reference.RevocationFence == nil
}

func proofUnavailable(cause error) error {
	if cause == nil {
		return ErrProofMediaUnavailable
	}
	return fmt.Errorf("%w: %v", ErrProofMediaUnavailable, cause)
}

var _ ApplicationMedia = (*CoreApplicationMedia)(nil)
var _ referenceBindingGateway = (*coremedia.ReferenceBindingService)(nil)
var _ proofMediaRepository = (*coremedia.GormRepository)(nil)
