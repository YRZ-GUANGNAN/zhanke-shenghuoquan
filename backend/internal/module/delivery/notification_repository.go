package delivery

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// NotificationEvent is a metadata-only fact. It carries no address, phone,
// customer name, message text or payment-provider credentials.
type NotificationEvent struct {
	EventPublicID     string
	EventType         string
	AggregatePublicID string
	AggregateVersion  uint64
	Payload           json.RawMessage
	OccurredAt        time.Time
}

// NotificationEventAppender must only persist using the supplied transaction.
// The composition root supplies the adapter; no network work belongs here.
type NotificationEventAppender func(*gorm.DB, NotificationEvent) error
type RepositoryOption func(*GormRepository)

func WithNotificationEvents(appendEvent NotificationEventAppender) RepositoryOption {
	return func(r *GormRepository) { r.notificationAppender = appendEvent }
}

const (
	EventOrderStatusChanged      = "DeliveryOrderStatusChanged"
	EventRiderIncomeRecorded     = "RiderIncomeRecorded"
	EventRiderPayoutCompleted    = "RiderPayoutCompleted"
	EventRiderWithdrawalChanged  = "RiderWithdrawalChanged"
	EventMerchantRefundChanged   = "MerchantRefundChanged"
	EventMerchantWalletRecharged = "MerchantWalletRecharged"
)

// The financial facts below are deliberately small metadata snapshots.  They
// are appended inside the caller's transaction and contain no provider
// credentials, openids, addresses, phone numbers, notes, or payment bodies.
type RiderIncomeNotificationPayload struct {
	EarningPublicID  string `json:"earningPublicId"`
	OrderPublicID    string `json:"orderPublicId"`
	RiderUserID      uint64 `json:"riderUserId"`
	AmountCents      uint64 `json:"amountCents"`
	SettlementStatus string `json:"status"`
}

type RiderPayoutNotificationPayload struct {
	PayoutPublicID  string `json:"payoutPublicId"`
	EarningPublicID string `json:"earningPublicId"`
	RiderUserID     uint64 `json:"riderUserId"`
	AmountCents     uint64 `json:"amountCents"`
	Status          string `json:"status"`
}

type RiderWithdrawalNotificationPayload struct {
	WithdrawPublicID string `json:"withdrawPublicId"`
	RiderUserID      uint64 `json:"riderUserId"`
	AmountCents      uint64 `json:"amountCents"`
	Status           string `json:"status"`
}

type MerchantRefundNotificationPayload struct {
	RefundPublicID string `json:"refundPublicId"`
	OrderPublicID  string `json:"orderPublicId"`
	MerchantUserID uint64 `json:"merchantUserId"`
	AmountCents    uint64 `json:"amountCents"`
	Status         string `json:"status"`
}

type MerchantWalletNotificationPayload struct {
	PaymentPublicID string `json:"paymentPublicId"`
	MerchantUserID  uint64 `json:"merchantUserId"`
	AmountCents     uint64 `json:"amountCents"`
	Status          string `json:"status"`
}

func newMetadataNotificationEvent(eventType, aggregateID string, aggregateVersion uint64, payload any, now time.Time) (NotificationEvent, error) {
	if eventType == "" || !ValidULID(aggregateID) || aggregateVersion == 0 || now.IsZero() || payload == nil {
		return NotificationEvent{}, ErrInvalidRecord
	}
	id, err := NewULID()
	if err != nil {
		return NotificationEvent{}, err
	}
	raw, err := json.Marshal(payload)
	if err != nil || len(raw) == 0 {
		return NotificationEvent{}, ErrInvalidRecord
	}
	return NotificationEvent{EventPublicID: id, EventType: eventType,
		AggregatePublicID: aggregateID, AggregateVersion: aggregateVersion,
		Payload: raw, OccurredAt: now}, nil
}

func appendMetadataNotificationTx(tx *gorm.DB, appender NotificationEventAppender, event NotificationEvent) error {
	if appender == nil {
		return nil
	}
	if tx == nil || event.EventPublicID == "" {
		return ErrInvalidRecord
	}
	return appender(tx, event)
}

