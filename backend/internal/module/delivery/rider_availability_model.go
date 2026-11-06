package delivery

import (
	"context"
	"errors"
	"time"
)

var (
	ErrRiderAvailabilityUnavailable = errors.New("rider availability unavailable")
	ErrRiderAvailabilityConflict    = errors.New("rider availability version conflict")
	ErrRiderAtCapacity              = errors.New("rider is at delivery capacity")
	ErrRiderOffDuty                 = errors.New("rider is off duty")
)

type RiderDutyState string

const (
	RiderOffDuty RiderDutyState = "off_duty"
	RiderOnDuty  RiderDutyState = "on_duty"
)

func (s RiderDutyState) Valid() bool { return s == RiderOffDuty || s == RiderOnDuty }

const RiderAvailabilityScopeCampus = "campus"

type RiderAvailability struct {
	ID                  uint64         `gorm:"column:id;primaryKey"`
	RiderUserID         uint64         `gorm:"column:rider_user_id;not null;uniqueIndex:uq_delivery_rider_availability_rider"`
	State               RiderDutyState `gorm:"column:state;size:16;not null"`
	Scope               string         `gorm:"column:scope;size:16;not null"`
	EligibilityRevision string         `gorm:"column:eligibility_revision;size:128;not null"`
	RowVersion          uint64         `gorm:"column:row_version;not null"`
	CreatedAt           time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt           time.Time      `gorm:"column:updated_at;not null"`
}

func (RiderAvailability) TableName() string { return "delivery_rider_availability" }

func (v RiderAvailability) Valid() bool {
	return v.RiderUserID > 0 && v.State.Valid() && v.Scope == RiderAvailabilityScopeCampus && validRiderEligibilityRevision(v.EligibilityRevision) &&
		v.RowVersion > 0 && !v.CreatedAt.IsZero() && !v.UpdatedAt.IsZero()
}

// Identity revisions are opaque ASCII tokens supplied by the composition root.
// They must change on revocation even if identity is restored before the next read.
func validRiderEligibilityRevision(revision string) bool {
	if len(revision) == 0 || len(revision) > 128 {
		return false
	}
	for _, ch := range []byte(revision) {
		if ch < 33 || ch > 126 {
			return false
		}
	}
	return true
}

type RiderBroadcastConfig struct {
	ID                  uint64    `gorm:"column:id;primaryKey"`
	ConfigScope         string    `gorm:"column:config_scope;size:16;not null;uniqueIndex:uq_delivery_rider_broadcast_config_scope"`
	MaxInFlightPerRider uint32    `gorm:"column:max_in_flight_per_rider;not null"`
	RowVersion          uint64    `gorm:"column:row_version;not null"`
	CreatedAt           time.Time `gorm:"column:created_at;not null"`
	UpdatedAt           time.Time `gorm:"column:updated_at;not null"`
}

func (RiderBroadcastConfig) TableName() string { return "delivery_rider_broadcast_config" }

func (v RiderBroadcastConfig) Valid() bool {
	return v.ConfigScope == "global" && v.MaxInFlightPerRider > 0 && v.MaxInFlightPerRider <= 100 &&
		v.RowVersion > 0 && !v.CreatedAt.IsZero() && !v.UpdatedAt.IsZero()
}

type RiderAvailabilitySnapshot struct {
	RiderUserID uint64         `json:"riderUserId"`
	State       RiderDutyState `json:"state"`
	Scope       string         `json:"scope"`
	InFlight    uint32         `json:"inFlight"`
	MaxInFlight uint32         `json:"maxInFlight"`
	RowVersion  uint64         `json:"rowVersion"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

func (v RiderAvailabilitySnapshot) Valid() bool {
	return v.RiderUserID > 0 && v.State.Valid() && v.Scope == RiderAvailabilityScopeCampus &&
		v.MaxInFlight > 0 && v.MaxInFlight <= 100 &&
		((v.RowVersion == 0 && v.State == RiderOffDuty && v.UpdatedAt.IsZero()) ||
			(v.RowVersion > 0 && !v.UpdatedAt.IsZero()))
}

type RiderAvailabilityRepository interface {
	ReadRiderAvailability(context.Context, uint64) (RiderAvailabilitySnapshot, error)
	SetRiderDuty(context.Context, uint64, RiderDutyState, uint64, time.Time) (RiderAvailabilitySnapshot, error)
	ReadRiderBroadcastConfig(context.Context) (RiderBroadcastConfig, error)
	UpdateRiderBroadcastConfig(context.Context, uint32, uint64, time.Time) (RiderBroadcastConfig, error)
	ListOnDutyRiderAvailability(context.Context, time.Time) ([]RiderAvailabilitySnapshot, error)
}

// RiderAvailabilityClaimRepository is optional in unit-only dispatch tests;
// production injects the GORM implementation so claim writes are capacity-safe.
type RiderAvailabilityClaimRepository interface {
	ClaimOrderWithAvailability(context.Context, uint64, string, uint64, time.Time) (Order, error)
}

// UnavailableRiderAvailabilityClaim prevents the legacy claim path from
// bypassing duty/capacity admission while the broadcast feature is disabled.
type UnavailableRiderAvailabilityClaim struct{}

func (UnavailableRiderAvailabilityClaim) ClaimOrderWithAvailability(context.Context, uint64, string, uint64, time.Time) (Order, error) {
	return Order{}, ErrRiderAvailabilityUnavailable
}
