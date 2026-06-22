package takeout

import (
	"strings"
	"time"
	"unicode/utf8"
)

type WithdrawStatus string

const (
	WithdrawPending  WithdrawStatus = "pending"
	WithdrawPaid     WithdrawStatus = "paid"
	WithdrawRejected WithdrawStatus = "rejected"
)

func (s WithdrawStatus) Valid() bool {
	return s == WithdrawPending || s == WithdrawPaid || s == WithdrawRejected
}

func (s WithdrawStatus) Terminal() bool {
	return s == WithdrawPaid || s == WithdrawRejected
}

type WithdrawRequest struct {
	ID               uint64         `gorm:"column:id;primaryKey"`
	WithdrawPublicID string         `gorm:"column:withdraw_public_id;size:26;not null;uniqueIndex:uq_takeout_withdraw_public_id"`
	RiderUserID      uint64         `gorm:"column:rider_user_id;not null;index:idx_takeout_withdraw_rider"`
	AmountCents      uint64         `gorm:"column:amount_cents;not null"`
	Status           WithdrawStatus `gorm:"column:status;size:24;not null;index:idx_takeout_withdraw_status"`
	Note             string         `gorm:"column:note;size:256;not null"`
	PaidAt           *time.Time     `gorm:"column:paid_at"`
	RowVersion       uint64         `gorm:"column:row_version;not null"`
	CreatedAt        time.Time      `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time      `gorm:"column:updated_at;not null"`
}

func (WithdrawRequest) TableName() string { return "takeout_withdraw_requests" }

func (w WithdrawRequest) ValidateNew() error {
	if !ValidULID(w.WithdrawPublicID) || w.RiderUserID == 0 || w.AmountCents == 0 ||
		w.Status != WithdrawPending || w.PaidAt != nil || w.RowVersion != 1 ||
		w.CreatedAt.IsZero() || w.UpdatedAt.IsZero() ||
		utf8.RuneCountInString(strings.TrimSpace(w.Note)) > 256 {
		return ErrInvalidRecord
	}
	return nil
}
