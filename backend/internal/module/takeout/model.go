// Package takeout is business domain G. G-1 owns merchant lifecycle facts
// only and has no dependency on A-F domain models or tables.
package takeout

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidRecord     = errors.New("invalid merchant record")
	ErrInvalidTransition = errors.New("invalid merchant state transition")
	ErrNotFound          = errors.New("merchant not found")
	ErrConcurrentUpdate  = errors.New("merchant changed concurrently")
)

type MerchantStatus string

const (
	MerchantPendingReview MerchantStatus = "pending_review"
	MerchantActive        MerchantStatus = "active"
	MerchantResting       MerchantStatus = "resting"
	MerchantRejected      MerchantStatus = "rejected"
	MerchantSuspended     MerchantStatus = "suspended"
)

func (s MerchantStatus) Valid() bool {
	switch s {
	case MerchantPendingReview, MerchantActive, MerchantResting, MerchantRejected, MerchantSuspended:
		return true
	default:
		return false
	}
}

func (s MerchantStatus) Terminal() bool { return s == MerchantRejected }

type Merchant struct {
	ID                uint64         `gorm:"column:id;primaryKey"`
	MerchantPublicID  string         `gorm:"column:merchant_public_id;size:26;not null;uniqueIndex:uq_takeout_merchant_public_id"`
	OwnerUserID       uint64         `gorm:"column:owner_user_id;not null;index:idx_takeout_merchant_owner"`
	Name              string         `gorm:"column:name;size:128;not null"`
	ContactPhone      string         `gorm:"column:contact_phone;size:32;not null"`
	Location          string         `gorm:"column:location;size:128;not null"`
	BusinessHours     string         `gorm:"column:business_hours;size:64;not null"`
	LogoAssetPublicID *string        `gorm:"column:logo_asset_public_id;size:26"`
	Status            MerchantStatus `gorm:"column:status;size:24;not null;index:idx_takeout_merchant_status"`
	RowVersion        uint64         `gorm:"column:row_version;not null"`
	CreatedAt         time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt         time.Time      `gorm:"column:updated_at;not null"`
}

func (Merchant) TableName() string { return "takeout_merchants" }

func (m Merchant) ValidateNew() error {
	if !ValidULID(m.MerchantPublicID) || m.OwnerUserID == 0 || strings.TrimSpace(m.Name) == "" {
		return ErrInvalidRecord
	}
	if utf8.RuneCountInString(m.Name) > 128 || utf8.RuneCountInString(m.ContactPhone) > 32 ||
		utf8.RuneCountInString(m.Location) > 128 || utf8.RuneCountInString(m.BusinessHours) > 64 {
		return ErrInvalidRecord
	}
	if m.LogoAssetPublicID != nil && !ValidULID(*m.LogoAssetPublicID) {
		return ErrInvalidRecord
	}
	if m.Status != MerchantPendingReview || m.RowVersion != 1 || m.CreatedAt.IsZero() || m.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	return nil
}
