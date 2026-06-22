package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MediaConsumerRepository interface {
	ClaimNextConsumerEvent(context.Context, time.Time, time.Duration) (ConsumerDelivery, bool, error)
	ClassifyConsumerOrder(context.Context, ConsumerDelivery) (ConsumerOrder, error)
	CompleteConsumerEvent(context.Context, ConsumerDelivery, time.Time) error
	FailConsumerEvent(context.Context, ConsumerDelivery, string, time.Time, uint32, bool, time.Time) (ConsumerStatus, error)
}

func (r *GormRepository) ClaimNextConsumerEvent(ctx context.Context, now time.Time, leaseTTL time.Duration) (ConsumerDelivery, bool, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		delivery, found, err := r.claimNextConsumerEventOnce(ctx, now, leaseTTL)
		if err == nil || !consumerTransactionRetryable(err) {
			return delivery, found, err
		}
		lastErr = err
	}
	return ConsumerDelivery{}, false, lastErr
}

func (r *GormRepository) claimNextConsumerEventOnce(ctx context.Context, now time.Time, leaseTTL time.Duration) (ConsumerDelivery, bool, error) {
	leaseToken, err := NewULID()
	if err != nil {
		return ConsumerDelivery{}, false, err
	}
	var result ConsumerDelivery
	found := false
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for scan := 0; scan < 100; scan++ {
			var event OutboxEvent
			err := tx.Table("media_outbox AS o").
				Select("o.*").
				Joins("LEFT JOIN media_event_inbox AS i ON i.event_id = o.event_id").
				Joins("LEFT JOIN media_consumer_checkpoints AS c ON c.aggregate_id = o.aggregate_id").
				Where(`o.processed_at IS NULL AND (
					i.event_id IS NULL OR i.status = ? OR
					(i.status = ? AND (i.next_attempt_at IS NULL OR i.next_attempt_at <= ?)) OR
					(i.status = ? AND (i.lease_expires_at IS NULL OR i.lease_expires_at <= ?)) OR
					i.status IN (?, ?)
				) AND (
					c.aggregate_id IS NULL OR c.active_event_id IS NULL OR c.active_event_id = o.event_id OR
					c.lease_expires_at IS NULL OR c.lease_expires_at <= ?
				)`, ConsumerPending, ConsumerRetryWait, now, ConsumerProcessing, now, ConsumerSucceeded, ConsumerDeadletter, now).
				Order("o.created_at ASC, o.event_id ASC").
				Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
				Take(&event).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return err
			}

			inbox, inboxFound, err := loadConsumerInbox(tx, event.EventID, true)
			if err != nil {
				return err
			}
			if !inboxFound {
				inbox = ConsumerInbox{
					EventID: event.EventID, AggregateID: event.AggregateID, AggregateVersion: event.AggregateVersion,
					Operation: event.Operation, PayloadHash: hashConsumerPayload(event.Payload), Status: ConsumerPending,
					RowVersion: 1, CreatedAt: now, UpdatedAt: now,
				}
				if err := tx.Create(&inbox).Error; err != nil {
					return err
				}
			}
			if inbox.Status == ConsumerSucceeded || inbox.Status == ConsumerDeadletter {
				if err := markConsumerOutboxProcessed(tx, event, now); err != nil {
					return err
				}
				continue
			}
			claimed, err := claimConsumerAggregate(tx, event, leaseToken, now, leaseTTL)
			if err != nil {
				return err
			}
			if !claimed {
				return nil
			}
			from := inbox.Status
			if from != ConsumerPending && from != ConsumerRetryWait && from != ConsumerProcessing {
				return ErrInvalidTransition
			}
			query := tx.Model(&ConsumerInbox{}).
				Where("event_id = ? AND status = ? AND row_version = ?", inbox.EventID, from, inbox.RowVersion)
			if from == ConsumerRetryWait {
				query = query.Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now)
			}
			if from == ConsumerProcessing {
				query = query.Where("lease_expires_at IS NULL OR lease_expires_at <= ?", now)
			}
			if err := casUpdate(query.Updates(map[string]any{
				"status": ConsumerProcessing, "lease_token": leaseToken, "lease_expires_at": now.Add(leaseTTL),
				"attempt_count": gorm.Expr("attempt_count + 1"), "next_attempt_at": nil,
				"last_error_code": "", "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})); err != nil {
				return err
			}
			inbox.Status = ConsumerProcessing
			inbox.LeaseToken = stringPointer(leaseToken)
			inbox.LeaseExpiresAt = timePointer(now.Add(leaseTTL))
			inbox.AttemptCount++
			inbox.NextAttemptAt = nil
			inbox.LastErrorCode = ""
			inbox.RowVersion++
			inbox.UpdatedAt = now
			result = ConsumerDelivery{Event: event, Inbox: inbox, LeaseToken: leaseToken}
			found = true
			return nil
		}
		return nil
	})
	return result, found, err
}

