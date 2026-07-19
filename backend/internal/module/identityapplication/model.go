// Package identityapplication owns rider and merchant identity applications.
// It is independent from the legacy takeout runtime and persists the current
// application, its proof links, append-only events, and resulting grants.
package identityapplication

import (
	"encoding/json"
	"strconv"
	"time"
)

type Role string
type Status string
type GrantStatus string
type EventType string

const (
	RoleRider    Role = "rider"
	RoleMerchant Role = "merchant"

	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"

	GrantActive    GrantStatus = "active"
	GrantSuspended GrantStatus = "suspended"

	EventSubmitted   EventType = "submitted"
	EventResubmitted EventType = "resubmitted"
	EventApproved    EventType = "approved"
	EventRejected    EventType = "rejected"
)

func (r Role) Valid() bool { return r == RoleRider || r == RoleMerchant }

func (s Status) Valid() bool {
	return s == StatusPending || s == StatusApproved || s == StatusRejected
}

func (e EventType) Valid() bool {
	return e == EventSubmitted || e == EventResubmitted || e == EventApproved || e == EventRejected
}

type SubmitInput struct {
	RealName           string
	Phone              string
	MerchantName       string
	MerchantAddress    string
	AssetPublicIDs     []string
	ExpectedRowVersion uint64
}

type DecisionInput struct {
	ReviewerUserID      uint64
	ApplicationPublicID string
	ExpectedRowVersion  uint64
	Reason              string
	DecidedAt           time.Time
}

type ApprovedIdentity struct {
	UserID   uint64
	Role     Role
	RealName string
	Phone    string
}

type Application struct {
	ID                  uint64             `gorm:"column:id;primaryKey"`
	ApplicationPublicID string             `gorm:"column:application_public_id;type:char(26);not null;uniqueIndex:uq_identity_application_public_id"`
	UserID              uint64             `gorm:"column:user_id;not null;uniqueIndex:uq_identity_application_user_role,priority:1"`
	Role                Role               `gorm:"column:role;type:varchar(16);not null;uniqueIndex:uq_identity_application_user_role,priority:2"`
	Status              Status             `gorm:"column:status;type:varchar(16);not null;default:pending;index:idx_identity_application_status_submitted,priority:1"`
	RealName            string             `gorm:"column:real_name;size:64;not null"`
	Phone               string             `gorm:"column:phone;size:32;not null"`
	MerchantName        string             `gorm:"column:merchant_name;size:120;not null;default:''"`
	MerchantAddress     string             `gorm:"column:merchant_address;size:240;not null;default:''"`
	ReviewReason        string             `gorm:"column:review_reason;size:500;not null;default:''"`
	ReviewerUserID      *uint64            `gorm:"column:reviewer_user_id"`
	ReviewedAt          *time.Time         `gorm:"column:reviewed_at;type:datetime(3)"`
	SubmittedAt         time.Time          `gorm:"column:submitted_at;type:datetime(3);not null;index:idx_identity_application_status_submitted,priority:2"`
	RowVersion          uint64             `gorm:"column:row_version;not null;default:1"`
	CreatedAt           time.Time          `gorm:"column:created_at;type:datetime(3);not null"`
	UpdatedAt           time.Time          `gorm:"column:updated_at;type:datetime(3);not null"`
	Assets              []ApplicationAsset `gorm:"foreignKey:ApplicationID"`
}

func (Application) TableName() string { return "identity_applications" }

type ApplicationAsset struct {
	ID            uint64    `gorm:"column:id;primaryKey"`
	ApplicationID uint64    `gorm:"column:application_id;not null;uniqueIndex:uq_identity_application_asset_slot,priority:1;uniqueIndex:uq_identity_application_asset,priority:1"`
	AssetPublicID string    `gorm:"column:asset_public_id;type:char(26);not null;uniqueIndex:uq_identity_application_asset,priority:2"`
	SortOrder     uint16    `gorm:"column:sort_order;not null;uniqueIndex:uq_identity_application_asset_slot,priority:2"`
	CreatedAt     time.Time `gorm:"column:created_at;type:datetime(3);not null"`
}

func (ApplicationAsset) TableName() string { return "identity_application_assets" }

type ApplicationEvent struct {
	ID            uint64          `gorm:"column:id;primaryKey"`
	EventPublicID string          `gorm:"column:event_public_id;type:char(26);not null;uniqueIndex:uq_identity_application_event_public_id"`
	ApplicationID uint64          `gorm:"column:application_id;not null;index:idx_identity_application_event_application,priority:1"`
	ActorUserID   uint64          `gorm:"column:actor_user_id;not null"`
	EventType     EventType       `gorm:"column:event_type;type:varchar(16);not null"`
	Status        Status          `gorm:"column:status;type:varchar(16);not null"`
	Reason        string          `gorm:"column:reason;size:500;not null;default:''"`
	SnapshotJSON  json.RawMessage `gorm:"column:snapshot_json;type:json;not null"`
	CreatedAt     time.Time       `gorm:"column:created_at;type:datetime(3);not null;index:idx_identity_application_event_application,priority:2"`
}

