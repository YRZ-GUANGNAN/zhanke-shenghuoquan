package delivery

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrRiderPayoutReserved       = errors.New("rider payout is reserved in payout queue")
	ErrPayoutQueueNotCancellable = errors.New("rider payout queue is not cancellable")
	ErrPayoutQueueUnavailable    = errors.New("rider payout queue is unavailable")
	ErrPayoutQueueSourceBlocked  = errors.New("rider payout source is blocked")
)

type RiderPayoutQueueStatus string

const (
	RiderPayoutQueueBlocked   RiderPayoutQueueStatus = "blocked"
	RiderPayoutQueueQueued    RiderPayoutQueueStatus = "queued"
	RiderPayoutQueueCancelled RiderPayoutQueueStatus = "cancelled"
)

func (s RiderPayoutQueueStatus) Valid() bool {
	return s == RiderPayoutQueueBlocked || s == RiderPayoutQueueQueued || s == RiderPayoutQueueCancelled
}

type RiderPayoutReservationStatus string

const (
	RiderPayoutReservationActive   RiderPayoutReservationStatus = "active"
	RiderPayoutReservationReleased RiderPayoutReservationStatus = "released"
)

func (s RiderPayoutReservationStatus) Valid() bool {
	return s == RiderPayoutReservationActive || s == RiderPayoutReservationReleased
}

type RiderPayoutQueueEventType string

const (
	RiderPayoutQueueEventBlocked           RiderPayoutQueueEventType = "blocked"
	RiderPayoutQueueEventQueued            RiderPayoutQueueEventType = "queued"
	RiderPayoutQueueEventCancelled         RiderPayoutQueueEventType = "cancelled"
	RiderPayoutQueueEventSourceInvalidated RiderPayoutQueueEventType = "source_invalidated"
)

func (s RiderPayoutQueueEventType) Valid() bool {
	return s == RiderPayoutQueueEventBlocked || s == RiderPayoutQueueEventQueued ||
		s == RiderPayoutQueueEventCancelled || s == RiderPayoutQueueEventSourceInvalidated
}

type RiderPayoutQueueActorType string

const (
	RiderPayoutQueueActorSystem RiderPayoutQueueActorType = "system"
	RiderPayoutQueueActorAdmin  RiderPayoutQueueActorType = "admin"
)

func (s RiderPayoutQueueActorType) Valid() bool {
	return s == RiderPayoutQueueActorSystem || s == RiderPayoutQueueActorAdmin
}

type RiderPayoutQueue struct {
	ID                    uint64                 `gorm:"column:id;primaryKey"`
	PayoutQueuePublicID   string                 `gorm:"column:payout_queue_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_payout_queue_public_id"`
	EarningPublicID       string                 `gorm:"column:earning_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_payout_queue_earning"`
	DeliveryOrderPublicID string                 `gorm:"column:delivery_order_public_id;size:26;not null"`
	RiderUserID           uint64                 `gorm:"column:rider_user_id;not null;index:idx_delivery_rider_payout_queue_rider_created,priority:1"`
	AmountCents           uint64                 `gorm:"column:amount_cents;not null"`
	Currency              string                 `gorm:"column:currency;size:8;not null"`
	Status                RiderPayoutQueueStatus `gorm:"column:status;size:16;not null;index:idx_delivery_rider_payout_queue_status_created,priority:1"`
	ReasonCode            string                 `gorm:"column:reason_code;size:64;not null"`
	SourcePolicyVersion   string                 `gorm:"column:source_policy_version;size:32;not null"`
	SourceEvidenceHash    string                 `gorm:"column:source_evidence_hash;size:64;not null"`
	RowVersion            uint64                 `gorm:"column:row_version;not null"`
	CancelledAt           *time.Time             `gorm:"column:cancelled_at"`
	CreatedAt             time.Time              `gorm:"column:created_at;not null;index:idx_delivery_rider_payout_queue_rider_created,priority:2;index:idx_delivery_rider_payout_queue_status_created,priority:2"`
	UpdatedAt             time.Time              `gorm:"column:updated_at;not null"`
}

func (RiderPayoutQueue) TableName() string { return "delivery_rider_payout_queue" }

type RiderPayoutReservation struct {
	ID                  uint64                       `gorm:"column:id;primaryKey"`
	ReservationPublicID string                       `gorm:"column:reservation_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_payout_reservation_public_id"`
	PayoutQueuePublicID string                       `gorm:"column:payout_queue_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_payout_reservation_queue"`
	EarningPublicID     string                       `gorm:"column:earning_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_payout_reservation_earning"`
	WalletPublicID      string                       `gorm:"column:wallet_public_id;size:26;not null"`
	RiderUserID         uint64                       `gorm:"column:rider_user_id;not null;index:idx_delivery_rider_payout_reservation_rider_status,priority:1"`
	AmountCents         uint64                       `gorm:"column:amount_cents;not null"`
	Status              RiderPayoutReservationStatus `gorm:"column:status;size:16;not null"`
	RowVersion          uint64                       `gorm:"column:row_version;not null"`
	ReservedAt          time.Time                    `gorm:"column:reserved_at;not null"`
	ReleasedAt          *time.Time                   `gorm:"column:released_at"`
}

