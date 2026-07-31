package trade

import (
	"time"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
	"gorm.io/gorm"
)

const (
	ProductCommentCreatedEventType = "trade_product_comment_created"
	ProductCommentAggregateType    = "product_comment"
)

type ProductComment struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	ProductID       uint           `json:"productId" gorm:"index"`
	UserID          uint           `json:"userId" gorm:"index"`
	ParentCommentID uint           `json:"parentCommentId" gorm:"index"`
	ReplyToUserID   uint           `json:"replyToUserId" gorm:"index"`
	Content         string         `json:"content" gorm:"size:500"`
	ReviewStatus    review.Status  `json:"reviewStatus" gorm:"size:24;index;default:submitted"`
	ClientRequestID string         `json:"-" gorm:"type:char(26)"`
	RequestHash     []byte         `json:"-" gorm:"type:binary(32)"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"-"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

type ProductCommentInput struct {
	Content         string `json:"content"`
	ClientRequestID string `json:"clientRequestId"`
}

type ProductCommentView struct {
	ID              uint                 `json:"id"`
	ProductID       uint                 `json:"productId"`
	ParentCommentID uint                 `json:"parentCommentId"`
	ReplyToUserID   uint                 `json:"replyToUserId"`
	Content         string               `json:"content"`
	ReviewStatus    review.Status        `json:"reviewStatus"`
	Author          user.Profile         `json:"author"`
	ReplyTo         *user.Profile        `json:"replyTo,omitempty"`
	IsSeller        bool                 `json:"isSeller"`
	Replies         []ProductCommentView `json:"replies"`
	CreatedAt       time.Time            `json:"createdAt"`
}

type ProductCommentPage struct {
	Items    []ProductCommentView `json:"items"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
	Total    int64                `json:"total"`
}
