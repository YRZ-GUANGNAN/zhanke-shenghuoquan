package delivery

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrFeeConfigNotFound = errors.New("delivery fee config not found")
	ErrEarningSettled    = errors.New("delivery rider earning already settled")
)

type FeeConfigStatus string

const (
	FeeConfigActive   FeeConfigStatus = "active"
	FeeConfigDisabled FeeConfigStatus = "disabled"
)

func (s FeeConfigStatus) Valid() bool {
	return s == FeeConfigActive || s == FeeConfigDisabled
}

type DeliveryFeeConfig struct {
	ID               uint64          `gorm:"column:id;primaryKey"`
	ConfigScope      string          `gorm:"column:config_scope;size:32;not null;uniqueIndex:uq_delivery_fee_config_scope"`
	DeliveryFeeCents uint64          `gorm:"column:delivery_fee_cents;not null"`
	PlatformCutCents uint64          `gorm:"column:platform_cut_cents;not null"`
	Status           FeeConfigStatus `gorm:"column:status;size:16;not null;index:idx_delivery_fee_config_status"`
	RowVersion       uint64          `gorm:"column:row_version;not null"`
	CreatedAt        time.Time       `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time       `gorm:"column:updated_at;not null"`
}

func (DeliveryFeeConfig) TableName() string { return "delivery_fee_configs" }

type MerchantFeeOverride struct {
	ID               uint64          `gorm:"column:id;primaryKey"`
	MerchantUserID   uint64          `gorm:"column:merchant_user_id;not null;uniqueIndex:uq_delivery_merchant_fee_override"`
	DeliveryFeeCents uint64          `gorm:"column:delivery_fee_cents;not null"`
	PlatformCutCents uint64          `gorm:"column:platform_cut_cents;not null"`
	Status           FeeConfigStatus `gorm:"column:status;size:16;not null;index:idx_delivery_merchant_fee_status"`
	RowVersion       uint64          `gorm:"column:row_version;not null"`
	CreatedAt        time.Time       `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time       `gorm:"column:updated_at;not null"`
}

func (MerchantFeeOverride) TableName() string { return "delivery_merchant_fee_overrides" }

type RiderLedgerDirection string

const (
	RiderLedgerCredit RiderLedgerDirection = "credit"
	RiderLedgerDebit  RiderLedgerDirection = "debit"
)

type RiderLedgerEntryType string

const (
	RiderLedgerIncome     RiderLedgerEntryType = "income"
	RiderLedgerAdjustment RiderLedgerEntryType = "adjustment"
	RiderLedgerPayout     RiderLedgerEntryType = "payout"
)

type RiderEarningStatus string

const (
	RiderEarningPending     RiderEarningStatus = "pending"
	RiderEarningSettled     RiderEarningStatus = "settled"
	RiderEarningPaidOffline RiderEarningStatus = "paid_offline"
)

func (s RiderEarningStatus) Valid() bool {
	return s == RiderEarningPending || s == RiderEarningSettled || s == RiderEarningPaidOffline
}

type RiderWallet struct {
	ID                    uint64    `gorm:"column:id;primaryKey"`
	WalletPublicID        string    `gorm:"column:wallet_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_wallet_public_id"`
	RiderUserID           uint64    `gorm:"column:rider_user_id;not null;uniqueIndex:uq_delivery_rider_wallet_rider"`
	AvailableBalanceCents uint64    `gorm:"column:available_balance_cents;not null"`
	RowVersion            uint64    `gorm:"column:row_version;not null"`
	CreatedAt             time.Time `gorm:"column:created_at;not null"`
	UpdatedAt             time.Time `gorm:"column:updated_at;not null"`
}

func (RiderWallet) TableName() string { return "delivery_rider_wallets" }

type RiderEarningRecord struct {
	ID                    uint64             `gorm:"column:id;primaryKey"`
	EarningPublicID       string             `gorm:"column:earning_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_earning_public_id"`
	DeliveryOrderPublicID string             `gorm:"column:delivery_order_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_earning_order"`
	RiderUserID           uint64             `gorm:"column:rider_user_id;not null;index:idx_delivery_rider_earning_rider_status,priority:1"`
	DeliveryFeeCents      uint64             `gorm:"column:delivery_fee_cents;not null"`
	PlatformCutCents      uint64             `gorm:"column:platform_cut_cents;not null"`
	RiderFeeCents         uint64             `gorm:"column:rider_fee_cents;not null"`
	SettlementStatus      RiderEarningStatus `gorm:"column:settlement_status;size:24;not null;index:idx_delivery_rider_earning_rider_status,priority:2;index:idx_delivery_rider_earning_status"`
	PaidOfflineAt         *time.Time         `gorm:"column:paid_offline_at"`
	RowVersion            uint64             `gorm:"column:row_version;not null"`
	CreatedAt             time.Time          `gorm:"column:created_at;not null"`
	UpdatedAt             time.Time          `gorm:"column:updated_at;not null"`
}

