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

	RoleUser       Role = "user"
	RoleAdmin      Role = "admin"
	RoleSuperAdmin Role = "superadmin"
)

type User struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	OpenID          string         `json:"-" gorm:"uniqueIndex;size:96"`
	Nickname        string         `json:"nickname" gorm:"size:64"`
	Avatar          string         `json:"avatar" gorm:"size:512"`
	AvatarAssetID   *uint64        `json:"-" gorm:"index"`
	School          string         `json:"school" gorm:"size:80"`
	Campus          string         `json:"campus" gorm:"size:80"`
	CurrentIdentity Identity       `json:"currentIdentity" gorm:"column:current_identity;size:24;not null;default:student"`
	Role            Role           `json:"role" gorm:"size:24;default:user"`
	Status          Status         `json:"status" gorm:"size:24;default:active"`
	TokenVersion    uint64         `json:"-" gorm:"not null;default:1"`
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
	return Profile{ID: u.ID, Nickname: u.Nickname, Avatar: u.Avatar, AvatarAssetID: u.AvatarAssetID, School: u.School, Campus: u.Campus}
}
