package takeout

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MerchantRepository interface {
	CreateMerchant(context.Context, *Merchant) error
	FindMerchant(context.Context, string) (Merchant, error)
	TransitionMerchantStatus(context.Context, string, *uint64, uint64, MerchantStatus, MerchantStatus, time.Time) (Merchant, error)
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

var _ MerchantRepository = (*GormRepository)(nil)

func (r *GormRepository) CreateMerchant(ctx context.Context, value *Merchant) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(value).Error
	})
}

func (r *GormRepository) FindMerchant(ctx context.Context, publicID string) (Merchant, error) {
	var value Merchant
	err := r.db.WithContext(ctx).Where("merchant_public_id = ?", publicID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) TransitionMerchantStatus(
	ctx context.Context,
	publicID string,
	ownerUserID *uint64,
	expectedRowVersion uint64,
	from MerchantStatus,
	to MerchantStatus,
	updatedAt time.Time,
) (Merchant, error) {
	var result Merchant
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("merchant_public_id = ?", publicID)
		if ownerUserID != nil {
			query = query.Where("owner_user_id = ?", *ownerUserID)
		}
		if err := query.First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.Status == to {
			return nil
		}

		update := tx.Model(&Merchant{}).
			Where("id = ? AND row_version = ? AND status = ?", result.ID, expectedRowVersion, from)
		if ownerUserID != nil {
			update = update.Where("owner_user_id = ?", *ownerUserID)
		}
		update = update.Updates(map[string]any{
			"status": to, "updated_at": updatedAt,
			"row_version": gorm.Expr("row_version + 1"),
		})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		result.Status = to
		result.RowVersion++
		result.UpdatedAt = updatedAt
		return nil
	})
	return result, err
}

func normalizeNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

// G-2 Catalog methods extend the same Repository boundary without changing
// any G-1 Merchant method or behavior.
func (r *GormRepository) CreateCategory(ctx context.Context, value *DishCategory) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(value).Error
	})
}

func (r *GormRepository) FindCategory(ctx context.Context, publicID string) (DishCategory, error) {
	var value DishCategory
	err := r.db.WithContext(ctx).Where("category_public_id = ?", publicID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) RenameCategory(ctx context.Context, publicID, merchantPublicID string, expectedRowVersion uint64, name string, updatedAt time.Time) (DishCategory, error) {
	return r.mutateCategory(ctx, publicID, merchantPublicID, expectedRowVersion, updatedAt,
		func(value DishCategory) bool { return value.Name == name },
		map[string]any{"name": name},
		func(value *DishCategory) { value.Name = name },
	)
}

func (r *GormRepository) ReorderCategory(ctx context.Context, publicID, merchantPublicID string, expectedRowVersion uint64, sortOrder int32, updatedAt time.Time) (DishCategory, error) {
	return r.mutateCategory(ctx, publicID, merchantPublicID, expectedRowVersion, updatedAt,
		func(value DishCategory) bool { return value.SortOrder == sortOrder },
		map[string]any{"sort_order": sortOrder},
		func(value *DishCategory) { value.SortOrder = sortOrder },
	)
}

func (r *GormRepository) mutateCategory(ctx context.Context, publicID, merchantPublicID string, expectedRowVersion uint64, updatedAt time.Time, idempotent func(DishCategory) bool, updates map[string]any, apply func(*DishCategory)) (DishCategory, error) {
	var result DishCategory
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("category_public_id = ? AND merchant_public_id = ?", publicID, merchantPublicID).
			First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if idempotent(result) {
			return nil
		}
		updates["updated_at"] = updatedAt
		updates["row_version"] = gorm.Expr("row_version + 1")
		update := tx.Model(&DishCategory{}).
			Where("id = ? AND merchant_public_id = ? AND row_version = ?", result.ID, merchantPublicID, expectedRowVersion).
			Updates(updates)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		apply(&result)
		result.RowVersion++
		result.UpdatedAt = updatedAt
		return nil
	})
	return result, err
}

