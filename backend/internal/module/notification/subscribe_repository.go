package notification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func authorizationTemplateTag(appID, key, templateID string) string {
	digest := sha256.Sum256([]byte(appID + "\x00" + key + "\x00" + templateID))
	return hex.EncodeToString(digest[:16])
}

func (r *GormRepository) FindEvent(ctx context.Context, id string) (Event, error) {
	if r == nil || r.db == nil || !validID(id) {
		return Event{}, ErrInvalidRecord
	}
	var event Event
	err := r.db.WithContext(ctx).Where("event_public_id = ?", id).First(&event).Error
	return event, err
}

func liveSubscribeLease(tx *gorm.DB, lease DeliveryLease, now time.Time) *gorm.DB {
	return tx.Model(&DeliveryItem{}).Where("id = ? AND channel = ? AND status = ? AND row_version = ? AND lease_token = ? AND lease_expires_at > ? AND lease_expires_at > UTC_TIMESTAMP(3)", lease.Item.ID, ChannelWechatSubscribe, DeliveryProcessing, lease.Item.RowVersion, lease.LeaseToken, now)
}

// Recovery never releases consent after an unobserved external request. The
// terminal unknown marker and quota consumption commit together.
func expireExternalDeliveriesTx(tx *gorm.DB, now time.Time) error {
	var expired []DeliveryItem
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("channel IN ? AND status = ? AND lease_expires_at <= ?", []Channel{ChannelWechatSubscribe, ChannelWecom}, DeliveryProcessing, now).Order("id ASC").Limit(100).Find(&expired).Error; err != nil {
		return err
	}
	for _, item := range expired {
		if item.SubscriptionID != nil && item.SubscriptionNonce != nil {
			if err := tx.Model(&Subscription{}).Where("id = ? AND user_id = ? AND reservation_nonce = ? AND reserved_quota = 1", *item.SubscriptionID, item.RecipientUserID, *item.SubscriptionNonce).Updates(map[string]any{
				"remaining_quota": 0, "reserved_quota": 0, "reservation_nonce": nil, "reserved_at": nil,
				"status": gorm.Expr("CASE WHEN status = 'accepted' THEN 'exhausted' ELSE status END"), "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			}).Error; err != nil {
				return err
			}
		}
		update := tx.Model(&DeliveryItem{}).Where("id = ? AND status = ? AND row_version = ?", item.ID, DeliveryProcessing, item.RowVersion).Updates(map[string]any{"status": DeliveryUnknown, "lease_token": nil, "lease_expires_at": nil, "next_attempt_at": nil, "last_error_code": "lease_expired_unknown", "processed_at": now, "row_version": gorm.Expr("row_version + 1"), "updated_at": now})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
	}
	return nil
}

func (r *GormRepository) ReserveForDelivery(ctx context.Context, lease DeliveryLease, appID, openid, templateID string, now time.Time) (DeliveryLease, error) {
	if r == nil || r.db == nil || lease.Item.ID == 0 || lease.LeaseToken == "" || now.IsZero() || appID == "" || openid == "" || templateID == "" {
		return DeliveryLease{}, ErrInvalidRecord
	}
	var result DeliveryLease
	err := r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Discard}).Transaction(func(tx *gorm.DB) error {
		var item DeliveryItem
		if err := liveSubscribeLease(tx, lease, now).Clauses(clause.Locking{Strength: "UPDATE"}).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrConcurrentUpdate
			}
			return err
		}
		if item.SubscriptionID != nil || item.SubscriptionNonce != nil || item.RecipientUserID != lease.Item.RecipientUserID || item.TemplateKey != lease.Item.TemplateKey || (item.ExpiresAt != nil && !item.ExpiresAt.After(now)) {
			return ErrConcurrentUpdate
		}
		var subscription Subscription
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND app_id = ? AND openid = ? AND template_key = ? AND status = ? AND remaining_quota = 1 AND reserved_quota = 0 AND reservation_nonce IS NULL AND RIGHT(authorization_nonce,32) = ? AND (expires_at IS NULL OR expires_at > ?)", item.RecipientUserID, appID, openid, item.TemplateKey, "accepted", authorizationTemplateTag(appID, item.TemplateKey, templateID), now).Order("id ASC").First(&subscription).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrSubscriptionEmpty
		}
		if err != nil {
			return err
		}
		nonce := item.DeliveryPublicID
		update := tx.Model(&Subscription{}).Where("id = ? AND row_version = ? AND remaining_quota = 1 AND reservation_nonce IS NULL", subscription.ID, subscription.RowVersion).Updates(map[string]any{"remaining_quota": 0, "reserved_quota": 1, "reservation_nonce": nonce, "reserved_at": now, "row_version": gorm.Expr("row_version + 1"), "updated_at": now})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		update = liveSubscribeLease(tx, lease, now).Updates(map[string]any{"subscription_id": subscription.ID, "subscription_nonce": nonce, "row_version": gorm.Expr("row_version + 1"), "updated_at": now})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		item.SubscriptionID = &subscription.ID
		item.SubscriptionNonce = &nonce
		item.RowVersion++
		item.UpdatedAt = now
		result = DeliveryLease{Item: item, LeaseToken: lease.LeaseToken}
		return nil
	})
	return result, err
}

