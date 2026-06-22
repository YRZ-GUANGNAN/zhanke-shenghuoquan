package media

import (
	"context"
	"encoding/hex"
	"strings"
	"time"
)

type ReviewProjectionService struct {
	repository ReviewProjectionRepository
	now        func() time.Time
}

func NewReviewProjectionService(repository ReviewProjectionRepository, now func() time.Time) (*ReviewProjectionService, error) {
	if repository == nil {
		return nil, ErrInvalidRecord
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &ReviewProjectionService{repository: repository, now: now}, nil
}

func (s *ReviewProjectionService) EnsureProjection(ctx context.Context, assetPublicID string) (ReviewProjection, bool, error) {
	if !ValidULID(assetPublicID) {
		return ReviewProjection{}, false, ErrInvalidULID
	}
	projectionID, err := NewULID()
	if err != nil {
		return ReviewProjection{}, false, err
	}
	return s.repository.CreateReviewProjection(ctx, assetPublicID, projectionID, s.now().UTC())
}

func (s *ReviewProjectionService) Submit(ctx context.Context, projectionID string, reviewTaskID, expectedRowVersion uint64) (ReviewProjection, error) {
	if !ValidULID(projectionID) || reviewTaskID == 0 || expectedRowVersion == 0 {
		return ReviewProjection{}, ErrInvalidRecord
	}
	return s.repository.TransitionReviewProjection(ctx, projectionID, expectedRowVersion, ProjectionPending, ProjectionSubmitted, &reviewTaskID, s.now().UTC())
}

func (s *ReviewProjectionService) StartReview(ctx context.Context, projectionID string, expectedRowVersion uint64) (ReviewProjection, error) {
	if !ValidULID(projectionID) || expectedRowVersion == 0 {
		return ReviewProjection{}, ErrInvalidRecord
	}
	return s.repository.TransitionReviewProjection(ctx, projectionID, expectedRowVersion, ProjectionSubmitted, ProjectionReviewing, nil, s.now().UTC())
}

func (s *ReviewProjectionService) RecordDecision(ctx context.Context, input ApplyReviewDecisionInput) (ReviewProjection, error) {
	input.PayloadHash = strings.ToLower(strings.TrimSpace(input.PayloadHash))
	input.ResultCode = strings.TrimSpace(input.ResultCode)
	if !ValidULID(input.ProjectionID) || input.ReviewTaskID == 0 || input.DecisionVersion == 0 ||
		input.ExpectedRowVersion == 0 || !input.ExpectedStatus.Valid() || !input.Decision.Decision() ||
		!validSHA256(input.PayloadHash) || !CanTransitionReviewProjection(input.ExpectedStatus, input.Decision) {
		return ReviewProjection{}, ErrInvalidRecord
	}
	if input.Decision == ProjectionReviewFailed {
		if input.ResultCode == "" || len(input.ResultCode) > 64 {
			return ReviewProjection{}, ErrInvalidRecord
		}
	} else if input.ResultCode != "" {
		return ReviewProjection{}, ErrInvalidRecord
	}
	return s.repository.ApplyReviewDecision(ctx, input, s.now().UTC())
}

func (s *ReviewProjectionService) Find(ctx context.Context, projectionID string) (ReviewProjection, error) {
	if !ValidULID(projectionID) {
		return ReviewProjection{}, ErrInvalidULID
	}
	return s.repository.FindReviewProjection(ctx, projectionID)
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
