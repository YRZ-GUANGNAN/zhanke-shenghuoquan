package takeout

var merchantTransitions = map[MerchantStatus]map[MerchantStatus]struct{}{
	MerchantPendingReview: merchantSetOf(MerchantActive, MerchantRejected),
	MerchantActive:        merchantSetOf(MerchantResting, MerchantSuspended),
	MerchantResting:       merchantSetOf(MerchantActive, MerchantSuspended),
	MerchantSuspended:     merchantSetOf(MerchantActive),
	MerchantRejected:      {},
}

func CanTransitionMerchant(from, to MerchantStatus) bool {
	_, ok := merchantTransitions[from][to]
	return ok
}

func merchantSetOf(values ...MerchantStatus) map[MerchantStatus]struct{} {
	result := make(map[MerchantStatus]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
