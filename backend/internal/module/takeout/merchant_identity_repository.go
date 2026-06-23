package takeout

import "context"

func (r *GormRepository) FindMerchantByOwnerUserID(ctx context.Context, ownerUserID uint64) (Merchant, error) {
	var value Merchant
	err := r.db.WithContext(ctx).Where("owner_user_id = ?", ownerUserID).Order("id DESC").First(&value).Error
	return value, normalizeNotFound(err)
}
