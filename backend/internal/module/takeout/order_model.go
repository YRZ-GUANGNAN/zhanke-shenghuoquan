package takeout

import (
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrDishUnavailable = errors.New("dish is unavailable for order")
	ErrAmountOverflow  = errors.New("order amount overflow")
	ErrRiderRequired   = errors.New("rider is required")
)

type OrderStatus string

const (
	OrderPendingPayment OrderStatus = "pending_payment"
	OrderPaid           OrderStatus = "paid"
	OrderAccepted       OrderStatus = "accepted"
	OrderPreparing      OrderStatus = "preparing"
	OrderReadyForPickup OrderStatus = "ready_for_pickup"
	OrderDelivering     OrderStatus = "delivering"
	OrderCompleted      OrderStatus = "completed"
	OrderCancelled      OrderStatus = "cancelled"
	OrderRefunded       OrderStatus = "refunded"
)

func (s OrderStatus) Valid() bool {
	switch s {
	case OrderPendingPayment, OrderPaid, OrderAccepted, OrderPreparing, OrderReadyForPickup,
		OrderDelivering, OrderCompleted, OrderCancelled, OrderRefunded:
		return true
	default:
		return false
	}
}

func (s OrderStatus) Terminal() bool {
	return s == OrderCompleted || s == OrderCancelled || s == OrderRefunded
}

type SettlementStatus string

const (
	SettlementPending SettlementStatus = "pending_settlement"
	SettlementSettled SettlementStatus = "settled"
)

func (s SettlementStatus) Valid() bool {
	return s == SettlementPending || s == SettlementSettled
}

func (s SettlementStatus) Terminal() bool { return s == SettlementSettled }

type Order struct {
	ID               uint64      `gorm:"column:id;primaryKey"`
	OrderPublicID    string      `gorm:"column:order_public_id;size:26;not null;uniqueIndex:uq_takeout_order_public_id"`
	BuyerUserID      uint64      `gorm:"column:buyer_user_id;not null;index:idx_takeout_order_buyer"`
	MerchantPublicID string      `gorm:"column:merchant_public_id;size:26;not null;index:idx_takeout_order_merchant"`
	RiderUserID      *uint64     `gorm:"column:rider_user_id;index:idx_takeout_order_rider"`
	Status           OrderStatus `gorm:"column:status;size:24;not null;index:idx_takeout_order_status"`
	ItemsAmountCents uint64      `gorm:"column:items_amount_cents;not null"`
	DeliveryFeeCents uint64      `gorm:"column:delivery_fee_cents;not null"`
	TotalAmountCents uint64      `gorm:"column:total_amount_cents;not null"`
	ContactName      string      `gorm:"column:contact_name;size:64;not null"`
	ContactPhone     string      `gorm:"column:contact_phone;size:32;not null"`
	AddressBuilding  string      `gorm:"column:address_building;size:128;not null"`
	AddressRoom      string      `gorm:"column:address_room;size:64;not null"`
	Remark           string      `gorm:"column:remark;size:512;not null"`
	PayNo            *string     `gorm:"column:pay_no;size:64"`
	PaidAt           *time.Time  `gorm:"column:paid_at"`
	AcceptedAt       *time.Time  `gorm:"column:accepted_at"`
	PreparingAt      *time.Time  `gorm:"column:preparing_at"`
	ReadyAt          *time.Time  `gorm:"column:ready_at"`
	DeliveringAt     *time.Time  `gorm:"column:delivering_at"`
	CompletedAt      *time.Time  `gorm:"column:completed_at"`
	CancelledAt      *time.Time  `gorm:"column:cancelled_at"`
	RefundedAt       *time.Time  `gorm:"column:refunded_at"`
	RowVersion       uint64      `gorm:"column:row_version;not null"`
	CreatedAt        time.Time   `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time   `gorm:"column:updated_at;not null"`
}

func (Order) TableName() string { return "takeout_orders" }

func (o Order) ValidateNew() error {
	total, ok := checkedAddAmount(o.ItemsAmountCents, o.DeliveryFeeCents)
	if !ValidULID(o.OrderPublicID) || o.BuyerUserID == 0 || !ValidULID(o.MerchantPublicID) ||
		o.RiderUserID != nil || o.Status != OrderPendingPayment || o.ItemsAmountCents == 0 ||
		!ok || total != o.TotalAmountCents || o.PayNo != nil || o.RowVersion != 1 ||
		o.CreatedAt.IsZero() || o.UpdatedAt.IsZero() || !validOrderContact(o) {
		return ErrInvalidRecord
	}
	return nil
}

func validOrderContact(o Order) bool {
	return utf8.RuneCountInString(o.ContactName) <= 64 && utf8.RuneCountInString(o.ContactPhone) <= 32 &&
		utf8.RuneCountInString(o.AddressBuilding) <= 128 && utf8.RuneCountInString(o.AddressRoom) <= 64 &&
		utf8.RuneCountInString(o.Remark) <= 512
}

type OrderItem struct {
	ID                     uint64    `gorm:"column:id;primaryKey"`
	OrderPublicID          string    `gorm:"column:order_public_id;size:26;not null;index:idx_takeout_order_item_order"`
	DishPublicID           string    `gorm:"column:dish_public_id;size:26;not null"`
	DishNameSnapshot       string    `gorm:"column:dish_name_snapshot;size:128;not null"`
	UnitPriceCentsSnapshot uint64    `gorm:"column:unit_price_cents_snapshot;not null"`
	Quantity               uint32    `gorm:"column:quantity;not null"`
	LineAmountCents        uint64    `gorm:"column:line_amount_cents;not null"`
	CreatedAt              time.Time `gorm:"column:created_at;not null"`
}

func (OrderItem) TableName() string { return "takeout_order_items" }

func (i OrderItem) ValidateNew() error {
	line, ok := checkedMultiplyAmount(i.UnitPriceCentsSnapshot, i.Quantity)
	if !ValidULID(i.OrderPublicID) || !ValidULID(i.DishPublicID) ||
		strings.TrimSpace(i.DishNameSnapshot) == "" || utf8.RuneCountInString(i.DishNameSnapshot) > 128 ||
		i.UnitPriceCentsSnapshot == 0 || i.Quantity == 0 || !ok || line != i.LineAmountCents || i.CreatedAt.IsZero() {
		return ErrInvalidRecord
	}
	return nil
}

type RiderEarningRecord struct {
	ID               uint64           `gorm:"column:id;primaryKey"`
	EarningPublicID  string           `gorm:"column:earning_public_id;size:26;not null;uniqueIndex:uq_takeout_earning_public_id"`
	OrderPublicID    string           `gorm:"column:order_public_id;size:26;not null;uniqueIndex:uq_takeout_earning_order"`
	RiderUserID      uint64           `gorm:"column:rider_user_id;not null"`
	RiderFeeCents    uint64           `gorm:"column:rider_fee_cents;not null"`
	PlatformCutCents uint64           `gorm:"column:platform_cut_cents;not null"`
	SettlementStatus SettlementStatus `gorm:"column:settlement_status;size:24;not null;index:idx_takeout_earning_settlement_status"`
	SettledAt        *time.Time       `gorm:"column:settled_at"`
	RowVersion       uint64           `gorm:"column:row_version;not null"`
	CreatedAt        time.Time        `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time        `gorm:"column:updated_at;not null"`
}

