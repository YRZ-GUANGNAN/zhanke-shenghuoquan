package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DispatchRepository interface {
	CreateIdempotent(context.Context, *Order) (Order, error)
	FindOwnedByMerchant(context.Context, string, uint64) (Order, error)
	FindAssignedToRider(context.Context, string, uint64) (Order, error)
	ListByMerchantUserID(context.Context, uint64, Page) ([]Order, int64, error)
	ListClaimable(context.Context, Page) ([]Order, int64, error)
	ListByRiderUserID(context.Context, uint64, Page) ([]Order, int64, error)
	TransitionToReadyAndEnqueue(context.Context, string, *uint64, uint64, Status, *uint64, time.Time) (Order, OutboxEvent, error)

	ClaimNextOutbox(context.Context, time.Time, time.Duration) (OutboxDelivery, bool, error)
	CompleteOutbox(context.Context, OutboxDelivery, OutboxStatus, time.Time) error
	FailOutbox(context.Context, OutboxDelivery, string, string, time.Time, uint32, time.Time) (OutboxStatus, error)
}

var _ DispatchRepository = (*GormRepository)(nil)

func (r *GormRepository) CreateIdempotent(ctx context.Context, value *Order) (Order, error) {
	if value.MerchantRequestID == nil || !ValidULID(*value.MerchantRequestID) {
		return Order{}, ErrInvalidRecord
	}
	var result Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(value).Error; err == nil {
			result = *value
			return nil
		} else if !isDuplicateKey(err) {
			return err
		}
		if err := tx.Where("merchant_user_id = ? AND merchant_request_id = ?",
			value.MerchantUserID, *value.MerchantRequestID).First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if !sameCreateSnapshot(result, *value) {
			return ErrIdempotencyConflict
		}
		return nil
	})
	return result, err
}

func (r *GormRepository) FindOwnedByMerchant(ctx context.Context, publicID string, merchantUserID uint64) (Order, error) {
	var value Order
	err := r.db.WithContext(ctx).
		Where("delivery_order_public_id = ? AND merchant_user_id = ?", publicID, merchantUserID).
		First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Order{}, ErrForbiddenOrder
	}
	return value, err
}

func (r *GormRepository) FindAssignedToRider(ctx context.Context, publicID string, riderUserID uint64) (Order, error) {
	var value Order
	err := r.db.WithContext(ctx).
		Where("delivery_order_public_id = ? AND rider_user_id = ?", publicID, riderUserID).
		First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Order{}, ErrForbiddenOrder
	}
	return value, err
}

func (r *GormRepository) ListByMerchantUserID(ctx context.Context, merchantUserID uint64, page Page) ([]Order, int64, error) {
	return listOrders(r.db.WithContext(ctx).Where("merchant_user_id = ?", merchantUserID), page)
}

func (r *GormRepository) ListClaimable(ctx context.Context, page Page) ([]Order, int64, error) {
	return listOrders(r.db.WithContext(ctx).Where("status = ? AND rider_user_id IS NULL", StatusReadyForClaim), page)
}

func (r *GormRepository) ListByRiderUserID(ctx context.Context, riderUserID uint64, page Page) ([]Order, int64, error) {
	return listOrders(r.db.WithContext(ctx).Where("rider_user_id = ?", riderUserID), page)
}

func listOrders(query *gorm.DB, page Page) ([]Order, int64, error) {
	if !page.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	var total int64
	if err := query.Model(&Order{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []Order
	err := query.Order("id DESC").Offset((page.Page - 1) * page.Size).Limit(page.Size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) TransitionToReadyAndEnqueue(
	ctx context.Context,
	publicID string,
	merchantUserID *uint64,
	expectedRowVersion uint64,
	from Status,
	riderUserID *uint64,
	updatedAt time.Time,
) (Order, OutboxEvent, error) {
	if !CanTransition(from, StatusReadyForClaim) {
		return Order{}, OutboxEvent{}, ErrInvalidTransition
	}
	var result Order
	var event OutboxEvent
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("delivery_order_public_id = ?", publicID)
		if merchantUserID != nil {
			query = query.Where("merchant_user_id = ?", *merchantUserID)
		}
		if err := query.First(&result).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) && merchantUserID != nil {
				return ErrForbiddenOrder
			}
			return normalizeNotFound(err)
		}
		if result.Status == StatusReadyForClaim {
			return loadOrInsertRiderPoolEvent(tx, result, updatedAt, &event)
		}
		updates, err := buildTransitionUpdates(result, from, StatusReadyForClaim, riderUserID, updatedAt)
		if err != nil {
			return err
		}
		updates["status"] = StatusReadyForClaim
		updates["updated_at"] = updatedAt
		updates["row_version"] = gorm.Expr("row_version + 1")
		update := tx.Model(&Order{}).
			Where("id = ? AND status = ? AND row_version = ?", result.ID, from, expectedRowVersion).
			Updates(updates)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		applyTransition(&result, StatusReadyForClaim, riderUserID, updatedAt)
		result.RowVersion++
		result.UpdatedAt = updatedAt
		return loadOrInsertRiderPoolEvent(tx, result, updatedAt, &event)
	})
	return result, event, err
}