func (r *GormRepository) CreateDish(ctx context.Context, value *Dish) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(value).Error
	})
}

func (r *GormRepository) FindDish(ctx context.Context, publicID string) (Dish, error) {
	var value Dish
	err := r.db.WithContext(ctx).Where("dish_public_id = ?", publicID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) UpdateDish(ctx context.Context, publicID, merchantPublicID string, expectedRowVersion uint64, expectedStatus DishStatus, details dishDetails, updatedAt time.Time) (Dish, error) {
	var result Dish
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("dish_public_id = ? AND merchant_public_id = ?", publicID, merchantPublicID).
			First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if sameDishDetails(result, details) {
			return nil
		}
		update := tx.Model(&Dish{}).
			Where("id = ? AND merchant_public_id = ? AND row_version = ? AND status = ?", result.ID, merchantPublicID, expectedRowVersion, expectedStatus).
			Updates(map[string]any{
				"category_public_id": details.CategoryPublicID, "name": details.Name,
				"description": details.Description, "price_cents": details.PriceCents,
				"image_asset_public_id": details.ImageAssetPublicID, "sort_order": details.SortOrder,
				"updated_at": updatedAt, "row_version": gorm.Expr("row_version + 1"),
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		applyDishDetails(&result, details)
		result.RowVersion++
		result.UpdatedAt = updatedAt
		return nil
	})
	return result, err
}

func (r *GormRepository) TransitionDishStatus(ctx context.Context, publicID, merchantPublicID string, expectedRowVersion uint64, from, to DishStatus, updatedAt time.Time) (Dish, error) {
	var result Dish
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("dish_public_id = ? AND merchant_public_id = ?", publicID, merchantPublicID).
			First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.Status == to {
			return nil
		}
		update := tx.Model(&Dish{}).
			Where("id = ? AND merchant_public_id = ? AND row_version = ? AND status = ?", result.ID, merchantPublicID, expectedRowVersion, from).
			Updates(map[string]any{"status": to, "updated_at": updatedAt, "row_version": gorm.Expr("row_version + 1")})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		result.Status = to
		result.RowVersion++
		result.UpdatedAt = updatedAt
		return nil
	})
	return result, err
}

func sameDishDetails(value Dish, details dishDetails) bool {
	return equalStringPointers(value.CategoryPublicID, details.CategoryPublicID) &&
		value.Name == details.Name && value.Description == details.Description &&
		value.PriceCents == details.PriceCents &&
		equalStringPointers(value.ImageAssetPublicID, details.ImageAssetPublicID) &&
		value.SortOrder == details.SortOrder
}

func equalStringPointers(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func applyDishDetails(value *Dish, details dishDetails) {
	value.CategoryPublicID = details.CategoryPublicID
	value.Name = details.Name
	value.Description = details.Description
	value.PriceCents = details.PriceCents
	value.ImageAssetPublicID = details.ImageAssetPublicID
	value.SortOrder = details.SortOrder
}

// G-3 Order methods extend the same Repository boundary without changing any
// G-1 Merchant or G-2 Catalog method or behavior.
func (r *GormRepository) CreateOrder(ctx context.Context, order *Order, items []OrderItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return ErrInvalidRecord
		}
		return tx.Create(&items).Error
	})
}

