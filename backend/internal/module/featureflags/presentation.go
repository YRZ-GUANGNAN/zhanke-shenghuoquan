package featureflags

import "errors"

const (
	PresentationStandard   = "standard"
	PresentationCampusInfo = "campus_info"
)

var ErrInvalidPresentationMode = errors.New("presentationMode must be standard or campus_info")

func validPresentationMode(mode string) bool {
	return mode == PresentationStandard || mode == PresentationCampusInfo
}

// EffectivePolicy is the same for every visitor. Keep the durable original
// settings separate so selecting another presentation never destroys them.
func EffectivePolicy(value Policy) Policy {
	value = clonePolicy(value)
	if value.PresentationMode != PresentationStandard {
		for _, key := range AllKeys {
			value.Flags[key] = false
		}
		value.PostingMode = "admin_only"
		value.Enabled = true
	}
	return value
}