func (RiderPayoutReservation) TableName() string { return "delivery_rider_payout_reservations" }

type RiderPayoutQueueEvent struct {
	ID                  uint64                    `gorm:"column:id;primaryKey"`
	EventPublicID       string                    `gorm:"column:event_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_payout_event_public_id"`
	PayoutQueuePublicID string                    `gorm:"column:payout_queue_public_id;size:26;not null;index:idx_delivery_rider_payout_event_queue_created,priority:1"`
	EventType           RiderPayoutQueueEventType `gorm:"column:event_type;size:24;not null"`
	ActorType           RiderPayoutQueueActorType `gorm:"column:actor_type;size:16;not null"`
	OperatorUserID      *uint64                   `gorm:"column:operator_user_id"`
	ReasonCode          string                    `gorm:"column:reason_code;size:64;not null"`
	Reason              string                    `gorm:"column:reason;size:256;not null"`
	IdempotencyKey      *string                   `gorm:"column:idempotency_key;size:26;uniqueIndex:uq_delivery_rider_payout_event_idempotency"`
	ExpectedRowVersion  *uint64                   `gorm:"column:expected_row_version"`
	CreatedAt           time.Time                 `gorm:"column:created_at;not null;index:idx_delivery_rider_payout_event_queue_created,priority:2"`
}

func (RiderPayoutQueueEvent) TableName() string { return "delivery_rider_payout_events" }

type RiderPayoutSourceState string

const (
	RiderPayoutSourceVerified RiderPayoutSourceState = "verified"
	RiderPayoutSourceBlocked  RiderPayoutSourceState = "blocked"
)

type RiderPayoutSourceSnapshot struct {
	State        RiderPayoutSourceState
	ReasonCode   string
	Policy       string
	EvidenceHash string
}

type RiderPayoutQueueResult struct {
	Queue       RiderPayoutQueue
	Reservation *RiderPayoutReservation
}

func validateRiderPayoutQueue(value RiderPayoutQueue) error {
	if !ValidULID(value.PayoutQueuePublicID) || !ValidULID(value.EarningPublicID) ||
		!ValidULID(value.DeliveryOrderPublicID) || value.RiderUserID == 0 || value.AmountCents == 0 ||
		value.Currency != "CNY" || !value.Status.Valid() || value.RowVersion == 0 ||
		strings.TrimSpace(value.SourcePolicyVersion) == "" || value.CreatedAt.IsZero() || value.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	if value.Status == RiderPayoutQueueCancelled {
		if value.CancelledAt == nil || value.CancelledAt.IsZero() {
			return ErrInvalidRecord
		}
	} else if value.CancelledAt != nil {
		return ErrInvalidRecord
	}
	return nil
}

func validateRiderPayoutReservation(value RiderPayoutReservation) error {
	if !ValidULID(value.ReservationPublicID) || !ValidULID(value.PayoutQueuePublicID) ||
		!ValidULID(value.EarningPublicID) || !ValidULID(value.WalletPublicID) || value.RiderUserID == 0 ||
		value.AmountCents == 0 || !value.Status.Valid() || value.RowVersion == 0 || value.ReservedAt.IsZero() {
		return ErrInvalidRecord
	}
	if value.Status == RiderPayoutReservationActive && value.ReleasedAt != nil {
		return ErrInvalidRecord
	}
	if value.Status == RiderPayoutReservationReleased && (value.ReleasedAt == nil || value.ReleasedAt.IsZero()) {
		return ErrInvalidRecord
	}
	return nil
}

func validateRiderPayoutQueueEvent(value RiderPayoutQueueEvent) error {
	if !ValidULID(value.EventPublicID) || !ValidULID(value.PayoutQueuePublicID) ||
		!value.EventType.Valid() || !value.ActorType.Valid() || value.CreatedAt.IsZero() ||
		utf8.RuneCountInString(value.Reason) > 256 {
		return ErrInvalidRecord
	}
	if value.ActorType == RiderPayoutQueueActorSystem && value.OperatorUserID != nil {
		return ErrInvalidRecord
	}
	if value.ActorType == RiderPayoutQueueActorAdmin && (value.OperatorUserID == nil || *value.OperatorUserID == 0) {
		return ErrInvalidRecord
	}
	if value.IdempotencyKey != nil && !ValidULID(*value.IdempotencyKey) {
		return ErrInvalidRecord
	}
	return nil
}

func riderPayoutReservedAmount(available, reserved uint64) (uint64, error) {
	if reserved > available {
		return 0, ErrFundingSourceMismatch
	}
	return available - reserved, nil
}
