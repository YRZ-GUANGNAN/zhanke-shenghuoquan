// Package media is the isolated P0-4 Media Domain. It does not import or query
// user, community, trade, or other business-domain models.
package media

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Purpose string

const (
	PurposePost                Purpose = "post"
	PurposeProduct             Purpose = "product"
	PurposeAvatar              Purpose = "avatar"
	PurposeDish                Purpose = "dish"
	PurposeMerchantLogo        Purpose = "merchant_logo"
	PurposeDeliveryProof       Purpose = "delivery_proof"
	PurposeDeliveryIM          Purpose = "delivery_im"
	PurposeFeedback            Purpose = "feedback"
	PurposeIdentityApplication Purpose = "identity_application"
)

func (v Purpose) Valid() bool {
	switch v {
	case PurposePost, PurposeProduct, PurposeAvatar, PurposeDish, PurposeMerchantLogo, PurposeDeliveryProof, PurposeDeliveryIM, PurposeFeedback, PurposeIdentityApplication:
		return true
	default:
		return false
	}
}

// PubliclyResolvable is an explicit privacy boundary for the generic mini-app
// media endpoints. Delivery proof photos are resolved only after the Delivery
// participant fence has been checked.
func (v Purpose) PubliclyResolvable() bool {
	switch v {
	case PurposePost, PurposeProduct, PurposeAvatar, PurposeDish, PurposeMerchantLogo:
		return true
	default:
		return false
	}
}

type StorageStage string

const (
	StorageStageStaging StorageStage = "staging"
	StorageStageFinal   StorageStage = "final"
)

type ReviewStatus string

const (
	ReviewPending  ReviewStatus = "pending"
	ReviewApproved ReviewStatus = "approved"
	ReviewRejected ReviewStatus = "rejected"
	ReviewFailed   ReviewStatus = "failed"
)

type TechnicalStatus string

const (
	TechnicalPendingUpload    TechnicalStatus = "pending_upload"
	TechnicalUploaded         TechnicalStatus = "uploaded"
	TechnicalValidating       TechnicalStatus = "validating"
	TechnicalPromoting        TechnicalStatus = "promoting"
	TechnicalFinalReady       TechnicalStatus = "final_ready"
	TechnicalSubmitted        TechnicalStatus = "submitted"
	TechnicalReviewing        TechnicalStatus = "reviewing"
	TechnicalApproved         TechnicalStatus = "approved"
	TechnicalRejected         TechnicalStatus = "rejected"
	TechnicalReviewFailed     TechnicalStatus = "review_failed"
	TechnicalDeletePending    TechnicalStatus = "delete_pending"
	TechnicalDeleteDeadletter TechnicalStatus = "delete_deadletter"
	TechnicalDeleted          TechnicalStatus = "deleted"
)

type ReferenceStatus string

const (
	ReferencePending  ReferenceStatus = "pending"
	ReferenceActive   ReferenceStatus = "active"
	ReferenceRevoking ReferenceStatus = "revoking"
	ReferenceRevoked  ReferenceStatus = "revoked"
)

type ParentVisibility string

const (
	ParentHidden  ParentVisibility = "hidden"
	ParentVisible ParentVisibility = "visible"
)

type TicketStatus string

const (
	TicketCreated             TicketStatus = "created"
	TicketAuthorizationIssued TicketStatus = "authorization_issued"
	TicketUploaded            TicketStatus = "uploaded"
	TicketConsumed            TicketStatus = "consumed"
	TicketExpired             TicketStatus = "expired"
	TicketCancelled           TicketStatus = "cancelled"
)

type ReservationStatus string

const (
	ReservationActive    ReservationStatus = "active"
	ReservationCommitted ReservationStatus = "committed"
	ReservationReleased  ReservationStatus = "released"
)

