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

// G-5 Rider methods extend the same Repository boundary without changing any
// G-1/G-2/G-3 method or behavior.
func (r *GormRepository) CreateRider(ctx context.Context, value *Rider) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(value).Error
	})
}

func (r *GormRepository) FindRider(ctx context.Context, riderUserID uint64) (Rider, error) {
	var value Rider
	err := r.db.WithContext(ctx).Where("rider_user_id = ?", riderUserID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) FindRiderForUpdate(ctx context.Context, riderUserID uint64) (Rider, error) {
	var value Rider
	err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("rider_user_id = ?", riderUserID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) TransitionRiderStatus(
	ctx context.Context,
	riderUserID, expectedRowVersion uint64,
	from, to RiderStatus,
	updatedAt time.Time,
) (Rider, error) {
	var result Rider
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("rider_user_id = ?", riderUserID).First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.Status == to {
			return nil
		}
		update := tx.Model(&Rider{}).
			Where("id = ? AND row_version = ? AND status = ?", result.ID, expectedRowVersion, from).
			Updates(map[string]any{
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

func (r *GormRepository) GetRiderConfig(ctx context.Context) (RiderConfig, error) {
	var value RiderConfig
	err := r.db.WithContext(ctx).Where("config_scope = ?", "global").First(&value).Error
	if err != nil {
		return value, normalizeNotFound(err)
	}
	if !value.Valid() {
		return RiderConfig{}, ErrInvalidRecord
	}
	return value, nil
}

func (r *GormRepository) UpdateRiderConfig(
	ctx context.Context,
	maxInFlight uint32,
	expectedRowVersion uint64,
	updatedAt time.Time,
) (RiderConfig, error) {
	var result RiderConfig
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_scope = ?", "global").First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.MaxInFlightPerRider == maxInFlight {
			return nil
		}
		update := tx.Model(&RiderConfig{}).
			Where("id = ? AND config_scope = ? AND row_version = ?", result.ID, "global", expectedRowVersion).
			Updates(map[string]any{
				"max_in_flight_per_rider": maxInFlight, "updated_at": updatedAt,
				"row_version": gorm.Expr("row_version + 1"),
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		result.MaxInFlightPerRider = maxInFlight
		result.RowVersion++
		result.UpdatedAt = updatedAt
		return nil
	})
	return result, err
}

func (r *GormRepository) ListOrdersByStatus(ctx context.Context, status OrderStatus) ([]Order, error) {
	var values []Order
	err := r.db.WithContext(ctx).Where("status = ?", status).Order("created_at ASC, id ASC").Find(&values).Error
	return values, err
}

func (r *GormRepository) ClaimOrder(
	ctx context.Context,
	riderUserID uint64,
	orderPublicID string,
	expectedOrderRowVersion uint64,
	fulfillmentPublicID string,
	orderOps RiderOrderOps,
	createdAt time.Time,
) (Fulfillment, error) {
	var result Fulfillment
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rider Rider
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("rider_user_id = ?", riderUserID).First(&rider).Error; err != nil {
			return normalizeNotFound(err)
		}

		var existing Fulfillment
		existingErr := tx.Where("order_public_id = ?", orderPublicID).First(&existing).Error
		switch {
		case existingErr == nil:
			if existing.RiderUserID != riderUserID {
				return ErrOrderAlreadyClaimed
			}
			result = existing
			return nil
		case !errors.Is(existingErr, gorm.ErrRecordNotFound):
			return existingErr
		}
		if rider.Status != RiderActive {
			return ErrRiderUnavailable
		}

		var config RiderConfig
		if err := tx.Where("config_scope = ?", "global").First(&config).Error; err != nil {
			return normalizeNotFound(err)
		}
		if !config.Valid() {
			return ErrInvalidRecord
		}
		var inFlight int64
		if err := tx.Model(&Fulfillment{}).
			Where("rider_user_id = ? AND status <> ?", riderUserID, FulfillmentDelivered).
			Count(&inFlight).Error; err != nil {
			return err
		}
		if uint64(inFlight) >= uint64(config.MaxInFlightPerRider) {
			return ErrInFlightLimit
		}

		txRepository := NewRepository(tx)
		if _, err := orderOps.StartDelivering(ctx, txRepository, orderPublicID, expectedOrderRowVersion, riderUserID); err != nil {
			if errors.Is(err, ErrConcurrentUpdate) || errors.Is(err, ErrInvalidRecord) {
				return ErrOrderAlreadyClaimed
			}
			return err
		}
		result = Fulfillment{
			FulfillmentPublicID: fulfillmentPublicID, OrderPublicID: orderPublicID, RiderUserID: riderUserID,
			Status: FulfillmentAssigned, AssignedAt: &createdAt, RowVersion: 1,
			CreatedAt: createdAt, UpdatedAt: createdAt,
		}
		if err := result.ValidateNew(); err != nil {
			return err
		}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		return nil
	})
	return result, err
}

func (r *GormRepository) TransitionFulfillmentStatus(
	ctx context.Context,
	fulfillmentPublicID string,
	expectedRowVersion uint64,
	from, to FulfillmentStatus,
	updatedAt time.Time,
) (Fulfillment, error) {
	if !CanTransitionFulfillment(from, to) || to == FulfillmentDelivered {
		return Fulfillment{}, ErrInvalidTransition
	}
	var result Fulfillment
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("fulfillment_public_id = ?", fulfillmentPublicID).First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.Status == to {
			return nil
		}
		updates, err := fulfillmentTransitionUpdates(to, updatedAt)
		if err != nil {
			return err
		}
		updates["status"] = to
		updates["updated_at"] = updatedAt
		updates["row_version"] = gorm.Expr("row_version + 1")
		update := tx.Model(&Fulfillment{}).
			Where("id = ? AND row_version = ? AND status = ?", result.ID, expectedRowVersion, from).
			Updates(updates)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		applyFulfillmentTransition(&result, to, updatedAt)
		result.RowVersion++
		result.UpdatedAt = updatedAt
		return nil
	})
	return result, err
}

