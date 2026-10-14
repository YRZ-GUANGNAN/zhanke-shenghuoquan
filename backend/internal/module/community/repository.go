package community

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/pkg/dbtx"
)

var ErrNotFound = errors.New("community content not found")
var ErrInteractionInvariant = errors.New("community interaction count invariant violated")

type FeedQuery struct {
	Topic, Keyword, Sort string
	Page, PageSize       int
}

type Repository interface {
	CreatePost(context.Context, *Post) error
	FindPost(context.Context, uint, bool) (Post, error)
	ListPosts(context.Context, FeedQuery, bool) ([]Post, int64, error)
	ListFollowingPosts(context.Context, uint, FeedQuery) ([]Post, int64, error)
	CreateComment(context.Context, *Comment) error
	ListComments(context.Context, uint, int, int, bool) ([]Comment, int64, error)
	ToggleLike(context.Context, uint, uint) (bool, error)
	ToggleFavorite(context.Context, uint, uint) (bool, error)
	ToggleFollow(context.Context, uint, uint) (bool, error)
	ViewerState(context.Context, uint, uint, uint) (PostViewerState, error)
	SetLike(context.Context, uint, uint, bool) (bool, error)
	SetFavorite(context.Context, uint, uint, bool) (bool, error)
	SetFollow(context.Context, uint, uint, bool) (bool, error)
	ListNotifications(context.Context, uint, int, int) ([]Notification, int64, int64, error)
	MarkNotificationRead(context.Context, uint, uint) error
	SetReviewStatus(context.Context, string, uint, review.Status) error
	AdminTakedownPost(context.Context, uint) error
	Count(context.Context) (int64, int64, error)
}

type PostViewerState struct {
	Liked           bool  `json:"liked"`
	LikedCount      int64 `json:"likedCount"`
	Favorited       bool  `json:"favorited"`
	FollowingAuthor bool  `json:"followingAuthor"`
}

func (r *GormRepository) ListFollowingPosts(ctx context.Context, userID uint, q FeedQuery) ([]Post, int64, error) {
	var values []Post
	var total int64
	query := r.db.WithContext(ctx).Model(&Post{}).
		Joins("JOIN user_follows ON user_follows.followee_id = posts.user_id").
		Where("user_follows.follower_id = ? AND posts.review_status = ?", userID, review.StatusApproved)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort ASC") }).
		Order("posts.created_at DESC, posts.id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&values).Error
	return values, total, err
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) searchPostsQuery(ctx context.Context, viewerID uint, keyword string, filters SearchFilters, now time.Time) *gorm.DB {
	like, prefix := "%"+keyword+"%", keyword+"%"
	query := r.db.WithContext(ctx).Model(&Post{}).
		Where("review_status = ?", review.StatusApproved).
		Where("NOT EXISTS (SELECT 1 FROM user_blocks AS ub WHERE ub.blocker_id = ? AND ub.blocked_id = posts.user_id)", viewerID).
		Where("title LIKE ? OR content LIKE ? OR CAST(tags AS CHAR) LIKE ?", like, like, like)
	if filters.ContentType == "image" {
		query = query.Where("EXISTS (SELECT 1 FROM post_images AS media WHERE media.post_id = posts.id AND TRIM(media.url) <> '')")
	} else if filters.ContentType == "text" {
		query = query.Where("NOT EXISTS (SELECT 1 FROM post_images AS media WHERE media.post_id = posts.id AND TRIM(media.url) <> '')")
	}
	if filters.WithinDays > 0 {
		query = query.Where("posts.created_at >= ?", now.Add(-time.Duration(filters.WithinDays)*24*time.Hour))
	}
	switch filters.Sort {
	case "recent":
		return query.Order("created_at DESC, id DESC")
	case "likes":
		return query.Order("liked_count DESC, created_at DESC, id DESC")
	case "comments":
		return query.Order("comment_count DESC, created_at DESC, id DESC")
	case "favorites":
		return query.Order("favorite_count DESC, created_at DESC, id DESC")
	default:
		// Order(clause.Expr) is ignored by GORM; an OrderBy expression preserves the intended relevance ranking.
		return query.Clauses(clause.OrderBy{Expression: clause.Expr{SQL: `CASE
			WHEN title = ? THEN 0 WHEN title LIKE ? THEN 1 WHEN title LIKE ? THEN 2
			WHEN content LIKE ? THEN 3 ELSE 4 END,
			created_at DESC, (liked_count + comment_count + favorite_count) DESC, id DESC`, Vars: []any{keyword, prefix, like, like}}})
	}
}

func (r *GormRepository) SearchPosts(ctx context.Context, viewerID uint, keyword string, page, size int, options ...SearchFilters) ([]Post, int64, error) {
	filters := SearchFilters{}
	if len(options) > 0 {
		filters = options[0]
	}
	if err := validateSearchFilters(filters); err != nil {
		return nil, 0, err
	}
	query := r.searchPostsQuery(ctx, viewerID, keyword, filters, time.Now())
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	values := make([]Post, 0, size)
	err := query.Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort ASC") }).
		Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) ListRecentTagPosts(ctx context.Context, since time.Time) ([]Post, error) {
	values := make([]Post, 0)
	err := r.db.WithContext(ctx).Where(
		"review_status = ? AND created_at >= ? AND tags IS NOT NULL", review.StatusApproved, since,
	).Order("created_at ASC, id ASC").Find(&values).Error
	return values, err
}

func (r *GormRepository) CreatePost(ctx context.Context, value *Post) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		images := value.Images
		value.Images = nil
		if err := tx.Create(value).Error; err != nil {
			return err
		}
		for index := range images {
			images[index].PostID = value.ID
			images[index].Sort = index
		}
		if len(images) > 0 {
			if err := tx.Create(&images).Error; err != nil {
				return err
			}
		}
		value.Images = images
		return nil
	})
}

