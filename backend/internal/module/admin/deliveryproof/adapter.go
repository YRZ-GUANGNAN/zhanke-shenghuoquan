package deliveryproof

import (
	"context"
	"errors"
	"strings"
	"time"

	"campus-circle/backend/internal/module/delivery"
	coremedia "campus-circle/backend/internal/module/media"
)

type MediaAdapter struct {
	service *coremedia.Service
	gateway *coremedia.MediaGateway
	display *coremedia.MediaDisplayService
}

func NewMediaAdapter(
	service *coremedia.Service,
	gateway *coremedia.MediaGateway,
	display *coremedia.MediaDisplayService,
) (*MediaAdapter, error) {
	if service == nil || gateway == nil || display == nil {
		return nil, delivery.ErrInvalidProof
	}
	return &MediaAdapter{service: service, gateway: gateway, display: display}, nil
}

func (a *MediaAdapter) ValidateOwnedApprovedAsset(ctx context.Context, ownerID uint64, assetPublicID string) error {
	assetPublicID = strings.TrimSpace(assetPublicID)
	if ownerID == 0 || !coremedia.ValidULID(assetPublicID) {
		return delivery.ErrProofAssetRejected
	}
	asset, err := a.service.FindAsset(ctx, assetPublicID)
	if err != nil {
		return delivery.ErrProofAssetRejected
	}
	if asset.OwnerID != ownerID || asset.Purpose != coremedia.PurposeDeliveryProof ||
		asset.StorageStage != coremedia.StorageStageFinal ||
		asset.TechnicalStatus != coremedia.TechnicalFinalReady ||
		asset.FinalKey == nil || strings.TrimSpace(*asset.FinalKey) == "" ||
		asset.FinalVersionID == nil || strings.TrimSpace(*asset.FinalVersionID) == "" {
		return delivery.ErrProofAssetRejected
	}
	projection, err := a.gateway.GetReviewProjection(ctx, assetPublicID)
	if err != nil || projection.Status != coremedia.ProjectionApproved {
		return delivery.ErrProofAssetRejected
	}
	return nil
}

func (a *MediaAdapter) PublishProofReference(
	ctx context.Context,
	assetPublicID string,
	proofPublicID string,
	slot uint16,
) error {
	if !coremedia.ValidULID(strings.TrimSpace(proofPublicID)) || slot != 0 {
		return delivery.ErrProofAssetRejected
	}
	if err := a.service.PublishReference(
		ctx, assetPublicID, coremedia.PurposeDeliveryProof, proofPublicID, slot,
	); err == nil {
		return nil
	} else if !errors.Is(err, coremedia.ErrNotFound) {
		return err
	}
	asset, err := a.service.FindAsset(ctx, assetPublicID)
	if err != nil {
		return err
	}
	operationID, err := coremedia.NewULID()
	if err != nil {
		return err
	}
	_, createErr := a.service.CreateReferenceRecord(ctx, coremedia.AssetReference{
		AssetID: asset.ID, ReferenceType: coremedia.PurposeDeliveryProof,
		ReferenceID: proofPublicID, Slot: slot, Status: coremedia.ReferencePending,
		ParentVisibility: coremedia.ParentHidden, AggregateVersion: 1,
		OperationID: operationID, RowVersion: 1,
	})
	if createErr != nil {
		// A concurrent retry may have inserted the same immutable coordinate.
		// PublishReference is the authoritative idempotency check.
		if retryErr := a.service.PublishReference(
			ctx, assetPublicID, coremedia.PurposeDeliveryProof, proofPublicID, slot,
		); retryErr == nil {
			return nil
		}
		return createErr
	}
	return a.service.PublishReference(
		ctx, assetPublicID, coremedia.PurposeDeliveryProof, proofPublicID, slot,
	)
}

func (a *MediaAdapter) ResolveProofURL(
	ctx context.Context,
	proofPublicID string,
	slot uint16,
	ttl time.Duration,
) (string, error) {
	if ttl != delivery.PrivateProofURLTTL {
		return "", delivery.ErrInvalidProof
	}
	return a.display.ContentURL(
		ctx, coremedia.PurposeDeliveryProof, strings.TrimSpace(proofPublicID), slot,
	)
}

var _ delivery.MediaProofPort = (*MediaAdapter)(nil)
