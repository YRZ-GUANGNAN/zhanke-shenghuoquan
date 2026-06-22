package takeout

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrRiderUnavailable    = errors.New("rider is not active")
	ErrInFlightLimit       = errors.New("rider in-flight limit reached")
	ErrOrderAlreadyClaimed = errors.New("order already claimed")
)

type RiderStatus string

const (
	RiderActive    RiderStatus = "active"
	RiderResting   RiderStatus = "resting"
	RiderSuspended RiderStatus = "suspended"
)

func (s RiderStatus) Valid() bool {
	return s == RiderActive || s == RiderResting || s == RiderSuspended
}

func (RiderStatus) Terminal() bool { return false }

type FulfillmentStatus string

const (
	FulfillmentAssigned  FulfillmentStatus = "ASSIGNED"
	FulfillmentToStore   FulfillmentStatus = "TO_STORE"
	FulfillmentPicked    FulfillmentStatus = "PICKED"
	FulfillmentToUser    FulfillmentStatus = "TO_USER"
	FulfillmentDelivered FulfillmentStatus = "DELIVERED"
)

func (s FulfillmentStatus) Valid() bool {
	switch s {
	case FulfillmentAssigned, FulfillmentToStore, FulfillmentPicked, FulfillmentToUser, FulfillmentDelivered:
		return true
	default:
		return false
	}
}

func (s FulfillmentStatus) Terminal() bool { return s == FulfillmentDelivered }

type Rider struct {
	ID          uint64      `gorm:"column:id;primaryKey"`
	RiderUserID uint64      `gorm:"column:rider_user_id;not null;uniqueIndex:uq_takeout_rider_user"`
	Status      RiderStatus `gorm:"column:status;size:24;not null;index:idx_takeout_rider_status"`
	RealName    string      `gorm:"column:real_name;size:64;not null"`
	Phone       string      `gorm:"column:phone;size:32;not null"`
	RowVersion  uint64      `gorm:"column:row_version;not null"`
	CreatedAt   time.Time   `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time   `gorm:"column:updated_at;not null"`
}

func (Rider) TableName() string { return "takeout_riders" }

func (r Rider) ValidateNew() error {
	if r.RiderUserID == 0 || r.Status != RiderActive || r.RowVersion != 1 ||
		r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() ||
		utf8.RuneCountInString(strings.TrimSpace(r.RealName)) > 64 ||
		utf8.RuneCountInString(strings.TrimSpace(r.Phone)) > 32 {
		return ErrInvalidRecord
	}
	return nil
}

type Fulfillment struct {
	ID                  uint64            `gorm:"column:id;primaryKey"`
	FulfillmentPublicID string            `gorm:"column:fulfillment_public_id;size:26;not null;uniqueIndex:uq_takeout_fulfillment_public_id"`
	OrderPublicID       string            `gorm:"column:order_public_id;size:26;not null;uniqueIndex:uq_takeout_fulfillment_order"`
	RiderUserID         uint64            `gorm:"column:rider_user_id;not null;index:idx_takeout_fulfillment_rider;index:idx_takeout_fulfillment_rider_status,priority:1"`
	Status              FulfillmentStatus `gorm:"column:status;size:24;not null;index:idx_takeout_fulfillment_status;index:idx_takeout_fulfillment_rider_status,priority:2"`
	AssignedAt          *time.Time        `gorm:"column:assigned_at"`
	ToStoreAt           *time.Time        `gorm:"column:to_store_at"`
	PickedAt            *time.Time        `gorm:"column:picked_at"`
	ToUserAt            *time.Time        `gorm:"column:to_user_at"`
	DeliveredAt         *time.Time        `gorm:"column:delivered_at"`
	RowVersion          uint64            `gorm:"column:row_version;not null"`
	CreatedAt           time.Time         `gorm:"column:created_at;not null"`
	UpdatedAt           time.Time         `gorm:"column:updated_at;not null"`
}

func (Fulfillment) TableName() string { return "takeout_fulfillments" }

func (f Fulfillment) ValidateNew() error {
	if !ValidULID(f.FulfillmentPublicID) || !ValidULID(f.OrderPublicID) || f.RiderUserID == 0 ||
		f.Status != FulfillmentAssigned || f.AssignedAt == nil || f.AssignedAt.IsZero() ||
		f.ToStoreAt != nil || f.PickedAt != nil || f.ToUserAt != nil || f.DeliveredAt != nil ||
		f.RowVersion != 1 || f.CreatedAt.IsZero() || f.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	return nil
}

type RiderConfig struct {
	ID                  uint64    `gorm:"column:id;primaryKey"`
	ConfigScope         string    `gorm:"column:config_scope;size:32;not null;uniqueIndex:uq_takeout_rider_config_scope"`
	MaxInFlightPerRider uint32    `gorm:"column:max_in_flight_per_rider;not null"`
	RowVersion          uint64    `gorm:"column:row_version;not null"`
	UpdatedAt           time.Time `gorm:"column:updated_at;not null"`
}

func (RiderConfig) TableName() string { return "takeout_rider_config" }

func (c RiderConfig) Valid() bool {
	return c.ConfigScope == "global" && c.MaxInFlightPerRider > 0 && c.RowVersion > 0 && !c.UpdatedAt.IsZero()
}
