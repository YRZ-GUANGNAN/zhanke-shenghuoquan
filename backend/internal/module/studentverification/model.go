// Package studentverification owns the student identity verification loop:
// submission, admin review, user-initiated cancel requests and admin
// cancellation. Each user has at most one current verification record.
package studentverification

import (
	"encoding/json"
	"strconv"
	"time"
)

type Status string
type EventType string

const (
	StatusPending   Status = "pending"
	StatusApproved  Status = "approved"
	StatusRejected  Status = "rejected"
	StatusCancelled Status = "cancelled"

	EventSubmitted       EventType = "submitted"
	EventResubmitted     EventType = "resubmitted"
	EventApproved        EventType = "approved"
	EventRejected        EventType = "rejected"
	EventCancelRequested EventType = "cancel_requested"
	EventCancelApproved  EventType = "cancel_approved"
)

func (s Status) Valid() bool {
	return s == StatusPending || s == StatusApproved || s == StatusRejected || s == StatusCancelled
}

func (e EventType) Valid() bool {
	return e == EventSubmitted || e == EventResubmitted || e == EventApproved ||
		e == EventRejected || e == EventCancelRequested || e == EventCancelApproved
}

type SubmitInput struct {
	RealName           string
	StudentNo          string
	College            string
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

type Application struct {
	ID                  uint64             `gorm:"column:id;primaryKey"`
	ApplicationPublicID string             `gorm:"column:public_id;type:char(26);not null;uniqueIndex:uq_student_verification_public_id"`
	UserID              uint64             `gorm:"column:user_id;not null;uniqueIndex:uq_student_verification_user"`
	Status              Status             `gorm:"column:status;type:varchar(16);not null;default:pending;index:idx_student_verification_status_submitted,priority:1"`
	RealName            string             `gorm:"column:real_name;size:64;not null"`
	StudentNo           string             `gorm:"column:student_no;size:64;not null"`
	College             string             `gorm:"column:college;size:120;not null;default:''"`
	ReviewReason        string             `gorm:"column:review_reason;size:500;not null;default:''"`
	ReviewerUserID      *uint64            `gorm:"column:reviewer_user_id"`
	ReviewedAt          *time.Time         `gorm:"column:reviewed_at;type:datetime(3)"`
	CancelRequestedAt   *time.Time         `gorm:"column:cancel_requested_at;type:datetime(3)"`
	SubmittedAt         time.Time          `gorm:"column:submitted_at;type:datetime(3);not null;index:idx_student_verification_status_submitted,priority:2"`
	RowVersion          uint64             `gorm:"column:row_version;not null;default:1"`
	CreatedAt           time.Time          `gorm:"column:created_at;type:datetime(3);not null"`
	UpdatedAt           time.Time          `gorm:"column:updated_at;type:datetime(3);not null"`
	Assets              []ApplicationAsset `gorm:"foreignKey:VerificationID"`
}

func (Application) TableName() string { return "student_verifications" }

type ApplicationAsset struct {
	ID             uint64    `gorm:"column:id;primaryKey"`
	VerificationID uint64    `gorm:"column:verification_id;not null;uniqueIndex:uq_student_verification_asset_slot,priority:1;uniqueIndex:uq_student_verification_asset,priority:1"`
	AssetPublicID  string    `gorm:"column:asset_public_id;type:char(26);not null;uniqueIndex:uq_student_verification_asset,priority:2"`
	SortOrder      uint16    `gorm:"column:sort_order;not null;uniqueIndex:uq_student_verification_asset_slot,priority:2"`
	CreatedAt      time.Time `gorm:"column:created_at;type:datetime(3);not null"`
}

func (ApplicationAsset) TableName() string { return "student_verification_assets" }

type ApplicationEvent struct {
	ID             uint64          `gorm:"column:id;primaryKey"`
	EventPublicID  string          `gorm:"column:event_public_id;type:char(26);not null;uniqueIndex:uq_student_verification_event_public_id"`
	VerificationID uint64          `gorm:"column:verification_id;not null;index:idx_student_verification_event_verification,priority:1"`
	ActorUserID    uint64          `gorm:"column:actor_user_id;not null"`
	EventType      EventType       `gorm:"column:event_type;type:varchar(24);not null"`
	Status         Status          `gorm:"column:status;type:varchar(16);not null"`
	Reason         string          `gorm:"column:reason;size:500;not null;default:''"`
	SnapshotJSON   json.RawMessage `gorm:"column:snapshot_json;type:json;not null"`
	CreatedAt      time.Time       `gorm:"column:created_at;type:datetime(3);not null;index:idx_student_verification_event_verification,priority:2"`
}

func (ApplicationEvent) TableName() string { return "student_verification_events" }

type VerificationView struct {
	ID                  uint64           `json:"-"`
	ApplicationPublicID string           `json:"applicationId"`
	UserID              string           `json:"userId"`
	Status              Status           `json:"status"`
	RealName            string           `json:"realName"`
	StudentNo           string           `json:"studentNo"`
	College             string           `json:"college"`
	AssetPublicIDs      []string         `json:"assetIds"`
	Proofs              []ProofAssetView `json:"proofs,omitempty"`
	ReviewReason        string           `json:"reviewReason"`
	ReviewerUserID      *string          `json:"reviewerUserId,omitempty"`
	CancelRequested     bool             `json:"cancelRequested"`
	CancelRequestedAt   *time.Time       `json:"cancelRequestedAt,omitempty"`
	SubmittedAt         time.Time        `json:"submittedAt"`
	ReviewedAt          *time.Time       `json:"reviewedAt,omitempty"`
	RowVersion          uint64           `json:"rowVersion"`
}

type ProofAssetView struct {
	AssetID   string    `json:"assetId"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type VerificationEventView struct {
	EventPublicID string    `json:"eventId"`
	ActorUserID   string    `json:"actorUserId"`
	EventType     EventType `json:"eventType"`
	Status        Status    `json:"status"`
	Reason        string    `json:"reason"`
	CreatedAt     time.Time `json:"createdAt"`
}

type VerificationDetail struct {
	VerificationView
	Events []VerificationEventView `json:"events"`
}

type AdminListFilter struct {
	Status   Status
	Page     int
	PageSize int
}

type AdminListView struct {
	Items    []VerificationView `json:"items"`
	Total    int64              `json:"total"`
	Page     int                `json:"page"`
	PageSize int                `json:"pageSize"`
}

type verificationSnapshot struct {
	ApplicationPublicID string     `json:"applicationId"`
	UserID              string     `json:"userId"`
	Status              Status     `json:"status"`
	RealName            string     `json:"realName"`
	StudentNo           string     `json:"studentNo"`
	College             string     `json:"college"`
	AssetPublicIDs      []string   `json:"assetIds"`
	ReviewReason        string     `json:"reviewReason"`
	ReviewerUserID      *string    `json:"reviewerUserId,omitempty"`
	ReviewedAt          *time.Time `json:"reviewedAt,omitempty"`
	CancelRequestedAt   *time.Time `json:"cancelRequestedAt,omitempty"`
	RowVersion          uint64     `json:"rowVersion"`
}

func toVerificationView(value Application) VerificationView {
	assetIDs := make([]string, len(value.Assets))
	for index := range value.Assets {
		assetIDs[index] = value.Assets[index].AssetPublicID
	}
	var reviewerUserID *string
	if value.ReviewerUserID != nil {
		formatted := strconv.FormatUint(*value.ReviewerUserID, 10)
		reviewerUserID = &formatted
	}
	return VerificationView{
		ID:                  value.ID,
		ApplicationPublicID: value.ApplicationPublicID,
		UserID:              strconv.FormatUint(value.UserID, 10),
		Status:              value.Status,
		RealName:            value.RealName,
		StudentNo:           value.StudentNo,
		College:             value.College,
		AssetPublicIDs:      assetIDs,
		ReviewReason:        value.ReviewReason,
		ReviewerUserID:      reviewerUserID,
		CancelRequested:     value.CancelRequestedAt != nil,
		CancelRequestedAt:   value.CancelRequestedAt,
		SubmittedAt:         value.SubmittedAt,
		ReviewedAt:          value.ReviewedAt,
		RowVersion:          value.RowVersion,
	}
}
