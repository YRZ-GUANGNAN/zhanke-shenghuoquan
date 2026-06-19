// Package im is business domain E. It stores only Tencent IM identity mappings and
// conversation relationships; message content is never persisted by this backend.
package im

import "time"

type UserMapping struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"userId" gorm:"uniqueIndex"`
	IMUserID  string    `json:"imUserId" gorm:"size:64;uniqueIndex"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Conversation struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	ConversationID string    `json:"conversationId" gorm:"size:128;index"`
	BuyerID        uint      `json:"buyerId" gorm:"index;uniqueIndex:idx_im_relation"`
	SellerID       uint      `json:"sellerId" gorm:"index;uniqueIndex:idx_im_relation"`
	ReferenceType  string    `json:"referenceType" gorm:"size:32;uniqueIndex:idx_im_relation"`
	ReferenceID    uint      `json:"referenceId" gorm:"uniqueIndex:idx_im_relation"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (UserMapping) TableName() string  { return "im_user_mappings" }
func (Conversation) TableName() string { return "im_conversations" }
