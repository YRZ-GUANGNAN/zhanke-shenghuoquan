package delivery

var proofTransitions = map[ProofStatus]map[ProofStatus]struct{}{
	ProofBinding: {
		ProofBinding:    {},
		ProofActive:     {},
		ProofDeadletter: {},
	},
	ProofActive: {
		ProofRevoked: {},
	},
	ProofDeadletter: {},
	ProofRevoked:    {},
}

func CanTransitionProof(from, to ProofStatus) bool {
	targets, ok := proofTransitions[from]
	if !ok {
		return false
	}
	_, ok = targets[to]
	return ok
}
