package takeout

import (
	"context"
	"time"
)

type DomainRiderOrderOps struct {
	reader ClaimableOrderReader
	now    func() time.Time
}

func NewRiderOrderOps(reader ClaimableOrderReader, now func() time.Time) (*DomainRiderOrderOps, error) {
	if reader == nil {
		return nil, ErrInvalidRecord
	}
	if now == nil {
		now = time.Now
	}
	return &DomainRiderOrderOps{reader: reader, now: now}, nil
}

func (o *DomainRiderOrderOps) StartDelivering(
	ctx context.Context,
	unit RiderOrderUnit,
	orderPublicID string,
	expectedRowVersion, riderUserID uint64,
) (Order, error) {
	service, err := NewOrderService(unit, unit, o.now)
	if err != nil {
		return Order{}, err
	}
	return service.StartDelivering(ctx, orderPublicID, expectedRowVersion, riderUserID)
}

func (o *DomainRiderOrderOps) CompleteDelivery(ctx context.Context, unit RiderOrderUnit, orderPublicID string) (Order, error) {
	order, err := unit.FindOrder(ctx, orderPublicID)
	if err != nil {
		return Order{}, err
	}
	service, err := NewOrderService(unit, unit, o.now)
	if err != nil {
		return Order{}, err
	}
	return service.CompleteDelivery(ctx, orderPublicID, order.RowVersion)
}

func (o *DomainRiderOrderOps) ListClaimableOrders(ctx context.Context) ([]Order, error) {
	return o.reader.ListOrdersByStatus(ctx, OrderReadyForPickup)
}

var _ RiderOrderOps = (*DomainRiderOrderOps)(nil)
