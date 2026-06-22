package media

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CompletionRepository interface {
	ClaimCompletion(context.Context, uint64, string, time.Time, time.Duration, time.Duration) (CompletionWork, error)
	PinCompletionSource(context.Context, CompletionWork, CompletionObject, time.Time) (CompletionWork, error)
	MarkCompletionPromoting(context.Context, CompletionWork, CompletionValidation, time.Time) (CompletionWork, error)
	RecordCompletionRetry(context.Context, string, string, CompletionPhase, string, time.Time) error
	FailCompletionValidation(context.Context, CompletionWork, string, time.Time) (UploadBundle, error)
	FinalizeCompletion(context.Context, CompletionWork, CompletionObject, time.Time) (UploadBundle, error)
	FindCompletion(context.Context, uint64, string) (CompletionWork, error)
}

func (r *GormRepository) PinCompletionSource(ctx context.Context, work CompletionWork, object CompletionObject, now time.Time) (CompletionWork, error) {
	var result CompletionWork
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		completion, found, err := loadCompletionRecord(tx, work.Completion.TicketID, true)
		if err != nil || !found {
			if err != nil {
				return err
			}
			return ErrNotFound
		}
		var asset Asset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&asset, completion.AssetID).Error; err != nil {
			return normalizeNotFound(err)
		}
		if completion.Phase != CompletionValidating || completion.RowVersion != work.Completion.RowVersion ||
			!sameLease(completion, work.LeaseToken, now) || asset.TechnicalStatus != TechnicalValidating || object.VersionID == "" || object.ETag == "" {
			return ErrConcurrentUpdate
		}
		if completion.StagingVersionID != nil {
			if *completion.StagingVersionID != object.VersionID || completion.StagingETag == nil || *completion.StagingETag != object.ETag {
				return ErrCompletionInvariant
			}
			result = CompletionWork{Bundle: work.Bundle, Completion: completion, LeaseToken: work.LeaseToken, ShouldProcess: true}
			result.Bundle.Asset = asset
			return nil
		}
		if err := casUpdate(tx.Model(&Asset{}).
			Where("id = ? AND row_version = ? AND technical_status = ?", asset.ID, asset.RowVersion, TechnicalValidating).
			Updates(map[string]any{"staging_version_id": object.VersionID, "row_version": gorm.Expr("row_version + 1"), "updated_at": now})); err != nil {
			return err
		}
		if err := casUpdate(tx.Model(&UploadCompletion{}).
			Where("ticket_id = ? AND row_version = ? AND phase = ? AND lease_token = ?", completion.TicketID, completion.RowVersion, CompletionValidating, work.LeaseToken).
			Updates(map[string]any{
				"staging_version_id": object.VersionID, "staging_etag": object.ETag,
				"staging_last_modified": object.LastModified, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})); err != nil {
			return err
		}
		asset.StagingVersionID = stringPointer(object.VersionID)
		asset.RowVersion++
		completion.StagingVersionID = stringPointer(object.VersionID)
		completion.StagingETag = stringPointer(object.ETag)
		completion.StagingLastModified = timePointer(object.LastModified)
		completion.RowVersion++
		result = CompletionWork{Bundle: work.Bundle, Completion: completion, LeaseToken: work.LeaseToken, ShouldProcess: true}
		result.Bundle.Asset = asset
		return nil
	})
	return result, err
}

