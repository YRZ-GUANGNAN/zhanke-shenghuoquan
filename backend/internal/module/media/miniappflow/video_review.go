package miniappflow

import (
	coremedia "campus-circle/backend/internal/module/media"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type videoReviewer interface {
	ReviewVideo(context.Context, coremedia.Asset) (coremedia.VideoReviewJob, error)
}

func WithVideoReviewer(value videoReviewer) ServiceOption {
	return func(s *Service) { s.videoReview = value }
}
func (s *Service) VideoEnabled() bool { return s.videoReview != nil }

func (s *Service) advanceVideoReview(ctx context.Context, asset coremedia.Asset) error {
	if s.videoReview == nil {
		return coremedia.ErrVideoUnavailable
	}
	projection, _, err := s.projections.EnsureProjection(ctx, asset.AssetPublicID)
	if err != nil {
		return err
	}
	if projection.Status.Terminal() {
		return nil
	}
	job, err := s.videoReview.ReviewVideo(ctx, asset)
	if err != nil {
		return err
	}
	if job.JobID == "" && !job.Decision.Terminal() {
		// A transient submission failure has no provider identity yet. Do not
		// bind the projection until a durable job exists or the retry window ends.
		return nil
	}
	// Namespace task identity by asset as well as provider job. A timeout before
	// cloud submission still receives a stable identity and a failed decision.
	taskID, err := WechatReviewTaskID("cos-video:" + asset.AssetPublicID + ":" + job.JobID)
	if err != nil {
		return err
	}
	if projection.Status == coremedia.ProjectionPending {
		if err := s.appendReviewSubmission(ctx, projection, taskID); err != nil {
			return err
		}
		if err := s.consumeProjectionEvent(ctx); err != nil {
			return err
		}
		projection, err = s.projections.Find(ctx, projection.ProjectionID)
		if err != nil {
			return err
		}
	}
	if projection.Status == coremedia.ProjectionSubmitted {
		if err := s.appendReviewing(ctx, projection); err != nil {
			return err
		}
		if err := s.consumeProjectionEvent(ctx); err != nil {
			return err
		}
		projection, err = s.projections.Find(ctx, projection.ProjectionID)
		if err != nil {
			return err
		}
	}
	if !job.Decision.Terminal() {
		return nil
	}
	if projection.Status.Terminal() {
		return nil
	}
	if projection.Status != coremedia.ProjectionReviewing || projection.ReviewTaskID == nil || *projection.ReviewTaskID != taskID {
		return coremedia.ErrReviewInvariant
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("cos-video:%s:%s:%s:%s", asset.AssetPublicID, job.JobID, job.Decision, job.FailureCode)))
	payload, err := json.Marshal(coremedia.ConsumerEventPayload{
		ProjectionID: projection.ProjectionID, ReviewTaskID: taskID, ExpectedRowVersion: projection.RowVersion,
		ExpectedStatus: projection.Status, DecisionVersion: projection.AppliedDecisionVersion + 1,
		Decision: job.Decision, ResultCode: job.FailureCode, PayloadHash: hex.EncodeToString(hash[:]),
	})
	if err != nil {
		return err
	}
	if err := s.appendProjectionEvent(ctx, projection, coremedia.ConsumerOperationDecision, payload); err != nil {
		return err
	}
	return s.consumeProjectionEvent(ctx)
}
