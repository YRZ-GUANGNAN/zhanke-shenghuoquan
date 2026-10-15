package media

// ContentReviewRequired is a server-owned purpose policy. Identity proof
// uploads remain private and still require a separate human identity decision.
func (p Purpose) ContentReviewRequired() bool {
	switch p {
	case PurposeAvatar, PurposeProfileCover, PurposeIdentityApplication:
		return false
	default:
		return true
	}
}

// ReviewAllowsUse only addresses review eligibility. Callers must still check
// ownership, fixed final object identity, deletion and reference visibility.
func ReviewAllowsUse(p Purpose, status ProjectionStatus) bool {
	return p.Valid() && (status == ProjectionApproved || (!p.ContentReviewRequired() && status == ProjectionNotRequired))
}

// These placeholders use the same allowlist as ContentReviewRequired, while
// retaining the database recheck immediately before a display URL is signed.
const displayReviewPredicate = "(p.status = ? OR (p.status = ? AND a.purpose IN ?))"

func directUsePurposes() []Purpose {
	return []Purpose{PurposeAvatar, PurposeProfileCover, PurposeIdentityApplication}
}
