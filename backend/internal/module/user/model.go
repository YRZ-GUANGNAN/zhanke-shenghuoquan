// Package user is business domain A: identity, profiles and authorization.
package user

import (
	"time"

	"gorm.io/gorm"
)

type Status string
type Role string

const (
	StatusActive Status = "active"
	StatusMuted  Status = "muted"
	StatusBanned Status = "banned"
	StatusClosed Status = "closed"

	RoleUser       Role = "user"
	RoleAdmin      Role = "admin"
	RoleSuperAdmin Role = "superadmin"
)

type User struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	OpenID          string         `json:"-" gorm:"uniqueIndex;size:96"`
	Phone           *string        `json:"-" gorm:"uniqueIndex;size:32"`
	Nickname        string         `json:"nickname" gorm:"size:64"`
	Avatar          string         `json:"avatar" gorm:"size:512"`
	AvatarAssetID   *uint64        `json:"-" gorm:"index"`
	School          string         `json:"school" gorm:"size:80"`
	Campus          string         `json:"campus" gorm:"size:80"`
	CurrentIdentity Identity       `json:"currentIdentity" gorm:"column:current_identity;size:24;not null;default:student"`
	Role            Role           `json:"role" gorm:"size:24;default:user"`
	Status          Status         `json:"status" gorm:"size:24;default:active"`
	TokenVersion    uint64         `json:"-" gorm:"not null;default:1"`
	ClosedAt        *time.Time     `json:"-"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

type Profile struct {
	ID            uint    `json:"id"`
	Nickname      string  `json:"nickname"`
	Avatar        string  `json:"avatar"`
	AvatarAssetID *uint64 `json:"-"`
	School        string  `json:"school"`
	Campus        string  `json:"campus"`
}

func (u User) Profile() Profile {
	if u.Status == StatusClosed {
		return Profile{ID: u.ID, Nickname: "账号已注销"}
	}
	return Profile{ID: u.ID, Nickname: u.Nickname, Avatar: u.Avatar, AvatarAssetID: u.AvatarAssetID, School: u.School, Campus: u.Campus}
}

type UserSettings struct {
	ID           uint      `json:"-" gorm:"primaryKey"`
	UserID       uint      `json:"-" gorm:"uniqueIndex;not null"`
	OrderNotice  bool      `json:"orderNotice" gorm:"not null;default:true"`
	SocialNotice bool      `json:"socialNotice" gorm:"not null;default:true"`
	QuietHours   bool      `json:"quietHours" gorm:"not null;default:false"`
	Personalized bool      `json:"personalized" gorm:"not null;default:true"`
	CreatedAt    time.Time `json:"-"`
	UpdatedAt    time.Time `json:"-"`
}

type SettingsView struct {
	OrderNotice    bool   `json:"orderNotice"`
	SocialNotice   bool   `json:"socialNotice"`
	QuietHours     bool   `json:"quietHours"`
	Personalized   bool   `json:"personalized"`
	PhoneBound     bool   `json:"phoneBound"`
	MaskedPhone    string `json:"maskedPhone"`
	BlacklistCount int64  `json:"blacklistCount"`
}

type UserBlock struct {
	UserID        uint      `json:"-" gorm:"primaryKey"`
	BlockedUserID uint      `json:"userId" gorm:"primaryKey"`
	CreatedAt     time.Time `json:"createdAt"`
}

type BlockedUserView struct {
	UserID    uint      `json:"userId"`
	Nickname  string    `json:"nickname"`
	Avatar    string    `json:"avatar"`
	CreatedAt time.Time `json:"createdAt"`
}

type Feedback struct {
	ID               uint            `json:"-" gorm:"primaryKey"`
	FeedbackPublicID string          `json:"id" gorm:"size:26;uniqueIndex;not null"`
	UserID           uint            `json:"userId" gorm:"not null;index"`
	Category         string          `json:"category" gorm:"size:24;not null"`
	Content          string          `json:"content" gorm:"size:1000;not null"`
	Status           string          `json:"status" gorm:"size:24;not null;default:open"`
	Images           []FeedbackImage `json:"images,omitempty" gorm:"foreignKey:FeedbackID"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

type FeedbackImage struct {
	ID            uint      `json:"-" gorm:"primaryKey"`
	FeedbackID    uint      `json:"-" gorm:"not null;index"`
	AssetPublicID string    `json:"assetId" gorm:"size:26;not null"`
	Slot          uint16    `json:"slot" gorm:"not null"`
	CreatedAt     time.Time `json:"-"`
}
