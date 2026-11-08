package delivery

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrWithdrawalAlreadyProcessed      = errors.New("rider withdrawal already processed")
	ErrWithdrawalNotFound              = errors.New("rider withdrawal not found")
	ErrWithdrawalReasonRequired        = errors.New("rider withdrawal rejection reason is required")
	ErrWithdrawalPaymentRequired       = errors.New("manual payment reference is required")
	ErrWithdrawalAllocationUnavailable = errors.New("rider withdrawal allocation is unavailable")
	ErrWithdrawalNotClaimed            = errors.New("rider withdrawal is not claimed by this operator")
	ErrWithdrawalClaimConflict         = errors.New("rider withdrawal claim conflict")
	ErrWithdrawalSourceMismatch        = errors.New("rider withdrawal source changed")
	ErrWithdrawalEarningsInsufficient  = errors.New("settled rider earnings are insufficient")
	ErrWithdrawalPaymentReferenceConflict = errors.New("rider withdrawal payment reference is already bound")
)

const MaxRiderMoneyAmount uint64 = 9007199254740991

type RiderWithdrawalStatus string

const (
	RiderWithdrawalProcessing RiderWithdrawalStatus = "processing"
	RiderWithdrawalPaid       RiderWithdrawalStatus = "paid"
	RiderWithdrawalRejected   RiderWithdrawalStatus = "rejected"
)

func (s RiderWithdrawalStatus) Valid() bool {
	return s == RiderWithdrawalProcessing || s == RiderWithdrawalPaid || s == RiderWithdrawalRejected
}

type RiderWithdrawalAllocationStatus string

const (
	RiderWithdrawalAllocationInFlight RiderWithdrawalAllocationStatus = "in_flight"
	RiderWithdrawalAllocationPaid     RiderWithdrawalAllocationStatus = "paid"
	RiderWithdrawalAllocationReleased RiderWithdrawalAllocationStatus = "released"
)

func (s RiderWithdrawalAllocationStatus) Valid() bool {
	return s == RiderWithdrawalAllocationInFlight || s == RiderWithdrawalAllocationPaid || s == RiderWithdrawalAllocationReleased
}

type RiderWithdrawalEventType string

const (
	RiderWithdrawalEventClaimed   RiderWithdrawalEventType = "claimed"
	RiderWithdrawalEventReleased  RiderWithdrawalEventType = "released"
	RiderWithdrawalEventPaid      RiderWithdrawalEventType = "paid"
	RiderWithdrawalEventRejected  RiderWithdrawalEventType = "rejected"
	RiderWithdrawalEventException RiderWithdrawalEventType = "exception"
)

func (t RiderWithdrawalEventType) Valid() bool {
	return t == RiderWithdrawalEventClaimed || t == RiderWithdrawalEventReleased || t == RiderWithdrawalEventPaid || t == RiderWithdrawalEventRejected || t == RiderWithdrawalEventException
}

type RiderWithdrawal struct {
	ID               uint64                `gorm:"column:id;primaryKey"`
	WithdrawPublicID string                `gorm:"column:withdraw_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_withdrawal_public_id"`
	RiderUserID      uint64                `gorm:"column:rider_user_id;not null;index:idx_delivery_rider_withdrawal_rider_status_created,priority:1"`
	WalletPublicID   string                `gorm:"column:wallet_public_id;size:26;not null;index:idx_delivery_rider_withdrawal_wallet"`
	AmountCents      uint64                `gorm:"column:amount_cents;not null"`
	Currency         string                `gorm:"column:currency;size:8;not null"`
	Status           RiderWithdrawalStatus `gorm:"column:status;size:16;not null;index:idx_delivery_rider_withdrawal_status_created,priority:1;index:idx_delivery_rider_withdrawal_rider_status_created,priority:2"`
	Reason           string                `gorm:"column:reason;size:256;not null"`
	IdempotencyKey   string                `gorm:"column:idempotency_key;size:26;not null;uniqueIndex:uq_delivery_rider_withdrawal_idempotency"`
	RowVersion       uint64                `gorm:"column:row_version;not null"`
	ProcessedBy      *uint64               `gorm:"column:processed_by"`
	ProcessKey       *string               `gorm:"column:process_key;size:26;uniqueIndex:uq_delivery_rider_withdrawal_process_key"`
	ProcessVersion   *uint64               `gorm:"column:process_version"`
	PaymentReference string                `gorm:"column:payment_reference;size:128;not null"`
	ProcessedAt      *time.Time            `gorm:"column:processed_at"`
	CreatedAt        time.Time             `gorm:"column:created_at;not null;index:idx_delivery_rider_withdrawal_status_created,priority:2;index:idx_delivery_rider_withdrawal_rider_status_created,priority:3"`
	UpdatedAt        time.Time             `gorm:"column:updated_at;not null"`
}