func (r *GormRepository) ClaimCompletion(ctx context.Context, ownerID uint64, ticketID string, now time.Time, leaseTTL, processingTTL time.Duration) (CompletionWork, error) {
	leaseToken, err := NewULID()
	if err != nil {
		return CompletionWork{}, err
	}
	var result CompletionWork
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		bundle, err := loadUploadBundle(tx, ticketID, ownerID, true)
		if err != nil {
			return err
		}
		completion, found, err := loadCompletionRecord(tx, ticketID, true)
		if err != nil {
			return err
		}
		if bundle.Asset.TechnicalStatus == TechnicalFinalReady && bundle.Asset.StorageStage == StorageStageFinal &&
			bundle.Ticket.Status == TicketConsumed && bundle.Reservation.Status == ReservationCommitted && found && completion.Phase == CompletionFinalReady {
			result = CompletionWork{Bundle: bundle, Completion: completion}
			return nil
		}
		if bundle.Ticket.Status != TicketUploaded || bundle.Reservation.Status != ReservationActive {
			return ErrCompletionInvariant
		}
		if !found {
			if bundle.Asset.TechnicalStatus != TechnicalUploaded || bundle.Asset.StorageStage != StorageStageStaging {
				return ErrCompletionInvariant
			}
			deadline := now.Add(processingTTL)
			completion = UploadCompletion{
				TicketID: ticketID, AssetID: bundle.Asset.ID, Phase: CompletionValidating,
				LeaseToken: &leaseToken, LeaseExpiresAt: timePointer(now.Add(leaseTTL)), ProcessingDeadlineAt: deadline,
				ValidationAttempts: 1, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&completion).Error; err != nil {
				return err
			}
			if err := casUpdate(tx.Model(&Asset{}).
				Where("id = ? AND row_version = ? AND technical_status = ? AND storage_stage = ?", bundle.Asset.ID, bundle.Asset.RowVersion, TechnicalUploaded, StorageStageStaging).
				Updates(map[string]any{"technical_status": TechnicalValidating, "row_version": gorm.Expr("row_version + 1"), "updated_at": now})); err != nil {
				return err
			}
			// Upload expiry no longer owns the lifecycle once completion processing starts.
			if err := tx.Model(&UploadTicket{}).Where("ticket_id = ? AND row_version = ? AND status = ?", ticketID, bundle.Ticket.RowVersion, TicketUploaded).
				Update("expires_at", gorm.Expr("GREATEST(expires_at, ?)", deadline)).Error; err != nil {
				return err
			}
			bundle.Asset.TechnicalStatus = TechnicalValidating
			bundle.Asset.RowVersion++
			result = CompletionWork{Bundle: bundle, Completion: completion, LeaseToken: leaseToken, ShouldProcess: true}
			return nil
		}
		if completion.AssetID != bundle.Asset.ID {
			return ErrCompletionInvariant
		}
		if completion.Phase == CompletionValidationFailed {
			return ErrCompletionValidation
		}
		if completion.Phase == CompletionFinalReady {
			return ErrCompletionInvariant
		}
		if completion.ProcessingDeadlineAt.Before(now) || completion.ProcessingDeadlineAt.Equal(now) {
			return ErrCompletionAttemptsExceeded
		}
		if completion.LeaseToken != nil && completion.LeaseExpiresAt != nil && completion.LeaseExpiresAt.After(now) {
			return ErrCompletionInProgress
		}
		expectedAssetStatus := TechnicalValidating
		attemptField := "validation_attempts"
		if completion.Phase == CompletionPromoting {
			expectedAssetStatus = TechnicalPromoting
			attemptField = "promote_attempts"
		}
		if bundle.Asset.TechnicalStatus != expectedAssetStatus {
			return ErrCompletionInvariant
		}
		updates := map[string]any{
			"lease_token": leaseToken, "lease_expires_at": now.Add(leaseTTL),
			attemptField: gorm.Expr(attemptField + " + 1"), "row_version": gorm.Expr("row_version + 1"),
			"last_error_code": "", "updated_at": now,
		}
		if err := casUpdate(tx.Model(&UploadCompletion{}).
			Where("ticket_id = ? AND row_version = ? AND phase = ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?)", ticketID, completion.RowVersion, completion.Phase, now).
			Updates(updates)); err != nil {
			return err
		}
		completion.LeaseToken = &leaseToken
		completion.LeaseExpiresAt = timePointer(now.Add(leaseTTL))
		completion.RowVersion++
		completion.UpdatedAt = now
		if completion.Phase == CompletionPromoting {
			completion.PromoteAttempts++
		} else {
			completion.ValidationAttempts++
		}
		result = CompletionWork{Bundle: bundle, Completion: completion, LeaseToken: leaseToken, ShouldProcess: true}
		return nil
	})
	return result, err
}

