package media

import (
	"errors"
	"time"
)

// ProjectionStatus is deliberately separate from the technical media status.
// A review result cannot promote, copy, delete, or otherwise mutate an object.
type ProjectionStatus string

const (
	ProjectionPending      ProjectionStatus = "pending"
	ProjectionSubmitted    ProjectionStatus = "submitted"
	ProjectionReviewing    ProjectionStatus = "reviewing"
	ProjectionApproved     ProjectionStatus = "approved"
	ProjectionRejected     ProjectionStatus = "rejected"
	ProjectionReviewFailed ProjectionStatus = "review_failed"
	ProjectionNotRequired  ProjectionStatus = "not_required"
)

func (s ProjectionStatus) Valid() bool {
	switch s {
	case ProjectionPending, ProjectionSubmitted, ProjectionReviewing,
		ProjectionApproved, ProjectionRejected, ProjectionReviewFailed, ProjectionNotRequired:
		return true
	default:
		return false
	}
}

func (s ProjectionStatus) Terminal() bool {
	return s.Decision() || s == ProjectionNotRequired
}

func (s ProjectionStatus) Decision() bool {
	return s == ProjectionApproved || s == ProjectionRejected || s == ProjectionReviewFailed
}

type ReviewProjection struct {
	ProjectionID           string           `gorm:"column:projection_id;size:26;primaryKey"`
	AssetID                uint64           `gorm:"column:asset_id;not null;uniqueIndex:uq_media_review_projection_asset"`
	ReviewTaskID           *uint64          `gorm:"column:review_task_id;uniqueIndex:uq_media_review_projection_task"`
	Status                 ProjectionStatus `gorm:"column:status;size:24;not null;index:idx_media_review_projection_status,priority:1"`
	AppliedDecisionVersion uint64           `gorm:"column:applied_decision_version;not null"`
	LastDecisionHash       *string          `gorm:"column:last_decision_hash;size:64"`
	FailureCode            string           `gorm:"column:failure_code;size:64;not null"`
	SubmittedAt            *time.Time       `gorm:"column:submitted_at"`
	ReviewingAt            *time.Time       `gorm:"column:reviewing_at"`
	DecidedAt              *time.Time       `gorm:"column:decided_at"`
	RowVersion             uint64           `gorm:"column:row_version;not null"`
	CreatedAt              time.Time        `gorm:"column:created_at;not null"`
	UpdatedAt              time.Time        `gorm:"column:updated_at;not null;index:idx_media_review_projection_status,priority:2"`
}

func (ReviewProjection) TableName() string { return "media_review_projections" }

type ReviewDecision struct {
	ID              uint64           `gorm:"column:id;primaryKey"`
	ProjectionID    string           `gorm:"column:projection_id;size:26;not null;uniqueIndex:uq_media_review_decision_projection_version,priority:1"`
	ReviewTaskID    uint64           `gorm:"column:review_task_id;not null;uniqueIndex:uq_media_review_decision_task_version,priority:1"`
	DecisionVersion uint64           `gorm:"column:decision_version;not null;uniqueIndex:uq_media_review_decision_task_version,priority:2;uniqueIndex:uq_media_review_decision_projection_version,priority:2"`
	Decision        ProjectionStatus `gorm:"column:decision;size:24;not null"`
	PayloadHash     string           `gorm:"column:payload_hash;size:64;not null"`
	ResultCode      string           `gorm:"column:result_code;size:64;not null"`
	ReceivedAt      time.Time        `gorm:"column:received_at;not null"`
}

func (ReviewDecision) TableName() string { return "media_review_decisions" }

type ApplyReviewDecisionInput struct {
	ProjectionID       string
	ReviewTaskID       uint64
	DecisionVersion    uint64
	ExpectedStatus     ProjectionStatus
	ExpectedRowVersion uint64
	Decision           ProjectionStatus
	PayloadHash        string
	ResultCode         string
}

var (
	ErrReviewInvariant    = errors.New("media review projection invariant failed")
	ErrReviewTaskConflict = errors.New("media review task conflict")
	ErrDecisionConflict   = errors.New("media review decision conflict")
	ErrStaleDecision      = errors.New("stale media review decision")
)