func (r *GormRepository) FindPost(ctx context.Context, id uint, publicOnly bool) (Post, error) {
	var value Post
	query := r.db.WithContext(ctx).Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort ASC") })
	if publicOnly {
		query = query.Where("review_status = ?", review.StatusApproved)
	}
	err := query.First(&value, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Post{}, ErrNotFound
	}
	return value, err
}

func (r *GormRepository) ListPosts(ctx context.Context, q FeedQuery, publicOnly bool) ([]Post, int64, error) {
	var values []Post
	var total int64
	query := r.db.WithContext(ctx).Model(&Post{})
	if publicOnly {
		query = query.Where("review_status = ?", review.StatusApproved)
	}
	if q.Topic != "" {
		query = query.Where("topic = ?", q.Topic)
	}
	if q.Keyword != "" {
		like := "%" + q.Keyword + "%"
		query = query.Where("title LIKE ? OR content LIKE ?", like, like)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := "pinned DESC, created_at DESC"
	if q.Sort == "popular" {
		order = "pinned DESC, liked_count DESC, comment_count DESC, created_at DESC"
	}
	err := query.Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort ASC") }).Order(order).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) CreateComment(ctx context.Context, value *Comment) error {
	return r.db.WithContext(ctx).Create(value).Error
}
func (r *GormRepository) ListComments(ctx context.Context, postID uint, page, size int, publicOnly bool) ([]Comment, int64, error) {
	var values []Comment
	var total int64
	query := r.db.WithContext(ctx).Model(&Comment{}).Where("post_id = ? AND parent_comment_id = 0", postID)
	if publicOnly {
		query = query.Where("review_status = ?", review.StatusApproved)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("created_at ASC, id ASC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) ToggleLike(ctx context.Context, postID, userID uint) (bool, error) {
	liked := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var value PostLike
		err := tx.Where("post_id = ? AND user_id = ?", postID, userID).First(&value).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&PostLike{PostID: postID, UserID: userID})
			if result.Error != nil {
				return result.Error
			}
			liked = true
			if result.RowsAffected == 0 {
				return nil
			}
			if err := tx.Model(&Post{}).Where("id = ?", postID).UpdateColumn("liked_count", gorm.Expr("liked_count + 1")).Error; err != nil {
				return err
			}
			return r.createPostNotification(tx, postID, userID, NotificationLike, 0)
		}
		if err != nil {
			return err
		}
		if err := tx.Delete(&value).Error; err != nil {
			return err
		}
		return tx.Model(&Post{}).Where("id = ?", postID).UpdateColumn("liked_count", gorm.Expr("GREATEST(liked_count - 1, 0)")).Error
	})
	return liked, err
}

