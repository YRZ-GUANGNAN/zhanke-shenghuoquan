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
