package notification

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EventLease is the only mutable handle an expander may use. Completion and
// failure both compare the event row version and lease token in one update.
type EventLease struct {
	Event      Event
	LeaseToken string
}

type EventStore interface {
	ClaimNextEvent(context.Context, time.Time, time.Duration) (EventLease, bool, error)
	ExpandAndComplete(context.Context, EventLease, []InboxItem, []DeliveryItem, EventStatus, time.Time) error
	FailEvent(context.Context, EventLease, string, string, time.Time, uint32, time.Time) (EventStatus, error)
}

type GormEventRepository struct {
	db            *gorm.DB
	notifications *GormRepository
}

func NewEventRepository(db *gorm.DB) *GormEventRepository {
	return &GormEventRepository{db: db, notifications: NewRepository(db)}
}

var _ EventStore = (*GormEventRepository)(nil)

func (r *GormEventRepository) ClaimNextEvent(ctx context.Context, now time.Time, lease time.Duration) (EventLease, bool, error) {
	if r == nil || r.db == nil || now.IsZero() || lease <= 0 {
		return EventLease{}, false, ErrInvalidRecord
	}
	var result EventLease
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var event Event
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("(status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)) OR (status = ? AND lease_expires_at <= ?)", []EventStatus{EventPending, EventRetryWait}, now, EventProcessing, now).
			Order("id ASC").First(&event).Error
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
		update := tx.Model(&Event{}).Where("id = ? AND row_version = ?", event.ID, event.RowVersion).Updates(map[string]any{
			"status": EventProcessing, "attempt_count": gorm.Expr("attempt_count + 1"),
			"lease_token": token, "lease_expires_at": expires, "next_attempt_at": nil,
			"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		event.Status = EventProcessing
		event.AttemptCount++
		event.LeaseToken = &token
		event.LeaseExpiresAt = &expires
		event.NextAttemptAt = nil
		event.RowVersion++
		event.UpdatedAt = now
		result = EventLease{Event: event, LeaseToken: token}
		return nil
	})
	if err != nil || result.LeaseToken == "" {
		return EventLease{}, false, err
	}
	return result, true, nil
}

func (r *GormEventRepository) ExpandAndComplete(ctx context.Context, lease EventLease, inbox []InboxItem, deliveries []DeliveryItem, status EventStatus, now time.Time) error {
	if r == nil || r.db == nil || lease.Event.ID == 0 || strings.TrimSpace(lease.LeaseToken) == "" || now.IsZero() || (status != EventExpanded && status != EventSkipped) {
		return ErrInvalidRecord
	}
	if !eventLeaseValid(lease, now) {
		return ErrConcurrentUpdate
	}
	if status == EventSkipped && (len(inbox) > 0 || len(deliveries) > 0) {
		return ErrInvalidRecord
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range inbox {
			if item.EventPublicID != lease.Event.EventPublicID || item.AggregateType != lease.Event.AggregateType || item.AggregatePublicID != lease.Event.AggregatePublicID || item.AggregateVersion != lease.Event.AggregateVersion {
				return ErrInvalidRecord
			}
			if err := r.notifications.EnsureInboxTx(tx, item); err != nil {
				return err
			}
			// Delivery and event completion share this transaction. Retried
			// expansion can never move an already-read item back to delivered.
			if err := tx.Model(&InboxItem{}).Where("event_public_id = ? AND recipient_user_id = ? AND purpose = ? AND status = ?", item.EventPublicID, item.RecipientUserID, item.Purpose, DeliveryPending).Updates(inboxDeliveredUpdates(now)).Error; err != nil {
				return err
			}
		}
		for _, item := range deliveries {
			if item.EventPublicID != lease.Event.EventPublicID {
				return ErrInvalidRecord
			}
			if err := r.notifications.EnsureDeliveryTx(tx, item); err != nil {
				return err
			}
			if item.Channel == ChannelInApp {
				if err := tx.Model(&DeliveryItem{}).Where("event_public_id = ? AND recipient_user_id = ? AND channel = ? AND purpose = ? AND status = ?", item.EventPublicID, item.RecipientUserID, ChannelInApp, item.Purpose, DeliveryPending).Updates(inAppDeliveredUpdates(now)).Error; err != nil {
					return err
				}
			}
		}
		updates := map[string]any{
			"status": status, "lease_token": nil, "lease_expires_at": nil,
			"processed_at": now, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		}
		query := tx.Model(&Event{}).Where("id = ? AND status = ? AND row_version = ? AND lease_token = ? AND lease_expires_at > ? AND lease_expires_at > UTC_TIMESTAMP(3)", lease.Event.ID, EventProcessing, lease.Event.RowVersion, lease.LeaseToken, now)
		if status == EventExpanded {
			query = query.Where("expires_at IS NULL OR expires_at > UTC_TIMESTAMP(3)")
		}
		result := query.Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		return nil
	})
}

