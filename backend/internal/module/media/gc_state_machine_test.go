package media

import "testing"

func TestGCStateMachine(t *testing.T) {
	allowed := [][2]GCStatus{
		{GCPending, GCProcessing}, {GCProcessing, GCRetryWait},
		{GCProcessing, GCSucceeded}, {GCProcessing, GCDeadletter},
		{GCRetryWait, GCProcessing},
	}
	for _, transition := range allowed {
		if !CanTransitionGC(transition[0], transition[1]) {
			t.Fatalf("expected %s -> %s", transition[0], transition[1])
		}
	}
	for _, terminal := range []GCStatus{GCSucceeded, GCDeadletter} {
		if !terminal.Terminal() {
			t.Fatalf("%s must be terminal", terminal)
		}
		for _, target := range []GCStatus{GCPending, GCProcessing, GCRetryWait, GCSucceeded, GCDeadletter} {
			if CanTransitionGC(terminal, target) {
				t.Fatalf("terminal transition allowed: %s -> %s", terminal, target)
			}
		}
	}
}