func (r *GormRepository) FinishSubscribe(ctx context.Context, lease DeliveryLease, outcome SubscribeOutcome, now time.Time) error {
	if r == nil || r.db == nil || lease.Item.ID == 0 || lease.LeaseToken == "" || now.IsZero() || !validSubscribeOutcome(outcome) {
		return ErrInvalidRecord
	}
	return r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Discard}).Transaction(func(tx *gorm.DB) error {
		var item DeliveryItem
		if err := liveSubscribeLease(tx, lease, now).Clauses(clause.Locking{Strength: "UPDATE"}).First(&item).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrConcurrentUpdate
			}
			return err
		}
		if item.SubscriptionID != nil {
			if item.SubscriptionNonce == nil || outcome.QuotaAction == "none" {
				return ErrInvalidRecord
			}
			var subscription Subscription
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND user_id = ? AND template_key = ? AND reservation_nonce = ? AND reserved_quota = 1", *item.SubscriptionID, item.RecipientUserID, item.TemplateKey, *item.SubscriptionNonce).First(&subscription).Error; err != nil {
				return ErrConcurrentUpdate
			}
			updates := map[string]any{"reserved_quota": 0, "reservation_nonce": nil, "reserved_at": nil, "row_version": gorm.Expr("row_version + 1"), "updated_at": now}
			switch outcome.QuotaAction {
			case "consume":
				updates["remaining_quota"] = 0
				if subscription.Status == "accepted" {
					updates["status"] = "exhausted"
				}
			case "release":
				if subscription.Status == "accepted" && (subscription.ExpiresAt == nil || subscription.ExpiresAt.After(now)) {
					updates["remaining_quota"] = 1
				} else {
					updates["remaining_quota"] = 0
				}
			case "invalidate":
				updates["remaining_quota"] = 0
				updates["status"] = "invalid"
				updates["invalidated_at"] = now
			default:
				return ErrInvalidRecord
			}
			update := tx.Model(&Subscription{}).Where("id = ? AND row_version = ?", subscription.ID, subscription.RowVersion).Updates(updates)
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return ErrConcurrentUpdate
			}
		} else if outcome.QuotaAction != "none" {
			return ErrInvalidRecord
		}
		updates := map[string]any{"status": outcome.Status, "last_error_code": outcome.Code, "last_error": "", "provider_message_id": outcome.ProviderMessageID, "lease_token": nil, "lease_expires_at": nil, "next_attempt_at": outcome.NextAttemptAt, "processed_at": now, "row_version": gorm.Expr("row_version + 1"), "updated_at": now}
		if outcome.Status == DeliveryRetryWait {
			updates["processed_at"] = nil
			updates["subscription_id"] = nil
			updates["subscription_nonce"] = nil
		}
		update := liveSubscribeLease(tx, lease, now).Updates(updates)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		return nil
	})
}

func validSubscribeOutcome(value SubscribeOutcome) bool {
	if !safeText(value.Code, 64, false) || (value.Code != "" && !safeIdentifier.MatchString(value.Code)) || !safeText(value.ProviderMessageID, 128, false) {
		return false
	}
	if value.Status == DeliveryRetryWait {
		return value.NextAttemptAt != nil && !value.NextAttemptAt.IsZero() && (value.QuotaAction == "release" || value.QuotaAction == "none")
	}
	if value.NextAttemptAt != nil {
		return false
	}
	switch value.Status {
	case DeliveryAccepted, DeliveryUnknown:
		return value.QuotaAction == "consume"
	case DeliveryBlocked, DeliverySkipped, DeliveryDeadletter:
		return value.QuotaAction == "none" || value.QuotaAction == "release" || value.QuotaAction == "invalidate"
	default:
		return false
	}
}