func (r *GormEventRepository) FailEvent(ctx context.Context, lease EventLease, code, safeMessage string, next time.Time, max uint32, now time.Time) (EventStatus, error) {
	if r == nil || r.db == nil || lease.Event.ID == 0 || strings.TrimSpace(lease.LeaseToken) == "" || strings.TrimSpace(code) == "" || now.IsZero() || max == 0 {
		return "", ErrInvalidRecord
	}
	if !eventLeaseValid(lease, now) {
		return "", ErrConcurrentUpdate
	}
	status, updates, err := eventFailureUpdates(lease.Event.AttemptCount, code, next, max, now)
	if err != nil {
		return "", err
	}
	// Event schema intentionally stores only the fixed error code. Never persist
	// the resolver/driver message, which can contain private provider information.
	_ = safeMessage
	result := r.db.WithContext(ctx).Model(&Event{}).Where("id = ? AND status = ? AND row_version = ? AND lease_token = ? AND lease_expires_at > ? AND lease_expires_at > UTC_TIMESTAMP(3)", lease.Event.ID, EventProcessing, lease.Event.RowVersion, lease.LeaseToken, now).Updates(updates)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected != 1 {
		return "", ErrConcurrentUpdate
	}
	return status, nil
}

func eventLeaseValid(lease EventLease, now time.Time) bool {
	return lease.Event.ID != 0 && lease.Event.RowVersion != 0 && lease.Event.Status == EventProcessing && lease.LeaseToken != "" && lease.Event.LeaseToken != nil && *lease.Event.LeaseToken == lease.LeaseToken && lease.Event.LeaseExpiresAt != nil && lease.Event.LeaseExpiresAt.After(now)
}

func inboxDeliveredUpdates(now time.Time) map[string]any {
	return map[string]any{"status": DeliveryDelivered, "delivered_at": now, "updated_at": now, "row_version": gorm.Expr("row_version + 1")}
}

func inAppDeliveredUpdates(now time.Time) map[string]any {
	return map[string]any{"status": DeliveryDelivered, "processed_at": now, "updated_at": now, "row_version": gorm.Expr("row_version + 1")}
}

func eventFailureUpdates(attempt uint32, code string, next time.Time, max uint32, now time.Time) (EventStatus, map[string]any, error) {
	if max == 0 || !safeText(code, 64, true) || !safeIdentifier.MatchString(code) || now.IsZero() {
		return "", nil, ErrInvalidRecord
	}
	status := EventRetryWait
	if attempt >= max {
		status = EventDeadletter
	}
	updates := map[string]any{"status": status, "lease_token": nil, "lease_expires_at": nil, "last_error_code": code, "row_version": gorm.Expr("row_version + 1"), "updated_at": now}
	if status == EventRetryWait {
		if !next.After(now) {
			return "", nil, ErrInvalidRecord
		}
		updates["next_attempt_at"] = next
	} else {
		updates["next_attempt_at"] = nil
		updates["processed_at"] = now
	}
	return status, updates, nil
}
