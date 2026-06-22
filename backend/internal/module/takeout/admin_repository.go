package takeout

import (
	"context"
)

func (r *GormRepository) ListMerchants(ctx context.Context, status MerchantStatus, page, size int) ([]Merchant, int64, error) {
	query := r.db.WithContext(ctx).Model(&Merchant{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []Merchant
	err := query.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) ListCategories(ctx context.Context, merchantPublicID string, page, size int) ([]DishCategory, int64, error) {
	query := r.db.WithContext(ctx).Model(&DishCategory{})
	if merchantPublicID != "" {
		query = query.Where("merchant_public_id = ?", merchantPublicID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []DishCategory
	err := query.Order("sort_order ASC, created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) ListDishes(ctx context.Context, merchantPublicID string, status DishStatus, page, size int) ([]Dish, int64, error) {
	query := r.db.WithContext(ctx).Model(&Dish{})
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

func (r *GormRepository) ListOrders(ctx context.Context, status OrderStatus, page, size int) ([]Order, int64, error) {
	query := r.db.WithContext(ctx).Model(&Order{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []Order
	err := query.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) ListRiders(ctx context.Context, status RiderStatus, page, size int) ([]Rider, int64, error) {
	query := r.db.WithContext(ctx).Model(&Rider{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []Rider
	err := query.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) ListSettlementSummaries(ctx context.Context, page, size int) ([]SettlementSummary, int64, error) {
	base := r.db.WithContext(ctx).Model(&RiderEarningRecord{}).
		Where("settlement_status = ?", SettlementPending).
		Select("rider_user_id")
	var total int64
	if err := r.db.WithContext(ctx).Table("(?) AS pending_riders", base.Group("rider_user_id")).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []SettlementSummary
	err := r.db.WithContext(ctx).Model(&RiderEarningRecord{}).
		Select("rider_user_id, COUNT(*) AS pending_count, COALESCE(SUM(rider_fee_cents), 0) AS rider_fee_cents, COALESCE(SUM(platform_cut_cents), 0) AS platform_cut_cents").
		Where("settlement_status = ?", SettlementPending).
		Group("rider_user_id").Order("rider_user_id ASC").
		Offset((page - 1) * size).Limit(size).Scan(&values).Error
	return values, total, err
}

var _ AdminQueryRepository = (*GormRepository)(nil)
