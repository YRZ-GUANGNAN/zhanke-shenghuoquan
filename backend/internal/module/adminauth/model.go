package adminauth

import "time"

const (
	CookieName = "__Host-zhandui_admin_session"
)

// Role 是后台管理员账号自身的角色，与小程序 users.role 完全独立。
type Role string

const (
	RoleAdmin      Role = "admin"
	RoleSuperAdmin Role = "superadmin"
)

func (r Role) Valid() bool { return r == RoleAdmin || r == RoleSuperAdmin }

type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

func (s Status) Valid() bool { return s == StatusActive || s == StatusDisabled }

// Account 是独立的后台管理员账号（admin_credentials 表）。
// UserID/LegacyUserID 仅用于迁移期的历史映射；新建账号两者均为 NULL。
type Account struct {
	ID                uint64 `gorm:"primaryKey"`
	Username          string `gorm:"size:64;uniqueIndex;not null"`
	Role              Role   `gorm:"size:24;not null"`
	Status            Status `gorm:"size:24;not null"`
	DisplayName       string `gorm:"size:64;not null"`
	LegacyUserID      *uint64
	UserID            *uint64
	PasswordHash      string `gorm:"size:255;not null"`
	FailedAttempts    uint16 `gorm:"not null"`
	LockedUntil       *time.Time
	PasswordChangedAt time.Time
	RowVersion        uint64 `gorm:"not null"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (Account) TableName() string { return "admin_credentials" }

type WebSession struct {
	ID                uint64 `gorm:"primaryKey"`
	CredentialID      uint64
	UserID            *uint64
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
	ID                uint64 `gorm:"primaryKey"`
	CredentialID      *uint64
	ActorCredentialID *uint64
	UserID            *uint64
	Username          string
	EventType         string
	Outcome           string
	IPHash            []byte `gorm:"type:binary(32)"`
	UserAgentHash     []byte `gorm:"type:binary(32)"`
	Detail            string
	CreatedAt         time.Time
}

func (AuditLog) TableName() string { return "admin_auth_audit_logs" }

// AccountView 是返回给后台前端的管理员账号视图。
type AccountView struct {
	ID            uint64    `json:"id"`
	Username      string    `json:"username"`
	DisplayName   string    `json:"displayName"`
	Role          Role      `json:"role"`
	Status        Status    `json:"status"`
	Locked        bool      `json:"locked"`
	PasswordSetAt time.Time `json:"passwordSetAt"`
	LegacyUserID  *uint64   `json:"legacyUserId,omitempty"`
}
