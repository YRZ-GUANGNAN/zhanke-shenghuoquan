package media

import "context"

func (r *GormRepository) FindVisibleDisplayObjectByReference(ctx context.Context, referenceType Purpose, referenceID string, slot uint16) (DisplayObjectIdentity, error) {
	var value DisplayObjectIdentity
	result := r.db.WithContext(ctx).
		Table("media_asset_references AS r").
		Select("a.asset_public_id AS media_id, a.final_key AS final_key, a.final_version_id AS final_version_id").
		Joins("JOIN media_assets AS a ON a.id = r.asset_id").
		Joins("JOIN media_review_projections AS p ON p.asset_id = a.id").
		Where("r.reference_type = ? AND r.reference_id = ? AND r.slot = ?", referenceType, referenceID, slot).
		Where("r.reference_type = a.purpose").
		Where("r.status = ? AND r.parent_visibility = ?", ReferenceActive, ParentVisible).
		Where("(r.revocation_fence IS NULL OR r.revocation_fence = '')").
		Where("a.storage_stage = ? AND a.technical_status = ?", StorageStageFinal, TechnicalFinalReady).
		Where("a.final_key IS NOT NULL AND a.final_key <> '' AND a.final_version_id IS NOT NULL AND a.final_version_id <> ''").
		Where("a.delete_requested_at IS NULL AND a.deleted_at IS NULL").
		Where("p.status = ?", ProjectionApproved).
		Limit(1).
		Scan(&value)
	if result.Error != nil {
		return DisplayObjectIdentity{}, result.Error
	}
	if result.RowsAffected == 0 {
		return DisplayObjectIdentity{}, ErrNotFound
	}
	return value, nil
}

func (r *GormRepository) FindVisibleDisplayObjectByAsset(ctx context.Context, assetPublicID string) (DisplayObjectIdentity, error) {
	var value DisplayObjectIdentity
	result := r.db.WithContext(ctx).
		Table("media_asset_references AS r").
		Select("a.asset_public_id AS media_id, a.final_key AS final_key, a.final_version_id AS final_version_id").
		Joins("JOIN media_assets AS a ON a.id = r.asset_id").
		Joins("JOIN media_review_projections AS p ON p.asset_id = a.id").
		Where("a.asset_public_id = ?", assetPublicID).
		Where("r.reference_type = a.purpose").
		Where("r.status = ? AND r.parent_visibility = ?", ReferenceActive, ParentVisible).
		Where("(r.revocation_fence IS NULL OR r.revocation_fence = '')").
		Where("a.storage_stage = ? AND a.technical_status = ?", StorageStageFinal, TechnicalFinalReady).
		Where("a.final_key IS NOT NULL AND a.final_key <> '' AND a.final_version_id IS NOT NULL AND a.final_version_id <> ''").
		Where("a.delete_requested_at IS NULL AND a.deleted_at IS NULL").
		Where("p.status = ?", ProjectionApproved).
		Order("r.id ASC").
		Limit(1).
		Scan(&value)
	if result.Error != nil {
		return DisplayObjectIdentity{}, result.Error
	}
	if result.RowsAffected == 0 {
		return DisplayObjectIdentity{}, ErrNotFound
	}
	return value, nil
}

var _ MediaDisplayReadRepository = (*GormRepository)(nil)
