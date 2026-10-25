package delivery

import (
	"errors"
	"time"
)

var ErrFundingSourceMismatch = errors.New("delivery funding source mismatch")

type FundingSource string

const (
	FundingSourceWechatRecharge    FundingSource = "wechat_recharge"
	FundingSourceMockRecharge      FundingSource = "mock_recharge"
	FundingSourceHistoricalUnknown FundingSource = "historical_unknown"
)

type FundingVerificationStatus string

const (
	FundingVerificationVerified   FundingVerificationStatus = "verified"
	FundingVerificationUnverified FundingVerificationStatus = "unverified"
)

type FundingLot struct {
	ID                   uint64                    `gorm:"column:id;primaryKey"`
	FundingLotPublicID   string                    `gorm:"column:funding_lot_public_id;size:26;not null;uniqueIndex:uq_delivery_funding_lot_public_id"`
	WalletPublicID       string                    `gorm:"column:wallet_public_id;size:26;not null;index:idx_delivery_funding_lot_wallet_fifo,priority:1"`
	MerchantUserID       uint64                    `gorm:"column:merchant_user_id;not null;index:idx_delivery_funding_lot_merchant_verification,priority:1"`
	Source               FundingSource             `gorm:"column:source;size:32;not null"`
	VerificationStatus   FundingVerificationStatus `gorm:"column:verification_status;size:24;not null;index:idx_delivery_funding_lot_merchant_verification,priority:2"`
	PaymentPublicID      *string                   `gorm:"column:payment_public_id;size:26;uniqueIndex:uq_delivery_funding_lot_payment"`
	TotalAmountCents     uint64                    `gorm:"column:total_amount_cents;not null"`
	AllocatedAmountCents uint64                    `gorm:"column:allocated_amount_cents;not null"`
	RefundedAmountCents  uint64                    `gorm:"column:refunded_amount_cents;not null"`
	Currency             string                    `gorm:"column:currency;size:8;not null"`
	RowVersion           uint64                    `gorm:"column:row_version;not null"`
	CreatedAt            time.Time                 `gorm:"column:created_at;not null;index:idx_delivery_funding_lot_wallet_fifo,priority:2;index:idx_delivery_funding_lot_merchant_verification,priority:3"`
	UpdatedAt            time.Time                 `gorm:"column:updated_at;not null"`
}

func (FundingLot) TableName() string { return "delivery_funding_lots" }

type FundingAllocation struct {
	ID                        uint64     `gorm:"column:id;primaryKey"`
	FundingAllocationPublicID string     `gorm:"column:funding_allocation_public_id;size:26;not null;uniqueIndex:uq_delivery_funding_allocation_public_id"`
	SettlementPublicID        string     `gorm:"column:settlement_public_id;size:26;not null;uniqueIndex:uq_delivery_funding_allocation_settlement_lot,priority:1"`
	FundingLotPublicID        string     `gorm:"column:funding_lot_public_id;size:26;not null;uniqueIndex:uq_delivery_funding_allocation_settlement_lot,priority:2;index:idx_delivery_funding_allocation_lot,priority:1"`
	MerchantUserID            uint64     `gorm:"column:merchant_user_id;not null;index:idx_delivery_funding_allocation_merchant,priority:1"`
	AmountCents               uint64     `gorm:"column:amount_cents;not null"`
	RefundedAmountCents       uint64     `gorm:"column:refunded_amount_cents;not null"`
	RowVersion                uint64     `gorm:"column:row_version;not null"`
	CreatedAt                 time.Time  `gorm:"column:created_at;not null;index:idx_delivery_funding_allocation_merchant,priority:2"`
	RefundedAt                *time.Time `gorm:"column:refunded_at"`
}

func (FundingAllocation) TableName() string { return "delivery_funding_allocations" }

func fundingLotAvailableCents(lot FundingLot) (uint64, error) {
	if lot.RefundedAmountCents > lot.AllocatedAmountCents {
		return 0, ErrFundingSourceMismatch
	}
	netAllocated := lot.AllocatedAmountCents - lot.RefundedAmountCents
	if netAllocated > lot.TotalAmountCents {
		return 0, ErrFundingSourceMismatch
	}
	return lot.TotalAmountCents - netAllocated, nil
}

func validateFundingLot(value FundingLot) error {
	if !ValidULID(value.FundingLotPublicID) || !ValidULID(value.WalletPublicID) ||
		value.MerchantUserID == 0 || value.TotalAmountCents == 0 || value.Currency != "CNY" ||
		value.RowVersion == 0 || value.CreatedAt.IsZero() || value.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	if _, err := fundingLotAvailableCents(value); err != nil {
		return ErrInvalidRecord
	}
	switch value.Source {
	case FundingSourceWechatRecharge:
		if value.VerificationStatus != FundingVerificationVerified ||
			value.PaymentPublicID == nil || !ValidULID(*value.PaymentPublicID) {
			return ErrInvalidRecord
		}
	case FundingSourceMockRecharge:
		if value.VerificationStatus != FundingVerificationUnverified ||
			value.PaymentPublicID == nil || !ValidULID(*value.PaymentPublicID) {
			return ErrInvalidRecord
		}
	case FundingSourceHistoricalUnknown:
		if value.VerificationStatus != FundingVerificationUnverified || value.PaymentPublicID != nil {
			return ErrInvalidRecord
		}
	default:
		return ErrInvalidRecord
	}
	return nil
}

func validateFundingAllocation(value FundingAllocation) error {
	if !ValidULID(value.FundingAllocationPublicID) || !ValidULID(value.SettlementPublicID) ||
		!ValidULID(value.FundingLotPublicID) || value.MerchantUserID == 0 ||
		value.AmountCents == 0 || value.RefundedAmountCents > value.AmountCents ||
		value.RowVersion == 0 || value.CreatedAt.IsZero() {
		return ErrInvalidRecord
	}
	if (value.RefundedAmountCents == 0) != (value.RefundedAt == nil) {
		return ErrInvalidRecord
	}
	return nil
}
