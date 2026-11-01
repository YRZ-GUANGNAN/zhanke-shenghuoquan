package notification

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
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

// NewGormRepository is kept as an explicit constructor for composition roots
// that name adapters by their persistence implementation.
func NewGormRepository(db *gorm.DB) *GormRepository { return NewRepository(db) }

var _ Repository = (*GormRepository)(nil)

func (r *GormRepository) AppendEventTx(tx *gorm.DB, event Event) error {
	if tx == nil || !validID(event.EventPublicID) || !safeText(event.EventType, 64, true) || !safeText(event.AggregateType, 32, true) || !safeText(event.AggregatePublicID, 128, true) || event.AggregateVersion == 0 || event.PayloadVersion == 0 || !validMetadata(event.Payload) || event.OccurredAt.IsZero() || event.CreatedAt.IsZero() || event.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	if event.Status == "" {
		event.Status = EventPending
	}
	if event.RowVersion == 0 {
		event.RowVersion = 1
	}
	if event.Status != EventPending || event.RowVersion != 1 || event.AttemptCount != 0 || event.LeaseToken != nil || event.LeaseExpiresAt != nil {
		return ErrInvalidRecord
	}
	err := tx.Create(&event).Error
	if !duplicateKey(err) {
		return err
	}
	var existing Event
	if err := tx.Where("aggregate_type = ? AND aggregate_public_id = ? AND aggregate_version = ? AND event_type = ?", event.AggregateType, event.AggregatePublicID, event.AggregateVersion, event.EventType).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrIdempotencyConflict
		}
		return err
	}
	if existing.PayloadVersion != event.PayloadVersion || !sameJSON(existing.Payload, event.Payload) {
		return ErrIdempotencyConflict
	}
	return nil
}

func (r *GormRepository) EnsureInboxTx(tx *gorm.DB, item InboxItem) error {
	if tx == nil || !validID(item.InboxPublicID) || !validID(item.EventPublicID) || item.RecipientUserID == 0 || !safeText(string(item.Purpose), 48, true) || !safeText(item.Title, 128, true) || !safeText(item.Summary, 256, false) || !safeText(item.TargetPath, 256, false) || !safeText(item.AggregateType, 32, true) || !safeText(item.AggregatePublicID, 128, true) || item.AggregateVersion == 0 || item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	if item.Status == "" {
		item.Status = DeliveryPending
	}
	if item.RowVersion == 0 {
		item.RowVersion = 1
	}
	if item.Status != DeliveryPending || item.RowVersion != 1 || item.ReadAt != nil || item.DeliveredAt != nil {
		return ErrInvalidRecord
	}
	err := tx.Create(&item).Error
	if !duplicateKey(err) {
		return err
	}
	var existing InboxItem
	if err := tx.Where("event_public_id = ? AND recipient_user_id = ? AND purpose = ?", item.EventPublicID, item.RecipientUserID, item.Purpose).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrIdempotencyConflict
		}
		return err
	}
	if existing.Title != item.Title || existing.Summary != item.Summary || existing.AggregateType != item.AggregateType || existing.AggregatePublicID != item.AggregatePublicID || existing.AggregateVersion != item.AggregateVersion || existing.TargetPath != item.TargetPath {
		return ErrIdempotencyConflict
	}
	return nil
}

