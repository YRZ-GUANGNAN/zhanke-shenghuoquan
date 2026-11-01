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

const EventOrderStatusChanged = "DeliveryOrderStatusChanged"

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