func appendFinancialNotificationTx(tx *gorm.DB, appender NotificationEventAppender, eventType, aggregateID string, aggregateVersion uint64, payload any, now time.Time) error {
	// Disabled notifications must not introduce ID generation or validation
	// failures into legacy money transactions.
	if appender == nil {
		return nil
	}
	event, err := newMetadataNotificationEvent(eventType, aggregateID, aggregateVersion, payload, now)
	if err != nil {
		return err
	}
	return appendMetadataNotificationTx(tx, appender, event)
}

func appendRefundNotificationTx(tx *gorm.DB, appender NotificationEventAppender, before *Refund, after Refund, now time.Time) error {
	if appender == nil {
		return nil
	}
	if after.MerchantUserID == 0 || after.AmountCents == 0 || !ValidULID(after.DeliveryOrderPublicID) {
		return ErrInvalidRecord
	}
	if before != nil {
		if before.RefundPublicID != after.RefundPublicID || before.MerchantUserID != after.MerchantUserID ||
			before.DeliveryOrderPublicID != after.DeliveryOrderPublicID || before.AmountCents != after.AmountCents {
			return ErrInvalidRecord
		}
		// Verification leases and repeated PROCESSING observations may advance
		// row_version but are not new user-visible financial facts.
		if before.Status == after.Status {
			return nil
		}
		if after.RowVersion != before.RowVersion+1 {
			return ErrInvalidRecord
		}
	} else if after.RowVersion != 1 {
		return ErrInvalidRecord
	}
	switch after.Status {
	case RefundPending, RefundRetryPending, RefundSucceeded, RefundFailed:
	default:
		return ErrInvalidRecord
	}
	return appendFinancialNotificationTx(tx, appender, EventMerchantRefundChanged, after.RefundPublicID, after.RowVersion,
		MerchantRefundNotificationPayload{RefundPublicID: after.RefundPublicID, OrderPublicID: after.DeliveryOrderPublicID,
			MerchantUserID: after.MerchantUserID, AmountCents: after.AmountCents, Status: string(after.Status)}, now)
}

type OrderNotificationPayload struct {
	OrderPublicID       string  `json:"orderPublicId"`
	MerchantUserID      uint64  `json:"merchantUserId"`
	RiderUserID         *uint64 `json:"riderUserId,omitempty"`
	PreviousRiderUserID *uint64 `json:"previousRiderUserId,omitempty"`
	From                Status  `json:"from"`
	To                  Status  `json:"to"`
	DeliveryFeeCents    uint64  `json:"deliveryFeeCents"`
}

func orderNotification(before, after Order, now time.Time) (NotificationEvent, error) {
	if !ValidULID(after.DeliveryOrderPublicID) || after.MerchantUserID == 0 ||
		before.DeliveryOrderPublicID != after.DeliveryOrderPublicID ||
		after.RowVersion != before.RowVersion+1 || !CanTransition(before.Status, after.Status) || now.IsZero() {
		return NotificationEvent{}, ErrInvalidRecord
	}
	id, err := NewULID()
	if err != nil {
		return NotificationEvent{}, err
	}
	payload, err := json.Marshal(OrderNotificationPayload{
		OrderPublicID: after.DeliveryOrderPublicID, MerchantUserID: after.MerchantUserID,
		RiderUserID: after.RiderUserID, PreviousRiderUserID: before.RiderUserID,
		From: before.Status, To: after.Status, DeliveryFeeCents: after.DeliveryFeeCents,
	})
	if err != nil {
		return NotificationEvent{}, err
	}
	return NotificationEvent{EventPublicID: id, EventType: EventOrderStatusChanged,
		AggregatePublicID: after.DeliveryOrderPublicID, AggregateVersion: after.RowVersion,
		Payload: payload, OccurredAt: now}, nil
}

func appendOrderNotificationTx(tx *gorm.DB, appender NotificationEventAppender, before, after Order, now time.Time) error {
	if appender == nil {
		return nil
	} // Rollout is explicitly opt-in at composition.
	if tx == nil {
		return ErrInvalidRecord
	}
	event, err := orderNotification(before, after, now)
	if err != nil {
		return err
	}
	return appender(tx, event)
}
