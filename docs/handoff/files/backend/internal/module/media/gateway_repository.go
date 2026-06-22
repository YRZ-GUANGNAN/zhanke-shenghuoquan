package media

import (
	"context"
)

// MediaGatewayReadRepository is a query-only boundary. Implementations must
// not expose a database handle and must not execute any mutation.
type MediaGatewayReadRepository interface {
	FindGatewayAssetSnapshots(context.Context, string) ([]GatewaySnapshot, error)
	FindGatewayReferenceSnapshots(context.Context, Purpose, string) ([]GatewaySnapshot, error)
}

const gatewaySnapshotSelect = `
	a.asset_public_id AS asset_public_id,
	a.purpose AS purpose,
	a.storage_stage AS storage_stage,
	a.technical_status AS technical_status,
	a.verified_mime AS verified_mime,
	a.byte_size AS byte_size,
	a.width AS width,
	a.height AS height,
	(a.final_key IS NOT NULL AND a.final_key <> '' AND a.final_version_id IS NOT NULL AND a.final_version_id <> '') AS final_identity_ready,
	(a.delete_requested_at IS NOT NULL OR a.deleted_at IS NOT NULL) AS delete_requested,
	COALESCE(p.status, '') AS projection_status,
	COALESCE(p.applied_decision_version, 0) AS applied_decision_version,
	p.decided_at AS projection_decided_at,
	p.updated_at AS projection_updated_at,
	(r.id IS NOT NULL) AS has_reference,
	COALESCE(r.reference_type, '') AS reference_type,
	COALESCE(r.reference_id, '') AS reference_id,
	COALESCE(r.slot, 0) AS reference_slot,
	COALESCE(r.status, '') AS reference_status,
	COALESCE(r.parent_visibility, '') AS parent_visibility,
	COALESCE(r.revocation_fence, '') AS revocation_fence`

func (r *GormRepository) FindGatewayAssetSnapshots(ctx context.Context, assetPublicID string) ([]GatewaySnapshot, error) {
	var values []GatewaySnapshot
	err := r.db.WithContext(ctx).
		Table("media_assets AS a").
		Select(gatewaySnapshotSelect).
		Joins("LEFT JOIN media_review_projections AS p ON p.asset_id = a.id").
		Joins("LEFT JOIN media_asset_references AS r ON r.asset_id = a.id").
		Where("a.asset_public_id = ?", assetPublicID).
		Order("r.slot ASC, r.id ASC").
		Scan(&values).Error
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, ErrNotFound
	}
	return values, nil
}

func (r *GormRepository) FindGatewayReferenceSnapshots(ctx context.Context, referenceType Purpose, referenceID string) ([]GatewaySnapshot, error) {
	var values []GatewaySnapshot
	err := r.db.WithContext(ctx).
		Table("media_asset_references AS r").
		Select(gatewaySnapshotSelect).
		Joins("JOIN media_assets AS a ON a.id = r.asset_id").
		Joins("LEFT JOIN media_review_projections AS p ON p.asset_id = a.id").
		Where("r.reference_type = ? AND r.reference_id = ?", referenceType, referenceID).
		Order("r.slot ASC, r.id ASC").
		Scan(&values).Error
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, ErrNotFound
	}
	return values, nil
}

var _ MediaGatewayReadRepository = (*GormRepository)(nil)
