package media

var consumerTransitions = map[ConsumerStatus]map[ConsumerStatus]struct{}{
	ConsumerPending:    setOf(ConsumerProcessing),
	ConsumerProcessing: setOf(ConsumerRetryWait, ConsumerSucceeded, ConsumerDeadletter),
	ConsumerRetryWait:  setOf(ConsumerProcessing),
	ConsumerSucceeded:  {},
	ConsumerDeadletter: {},
}

func CanTransitionConsumer(from, to ConsumerStatus) bool {
	return contains(consumerTransitions[from], to)
}