func (r *GormRepository) MarkCompletionPromoting(ctx context.Context, work CompletionWork, validation CompletionValidation, now time.Time) (CompletionWork, error) {
	var result CompletionWork
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		completion, found, err := loadCompletionRecord(tx, work.Completion.TicketID, true)
		if err != nil || !found {
			if err != nil {
				return err
			}
			return ErrNotFound
		}
		var asset Asset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&asset, completion.AssetID).Error; err != nil {
			return normalizeNotFound(err)
		}
		if completion.Phase != CompletionValidating || completion.RowVersion != work.Completion.RowVersion || !sameLease(completion, work.LeaseToken, now) ||
			asset.TechnicalStatus != TechnicalValidating || asset.RowVersion != work.Bundle.Asset.RowVersion {
			return ErrConcurrentUpdate
		}
		if err := casUpdate(tx.Model(&Asset{}).
			Where("id = ? AND row_version = ? AND technical_status = ? AND storage_stage = ?", asset.ID, asset.RowVersion, TechnicalValidating, StorageStageStaging).
			Updates(map[string]any{
				"technical_status": TechnicalPromoting, "staging_version_id": validation.StagingVersionID,
				"trusted_sha256": validation.TrustedSHA256, "verified_mime": validation.VerifiedMIME,
				"byte_size": validation.VerifiedSize, "width": validation.Width, "height": validation.Height,
				"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})); err != nil {
			return err
		}
		if err := casUpdate(tx.Model(&UploadCompletion{}).
			Where("ticket_id = ? AND row_version = ? AND phase = ? AND lease_token = ?", completion.TicketID, completion.RowVersion, CompletionValidating, work.LeaseToken).
			Updates(map[string]any{
				"phase": CompletionPromoting, "staging_version_id": validation.StagingVersionID,
				"staging_etag": validation.StagingETag, "staging_last_modified": validation.StagingLastModified,
				"trusted_sha256": validation.TrustedSHA256, "verified_mime": validation.VerifiedMIME,
				"verified_size": validation.VerifiedSize, "verified_width": validation.Width,
				"verified_height": validation.Height, "final_key": validation.FinalKey,
				"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})); err != nil {
			return err
		}
		asset.TechnicalStatus = TechnicalPromoting
		asset.StagingVersionID = stringPointer(validation.StagingVersionID)
		asset.TrustedSHA256 = stringPointer(validation.TrustedSHA256)
		asset.VerifiedMIME, asset.ByteSize, asset.Width, asset.Height = validation.VerifiedMIME, validation.VerifiedSize, validation.Width, validation.Height
		asset.RowVersion++
		completion.Phase = CompletionPromoting
		completion.StagingVersionID = stringPointer(validation.StagingVersionID)
		completion.StagingETag = stringPointer(validation.StagingETag)
		completion.StagingLastModified = timePointer(validation.StagingLastModified)
		completion.TrustedSHA256 = stringPointer(validation.TrustedSHA256)
		completion.VerifiedMIME = stringPointer(validation.VerifiedMIME)
		completion.VerifiedSize = uint64Pointer(validation.VerifiedSize)
		completion.VerifiedWidth = uint32Pointer(validation.Width)
		completion.VerifiedHeight = uint32Pointer(validation.Height)
		completion.FinalKey = stringPointer(validation.FinalKey)
		completion.RowVersion++
		result = CompletionWork{Bundle: work.Bundle, Completion: completion, LeaseToken: work.LeaseToken, ShouldProcess: true}
		result.Bundle.Asset = asset
		return nil
	})
	return result, err
}

func (r *GormRepository) RecordCompletionRetry(ctx context.Context, ticketID, leaseToken string, phase CompletionPhase, errorCode string, now time.Time) error {
	return casUpdate(r.db.WithContext(ctx).Model(&UploadCompletion{}).
		Where("ticket_id = ? AND phase = ? AND lease_token = ?", ticketID, phase, leaseToken).
		Updates(map[string]any{
			"lease_token": nil, "lease_expires_at": now, "last_error_code": errorCode,
			"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		}))
}

func (r *GormRepository) FailCompletionValidation(ctx context.Context, work CompletionWork, errorCode string, now time.Time) (UploadBundle, error) {
	var result UploadBundle
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		bundle, err := loadUploadBundle(tx, work.Completion.TicketID, work.Bundle.Asset.OwnerID, true)
		if err != nil {
			return err
		}
		completion, found, err := loadCompletionRecord(tx, work.Completion.TicketID, true)
		if err != nil || !found {
			if err != nil {
				return err
			}
			return ErrNotFound
		}
		if completion.Phase != CompletionValidating || !sameLease(completion, work.LeaseToken, now) ||
			bundle.Asset.TechnicalStatus != TechnicalValidating || bundle.Ticket.Status != TicketUploaded || bundle.Reservation.Status != ReservationActive {
			return ErrConcurrentUpdate
		}
		if err := casUpdate(tx.Model(&Asset{}).
			Where("id = ? AND row_version = ? AND technical_status = ?", bundle.Asset.ID, bundle.Asset.RowVersion, TechnicalValidating).
			Updates(map[string]any{"technical_status": TechnicalDeletePending, "row_version": gorm.Expr("row_version + 1"), "updated_at": now})); err != nil {
			return err
		}
		if err := updateTicketCAS(tx, &bundle.Ticket, bundle.Ticket.RowVersion, TicketUploaded, TicketCancelled, now); err != nil {
			return err
		}
		if err := updateReservationCAS(tx, &bundle.Reservation, ReservationActive, ReservationReleased, now); err != nil {
			return err
		}
		if err := casUpdate(tx.Model(&UploadCompletion{}).
			Where("ticket_id = ? AND row_version = ? AND phase = ? AND lease_token = ?", completion.TicketID, completion.RowVersion, CompletionValidating, work.LeaseToken).
			Updates(map[string]any{
				"phase": CompletionValidationFailed, "lease_token": nil, "lease_expires_at": nil,
				"last_error_code": errorCode, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})); err != nil {
			return err
		}
		bundle.Asset.TechnicalStatus = TechnicalDeletePending
		bundle.Asset.RowVersion++
		result = bundle
		return nil
	})
	return result, err
}

func (r *GormRepository) FinalizeCompletion(ctx context.Context, work CompletionWork, final CompletionObject, now time.Time) (UploadBundle, error) {
	var result UploadBundle
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		bundle, err := loadUploadBundle(tx, work.Completion.TicketID, work.Bundle.Asset.OwnerID, true)
		if err != nil {
			return err
		}
		completion, found, err := loadCompletionRecord(tx, work.Completion.TicketID, true)
		if err != nil || !found {
			if err != nil {
				return err
			}
			return ErrNotFound
		}
		if completion.Phase == CompletionFinalReady && bundle.Asset.TechnicalStatus == TechnicalFinalReady &&
			bundle.Ticket.Status == TicketConsumed && bundle.Reservation.Status == ReservationCommitted {
			result = bundle
			return nil
		}
		if completion.Phase != CompletionPromoting || !sameLease(completion, work.LeaseToken, now) ||
			bundle.Asset.TechnicalStatus != TechnicalPromoting || bundle.Asset.StorageStage != StorageStageStaging ||
			bundle.Ticket.Status != TicketUploaded || bundle.Reservation.Status != ReservationActive ||
			completion.FinalKey == nil || *completion.FinalKey != final.Key || final.VersionID == "" {
			return ErrConcurrentUpdate
		}
		if err := casUpdate(tx.Model(&Asset{}).
			Where("id = ? AND row_version = ? AND technical_status = ? AND storage_stage = ?", bundle.Asset.ID, bundle.Asset.RowVersion, TechnicalPromoting, StorageStageStaging).
			Updates(map[string]any{
				"technical_status": TechnicalFinalReady, "storage_stage": StorageStageFinal,
				"final_key": final.Key, "final_version_id": final.VersionID,
				"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})); err != nil {
			return err
		}
		if err := updateTicketCAS(tx, &bundle.Ticket, bundle.Ticket.RowVersion, TicketUploaded, TicketConsumed, now); err != nil {
			return err
		}
		if err := updateReservationCAS(tx, &bundle.Reservation, ReservationActive, ReservationCommitted, now); err != nil {
			return err
		}
		if err := casUpdate(tx.Model(&UploadCompletion{}).
			Where("ticket_id = ? AND row_version = ? AND phase = ? AND lease_token = ?", completion.TicketID, completion.RowVersion, CompletionPromoting, work.LeaseToken).
			Updates(map[string]any{
				"phase": CompletionFinalReady, "final_version_id": final.VersionID,
				"final_etag": final.ETag, "final_crc64": final.CRC64,
				"lease_token": nil, "lease_expires_at": nil, "last_error_code": "", "completed_at": now,
				"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})); err != nil {
			return err
		}
		bundle.Asset.TechnicalStatus = TechnicalFinalReady
		bundle.Asset.StorageStage = StorageStageFinal
		bundle.Asset.FinalKey = stringPointer(final.Key)
		bundle.Asset.FinalVersionID = stringPointer(final.VersionID)
		bundle.Asset.RowVersion++
		result = bundle
		return nil
	})
	return result, err
}

func (r *GormRepository) FindCompletion(ctx context.Context, ownerID uint64, ticketID string) (CompletionWork, error) {
	bundle, err := loadUploadBundle(r.db.WithContext(ctx), ticketID, ownerID, false)
	if err != nil {
		return CompletionWork{}, err
	}
	completion, found, err := loadCompletionRecord(r.db.WithContext(ctx), ticketID, false)
	if err != nil {
		return CompletionWork{}, err
	}
	if !found {
		return CompletionWork{}, ErrNotFound
	}
	return CompletionWork{Bundle: bundle, Completion: completion}, nil
}

func loadCompletionRecord(tx *gorm.DB, ticketID string, lock bool) (UploadCompletion, bool, error) {
	query := tx
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var value UploadCompletion
	err := query.Where("ticket_id = ?", ticketID).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return UploadCompletion{}, false, nil
	}
	return value, err == nil, err
}

func sameLease(value UploadCompletion, lease string, now time.Time) bool {
	return value.LeaseToken != nil && *value.LeaseToken == lease && value.LeaseExpiresAt != nil && value.LeaseExpiresAt.After(now)
}

func timePointer(value time.Time) *time.Time { return &value }
func stringPointer(value string) *string     { return &value }
func uint64Pointer(value uint64) *uint64     { return &value }
func uint32Pointer(value uint32) *uint32     { return &value }

var _ CompletionRepository = (*GormRepository)(nil)
