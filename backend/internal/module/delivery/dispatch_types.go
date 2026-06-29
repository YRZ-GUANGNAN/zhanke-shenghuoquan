package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrForbiddenOrder      = errors.New("delivery order not found")
	ErrRiderNotRegistered  = errors.New("rider is not registered")
	ErrRiderUnavailable    = errors.New("rider is not active")
	ErrOutboxLease         = errors.New("delivery outbox lease lost")
	ErrOutboxPayload       = errors.New("invalid delivery outbox payload")
	ErrIdempotencyConflict = errors.New("delivery idempotency conflict")
)

type RiderEligibility interface {
	RequireActive(context.Context, uint64) error
}

type OutboxStatus string

const (
	OutboxPending    OutboxStatus = "pending"
	OutboxProcessing OutboxStatus = "processing"
	OutboxRetryWait  OutboxStatus = "retry_wait"
	OutboxSucceeded  OutboxStatus = "succeeded"
	OutboxSkipped    OutboxStatus = "skipped"
	OutboxDeadletter OutboxStatus = "deadletter"
)

const EventRiderPoolOpened = "RiderPoolOpened"

type OutboxEvent struct {
	ID                uint64          `gorm:"column:id;primaryKey"`
	EventPublicID     string          `gorm:"column:event_public_id;size:26;not null;uniqueIndex:uq_delivery_outbox_event_public_id"`
	AggregatePublicID string          `gorm:"column:aggregate_public_id;size:26;not null;uniqueIndex:uq_delivery_outbox_aggregate_event,priority:1"`
	AggregateVersion  uint64          `gorm:"column:aggregate_version;not null;uniqueIndex:uq_delivery_outbox_aggregate_event,priority:2"`
	EventType         string          `gorm:"column:event_type;size:48;not null;uniqueIndex:uq_delivery_outbox_aggregate_event,priority:3"`
	Payload           json.RawMessage `gorm:"column:payload;type:json;not null"`
	Status            OutboxStatus    `gorm:"column:status;size:24;not null;index:idx_delivery_outbox_claim,priority:1"`
	AttemptCount      uint32          `gorm:"column:attempt_count;not null"`
	NextAttemptAt     *time.Time      `gorm:"column:next_attempt_at;index:idx_delivery_outbox_claim,priority:2"`
	LeaseToken        *string         `gorm:"column:lease_token;size:26"`
	LeaseExpiresAt    *time.Time      `gorm:"column:lease_expires_at;index:idx_delivery_outbox_claim,priority:3"`
	LastErrorCode     string          `gorm:"column:last_error_code;size:64;not null"`
	LastError         string          `gorm:"column:last_error;size:512;not null"`
	ProcessedAt       *time.Time      `gorm:"column:processed_at"`
	RowVersion        uint64          `gorm:"column:row_version;not null"`
	CreatedAt         time.Time       `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time       `gorm:"column:updated_at;not null"`
}

func (OutboxEvent) TableName() string { return "delivery_outbox" }

type RiderPoolOpenedPayload struct {
	OrderPublicID    string `json:"orderPublicId"`
	RouteSummary     string `json:"routeSummary"`
	DeliveryFeeCents uint64 `json:"deliveryFeeCents"`
}

type OutboxDelivery struct {
	Event      OutboxEvent
	LeaseToken string
}

type Page struct {
	Page int
	Size int
}

func (p Page) Valid() bool {
	return p.Page > 0 && p.Size > 0 && p.Size <= 100
}
