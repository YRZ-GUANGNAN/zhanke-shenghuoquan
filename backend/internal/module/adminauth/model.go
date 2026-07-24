package adminauth

import "time"

const (
	FixedUsername = "administrator"
	CookieName    = "__Host-zhandui_admin_session"
)

type Credential struct {
	ID                uint64 `gorm:"primaryKey"`
	Username          string `gorm:"size:64;uniqueIndex;not null"`
	UserID            uint   `gorm:"not null;uniqueIndex"`
	PasswordHash      string `gorm:"size:255;not null"`
	FailedAttempts    uint16 `gorm:"not null"`
	LockedUntil       *time.Time
	PasswordChangedAt time.Time
	RowVersion        uint64 `gorm:"not null"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (Credential) TableName() string { return "admin_credentials" }

type WebSession struct {
	ID                uint64 `gorm:"primaryKey"`
	CredentialID      uint64
	UserID            uint
	TokenHash         []byte `gorm:"type:binary(32);uniqueIndex"`
	CSRFHash          []byte `gorm:"type:binary(32)"`
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
	LastSeenAt        time.Time
	RevokedAt         *time.Time
	IPHash            []byte `gorm:"type:binary(32)"`
	UserAgentHash     []byte `gorm:"type:binary(32)"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (WebSession) TableName() string { return "admin_web_sessions" }

type AuditLog struct {
	ID            uint64 `gorm:"primaryKey"`
	CredentialID  *uint64
	UserID        *uint
	Username      string
	EventType     string
	Outcome       string
	IPHash        []byte `gorm:"type:binary(32)"`
	UserAgentHash []byte `gorm:"type:binary(32)"`
	Detail        string
	CreatedAt     time.Time
}

func (AuditLog) TableName() string { return "admin_auth_audit_logs" }
