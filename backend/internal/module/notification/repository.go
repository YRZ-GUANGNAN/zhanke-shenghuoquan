package notification

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	AppendEventTx(tx *gorm.DB, event Event) error
	EnsureInboxTx(tx *gorm.DB, item InboxItem) error
	EnsureDeliveryTx(tx *gorm.DB, item DeliveryItem) error
	ClaimNextDelivery(ctx context.Context, now time.Time, lease time.Duration) (DeliveryLease, bool, error)
	CompleteDelivery(ctx context.Context, lease DeliveryLease, status DeliveryStatus, now time.Time) error
	FailDelivery(ctx context.Context, lease DeliveryLease, code, safeMessage string, next time.Time, max uint32, now time.Time) (DeliveryStatus, error)
	ReserveSubscription(ctx context.Context, userID uint64, appID, templateKey, nonce string, now time.Time) (SubscriptionReservation, error)
	RecordCallbackOnce(ctx context.Context, receipt CallbackReceipt) (bool, error)
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

var _ Repository = (*GormRepository)(nil)

func (r *GormRepository) AppendEventTx(tx *gorm.DB, event Event) error {
	if tx == nil || !validID(event.EventPublicID) || strings.TrimSpace(event.EventType) == "" || strings.TrimSpace(event.AggregateType) == "" || strings.TrimSpace(event.AggregatePublicID) == "" || event.AggregateVersion == 0 || event.PayloadVersion == 0 || len(event.Payload) == 0 || event.OccurredAt.IsZero() || event.CreatedAt.IsZero() || event.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	if event.Status == "" {
		event.Status = EventPending
	}
	if event.RowVersion == 0 {
		event.RowVersion = 1
	}
	result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&event)
	return result.Error
}

func (r *GormRepository) EnsureInboxTx(tx *gorm.DB, item InboxItem) error {
	if tx == nil || !validID(item.InboxPublicID) || !validID(item.EventPublicID) || item.RecipientUserID == 0 || strings.TrimSpace(string(item.Purpose)) == "" || strings.TrimSpace(item.Title) == "" || item.AggregateVersion == 0 || item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	if item.Status == "" {
		item.Status = DeliveryPending
	}
	if item.RowVersion == 0 {
		item.RowVersion = 1
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&item).Error
}

func (r *GormRepository) EnsureDeliveryTx(tx *gorm.DB, item DeliveryItem) error {
	if tx == nil || !validID(item.DeliveryPublicID) || !validID(item.EventPublicID) || item.RecipientUserID == 0 || !validChannel(item.Channel) || strings.TrimSpace(string(item.Purpose)) == "" || len(item.TemplateSnapshot) == 0 || item.PayloadVersion == 0 || item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	if item.Status == "" {
		item.Status = DeliveryPending
	}
	if item.RowVersion == 0 {
		item.RowVersion = 1
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&item).Error
}

