package media

import (
	"context"
	"errors"
	"time"
)

type GCStatus string

const (
	GCPending    GCStatus = "pending"
	GCProcessing GCStatus = "processing"
	GCRetryWait  GCStatus = "retry_wait"
	GCSucceeded  GCStatus = "succeeded"
	GCDeadletter GCStatus = "deadletter"
)

type GCObjectStage string

const (
	GCStageStaging GCObjectStage = "staging"
	GCStageFinal   GCObjectStage = "final"
)

type GCJob struct {
	JobID             string        `gorm:"column:job_id;size:26;primaryKey"`
	AssetID           uint64        `gorm:"column:asset_id;not null;uniqueIndex:uq_media_gc_object_identity,priority:1;index:idx_media_gc_asset_status,priority:1"`
	ObjectStage       GCObjectStage `gorm:"column:object_stage;size:16;not null;uniqueIndex:uq_media_gc_object_identity,priority:2;index:idx_media_gc_asset_status,priority:3"`
	ObjectKey         string        `gorm:"column:object_key;size:512;not null;uniqueIndex:uq_media_gc_object_identity,priority:3"`
	ObjectVersionID   string        `gorm:"column:object_version_id;size:128;not null;uniqueIndex:uq_media_gc_object_identity,priority:4"`
	Status            GCStatus      `gorm:"column:status;size:24;not null;index:idx_media_gc_claim,priority:1;index:idx_media_gc_asset_status,priority:2"`
	LeaseToken        *string       `gorm:"column:lease_token;size:26"`
	LeaseExpiresAt    *time.Time    `gorm:"column:lease_expires_at;index:idx_media_gc_claim,priority:3"`
	AttemptCount      uint32        `gorm:"column:attempt_count;not null"`
	NextAttemptAt     *time.Time    `gorm:"column:next_attempt_at;index:idx_media_gc_claim,priority:2"`
	LastErrorCode     string        `gorm:"column:last_error_code;size:64;not null"`
	ProviderRequestID string        `gorm:"column:provider_request_id;size:128;not null"`
	DeletedAt         *time.Time    `gorm:"column:deleted_at"`
	RowVersion        uint64        `gorm:"column:row_version;not null"`
	CreatedAt         time.Time     `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time     `gorm:"column:updated_at;not null"`
}

func (GCJob) TableName() string { return "media_gc_jobs" }

type GCAuxiliaryJob struct {
	JobID          string     `gorm:"column:job_id;size:26;primaryKey"`
	AssetID        uint64     `gorm:"column:asset_id;not null;uniqueIndex:uq_media_gc_aux_asset"`
	Status         GCStatus   `gorm:"column:status;size:24;not null;index:idx_media_gc_aux_claim,priority:1"`
	LeaseToken     *string    `gorm:"column:lease_token;size:26"`
	LeaseExpiresAt *time.Time `gorm:"column:lease_expires_at;index:idx_media_gc_aux_claim,priority:2"`
	AttemptCount   uint32     `gorm:"column:attempt_count;not null"`
	LastErrorCode  string     `gorm:"column:last_error_code;size:64;not null"`
	PurgedAt       *time.Time `gorm:"column:purged_at"`
	RowVersion     uint64     `gorm:"column:row_version;not null"`
	CreatedAt      time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;not null"`
}

func (GCAuxiliaryJob) TableName() string { return "media_gc_auxiliary_jobs" }

type GCDelivery struct {
	Job        GCJob
	LeaseToken string
}

type GCDeleteResult struct {
	ProviderRequestID string
	AlreadyMissing    bool
}

// GCObjectStore can only delete a single, immutable object version. There is
// deliberately no API for deleting latest objects, prefixes, or buckets.
type GCObjectStore interface {
	DeleteVersion(context.Context, string, string) (GCDeleteResult, error)
}

type GCPolicy struct {
	StagingRetention   time.Duration
	RejectedRetention  time.Duration
	ApprovedRetention  time.Duration
	AuxiliaryRetention time.Duration
	LeaseTTL           time.Duration
	RetryDelay         time.Duration
	MaxAttempts        uint32
}

type GCBatchResult struct {
	Claimed     int
	Succeeded   int
	Retried     int
	Deadletters int
}

type GCAuxiliaryPurgeResult struct {
	Reservations int64
	Completions  int64
	Tickets      int64
}

var (
	ErrGCInvariant       = errors.New("media GC invariant violated")
	ErrGCNotEligible     = errors.New("media asset is not eligible for garbage collection")
	ErrGCLease           = errors.New("media GC lease is stale or expired")
	ErrGCFixedIdentity   = errors.New("media GC requires an exact key and versionId")
	ErrGCReferenceExists = errors.New("media GC blocked by a non-revoked reference fact")
)
