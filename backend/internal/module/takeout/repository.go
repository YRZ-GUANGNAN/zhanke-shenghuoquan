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
