package takeout

import "testing"

func TestRiderAndFulfillmentStateMachines(t *testing.T) {
	riderAllowed := map[[2]RiderStatus]bool{
		{RiderActive, RiderResting}: true, {RiderActive, RiderSuspended}: true,
		{RiderResting, RiderActive}: true, {RiderResting, RiderSuspended}: true,
		{RiderSuspended, RiderActive}: true,
	}
	riderStatuses := []RiderStatus{RiderActive, RiderResting, RiderSuspended}
	for _, from := range riderStatuses {
		if !from.Valid() || from.Terminal() {
			t.Fatalf("invalid rider status semantics for %s", from)
		}
		for _, to := range riderStatuses {
			if got := CanTransitionRider(from, to); got != riderAllowed[[2]RiderStatus{from, to}] {
				t.Errorf("rider transition %s -> %s got=%v", from, to, got)
			}
		}
	}

	fulfillmentAllowed := map[[2]FulfillmentStatus]bool{
		{FulfillmentAssigned, FulfillmentToStore}: true,
		{FulfillmentToStore, FulfillmentPicked}:   true,
		{FulfillmentPicked, FulfillmentToUser}:    true,
		{FulfillmentToUser, FulfillmentDelivered}: true,
	}
	fulfillmentStatuses := []FulfillmentStatus{
		FulfillmentAssigned, FulfillmentToStore, FulfillmentPicked, FulfillmentToUser, FulfillmentDelivered,
	}
	for _, from := range fulfillmentStatuses {
		if !from.Valid() {
			t.Fatalf("invalid fulfillment status %s", from)
		}
		for _, to := range fulfillmentStatuses {
			if got := CanTransitionFulfillment(from, to); got != fulfillmentAllowed[[2]FulfillmentStatus{from, to}] {
				t.Errorf("fulfillment transition %s -> %s got=%v", from, to, got)
			}
		}
	}
	if !FulfillmentDelivered.Terminal() || FulfillmentToUser.Terminal() ||
		RiderStatus("unknown").Valid() || FulfillmentStatus("unknown").Valid() {
		t.Fatal("terminal/valid semantics mismatch")
	}
}