func (r *GormRepository) ToggleFavorite(ctx context.Context, postID, userID uint) (bool, error) {
	favorited := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var value PostFavorite
		err := tx.Where("post_id = ? AND user_id = ?", postID, userID).First(&value).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&PostFavorite{PostID: postID, UserID: userID})
			if result.Error != nil {
				return result.Error
			}
			favorited = true
			if result.RowsAffected == 0 {
				return nil
			}
			return tx.Model(&Post{}).Where("id = ?", postID).UpdateColumn("favorite_count", gorm.Expr("favorite_count + 1")).Error
		}
		if err != nil {
			return err
		}
		if err := tx.Delete(&value).Error; err != nil {
			return err
		}
		return tx.Model(&Post{}).Where("id = ?", postID).UpdateColumn("favorite_count", gorm.Expr("GREATEST(favorite_count - 1, 0)")).Error
	})
	return favorited, err
}

func (r *GormRepository) ToggleFollow(ctx context.Context, followerID, followeeID uint) (bool, error) {
	followed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var value UserFollow
		err := tx.Where("follower_id = ? AND followee_id = ?", followerID, followeeID).First(&value).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&UserFollow{FollowerID: followerID, FolloweeID: followeeID})
			if result.Error != nil {
				return result.Error
			}
			followed = true
			if result.RowsAffected == 0 {
				return nil
			}
			return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&Notification{
				RecipientID: followeeID, ActorID: followerID, Type: NotificationFollow,
			}).Error
		}
		if err != nil {
			return err
		}
		return tx.Delete(&value).Error
	})
	return followed, err
}

func (r *GormRepository) ViewerState(ctx context.Context, postID, viewerID, authorID uint) (PostViewerState, error) {
	var state PostViewerState
	result := r.db.WithContext(ctx).Model(&Post{}).Where("id = ? AND review_status = ?", postID, review.StatusApproved).
		Select(`liked_count,
		 EXISTS (SELECT 1 FROM post_likes WHERE post_id = posts.id AND user_id = ?) AS liked,
		 EXISTS (SELECT 1 FROM post_favorites WHERE post_id = posts.id AND user_id = ?) AS favorited,
		 EXISTS (SELECT 1 FROM user_follows WHERE follower_id = ? AND followee_id = posts.user_id) AS following_author`, viewerID, viewerID, viewerID).Scan(&state)
	if result.Error == nil && result.RowsAffected == 0 {
		return PostViewerState{}, ErrNotFound
	}
	return state, result.Error
}

func (r *GormRepository) SetLike(ctx context.Context, postID, userID uint, desired bool) (bool, error) {
	state, err := r.SetLikeState(ctx, postID, userID, desired)
	return state.Liked, err
}

func (r *GormRepository) SetLikeState(ctx context.Context, postID, userID uint, desired bool) (CommentLikeState, error) {
	state := CommentLikeState{Liked: desired}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var post Post
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "review_status", "liked_count").First(&post, postID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if post.ReviewStatus != review.StatusApproved {
			return ErrNotFound
		}
		state.LikedCount = post.LikedCount
		if desired {
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&PostLike{PostID: postID, UserID: userID})
			if result.Error != nil || result.RowsAffected == 0 {
				return result.Error
			}
			if err := requireInteractionUpdate(tx.Model(&Post{}).Where("id = ?", postID).
				UpdateColumn("liked_count", gorm.Expr("liked_count + 1"))); err != nil {
				return err
			}
			state.LikedCount++
			return r.createPostNotification(tx, postID, userID, NotificationLike, 0)
		}
		result := tx.Where("post_id = ? AND user_id = ?", postID, userID).Delete(&PostLike{})
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		state.LikedCount--
		return requireInteractionUpdate(tx.Model(&Post{}).Where("id = ? AND liked_count > 0", postID).
			UpdateColumn("liked_count", gorm.Expr("liked_count - 1")))
	})
	return state, err
}

func (r *GormRepository) SetFavorite(ctx context.Context, postID, userID uint, desired bool) (bool, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var post Post
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "review_status").First(&post, postID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if post.ReviewStatus != review.StatusApproved {
			return ErrNotFound
		}
		if desired {
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&PostFavorite{PostID: postID, UserID: userID})
			if result.Error != nil || result.RowsAffected == 0 {
				return result.Error
			}
			return requireInteractionUpdate(tx.Model(&Post{}).Where("id = ?", postID).
				UpdateColumn("favorite_count", gorm.Expr("favorite_count + 1")))
		}
		result := tx.Where("post_id = ? AND user_id = ?", postID, userID).Delete(&PostFavorite{})
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		return requireInteractionUpdate(tx.Model(&Post{}).Where("id = ? AND favorite_count > 0", postID).
			UpdateColumn("favorite_count", gorm.Expr("favorite_count - 1")))
	})
	return desired, err
}

func requireInteractionUpdate(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrInteractionInvariant
	}
	return nil
}