func (r *GormRepository) FindOrder(ctx context.Context, publicID string) (Order, error) {
	var value Order
	err := r.db.WithContext(ctx).Where("order_public_id = ?", publicID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) ListOrderItems(ctx context.Context, orderPublicID string) ([]OrderItem, error) {
	var values []OrderItem
	err := r.db.WithContext(ctx).Where("order_public_id = ?", orderPublicID).Order("id ASC").Find(&values).Error
	return values, err
}

func (r *GormRepository) TransitionOrderStatus(
	ctx context.Context,
	publicID string,
	expectedRowVersion uint64,
	from, to OrderStatus,
	payNo *string,
	riderUserID *uint64,
	updatedAt time.Time,
) (Order, error) {
	if !CanTransitionOrder(from, to) || to == OrderCompleted {
		return Order{}, ErrInvalidTransition
	}
	var result Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("order_public_id = ?", publicID).First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.Status == to {
			return validateIdempotentOrderTransition(result, to, payNo, riderUserID)
		}
		updates, err := buildOrderTransitionUpdates(to, payNo, riderUserID, updatedAt)
		if err != nil {
			return err
		}
		updates["status"] = to
		updates["updated_at"] = updatedAt
		updates["row_version"] = gorm.Expr("row_version + 1")
		update := tx.Model(&Order{}).
			Where("id = ? AND row_version = ? AND status = ?", result.ID, expectedRowVersion, from).
			Updates(updates)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		applyOrderTransition(&result, to, payNo, riderUserID, updatedAt)
		result.RowVersion++
		result.UpdatedAt = updatedAt
		return nil
	})
	return result, err
}

func buildOrderTransitionUpdates(to OrderStatus, payNo *string, riderUserID *uint64, at time.Time) (map[string]any, error) {
	updates := make(map[string]any)
	switch to {
	case OrderPaid:
		if payNo == nil || *payNo == "" {
			return nil, ErrInvalidRecord
		}
		updates["pay_no"] = *payNo
		updates["paid_at"] = at
	case OrderAccepted:
		updates["accepted_at"] = at
	case OrderPreparing:
		updates["preparing_at"] = at
	case OrderReadyForPickup:
		updates["ready_at"] = at
	case OrderDelivering:
		if riderUserID == nil || *riderUserID == 0 {
			return nil, ErrRiderRequired
		}
		updates["rider_user_id"] = *riderUserID
		updates["delivering_at"] = at
	case OrderCancelled:
		updates["cancelled_at"] = at
	case OrderRefunded:
		updates["refunded_at"] = at
	default:
		return nil, ErrInvalidTransition
	}
	return updates, nil
}

func validateIdempotentOrderTransition(value Order, to OrderStatus, payNo *string, riderUserID *uint64) error {
	switch to {
	case OrderPaid:
		if payNo == nil || value.PayNo == nil || *value.PayNo != *payNo {
			return ErrInvalidRecord
		}
	case OrderDelivering:
		if riderUserID == nil || value.RiderUserID == nil || *value.RiderUserID != *riderUserID {
			return ErrInvalidRecord
		}
	}
	return nil
}

func applyOrderTransition(value *Order, to OrderStatus, payNo *string, riderUserID *uint64, at time.Time) {
	value.Status = to
	switch to {
	case OrderPaid:
		value.PayNo = payNo
		value.PaidAt = &at
	case OrderAccepted:
		value.AcceptedAt = &at
	case OrderPreparing:
		value.PreparingAt = &at
	case OrderReadyForPickup:
		value.ReadyAt = &at
	case OrderDelivering:
		value.RiderUserID = riderUserID
		value.DeliveringAt = &at
	case OrderCancelled:
		value.CancelledAt = &at
	case OrderRefunded:
		value.RefundedAt = &at
	}
}