type RiderWithdrawalAllocation struct {
	ID                    uint64                          `gorm:"column:id;primaryKey"`
	AllocationPublicID    string                          `gorm:"column:allocation_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_withdrawal_allocation_public_id"`
	WithdrawPublicID      string                          `gorm:"column:withdraw_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_withdrawal_allocation_pair,priority:1"`
	EarningPublicID       string                          `gorm:"column:earning_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_withdrawal_allocation_pair,priority:2"`
	DeliveryOrderPublicID string                          `gorm:"column:delivery_order_public_id;size:26;not null"`
	RiderUserID           uint64                          `gorm:"column:rider_user_id;not null"`
	WalletPublicID        string                          `gorm:"column:wallet_public_id;size:26;not null"`
	AmountCents           uint64                          `gorm:"column:amount_cents;not null"`
	Currency              string                          `gorm:"column:currency;size:8;not null"`
	Status                RiderWithdrawalAllocationStatus `gorm:"column:status;size:16;not null"`
	SourcePolicyVersion   string                          `gorm:"column:source_policy_version;size:32;not null"`
	SourceEvidenceHash    string                          `gorm:"column:source_evidence_hash;size:64;not null"`
	EarningRowVersion     uint64                          `gorm:"column:earning_row_version;not null"`
	CreatedAt             time.Time                       `gorm:"column:created_at;not null"`
	UpdatedAt             time.Time                       `gorm:"column:updated_at;not null"`
}

func (RiderWithdrawalAllocation) TableName() string { return "delivery_rider_withdrawal_allocations" }

type RiderWithdrawalEvent struct {
	ID                 uint64                   `gorm:"column:id;primaryKey"`
	EventPublicID      string                   `gorm:"column:event_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_withdrawal_event_public_id"`
	WithdrawPublicID   string                   `gorm:"column:withdraw_public_id;size:26;not null"`
	EventType          RiderWithdrawalEventType `gorm:"column:event_type;size:24;not null"`
	OperatorUserID     uint64                   `gorm:"column:operator_user_id;not null"`
	Reason             string                   `gorm:"column:reason;size:256;not null"`
	IdempotencyKey     string                   `gorm:"column:idempotency_key;size:26;not null;uniqueIndex:uq_delivery_rider_withdrawal_event_idempotency"`
	PaymentReference   string                   `gorm:"column:payment_reference;size:128;not null"`
	ClaimKey           string                   `gorm:"column:claim_key;size:26;not null"`
	RequestHash        string                   `gorm:"column:request_hash;size:64;not null"`
	ExpectedRowVersion uint64                   `gorm:"column:expected_row_version;not null"`
	CreatedAt          time.Time                `gorm:"column:created_at;not null"`
}

func (RiderWithdrawalEvent) TableName() string { return "delivery_rider_withdrawal_events" }

type RiderWithdrawalClaimStatus string

const (
	RiderWithdrawalClaimActive    RiderWithdrawalClaimStatus = "active"
	RiderWithdrawalClaimReleased  RiderWithdrawalClaimStatus = "released"
	RiderWithdrawalClaimCompleted RiderWithdrawalClaimStatus = "completed"
	RiderWithdrawalClaimException RiderWithdrawalClaimStatus = "exception"
)

