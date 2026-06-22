package takeout

import (
	"context"
	"time"
)

type CatalogReader interface {
	FindDish(context.Context, string) (Dish, error)
}

type OrderRepository interface {
	CreateOrder(context.Context, *Order, []OrderItem) error
	FindOrder(context.Context, string) (Order, error)
	ListOrderItems(context.Context, string) ([]OrderItem, error)
	TransitionOrderStatus(context.Context, string, uint64, OrderStatus, OrderStatus, *string, *uint64, time.Time) (Order, error)
	CompleteDelivery(context.Context, string, uint64, string, time.Time) (Order, error)

	GetActiveFeeConfig(context.Context) (FeeConfig, error)
	UpdateFeeConfig(context.Context, uint64, uint64, uint64, time.Time) (FeeConfig, error)
	FindEarningByOrder(context.Context, string) (RiderEarningRecord, error)
	MarkSettled(context.Context, string, uint64, time.Time) (RiderEarningRecord, error)
}

var _ CatalogReader = (*GormRepository)(nil)
var _ OrderRepository = (*GormRepository)(nil)
