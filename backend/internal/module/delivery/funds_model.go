package delivery

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrWalletInsufficient = errors.New("insufficient delivery wallet balance")
	ErrPaymentForbidden   = errors.New("delivery payment is not owned by merchant")
	ErrPaymentNotPending  = errors.New("delivery payment is not pending")
	ErrPaymentAmount      = errors.New("delivery payment amount mismatch")
	ErrPaymentConfigured  = errors.New("delivery payment provider is not configured")
	ErrAlreadySettled     = errors.New("delivery order fee is already settled")
	ErrRefundNotAllowed   = errors.New("delivery refund is not allowed")
	ErrRefundNotPending   = errors.New("delivery refund is not pending")
)

type LedgerDirection string

const (
	LedgerCredit LedgerDirection = "credit"
	LedgerDebit  LedgerDirection = "debit"
)

type LedgerEntryType string

const (
	LedgerRecharge LedgerEntryType = "recharge"
	LedgerOrderFee LedgerEntryType = "order_fee"
	LedgerRefund   LedgerEntryType = "refund"
)

type PaymentPurpose string

const (
	PaymentOrderFee       PaymentPurpose = "order_fee"
	PaymentWalletRecharge PaymentPurpose = "wallet_recharge"
)

type PaymentProvider string

const (
	PaymentWechat PaymentProvider = "wechat"
	PaymentMock   PaymentProvider = "mock"
)

type PaymentStatus string

const (
	PaymentCreated      PaymentStatus = "created"
	PaymentPrepayIssued PaymentStatus = "prepay_issued"
	PaymentPaid         PaymentStatus = "paid"
	PaymentFailed       PaymentStatus = "failed"
	PaymentClosed       PaymentStatus = "closed"
)

type NotificationStatus string

const (
	NotificationReceived  NotificationStatus = "received"
	NotificationProcessed NotificationStatus = "processed"
	NotificationIgnored   NotificationStatus = "ignored"
	NotificationFailed    NotificationStatus = "failed"
)

type SettlementSource string

const (
	SettlementWallet SettlementSource = "wallet"
	SettlementWechat SettlementSource = "wechat"
	SettlementMock   SettlementSource = "mock"
)

type SettlementStatus string

const (
	SettlementPaid          SettlementStatus = "paid"
	SettlementRefundPending SettlementStatus = "refund_pending"
	SettlementRefunded      SettlementStatus = "refunded"
)

type RefundStatus string

const (
	RefundPending      RefundStatus = "pending"
	RefundSucceeded    RefundStatus = "succeeded"
	RefundFailed       RefundStatus = "failed"
	RefundRetryPending RefundStatus = "refund_pending"
)

type MerchantWallet struct {
	ID                    uint64    `gorm:"column:id;primaryKey"`
	WalletPublicID        string    `gorm:"column:wallet_public_id;size:26;not null;uniqueIndex:uq_delivery_wallet_public_id"`
	MerchantUserID        uint64    `gorm:"column:merchant_user_id;not null;uniqueIndex:uq_delivery_wallet_merchant"`
	AvailableBalanceCents uint64    `gorm:"column:available_balance_cents;not null"`
	RowVersion            uint64    `gorm:"column:row_version;not null"`
	CreatedAt             time.Time `gorm:"column:created_at;not null"`
	UpdatedAt             time.Time `gorm:"column:updated_at;not null"`
}

func (MerchantWallet) TableName() string { return "delivery_merchant_wallets" }

