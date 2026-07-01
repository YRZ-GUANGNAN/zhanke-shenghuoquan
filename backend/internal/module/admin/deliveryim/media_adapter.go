package deliveryim

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
		return nil, delivery.ErrInvalidRecord
	}
	return &MediaAdapter{service: service, gateway: gateway, display: display}, nil
}

func (a *MediaAdapter) ValidateOwnedApprovedConversationAsset(
	ctx context.Context,
	ownerID uint64,
	assetPublicID string,
) error {
	assetPublicID = strings.TrimSpace(assetPublicID)
	if ownerID == 0 || !coremedia.ValidULID(assetPublicID) {
		return delivery.ErrConversationImageDenied
	}
	asset, err := a.service.FindAsset(ctx, assetPublicID)
	if err != nil || asset.OwnerID != ownerID || asset.Purpose != coremedia.PurposeDeliveryIM ||
		asset.StorageStage != coremedia.StorageStageFinal ||
		asset.TechnicalStatus != coremedia.TechnicalFinalReady ||
		asset.FinalKey == nil || strings.TrimSpace(*asset.FinalKey) == "" ||
		asset.FinalVersionID == nil || strings.TrimSpace(*asset.FinalVersionID) == "" {
		return delivery.ErrConversationImageDenied
	}
	projection, err := a.gateway.GetReviewProjection(ctx, assetPublicID)
	if err != nil || projection.Status != coremedia.ProjectionApproved {
		return delivery.ErrConversationImageDenied
	}
	return nil
}

func (a *MediaAdapter) PublishConversationImage(
	ctx context.Context,
	assetPublicID, conversationPublicID string,
	slot uint16,
) error {
	if !coremedia.ValidULID(strings.TrimSpace(conversationPublicID)) {
		return delivery.ErrConversationImageDenied
	}
	if err := a.service.PublishReference(
		ctx, assetPublicID, coremedia.PurposeDeliveryIM, conversationPublicID, slot,
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
		AssetID: asset.ID, ReferenceType: coremedia.PurposeDeliveryIM,
		ReferenceID: conversationPublicID, Slot: slot, Status: coremedia.ReferencePending,
		ParentVisibility: coremedia.ParentHidden, AggregateVersion: 1,
		OperationID: operationID, RowVersion: 1,
	})
	if createErr != nil {
		if retryErr := a.service.PublishReference(
			ctx, assetPublicID, coremedia.PurposeDeliveryIM, conversationPublicID, slot,
		); retryErr == nil {
			return nil
		}
		return createErr
	}
	return a.service.PublishReference(
		ctx, assetPublicID, coremedia.PurposeDeliveryIM, conversationPublicID, slot,
	)
}

func (a *MediaAdapter) ResolveConversationImage(
	ctx context.Context,
	conversationPublicID string,
	slot uint16,
	ttl time.Duration,
) (string, error) {
	if ttl != delivery.PrivateConversationImageURLTTL {
		return "", delivery.ErrInvalidRecord
	}
	return a.display.ContentURL(
		ctx, coremedia.PurposeDeliveryIM, strings.TrimSpace(conversationPublicID), slot,
	)
}

var _ delivery.ConversationMediaPort = (*MediaAdapter)(nil)
