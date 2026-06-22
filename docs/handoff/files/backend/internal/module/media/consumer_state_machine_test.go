package media

import "testing"

func TestMediaConsumerStateMachineMatrix(t *testing.T) {
	states := []ConsumerStatus{ConsumerPending, ConsumerProcessing, ConsumerRetryWait, ConsumerSucceeded, ConsumerDeadletter}
	allowed := map[[2]ConsumerStatus]bool{
		{ConsumerPending, ConsumerProcessing}:    true,
		{ConsumerProcessing, ConsumerRetryWait}:  true,
		{ConsumerProcessing, ConsumerSucceeded}:  true,
		{ConsumerProcessing, ConsumerDeadletter}: true,
		{ConsumerRetryWait, ConsumerProcessing}:  true,
	}
	for _, from := range states {
		for _, to := range states {
			if got := CanTransitionConsumer(from, to); got != allowed[[2]ConsumerStatus{from, to}] {
				t.Fatalf("transition %s -> %s got %v", from, to, got)
			}
		}
	}
}

func TestMediaConsumerTerminalStatesHaveNoOutboundTransition(t *testing.T) {
	for _, terminal := range []ConsumerStatus{ConsumerSucceeded, ConsumerDeadletter} {
		for _, target := range []ConsumerStatus{ConsumerPending, ConsumerProcessing, ConsumerRetryWait, ConsumerSucceeded, ConsumerDeadletter} {
			if CanTransitionConsumer(terminal, target) {
				t.Fatalf("terminal state %s transitioned to %s", terminal, target)
			}
		}
	}
}
