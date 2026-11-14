package delivery

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func validateAdminAction(actorUserID, expectedRowVersion uint64, reason string) error {
	if actorUserID == 0 || expectedRowVersion == 0 {
		return ErrInvalidRecord
	}
	if reason = strings.TrimSpace(reason); reason == "" || len([]rune(reason)) > 255 {
		return ErrInvalidRecord
	}
	return nil
}

func (r *GormRepository) ReassignRider(
	ctx context.Context,
	publicID string,
	expectedRowVersion, newRiderUserID, actorUserID uint64,
	reason string,
	now time.Time,
) (Order, error) {
	if newRiderUserID == 0 {
		return Order{}, ErrRiderRequired
	}
	if err := validateAdminAction(actorUserID, expectedRowVersion, reason); err != nil {
		return Order{}, err
	}
	reason = strings.TrimSpace(reason)
	var result Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("delivery_order_public_id = ?", publicID).
			First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.RowVersion != expectedRowVersion {
			return ErrConcurrentUpdate
		}
		if _, ok := adminReassignableStatuses[result.Status]; !ok {
			return ErrInvalidTransition
		}
		if result.RiderUserID == nil || *result.RiderUserID == 0 {
			return ErrRiderRequired
		}
		if *result.RiderUserID == newRiderUserID {
			return nil // already assigned to the requested rider; idempotent success.
		}
		var active int64
		if err := tx.Table("takeout_riders").
			Where("rider_user_id = ? AND status = ?", newRiderUserID, "active").
			Count(&active).Error; err != nil {
			return err
		}
		if active == 0 {
			return ErrRiderUnavailable
		}
		fromRider := *result.RiderUserID
		action := AdminOrderAction{
			DeliveryOrderPublicID: result.DeliveryOrderPublicID,
			Action:                AdminActionReassign,
			ActorUserID:           actorUserID,
			FromRiderUserID:       &fromRider,
			ToRiderUserID:         &newRiderUserID,
			FromStatus:            result.Status,
			Reason:                reason,
			CreatedAt:             now.UTC(),
		}
		if err := tx.Create(&action).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"rider_user_id": newRiderUserID,
			"row_version":   gorm.Expr("row_version + 1"),
			"updated_at":    now.UTC(),
		}
		if err := tx.Model(&Order{}).Where("id = ?", result.ID).Updates(updates).Error; err != nil {
			return err
		}
		claimed := newRiderUserID
		result.RiderUserID = &claimed
		result.RowVersion++
		result.UpdatedAt = now.UTC()
		return nil
	})
	if err != nil {
		return Order{}, err
	}
	return result, nil
}

func (r *GormRepository) CancelWithReason(
	ctx context.Context,
	publicID string,
	expectedRowVersion, actorUserID uint64,
	reason string,
	now time.Time,
) (Order, error) {
	if err := validateAdminAction(actorUserID, expectedRowVersion, reason); err != nil {
		return Order{}, err
	}
	reason = strings.TrimSpace(reason)
	var result Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("delivery_order_public_id = ?", publicID).
			First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.RowVersion != expectedRowVersion {
			return ErrConcurrentUpdate
		}
		if _, ok := adminCancellableStatuses[result.Status]; !ok {
			return ErrInvalidTransition
		}
		action := AdminOrderAction{
			DeliveryOrderPublicID: result.DeliveryOrderPublicID,
			Action:                AdminActionCancel,
			ActorUserID:           actorUserID,
			FromRiderUserID:       result.RiderUserID,
			FromStatus:            result.Status,
			Reason:                reason,
			CreatedAt:             now.UTC(),
		}
		if err := tx.Create(&action).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"status":        StatusCancelled,
			"cancel_reason": reason,
			"cancelled_at":  now.UTC(),
			"row_version":   gorm.Expr("row_version + 1"),
			"updated_at":    now.UTC(),
		}
		if err := tx.Model(&Order{}).Where("id = ?", result.ID).Updates(updates).Error; err != nil {
			return err
		}
		result.Status = StatusCancelled
		result.CancelReason = reason
		result.CancelledAt = &now
		result.RowVersion++
		result.UpdatedAt = now.UTC()
		return nil
	})
	if err != nil {
		return Order{}, err
	}
	return result, nil
}
