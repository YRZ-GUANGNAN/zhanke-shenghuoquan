// Package mediaconsole is Domain F's read-only operational projection over
// immutable Media Domain facts. It owns no media state and exposes no writes.
package mediaconsole

import (
	"errors"
	"time"
)

type ListFilter struct {
	Page         int
	PageSize     int
	AssetID      string
	Status       string
	ReviewStatus string
	Purpose      string
	TaskID       *uint64
	Operation    string
	Kind         string
	From         *time.Time
	To           *time.Time
}

type Page[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type Summary struct {
	Assets              int64 `json:"assets"`
	PendingReview       int64 `json:"pendingReview"`
	ConsumerDeadletters int64 `json:"consumerDeadletters"`
	GCDeadletters       int64 `json:"gcDeadletters"`
}

type AssetRow struct {
	DBAssetID          uint64     `json:"-" gorm:"column:db_asset_id"`
	AssetPublicID      string     `json:"assetId" gorm:"column:asset_public_id"`
	OwnerID            uint64     `json:"ownerId" gorm:"column:owner_id"`
	Purpose            string     `json:"purpose" gorm:"column:purpose"`
	StorageStage       string     `json:"storageStage" gorm:"column:storage_stage"`
	TechnicalStatus    string     `json:"technicalStatus" gorm:"column:technical_status"`
	ProjectionStatus   string     `json:"reviewStatus" gorm:"column:projection_status"`
	ReviewTaskID       *uint64    `json:"reviewTaskId,omitempty" gorm:"column:review_task_id"`
	TicketID           *string    `json:"ticketId,omitempty" gorm:"column:ticket_id"`
	TicketStatus       *string    `json:"ticketStatus,omitempty" gorm:"column:ticket_status"`
	CompletionPhase    *string    `json:"completionPhase,omitempty" gorm:"column:completion_phase"`
	ReferenceCount     uint64     `json:"referenceCount" gorm:"column:reference_count"`
	BlockingReferences uint64     `json:"blockingReferences" gorm:"column:blocking_references"`
	MIMEType           string     `json:"mimeType" gorm:"column:verified_mime"`
	ByteSize           uint64     `json:"byteSize" gorm:"column:byte_size"`
	Width              uint32     `json:"width" gorm:"column:width"`
	Height             uint32     `json:"height" gorm:"column:height"`
	CreatedAt          time.Time  `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt          time.Time  `json:"updatedAt" gorm:"column:updated_at"`
	DeleteRequestedAt  *time.Time `json:"deleteRequestedAt,omitempty" gorm:"column:delete_requested_at"`
	DeletedAt          *time.Time `json:"deletedAt,omitempty" gorm:"column:deleted_at"`
}

type ReferenceRow struct {
	ID               uint64     `json:"id" gorm:"column:id"`
	ReferenceType    string     `json:"referenceType" gorm:"column:reference_type"`
	ReferenceID      string     `json:"referenceId" gorm:"column:reference_id"`
	Slot             uint16     `json:"slot" gorm:"column:slot"`
	Status           string     `json:"status" gorm:"column:status"`
	ParentVisibility string     `json:"parentVisibility" gorm:"column:parent_visibility"`
	AggregateVersion uint64     `json:"aggregateVersion" gorm:"column:aggregate_version"`
	FenceExpiresAt   *time.Time `json:"fenceExpiresAt,omitempty" gorm:"column:fence_expires_at"`
	UpdatedAt        time.Time  `json:"updatedAt" gorm:"column:updated_at"`
}

type TicketRow struct {
	TicketID             string    `json:"ticketId" gorm:"column:ticket_id"`
	Status               string    `json:"status" gorm:"column:status"`
	ExpectedSize         uint64    `json:"expectedSize" gorm:"column:expected_size"`
	ExpectedContentType  string    `json:"expectedContentType" gorm:"column:expected_content_type"`
	CredentialIssueCount uint8     `json:"credentialIssueCount" gorm:"column:credential_issue_count"`
	ExpiresAt            time.Time `json:"expiresAt" gorm:"column:expires_at"`
	CreatedAt            time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt            time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

type CompletionRow struct {
	TicketID           string     `json:"ticketId" gorm:"column:ticket_id"`
	Phase              string     `json:"phase" gorm:"column:phase"`
	ValidationAttempts uint32     `json:"validationAttempts" gorm:"column:validation_attempts"`
	PromoteAttempts    uint32     `json:"promoteAttempts" gorm:"column:promote_attempts"`
	LastErrorCode      string     `json:"lastErrorCode" gorm:"column:last_error_code"`
	CompletedAt        *time.Time `json:"completedAt,omitempty" gorm:"column:completed_at"`
	UpdatedAt          time.Time  `json:"updatedAt" gorm:"column:updated_at"`
}

type ProjectionRow struct {
	ProjectionID           string     `json:"projectionId" gorm:"column:projection_id"`
	ReviewTaskID           *uint64    `json:"reviewTaskId,omitempty" gorm:"column:review_task_id"`
	Status                 string     `json:"status" gorm:"column:status"`
	AppliedDecisionVersion uint64     `json:"appliedDecisionVersion" gorm:"column:applied_decision_version"`
	FailureCode            string     `json:"failureCode" gorm:"column:failure_code"`
	SubmittedAt            *time.Time `json:"submittedAt,omitempty" gorm:"column:submitted_at"`
	ReviewingAt            *time.Time `json:"reviewingAt,omitempty" gorm:"column:reviewing_at"`
	DecidedAt              *time.Time `json:"decidedAt,omitempty" gorm:"column:decided_at"`
	UpdatedAt              time.Time  `json:"updatedAt" gorm:"column:updated_at"`
}

type DecisionRow struct {
	AssetPublicID string    `json:"assetId" gorm:"column:asset_public_id"`
	ProjectionID  string    `json:"projectionId" gorm:"column:projection_id"`
	ReviewTaskID  uint64    `json:"reviewTaskId" gorm:"column:review_task_id"`
	Version       uint64    `json:"decisionVersion" gorm:"column:decision_version"`
	Decision      string    `json:"decision" gorm:"column:decision"`
	PayloadHash   string    `json:"payloadHash" gorm:"column:payload_hash"`
	ResultCode    string    `json:"resultCode" gorm:"column:result_code"`
	ReceivedAt    time.Time `json:"receivedAt" gorm:"column:received_at"`
}

type EventRow struct {
	EventID          string     `json:"eventId" gorm:"column:event_id"`
	AggregateID      string     `json:"aggregateId" gorm:"column:aggregate_id"`
	AggregateVersion uint64     `json:"aggregateVersion" gorm:"column:aggregate_version"`
	Operation        string     `json:"operation" gorm:"column:operation"`
	ConsumerStatus   string     `json:"consumerStatus" gorm:"column:consumer_status"`
	AttemptCount     uint32     `json:"attemptCount" gorm:"column:attempt_count"`
	LastErrorCode    string     `json:"lastErrorCode" gorm:"column:last_error_code"`
	NextAttemptAt    *time.Time `json:"nextAttemptAt,omitempty" gorm:"column:next_attempt_at"`
	ProcessedAt      *time.Time `json:"processedAt,omitempty" gorm:"column:processed_at"`
	CreatedAt        time.Time  `json:"createdAt" gorm:"column:created_at"`
}

type CheckpointRow struct {
	AggregateID          string     `json:"aggregateId" gorm:"column:aggregate_id"`
	LastEventID          string     `json:"lastEventId" gorm:"column:last_event_id"`
	LastAggregateVersion uint64     `json:"lastAggregateVersion" gorm:"column:last_aggregate_version"`
	LastOperation        string     `json:"lastOperation" gorm:"column:last_operation"`
	ActiveEventID        *string    `json:"activeEventId,omitempty" gorm:"column:active_event_id"`
	LeaseExpiresAt       *time.Time `json:"leaseExpiresAt,omitempty" gorm:"column:lease_expires_at"`
	UpdatedAt            time.Time  `json:"updatedAt" gorm:"column:updated_at"`
}

type GCJobRow struct {
	Kind              string     `json:"kind" gorm:"column:kind"`
	JobID             string     `json:"jobId" gorm:"column:job_id"`
	AssetPublicID     string     `json:"assetId" gorm:"column:asset_public_id"`
	ObjectStage       string     `json:"objectStage,omitempty" gorm:"column:object_stage"`
	Status            string     `json:"status" gorm:"column:status"`
	AttemptCount      uint32     `json:"attemptCount" gorm:"column:attempt_count"`
	LastErrorCode     string     `json:"lastErrorCode" gorm:"column:last_error_code"`
	ProviderRequestID string     `json:"providerRequestId,omitempty" gorm:"column:provider_request_id"`
	DeletedAt         *time.Time `json:"deletedAt,omitempty" gorm:"column:deleted_at"`
	CreatedAt         time.Time  `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt         time.Time  `json:"updatedAt" gorm:"column:updated_at"`
}

type AssetDetail struct {
	Asset      AssetRow       `json:"asset"`
	References []ReferenceRow `json:"references"`
	Ticket     *TicketRow     `json:"ticket,omitempty"`
	Completion *CompletionRow `json:"completion,omitempty"`
	Projection *ProjectionRow `json:"projection,omitempty"`
	Decisions  []DecisionRow  `json:"decisions"`
	Events     []EventRow     `json:"events"`
	Checkpoint *CheckpointRow `json:"checkpoint,omitempty"`
	GCJobs     []GCJobRow     `json:"gcJobs"`
}

type DeadletterView struct {
	Consumer Page[EventRow] `json:"consumer"`
	GC       Page[GCJobRow] `json:"gc"`
}

var (
	ErrInvalidQuery = errors.New("invalid media admin query")
	ErrNotFound     = errors.New("media admin record not found")
)
