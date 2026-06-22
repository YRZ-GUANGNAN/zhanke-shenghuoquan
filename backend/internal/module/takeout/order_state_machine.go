package takeout

var orderTransitions = map[OrderStatus]map[OrderStatus]struct{}{
	OrderPendingPayment: orderSetOf(OrderPaid, OrderCancelled),
	OrderPaid:           orderSetOf(OrderAccepted, OrderRefunded),
	OrderAccepted:       orderSetOf(OrderPreparing, OrderRefunded),
	OrderPreparing:      orderSetOf(OrderReadyForPickup),
	OrderReadyForPickup: orderSetOf(OrderDelivering),
	OrderDelivering:     orderSetOf(OrderCompleted),
	OrderCompleted:      {},
	OrderCancelled:      {},
	OrderRefunded:       {},
}

func CanTransitionOrder(from, to OrderStatus) bool {
	_, ok := orderTransitions[from][to]
	return ok
}

func orderSetOf(values ...OrderStatus) map[OrderStatus]struct{} {
	result := make(map[OrderStatus]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func CanTransitionSettlement(from, to SettlementStatus) bool {
	return from == SettlementPending && to == SettlementSettled
}
