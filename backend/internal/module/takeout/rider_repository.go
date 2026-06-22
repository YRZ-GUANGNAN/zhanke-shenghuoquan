package takeout

import (
	"context"
	"time"
)

type RiderOrderUnit interface {
	OrderRepository
	CatalogReader
}

type ClaimableOrderReader interface {
	ListOrdersByStatus(context.Context, OrderStatus) ([]Order, error)
}

type RiderOrderOps interface {
	StartDelivering(context.Context, RiderOrderUnit, string, uint64, uint64) (Order, error)
	CompleteDelivery(context.Context, RiderOrderUnit, string) (Order, error)
	ListClaimableOrders(context.Context) ([]Order, error)
}

type RiderRepository interface {
	CreateRider(context.Context, *Rider) error
	FindRider(context.Context, uint64) (Rider, error)
	TransitionRiderStatus(context.Context, uint64, uint64, RiderStatus, RiderStatus, time.Time) (Rider, error)

	GetRiderConfig(context.Context) (RiderConfig, error)
	UpdateRiderConfig(context.Context, uint32, uint64, time.Time) (RiderConfig, error)

	ClaimOrder(context.Context, uint64, string, uint64, string, RiderOrderOps, time.Time) (Fulfillment, error)
	TransitionFulfillmentStatus(context.Context, string, uint64, FulfillmentStatus, FulfillmentStatus, time.Time) (Fulfillment, error)
	CompleteFulfillment(context.Context, string, uint64, RiderOrderOps, time.Time) (Fulfillment, error)
	FindFulfillmentByOrder(context.Context, string) (Fulfillment, error)
}

var _ RiderOrderUnit = (*GormRepository)(nil)
var _ ClaimableOrderReader = (*GormRepository)(nil)
var _ RiderRepository = (*GormRepository)(nil)