type RiderWithdrawalClaim struct {
	ID                 uint64                     `gorm:"column:id;primaryKey"`
	ClaimPublicID      string                     `gorm:"column:claim_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_withdrawal_claim_public_id"`
	WithdrawPublicID   string                     `gorm:"column:withdraw_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_withdrawal_claim_withdraw"`
	OperatorUserID     uint64                     `gorm:"column:operator_user_id;not null"`
	ClaimKey           string                     `gorm:"column:claim_key;size:26;not null;uniqueIndex:uq_delivery_rider_withdrawal_claim_key"`
	ExpectedRowVersion uint64                     `gorm:"column:expected_row_version;not null"`
	Status             RiderWithdrawalClaimStatus `gorm:"column:status;size:16;not null"`
	Reason             string                     `gorm:"column:reason;size:256;not null"`
	PaymentReference   string                     `gorm:"column:payment_reference;size:128;not null"`
	IdempotencyKey     string                     `gorm:"column:idempotency_key;size:26;not null;uniqueIndex:uq_delivery_rider_withdrawal_claim_idempotency"`
	CreatedAt          time.Time                  `gorm:"column:created_at;not null"`
	UpdatedAt          time.Time                  `gorm:"column:updated_at;not null"`
}

func (RiderWithdrawalClaim) TableName() string { return "delivery_rider_withdrawal_claims" }

// RiderWithdrawalPaymentReference is the durable, cross-withdrawal reservation
// for a manual transfer receipt.  Keeping this in its own table means the
// receipt remains uniquely bound even when the withdrawal row is retried or
// later audited.
type RiderWithdrawalPaymentReference struct {
	ID                uint64    `gorm:"column:id;primaryKey"`
	PaymentReference  string    `gorm:"column:payment_reference;size:128;not null;uniqueIndex:uq_delivery_rider_withdrawal_payment_reference"`
	WithdrawPublicID  string    `gorm:"column:withdraw_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_withdrawal_payment_withdraw"`
	CreatedAt         time.Time `gorm:"column:created_at;not null"`
}

func (RiderWithdrawalPaymentReference) TableName() string {
	return "delivery_rider_withdrawal_payment_references"
}

type RiderWithdrawalAction struct {
	WithdrawPublicID   string
	ExpectedRowVersion uint64
	OperatorUserID     uint64
	IdempotencyKey     string
	ClaimKey           string
	Reason             string
	PaymentReference   string
	ConfirmedUnpaid    bool
}

func addRiderMoney(a, b uint64) (uint64, error) {
	if a > MaxRiderMoneyAmount || b > MaxRiderMoneyAmount-a {
		return 0, ErrAmountOverflow
	}
	return a + b, nil
}

func withdrawalReplay(value RiderWithdrawal, riderID, amount uint64) error {
	if value.RiderUserID != riderID || value.AmountCents != amount {
		return ErrIdempotencyConflict
	}
	return nil
}

func withdrawalDecision(value RiderWithdrawal, status RiderWithdrawalStatus, version, operator uint64, key, reason, reference string) (bool, error) {
	if value.ProcessKey != nil && *value.ProcessKey == key {
		if value.Status != status || value.ProcessedBy == nil || *value.ProcessedBy != operator || value.ProcessVersion == nil || *value.ProcessVersion != version || value.Reason != reason || value.PaymentReference != reference {
			return false, ErrIdempotencyConflict
		}
		return true, nil
	}
	if value.Status != RiderWithdrawalProcessing {
		return false, ErrWithdrawalAlreadyProcessed
	}
	if value.RowVersion != version {
		return false, ErrConcurrentUpdate
	}
	return false, nil
}

func (RiderWithdrawal) TableName() string { return "delivery_rider_withdrawals" }

func validateRiderWithdrawalInput(riderUserID, amountCents uint64, idempotencyKey string) error {
	if riderUserID == 0 || amountCents == 0 || amountCents > MaxRiderMoneyAmount || !ValidULID(idempotencyKey) {
		return ErrInvalidRecord
	}
	return nil
}

func validateWithdrawalReason(reason string) error {
	if strings.TrimSpace(reason) == "" {
		return ErrWithdrawalReasonRequired
	}
	if len([]rune(reason)) > 256 {
		return ErrInvalidRecord
	}
	return nil
}