func (r *GormRepository) EnsureDeliveryTx(tx *gorm.DB, item DeliveryItem) error {
	if tx == nil || !validID(item.DeliveryPublicID) || !validID(item.EventPublicID) || item.RecipientUserID == 0 || !validChannel(item.Channel) || !safeText(string(item.Purpose), 48, true) || !safeText(item.TemplateKey, 64, false) || !validMetadata(item.TemplateSnapshot) || item.PayloadVersion == 0 || item.CreatedAt.IsZero() || item.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	if item.Status == "" {
		item.Status = DeliveryPending
	}
	if item.RowVersion == 0 {
		item.RowVersion = 1
	}
	if item.Status != DeliveryPending || item.RowVersion != 1 || item.AttemptCount != 0 || item.LeaseToken != nil || item.LeaseExpiresAt != nil || item.SubscriptionID != nil || item.SubscriptionNonce != nil {
		return ErrInvalidRecord
	}
	err := tx.Create(&item).Error
	if !duplicateKey(err) {
		return err
	}
	var existing DeliveryItem
	if err := tx.Where("event_public_id = ? AND recipient_user_id = ? AND channel = ? AND purpose = ?", item.EventPublicID, item.RecipientUserID, item.Channel, item.Purpose).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrIdempotencyConflict
		}
		return err
	}
	if existing.TemplateKey != item.TemplateKey || existing.PayloadVersion != item.PayloadVersion || !sameJSON(existing.TemplateSnapshot, item.TemplateSnapshot) {
		return ErrIdempotencyConflict
	}
	return nil
}

func (r *GormRepository) ClaimNextDelivery(ctx context.Context, now time.Time, lease time.Duration) (DeliveryLease, bool, error) {
	return r.claimDelivery(ctx, now, lease, "")
}

func (r *GormRepository) ClaimWechatDelivery(ctx context.Context, now time.Time, lease time.Duration) (DeliveryLease, bool, error) {
	return r.claimDelivery(ctx, now, lease, ChannelWechatSubscribe)
}

func (r *GormRepository) claimDelivery(ctx context.Context, now time.Time, lease time.Duration, channel Channel) (DeliveryLease, bool, error) {
	if now.IsZero() || lease <= 0 {
		return DeliveryLease{}, false, ErrInvalidRecord
	}
	var result DeliveryLease
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// An expired external lease may already have sent its request. Preserve
		// that uncertainty instead of automatically sending a second message.
		if err := expireExternalDeliveriesTx(tx, now); err != nil {
			return err
		}
		var item DeliveryItem
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		if channel != "" {
			query = query.Where("channel = ?", channel)
		}
		err := query.
			Where("(status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)) OR (channel = ? AND status = ? AND lease_expires_at <= ?)", []DeliveryStatus{DeliveryPending, DeliveryRetryWait}, now, ChannelInApp, DeliveryProcessing, now).
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
	if userID == 0 || !safeText(appID, 64, true) || !safeText(templateKey, 64, true) || !safeText(nonce, 64, true) || !safeIdentifier.MatchString(nonce) || now.IsZero() {
		return SubscriptionReservation{}, ErrInvalidRecord
	}
	var result SubscriptionReservation
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		if !retryableReservation(err) {
			break
		}
	}
	return result, err
}

func (r *GormRepository) RecordCallbackOnce(ctx context.Context, receipt CallbackReceipt) (bool, error) {
	if !validCallbackReceipt(receipt) {
		return false, ErrInvalidRecord
	}
	if receipt.Status == "" {
		receipt.Status = "received"
	}
	if receipt.Status != "received" {
		return false, ErrInvalidRecord
	}
	err := r.db.WithContext(ctx).Create(&receipt).Error
	if duplicateKey(err) {
		return false, nil
	}
	return err == nil, err
}

// RecordCallbackAndEvent atomically records the provider receipt and the
// metadata-only notification event. The duplicate receipt path is treated as
// an idempotent success; it intentionally does not attempt to append the event
// again, because the first transaction either committed both rows or rolled
// back both rows.
func (r *GormRepository) RecordCallbackAndEvent(ctx context.Context, receipt CallbackReceipt, event Event) (bool, error) {
	if r == nil || r.db == nil || !validCallbackReceipt(receipt) || receipt.ConversationType != "c2c" || !validPrivateMessageEventCoordinate(receipt, event) {
		return false, ErrInvalidRecord
	}
	inserted := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if receipt.Status == "" {
			receipt.Status = "received"
		}
		if receipt.Status != "received" {
			return ErrInvalidRecord
		}
		result := tx.Create(&receipt)
		if duplicateKey(result.Error) {
			return nil
		}
		if result.Error != nil {
			return result.Error
		}
		if err := r.AppendEventTx(tx, event); err != nil {
			return err
		}
		inserted = true
		return nil
	})
	return inserted && err == nil, err
}

