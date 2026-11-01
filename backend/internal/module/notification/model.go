package notification

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

var (
	ErrInvalidRecord     = errors.New("invalid notification record")
	ErrConcurrentUpdate  = errors.New("notification lease lost")
	ErrSubscriptionEmpty = errors.New("notification subscription unavailable")
)

type EventStatus string

const (
	EventPending    EventStatus = "pending"
	EventProcessing EventStatus = "processing"
	EventRetryWait  EventStatus = "retry_wait"
	EventExpanded   EventStatus = "expanded"
	EventSkipped    EventStatus = "skipped"
	EventDeadletter EventStatus = "deadletter"
)

type DeliveryStatus string

const (
	DeliveryPending    DeliveryStatus = "pending"
	DeliveryProcessing DeliveryStatus = "processing"
	DeliveryAccepted   DeliveryStatus = "accepted"
	DeliveryDelivered  DeliveryStatus = "delivered"
	DeliveryRead       DeliveryStatus = "read"
	DeliveryUnknown    DeliveryStatus = "unknown"
	DeliveryRetryWait  DeliveryStatus = "retry_wait"
	DeliveryBlocked    DeliveryStatus = "blocked"
	DeliverySkipped    DeliveryStatus = "skipped"
	DeliveryDeadletter DeliveryStatus = "deadletter"
)

type Channel string

const (
	ChannelInApp           Channel = "in_app"
	ChannelWechatSubscribe Channel = "wechat_subscribe"
	ChannelWecom           Channel = "wecom"
)

type NotificationPurpose string

const (
	PurposeOrder          NotificationPurpose = "order"
	PurposePayment        NotificationPurpose = "payment"
	PurposeDelivery       NotificationPurpose = "delivery"
	PurposePrivateMessage NotificationPurpose = "private_message"
	PurposeRiderIncome    NotificationPurpose = "rider_income"
	PurposeWithdrawal     NotificationPurpose = "withdrawal"
)

type Event struct {
	ID                uint64          `gorm:"column:id;primaryKey"`
	EventPublicID     string          `gorm:"column:event_public_id;size:26;not null;uniqueIndex:uq_delivery_notification_event_public_id"`
	EventType         string          `gorm:"column:event_type;size:64;not null"`
	AggregateType     string          `gorm:"column:aggregate_type;size:32;not null"`
	AggregatePublicID string          `gorm:"column:aggregate_public_id;size:128;not null"`
	AggregateVersion  uint64          `gorm:"column:aggregate_version;not null"`
	PayloadVersion    uint32          `gorm:"column:payload_version;not null"`
	Payload           json.RawMessage `gorm:"column:payload;type:json;not null"`
	Status            EventStatus     `gorm:"column:status;size:24;not null"`
	AttemptCount      uint32          `gorm:"column:attempt_count;not null"`
	NextAttemptAt     *time.Time      `gorm:"column:next_attempt_at"`
	LeaseToken        *string         `gorm:"column:lease_token;size:32"`
	LeaseExpiresAt    *time.Time      `gorm:"column:lease_expires_at"`
	LastErrorCode     string          `gorm:"column:last_error_code;size:64;not null"`
	RowVersion        uint64          `gorm:"column:row_version;not null"`
	OccurredAt        time.Time       `gorm:"column:occurred_at;not null"`
	ExpiresAt         *time.Time      `gorm:"column:expires_at"`
	ProcessedAt       *time.Time      `gorm:"column:processed_at"`
	CreatedAt         time.Time       `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time       `gorm:"column:updated_at;not null"`
}

func (Event) TableName() string { return "delivery_notification_events" }

type InboxItem struct {
	ID                uint64              `gorm:"column:id;primaryKey"`
	InboxPublicID     string              `gorm:"column:inbox_public_id;size:26;not null;uniqueIndex:uq_delivery_notification_inbox_public_id"`
	EventPublicID     string              `gorm:"column:event_public_id;size:26;not null"`
	RecipientUserID   uint64              `gorm:"column:recipient_user_id;not null"`
	Purpose           NotificationPurpose `gorm:"column:purpose;size:48;not null"`
	Title             string              `gorm:"column:title;size:128;not null"`
	Summary           string              `gorm:"column:summary;size:256;not null"`
	AggregateType     string              `gorm:"column:aggregate_type;size:32;not null"`
	AggregatePublicID string              `gorm:"column:aggregate_public_id;size:128;not null"`
	AggregateVersion  uint64              `gorm:"column:aggregate_version;not null"`
	TargetPath        string              `gorm:"column:target_path;size:256;not null"`
	Status            DeliveryStatus      `gorm:"column:status;size:24;not null"`
	DeliveredAt       *time.Time          `gorm:"column:delivered_at"`
	ReadAt            *time.Time          `gorm:"column:read_at"`
	RowVersion        uint64              `gorm:"column:row_version;not null"`
	CreatedAt         time.Time           `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time           `gorm:"column:updated_at;not null"`
}

