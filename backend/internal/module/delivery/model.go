package delivery

import (
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidRecord     = errors.New("invalid delivery record")
	ErrInvalidTransition = errors.New("invalid delivery transition")
	ErrConcurrentUpdate  = errors.New("concurrent delivery update")
	ErrNotFound          = errors.New("delivery order not found")
	ErrRiderRequired     = errors.New("rider is required")
	ErrRiderMismatch     = errors.New("delivery rider mismatch")
	ErrAmountOverflow    = errors.New("delivery amount overflow")
)

type Status string

const (
	StatusPendingFeePayment Status = "pending_fee_payment"
	StatusPaymentFailed     Status = "payment_failed"
	StatusReadyForClaim     Status = "ready_for_claim"
	StatusClaimed           Status = "claimed"
	StatusToPickup          Status = "to_pickup"
	StatusPickedUp          Status = "picked_up"
	StatusDelivering        Status = "delivering"
	StatusDelivered         Status = "delivered"
	StatusCancelled         Status = "cancelled"
)

func (s Status) Valid() bool {
	switch s {
	case StatusPendingFeePayment, StatusPaymentFailed, StatusReadyForClaim,
		StatusClaimed, StatusToPickup, StatusPickedUp, StatusDelivering,
		StatusDelivered, StatusCancelled:
		return true
	default:
		return false
	}
}

func (s Status) Terminal() bool {
	return s == StatusDelivered || s == StatusCancelled
}

type Order struct {
	ID                    uint64     `gorm:"column:id;primaryKey"`
	DeliveryOrderPublicID string     `gorm:"column:delivery_order_public_id;size:26;not null;uniqueIndex:uq_delivery_order_public_id"`
	MerchantUserID        uint64     `gorm:"column:merchant_user_id;not null;index:idx_delivery_order_merchant_created,priority:1"`
	RiderUserID           *uint64    `gorm:"column:rider_user_id;index:idx_delivery_order_rider_status,priority:1"`
	PickupContactName     string     `gorm:"column:pickup_contact_name;size:64;not null"`
	PickupContactPhone    string     `gorm:"column:pickup_contact_phone;size:32;not null"`
	PickupAddress         string     `gorm:"column:pickup_address;size:256;not null"`
	RecipientName         string     `gorm:"column:recipient_name;size:64;not null"`
	RecipientPhone        string     `gorm:"column:recipient_phone;size:32;not null"`
	DeliveryAddress       string     `gorm:"column:delivery_address;size:256;not null"`
	Remark                string     `gorm:"column:remark;size:512;not null"`
	DeliveryFeeCents      uint64     `gorm:"column:delivery_fee_cents;not null"`
	RiderFeeCents         uint64     `gorm:"column:rider_fee_cents;not null"`
	PlatformCutCents      uint64     `gorm:"column:platform_cut_cents;not null"`
	Status                Status     `gorm:"column:status;size:24;not null;index:idx_delivery_order_status_created,priority:1;index:idx_delivery_order_rider_status,priority:2"`
	ReadyForClaimAt       *time.Time `gorm:"column:ready_for_claim_at"`
	ClaimedAt             *time.Time `gorm:"column:claimed_at"`
	ToPickupAt            *time.Time `gorm:"column:to_pickup_at"`
	PickedUpAt            *time.Time `gorm:"column:picked_up_at"`
	DeliveringAt          *time.Time `gorm:"column:delivering_at"`
	DeliveredAt           *time.Time `gorm:"column:delivered_at"`
	PaymentFailedAt       *time.Time `gorm:"column:payment_failed_at"`
	CancelledAt           *time.Time `gorm:"column:cancelled_at"`
	RowVersion            uint64     `gorm:"column:row_version;not null"`
	CreatedAt             time.Time  `gorm:"column:created_at;not null;index:idx_delivery_order_merchant_created,priority:2;index:idx_delivery_order_status_created,priority:2"`
	UpdatedAt             time.Time  `gorm:"column:updated_at;not null"`
}

func (Order) TableName() string { return "delivery_orders" }

func (o Order) ValidateNew() error {
	fee, ok := checkedAddAmount(o.RiderFeeCents, o.PlatformCutCents)
	if !ValidULID(o.DeliveryOrderPublicID) || o.MerchantUserID == 0 || o.RiderUserID != nil ||
		o.Status != StatusPendingFeePayment || o.DeliveryFeeCents == 0 || !ok ||
		fee != o.DeliveryFeeCents || o.RowVersion != 1 || o.CreatedAt.IsZero() ||
		o.UpdatedAt.IsZero() || !validContactSnapshot(o) {
		return ErrInvalidRecord
	}
	return nil
}

func validContactSnapshot(o Order) bool {
	required := []string{
		o.PickupContactName, o.PickupContactPhone, o.PickupAddress,
		o.RecipientName, o.RecipientPhone, o.DeliveryAddress,
	}
	for _, value := range required {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return utf8.RuneCountInString(o.PickupContactName) <= 64 &&
		utf8.RuneCountInString(o.PickupContactPhone) <= 32 &&
		utf8.RuneCountInString(o.PickupAddress) <= 256 &&
		utf8.RuneCountInString(o.RecipientName) <= 64 &&
		utf8.RuneCountInString(o.RecipientPhone) <= 32 &&
		utf8.RuneCountInString(o.DeliveryAddress) <= 256 &&
		utf8.RuneCountInString(o.Remark) <= 512
}

func checkedAddAmount(left, right uint64) (uint64, bool) {
	if left > math.MaxUint64-right {
		return 0, false
	}
	return left + right, true
}