func loadOrInsertRiderPoolEvent(tx *gorm.DB, order Order, now time.Time, result *OutboxEvent) error {
	payload, err := json.Marshal(RiderPoolOpenedPayload{
		OrderPublicID: order.DeliveryOrderPublicID, RouteSummary: "校园配送单",
		DeliveryFeeCents: order.DeliveryFeeCents,
	})
	if err != nil {
		return err
	}
	eventID, err := NewULID()
	if err != nil {
		return err
	}
	nextAttempt := now
	candidate := OutboxEvent{
		EventPublicID: eventID, AggregatePublicID: order.DeliveryOrderPublicID,
		AggregateVersion: order.RowVersion, EventType: EventRiderPoolOpened,
		Payload: payload, Status: OutboxPending, NextAttemptAt: &nextAttempt,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate).Error; err != nil {
		return err
	}
	if err := tx.Where("aggregate_public_id = ? AND aggregate_version = ? AND event_type = ?",
		order.DeliveryOrderPublicID, order.RowVersion, EventRiderPoolOpened).First(result).Error; err != nil {
		return err
	}
	if !sameOutboxPayload(result.Payload, payload) {
		return ErrIdempotencyConflict
	}
	return nil
}

func (r *GormRepository) ClaimNextOutbox(
	ctx context.Context,
	now time.Time,
	leaseTTL time.Duration,
) (OutboxDelivery, bool, error) {
	var result OutboxDelivery
	found := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var event OutboxEvent
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(`(
				(status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?))
				OR (status = ? AND lease_expires_at <= ?)
			)`, []OutboxStatus{OutboxPending, OutboxRetryWait}, now, OutboxProcessing, now).
			Order("id ASC").First(&event).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		leaseToken, err := NewULID()
		if err != nil {
			return err
		}
		leaseExpiresAt := now.Add(leaseTTL)
		update := tx.Model(&OutboxEvent{}).
			Where("id = ? AND row_version = ? AND status = ?", event.ID, event.RowVersion, event.Status).
			Updates(map[string]any{
				"status": OutboxProcessing, "attempt_count": gorm.Expr("attempt_count + 1"),
				"lease_token": leaseToken, "lease_expires_at": leaseExpiresAt,
				"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrOutboxLease
		}
		event.Status = OutboxProcessing
		event.AttemptCount++
		event.LeaseToken = &leaseToken
		event.LeaseExpiresAt = &leaseExpiresAt
		event.RowVersion++
		event.UpdatedAt = now
		result = OutboxDelivery{Event: event, LeaseToken: leaseToken}
		found = true
		return nil
	})
	return result, found, err
}

func (r *GormRepository) CompleteOutbox(
	ctx context.Context,
	delivery OutboxDelivery,
	status OutboxStatus,
	now time.Time,
) error {
	if status != OutboxSucceeded && status != OutboxSkipped {
		return ErrInvalidRecord
	}
	update := r.db.WithContext(ctx).Model(&OutboxEvent{}).
		Where("id = ? AND row_version = ? AND status = ? AND lease_token = ?",
			delivery.Event.ID, delivery.Event.RowVersion, OutboxProcessing, delivery.LeaseToken).
		Updates(map[string]any{
			"status": status, "processed_at": now, "lease_token": nil, "lease_expires_at": nil,
			"last_error_code": "", "last_error": "", "row_version": gorm.Expr("row_version + 1"),
			"updated_at": now,
		})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return ErrOutboxLease
	}
	return nil
}

func (r *GormRepository) FailOutbox(
	ctx context.Context,
	delivery OutboxDelivery,
	code, message string,
	nextAttemptAt time.Time,
	maxAttempts uint32,
	now time.Time,
) (OutboxStatus, error) {
	status := OutboxRetryWait
	processedAt := any(nil)
	if delivery.Event.AttemptCount >= maxAttempts {
		status = OutboxDeadletter
		processedAt = now
	}
	if len(code) > 64 {
		code = code[:64]
	}
	if len(message) > 512 {
		message = message[:512]
	}
	update := r.db.WithContext(ctx).Model(&OutboxEvent{}).
		Where("id = ? AND row_version = ? AND status = ? AND lease_token = ?",
			delivery.Event.ID, delivery.Event.RowVersion, OutboxProcessing, delivery.LeaseToken).
		Updates(map[string]any{
			"status": status, "next_attempt_at": nextAttemptAt, "lease_token": nil,
			"lease_expires_at": nil, "last_error_code": code, "last_error": message,
			"processed_at": processedAt, "row_version": gorm.Expr("row_version + 1"),
			"updated_at": now,
		})
	if update.Error != nil {
		return "", update.Error
	}
	if update.RowsAffected != 1 {
		return "", ErrOutboxLease
	}
	return status, nil
}

func sameCreateSnapshot(left, right Order) bool {
	return left.MerchantUserID == right.MerchantUserID &&
		stringPointerEqual(left.MerchantRequestID, right.MerchantRequestID) &&
		left.PickupContactName == right.PickupContactName &&
		left.PickupContactPhone == right.PickupContactPhone &&
		left.PickupAddress == right.PickupAddress &&
		left.RecipientName == right.RecipientName &&
		left.RecipientPhone == right.RecipientPhone &&
		left.DeliveryAddress == right.DeliveryAddress &&
		left.Remark == right.Remark &&
		left.DeliveryFeeCents == right.DeliveryFeeCents &&
		left.RiderFeeCents == right.RiderFeeCents &&
		left.PlatformCutCents == right.PlatformCutCents
}

func stringPointerEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func sameOutboxPayload(left, right json.RawMessage) bool {
	var leftValue, rightValue RiderPoolOpenedPayload
	return json.Unmarshal(left, &leftValue) == nil &&
		json.Unmarshal(right, &rightValue) == nil &&
		leftValue == rightValue
}

func isDuplicateKey(err error) bool {
	var mysqlError *mysql.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}
