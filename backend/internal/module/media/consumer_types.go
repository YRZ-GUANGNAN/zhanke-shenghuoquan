package media

import (
	"errors"
	"time"
)

type ConsumerStatus string

const (
	ConsumerPending    ConsumerStatus = "pending"
	ConsumerProcessing ConsumerStatus = "processing"
	ConsumerRetryWait  ConsumerStatus = "retry_wait"
	ConsumerSucceeded  ConsumerStatus = "succeeded"
	ConsumerDeadletter ConsumerStatus = "deadletter"
)

type ConsumerInbox struct {
	EventID          string         `gorm:"column:event_id;size:26;primaryKey"`
	AggregateID      string         `gorm:"column:aggregate_id;size:26;not null;index:idx_media_event_inbox_aggregate,priority:1"`
	AggregateVersion uint64         `gorm:"column:aggregate_version;not null;index:idx_media_event_inbox_aggregate,priority:2"`
	Operation        string         `gorm:"column:operation;size:48;not null"`
	PayloadHash      string         `gorm:"column:payload_hash;size:64;not null"`
	Status           ConsumerStatus `gorm:"column:status;size:24;not null;index:idx_media_event_inbox_claim,priority:1"`
	LeaseToken       *string        `gorm:"column:lease_token;size:26"`
	LeaseExpiresAt   *time.Time     `gorm:"column:lease_expires_at;index:idx_media_event_inbox_claim,priority:3"`
	AttemptCount     uint32         `gorm:"column:attempt_count;not null"`
	NextAttemptAt    *time.Time     `gorm:"column:next_attempt_at;index:idx_media_event_inbox_claim,priority:2"`
	LastErrorCode    string         `gorm:"column:last_error_code;size:64;not null"`
	ProcessedAt      *time.Time     `gorm:"column:processed_at"`
	RowVersion       uint64         `gorm:"column:row_version;not null"`
	CreatedAt        time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time      `gorm:"column:updated_at;not null"`
}

func (ConsumerInbox) TableName() string { return "media_event_inbox" }

type ConsumerCheckpoint struct {
	AggregateID          string     `gorm:"column:aggregate_id;size:26;primaryKey"`
	LastEventID          string     `gorm:"column:last_event_id;size:26;not null"`
	LastAggregateVersion uint64     `gorm:"column:last_aggregate_version;not null"`
	LastOperation        string     `gorm:"column:last_operation;size:48;not null"`
	LastPayloadHash      string     `gorm:"column:last_payload_hash;size:64;not null"`
	ActiveEventID        *string    `gorm:"column:active_event_id;size:26"`
	LeaseToken           *string    `gorm:"column:lease_token;size:26"`
	LeaseExpiresAt       *time.Time `gorm:"column:lease_expires_at"`
	RowVersion           uint64     `gorm:"column:row_version;not null"`
	CreatedAt            time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt            time.Time  `gorm:"column:updated_at;not null"`
}

func (ConsumerCheckpoint) TableName() string { return "media_consumer_checkpoints" }

type ConsumerDelivery struct {
	Event      OutboxEvent
	Inbox      ConsumerInbox
	LeaseToken string
}

type ConsumerOrder string

const (
	ConsumerOrderReady      ConsumerOrder = "ready"
	ConsumerOrderDuplicate  ConsumerOrder = "duplicate"
	ConsumerOrderOutOfOrder ConsumerOrder = "out_of_order"
	ConsumerOrderConflict   ConsumerOrder = "conflict"
)

type ConsumerBatchResult struct {
	Claimed     int
	Succeeded   int
	Duplicates  int
	Retried     int
	Deadletters int
}

const (
	ConsumerOperationSubmit    = "ReviewProjectionSubmitted"
	ConsumerOperationReviewing = "ReviewProjectionReviewing"
	ConsumerOperationDecision  = "ReviewProjectionDecision"
)

type ConsumerEventPayload struct {
	ProjectionID       string           `json:"projectionId"`
	ReviewTaskID       uint64           `json:"reviewTaskId,omitempty"`
	ExpectedRowVersion uint64           `json:"expectedRowVersion"`
	ExpectedStatus     ProjectionStatus `json:"expectedStatus,omitempty"`
	DecisionVersion    uint64           `json:"decisionVersion,omitempty"`
	Decision           ProjectionStatus `json:"decision,omitempty"`
	PayloadHash        string           `json:"payloadHash,omitempty"`
	ResultCode         string           `json:"resultCode,omitempty"`
}

var (
	ErrConsumerPayload    = errors.New("invalid media consumer payload")
	ErrConsumerOutOfOrder = errors.New("media consumer event out of order")
	ErrConsumerConflict   = errors.New("media consumer event conflict")
	ErrConsumerLease      = errors.New("media consumer lease lost")
)
