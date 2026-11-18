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
		// 资格校验与骑手工作台一致：骑手状态 active + 账号未注销 + 身份授权有效。
		var active int64
		if err := tx.Table("takeout_riders").
			Joins("JOIN users u ON u.id = takeout_riders.rider_user_id AND u.status = 'active' AND u.deleted_at IS NULL").
			Joins("JOIN user_identity_grants g ON g.user_id = takeout_riders.rider_user_id AND g.role = 'rider' AND g.status = 'active'").
			Where("takeout_riders.rider_user_id = ? AND takeout_riders.status = ?", newRiderUserID, "active").
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
			ActorAdminID:          &actorUserID,
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
		before := result
		claimed := newRiderUserID
		result.RiderUserID = &claimed
		result.RowVersion++
		result.UpdatedAt = now.UTC()
		if err := appendReassignNotificationTx(tx, r.notificationAppender, before, result, now); err != nil {
			return err
		}
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
		// 防御：只要存在已支付（未退款）的配送费结算，就不能直接取消。
		var paidSettlements int64
		if err := tx.Table("delivery_fee_settlements").
			Where("delivery_order_public_id = ? AND status = ? AND refunded_at IS NULL", result.DeliveryOrderPublicID, "paid").
			Count(&paidSettlements).Error; err != nil {
			return err
		}
		if paidSettlements > 0 {
			return ErrInvalidTransition
		}
		action := AdminOrderAction{
			DeliveryOrderPublicID: result.DeliveryOrderPublicID,
			Action:                AdminActionCancel,
			ActorUserID:           actorUserID,
			ActorAdminID:          &actorUserID,
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
		before := result
		result.Status = StatusCancelled
		result.CancelReason = reason
		result.CancelledAt = &now
		result.RowVersion++
		result.UpdatedAt = now.UTC()
		// 取消同样广播订单通知（商家群/骑手群会收到「配送单已取消」卡片）。
		if err := appendOrderNotificationTx(tx, r.notificationAppender, before, result, now); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return Order{}, err
	}
	return result, nil
}
