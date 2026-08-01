package takeout

import (
	"context"

	"campus-circle/backend/internal/pkg/dbtx"
)

func (r *GormRepository) FindMerchantByInternalID(ctx context.Context, id uint64) (Merchant, error) {
	var value Merchant
	err := dbtx.DB(ctx, r.db).Where("id = ?", id).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) FindDishByInternalID(ctx context.Context, id uint64) (Dish, error) {
	var value Dish
	err := dbtx.DB(ctx, r.db).Where("id = ?", id).First(&value).Error
	return value, normalizeNotFound(err)
}
