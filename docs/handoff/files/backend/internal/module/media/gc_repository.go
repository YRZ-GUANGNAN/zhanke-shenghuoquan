package media

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MediaGCRepository interface {
	PlanGCJob(context.Context, string, GCObjectStage, time.Time, GCPolicy) (GCJob, bool, error)
	ClaimNextGCJob(context.Context, time.Time, time.Duration) (GCDelivery, bool, error)
	ExecuteGCDeletion(context.Context, GCDelivery, time.Time, GCPolicy, func(context.Context, string, string) (GCDeleteResult, error)) (GCJob, error)
	FailGCJob(context.Context, GCDelivery, string, time.Time, uint32, bool, time.Time) (GCStatus, error)
	PurgeTerminalAuxiliary(context.Context, time.Time, time.Time, time.Duration, int) (GCAuxiliaryPurgeResult, error)
}

func (r *GormRepository) PlanGCJob(ctx context.Context, assetPublicID string, stage GCObjectStage, now time.Time, policy GCPolicy) (GCJob, bool, error) {
	var result GCJob
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		asset, err := loadGCAsset(tx, assetPublicID)
		if err != nil {
			return err
		}
		key, version, err := validateGCCandidate(tx, asset, stage, now, policy)
		if err != nil {
			return err
		}
		var existing GCJob
		err = tx.Where("asset_id = ? AND object_stage = ? AND object_key = ? AND object_version_id = ?", asset.ID, stage, key, version).First(&existing).Error
		if err == nil {
			result = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		jobID, err := NewULID()
		if err != nil {
			return err
		}
		result = GCJob{
			JobID: jobID, AssetID: asset.ID, ObjectStage: stage, ObjectKey: key,
			ObjectVersionID: version, Status: GCPending, RowVersion: 1,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	return result, created, err
}

func (r *GormRepository) ClaimNextGCJob(ctx context.Context, now time.Time, leaseTTL time.Duration) (GCDelivery, bool, error) {
	leaseToken, err := NewULID()
	if err != nil {
		return GCDelivery{}, false, err
	}
	var delivery GCDelivery
	found := false
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var job GCJob
		err := tx.Where(`status = ? OR (status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)) OR
			(status = ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?))`,
			GCPending, GCRetryWait, now, GCProcessing, now).
			Order("created_at ASC, job_id ASC").
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Take(&job).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		from := job.Status
		if from != GCPending && from != GCRetryWait && from != GCProcessing {
			return ErrInvalidTransition
		}
		query := tx.Model(&GCJob{}).Where("job_id = ? AND status = ? AND row_version = ?", job.JobID, from, job.RowVersion)
		if from == GCRetryWait {
			query = query.Where("next_attempt_at IS NULL OR next_attempt_at <= ?", now)
		}
		if from == GCProcessing {
			query = query.Where("lease_expires_at IS NULL OR lease_expires_at <= ?", now)
		}
		if err := casUpdate(query.Updates(map[string]any{
			"status": GCProcessing, "lease_token": leaseToken, "lease_expires_at": now.Add(leaseTTL),
			"attempt_count": gorm.Expr("attempt_count + 1"), "next_attempt_at": nil,
			"last_error_code": "", "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		})); err != nil {
			return err
		}
		job.Status = GCProcessing
		job.LeaseToken = stringPointer(leaseToken)
		job.LeaseExpiresAt = timePointer(now.Add(leaseTTL))
		job.AttemptCount++
		job.NextAttemptAt = nil
		job.LastErrorCode = ""
		job.RowVersion++
		job.UpdatedAt = now
		delivery = GCDelivery{Job: job, LeaseToken: leaseToken}
		found = true
		return nil
	})
	return delivery, found, err
}

func (r *GormRepository) ExecuteGCDeletion(ctx context.Context, delivery GCDelivery, now time.Time, policy GCPolicy, deleteVersion func(context.Context, string, string) (GCDeleteResult, error)) (GCJob, error) {
	var completed GCJob
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		job, err := loadGCJob(tx, delivery.Job.JobID)
		if err != nil {
			return err
		}
		if !gcLeaseMatches(job, delivery, now) {
			return ErrGCLease
		}
		asset, err := loadGCAssetByID(tx, job.AssetID)
		if err != nil {
			return err
		}
		key, version, err := validateGCCandidate(tx, asset, job.ObjectStage, now, policy)
		if err != nil {
			return err
		}
		if key != job.ObjectKey || version != job.ObjectVersionID || key == "" || version == "" {
			return ErrGCFixedIdentity
		}
		deleted, err := deleteVersion(ctx, key, version)
		if err != nil {
			return err
		}
		if job.ObjectStage == GCStageFinal || (job.ObjectStage == GCStageStaging && asset.StorageStage == StorageStageStaging) {
			if !CanTransitionTechnical(asset.TechnicalStatus, TechnicalDeleted) {
				return ErrInvalidTransition
			}
			if err := casUpdate(tx.Model(&Asset{}).
				Where("id = ? AND technical_status = ? AND row_version = ?", asset.ID, asset.TechnicalStatus, asset.RowVersion).
				Updates(map[string]any{
					"technical_status": TechnicalDeleted, "deleted_at": now,
					"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
				})); err != nil {
				return err
			}
		}
		if err := casUpdate(tx.Model(&GCJob{}).
			Where("job_id = ? AND status = ? AND row_version = ? AND lease_token = ?", job.JobID, GCProcessing, job.RowVersion, delivery.LeaseToken).
			Updates(map[string]any{
				"status": GCSucceeded, "lease_token": nil, "lease_expires_at": nil,
				"next_attempt_at": nil, "last_error_code": "", "provider_request_id": deleted.ProviderRequestID,
				"deleted_at": now, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})); err != nil {
			return err
		}
		job.Status = GCSucceeded
		job.LeaseToken = nil
		job.LeaseExpiresAt = nil
		job.ProviderRequestID = deleted.ProviderRequestID
		job.DeletedAt = timePointer(now)
		job.RowVersion++
		job.UpdatedAt = now
		completed = job
		return nil
	})
	return completed, err
}

