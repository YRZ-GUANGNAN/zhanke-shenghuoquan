package takeout

import "context"

func (r *GormRepository) ListDishesFiltered(
	ctx context.Context,
	merchantPublicID, categoryPublicID string,
	status DishStatus,
	page, size int,
) ([]Dish, int64, error) {
	query := r.db.WithContext(ctx).Model(&Dish{}).Where("category_public_id = ?", categoryPublicID)
	if merchantPublicID != "" {
		query = query.Where("merchant_public_id = ?", merchantPublicID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []Dish
	err := query.Order("sort_order ASC, created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}