func (ApplicationEvent) TableName() string { return "identity_application_events" }

type IdentityGrant struct {
	ID                  uint64      `gorm:"column:id;primaryKey"`
	UserID              uint64      `gorm:"column:user_id;not null;uniqueIndex:uq_identity_grant_user_role,priority:1;index:idx_identity_grant_status,priority:3"`
	Role                Role        `gorm:"column:role;type:varchar(16);not null;uniqueIndex:uq_identity_grant_user_role,priority:2;index:idx_identity_grant_status,priority:2"`
	Status              GrantStatus `gorm:"column:status;type:varchar(16);not null;default:active;index:idx_identity_grant_status,priority:1"`
	SourceApplicationID *uint64     `gorm:"column:source_application_id"`
	GrantedAt           time.Time   `gorm:"column:granted_at;type:datetime(3);not null"`
	RowVersion          uint64      `gorm:"column:row_version;not null;default:1"`
	CreatedAt           time.Time   `gorm:"column:created_at;type:datetime(3);not null"`
	UpdatedAt           time.Time   `gorm:"column:updated_at;type:datetime(3);not null"`
}

func (IdentityGrant) TableName() string { return "user_identity_grants" }

type ApplicationView struct {
	ID                  uint64           `json:"-"`
	ApplicationPublicID string           `json:"applicationId"`
	UserID              string           `json:"userId"`
	Role                Role             `json:"role"`
	Status              Status           `json:"status"`
	RealName            string           `json:"realName"`
	Phone               string           `json:"phone"`
	MerchantName        string           `json:"merchantName"`
	MerchantAddress     string           `json:"merchantAddress"`
	AssetPublicIDs      []string         `json:"assetIds"`
	Proofs              []ProofAssetView `json:"proofs,omitempty"`
	ReviewReason        string           `json:"reviewReason"`
	ReviewerUserID      *string          `json:"reviewerUserId,omitempty"`
	SubmittedAt         time.Time        `json:"submittedAt"`
	ReviewedAt          *time.Time       `json:"reviewedAt,omitempty"`
	RowVersion          uint64           `json:"rowVersion"`
}

type ProofAssetView struct {
	AssetID   string    `json:"assetId"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type ApplicationEventView struct {
	EventPublicID string    `json:"eventId"`
	ActorUserID   string    `json:"actorUserId"`
	EventType     EventType `json:"eventType"`
	Status        Status    `json:"status"`
	Reason        string    `json:"reason"`
	CreatedAt     time.Time `json:"createdAt"`
}

type ApplicationDetail struct {
	ApplicationView
	Events []ApplicationEventView `json:"events"`
}

type AdminListFilter struct {
	Role     Role
	Status   Status
	Page     int
	PageSize int
}

type AdminListView struct {
	Items    []ApplicationView `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}

type applicationSnapshot struct {
	ApplicationPublicID string     `json:"applicationId"`
	UserID              string     `json:"userId"`
	Role                Role       `json:"role"`
	Status              Status     `json:"status"`
	RealName            string     `json:"realName"`
	Phone               string     `json:"phone"`
	MerchantName        string     `json:"merchantName"`
	MerchantAddress     string     `json:"merchantAddress"`
	AssetPublicIDs      []string   `json:"assetIds"`
	ReviewReason        string     `json:"reviewReason"`
	ReviewerUserID      *string    `json:"reviewerUserId,omitempty"`
	ReviewedAt          *time.Time `json:"reviewedAt,omitempty"`
	RowVersion          uint64     `json:"rowVersion"`
}

func toApplicationView(value Application) ApplicationView {
	assetIDs := make([]string, len(value.Assets))
	for index := range value.Assets {
		assetIDs[index] = value.Assets[index].AssetPublicID
	}
	var reviewerUserID *string
	if value.ReviewerUserID != nil {
		formatted := strconv.FormatUint(*value.ReviewerUserID, 10)
		reviewerUserID = &formatted
	}
	return ApplicationView{
		ID:                  value.ID,
		ApplicationPublicID: value.ApplicationPublicID,
		UserID:              strconv.FormatUint(value.UserID, 10),
		Role:                value.Role,
		Status:              value.Status,
		RealName:            value.RealName,
		Phone:               value.Phone,
		MerchantName:        value.MerchantName,
		MerchantAddress:     value.MerchantAddress,
		AssetPublicIDs:      assetIDs,
		ReviewReason:        value.ReviewReason,
		ReviewerUserID:      reviewerUserID,
		SubmittedAt:         value.SubmittedAt,
		ReviewedAt:          value.ReviewedAt,
		RowVersion:          value.RowVersion,
	}
}