func fulfillmentTransitionUpdates(to FulfillmentStatus, at time.Time) (map[string]any, error) {
	switch to {
	case FulfillmentToStore:
		return map[string]any{"to_store_at": at}, nil
	case FulfillmentPicked:
		return map[string]any{"picked_at": at}, nil
	case FulfillmentToUser:
		return map[string]any{"to_user_at": at}, nil
	case FulfillmentDelivered:
		return map[string]any{"delivered_at": at}, nil
	default:
		return nil, ErrInvalidTransition
	}
}

func applyFulfillmentTransition(value *Fulfillment, to FulfillmentStatus, at time.Time) {
	value.Status = to
	switch to {
	case FulfillmentToStore:
		value.ToStoreAt = &at
	case FulfillmentPicked:
		value.PickedAt = &at
	case FulfillmentToUser:
		value.ToUserAt = &at
	case FulfillmentDelivered:
		value.DeliveredAt = &at
	}
}

func (r *GormRepository) CompleteFulfillment(
	ctx context.Context,
	fulfillmentPublicID string,
	expectedRowVersion uint64,
	orderOps RiderOrderOps,
	updatedAt time.Time,
) (Fulfillment, error) {
	var result Fulfillment
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("fulfillment_public_id = ?", fulfillmentPublicID).First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		txRepository := NewRepository(tx)
		if result.Status == FulfillmentDelivered {
			_, err := orderOps.CompleteDelivery(ctx, txRepository, result.OrderPublicID)
			return err
		}
		updates, err := fulfillmentTransitionUpdates(FulfillmentDelivered, updatedAt)
		if err != nil {
			return err
		}
		updates["status"] = FulfillmentDelivered
		updates["updated_at"] = updatedAt
		updates["row_version"] = gorm.Expr("row_version + 1")
		update := tx.Model(&Fulfillment{}).
			Where("id = ? AND row_version = ? AND status = ?", result.ID, expectedRowVersion, FulfillmentToUser).
			Updates(updates)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		if _, err := orderOps.CompleteDelivery(ctx, txRepository, result.OrderPublicID); err != nil {
			return err
		}
		applyFulfillmentTransition(&result, FulfillmentDelivered, updatedAt)
		result.RowVersion++
		result.UpdatedAt = updatedAt
		return nil
	})
	return result, err
}

