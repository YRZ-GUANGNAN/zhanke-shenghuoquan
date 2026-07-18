// Package community is business domain B: posts, media, comments and interactions.
package community

import (
	"time"

	"gorm.io/gorm"

	"campus-circle/backend/internal/module/admin/review"
)

type Post struct {
	ID                uint           `json:"id" gorm:"primaryKey"`
	UserID            uint           `json:"userId" gorm:"index"`
	ClientRequestID   *string        `json:"-" gorm:"column:client_request_id;type:char(26)"`
	CreateRequestHash []byte         `json:"-" gorm:"column:create_request_hash;type:binary(32)"`
	Topic             string         `json:"topic" gorm:"size:64;index"`
	Tags              []string       `json:"tags" gorm:"serializer:json"`
	Title             string         `json:"title" gorm:"size:120"`
	Content           string         `json:"content" gorm:"type:text"`
	ReviewStatus      review.Status  `json:"reviewStatus" gorm:"size:24;index;default:submitted"`
	Pinned            bool           `json:"pinned" gorm:"default:false"`
	LikedCount        int64          `json:"likedCount"`
	CommentCount      int64          `json:"commentCount"`
	FavoriteCount     int64          `json:"favoriteCount"`
	Images            []PostImage    `json:"images"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`
}

type PostImage struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	PostID    uint      `json:"-" gorm:"uniqueIndex:idx_post_image_sort"`
	AssetID   *uint64   `json:"-" gorm:"index"`
	URL       string    `json:"url" gorm:"size:512"`
	Sort      int       `json:"sort" gorm:"uniqueIndex:idx_post_image_sort"`
	CreatedAt time.Time `json:"createdAt"`
}

type Comment struct {
	ID                uint          `json:"id" gorm:"primaryKey"`
	PostID            uint          `json:"postId" gorm:"index"`
	UserID            uint          `json:"userId" gorm:"index"`
	ClientRequestID   *string       `json:"-" gorm:"column:client_request_id;type:char(26)"`
	CreateRequestHash []byte        `json:"-" gorm:"column:create_request_hash;type:binary(32)"`
	Content           string        `json:"content" gorm:"type:text"`
	ReviewStatus      review.Status `json:"reviewStatus" gorm:"size:24;index;default:submitted"`
	CreatedAt         time.Time     `json:"createdAt"`
	UpdatedAt         time.Time     `json:"updatedAt"`
}

type PostLike struct {
	PostID    uint `gorm:"primaryKey"`
	UserID    uint `gorm:"primaryKey"`
	CreatedAt time.Time
}
type PostFavorite struct {
	PostID    uint `gorm:"primaryKey"`
	UserID    uint `gorm:"primaryKey"`
	CreatedAt time.Time
}

type UserFollow struct {
	FollowerID uint      `json:"followerId" gorm:"primaryKey"`
	FolloweeID uint      `json:"followeeId" gorm:"primaryKey"`
	CreatedAt  time.Time `json:"createdAt"`
}

type NotificationType string

const (
	ReviewStatusTakenDown review.Status = "taken_down"

	NotificationComment NotificationType = "post_commented"
	NotificationLike    NotificationType = "post_liked"
	NotificationFollow  NotificationType = "user_followed"
)

type Notification struct {
	ID          uint             `json:"id" gorm:"primaryKey"`
	RecipientID uint             `json:"recipientId" gorm:"index;uniqueIndex:idx_notification_event"`
	ActorID     uint             `json:"actorId" gorm:"index;uniqueIndex:idx_notification_event"`
	Type        NotificationType `json:"type" gorm:"size:32;index;uniqueIndex:idx_notification_event"`
	PostID      uint             `json:"postId" gorm:"index;uniqueIndex:idx_notification_event"`
	CommentID   uint             `json:"commentId" gorm:"uniqueIndex:idx_notification_event"`
	ReadAt      *time.Time       `json:"readAt" gorm:"index"`
	CreatedAt   time.Time        `json:"createdAt"`
}
