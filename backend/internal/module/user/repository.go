package user

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrNotFound = errors.New("user not found")

type settingsRepository interface {
	GetOrCreateSettings(context.Context, uint) (UserSettings, error)
	UpdateSettings(context.Context, uint, UserSettings) (UserSettings, error)
	UpdatePhone(context.Context, uint, string) error
}

type blocksRepository interface {
	ListBlocks(context.Context, uint) ([]BlockedUserView, error)
	AddBlock(context.Context, uint, uint) error
	RemoveBlock(context.Context, uint, uint) error
	CountBlocks(context.Context, uint) (int64, error)
}

type feedbackRepository interface {
	CreateFeedback(context.Context, *Feedback, []string) error
	DeleteFeedback(context.Context, uint, uint) error
	ListFeedbacks(context.Context, int, int) ([]Feedback, int64, error)
}

type Repository interface {
	FindByOpenID(context.Context, string) (User, error)
	FindByID(context.Context, uint) (User, error)
	FindProfiles(context.Context, []uint) (map[uint]Profile, error)
	Create(context.Context, *User) error
	UpdateProfile(context.Context, uint, Profile) error
	UpdateCurrentIdentity(context.Context, uint, Identity) error
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

func (r *GormRepository) UpdateCurrentIdentity(ctx context.Context, id uint, identity Identity) error {
	result := r.db.WithContext(ctx).Model(&User{}).Where("id = ?", id).Update("current_identity", identity)
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

func defaultUserSettings(userID uint) UserSettings {
	return UserSettings{UserID: userID, OrderNotice: true, SocialNotice: true, Personalized: true}
}

func (r *GormRepository) GetOrCreateSettings(ctx context.Context, userID uint) (UserSettings, error) {
	value := defaultUserSettings(userID)
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoNothing: true}).Create(&value).Error; err != nil {
		return UserSettings{}, err
	}
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&value).Error; err != nil {
		return UserSettings{}, err
	}
	return value, nil
}

func (r *GormRepository) UpdateSettings(ctx context.Context, userID uint, value UserSettings) (UserSettings, error) {
	if _, err := r.GetOrCreateSettings(ctx, userID); err != nil {
		return UserSettings{}, err
	}
	updates := map[string]any{
		"order_notice": value.OrderNotice, "social_notice": value.SocialNotice,
		"quiet_hours": value.QuietHours, "personalized": value.Personalized,
	}
	if err := r.db.WithContext(ctx).Model(&UserSettings{}).Where("user_id = ?", userID).Updates(updates).Error; err != nil {
		return UserSettings{}, err
	}
	return r.GetOrCreateSettings(ctx, userID)
}

func (r *GormRepository) UpdatePhone(ctx context.Context, userID uint, phone string) error {
	result := r.db.WithContext(ctx).Model(&User{}).Where("id = ? AND status <> ?", userID, StatusClosed).Update("phone", phone)
	if result.Error == nil && result.RowsAffected == 0 {
		return ErrNotFound
	}
	return result.Error
}

func (r *GormRepository) ListBlocks(ctx context.Context, userID uint) ([]BlockedUserView, error) {
	var rows []struct {
		UserID    uint
		Nickname  string
		Avatar    string
		Status    Status
		CreatedAt time.Time
	}
	err := r.db.WithContext(ctx).Table("user_blocks AS b").
		Select("u.id AS user_id, u.nickname, u.avatar, u.status, b.created_at").
		Joins("JOIN users AS u ON u.id = b.blocked_user_id").
		Where("b.user_id = ?", userID).Order("b.created_at DESC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	values := make([]BlockedUserView, 0, len(rows))
	for _, row := range rows {
		if row.Status == StatusClosed {
			row.Nickname, row.Avatar = "账号已注销", ""
		}
		values = append(values, BlockedUserView{UserID: row.UserID, Nickname: row.Nickname, Avatar: row.Avatar, CreatedAt: row.CreatedAt})
	}
	return values, nil
}

func (r *GormRepository) AddBlock(ctx context.Context, userID, blockedUserID uint) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&User{}).Where("id = ? AND status <> ?", blockedUserID, StatusClosed).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&UserBlock{UserID: userID, BlockedUserID: blockedUserID}).Error
}

func (r *GormRepository) RemoveBlock(ctx context.Context, userID, blockedUserID uint) error {
	return r.db.WithContext(ctx).Where("user_id = ? AND blocked_user_id = ?", userID, blockedUserID).Delete(&UserBlock{}).Error
}

func (r *GormRepository) CountBlocks(ctx context.Context, userID uint) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&UserBlock{}).Where("user_id = ?", userID).Count(&total).Error
	return total, err
}

func (r *GormRepository) CreateFeedback(ctx context.Context, value *Feedback, assetIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Images").Create(value).Error; err != nil {
			return err
		}
		for index, assetID := range assetIDs {
			image := FeedbackImage{FeedbackID: value.ID, AssetPublicID: assetID, Slot: uint16(index)}
			if err := tx.Create(&image).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *GormRepository) DeleteFeedback(ctx context.Context, userID, feedbackID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("feedback_id = ?", feedbackID).Delete(&FeedbackImage{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ? AND user_id = ?", feedbackID, userID).Delete(&Feedback{}).Error
	})
}

func (r *GormRepository) ListFeedbacks(ctx context.Context, page, size int) ([]Feedback, int64, error) {
	var values []Feedback
	var total int64
	query := r.db.WithContext(ctx).Model(&Feedback{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Preload("Images").Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}
