package delivery

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	Create(context.Context, *Order) error
	Find(context.Context, string) (Order, error)
	Transition(context.Context, string, uint64, Status, Status, *uint64, time.Time) (Order, error)
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

var _ Repository = (*GormRepository)(nil)

func (r *GormRepository) Create(ctx context.Context, value *Order) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(value).Error
	})
}

func (r *GormRepository) Find(ctx context.Context, publicID string) (Order, error) {
	var value Order
	err := r.db.WithContext(ctx).
		Where("delivery_order_public_id = ?", publicID).
		First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) Transition(
	ctx context.Context,
	publicID string,
	expectedRowVersion uint64,
	from, to Status,
	riderUserID *uint64,
	updatedAt time.Time,
) (Order, error) {
	if !CanTransition(from, to) {
		return Order{}, ErrInvalidTransition
	}
	var result Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("delivery_order_public_id = ?", publicID).
			First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.Status == to {
			if err := validateIdempotentTransition(result, to, riderUserID); err != nil {
				return err
			}
			if to == StatusDelivered {
				_, _, _, err := settleDeliveredOrderTx(tx, result, updatedAt)
				return err
			}
			return nil
		}
		updates, err := buildTransitionUpdates(result, from, to, riderUserID, updatedAt)
		if err != nil {
			return err
		}
		updates["status"] = to
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
		applyTransition(&result, to, riderUserID, updatedAt)
		result.RowVersion++
		result.UpdatedAt = updatedAt
		if to == StatusDelivered {
			_, _, _, err := settleDeliveredOrderTx(tx, result, updatedAt)
			return err
		}
		return nil
	})
	return result, err
}

func buildTransitionUpdates(current Order, from, to Status, riderUserID *uint64, at time.Time) (map[string]any, error) {
	if current.Status != from {
		return nil, ErrConcurrentUpdate
	}
	updates := make(map[string]any)
	switch to {
	case StatusPendingFeePayment:
	case StatusPaymentFailed:
		updates["payment_failed_at"] = at
	case StatusReadyForClaim:
		if from == StatusClaimed || from == StatusToPickup {
			if err := requireAssignedRider(current, riderUserID); err != nil {
				return nil, err
			}
		}
		updates["rider_user_id"] = nil
		updates["ready_for_claim_at"] = at
	case StatusClaimed:
		if riderUserID == nil || *riderUserID == 0 {
			return nil, ErrRiderRequired
		}
		if current.RiderUserID != nil {
			return nil, ErrRiderMismatch
		}
		updates["rider_user_id"] = *riderUserID
		updates["claimed_at"] = at
	case StatusToPickup:
		if err := requireAssignedRider(current, riderUserID); err != nil {
			return nil, err
		}
		updates["to_pickup_at"] = at
	case StatusPickedUp:
		if err := requireAssignedRider(current, riderUserID); err != nil {
			return nil, err
		}
		updates["picked_up_at"] = at
	case StatusDelivering:
		if err := requireAssignedRider(current, riderUserID); err != nil {
			return nil, err
		}
		updates["delivering_at"] = at
	case StatusDelivered:
		if err := requireAssignedRider(current, riderUserID); err != nil {
			return nil, err
		}
		updates["delivered_at"] = at
	case StatusCancelled:
		updates["cancelled_at"] = at
	default:
		return nil, ErrInvalidTransition
	}
	return updates, nil
}

func requireAssignedRider(current Order, riderUserID *uint64) error {
	if riderUserID == nil || *riderUserID == 0 {
		return ErrRiderRequired
	}
	if current.RiderUserID == nil || *current.RiderUserID != *riderUserID {
		return ErrRiderMismatch
	}
	return nil
}

func validateIdempotentTransition(current Order, to Status, riderUserID *uint64) error {
	switch to {
	case StatusClaimed, StatusToPickup, StatusPickedUp, StatusDelivering, StatusDelivered:
		return requireAssignedRider(current, riderUserID)
	default:
		return nil
	}
}

func applyTransition(value *Order, to Status, riderUserID *uint64, at time.Time) {
	value.Status = to
	switch to {
	case StatusPaymentFailed:
		value.PaymentFailedAt = timePointer(at)
	case StatusReadyForClaim:
		value.RiderUserID = nil
		value.ReadyForClaimAt = timePointer(at)
	case StatusClaimed:
		value.RiderUserID = uint64Pointer(*riderUserID)
		value.ClaimedAt = timePointer(at)
	case StatusToPickup:
		value.ToPickupAt = timePointer(at)
	case StatusPickedUp:
		value.PickedUpAt = timePointer(at)
	case StatusDelivering:
		value.DeliveringAt = timePointer(at)
	case StatusDelivered:
		value.DeliveredAt = timePointer(at)
	case StatusCancelled:
		value.CancelledAt = timePointer(at)
	}
}

func normalizeNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func timePointer(value time.Time) *time.Time { return &value }

func uint64Pointer(value uint64) *uint64 { return &value }
