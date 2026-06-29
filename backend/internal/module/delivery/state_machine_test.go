package delivery

import "testing"

func TestDeliveryStatusMachine(t *testing.T) {
	expected := map[Status][]Status{
		StatusPendingFeePayment: {StatusReadyForClaim, StatusPaymentFailed, StatusCancelled},
		StatusPaymentFailed:     {StatusPendingFeePayment, StatusCancelled},
		StatusReadyForClaim:     {StatusClaimed, StatusCancelled},
		StatusClaimed:           {StatusToPickup, StatusReadyForClaim, StatusCancelled},
		StatusToPickup:          {StatusPickedUp, StatusReadyForClaim, StatusCancelled},
		StatusPickedUp:          {StatusDelivering},
		StatusDelivering:        {StatusDelivered},
		StatusDelivered:         {},
		StatusCancelled:         {},
	}
	for from, targets := range expected {
		if !from.Valid() {
			t.Errorf("%q should be valid", from)
		}
		allowed := make(map[Status]struct{}, len(targets))
		for _, to := range targets {
			allowed[to] = struct{}{}
		}
		for to := range expected {
			_, want := allowed[to]
			if got := CanTransition(from, to); got != want {
				t.Errorf("CanTransition(%s,%s)=%v want=%v", from, to, got, want)
			}
		}
	}
	if !StatusDelivered.Terminal() || !StatusCancelled.Terminal() {
		t.Fatal("delivered and cancelled must be terminal")
	}
	for _, status := range []Status{
		StatusPendingFeePayment, StatusPaymentFailed, StatusReadyForClaim,
		StatusClaimed, StatusToPickup, StatusPickedUp, StatusDelivering,
	} {
		if status.Terminal() {
			t.Errorf("%s must not be terminal", status)
		}
	}
	if Status("unknown").Valid() || CanTransition(Status("unknown"), StatusDelivered) {
		t.Fatal("unknown status must fail closed")
	}
}
