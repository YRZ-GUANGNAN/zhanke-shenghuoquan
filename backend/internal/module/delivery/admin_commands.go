package delivery

import (
	"context"
	"strings"
	"time"
)

// Administrator order commands run outside the rider and merchant flows. They
// keep the same CAS contract as the rest of the delivery state machine and
// persist an immutable audit row for every command.
const (
	AdminActionReassign = "reassign"
	AdminActionCancel   = "cancel"
)

var adminReassignableStatuses = map[Status]struct{}{
	StatusClaimed:    {},
	StatusToPickup:   {},
	StatusPickedUp:   {},
	StatusDelivering: {},
}

// 只有未支付配送费的订单可以直接取消。已支付订单必须走退款流程
// （退款在同一事务内完成取消与资金退回），否则配送费会永久滞留。
var adminCancellableStatuses = map[Status]struct{}{
	StatusPendingFeePayment: {},
	StatusPaymentFailed:     {},
}

type AdminOrderAction struct {
	ID                    uint64    `gorm:"column:id;primaryKey"`
	DeliveryOrderPublicID string    `gorm:"column:delivery_order_public_id;size:26;not null"`
	Action                string    `gorm:"column:action;size:24;not null"`
	ActorUserID           uint64    `gorm:"column:actor_user_id;not null"`
	FromRiderUserID       *uint64   `gorm:"column:from_rider_user_id"`
	ToRiderUserID         *uint64   `gorm:"column:to_rider_user_id"`
	FromStatus            Status    `gorm:"column:from_status;size:24;not null"`
	Reason                string    `gorm:"column:reason;size:255;not null"`
	CreatedAt             time.Time `gorm:"column:created_at;not null"`
}

func (AdminOrderAction) TableName() string { return "delivery_order_admin_actions" }

// AdminOrderRepository is implemented by GormRepository and consumed by the
// administrator console through Service.
type AdminOrderRepository interface {
	ReassignRider(context.Context, string, uint64, uint64, uint64, string, time.Time) (Order, error)
	CancelWithReason(context.Context, string, uint64, uint64, string, time.Time) (Order, error)
}

func (s *Service) AdminReassignRider(
	ctx context.Context,
	publicID string,
	expectedRowVersion, newRiderUserID, actorUserID uint64,
	reason string,
) (Order, error) {
	publicID = strings.TrimSpace(publicID)
	if !ValidULID(publicID) {
		return Order{}, ErrInvalidULID
	}
	repository, ok := s.repository.(AdminOrderRepository)
	if !ok {
		return Order{}, ErrInvalidRecord
	}
	return repository.ReassignRider(ctx, publicID, expectedRowVersion, newRiderUserID, actorUserID, reason, s.now())
}

func (s *Service) AdminCancelOrder(
	ctx context.Context,
	publicID string,
	expectedRowVersion, actorUserID uint64,
	reason string,
) (Order, error) {
	publicID = strings.TrimSpace(publicID)
	if !ValidULID(publicID) {
		return Order{}, ErrInvalidULID
	}
	repository, ok := s.repository.(AdminOrderRepository)
	if !ok {
		return Order{}, ErrInvalidRecord
	}
	return repository.CancelWithReason(ctx, publicID, expectedRowVersion, actorUserID, reason, s.now())
}
