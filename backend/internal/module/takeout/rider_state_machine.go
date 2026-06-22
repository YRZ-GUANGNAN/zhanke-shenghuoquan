package takeout

var riderTransitions = map[RiderStatus]map[RiderStatus]struct{}{
	RiderActive:    riderSetOf(RiderResting, RiderSuspended),
	RiderResting:   riderSetOf(RiderActive, RiderSuspended),
	RiderSuspended: riderSetOf(RiderActive),
}

func CanTransitionRider(from, to RiderStatus) bool {
	_, ok := riderTransitions[from][to]
	return ok
}

func riderSetOf(values ...RiderStatus) map[RiderStatus]struct{} {
	result := make(map[RiderStatus]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

var fulfillmentTransitions = map[FulfillmentStatus]map[FulfillmentStatus]struct{}{
	FulfillmentAssigned:  fulfillmentSetOf(FulfillmentToStore),
	FulfillmentToStore:   fulfillmentSetOf(FulfillmentPicked),
	FulfillmentPicked:    fulfillmentSetOf(FulfillmentToUser),
	FulfillmentToUser:    fulfillmentSetOf(FulfillmentDelivered),
	FulfillmentDelivered: {},
}

func CanTransitionFulfillment(from, to FulfillmentStatus) bool {
	_, ok := fulfillmentTransitions[from][to]
	return ok
}

func fulfillmentSetOf(values ...FulfillmentStatus) map[FulfillmentStatus]struct{} {
	result := make(map[FulfillmentStatus]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
