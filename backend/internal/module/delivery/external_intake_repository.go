package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Only normalized business data is stored. Do not serialize this internal model
// directly to a public response (it contains customer contact information).
type ExternalInbox struct {
	ID             uint64 `gorm:"primaryKey"`
	BindingKey     string
	MerchantUserID uint64
	Platform       SourcePlatform
	ShopID         string
	SourceOrderID  string
	EventID        string
	EventType      ExternalOrderEventType
	OccurredAt     time.Time
	Payload        json.RawMessage `gorm:"type:json"`
	Fingerprint    string
	RequestID      string
	Status         string
	Attempts       uint32
	LeaseToken     *string
	LeaseExpiresAt *time.Time
	NextAttemptAt  *time.Time
	OrderPublicID  string
	Reason         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (ExternalInbox) TableName() string { return "delivery_external_inbox" }

func (s *ExternalIntake) insert(ctx context.Context, row *ExternalInbox) (bool, error) {
	err := s.db.WithContext(ctx).Create(row).Error
	if err == nil {
		return false, nil
	}
	if !isDuplicateKey(err) {
		return false, err
	}
	var existing ExternalInbox
	if err := s.db.WithContext(ctx).Where("platform = ? AND shop_id = ? AND event_id = ?", row.Platform, row.ShopID, row.EventID).First(&existing).Error; err != nil {
		return false, err
	}
	if existing.MerchantUserID != row.MerchantUserID || existing.BindingKey != row.BindingKey || existing.Fingerprint != row.Fingerprint {
		return false, ErrExternalEventConflict
	}
	*row = existing
	return true, nil
}

func (s *ExternalIntake) claim(ctx context.Context) (ExternalInbox, bool, error) {
	var row ExternalInbox
	found := false
	now := s.now().UTC()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = 'pending' OR (status = 'retry_wait' AND next_attempt_at <= ?) OR (status = 'processing' AND lease_expires_at <= ?)", now, now).
			Order("id ASC").First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		token, err := NewULID()
		if err != nil {
			return err
		}
		expires := now.Add(s.lease)
		row.Status, row.Attempts, row.LeaseToken, row.LeaseExpiresAt, row.UpdatedAt = "processing", row.Attempts+1, &token, &expires, now
		found = true
		return tx.Model(&row).Updates(map[string]any{"status": row.Status, "attempts": row.Attempts, "lease_token": token, "lease_expires_at": expires, "updated_at": now}).Error
	})
	return row, found, err
}

func leasedInbox(tx *gorm.DB, row *ExternalInbox, now time.Time) *gorm.DB {
	return tx.Model(&ExternalInbox{}).Where("id = ? AND status = 'processing' AND lease_token = ? AND lease_expires_at > ?", row.ID, row.LeaseToken, now)
}

func finishInbox(tx *gorm.DB, row *ExternalInbox, status, reason string, now time.Time) error {
	result := leasedInbox(tx, row, now).Updates(map[string]any{"status": status, "reason": reason, "order_public_id": row.OrderPublicID, "lease_token": nil, "lease_expires_at": nil, "next_attempt_at": nil, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrExternalLease
	}
	row.Status, row.Reason = status, reason
	return nil
}

func (s *ExternalIntake) finish(ctx context.Context, row *ExternalInbox, status, reason string) error {
	return finishInbox(s.db.WithContext(ctx), row, status, reason, s.now().UTC())
}

func (s *ExternalIntake) process(ctx context.Context, row *ExternalInbox) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked ExternalInbox
		if err := leasedInbox(tx, row, s.now().UTC()).Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrExternalLease
			}
			return err
		}
		var event ExternalOrderEvent
		if json.Unmarshal(row.Payload, &event) != nil {
			return finishInbox(tx, row, "needs_review", "invalid_payload", s.now().UTC())
		}
		if row.EventType != ExternalOrderCreated {
			return finishInbox(tx, row, "needs_review", "source_change_requires_review", s.now().UTC())
		}
		// A change/cancellation received before create is a tombstone, not an
		// instruction to resurrect or overwrite a delivery using a stale snapshot.
		var changes int64
		if err := tx.Model(&ExternalInbox{}).Where("platform = ? AND shop_id = ? AND source_order_id = ? AND event_type <> ?", row.Platform, row.ShopID, row.SourceOrderID, ExternalOrderCreated).Count(&changes).Error; err != nil {
			return err
		}
		if changes > 0 {
			return finishInbox(tx, row, "needs_review", "source_changed", s.now().UTC())
		}
		var existing Order
		err := tx.Where("merchant_user_id = ? AND source_platform = ? AND source_shop_id = ? AND source_order_id = ?", row.MerchantUserID, row.Platform, row.ShopID, row.SourceOrderID).First(&existing).Error
		if err == nil {
			row.OrderPublicID = existing.DeliveryOrderPublicID
			want := Order{MerchantUserID: row.MerchantUserID, MerchantRequestID: existing.MerchantRequestID,
				SourcePlatform: row.Platform, SourceShopID: &row.ShopID, SourceOrderID: &row.SourceOrderID, PickupCode: event.Source.PickupCode,
				PickupContactName: event.Pickup.Name, PickupContactPhone: event.Pickup.Phone, PickupAddress: event.Pickup.Address,
				RecipientName: event.Recipient.Name, RecipientPhone: event.Recipient.Phone, DeliveryAddress: event.Recipient.Address, Remark: event.Remark}
			if !sameCreateSnapshot(existing, want) {
				return finishInbox(tx, row, "needs_review", "order_snapshot_conflict", s.now().UTC())
			}
			return finishInbox(tx, row, "reused", "", s.now().UTC())
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Reuse the exact manual-create pipeline inside this transaction. It
		// reads server fees, validates contacts and creates only pending payment.
		dispatch := &DispatchService{repository: NewRepository(tx), feeProvider: s.fees, now: s.now}
		order, err := dispatch.CreateOrder(ctx, CreateDispatchInput{MerchantUserID: row.MerchantUserID, MerchantRequestID: row.RequestID,
			SourcePlatform: row.Platform, SourceShopID: row.ShopID, SourceOrderID: row.SourceOrderID, PickupCode: event.Source.PickupCode,
			Pickup: event.Pickup, Recipient: event.Recipient, Remark: event.Remark})
		if errors.Is(err, ErrInvalidRecord) {
			return finishInbox(tx, row, "needs_review", "invalid_order_or_fee", s.now().UTC())
		}
		if err != nil {
			return err
		}
		row.OrderPublicID = order.DeliveryOrderPublicID
		return finishInbox(tx, row, "created", "", s.now().UTC())
	})
}

func (s *ExternalIntake) fail(ctx context.Context, row *ExternalInbox) error {
	now := s.now().UTC()
	status := "retry_wait"
	if row.Attempts >= s.maxAttempts {
		status = "deadletter"
	}
	shift := min(row.Attempts, uint32(8))
	next := now.Add(time.Second * time.Duration(1<<shift))
	// Never persist DB/provider error strings, which can contain contact data.
	result := leasedInbox(s.db.WithContext(ctx), row, now).Updates(map[string]any{"status": status, "reason": "processing_failed", "next_attempt_at": next, "lease_token": nil, "lease_expires_at": nil, "updated_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrExternalLease
	}
	row.Status, row.Reason, row.OrderPublicID = status, "processing_failed", ""
	return nil
}
