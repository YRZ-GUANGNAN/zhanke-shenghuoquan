package community

import (
	"context"

	"campus-circle/backend/internal/module/admin/review"
	"gorm.io/gorm"
)

func (r *GormRepository) ListOwnedPosts(ctx context.Context, userID uint, page, size int) ([]Post, int64, error) {
	query := r.db.WithContext(ctx).Model(&Post{}).Where("user_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []Post
	err := query.Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort ASC") }).
		Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) ListFavoritePosts(ctx context.Context, userID uint, page, size int) ([]Post, int64, error) {
	query := r.db.WithContext(ctx).Model(&Post{}).
		Joins("JOIN post_favorites AS pf ON pf.post_id = posts.id").
		Where("pf.user_id = ? AND posts.review_status = ?", userID, review.StatusApproved)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []Post
	err := query.Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort ASC") }).
		Order("pf.created_at DESC, posts.id DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) ListOwnedComments(ctx context.Context, userID uint, page, size int) ([]MyCommentView, int64, error) {
	query := r.db.WithContext(ctx).Table("comments AS c").
		Joins("JOIN posts AS p ON p.id = c.post_id AND p.deleted_at IS NULL").
		Where("c.user_id = ?", userID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []MyCommentView
	err := query.Select("c.id, c.post_id, p.title AS post_title, c.content, c.review_status, c.created_at").
		Order("c.created_at DESC, c.id DESC").Offset((page - 1) * size).Limit(size).Scan(&values).Error
	return values, total, err
}

func (r *GormRepository) MyContentCounts(ctx context.Context, userID uint) (MyContentCounts, error) {
	var value MyContentCounts
	if err := r.db.WithContext(ctx).Table("post_favorites AS pf").
		Joins("JOIN posts AS p ON p.id = pf.post_id AND p.deleted_at IS NULL").
		Where("pf.user_id = ? AND p.review_status = ?", userID, review.StatusApproved).
		Count(&value.Favorites).Error; err != nil {
		return MyContentCounts{}, err
	}
	queries := []struct {
		table string
		where string
		dest  *int64
	}{
		{"posts", "user_id = ? AND deleted_at IS NULL", &value.Posts},
		{"comments", "user_id = ?", &value.Comments},
	}
	for _, query := range queries {
		if err := r.db.WithContext(ctx).Table(query.table).Where(query.where, userID).Count(query.dest).Error; err != nil {
			return MyContentCounts{}, err
		}
	}
	return value, nil
}
