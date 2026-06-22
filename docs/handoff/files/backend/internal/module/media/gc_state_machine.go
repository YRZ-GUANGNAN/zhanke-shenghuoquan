package media

var gcTransitions = map[GCStatus]map[GCStatus]struct{}{
	GCPending:    setOf(GCProcessing),
	GCProcessing: setOf(GCRetryWait, GCSucceeded, GCDeadletter),
	GCRetryWait:  setOf(GCProcessing),
	GCSucceeded:  {},
	GCDeadletter: {},
}

func CanTransitionGC(from, to GCStatus) bool { return contains(gcTransitions[from], to) }

func (s GCStatus) Terminal() bool { return s == GCSucceeded || s == GCDeadletter }
