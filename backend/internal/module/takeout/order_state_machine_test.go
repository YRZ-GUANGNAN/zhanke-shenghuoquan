package takeout

import "testing"

func TestOrderStatusMachine(t *testing.T) {
	allowed := map[[2]OrderStatus]bool{
		{OrderPendingPayment, OrderPaid}: true, {OrderPendingPayment, OrderCancelled}: true,
		{OrderPaid, OrderAccepted}: true, {OrderPaid, OrderRefunded}: true,
		{OrderAccepted, OrderPreparing}: true, {OrderAccepted, OrderRefunded}: true,
		{OrderPreparing, OrderReadyForPickup}:  true,
		{OrderReadyForPickup, OrderDelivering}: true,
		{OrderDelivering, OrderCompleted}:      true,
	}
	statuses := []OrderStatus{
		OrderPendingPayment, OrderPaid, OrderAccepted, OrderPreparing, OrderReadyForPickup,
		OrderDelivering, OrderCompleted, OrderCancelled, OrderRefunded,
	}
	for _, from := range statuses {
		if !from.Valid() {
			t.Fatalf("status %s should be valid", from)
		}
		for _, to := range statuses {
			if got := CanTransitionOrder(from, to); got != allowed[[2]OrderStatus{from, to}] {
				t.Errorf("transition %s -> %s got=%v", from, to, got)
			}
		}
	}
	for _, terminal := range []OrderStatus{OrderCompleted, OrderCancelled, OrderRefunded} {
		if !terminal.Terminal() {
			t.Errorf("%s should be terminal", terminal)
		}
	}
	if OrderStatus("unknown").Valid() || CanTransitionOrder(OrderCompleted, OrderPaid) {
		t.Fatal("unknown or terminal transition accepted")
	}
	if !CanTransitionSettlement(SettlementPending, SettlementSettled) ||
		CanTransitionSettlement(SettlementSettled, SettlementPending) || !SettlementSettled.Terminal() {
		t.Fatal("settlement state machine mismatch")
	}
}
