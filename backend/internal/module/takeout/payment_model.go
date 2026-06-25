package takeout

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrPaymentNotPending = errors.New("payment order is not pending")
	ErrPaymentForbidden  = errors.New("payment order is not owned by buyer")
	ErrPaymentAmount     = errors.New("payment amount mismatch")
)

type PaymentProvider string

const (
	PaymentProviderWechat PaymentProvider = "wechat"
	PaymentProviderMock   PaymentProvider = "mock"
)

func (p PaymentProvider) Valid() bool {
	return p == PaymentProviderWechat || p == PaymentProviderMock
}

type PaymentStatus string

const (
	PaymentCreated      PaymentStatus = "created"
	PaymentPrepayIssued PaymentStatus = "prepay_issued"
	PaymentPaid         PaymentStatus = "paid"
	PaymentFailed       PaymentStatus = "failed"
	PaymentClosed       PaymentStatus = "closed"
)

func (s PaymentStatus) Valid() bool {
	switch s {
	case PaymentCreated, PaymentPrepayIssued, PaymentPaid, PaymentFailed, PaymentClosed:
		return true
	default:
		return false
	}
}

func (s PaymentStatus) Terminal() bool {
	return s == PaymentPaid || s == PaymentFailed || s == PaymentClosed
}

type PaymentNotificationStatus string

const (
	PaymentNotificationReceived  PaymentNotificationStatus = "received"
	PaymentNotificationProcessed PaymentNotificationStatus = "processed"
	PaymentNotificationIgnored   PaymentNotificationStatus = "ignored"
	PaymentNotificationFailed    PaymentNotificationStatus = "failed"
)

func (s PaymentNotificationStatus) Valid() bool {
	switch s {
	case PaymentNotificationReceived, PaymentNotificationProcessed, PaymentNotificationIgnored, PaymentNotificationFailed:
		return true
	default:
		return false
	}
}

type PaymentAttempt struct {
	ID                  uint64          `gorm:"column:id;primaryKey"`
	PaymentPublicID     string          `gorm:"column:payment_public_id;size:26;not null;uniqueIndex:uq_takeout_payment_public_id"`
	OrderPublicID       string          `gorm:"column:order_public_id;size:26;not null;index:idx_takeout_payment_order"`
	BuyerUserID         uint64          `gorm:"column:buyer_user_id;not null;index:idx_takeout_payment_buyer"`
	Provider            PaymentProvider `gorm:"column:provider;size:16;not null"`
	Status              PaymentStatus   `gorm:"column:status;size:24;not null;index:idx_takeout_payment_status"`
	AmountCents         uint64          `gorm:"column:amount_cents;not null"`
	Currency            string          `gorm:"column:currency;size:8;not null"`
	OutTradeNo          string          `gorm:"column:out_trade_no;size:64;not null;uniqueIndex:uq_takeout_payment_out_trade_no"`
	PrepayID            *string         `gorm:"column:prepay_id;size:128"`
	WechatTransactionID *string         `gorm:"column:wechat_transaction_id;size:128;uniqueIndex:uq_takeout_payment_wechat_transaction_id"`
	PayNo               *string         `gorm:"column:pay_no;size:64"`
	MockTraceID         *string         `gorm:"column:mock_trace_id;size:64"`
	RowVersion          uint64          `gorm:"column:row_version;not null"`
	CreatedAt           time.Time       `gorm:"column:created_at;not null"`
	UpdatedAt           time.Time       `gorm:"column:updated_at;not null"`
	PaidAt              *time.Time      `gorm:"column:paid_at"`
	ClosedAt            *time.Time      `gorm:"column:closed_at"`
}

func (PaymentAttempt) TableName() string { return "takeout_payment_attempts" }

func (p PaymentAttempt) ValidateNew() error {
	if !ValidULID(p.PaymentPublicID) || !ValidULID(p.OrderPublicID) || p.BuyerUserID == 0 ||
		!p.Provider.Valid() || !p.Status.Valid() || p.AmountCents == 0 || p.Currency != "CNY" ||
		strings.TrimSpace(p.OutTradeNo) == "" || len(p.OutTradeNo) > 64 || p.RowVersion != 1 ||
		p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	if p.PrepayID != nil && (strings.TrimSpace(*p.PrepayID) == "" || len(*p.PrepayID) > 128) {
		return ErrInvalidRecord
	}
	if p.WechatTransactionID != nil && (strings.TrimSpace(*p.WechatTransactionID) == "" || len(*p.WechatTransactionID) > 128) {
		return ErrInvalidRecord
	}
	if p.PayNo != nil && (strings.TrimSpace(*p.PayNo) == "" || len(*p.PayNo) > 64) {
		return ErrInvalidRecord
	}
	if p.MockTraceID != nil && (strings.TrimSpace(*p.MockTraceID) == "" || len(*p.MockTraceID) > 64) {
		return ErrInvalidRecord
	}
	if p.Status == PaymentPrepayIssued && p.PrepayID == nil {
		return ErrInvalidRecord
	}
	if p.Status == PaymentPaid && (p.PayNo == nil || p.PaidAt == nil) {
		return ErrInvalidRecord
	}
	return nil
}

type PaymentNotification struct {
	ID                   uint64                    `gorm:"column:id;primaryKey"`
	NotificationPublicID string                    `gorm:"column:notification_public_id;size:26;not null;uniqueIndex:uq_takeout_payment_notification_public_id"`
	Provider             PaymentProvider           `gorm:"column:provider;size:16;not null"`
	OutTradeNo           string                    `gorm:"column:out_trade_no;size:64;not null;index:idx_takeout_payment_notification_out_trade_no"`
	WechatTransactionID  *string                   `gorm:"column:wechat_transaction_id;size:128"`
	BodySHA256           string                    `gorm:"column:body_sha256;size:64;not null;uniqueIndex:uq_takeout_payment_notification_body_sha256"`
	Status               PaymentNotificationStatus `gorm:"column:status;size:24;not null;index:idx_takeout_payment_notification_status"`
	ProcessedAt          *time.Time                `gorm:"column:processed_at"`
	CreatedAt            time.Time                 `gorm:"column:created_at;not null"`
}

func (PaymentNotification) TableName() string { return "takeout_payment_notifications" }

func (n PaymentNotification) ValidateNew() error {
	if !ValidULID(n.NotificationPublicID) || !n.Provider.Valid() || strings.TrimSpace(n.OutTradeNo) == "" ||
		len(n.OutTradeNo) > 64 || len(n.BodySHA256) != 64 || !n.Status.Valid() || n.CreatedAt.IsZero() {
		return ErrInvalidRecord
	}
	if n.WechatTransactionID != nil && (strings.TrimSpace(*n.WechatTransactionID) == "" || len(*n.WechatTransactionID) > 128) {
		return ErrInvalidRecord
	}
	if n.Status == PaymentNotificationProcessed && n.ProcessedAt == nil {
		return ErrInvalidRecord
	}
	return nil
}