type WalletLedger struct {
	ID                 uint64          `gorm:"column:id;primaryKey"`
	LedgerPublicID     string          `gorm:"column:ledger_public_id;size:26;not null;uniqueIndex:uq_delivery_ledger_public_id"`
	WalletPublicID     string          `gorm:"column:wallet_public_id;size:26;not null;index:idx_delivery_ledger_wallet_created,priority:1"`
	MerchantUserID     uint64          `gorm:"column:merchant_user_id;not null;index:idx_delivery_ledger_merchant_created,priority:1"`
	Direction          LedgerDirection `gorm:"column:direction;size:16;not null"`
	EntryType          LedgerEntryType `gorm:"column:entry_type;size:24;not null"`
	AmountCents        uint64          `gorm:"column:amount_cents;not null"`
	BalanceBeforeCents uint64          `gorm:"column:balance_before_cents;not null"`
	BalanceAfterCents  uint64          `gorm:"column:balance_after_cents;not null"`
	ReferenceType      string          `gorm:"column:reference_type;size:24;not null;index:idx_delivery_ledger_reference,priority:1"`
	ReferencePublicID  string          `gorm:"column:reference_public_id;size:26;not null;index:idx_delivery_ledger_reference,priority:2"`
	IdempotencyKey     string          `gorm:"column:idempotency_key;size:26;not null;uniqueIndex:uq_delivery_ledger_idempotency"`
	CreatedAt          time.Time       `gorm:"column:created_at;not null;index:idx_delivery_ledger_wallet_created,priority:2;index:idx_delivery_ledger_merchant_created,priority:2"`
}

func (WalletLedger) TableName() string { return "delivery_wallet_ledger" }

type PaymentAttempt struct {
	ID                    uint64          `gorm:"column:id;primaryKey"`
	PaymentPublicID       string          `gorm:"column:payment_public_id;size:26;not null;uniqueIndex:uq_delivery_payment_public_id"`
	MerchantUserID        uint64          `gorm:"column:merchant_user_id;not null;index:idx_delivery_payment_merchant_created,priority:1;uniqueIndex:uq_delivery_payment_client_request,priority:1"`
	Purpose               PaymentPurpose  `gorm:"column:purpose;size:24;not null;uniqueIndex:uq_delivery_payment_client_request,priority:2"`
	DeliveryOrderPublicID *string         `gorm:"column:delivery_order_public_id;size:26;index:idx_delivery_payment_order"`
	Provider              PaymentProvider `gorm:"column:provider;size:16;not null"`
	Status                PaymentStatus   `gorm:"column:status;size:24;not null;index:idx_delivery_payment_status"`
	AmountCents           uint64          `gorm:"column:amount_cents;not null"`
	Currency              string          `gorm:"column:currency;size:8;not null"`
	ClientRequestID       string          `gorm:"column:client_request_id;size:26;not null;uniqueIndex:uq_delivery_payment_client_request,priority:3"`
	OutTradeNo            string          `gorm:"column:out_trade_no;size:64;not null;uniqueIndex:uq_delivery_payment_out_trade_no"`
	PrepayID              *string         `gorm:"column:prepay_id;size:128"`
	WechatTransactionID   *string         `gorm:"column:wechat_transaction_id;size:128;uniqueIndex:uq_delivery_payment_wechat_transaction"`
	MockTraceID           *string         `gorm:"column:mock_trace_id;size:64"`
	RowVersion            uint64          `gorm:"column:row_version;not null"`
	CreatedAt             time.Time       `gorm:"column:created_at;not null;index:idx_delivery_payment_merchant_created,priority:2"`
	UpdatedAt             time.Time       `gorm:"column:updated_at;not null"`
	PaidAt                *time.Time      `gorm:"column:paid_at"`
	ClosedAt              *time.Time      `gorm:"column:closed_at"`
}

func (PaymentAttempt) TableName() string { return "delivery_payment_attempts" }

type PaymentNotification struct {
	ID                   uint64             `gorm:"column:id;primaryKey"`
	NotificationPublicID string             `gorm:"column:notification_public_id;size:26;not null;uniqueIndex:uq_delivery_notification_public_id"`
	OutTradeNo           string             `gorm:"column:out_trade_no;size:64;not null;index:idx_delivery_notification_out_trade_no"`
	WechatTransactionID  string             `gorm:"column:wechat_transaction_id;size:128;not null;uniqueIndex:uq_delivery_notification_transaction"`
	BodySHA256           string             `gorm:"column:body_sha256;size:64;not null;uniqueIndex:uq_delivery_notification_body_sha256"`
	Status               NotificationStatus `gorm:"column:status;size:24;not null;index:idx_delivery_notification_status"`
	ProcessedAt          *time.Time         `gorm:"column:processed_at"`
	CreatedAt            time.Time          `gorm:"column:created_at;not null"`
}

func (PaymentNotification) TableName() string { return "delivery_payment_notifications" }