func (r *GormRepository) FailGCJob(ctx context.Context, delivery GCDelivery, errorCode string, retryAt time.Time, maxAttempts uint32, permanent bool, now time.Time) (GCStatus, error) {
	target := GCRetryWait
	if permanent || delivery.Job.AttemptCount >= maxAttempts {
		target = GCDeadletter
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		job, err := loadGCJob(tx, delivery.Job.JobID)
		if err != nil {
			return err
		}
		if !gcLeaseMatches(job, delivery, now) {
			return ErrGCLease
		}
		updates := map[string]any{
			"status": target, "lease_token": nil, "lease_expires_at": nil,
			"last_error_code": errorCode, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		}
		if target == GCRetryWait {
			updates["next_attempt_at"] = retryAt
		} else {
			updates["next_attempt_at"] = nil
		}
		return casUpdate(tx.Model(&GCJob{}).
			Where("job_id = ? AND status = ? AND row_version = ? AND lease_token = ?", job.JobID, GCProcessing, job.RowVersion, delivery.LeaseToken).
			Updates(updates))
	})
	return target, err
}

// PurgeTerminalAuxiliary removes only terminal upload helpers belonging to an
// already-deleted asset. Review decisions, references and GC tombstones remain
// immutable audit facts.
func (r *GormRepository) PurgeTerminalAuxiliary(ctx context.Context, before, now time.Time, leaseTTL time.Duration, limit int) (GCAuxiliaryPurgeResult, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var result GCAuxiliaryPurgeResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var assetIDs []uint64
		if err := tx.Model(&Asset{}).
			Where("technical_status = ? AND deleted_at IS NOT NULL AND deleted_at < ?", TechnicalDeleted, before).
			Order("id ASC").Limit(limit).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Pluck("id", &assetIDs).Error; err != nil {
			return err
		}
		if len(assetIDs) == 0 {
			return nil
		}
		for _, assetID := range assetIDs {
			leaseToken, err := NewULID()
			if err != nil {
				return err
			}
			job, err := loadOrCreateGCAuxiliaryJob(tx, assetID, now)
			if err != nil {
				return err
			}
			if job.Status == GCSucceeded || (job.Status == GCProcessing && job.LeaseExpiresAt != nil && job.LeaseExpiresAt.After(now)) {
				continue
			}
			if job.Status != GCPending && job.Status != GCProcessing {
				continue
			}
			claim := tx.Model(&GCAuxiliaryJob{}).
				Where("job_id = ? AND status = ? AND row_version = ?", job.JobID, job.Status, job.RowVersion)
			if job.Status == GCProcessing {
				claim = claim.Where("lease_expires_at IS NULL OR lease_expires_at <= ?", now)
			}
			if err := casUpdate(claim.Updates(map[string]any{
				"status": GCProcessing, "lease_token": leaseToken, "lease_expires_at": now.Add(leaseTTL),
				"attempt_count": gorm.Expr("attempt_count + 1"), "last_error_code": "",
				"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})); err != nil {
				return err
			}
			job.Status = GCProcessing
			job.RowVersion++

			var asset Asset
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&asset, assetID).Error; err != nil {
				return err
			}
			if asset.TechnicalStatus != TechnicalDeleted || asset.DeletedAt == nil || !asset.DeletedAt.Before(before) {
				return ErrGCNotEligible
			}
			var ticketIDs []string
			if err := tx.Model(&UploadTicket{}).
				Where("asset_id = ? AND status IN ? AND updated_at < ?", assetID,
					[]TicketStatus{TicketConsumed, TicketExpired, TicketCancelled}, before).
				Pluck("ticket_id", &ticketIDs).Error; err != nil {
				return err
			}
			if len(ticketIDs) > 0 {
				deleted := tx.Where("ticket_id IN ? AND status IN ? AND updated_at < ?", ticketIDs,
					[]ReservationStatus{ReservationCommitted, ReservationReleased}, before).Delete(&QuotaReservation{})
				if deleted.Error != nil {
					return deleted.Error
				}
				result.Reservations += deleted.RowsAffected
				deleted = tx.Where("ticket_id IN ? AND phase IN ? AND updated_at < ?", ticketIDs,
					[]CompletionPhase{CompletionFinalReady, CompletionValidationFailed}, before).Delete(&UploadCompletion{})
				if deleted.Error != nil {
					return deleted.Error
				}
				result.Completions += deleted.RowsAffected
				deleted = tx.Where("ticket_id IN ? AND status IN ? AND updated_at < ?", ticketIDs,
					[]TicketStatus{TicketConsumed, TicketExpired, TicketCancelled}, before).Delete(&UploadTicket{})
				if deleted.Error != nil {
					return deleted.Error
				}
				result.Tickets += deleted.RowsAffected
			}
			if err := casUpdate(tx.Model(&GCAuxiliaryJob{}).
				Where("job_id = ? AND status = ? AND row_version = ? AND lease_token = ?", job.JobID, GCProcessing, job.RowVersion, leaseToken).
				Updates(map[string]any{
					"status": GCSucceeded, "lease_token": nil, "lease_expires_at": nil, "purged_at": now,
					"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
				})); err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}

func loadOrCreateGCAuxiliaryJob(tx *gorm.DB, assetID uint64, now time.Time) (GCAuxiliaryJob, error) {
	var job GCAuxiliaryJob
	err := tx.Where("asset_id = ?", assetID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&job).Error
	if err == nil {
		return job, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return GCAuxiliaryJob{}, err
	}
	jobID, err := NewULID()
	if err != nil {
		return GCAuxiliaryJob{}, err
	}
	job = GCAuxiliaryJob{JobID: jobID, AssetID: assetID, Status: GCPending, RowVersion: 1, CreatedAt: now, UpdatedAt: now}
	return job, tx.Create(&job).Error
}

func validateGCCandidate(tx *gorm.DB, asset Asset, stage GCObjectStage, now time.Time, policy GCPolicy) (string, string, error) {
	var references []AssetReference
	if err := tx.Where("asset_id = ?", asset.ID).Clauses(clause.Locking{Strength: "UPDATE"}).Find(&references).Error; err != nil {
		return "", "", err
	}
	for _, reference := range references {
		if !validReferenceStatus(reference.Status) {
			return "", "", ErrGCInvariant
		}
	}

	switch stage {
	case GCStageStaging:
		key, version := dereference(asset.StagingKey), dereference(asset.StagingVersionID)
		if key == "" || version == "" {
			return "", "", ErrGCFixedIdentity
		}
		if asset.StorageStage == StorageStageStaging {
			if asset.TechnicalStatus != TechnicalDeletePending || asset.DeleteRequestedAt == nil || now.Before(asset.DeleteRequestedAt.Add(policy.StagingRetention)) {
				return "", "", ErrGCNotEligible
			}
			return key, version, nil
		}
		if asset.StorageStage != StorageStageFinal || !postPromoteTechnicalStatus(asset.TechnicalStatus) || dereference(asset.FinalKey) == "" || dereference(asset.FinalVersionID) == "" {
			return "", "", ErrGCNotEligible
		}
		var completion UploadCompletion
		if err := tx.Where("asset_id = ?", asset.ID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&completion).Error; err != nil {
			return "", "", normalizeNotFound(err)
		}
		if completion.CompletedAt == nil || completion.Phase != CompletionFinalReady || dereference(completion.StagingVersionID) != version ||
			dereference(completion.FinalVersionID) != dereference(asset.FinalVersionID) || now.Before(completion.CompletedAt.Add(policy.StagingRetention)) {
			return "", "", ErrGCNotEligible
		}
		return key, version, nil
	case GCStageFinal:
		key, version := dereference(asset.FinalKey), dereference(asset.FinalVersionID)
		if key == "" || version == "" {
			return "", "", ErrGCFixedIdentity
		}
		if asset.StorageStage != StorageStageFinal || asset.TechnicalStatus != TechnicalDeletePending || asset.DeleteRequestedAt == nil {
			return "", "", ErrGCNotEligible
		}
		for _, reference := range references {
			if reference.Status != ReferenceRevoked {
				return "", "", ErrGCReferenceExists
			}
		}
		var projection ReviewProjection
		if err := tx.Where("asset_id = ?", asset.ID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&projection).Error; err != nil {
			return "", "", normalizeNotFound(err)
		}
		if !projection.Status.Terminal() || projection.DecidedAt == nil {
			return "", "", ErrGCNotEligible
		}
		retention := policy.ApprovedRetention
		if projection.Status == ProjectionRejected || projection.Status == ProjectionReviewFailed {
			retention = policy.RejectedRetention
		}
		anchor := *asset.DeleteRequestedAt
		if projection.DecidedAt.After(anchor) {
			anchor = *projection.DecidedAt
		}
		if now.Before(anchor.Add(retention)) {
			return "", "", ErrGCNotEligible
		}
		return key, version, nil
	default:
		return "", "", ErrGCInvariant
	}
}

func loadGCAsset(tx *gorm.DB, publicID string) (Asset, error) {
	var asset Asset
	err := tx.Where("asset_public_id = ?", publicID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&asset).Error
	return asset, normalizeNotFound(err)
}

func loadGCAssetByID(tx *gorm.DB, id uint64) (Asset, error) {
	var asset Asset
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&asset, id).Error
	return asset, normalizeNotFound(err)
}

func loadGCJob(tx *gorm.DB, jobID string) (GCJob, error) {
	var job GCJob
	err := tx.Where("job_id = ?", jobID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&job).Error
	return job, normalizeNotFound(err)
}

func gcLeaseMatches(job GCJob, delivery GCDelivery, now time.Time) bool {
	return job.Status == GCProcessing && job.RowVersion == delivery.Job.RowVersion &&
		job.LeaseToken != nil && *job.LeaseToken == delivery.LeaseToken &&
		job.LeaseExpiresAt != nil && job.LeaseExpiresAt.After(now)
}

func validReferenceStatus(status ReferenceStatus) bool {
	return status == ReferencePending || status == ReferenceActive || status == ReferenceRevoking || status == ReferenceRevoked
}

func postPromoteTechnicalStatus(status TechnicalStatus) bool {
	switch status {
	case TechnicalFinalReady, TechnicalSubmitted, TechnicalReviewing, TechnicalApproved,
		TechnicalRejected, TechnicalReviewFailed, TechnicalDeletePending:
		return true
	default:
		return false
	}
}

func dereference(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

var _ MediaGCRepository = (*GormRepository)(nil)
