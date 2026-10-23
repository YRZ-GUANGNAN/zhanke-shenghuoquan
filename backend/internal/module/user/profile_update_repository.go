package user

import (
	"context"
	"errors"

	"campus-circle/backend/internal/pkg/dbtx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *GormRepository) WithLockedProfile(ctx context.Context, id uint, change func(context.Context, User) (Profile, error)) (User, error) {
	var current User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		value, err := change(dbtx.WithDB(ctx, tx), current)
		if err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", id).Updates(map[string]any{
			"nickname": value.Nickname, "school": value.School, "campus": value.Campus,
			"avatar": value.Avatar, "avatar_asset_id": value.AvatarAssetID,
		}).Error; err != nil {
			return err
		}
		current.Nickname, current.School, current.Campus = value.Nickname, value.School, value.Campus
		current.Avatar, current.AvatarAssetID = value.Avatar, value.AvatarAssetID
		return nil
	})
	if err != nil {
		return User{}, err
	}
	return current, nil
}