func consumerTransactionRetryable(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && (mysqlError.Number == 1205 || mysqlError.Number == 1213)
}

func (r *GormRepository) ClassifyConsumerOrder(ctx context.Context, delivery ConsumerDelivery) (ConsumerOrder, error) {
	if delivery.Inbox.PayloadHash != hashConsumerPayload(delivery.Event.Payload) ||
		delivery.Inbox.AggregateID != delivery.Event.AggregateID || delivery.Inbox.AggregateVersion != delivery.Event.AggregateVersion ||
		delivery.Inbox.Operation != delivery.Event.Operation {
		return ConsumerOrderConflict, nil
	}
	var checkpoint ConsumerCheckpoint
	err := r.db.WithContext(ctx).Where("aggregate_id = ?", delivery.Event.AggregateID).First(&checkpoint).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if delivery.Event.AggregateVersion == 1 {
			return ConsumerOrderReady, nil
		}
		return ConsumerOrderOutOfOrder, nil
	}
	if err != nil {
		return "", err
	}
	switch {
	case delivery.Event.AggregateVersion == checkpoint.LastAggregateVersion+1:
		return ConsumerOrderReady, nil
	case delivery.Event.AggregateVersion == checkpoint.LastAggregateVersion &&
		delivery.Event.Operation == checkpoint.LastOperation && delivery.Inbox.PayloadHash == checkpoint.LastPayloadHash:
		return ConsumerOrderDuplicate, nil
	case delivery.Event.AggregateVersion > checkpoint.LastAggregateVersion+1:
		return ConsumerOrderOutOfOrder, nil
	default:
		return ConsumerOrderConflict, nil
	}
}

func (r *GormRepository) CompleteConsumerEvent(ctx context.Context, delivery ConsumerDelivery, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		inbox, found, err := loadConsumerInbox(tx, delivery.Event.EventID, true)
		if err != nil || !found {
			if err != nil {
				return err
			}
			return ErrNotFound
		}
		if !consumerLeaseMatches(inbox, delivery, now) {
			return ErrConsumerLease
		}

		checkpoint, checkpointFound, err := loadConsumerCheckpoint(tx, delivery.Event.AggregateID, true)
		if err != nil {
			return err
		}
		if !checkpointFound || !consumerCheckpointLeaseMatches(checkpoint, delivery, now) {
			return ErrConsumerLease
		}
		next := delivery.Event.AggregateVersion == checkpoint.LastAggregateVersion+1
		duplicate := delivery.Event.AggregateVersion == checkpoint.LastAggregateVersion &&
			delivery.Event.Operation == checkpoint.LastOperation && inbox.PayloadHash == checkpoint.LastPayloadHash
		if !next && !duplicate {
			return ErrConsumerOutOfOrder
		}
		checkpointUpdates := map[string]any{
			"active_event_id": nil, "lease_token": nil, "lease_expires_at": nil,
			"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		}
		if next {
			checkpointUpdates["last_event_id"] = delivery.Event.EventID
			checkpointUpdates["last_aggregate_version"] = delivery.Event.AggregateVersion
			checkpointUpdates["last_operation"] = delivery.Event.Operation
			checkpointUpdates["last_payload_hash"] = inbox.PayloadHash
		}
		if err := casUpdate(tx.Model(&ConsumerCheckpoint{}).
			Where("aggregate_id = ? AND row_version = ? AND active_event_id = ? AND lease_token = ?", checkpoint.AggregateID, checkpoint.RowVersion, delivery.Event.EventID, delivery.LeaseToken).
			Updates(checkpointUpdates)); err != nil {
			return err
		}

		if err := casUpdate(tx.Model(&ConsumerInbox{}).
			Where("event_id = ? AND status = ? AND row_version = ? AND lease_token = ?", inbox.EventID, ConsumerProcessing, delivery.Inbox.RowVersion, delivery.LeaseToken).
			Updates(map[string]any{
				"status": ConsumerSucceeded, "lease_token": nil, "lease_expires_at": nil,
				"processed_at": now, "last_error_code": "", "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})); err != nil {
			return err
		}
		return markConsumerOutboxProcessed(tx, delivery.Event, now)
	})
}

func (r *GormRepository) FailConsumerEvent(ctx context.Context, delivery ConsumerDelivery, errorCode string, retryAt time.Time, maxAttempts uint32, permanent bool, now time.Time) (ConsumerStatus, error) {
	target := ConsumerRetryWait
	if permanent || delivery.Inbox.AttemptCount >= maxAttempts {
		target = ConsumerDeadletter
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		inbox, found, err := loadConsumerInbox(tx, delivery.Event.EventID, true)
		if err != nil || !found {
			if err != nil {
				return err
			}
			return ErrNotFound
		}
		if !consumerLeaseMatches(inbox, delivery, now) {
			return ErrConsumerLease
		}
		checkpoint, checkpointFound, err := loadConsumerCheckpoint(tx, delivery.Event.AggregateID, true)
		if err != nil {
			return err
		}
		if !checkpointFound || !consumerCheckpointLeaseMatches(checkpoint, delivery, now) {
			return ErrConsumerLease
		}
		if err := casUpdate(tx.Model(&ConsumerCheckpoint{}).
			Where("aggregate_id = ? AND row_version = ? AND active_event_id = ? AND lease_token = ?", checkpoint.AggregateID, checkpoint.RowVersion, delivery.Event.EventID, delivery.LeaseToken).
			Updates(map[string]any{
				"active_event_id": nil, "lease_token": nil, "lease_expires_at": nil,
				"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})); err != nil {
			return err
		}
		updates := map[string]any{
			"status": target, "lease_token": nil, "lease_expires_at": nil,
			"last_error_code": errorCode, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		}
		if target == ConsumerRetryWait {
			updates["next_attempt_at"] = retryAt
		} else {
			updates["next_attempt_at"] = nil
			updates["processed_at"] = now
		}
		if err := casUpdate(tx.Model(&ConsumerInbox{}).
			Where("event_id = ? AND status = ? AND row_version = ? AND lease_token = ?", inbox.EventID, ConsumerProcessing, delivery.Inbox.RowVersion, delivery.LeaseToken).
			Updates(updates)); err != nil {
			return err
		}
		if target == ConsumerDeadletter {
			return markConsumerOutboxProcessed(tx, delivery.Event, now)
		}
		return nil
	})
	return target, err
}

func loadConsumerInbox(tx *gorm.DB, eventID string, lock bool) (ConsumerInbox, bool, error) {
	query := tx
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var value ConsumerInbox
	err := query.Where("event_id = ?", eventID).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ConsumerInbox{}, false, nil
	}
	return value, err == nil, err
}

func loadConsumerCheckpoint(tx *gorm.DB, aggregateID string, lock bool) (ConsumerCheckpoint, bool, error) {
	query := tx
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var value ConsumerCheckpoint
	err := query.Where("aggregate_id = ?", aggregateID).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ConsumerCheckpoint{}, false, nil
	}
	return value, err == nil, err
}

