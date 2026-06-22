package media

import "testing"

func TestReviewProjectionStateMachineMatrix(t *testing.T) {
	statuses := []ProjectionStatus{
		ProjectionPending, ProjectionSubmitted, ProjectionReviewing,
		ProjectionApproved, ProjectionRejected, ProjectionReviewFailed,
	}
	allowed := map[[2]ProjectionStatus]bool{
		{ProjectionPending, ProjectionSubmitted}:      true,
		{ProjectionSubmitted, ProjectionReviewing}:    true,
		{ProjectionSubmitted, ProjectionReviewFailed}: true,
		{ProjectionReviewing, ProjectionApproved}:     true,
		{ProjectionReviewing, ProjectionRejected}:     true,
		{ProjectionReviewing, ProjectionReviewFailed}: true,
	}
	for _, from := range statuses {
		for _, to := range statuses {
			got := CanTransitionReviewProjection(from, to)
			if got != allowed[[2]ProjectionStatus{from, to}] {
				t.Fatalf("transition %s -> %s: got %v", from, to, got)
			}
		}
	}
	for _, terminal := range []ProjectionStatus{ProjectionApproved, ProjectionRejected, ProjectionReviewFailed} {
		if !terminal.Terminal() {
			t.Fatalf("%s must be terminal", terminal)
		}
	}
}

func TestReviewProjectionRejectsUnknownStatus(t *testing.T) {
	unknown := ProjectionStatus("unknown")
	if unknown.Valid() || unknown.Terminal() || CanTransitionReviewProjection(ProjectionPending, unknown) || CanTransitionReviewProjection(unknown, ProjectionPending) {
		t.Fatal("unknown review state was accepted")
	}
}
