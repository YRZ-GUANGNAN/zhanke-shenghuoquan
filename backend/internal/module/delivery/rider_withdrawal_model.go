package delivery

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrWithdrawalAlreadyProcessed = errors.New("rider withdrawal already processed")
	ErrWithdrawalNotFound         = errors.New("rider withdrawal not found")
	ErrWithdrawalReasonRequired   = errors.New("rider withdrawal rejection reason is required")
	ErrWithdrawalPaymentRequired  = errors.New("manual payment reference is required")
)

type RiderWithdrawalStatus string

const (
	RiderWithdrawalProcessing RiderWithdrawalStatus = "processing"
	RiderWithdrawalPaid       RiderWithdrawalStatus = "paid"
	RiderWithdrawalRejected   RiderWithdrawalStatus = "rejected"
)

func (s RiderWithdrawalStatus) Valid() bool {
	return s == RiderWithdrawalProcessing || s == RiderWithdrawalPaid || s == RiderWithdrawalRejected
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

// Replay must match the original operation, not merely the final status.
func withdrawalReplay(value RiderWithdrawal, riderID, amount uint64) error {
	if value.RiderUserID != riderID || value.AmountCents != amount {
		return ErrIdempotencyConflict
	}
	return nil
}

func withdrawalDecision(value RiderWithdrawal, status RiderWithdrawalStatus, version, operator uint64, key, reason, reference string) (bool, error) {
	if value.ProcessKey != nil && *value.ProcessKey == key {
		if value.Status != status || value.ProcessedBy == nil || *value.ProcessedBy != operator ||
			value.ProcessVersion == nil || *value.ProcessVersion != version || value.Reason != reason || value.PaymentReference != reference {
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
	if riderUserID == 0 || amountCents == 0 || !ValidULID(idempotencyKey) {
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