type Asset struct {
	ID                     uint64          `gorm:"column:id;primaryKey"`
	AssetPublicID          string          `gorm:"column:asset_public_id;size:26;not null;uniqueIndex:uq_media_asset_public_id_v11"`
	OwnerID                uint64          `gorm:"column:owner_id;not null;uniqueIndex:uq_media_owner_idempotency_v11,priority:1"`
	Purpose                Purpose         `gorm:"column:purpose;size:24;not null"`
	StorageStage           StorageStage    `gorm:"column:storage_stage;size:16;not null"`
	ReviewStatus           ReviewStatus    `gorm:"column:review_status;size:16;not null"`
	TechnicalStatus        TechnicalStatus `gorm:"column:technical_status;size:24;not null;index:idx_media_technical_upload_v11,priority:1;index:idx_media_review_deadline_v11,priority:1;index:idx_media_delete_status_v11,priority:1"`
	TrustedSHA256          *string         `gorm:"column:trusted_sha256;size:64"`
	VerifiedMIME           string          `gorm:"column:verified_mime;size:64;not null"`
	ByteSize               uint64          `gorm:"column:byte_size;not null"`
	Width                  uint32          `gorm:"column:width;not null"`
	Height                 uint32          `gorm:"column:height;not null"`
	StagingKey             *string         `gorm:"column:staging_key;size:512;uniqueIndex:uq_media_staging_key_v11"`
	StagingVersionID       *string         `gorm:"column:staging_version_id;size:128"`
	FinalKey               *string         `gorm:"column:final_key;size:512;uniqueIndex:uq_media_final_key_v11"`
	FinalVersionID         *string         `gorm:"column:final_version_id;size:128"`
	ReviewTaskID           *uint64         `gorm:"column:review_task_id"`
	AppliedDecisionVersion uint64          `gorm:"column:applied_decision_version;not null"`
	ProjectedDecision      ReviewStatus    `gorm:"column:projected_decision;size:16;not null"`
	ProjectedDecidedAt     *time.Time      `gorm:"column:projected_decided_at"`
	ActiveRefCount         uint64          `gorm:"column:active_ref_count;not null"`
	IdempotencyKey         string          `gorm:"column:idempotency_key;size:96;not null;uniqueIndex:uq_media_owner_idempotency_v11,priority:2"`
	UploadExpiresAt        *time.Time      `gorm:"column:upload_expires_at;index:idx_media_technical_upload_v11,priority:2"`
	ReviewDeadlineAt       *time.Time      `gorm:"column:review_deadline_at;index:idx_media_review_deadline_v11,priority:2"`
	DeleteRequestedAt      *time.Time      `gorm:"column:delete_requested_at;index:idx_media_delete_status_v11,priority:2"`
	DeletedAt              *time.Time      `gorm:"column:deleted_at"`
	RowVersion             uint64          `gorm:"column:row_version;not null"`
	CreatedAt              time.Time       `gorm:"column:created_at;not null"`
	UpdatedAt              time.Time       `gorm:"column:updated_at;not null"`
}

func (Asset) TableName() string { return "media_assets" }

func (a Asset) ValidateNew() error {
	if !ValidULID(a.AssetPublicID) {
		return fmt.Errorf("asset_public_id: %w", ErrInvalidULID)
	}
	if a.OwnerID == 0 || !a.Purpose.Valid() {
		return ErrInvalidRecord
	}
	if a.StorageStage != StorageStageStaging || a.ReviewStatus != ReviewPending || a.TechnicalStatus != TechnicalPendingUpload {
		return ErrInvalidInitialState
	}
	if a.IdempotencyKey == "" {
		return ErrInvalidRecord
	}
	return nil
}