func (r *GormRepository) SetFollow(ctx context.Context, followerID, followeeID uint, desired bool) (bool, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if desired {
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&UserFollow{FollowerID: followerID, FolloweeID: followeeID})
			if result.Error != nil || result.RowsAffected == 0 {
				return result.Error
			}
			return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&Notification{
				RecipientID: followeeID, ActorID: followerID, Type: NotificationFollow,
			}).Error
		}
		return tx.Where("follower_id = ? AND followee_id = ?", followerID, followeeID).Delete(&UserFollow{}).Error
	})
	return desired, err
}

func (r *GormRepository) ListNotifications(ctx context.Context, recipientID uint, page, size int) ([]Notification, int64, int64, error) {
	var values []Notification
	var total, unread int64
	query := r.db.WithContext(ctx).Model(&Notification{}).Where("recipient_id = ?", recipientID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, 0, err
	}
	if err := query.Where("read_at IS NULL").Count(&unread).Error; err != nil {
		return nil, 0, 0, err
	}
	err := r.db.WithContext(ctx).Where("recipient_id = ?", recipientID).
		Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, unread, err
}

func (r *GormRepository) MarkNotificationRead(ctx context.Context, recipientID, id uint) error {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&Notification{}).
		Where("id = ? AND recipient_id = ?", id, recipientID).Update("read_at", &now)
	if result.Error == nil && result.RowsAffected == 0 {
		return ErrNotFound
	}
	return result.Error
}

func (r *GormRepository) createPostNotification(tx *gorm.DB, postID, actorID uint, notificationType NotificationType, commentID uint) error {
	var post Post
	if err := tx.Select("id", "user_id").First(&post, postID).Error; err != nil {
		return err
	}
	if post.UserID == actorID {
		return nil
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&Notification{
		RecipientID: post.UserID, ActorID: actorID, Type: notificationType, PostID: postID, CommentID: commentID,
	}).Error
}

func (r *GormRepository) SetReviewStatus(ctx context.Context, objectType string, id uint, status review.Status) error {
	if objectType == "post" {
		result := dbtx.DB(ctx, r.db).Model(&Post{}).Where("id = ?", id).Update("review_status", status)
		if result.Error == nil && result.RowsAffected == 0 {
			return ErrNotFound
		}
		return result.Error
	}
	if objectType != "comment" {
		return errors.New("unsupported community review target")
	}
	return dbtx.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var coordinates Comment
		if err := tx.Select("id", "post_id").First(&coordinates, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		value, root, err := lockThreadComment(tx, coordinates.PostID, id)
		if err != nil {
			return err
		}
		if value.ReviewStatus == status {
			return nil
		}
		if err := tx.Model(&Comment{}).Where("id = ?", id).Update("review_status", status).Error; err != nil {
			return err
		}
		amount := int64(1)
		if value.ParentCommentID == 0 {
			var replies int64
			if err := tx.Model(&Comment{}).Clauses(clause.Locking{Strength: "UPDATE"}).Where("post_id = ? AND parent_comment_id = ? AND review_status = ?", value.PostID, id, review.StatusApproved).Count(&replies).Error; err != nil {
				return err
			}
			amount += replies
		} else if root.ReviewStatus != review.StatusApproved {
			return nil
		}
		if status == review.StatusApproved {
			if err := tx.Model(&Post{}).Where("id = ?", value.PostID).UpdateColumn("comment_count", gorm.Expr("comment_count + ?", amount)).Error; err != nil {
				return err
			}
			return r.createPostNotification(tx, value.PostID, value.UserID, NotificationComment, value.ID)
		}
		if value.ReviewStatus == review.StatusApproved {
			return tx.Model(&Post{}).Where("id = ?", value.PostID).UpdateColumn("comment_count", gorm.Expr("GREATEST(comment_count - ?, 0)", amount)).Error
		}
		return nil
	})
}

func (r *GormRepository) AdminTakedownPost(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var value Post
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&value, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if value.ReviewStatus == ReviewStatusTakenDown {
			return nil
		}
		return tx.Model(&Post{}).Where("id = ?", id).Update("review_status", ReviewStatusTakenDown).Error
	})
}

func (r *GormRepository) Count(ctx context.Context) (int64, int64, error) {
	var total, today int64
	if err := r.db.WithContext(ctx).Model(&Post{}).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	start := time.Now().Truncate(24 * time.Hour)
	err := r.db.WithContext(ctx).Model(&Post{}).Where("created_at >= ?", start).Count(&today).Error
	return total, today, err
}
