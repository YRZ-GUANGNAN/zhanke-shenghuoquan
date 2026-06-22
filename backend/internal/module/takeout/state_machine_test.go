package takeout

import "testing"

func TestMerchantStateMachineMatchesG1Contract(t *testing.T) {
	allowed := [][2]MerchantStatus{
		{MerchantPendingReview, MerchantActive},
		{MerchantPendingReview, MerchantRejected},
		{MerchantActive, MerchantResting},
		{MerchantActive, MerchantSuspended},
		{MerchantResting, MerchantActive},
		{MerchantResting, MerchantSuspended},
		{MerchantSuspended, MerchantActive},
	}
	for _, transition := range allowed {
		if !CanTransitionMerchant(transition[0], transition[1]) {
			t.Errorf("rejected legal transition %s -> %s", transition[0], transition[1])
		}
	}
	for _, transition := range [][2]MerchantStatus{
		{MerchantPendingReview, MerchantSuspended},
		{MerchantActive, MerchantRejected},
		{MerchantResting, MerchantRejected},
		{MerchantSuspended, MerchantResting},
		{MerchantRejected, MerchantActive},
		{MerchantRejected, MerchantRejected},
		{MerchantActive, MerchantActive},
	} {
		if CanTransitionMerchant(transition[0], transition[1]) {
			t.Errorf("accepted illegal transition %s -> %s", transition[0], transition[1])
		}
	}
}

func TestMerchantStatusValidityAndTerminality(t *testing.T) {
	for _, status := range []MerchantStatus{MerchantPendingReview, MerchantActive, MerchantResting, MerchantRejected, MerchantSuspended} {
		if !status.Valid() {
			t.Errorf("status %q is not valid", status)
		}
	}
	if MerchantStatus("unknown").Valid() {
		t.Fatal("unknown status accepted")
	}
	if !MerchantRejected.Terminal() || MerchantSuspended.Terminal() || MerchantActive.Terminal() {
		t.Fatal("terminal status invariant failed")
	}
}