func (RiderEarningRecord) TableName() string { return "delivery_rider_earning_records" }

type RiderPayoutRecord struct {
	ID                    uint64    `gorm:"column:id;primaryKey"`
	PayoutPublicID        string    `gorm:"column:payout_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_payout_public_id"`
	EarningPublicID       string    `gorm:"column:earning_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_payout_earning"`
	DeliveryOrderPublicID string    `gorm:"column:delivery_order_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_payout_order"`
	RiderUserID           uint64    `gorm:"column:rider_user_id;not null;index:idx_delivery_rider_payout_rider_created,priority:1"`
	OperatorUserID        uint64    `gorm:"column:operator_user_id;not null"`
	AmountCents           uint64    `gorm:"column:amount_cents;not null"`
	IdempotencyKey        string    `gorm:"column:idempotency_key;size:26;not null;uniqueIndex:uq_delivery_rider_payout_idempotency"`
	Note                  string    `gorm:"column:note;size:256;not null"`
	PaidOfflineAt         time.Time `gorm:"column:paid_offline_at;not null"`
	RowVersion            uint64    `gorm:"column:row_version;not null"`
	CreatedAt             time.Time `gorm:"column:created_at;not null;index:idx_delivery_rider_payout_rider_created,priority:2"`
	UpdatedAt             time.Time `gorm:"column:updated_at;not null"`
}

func (RiderPayoutRecord) TableName() string { return "delivery_rider_payout_records" }

type RiderLedger struct {
	ID                 uint64               `gorm:"column:id;primaryKey"`
	LedgerPublicID     string               `gorm:"column:ledger_public_id;size:26;not null;uniqueIndex:uq_delivery_rider_ledger_public_id"`
	WalletPublicID     string               `gorm:"column:wallet_public_id;size:26;not null;index:idx_delivery_rider_ledger_wallet_created,priority:1"`
	RiderUserID        uint64               `gorm:"column:rider_user_id;not null;index:idx_delivery_rider_ledger_rider_created,priority:1"`
	Direction          RiderLedgerDirection `gorm:"column:direction;size:16;not null"`
	EntryType          RiderLedgerEntryType `gorm:"column:entry_type;size:24;not null"`
	AmountCents        uint64               `gorm:"column:amount_cents;not null"`
	BalanceBeforeCents uint64               `gorm:"column:balance_before_cents;not null"`
	BalanceAfterCents  uint64               `gorm:"column:balance_after_cents;not null"`
	ReferenceType      string               `gorm:"column:reference_type;size:32;not null;index:idx_delivery_rider_ledger_reference,priority:1"`
	ReferencePublicID  string               `gorm:"column:reference_public_id;size:26;not null;index:idx_delivery_rider_ledger_reference,priority:2"`
	IdempotencyKey     string               `gorm:"column:idempotency_key;size:26;not null;uniqueIndex:uq_delivery_rider_ledger_idempotency"`
	CreatedAt          time.Time            `gorm:"column:created_at;not null;index:idx_delivery_rider_ledger_wallet_created,priority:2;index:idx_delivery_rider_ledger_rider_created,priority:2"`
}

func (RiderLedger) TableName() string { return "delivery_rider_ledger" }

func feePolicyFromAmounts(deliveryFeeCents, platformCutCents uint64) (FeePolicy, error) {
	if deliveryFeeCents == 0 || platformCutCents >= deliveryFeeCents {
		return FeePolicy{}, ErrInvalidRecord
	}
	riderFeeCents := deliveryFeeCents - platformCutCents
	policy := FeePolicy{
		DeliveryFeeCents: deliveryFeeCents,
		PlatformCutCents: platformCutCents,
		RiderFeeCents:    riderFeeCents,
	}
	if !policy.Valid() {
		return FeePolicy{}, ErrInvalidRecord
	}
	return policy, nil
}

func validateFeeConfigAmount(deliveryFeeCents, platformCutCents uint64) error {
	_, err := feePolicyFromAmounts(deliveryFeeCents, platformCutCents)
	return err
}

func validateRiderEarningAmount(deliveryFeeCents, platformCutCents, riderFeeCents uint64) error {
	total, ok := checkedAddAmount(platformCutCents, riderFeeCents)
	if deliveryFeeCents == 0 || riderFeeCents == 0 || platformCutCents >= deliveryFeeCents ||
		!ok || total != deliveryFeeCents {
		return ErrInvalidRecord
	}
	return nil
}

func truncateRiderPayoutNote(value string) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= 256 {
		return value
	}
	runes := []rune(value)
	return string(runes[:256])
}
