package takeout

var withdrawTransitions = map[WithdrawStatus]map[WithdrawStatus]struct{}{
	WithdrawPending: {
		WithdrawPaid:     {},
		WithdrawRejected: {},
	},
}

func CanTransitionWithdraw(from, to WithdrawStatus) bool {
	if !from.Valid() || !to.Valid() || from.Terminal() {
		return false
	}
	_, ok := withdrawTransitions[from][to]
	return ok
}