func claimConsumerAggregate(tx *gorm.DB, event OutboxEvent, leaseToken string, now time.Time, leaseTTL time.Duration) (bool, error) {
	checkpoint, found, err := loadConsumerCheckpoint(tx, event.AggregateID, true)
	if err != nil {
		return false, err
	}
	if !found {
		checkpoint = ConsumerCheckpoint{
			AggregateID: event.AggregateID, ActiveEventID: stringPointer(event.EventID),
			LeaseToken: stringPointer(leaseToken), LeaseExpiresAt: timePointer(now.Add(leaseTTL)),
			RowVersion: 1, CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&checkpoint).Error; err != nil {
			return false, err
		}
		return true, nil
	}
	if checkpoint.ActiveEventID != nil && *checkpoint.ActiveEventID != event.EventID &&
		checkpoint.LeaseExpiresAt != nil && checkpoint.LeaseExpiresAt.After(now) {
		return false, nil
	}
	query := tx.Model(&ConsumerCheckpoint{}).
		Where("aggregate_id = ? AND row_version = ?", checkpoint.AggregateID, checkpoint.RowVersion).
		Where("active_event_id IS NULL OR active_event_id = ? OR lease_expires_at IS NULL OR lease_expires_at <= ?", event.EventID, now)
	if err := casUpdate(query.Updates(map[string]any{
		"active_event_id": event.EventID, "lease_token": leaseToken, "lease_expires_at": now.Add(leaseTTL),
		"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
	})); err != nil {
		return false, err
	}
	return true, nil
}

func markConsumerOutboxProcessed(tx *gorm.DB, event OutboxEvent, now time.Time) error {
	return casUpdate(tx.Model(&OutboxEvent{}).
		Where("event_id = ? AND row_version = ? AND processed_at IS NULL", event.EventID, event.RowVersion).
		Updates(map[string]any{
			"processed_at": now, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		}))
}

func consumerLeaseMatches(inbox ConsumerInbox, delivery ConsumerDelivery, now time.Time) bool {
	return inbox.Status == ConsumerProcessing && inbox.RowVersion == delivery.Inbox.RowVersion &&
		inbox.LeaseToken != nil && *inbox.LeaseToken == delivery.LeaseToken &&
		inbox.LeaseExpiresAt != nil && inbox.LeaseExpiresAt.After(now)
}

func consumerCheckpointLeaseMatches(checkpoint ConsumerCheckpoint, delivery ConsumerDelivery, now time.Time) bool {
	return checkpoint.ActiveEventID != nil && *checkpoint.ActiveEventID == delivery.Event.EventID &&
		checkpoint.LeaseToken != nil && *checkpoint.LeaseToken == delivery.LeaseToken &&
		checkpoint.LeaseExpiresAt != nil && checkpoint.LeaseExpiresAt.After(now)
}

func hashConsumerPayload(payload JSONDocument) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

var _ MediaConsumerRepository = (*GormRepository)(nil)
