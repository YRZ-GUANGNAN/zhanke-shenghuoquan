// Package featureboard owns the public feature-suggestion wall (submission,
// support toggling and ranked listing) and the maintainer's author notes.
package featureboard

import "time"

type SuggestionStatus string

const (
	StatusOpen     SuggestionStatus = "open"
	StatusAccepted SuggestionStatus = "accepted"
	StatusClosed   SuggestionStatus = "closed"
)

func (s SuggestionStatus) Valid() bool {
	return s == StatusOpen || s == StatusAccepted || s == StatusClosed
}

type Suggestion struct {
	ID           uint64           `gorm:"primaryKey"`
	PublicID     string           `gorm:"column:public_id;size:26"`
	UserID       uint64           `gorm:"column:user_id"`
	Content      string           `gorm:"size:500"`
	SupportCount uint32           `gorm:"column:support_count"`
	Status       SuggestionStatus `gorm:"size:16"`
	CreatedAt    time.Time        `gorm:"column:created_at"`
	UpdatedAt    time.Time        `gorm:"column:updated_at"`
}

func (Suggestion) TableName() string { return "feature_suggestions" }

type Support struct {
	ID           uint64    `gorm:"primaryKey"`
	SuggestionID uint64    `gorm:"column:suggestion_id"`
	UserID       uint64    `gorm:"column:user_id"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (Support) TableName() string { return "feature_suggestion_supports" }

type AuthorNote struct {
	ID        uint64    `gorm:"primaryKey"`
	PublicID  string    `gorm:"column:public_id;size:26"`
	Content   string    `gorm:"size:1000"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (AuthorNote) TableName() string { return "author_notes" }

// SuggestionRecord pairs a suggestion row with the viewer's support flag and
// the conversation size.
type SuggestionRecord struct {
	Suggestion   `gorm:"embedded"`
	Supported    bool
	CommentCount int64
}

type SuggestionView struct {
	ID           string           `json:"id"`
	Content      string           `json:"content"`
	Status       SuggestionStatus `json:"status"`
	SupportCount uint32           `json:"supportCount"`
	Supported    bool             `json:"supported"`
	CommentCount int64            `json:"commentCount"`
	CreatedAt    time.Time        `json:"createdAt"`
}

type AdminSuggestionView struct {
	ID           string           `json:"id"`
	UserID       uint64           `json:"userId"`
	Content      string           `json:"content"`
	Status       SuggestionStatus `json:"status"`
	SupportCount uint32           `json:"supportCount"`
	CommentCount int64            `json:"commentCount"`
	CreatedAt    time.Time        `json:"createdAt"`
}

type ListView struct {
	Items    []SuggestionView `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"pageSize"`
	Total    int64            `json:"total"`
}

type AdminListView struct {
	Items    []AdminSuggestionView `json:"items"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"pageSize"`
	Total    int64                 `json:"total"`
}

type NoteView struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// Comment is one message in a suggestion's conversation. Maintainer replies
// carry is_maintainer so the wall can label them as 作者.
type Comment struct {
	ID           uint64    `gorm:"primaryKey"`
	PublicID     string    `gorm:"column:public_id;size:26"`
	SuggestionID uint64    `gorm:"column:suggestion_id"`
	UserID       uint64    `gorm:"column:user_id"`
	IsMaintainer bool      `gorm:"column:is_maintainer"`
	Content      string    `gorm:"size:300"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (Comment) TableName() string { return "feature_suggestion_comments" }

type CommentView struct {
	ID           string    `json:"id"`
	Content      string    `json:"content"`
	IsMaintainer bool      `json:"isMaintainer"`
	CreatedAt    time.Time `json:"createdAt"`
}

type CommentListView struct {
	Items    []CommentView `json:"items"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
	Total    int64         `json:"total"`
}