type FeeSettlement struct {
	ID                    uint64           `gorm:"column:id;primaryKey"`
	SettlementPublicID    string           `gorm:"column:settlement_public_id;size:26;not null;uniqueIndex:uq_delivery_settlement_public_id"`
	DeliveryOrderPublicID string           `gorm:"column:delivery_order_public_id;size:26;not null;uniqueIndex:uq_delivery_settlement_order"`
	MerchantUserID        uint64           `gorm:"column:merchant_user_id;not null;index:idx_delivery_settlement_merchant"`
	Source                SettlementSource `gorm:"column:source;size:16;not null"`
	AmountCents           uint64           `gorm:"column:amount_cents;not null"`
	PaymentPublicID       *string          `gorm:"column:payment_public_id;size:26;uniqueIndex:uq_delivery_settlement_payment"`
	LedgerPublicID        *string          `gorm:"column:ledger_public_id;size:26;uniqueIndex:uq_delivery_settlement_ledger"`
	Status                SettlementStatus `gorm:"column:status;size:24;not null;index:idx_delivery_settlement_status"`
	PaidAt                time.Time        `gorm:"column:paid_at;not null"`
	RefundedAt            *time.Time       `gorm:"column:refunded_at"`
	RowVersion            uint64           `gorm:"column:row_version;not null"`
}

func (FeeSettlement) TableName() string { return "delivery_fee_settlements" }

type Refund struct {
	ID                    uint64           `gorm:"column:id;primaryKey"`
	RefundPublicID        string           `gorm:"column:refund_public_id;size:26;not null;uniqueIndex:uq_delivery_refund_public_id"`
	SettlementPublicID    string           `gorm:"column:settlement_public_id;size:26;not null;uniqueIndex:uq_delivery_refund_settlement"`
	DeliveryOrderPublicID string           `gorm:"column:delivery_order_public_id;size:26;not null;index:idx_delivery_refund_order"`
	MerchantUserID        uint64           `gorm:"column:merchant_user_id;not null"`
	Source                SettlementSource `gorm:"column:source;size:16;not null"`
	AmountCents           uint64           `gorm:"column:amount_cents;not null"`
	Status                RefundStatus     `gorm:"column:status;size:24;not null;index:idx_delivery_refund_status"`
	IdempotencyKey        string           `gorm:"column:idempotency_key;size:26;not null;uniqueIndex:uq_delivery_refund_idempotency"`
	OutRefundNo           string           `gorm:"column:out_refund_no;size:64;not null;uniqueIndex:uq_delivery_refund_out_refund_no"`
	WechatRefundID        *string          `gorm:"column:wechat_refund_id;size:128;uniqueIndex:uq_delivery_refund_wechat_id"`
	FailureReason         string           `gorm:"column:failure_reason;size:512;not null"`
	RowVersion            uint64           `gorm:"column:row_version;not null"`
	CreatedAt             time.Time        `gorm:"column:created_at;not null"`
	UpdatedAt             time.Time        `gorm:"column:updated_at;not null"`
	SucceededAt           *time.Time       `gorm:"column:succeeded_at"`
}

func (Refund) TableName() string { return "delivery_refunds" }

func validatePaymentAttempt(value PaymentAttempt) error {
	if !ValidULID(value.PaymentPublicID) || value.MerchantUserID == 0 ||
		!ValidULID(value.ClientRequestID) || strings.TrimSpace(value.OutTradeNo) == "" ||
		value.AmountCents == 0 || value.Currency != "CNY" || value.RowVersion != 1 ||
		value.CreatedAt.IsZero() || value.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	if value.Purpose == PaymentOrderFee {
		if value.DeliveryOrderPublicID == nil || !ValidULID(*value.DeliveryOrderPublicID) {
			return ErrInvalidRecord
		}
	} else if value.Purpose != PaymentWalletRecharge || value.DeliveryOrderPublicID != nil {
		return ErrInvalidRecord
	}
	if value.Provider != PaymentWechat && value.Provider != PaymentMock {
		return ErrInvalidRecord
	}
	if value.Status != PaymentCreated && value.Status != PaymentPaid {
		return ErrInvalidRecord
	}
	return nil
}
