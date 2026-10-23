package media

var reviewProjectionTransitions = map[ProjectionStatus]map[ProjectionStatus]struct{}{
	ProjectionPending:      setOf(ProjectionSubmitted),
	ProjectionSubmitted:    setOf(ProjectionReviewing, ProjectionReviewFailed),
	ProjectionReviewing:    setOf(ProjectionApproved, ProjectionRejected, ProjectionReviewFailed),
	ProjectionApproved:     {},
	ProjectionRejected:     {},
	ProjectionReviewFailed: {},
	ProjectionNotRequired:  {},
}

func CanTransitionReviewProjection(from, to ProjectionStatus) bool {
	return contains(reviewProjectionTransitions[from], to)
}
