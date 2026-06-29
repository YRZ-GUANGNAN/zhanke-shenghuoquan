package delivery

var transitions = map[Status]map[Status]struct{}{
	StatusPendingFeePayment: statusSetOf(StatusReadyForClaim, StatusPaymentFailed, StatusCancelled),
	StatusPaymentFailed:     statusSetOf(StatusPendingFeePayment, StatusCancelled),
	StatusReadyForClaim:     statusSetOf(StatusClaimed, StatusCancelled),
	StatusClaimed:           statusSetOf(StatusToPickup, StatusReadyForClaim, StatusCancelled),
	StatusToPickup:          statusSetOf(StatusPickedUp, StatusReadyForClaim, StatusCancelled),
	StatusPickedUp:          statusSetOf(StatusDelivering),
	StatusDelivering:        statusSetOf(StatusDelivered),
	StatusDelivered:         {},
	StatusCancelled:         {},
}

func CanTransition(from, to Status) bool {
	_, ok := transitions[from][to]
	return ok
}

func statusSetOf(values ...Status) map[Status]struct{} {
	result := make(map[Status]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
