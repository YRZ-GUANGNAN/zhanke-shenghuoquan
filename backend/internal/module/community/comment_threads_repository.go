package community

import (
	"context"
	"errors"

	"campus-circle/backend/internal/module/admin/review"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ commentThreadRepository = (*GormRepository)(nil)

func (r *GormRepository) FindThreadComment(ctx context.Context, id uint) (Comment, error) {
	var value Comment
	err := r.db.WithContext(ctx).First(&value, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Comment{}, ErrNotFound
	}
	return value, err
}

func threadVisibility(query *gorm.DB, viewerID uint) *gorm.DB {
	return query.Where("review_status = ? OR (user_id = ? AND ? <> 0 AND review_status IN ?)", review.StatusApproved, viewerID, viewerID,
		[]review.Status{review.StatusSubmitted, review.StatusReviewing, review.StatusRejected})
}

func (r *GormRepository) ListCommentLevel(ctx context.Context, postID, rootID, viewerID uint, page, size int) ([]Comment, int64, error) {
	query := threadVisibility(r.db.WithContext(ctx).Model(&Comment{}).Where("post_id = ? AND parent_comment_id = ?", postID, rootID), viewerID)
	query = query.Where("EXISTS (SELECT 1 FROM posts WHERE posts.id = comments.post_id AND posts.deleted_at IS NULL AND posts.review_status = ?)", review.StatusApproved)
	if rootID != 0 {
		query = query.Where("EXISTS (SELECT 1 FROM comments AS root WHERE root.id = comments.parent_comment_id AND root.post_id = comments.post_id AND root.parent_comment_id = 0 AND root.review_status = ?)", review.StatusApproved)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	values := make([]Comment, 0, size)
	err := query.Order("created_at ASC, id ASC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) CommentMetadata(ctx context.Context, postID uint, ids []uint, viewerID uint) (map[uint]CommentMetadata, error) {
	result := make(map[uint]CommentMetadata, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var likes []CommentLike
	if err := r.db.WithContext(ctx).Where("comment_id IN ? AND user_id = ?", ids, viewerID).Find(&likes).Error; err != nil {
		return nil, err
	}
	for _, like := range likes {
		result[like.CommentID] = CommentMetadata{Liked: true}
	}
	var counts []struct {
		ParentCommentID uint
		ReplyCount      int64
	}
	if err := threadVisibility(r.db.WithContext(ctx).Model(&Comment{}).Where("post_id = ? AND parent_comment_id IN ?", postID, ids), viewerID).
		Select("parent_comment_id, COUNT(*) AS reply_count").Group("parent_comment_id").Scan(&counts).Error; err != nil {
		return nil, err
	}
	for _, count := range counts {
		value := result[count.ParentCommentID]
		value.ReplyCount = count.ReplyCount
		result[count.ParentCommentID] = value
	}
	return result, nil
}

// Every thread mutation locks root -> target -> post, matching moderation.
// Parent coordinates are immutable; the first read only discovers the root.
func lockThreadComment(tx *gorm.DB, postID, id uint) (Comment, Comment, error) {
	var value Comment
	if err := tx.Where("id = ? AND post_id = ?", id, postID).First(&value).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Comment{}, Comment{}, ErrNotFound
		}
		return Comment{}, Comment{}, err
	}
	rootID := value.ID
	if value.ParentCommentID != 0 {
		rootID = value.ParentCommentID
	}
	var root Comment
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND post_id = ? AND parent_comment_id = 0", rootID, postID).First(&root).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Comment{}, Comment{}, ErrNotFound
		}
		return Comment{}, Comment{}, err
	}
	if id == rootID {
		return root, root, nil
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&value, id).Error; err != nil {
		return Comment{}, Comment{}, err
	}
	return value, root, nil
}

func requireApprovedPost(tx *gorm.DB, postID uint) error {
	var value Post
	err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Select("id", "review_status").First(&value, postID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if value.ReviewStatus != review.StatusApproved {
		return ErrNotFound
	}
	return nil
}

func (r *GormRepository) SetCommentLiked(ctx context.Context, postID, commentID, userID uint, desired bool) (CommentLikeState, error) {
	state := CommentLikeState{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		value, root, err := lockThreadComment(tx, postID, commentID)
		if err != nil {
			return err
		}
		if value.ReviewStatus != review.StatusApproved || root.ReviewStatus != review.StatusApproved {
			return ErrNotFound
		}
		if err = requireApprovedPost(tx, postID); err != nil {
			return err
		}
		var result *gorm.DB
		if desired {
			result = tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&CommentLike{CommentID: commentID, UserID: userID})
		} else {
			result = tx.Where("comment_id = ? AND user_id = ?", commentID, userID).Delete(&CommentLike{})
		}
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected > 0 {
			if desired {
				value.LikedCount++
			} else {
				value.LikedCount--
			}
			if value.LikedCount < 0 {
				return ErrInteractionInvariant
			}
			if err = tx.Model(&Comment{}).Where("id = ?", commentID).UpdateColumn("liked_count", value.LikedCount).Error; err != nil {
				return err
			}
		}
		state = CommentLikeState{Liked: desired, LikedCount: value.LikedCount}
		return nil
	})
	return state, err
}
