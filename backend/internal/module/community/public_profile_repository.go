package community

import (
	"context"

	"campus-circle/backend/internal/module/admin/review"
	"gorm.io/gorm"
)

func (r *GormRepository) PublicProfileStats(ctx context.Context, viewerID, userID uint) (PublicProfileView, error) {
	value := PublicProfileView{}
	queries := []struct {
		model any
		where string
		dest  *int64
	}{
		{&UserFollow{}, "follower_id = ?", &value.FollowingCount},
		{&UserFollow{}, "followee_id = ?", &value.FollowerCount},
	}
	for _, query := range queries {
		if err := r.db.WithContext(ctx).Model(query.model).Where(query.where, userID).Count(query.dest).Error; err != nil {
			return PublicProfileView{}, err
		}
	}
	if err := r.db.WithContext(ctx).Model(&Post{}).
		Where("user_id = ? AND review_status = ?", userID, review.StatusApproved).
		Select("COALESCE(SUM(liked_count), 0)").Scan(&value.LikedCount).Error; err != nil {
		return PublicProfileView{}, err
	}
	if viewerID != userID {
		var count int64
		if err := r.db.WithContext(ctx).Model(&UserFollow{}).
			Where("follower_id = ? AND followee_id = ?", viewerID, userID).Count(&count).Error; err != nil {
			return PublicProfileView{}, err
		}
		value.IsFollowing = count > 0
	}
	return value, nil
}

func (r *GormRepository) ListPublicUserPosts(ctx context.Context, userID uint, page, size int) ([]Post, int64, error) {
	query := r.db.WithContext(ctx).Model(&Post{}).
		Where("user_id = ? AND review_status = ?", userID, review.StatusApproved)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	values := make([]Post, 0, size)
	err := query.Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort ASC") }).Order("created_at DESC, id DESC").
		Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}