func (r *GormRepository) ClaimNextDelivery(ctx context.Context, now time.Time, lease time.Duration) (DeliveryLease, bool, error) {
	if now.IsZero() || lease <= 0 {
		return DeliveryLease{}, false, ErrInvalidRecord
	}
	var result DeliveryLease
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item DeliveryItem
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("(status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)) OR (status = ? AND lease_expires_at <= ?)", []DeliveryStatus{DeliveryPending, DeliveryRetryWait}, now, DeliveryProcessing, now).
			Order("id ASC").First(&item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		token, err := newID()
		if err != nil {
			return err
		}
		expires := now.Add(lease)
		update := tx.Model(&DeliveryItem{}).Where("id = ? AND row_version = ?", item.ID, item.RowVersion).Updates(map[string]any{
			"status": DeliveryProcessing, "attempt_count": gorm.Expr("attempt_count + 1"), "lease_token": token, "lease_expires_at": expires, "next_attempt_at": nil, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		item.Status, item.AttemptCount, item.LeaseToken, item.LeaseExpiresAt, item.NextAttemptAt, item.RowVersion, item.UpdatedAt = DeliveryProcessing, item.AttemptCount+1, &token, &expires, nil, item.RowVersion+1, now
		result = DeliveryLease{Item: item, LeaseToken: token}
		return nil
	})
	if err != nil || result.LeaseToken == "" {
		return DeliveryLease{}, false, err
	}
	return result, true, nil
}

func (r *GormRepository) CompleteDelivery(ctx context.Context, lease DeliveryLease, status DeliveryStatus, now time.Time) error {
	if lease.Item.ID == 0 || strings.TrimSpace(lease.LeaseToken) == "" || now.IsZero() || !terminalDeliveryStatus(status) {
		return ErrInvalidRecord
	}
	result := r.db.WithContext(ctx).Model(&DeliveryItem{}).Where("id = ? AND status = ? AND row_version = ? AND lease_token = ? AND lease_expires_at > ?", lease.Item.ID, DeliveryProcessing, lease.Item.RowVersion, lease.LeaseToken, now).Updates(map[string]any{
		"status": status, "lease_token": nil, "lease_expires_at": nil, "processed_at": now, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrConcurrentUpdate
	}
	return nil
}

func (r *GormRepository) FailDelivery(ctx context.Context, lease DeliveryLease, code, safeMessage string, next time.Time, max uint32, now time.Time) (DeliveryStatus, error) {
	if lease.Item.ID == 0 || strings.TrimSpace(lease.LeaseToken) == "" || strings.TrimSpace(code) == "" || now.IsZero() || max == 0 {
		return "", ErrInvalidRecord
	}
	status := DeliveryRetryWait
	if lease.Item.AttemptCount >= max {
		status = DeliveryDeadletter
	}
	updates := map[string]any{"status": status, "lease_token": nil, "lease_expires_at": nil, "last_error_code": strings.TrimSpace(code), "last_error": cleanError(safeMessage), "row_version": gorm.Expr("row_version + 1"), "updated_at": now}
	if status == DeliveryRetryWait {
		if next.IsZero() {
			return "", ErrInvalidRecord
		}
		updates["next_attempt_at"] = next
	} else {
		updates["next_attempt_at"] = nil
		updates["processed_at"] = now
	}
	result := r.db.WithContext(ctx).Model(&DeliveryItem{}).Where("id = ? AND status = ? AND row_version = ? AND lease_token = ? AND lease_expires_at > ?", lease.Item.ID, DeliveryProcessing, lease.Item.RowVersion, lease.LeaseToken, now).Updates(updates)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected != 1 {
		return "", ErrConcurrentUpdate
	}
	return status, nil
}

func (r *GormRepository) ReserveSubscription(ctx context.Context, userID uint64, appID, templateKey, nonce string, now time.Time) (SubscriptionReservation, error) {
	if userID == 0 || strings.TrimSpace(appID) == "" || strings.TrimSpace(templateKey) == "" || strings.TrimSpace(nonce) == "" || now.IsZero() {
		return SubscriptionReservation{}, ErrInvalidRecord
	}
	var result SubscriptionReservation
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var value Subscription
		// A request retry must reuse its prior occupied quota, including when
		// another authorization has since become available.
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND app_id = ? AND template_key = ? AND reservation_nonce = ?", userID, appID, templateKey, nonce).First(&value).Error
		if err == nil {
			result = SubscriptionReservation{Subscription: value, Nonce: nonce}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND app_id = ? AND template_key = ? AND status = ? AND remaining_quota > 0 AND reservation_nonce IS NULL AND (expires_at IS NULL OR expires_at > ?)", userID, appID, templateKey, "accepted", now).Order("id DESC").First(&value).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSubscriptionEmpty
		}
		if err != nil {
			return err
		}
		reserved := now
		update := tx.Model(&Subscription{}).Where("id = ? AND row_version = ? AND remaining_quota > 0 AND reservation_nonce IS NULL", value.ID, value.RowVersion).Updates(map[string]any{"remaining_quota": gorm.Expr("remaining_quota - 1"), "reserved_quota": 1, "reservation_nonce": nonce, "reserved_at": reserved, "row_version": gorm.Expr("row_version + 1"), "updated_at": now})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		value.RemainingQuota--
		value.ReservedQuota = 1
		value.ReservationNonce = &nonce
		value.ReservedAt = &reserved
		value.RowVersion++
		value.UpdatedAt = now
		result = SubscriptionReservation{Subscription: value, Nonce: nonce}
		return nil
	})
	return result, err
}

func (r *GormRepository) RecordCallbackOnce(ctx context.Context, receipt CallbackReceipt) (bool, error) {
	if receipt.SDKAppID == 0 || strings.TrimSpace(receipt.ProviderMessageID) == "" || receipt.SenderUserID == 0 || receipt.RecipientUserID == 0 || strings.TrimSpace(receipt.SenderAccount) == "" || strings.TrimSpace(receipt.RecipientAccount) == "" || strings.TrimSpace(receipt.CallbackHash) == "" || receipt.OccurredAt.IsZero() || receipt.CreatedAt.IsZero() {
		return false, ErrInvalidRecord
	}
	if receipt.Status == "" {
		receipt.Status = "received"
	}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&receipt)
	return result.RowsAffected == 1, result.Error
}

func validChannel(value Channel) bool {
	return value == ChannelInApp || value == ChannelWechatSubscribe || value == ChannelWecom
}
func terminalDeliveryStatus(value DeliveryStatus) bool {
	return value == DeliveryAccepted || value == DeliveryDelivered || value == DeliveryRead || value == DeliveryUnknown || value == DeliveryBlocked || value == DeliverySkipped || value == DeliveryDeadletter
}