func (r *GormRepository) CompleteDelivery(
	ctx context.Context,
	orderPublicID string,
	expectedRowVersion uint64,
	earningPublicID string,
	updatedAt time.Time,
) (Order, error) {
	var result Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("order_public_id = ?", orderPublicID).First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.Status == OrderCompleted {
			var existing RiderEarningRecord
			if err := tx.Where("order_public_id = ?", orderPublicID).First(&existing).Error; err != nil {
				return ErrInvalidRecord
			}
			return nil
		}
		if result.Status != OrderDelivering || result.RowVersion != expectedRowVersion {
			return ErrConcurrentUpdate
		}
		if result.RiderUserID == nil || *result.RiderUserID == 0 {
			return ErrRiderRequired
		}
		var config FeeConfig
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_scope = ?", "global").First(&config).Error; err != nil {
			return normalizeNotFound(err)
		}
		if !config.Valid() {
			return ErrInvalidRecord
		}
		update := tx.Model(&Order{}).
			Where("id = ? AND row_version = ? AND status = ?", result.ID, expectedRowVersion, OrderDelivering).
			Updates(map[string]any{
				"status": OrderCompleted, "completed_at": updatedAt, "updated_at": updatedAt,
				"row_version": gorm.Expr("row_version + 1"),
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		earning := RiderEarningRecord{
			EarningPublicID: earningPublicID, OrderPublicID: orderPublicID, RiderUserID: *result.RiderUserID,
			RiderFeeCents: config.RiderFeeCents, PlatformCutCents: config.PlatformCutCents,
			SettlementStatus: SettlementPending, RowVersion: 1, CreatedAt: updatedAt, UpdatedAt: updatedAt,
		}
		if err := earning.ValidateNew(); err != nil {
			return err
		}
		if err := tx.Create(&earning).Error; err != nil {
			return err
		}
		result.Status = OrderCompleted
		result.CompletedAt = &updatedAt
		result.RowVersion++
		result.UpdatedAt = updatedAt
		return nil
	})
	return result, err
}

func (r *GormRepository) GetActiveFeeConfig(ctx context.Context) (FeeConfig, error) {
	var value FeeConfig
	err := r.db.WithContext(ctx).Where("config_scope = ?", "global").First(&value).Error
	if err != nil {
		return value, normalizeNotFound(err)
	}
	if !value.Valid() {
		return FeeConfig{}, ErrInvalidRecord
	}
	return value, nil
}

func (r *GormRepository) UpdateFeeConfig(
	ctx context.Context,
	riderFeeCents, platformCutCents, expectedRowVersion uint64,
	updatedAt time.Time,
) (FeeConfig, error) {
	var result FeeConfig
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_scope = ?", "global").First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.RiderFeeCents == riderFeeCents && result.PlatformCutCents == platformCutCents {
			return nil
		}
		update := tx.Model(&FeeConfig{}).
			Where("id = ? AND row_version = ? AND config_scope = ?", result.ID, expectedRowVersion, "global").
			Updates(map[string]any{
				"rider_fee_cents": riderFeeCents, "platform_cut_cents": platformCutCents,
				"updated_at": updatedAt, "row_version": gorm.Expr("row_version + 1"),
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		result.RiderFeeCents = riderFeeCents
		result.PlatformCutCents = platformCutCents
		result.RowVersion++
		result.UpdatedAt = updatedAt
		return nil
	})
	return result, err
}

func (r *GormRepository) FindEarningByOrder(ctx context.Context, orderPublicID string) (RiderEarningRecord, error) {
	var value RiderEarningRecord
	err := r.db.WithContext(ctx).Where("order_public_id = ?", orderPublicID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) MarkSettled(
	ctx context.Context,
	earningPublicID string,
	expectedRowVersion uint64,
	updatedAt time.Time,
) (RiderEarningRecord, error) {
	var result RiderEarningRecord
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("earning_public_id = ?", earningPublicID).First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.SettlementStatus == SettlementSettled {
			return nil
		}
		if !CanTransitionSettlement(result.SettlementStatus, SettlementSettled) {
			return ErrInvalidTransition
		}
		update := tx.Model(&RiderEarningRecord{}).
			Where("id = ? AND row_version = ? AND settlement_status = ?", result.ID, expectedRowVersion, SettlementPending).
			Updates(map[string]any{
				"settlement_status": SettlementSettled, "settled_at": updatedAt, "updated_at": updatedAt,
				"row_version": gorm.Expr("row_version + 1"),
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		result.SettlementStatus = SettlementSettled
		result.SettledAt = &updatedAt
		result.RowVersion++
		result.UpdatedAt = updatedAt
		return nil
	})
	return result, err
}
