package notification

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func (r *GormRepository) FindOfficialAccountBinding(ctx context.Context, userID uint64, appID string) (OfficialAccountBinding, error) {
	if r == nil || r.db == nil || userID == 0 || appID == "" {
		return OfficialAccountBinding{}, ErrInvalidRecord
	}
	var binding OfficialAccountBinding
	err := r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Discard}).
		Where("user_id = ? AND app_id = ?", userID, appID).First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OfficialAccountBinding{}, ErrOfficialAccountNotBound
	}
	return binding, err
}

func (r *GormRepository) UpsertOfficialAccountBinding(ctx context.Context, binding OfficialAccountBinding, now time.Time) (OfficialAccountBinding, error) {
	if r == nil || r.db == nil || binding.UserID == 0 || binding.AppID == "" || binding.OpenID == "" ||
		!validOfficialFollowState(binding.FollowState) || now.IsZero() {
		return OfficialAccountBinding{}, ErrInvalidRecord
	}
	var result OfficialAccountBinding
	err := r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Discard}).Transaction(func(tx *gorm.DB) error {
		var other OfficialAccountBinding
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("app_id = ? AND openid = ? AND user_id <> ?", binding.AppID, binding.OpenID, binding.UserID).
			First(&other).Error
		if err == nil {
			return ErrIdempotencyConflict
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var existing OfficialAccountBinding
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND app_id = ?", binding.UserID, binding.AppID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			binding.FollowState = "unknown"
			binding.RowVersion = 1
			binding.BoundAt = now
			binding.UpdatedAt = now
			if err := tx.Create(&binding).Error; err != nil {
				return err
			}
			result = binding
			return nil
		}
		if err != nil {
			return err
		}
		updates := map[string]any{
			"openid": binding.OpenID, "follow_state": "unknown", "bound_at": now,
			"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		}
		if err := tx.Model(&OfficialAccountBinding{}).Where("user_id = ? AND app_id = ?", binding.UserID, binding.AppID).
			Updates(updates).Error; err != nil {
			return err
		}
		return tx.Where("user_id = ? AND app_id = ?", binding.UserID, binding.AppID).First(&result).Error
	})
	return result, err
}

func (r *GormRepository) UpdateOfficialAccountFollowState(ctx context.Context, userID uint64, appID, state string, now time.Time) error {
	if r == nil || r.db == nil || userID == 0 || appID == "" || !validOfficialFollowState(state) || now.IsZero() {
		return ErrInvalidRecord
	}
	result := r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Discard}).Model(&OfficialAccountBinding{}).
		Where("user_id = ? AND app_id = ?", userID, appID).
		Updates(map[string]any{"follow_state": state, "row_version": gorm.Expr("row_version + 1"), "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOfficialAccountNotBound
	}
	return nil
}

func (r *GormRepository) UnbindOfficialAccount(ctx context.Context, userID uint64, appID string) error {
	if r == nil || r.db == nil || userID == 0 || appID == "" {
		return ErrInvalidRecord
	}
	return r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Discard}).
		Where("user_id = ? AND app_id = ?", userID, appID).Delete(&OfficialAccountBinding{}).Error
}

func (r *GormRepository) CountOfficialAccountBindings(ctx context.Context, appID, followState string) (int64, error) {
	if r == nil || r.db == nil || appID == "" || (followState != "" && !validOfficialFollowState(followState)) {
		return 0, ErrInvalidRecord
	}
	query := r.db.WithContext(ctx).Model(&OfficialAccountBinding{}).Where("app_id = ?", appID)
	if followState != "" {
		query = query.Where("follow_state = ?", followState)
	}
	var count int64
	err := query.Count(&count).Error
	return count, err
}

func validOfficialFollowState(state string) bool {
	return state == "unknown" || state == "following" || state == "not_following"
}
