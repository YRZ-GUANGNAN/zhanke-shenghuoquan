package takeout

import "testing"

func TestWithdrawStateMachine(t *testing.T) {
	for _, transition := range []struct{ from, to WithdrawStatus }{
		{WithdrawPending, WithdrawPaid},
		{WithdrawPending, WithdrawRejected},
	} {
		if !CanTransitionWithdraw(transition.from, transition.to) {
			t.Fatalf("expected transition %s -> %s", transition.from, transition.to)
		}
	}
	for _, transition := range []struct{ from, to WithdrawStatus }{
		{WithdrawPaid, WithdrawPending},
		{WithdrawPaid, WithdrawRejected},
		{WithdrawRejected, WithdrawPaid},
		{WithdrawPending, WithdrawPending},
	} {
		if CanTransitionWithdraw(transition.from, transition.to) {
			t.Fatalf("unexpected transition %s -> %s", transition.from, transition.to)
		}
	}
}
