package media

import (
	"context"
	"time"
)

type VideoReviewService struct {
	repo     VideoReviewRepository
	provider VideoReviewProvider
	now      func() time.Time
}

func NewVideoReviewService(repo VideoReviewRepository, provider VideoReviewProvider) (*VideoReviewService, error) {
	if repo == nil || provider == nil {
		return nil, ErrVideoUnavailable
	}
	return &VideoReviewService{repo: repo, provider: provider, now: time.Now}, nil
}
func (s *VideoReviewService) ReviewVideo(ctx context.Context, asset Asset) (VideoReviewJob, error) {
	if !ValidULID(asset.AssetPublicID) || asset.Purpose != PurposePost || asset.VerifiedMIME != "video/mp4" || asset.TechnicalStatus != TechnicalFinalReady {
		return VideoReviewJob{}, ErrReviewInvariant
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	// The durable job timestamps are DATETIME(3). Normalize the injected clock
	// before comparing or persisting it so a freshly seeded job cannot appear
	// a few microseconds in the future after a database round trip.
	now := s.now().UTC().Truncate(time.Millisecond)
	var result VideoReviewJob
	var providerErr error
	err := s.repo.WithVideoJob(ctx, asset.AssetPublicID, now, func(job *VideoReviewJob) error {
		if job.Decision.Terminal() {
			result = *job
			return nil
		}
		if now.Sub(job.CreatedAt) > 30*time.Minute {
			job.Decision, job.FailureCode = ProjectionReviewFailed, "video_review_timeout"
			result = *job
			return nil
		}
		if now.Before(job.NextPollAt) {
			result = *job
			return nil
		}
		if job.JobID == "" {
			id, err := s.provider.SubmitVideo(ctx, asset)
			if err != nil {
				providerErr = err
				job.NextPollAt = now.Add(5 * time.Second)
				result = *job
				return nil
			}
			if id == "" || len(id) > 128 {
				return ErrReviewInvariant
			}
			job.JobID, job.NextPollAt = id, now.Add(5*time.Second)
			result = *job
			return nil
		}
		value, err := s.provider.QueryVideo(ctx, job.JobID)
		if err != nil {
			providerErr = err
			job.NextPollAt = now.Add(5 * time.Second)
			result = *job
			return nil
		}
		if value.JobID != job.JobID || value.AssetPublicID != asset.AssetPublicID {
			return ErrReviewInvariant
		}
		job.Decision, job.FailureCode = videoDecision(value)
		job.NextPollAt = now.Add(5 * time.Second)
		result = *job
		return nil
	})
	if err == nil {
		err = providerErr
	}
	return result, err
}
func videoDecision(value VideoReviewResult) (ProjectionStatus, string) {
	switch value.State {
	case "Submitted", "Snapshoting", "Auditing":
		return ProjectionPending, ""
	case "Failed":
		return ProjectionReviewFailed, "video_review_failed"
	case "Success":
		switch value.Result {
		case 0:
			return ProjectionApproved, ""
		case 1, 2:
			return ProjectionRejected, ""
		}
	}
	return ProjectionReviewFailed, "video_unknown_result"
}