func validCallbackReceipt(receipt CallbackReceipt) bool {
	return receipt.SDKAppID > 0 && safeText(receipt.ProviderMessageID, 128, true) &&
		receipt.SenderUserID > 0 && receipt.RecipientUserID > 0 && receipt.SenderUserID != receipt.RecipientUserID &&
		safeText(receipt.SenderAccount, 64, true) && safeText(receipt.RecipientAccount, 64, true) &&
		receipt.SenderAccount != receipt.RecipientAccount &&
		callbackHashPattern.MatchString(receipt.CallbackHash) && safeText(receipt.ConversationPublicID, 128, false) &&
		(receipt.ConversationType == "c2c" || receipt.ConversationType == "group") && !receipt.OccurredAt.IsZero() && !receipt.CreatedAt.IsZero() &&
		(receipt.Status == "" || receipt.Status == "received")
}

func validPrivateMessageEventCoordinate(receipt CallbackReceipt, event Event) bool {
	if !validID(event.EventPublicID) || event.EventType != EventPrivateMessageReceived ||
		event.AggregateType != "im_conversation" || event.AggregatePublicID == "" || event.AggregatePublicID != receipt.ConversationPublicID ||
		event.AggregateVersion != privateMessageVersion(IMCallback{SDKAppID: receipt.SDKAppID, ProviderMessageID: receipt.ProviderMessageID, SenderAccount: receipt.SenderAccount, RecipientAccount: receipt.RecipientAccount}) ||
		event.PayloadVersion != 1 || !event.OccurredAt.Equal(receipt.OccurredAt) || event.CreatedAt.IsZero() || event.UpdatedAt.IsZero() || !validMetadata(event.Payload) {
		return false
	}
	var payload struct {
		SenderUserID      uint64 `json:"senderUserId"`
		RecipientUserID   uint64 `json:"recipientUserId"`
		ProviderMessageID string `json:"providerMessageId"`
		TargetPath        string `json:"targetPath"`
	}
	if json.Unmarshal(event.Payload, &payload) != nil || payload.SenderUserID != receipt.SenderUserID || payload.RecipientUserID != receipt.RecipientUserID || payload.ProviderMessageID != receipt.ProviderMessageID || !validTargetPath(payload.TargetPath) {
		return false
	}
	return true
}

func duplicateKey(err error) bool {
	var value *mysqlDriver.MySQLError
	return errors.Is(err, gorm.ErrDuplicatedKey) || (errors.As(err, &value) && value.Number == 1062)
}
func retryableReservation(err error) bool {
	var value *mysqlDriver.MySQLError
	return duplicateKey(err) || (errors.As(err, &value) && (value.Number == 1213 || value.Number == 1205))
}
func sameJSON(left, right json.RawMessage) bool {
	var l, r any
	ld, rd := json.NewDecoder(strings.NewReader(string(left))), json.NewDecoder(strings.NewReader(string(right)))
	ld.UseNumber()
	rd.UseNumber()
	return ld.Decode(&l) == nil && rd.Decode(&r) == nil && reflect.DeepEqual(l, r)
}

func validChannel(value Channel) bool {
	return value == ChannelInApp || value == ChannelWechatSubscribe || value == ChannelWecom
}
func terminalDeliveryStatus(value DeliveryStatus) bool {
	return value == DeliveryAccepted || value == DeliveryDelivered || value == DeliveryRead || value == DeliveryUnknown || value == DeliveryBlocked || value == DeliverySkipped || value == DeliveryDeadletter
}
