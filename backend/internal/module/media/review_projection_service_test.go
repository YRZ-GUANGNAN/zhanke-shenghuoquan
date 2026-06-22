package media

import (
	"context"
	"errors"
	"testing"
	"time"
)

type reviewProjectionRepositoryStub struct {
	transitionInput struct {
		projectionID string
		version      uint64
		from         ProjectionStatus
		to           ProjectionStatus
		taskID       *uint64
	}
	decisionInput ApplyReviewDecisionInput
}

func (s *reviewProjectionRepositoryStub) CreateReviewProjection(context.Context, string, string, time.Time) (ReviewProjection, bool, error) {
	return ReviewProjection{}, false, nil
}
func (s *reviewProjectionRepositoryStub) FindReviewProjection(context.Context, string) (ReviewProjection, error) {
	return ReviewProjection{}, nil
}
func (s *reviewProjectionRepositoryStub) TransitionReviewProjection(_ context.Context, id string, version uint64, from, to ProjectionStatus, taskID *uint64, _ time.Time) (ReviewProjection, error) {
	s.transitionInput.projectionID = id
	s.transitionInput.version = version
	s.transitionInput.from = from
	s.transitionInput.to = to
	s.transitionInput.taskID = taskID
	return ReviewProjection{ProjectionID: id, Status: to, RowVersion: version + 1}, nil
}
func (s *reviewProjectionRepositoryStub) ApplyReviewDecision(_ context.Context, input ApplyReviewDecisionInput, _ time.Time) (ReviewProjection, error) {
	s.decisionInput = input
	return ReviewProjection{ProjectionID: input.ProjectionID, Status: input.Decision}, nil
}

func TestReviewProjectionServiceUsesExplicitCASBoundary(t *testing.T) {
	repository := &reviewProjectionRepositoryStub{}
	now := time.Date(2026, 6, 21, 16, 0, 0, 0, time.UTC)
	service, err := NewReviewProjectionService(repository, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	projectionID, _ := NewULID()
	if _, err := service.Submit(context.Background(), projectionID, 41001, 1); err != nil {
		t.Fatal(err)
	}
	if repository.transitionInput.from != ProjectionPending || repository.transitionInput.to != ProjectionSubmitted ||
		repository.transitionInput.version != 1 || repository.transitionInput.taskID == nil || *repository.transitionInput.taskID != 41001 {
		t.Fatalf("submit lost expected status/version CAS: %#v", repository.transitionInput)
	}
	if _, err := service.StartReview(context.Background(), projectionID, 2); err != nil {
		t.Fatal(err)
	}
	if repository.transitionInput.from != ProjectionSubmitted || repository.transitionInput.to != ProjectionReviewing || repository.transitionInput.version != 2 {
		t.Fatalf("start review lost expected status/version CAS: %#v", repository.transitionInput)
	}
}

func TestReviewProjectionServiceValidatesDecisionFailClosed(t *testing.T) {
	repository := &reviewProjectionRepositoryStub{}
	service, _ := NewReviewProjectionService(repository, nil)
	projectionID, _ := NewULID()
	base := ApplyReviewDecisionInput{
		ProjectionID: projectionID, ReviewTaskID: 42001, DecisionVersion: 1,
		ExpectedStatus: ProjectionReviewing, ExpectedRowVersion: 3,
		Decision: ProjectionApproved, PayloadHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	if _, err := service.RecordDecision(context.Background(), base); err != nil {
		t.Fatal(err)
	}
	if repository.decisionInput.ExpectedStatus != ProjectionReviewing || repository.decisionInput.ExpectedRowVersion != 3 {
		t.Fatal("decision CAS boundary was not preserved")
	}

	invalid := []ApplyReviewDecisionInput{
		func() ApplyReviewDecisionInput { value := base; value.PayloadHash = "not-sha256"; return value }(),
		func() ApplyReviewDecisionInput { value := base; value.Decision = ProjectionSubmitted; return value }(),
		func() ApplyReviewDecisionInput { value := base; value.ExpectedStatus = ProjectionPending; return value }(),
		func() ApplyReviewDecisionInput { value := base; value.ResultCode = "unexpected"; return value }(),
		func() ApplyReviewDecisionInput {
			value := base
			value.Decision = ProjectionReviewFailed
			value.ResultCode = ""
			return value
		}(),
	}
	for index, input := range invalid {
		if _, err := service.RecordDecision(context.Background(), input); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("invalid case %d returned %v", index, err)
		}
	}
}