type AssetReference struct {
	ID               uint64           `gorm:"column:id;primaryKey"`
	AssetID          uint64           `gorm:"column:asset_id;not null;uniqueIndex:uq_media_reference_asset_slot,priority:1;index:idx_media_reference_asset_status,priority:1"`
	ReferenceType    Purpose          `gorm:"column:reference_type;size:24;not null;uniqueIndex:uq_media_reference_slot,priority:1;uniqueIndex:uq_media_reference_asset_slot,priority:2;index:idx_media_reference_parent_status,priority:1"`
	ReferenceID      string           `gorm:"column:reference_id;size:64;not null;uniqueIndex:uq_media_reference_slot,priority:2;uniqueIndex:uq_media_reference_asset_slot,priority:3;index:idx_media_reference_parent_status,priority:2"`
	Slot             uint16           `gorm:"column:slot;not null;uniqueIndex:uq_media_reference_slot,priority:3;uniqueIndex:uq_media_reference_asset_slot,priority:4"`
	Status           ReferenceStatus  `gorm:"column:status;size:16;not null;index:idx_media_reference_asset_status,priority:2;index:idx_media_reference_parent_status,priority:3"`
	ParentVisibility ParentVisibility `gorm:"column:parent_visibility;size:16;not null"`
	AggregateVersion uint64           `gorm:"column:aggregate_version;not null"`
	OperationID      string           `gorm:"column:operation_id;size:26;not null;index:idx_media_reference_operation"`
	RevocationFence  *string          `gorm:"column:revocation_fence;size:26"`
	FenceExpiresAt   *time.Time       `gorm:"column:fence_expires_at"`
	RowVersion       uint64           `gorm:"column:row_version;not null"`
	CreatedAt        time.Time        `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time        `gorm:"column:updated_at;not null"`
}

func (AssetReference) TableName() string { return "media_asset_references" }

type UploadTicket struct {
	TicketID             string       `gorm:"column:ticket_id;size:26;primaryKey;index:idx_media_ticket_expiry_scan,priority:3"`
	AssetID              uint64       `gorm:"column:asset_id;not null;uniqueIndex:uq_media_ticket_asset"`
	OwnerID              uint64       `gorm:"column:owner_id;not null;index:idx_media_ticket_owner_status,priority:1"`
	ExpectedSize         uint64       `gorm:"column:expected_size;not null"`
	ExpectedMD5          string       `gorm:"column:expected_md5;size:24;not null"`
	ExpectedContentType  string       `gorm:"column:expected_content_type;size:64;not null"`
	Status               TicketStatus `gorm:"column:status;size:24;not null;index:idx_media_ticket_owner_status,priority:2;index:idx_media_ticket_expiry,priority:1;index:idx_media_ticket_expiry_scan,priority:1"`
	CredentialIssueCount uint8        `gorm:"column:credential_issue_count;not null"`
	ExpiresAt            time.Time    `gorm:"column:expires_at;not null;index:idx_media_ticket_owner_status,priority:3;index:idx_media_ticket_expiry,priority:2;index:idx_media_ticket_expiry_scan,priority:2"`
	RowVersion           uint64       `gorm:"column:row_version;not null"`
	CreatedAt            time.Time    `gorm:"column:created_at;not null"`
	UpdatedAt            time.Time    `gorm:"column:updated_at;not null"`
}

func (UploadTicket) TableName() string { return "media_upload_tickets" }

type QuotaReservation struct {
	ReservationID string            `gorm:"column:reservation_id;size:26;primaryKey"`
	OwnerID       uint64            `gorm:"column:owner_id;not null;index:idx_media_quota_owner_status,priority:1;index:idx_media_quota_owner_created,priority:1"`
	BytesReserved uint64            `gorm:"column:bytes_reserved;not null"`
	CountReserved uint32            `gorm:"column:count_reserved;not null"`
	Status        ReservationStatus `gorm:"column:status;size:16;not null;index:idx_media_quota_owner_status,priority:2;index:idx_media_quota_owner_created,priority:3"`
	TicketID      string            `gorm:"column:ticket_id;size:26;not null;uniqueIndex:uq_media_quota_ticket"`
	RowVersion    uint64            `gorm:"column:row_version;not null"`
	CreatedAt     time.Time         `gorm:"column:created_at;not null;index:idx_media_quota_owner_created,priority:2"`
	UpdatedAt     time.Time         `gorm:"column:updated_at;not null"`
}

func (QuotaReservation) TableName() string { return "media_quota_reservations" }

// JSONDocument keeps the repository independent from GORM's optional datatypes package.
type JSONDocument []byte

func (j JSONDocument) Value() (driver.Value, error) {
	if len(j) == 0 || !json.Valid(j) {
		return nil, ErrInvalidJSON
	}
	return []byte(j), nil
}

func (j *JSONDocument) Scan(value any) error {
	if value == nil {
		return errors.New("media outbox payload cannot be null")
	}
	var raw []byte
	switch typed := value.(type) {
	case []byte:
		raw = append([]byte(nil), typed...)
	case string:
		raw = []byte(typed)
	default:
		return fmt.Errorf("unsupported JSON source %T", value)
	}
	if !json.Valid(raw) {
		return ErrInvalidJSON
	}
	*j = raw
	return nil
}

type OutboxEvent struct {
	EventID          string       `gorm:"column:event_id;size:26;primaryKey"`
	AggregateID      string       `gorm:"column:aggregate_id;size:26;not null;uniqueIndex:uq_media_outbox_aggregate_operation,priority:1"`
	AggregateVersion uint64       `gorm:"column:aggregate_version;not null;uniqueIndex:uq_media_outbox_aggregate_operation,priority:2"`
	Operation        string       `gorm:"column:operation;size:48;not null;uniqueIndex:uq_media_outbox_aggregate_operation,priority:3"`
	Payload          JSONDocument `gorm:"column:payload;type:json;not null"`
	ProcessedAt      *time.Time   `gorm:"column:processed_at;index:idx_media_outbox_pending,priority:1"`
	RowVersion       uint64       `gorm:"column:row_version;not null"`
	CreatedAt        time.Time    `gorm:"column:created_at;not null;index:idx_media_outbox_pending,priority:2"`
	UpdatedAt        time.Time    `gorm:"column:updated_at;not null"`
}

func (OutboxEvent) TableName() string { return "media_outbox" }

var (
	ErrInvalidRecord       = errors.New("invalid media domain record")
	ErrInvalidInitialState = errors.New("invalid media initial state")
	ErrInvalidJSON         = errors.New("invalid JSON document")
)
