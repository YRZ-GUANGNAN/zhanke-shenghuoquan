package media

import "context"

type MediaGateway struct {
	repository MediaGatewayReadRepository
}

func NewMediaGateway(repository MediaGatewayReadRepository) (*MediaGateway, error) {
	if repository == nil {
		return nil, ErrInvalidRecord
	}
	return &MediaGateway{repository: repository}, nil
}

func (g *MediaGateway) CheckVisibility(ctx context.Context, mediaID string) (MediaVisibilityView, error) {
	if !ValidULID(mediaID) {
		return MediaVisibilityView{}, ErrInvalidULID
	}
	values, err := g.repository.FindGatewayAssetSnapshots(ctx, mediaID)
	if err != nil {
		return MediaVisibilityView{}, err
	}
	_, reason, visible := firstVisibleSnapshot(values)
	return MediaVisibilityView{MediaID: mediaID, Visible: visible, Reason: reason}, nil
}

func (g *MediaGateway) GetVisibleMetadata(ctx context.Context, mediaID string) (MediaMetadataView, error) {
	if !ValidULID(mediaID) {
		return MediaMetadataView{}, ErrInvalidULID
	}
	values, err := g.repository.FindGatewayAssetSnapshots(ctx, mediaID)
	if err != nil {
		return MediaMetadataView{}, err
	}
	value, _, visible := firstVisibleSnapshot(values)
	if !visible {
		return MediaMetadataView{}, ErrMediaNotVisible
	}
	return metadataView(value), nil
}

func (g *MediaGateway) GetReviewProjection(ctx context.Context, mediaID string) (ReviewProjectionView, error) {
	if !ValidULID(mediaID) {
		return ReviewProjectionView{}, ErrInvalidULID
	}
	values, err := g.repository.FindGatewayAssetSnapshots(ctx, mediaID)
	if err != nil {
		return ReviewProjectionView{}, err
	}
	value := values[0]
	if !value.ProjectionStatus.Valid() || value.ProjectionUpdatedAt == nil {
		return ReviewProjectionView{}, ErrReviewProjectionUnavailable
	}
	return ReviewProjectionView{
		MediaID: mediaID, Status: value.ProjectionStatus,
		AppliedDecisionVersion: value.AppliedDecisionVersion,
		DecidedAt:              value.ProjectionDecidedAt, UpdatedAt: *value.ProjectionUpdatedAt,
	}, nil
}

func (g *MediaGateway) BuildDisplayView(ctx context.Context, referenceType Purpose, referenceID string, slot uint16) (MediaDisplayView, error) {
	if !referenceType.Valid() || referenceID == "" {
		return MediaDisplayView{}, ErrInvalidRecord
	}
	values, err := g.repository.FindGatewayReferenceSnapshots(ctx, referenceType, referenceID)
	if err != nil {
		return MediaDisplayView{}, err
	}
	for _, value := range values {
		if value.ReferenceSlot != slot {
			continue
		}
		if reason := evaluateVisibility(value); reason != VisibilityAllowed {
			return MediaDisplayView{}, ErrMediaNotVisible
		}
		return displayView(value), nil
	}
	return MediaDisplayView{}, ErrNotFound
}

func (g *MediaGateway) ListDisplayViews(ctx context.Context, referenceType Purpose, referenceID string) ([]MediaDisplayView, error) {
	if !referenceType.Valid() || referenceID == "" {
		return nil, ErrInvalidRecord
	}
	values, err := g.repository.FindGatewayReferenceSnapshots(ctx, referenceType, referenceID)
	if err != nil {
		return nil, err
	}
	result := make([]MediaDisplayView, 0, len(values))
	for _, value := range values {
		if evaluateVisibility(value) == VisibilityAllowed {
			result = append(result, displayView(value))
		}
	}
	return result, nil
}

func firstVisibleSnapshot(values []GatewaySnapshot) (GatewaySnapshot, VisibilityReason, bool) {
	if len(values) == 0 {
		return GatewaySnapshot{}, VisibilityNotReady, false
	}
	reason := evaluateVisibility(values[0])
	for _, value := range values {
		current := evaluateVisibility(value)
		if current == VisibilityAllowed {
			return value, current, true
		}
		if reason == VisibilityReferenceInactive || reason == VisibilityParentHidden || reason == VisibilityRevoked || reason == VisibilityPurposeMismatch {
			reason = current
		}
	}
	return values[0], reason, false
}

func evaluateVisibility(value GatewaySnapshot) VisibilityReason {
	if value.TechnicalStatus != TechnicalFinalReady || value.StorageStage != StorageStageFinal ||
		!value.FinalIdentityReady || value.DeleteRequested {
		return VisibilityNotReady
	}
	if value.ProjectionStatus != ProjectionApproved {
		return VisibilityNotApproved
	}
	if !value.HasReference || value.ReferenceStatus != ReferenceActive {
		return VisibilityReferenceInactive
	}
	if value.ParentVisibility != ParentVisible {
		return VisibilityParentHidden
	}
	if value.RevocationFence != "" {
		return VisibilityRevoked
	}
	if value.ReferenceType != value.Purpose {
		return VisibilityPurposeMismatch
	}
	return VisibilityAllowed
}

func metadataView(value GatewaySnapshot) MediaMetadataView {
	return MediaMetadataView{
		MediaID: value.AssetPublicID, Purpose: value.Purpose, MIMEType: value.VerifiedMIME,
		ByteSize: value.ByteSize, Width: value.Width, Height: value.Height,
		ReviewStatus: value.ProjectionStatus,
	}
}

func displayView(value GatewaySnapshot) MediaDisplayView {
	return MediaDisplayView{
		Media: metadataView(value),
		Reference: MediaReferenceView{
			ReferenceType: value.ReferenceType, ReferenceID: value.ReferenceID, Slot: value.ReferenceSlot,
		},
	}
}
