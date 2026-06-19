package user

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrNotFound = errors.New("user not found")

type Repository interface {
	FindByOpenID(context.Context, string) (User, error)
	FindByID(context.Context, uint) (User, error)
	FindProfiles(context.Context, []uint) (map[uint]Profile, error)
	Create(context.Context, *User) error
	UpdateProfile(context.Context, uint, Profile) error
	List(context.Context, int, int) ([]User, int64, error)
	UpdateStatusAndRevoke(context.Context, uint, Status, bool) error
	UpdateRoleAndRevoke(context.Context, uint, Role, bool) error
	Count(context.Context) (total int64, today int64, err error)
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) FindByOpenID(ctx context.Context, openID string) (User, error) {
	var value User
	err := r.db.WithContext(ctx).Where("open_id = ?", openID).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, ErrNotFound
	}
	return value, err
}

func (r *GormRepository) FindByID(ctx context.Context, id uint) (User, error) {
	var value User
	err := r.db.WithContext(ctx).First(&value, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, ErrNotFound
	}
	return value, err
}

func (r *GormRepository) FindProfiles(ctx context.Context, ids []uint) (map[uint]Profile, error) {
	profiles := make(map[uint]Profile, len(ids))
	if len(ids) == 0 {
		return profiles, nil
	}
	var users []User
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	for _, value := range users {
		profiles[value.ID] = value.Profile()
	}
	return profiles, nil
}

func (r *GormRepository) Create(ctx context.Context, value *User) error {
	return r.db.WithContext(ctx).Create(value).Error
}

func (r *GormRepository) UpdateProfile(ctx context.Context, id uint, profile Profile) error {
	result := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Updates(map[string]any{"nickname": profile.Nickname, "avatar": profile.Avatar, "avatar_asset_id": profile.AvatarAssetID, "school": profile.School, "campus": profile.Campus})
	if result.Error == nil && result.RowsAffected == 0 {
		return ErrNotFound
	}
	return result.Error
}

func (r *GormRepository) List(ctx context.Context, page, pageSize int) ([]User, int64, error) {
	var values []User
	var total int64
	query := r.db.WithContext(ctx).Model(&User{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) UpdateStatusAndRevoke(ctx context.Context, id uint, status Status, protectLastSuperAdmin bool) error {
	return r.updateAdministrativeAccount(ctx, protectLastSuperAdmin, func(tx *gorm.DB) error {
		result := tx.Model(&User{}).Where("id = ?", id).Updates(map[string]any{
			"status":        status,
			"token_version": gorm.Expr("token_version + 1"),
		})
		if result.Error == nil && result.RowsAffected == 0 {
			return ErrNotFound
		}
		return result.Error
	})
}

func (r *GormRepository) UpdateRoleAndRevoke(ctx context.Context, id uint, role Role, protectLastSuperAdmin bool) error {
	return r.updateAdministrativeAccount(ctx, protectLastSuperAdmin, func(tx *gorm.DB) error {
		result := tx.Model(&User{}).Where("id = ?", id).Updates(map[string]any{
			"role":          role,
			"token_version": gorm.Expr("token_version + 1"),
		})
		if result.Error == nil && result.RowsAffected == 0 {
			return ErrNotFound
		}
		return result.Error
	})
}

func (r *GormRepository) updateAdministrativeAccount(ctx context.Context, protectLastSuperAdmin bool, update func(*gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if protectLastSuperAdmin {
			var active []User
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("role = ? AND status = ?", RoleSuperAdmin, StatusActive).
				Find(&active).Error; err != nil {
				return err
			}
			if len(active) <= 1 {
				return ErrLastActiveSuperAdmin
			}
		}
		return update(tx)
	})
}

func (r *GormRepository) Count(ctx context.Context) (int64, int64, error) {
	var total, today int64
	if err := r.db.WithContext(ctx).Model(&User{}).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	start := time.Now().Truncate(24 * time.Hour)
	err := r.db.WithContext(ctx).Model(&User{}).Where("created_at >= ?", start).Count(&today).Error
	return total, today, err
}
