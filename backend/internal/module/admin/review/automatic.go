package review

import "strings"

// AutomaticDecision is an allow-listed terminal decision derived from a
// trusted content-safety result. Apply is false for every ambiguous result so
// the task remains available for manual review.
type AutomaticDecision struct {
	Status Status
	Reason string
	Apply  bool
}

var automaticObjectTypes = map[string]struct{}{
	"post":            {},
	"comment":         {},
	"product":         {},
	"product_comment": {},
}

func ClassifyAutomaticDecision(objectType, risk string) AutomaticDecision {
	if _, ok := automaticObjectTypes[strings.TrimSpace(objectType)]; !ok {
		return AutomaticDecision{}
	}
	risk = strings.TrimSpace(risk)
	switch {
	case risk == "wechat pass":
		return AutomaticDecision{
			Status: StatusApproved,
			Reason: "wechat text safety pass",
			Apply:  true,
		}
	case strings.HasPrefix(risk, "wechat risky"):
		return AutomaticDecision{
			Status: StatusRejected,
			Reason: "wechat text safety risky",
			Apply:  true,
		}
	default:
		return AutomaticDecision{}
	}
}