func (RiderEarningRecord) TableName() string { return "takeout_rider_earning_records" }

func (r RiderEarningRecord) ValidateNew() error {
	if !ValidULID(r.EarningPublicID) || !ValidULID(r.OrderPublicID) || r.RiderUserID == 0 ||
		r.SettlementStatus != SettlementPending || r.SettledAt != nil || r.RowVersion != 1 ||
		r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	return nil
}

type FeeConfig struct {
	ID               uint64    `gorm:"column:id;primaryKey"`
	ConfigScope      string    `gorm:"column:config_scope;size:32;not null;uniqueIndex:uq_takeout_fee_config_scope"`
	RiderFeeCents    uint64    `gorm:"column:rider_fee_cents;not null"`
	PlatformCutCents uint64    `gorm:"column:platform_cut_cents;not null"`
	RowVersion       uint64    `gorm:"column:row_version;not null"`
	UpdatedAt        time.Time `gorm:"column:updated_at;not null"`
}

func (FeeConfig) TableName() string { return "takeout_fee_config" }

func (f FeeConfig) Valid() bool {
	_, ok := checkedAddAmount(f.RiderFeeCents, f.PlatformCutCents)
	return f.ConfigScope == "global" && f.RowVersion > 0 && !f.UpdatedAt.IsZero() && ok
}

func checkedMultiplyAmount(unit uint64, quantity uint32) (uint64, bool) {
	if quantity == 0 || unit > math.MaxUint64/uint64(quantity) {
		return 0, false
	}
	return unit * uint64(quantity), true
}

func checkedAddAmount(left, right uint64) (uint64, bool) {
	if left > math.MaxUint64-right {
		return 0, false
	}
	return left + right, true
}
