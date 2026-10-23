package user

import (
	"context"
	"errors"

	"campus-circle/backend/internal/pkg/dbtx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *GormRepository) WithLockedCover(ctx context.Context, id uint, change func(context.Context, User) (ProfileCover, error)) (User, error) {
	var current User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		cover, err := change(dbtx.WithDB(ctx, tx), current)
		if err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", id).Updates(map[string]any{"cover": cover.URL, "cover_asset_id": cover.AssetID}).Error; err != nil {
			return err
		}
		current.Cover, current.CoverAssetID = cover.URL, cover.AssetID
		return nil
	})
	if err != nil {
		return User{}, err
	}
	return current, nil
}