func (InboxItem) TableName() string { return "delivery_notification_inbox" }

type DeliveryItem struct {
	ID                uint64              `gorm:"column:id;primaryKey"`
	DeliveryPublicID  string              `gorm:"column:delivery_public_id;size:26;not null;uniqueIndex:uq_delivery_notification_delivery_public_id"`
	EventPublicID     string              `gorm:"column:event_public_id;size:26;not null"`
	RecipientUserID   uint64              `gorm:"column:recipient_user_id;not null"`
	Channel           Channel             `gorm:"column:channel;size:24;not null"`
	Purpose           NotificationPurpose `gorm:"column:purpose;size:48;not null"`
	TemplateKey       string              `gorm:"column:template_key;size:64;not null"`
	TemplateSnapshot  json.RawMessage     `gorm:"column:template_snapshot;type:json;not null"`
	PayloadVersion    uint32              `gorm:"column:payload_version;not null"`
	Status            DeliveryStatus      `gorm:"column:status;size:24;not null"`
	AttemptCount      uint32              `gorm:"column:attempt_count;not null"`
	NextAttemptAt     *time.Time          `gorm:"column:next_attempt_at"`
	LeaseToken        *string             `gorm:"column:lease_token;size:32"`
	LeaseExpiresAt    *time.Time          `gorm:"column:lease_expires_at"`
	SubscriptionID    *uint64             `gorm:"column:subscription_id"`
	SubscriptionNonce *string             `gorm:"column:subscription_nonce;size:64"`
	ProviderMessageID string              `gorm:"column:provider_message_id;size:128;not null"`
	LastErrorCode     string              `gorm:"column:last_error_code;size:64;not null"`
	LastError         string              `gorm:"column:last_error;size:256;not null"`
	RowVersion        uint64              `gorm:"column:row_version;not null"`
	ExpiresAt         *time.Time          `gorm:"column:expires_at"`
	ProcessedAt       *time.Time          `gorm:"column:processed_at"`
	CreatedAt         time.Time           `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time           `gorm:"column:updated_at;not null"`
}

func (DeliveryItem) TableName() string { return "delivery_notification_deliveries" }

type Subscription struct {
	ID                 uint64     `gorm:"column:id;primaryKey"`
	UserID             uint64     `gorm:"column:user_id;not null"`
	AppID              string     `gorm:"column:app_id;size:64;not null"`
	OpenID             string     `gorm:"column:openid;size:128;not null"`
	TemplateKey        string     `gorm:"column:template_key;size:64;not null"`
	AuthorizationNonce string     `gorm:"column:authorization_nonce;size:64;not null"`
	Status             string     `gorm:"column:status;size:24;not null"`
	RemainingQuota     uint32     `gorm:"column:remaining_quota;not null"`
	ReservedQuota      uint32     `gorm:"column:reserved_quota;not null"`
	ReservationNonce   *string    `gorm:"column:reservation_nonce;size:64"`
	AuthorizedAt       time.Time  `gorm:"column:authorized_at;not null"`
	ReservedAt         *time.Time `gorm:"column:reserved_at"`
	ExpiresAt          *time.Time `gorm:"column:expires_at"`
	InvalidatedAt      *time.Time `gorm:"column:invalidated_at"`
	InvalidationReason string     `gorm:"column:invalidation_reason;size:64;not null"`
	RowVersion         uint64     `gorm:"column:row_version;not null"`
	CreatedAt          time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;not null"`
}

