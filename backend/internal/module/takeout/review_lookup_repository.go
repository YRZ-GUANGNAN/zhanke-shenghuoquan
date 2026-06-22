package takeout

import "context"

func (r *GormRepository) FindMerchantByInternalID(ctx context.Context, id uint64) (Merchant, error) {
	var value Merchant
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) FindDishByInternalID(ctx context.Context, id uint64) (Dish, error) {
	var value Dish
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&value).Error
	return value, normalizeNotFound(err)
}