func (r *GormRepository) FindFulfillmentByOrder(ctx context.Context, orderPublicID string) (Fulfillment, error) {
	var value Fulfillment
	err := r.db.WithContext(ctx).Where("order_public_id = ?", orderPublicID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) CreateWithdrawRequest(ctx context.Context, value *WithdrawRequest) error {
	return r.db.WithContext(ctx).Create(value).Error
}

func (r *GormRepository) FindWithdrawByID(ctx context.Context, publicID string) (WithdrawRequest, error) {
	var value WithdrawRequest
	err := r.db.WithContext(ctx).Where("withdraw_public_id = ?", publicID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) ListWithdrawRequests(
	ctx context.Context,
	status WithdrawStatus,
	page, size int,
) ([]WithdrawRequest, int64, error) {
	query := r.db.WithContext(ctx).Model(&WithdrawRequest{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []WithdrawRequest
	err := query.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) MarkWithdrawPaid(
	ctx context.Context,
	publicID string,
	expectedRowVersion uint64,
	updatedAt time.Time,
) (WithdrawRequest, error) {
	var value WithdrawRequest
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("withdraw_public_id = ?", publicID).First(&value).Error; err != nil {
			return normalizeNotFound(err)
		}
		if value.Status == WithdrawPaid {
			return nil
		}
		if !CanTransitionWithdraw(value.Status, WithdrawPaid) {
			return ErrInvalidTransition
		}
		result := tx.Model(&WithdrawRequest{}).
			Where("id = ? AND row_version = ? AND status = ?", value.ID, expectedRowVersion, WithdrawPending).
			Updates(map[string]any{
				"status": WithdrawPaid, "paid_at": updatedAt, "updated_at": updatedAt,
				"row_version": gorm.Expr("row_version + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		var earnings []RiderEarningRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("rider_user_id = ? AND settlement_status = ?", value.RiderUserID, SettlementPending).
			Order("id ASC").Find(&earnings).Error; err != nil {
			return err
		}
		for _, earning := range earnings {
			settled := tx.Model(&RiderEarningRecord{}).
				Where("id = ? AND row_version = ? AND settlement_status = ?", earning.ID, earning.RowVersion, SettlementPending).
				Updates(map[string]any{
					"settlement_status": SettlementSettled, "settled_at": updatedAt, "updated_at": updatedAt,
					"row_version": gorm.Expr("row_version + 1"),
				})
			if settled.Error != nil {
				return settled.Error
			}
			if settled.RowsAffected != 1 {
				return ErrConcurrentUpdate
			}
		}
		value.Status = WithdrawPaid
		value.PaidAt = &updatedAt
		value.RowVersion++
		value.UpdatedAt = updatedAt
		return nil
	})
	return value, err
}

func (r *GormRepository) RejectWithdraw(
	ctx context.Context,
	publicID string,
	expectedRowVersion uint64,
	note string,
	updatedAt time.Time,
) (WithdrawRequest, error) {
	var value WithdrawRequest
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("withdraw_public_id = ?", publicID).First(&value).Error; err != nil {
			return normalizeNotFound(err)
		}
		if value.Status == WithdrawRejected {
			return nil
		}
		if !CanTransitionWithdraw(value.Status, WithdrawRejected) {
			return ErrInvalidTransition
		}
		result := tx.Model(&WithdrawRequest{}).
			Where("id = ? AND row_version = ? AND status = ?", value.ID, expectedRowVersion, WithdrawPending).
			Updates(map[string]any{
				"status": WithdrawRejected, "note": note, "updated_at": updatedAt,
				"row_version": gorm.Expr("row_version + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		value.Status = WithdrawRejected
		value.Note = note
		value.RowVersion++
		value.UpdatedAt = updatedAt
		return nil
	})
	return value, err
}

// G-4 Payment methods extend the same Repository boundary without changing
// any G-1/G-2/G-3/G-5/G-6 method or behavior.
func (r *GormRepository) CreatePaymentAttempt(ctx context.Context, value *PaymentAttempt) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(value).Error
	})
}

func (r *GormRepository) FindPaymentAttempt(ctx context.Context, publicID string) (PaymentAttempt, error) {
	var value PaymentAttempt
	err := r.db.WithContext(ctx).Where("payment_public_id = ?", publicID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) FindPaymentAttemptByOutTradeNo(ctx context.Context, outTradeNo string) (PaymentAttempt, error) {
	var value PaymentAttempt
	err := r.db.WithContext(ctx).Where("out_trade_no = ?", outTradeNo).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) FindLatestPaymentAttemptByOrder(ctx context.Context, orderPublicID string, provider PaymentProvider) (PaymentAttempt, error) {
	var value PaymentAttempt
	err := r.db.WithContext(ctx).
		Where("order_public_id = ? AND provider = ?", orderPublicID, provider).
		Order("id DESC").First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) MarkPaymentAttemptPrepayIssued(
	ctx context.Context,
	paymentPublicID string,
	expectedRowVersion uint64,
	prepayID string,
	updatedAt time.Time,
) (PaymentAttempt, error) {
	var result PaymentAttempt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("payment_public_id = ?", paymentPublicID).First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.Status == PaymentPrepayIssued {
			if result.PrepayID == nil || *result.PrepayID != prepayID {
				return ErrInvalidRecord
			}
			return nil
		}
		update := tx.Model(&PaymentAttempt{}).
			Where("id = ? AND row_version = ? AND status = ?", result.ID, expectedRowVersion, PaymentCreated).
			Updates(map[string]any{
				"status": PaymentPrepayIssued, "prepay_id": prepayID,
				"updated_at": updatedAt, "row_version": gorm.Expr("row_version + 1"),
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		result.Status = PaymentPrepayIssued
		result.PrepayID = &prepayID
		result.RowVersion++
		result.UpdatedAt = updatedAt
		return nil
	})
	return result, err
}

func (r *GormRepository) CreatePaidPaymentAttemptAndMarkOrderPaid(
	ctx context.Context,
	attempt *PaymentAttempt,
	expectedOrderRowVersion uint64,
) (PaymentAttempt, Order, error) {
	var order Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("order_public_id = ?", attempt.OrderPublicID).First(&order).Error; err != nil {
			return normalizeNotFound(err)
		}
		if order.BuyerUserID != attempt.BuyerUserID {
			return ErrPaymentForbidden
		}
		if order.TotalAmountCents != attempt.AmountCents {
			return ErrPaymentAmount
		}
		if order.Status != OrderPendingPayment {
			return ErrPaymentNotPending
		}
		if err := tx.Create(attempt).Error; err != nil {
			return err
		}
		payNo := ""
		if attempt.PayNo != nil {
			payNo = *attempt.PayNo
		}
		updates, err := buildOrderTransitionUpdates(OrderPaid, &payNo, nil, *attempt.PaidAt)
		if err != nil {
			return err
		}
		updates["status"] = OrderPaid
		updates["updated_at"] = *attempt.PaidAt
		updates["row_version"] = gorm.Expr("row_version + 1")
		update := tx.Model(&Order{}).
			Where("id = ? AND row_version = ? AND status = ?", order.ID, expectedOrderRowVersion, OrderPendingPayment).
			Updates(updates)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		applyOrderTransition(&order, OrderPaid, &payNo, nil, *attempt.PaidAt)
		order.RowVersion++
		order.UpdatedAt = *attempt.PaidAt
		return nil
	})
	return *attempt, order, err
}

func (r *GormRepository) RecordPaymentNotificationAndMarkPaid(
	ctx context.Context,
	notification *PaymentNotification,
	outTradeNo string,
	transactionID string,
	payNo string,
	amountCents uint64,
	paidAt time.Time,
) (PaymentAttempt, Order, bool, error) {
	var (
		attempt   PaymentAttempt
		order     Order
		duplicate bool
	)
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		createNotification := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "body_sha256"}},
			DoNothing: true,
		}).Create(notification)
		if createNotification.Error != nil {
			return createNotification.Error
		}
		if createNotification.RowsAffected == 0 {
			duplicate = true
		}

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("out_trade_no = ?", outTradeNo).First(&attempt).Error; err != nil {
			return normalizeNotFound(err)
		}
		if attempt.AmountCents != amountCents {
			return ErrPaymentAmount
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("order_public_id = ?", attempt.OrderPublicID).First(&order).Error; err != nil {
			return normalizeNotFound(err)
		}
		if order.TotalAmountCents != amountCents {
			return ErrPaymentAmount
		}
		if attempt.Status == PaymentPaid {
			if attempt.PayNo == nil || *attempt.PayNo != payNo ||
				(attempt.WechatTransactionID != nil && *attempt.WechatTransactionID != transactionID) {
				return ErrInvalidRecord
			}
			return nil
		}
		if attempt.Status != PaymentCreated && attempt.Status != PaymentPrepayIssued {
			return ErrInvalidTransition
		}
		wechatTransactionID := transactionID
		updateAttempt := tx.Model(&PaymentAttempt{}).
			Where("id = ? AND row_version = ? AND status IN ?", attempt.ID, attempt.RowVersion, []PaymentStatus{PaymentCreated, PaymentPrepayIssued}).
			Updates(map[string]any{
				"status": PaymentPaid, "wechat_transaction_id": wechatTransactionID, "pay_no": payNo,
				"paid_at": paidAt, "updated_at": paidAt, "row_version": gorm.Expr("row_version + 1"),
			})
		if updateAttempt.Error != nil {
			return updateAttempt.Error
		}
		if updateAttempt.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		if order.Status == OrderPaid {
			if order.PayNo == nil || *order.PayNo != payNo {
				return ErrInvalidRecord
			}
		} else {
			if order.Status != OrderPendingPayment {
				return ErrPaymentNotPending
			}
			updates, err := buildOrderTransitionUpdates(OrderPaid, &payNo, nil, paidAt)
			if err != nil {
				return err
			}
			updates["status"] = OrderPaid
			updates["updated_at"] = paidAt
			updates["row_version"] = gorm.Expr("row_version + 1")
			updateOrder := tx.Model(&Order{}).
				Where("id = ? AND row_version = ? AND status = ?", order.ID, order.RowVersion, OrderPendingPayment).
				Updates(updates)
			if updateOrder.Error != nil {
				return updateOrder.Error
			}
			if updateOrder.RowsAffected != 1 {
				return ErrConcurrentUpdate
			}
			applyOrderTransition(&order, OrderPaid, &payNo, nil, paidAt)
			order.RowVersion++
			order.UpdatedAt = paidAt
		}
		attempt.Status = PaymentPaid
		attempt.WechatTransactionID = &wechatTransactionID
		attempt.PayNo = &payNo
		attempt.PaidAt = &paidAt
		attempt.RowVersion++
		attempt.UpdatedAt = paidAt
		return nil
	})
	return attempt, order, duplicate, err
}
