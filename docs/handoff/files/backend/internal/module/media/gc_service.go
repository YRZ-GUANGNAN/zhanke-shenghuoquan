package media

import (
	"context"
	"errors"
	"strings"
	"time"
)

type MediaGCService struct {
	repository MediaGCRepository
	store      GCObjectStore
	policy     GCPolicy
	now        func() time.Time
}

func NewMediaGCService(repository MediaGCRepository, store GCObjectStore, policy GCPolicy, now func() time.Time) (*MediaGCService, error) {
	if repository == nil || store == nil {
		return nil, ErrGCInvariant
	}
	if policy.StagingRetention <= 0 || policy.RejectedRetention <= 0 || policy.ApprovedRetention <= 0 ||
		policy.AuxiliaryRetention <= 0 || policy.LeaseTTL <= 0 || policy.RetryDelay <= 0 || policy.MaxAttempts == 0 {
		return nil, ErrGCInvariant
	}
	if now == nil {
		now = time.Now
	}
	return &MediaGCService{repository: repository, store: store, policy: policy, now: now}, nil
}

func (s *MediaGCService) Plan(ctx context.Context, assetPublicID string, stage GCObjectStage) (GCJob, bool, error) {
	if !ValidULID(assetPublicID) || (stage != GCStageStaging && stage != GCStageFinal) {
		return GCJob{}, false, ErrGCInvariant
	}
	return s.repository.PlanGCJob(ctx, assetPublicID, stage, s.now().UTC(), s.policy)
}

func (s *MediaGCService) RunBatch(ctx context.Context, limit int) (GCBatchResult, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	result := GCBatchResult{}
	for result.Claimed < limit {
		now := s.now().UTC()
		delivery, found, err := s.repository.ClaimNextGCJob(ctx, now, s.policy.LeaseTTL)
		if err != nil {
			return result, err
		}
		if !found {
			break
		}
		result.Claimed++
		_, err = s.repository.ExecuteGCDeletion(ctx, delivery, now, s.policy, s.store.DeleteVersion)
		if err == nil {
			result.Succeeded++
			continue
		}
		permanent := gcPermanentError(err)
		status, failErr := s.repository.FailGCJob(ctx, delivery, gcErrorCode(err), now.Add(s.policy.RetryDelay), s.policy.MaxAttempts, permanent, now)
		if failErr != nil {
			return result, errors.Join(err, failErr)
		}
		if status == GCDeadletter {
			result.Deadletters++
		} else {
			result.Retried++
		}
	}
	return result, nil
}

func (s *MediaGCService) PurgeTerminalAuxiliary(ctx context.Context, limit int) (GCAuxiliaryPurgeResult, error) {
	now := s.now().UTC()
	return s.repository.PurgeTerminalAuxiliary(ctx, now.Add(-s.policy.AuxiliaryRetention), now, s.policy.LeaseTTL, limit)
}

func gcPermanentError(err error) bool {
	return errors.Is(err, ErrGCFixedIdentity) || errors.Is(err, ErrGCInvariant) ||
		errors.Is(err, ErrInvalidTransition) || errors.Is(err, ErrNotFound)
}

func gcErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrGCFixedIdentity):
		return "fixed_identity_required"
	case errors.Is(err, ErrGCReferenceExists):
		return "reference_exists"
	case errors.Is(err, ErrGCNotEligible):
		return "not_eligible"
	case errors.Is(err, ErrGCInvariant):
		return "invariant"
	case errors.Is(err, ErrConcurrentUpdate):
		return "cas_conflict"
	case errors.Is(err, ErrGCLease):
		return "stale_lease"
	case errors.Is(err, ErrNotFound):
		return "not_found"
	default:
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "timeout") || strings.Contains(message, "deadline") {
			return "storage_timeout"
		}
		return "storage_delete_failed"
	}
}