func (Subscription) TableName() string { return "notification_subscriptions" }

type CallbackReceipt struct {
	ID                   uint64    `gorm:"column:id;primaryKey"`
	SDKAppID             uint64    `gorm:"column:sdk_app_id;not null"`
	ProviderMessageID    string    `gorm:"column:provider_message_id;size:128;not null"`
	SenderAccount        string    `gorm:"column:sender_account;size:64;not null"`
	RecipientAccount     string    `gorm:"column:recipient_account;size:64;not null"`
	SenderUserID         uint64    `gorm:"column:sender_user_id;not null"`
	RecipientUserID      uint64    `gorm:"column:recipient_user_id;not null"`
	CallbackHash         string    `gorm:"column:callback_hash;size:64;not null"`
	ConversationType     string    `gorm:"column:conversation_type;size:16;not null"`
	ConversationPublicID string    `gorm:"column:conversation_public_id;size:128;not null"`
	Status               string    `gorm:"column:status;size:24;not null"`
	OccurredAt           time.Time `gorm:"column:occurred_at;not null"`
	CreatedAt            time.Time `gorm:"column:created_at;not null"`
}

func (CallbackReceipt) TableName() string { return "im_notification_receipts" }

type DeliveryLease struct {
	Item       DeliveryItem
	LeaseToken string
}
type SubscriptionReservation struct {
	Subscription Subscription
	Nonce        string
}

const crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func newID() (string, error) {
	return newIDAt(time.Now().UTC(), rand.Reader)
}

func newIDAt(now time.Time, source io.Reader) (string, error) {
	if source == nil || now.UnixMilli() < 0 || now.UnixMilli() >= 1<<48 {
		return "", ErrInvalidRecord
	}
	raw := make([]byte, 16)
	ms := now.UnixMilli()
	for i := 5; i >= 0; i-- {
		raw[i] = byte(ms)
		ms >>= 8
	}
	if _, err := io.ReadFull(source, raw[6:]); err != nil {
		return "", err
	}
	value := new(bigInt).SetBytes(raw)
	encoded := make([]byte, 26)
	for i := len(encoded) - 1; i >= 0; i-- {
		remainder := value.mod32()
		encoded[i] = crockfordAlphabet[remainder]
		value.div32()
	}
	return string(encoded), nil
}

// small fixed-width integer helper avoids adding a ULID dependency to this domain.
type bigInt struct{ bytes []byte }

func (v *bigInt) SetBytes(raw []byte) *bigInt { v.bytes = append(v.bytes[:0], raw...); return v }
func (v *bigInt) mod32() byte                 { return v.bytes[len(v.bytes)-1] & 31 }
func (v *bigInt) div32() {
	carry := byte(0)
	for i, b := range v.bytes {
		n := uint16(carry)*256 + uint16(b)
		v.bytes[i] = byte(n / 32)
		carry = byte(n % 32)
	}
	for len(v.bytes) > 1 && v.bytes[0] == 0 {
		v.bytes = v.bytes[1:]
	}
}

func validID(value string) bool {
	if len(value) != 26 || value[0] > '7' {
		return false
	}
	for _, r := range value {
		if !strings.ContainsRune(crockfordAlphabet, r) {
			return false
		}
	}
	return true
}

func cleanError(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 256 {
		return value[:256]
	}
	return value
}

func require(value bool, format string, args ...any) error {
	if !value {
		return fmt.Errorf("%w: %s", ErrInvalidRecord, fmt.Sprintf(format, args...))
	}
	return nil
}
