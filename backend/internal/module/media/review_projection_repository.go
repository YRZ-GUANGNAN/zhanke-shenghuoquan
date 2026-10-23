package media

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReviewProjectionRepository interface {
	CreateReviewProjection(context.Context, string, string, time.Time) (ReviewProjection, bool, error)
	FindReviewProjection(context.Context, string) (ReviewProjection, error)
	FindReviewProjectionByTaskID(context.Context, uint64) (ReviewProjection, error)
	TransitionReviewProjection(context.Context, string, uint64, ProjectionStatus, ProjectionStatus, *uint64, time.Time) (ReviewProjection, error)
	ApplyReviewDecision(context.Context, ApplyReviewDecisionInput, time.Time) (ReviewProjection, error)
}

func (r *GormRepository) CreateReviewProjection(ctx context.Context, assetPublicID, projectionID string, now time.Time) (ReviewProjection, bool, error) {
	var result ReviewProjection
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var asset Asset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("asset_public_id = ?", assetPublicID).First(&asset).Error; err != nil {
			return normalizeNotFound(err)
		}
		if asset.TechnicalStatus != TechnicalFinalReady || asset.StorageStage != StorageStageFinal ||
			asset.DeleteRequestedAt != nil || asset.DeletedAt != nil ||
			asset.FinalKey == nil || *asset.FinalKey == "" ||
			asset.FinalVersionID == nil || *asset.FinalVersionID == "" {
			return ErrReviewInvariant
		}
		existing, found, err := loadReviewProjectionByAsset(tx, asset.ID, false)
		if err != nil {
			return err
		}
		if found {
			result = existing
			return nil
		}
		value := ReviewProjection{
			ProjectionID: projectionID, AssetID: asset.ID, Status: ProjectionPending,
			RowVersion: 1, CreatedAt: now, UpdatedAt: now,
		}
		if !asset.Purpose.ContentReviewRequired() {
			value.Status = ProjectionNotRequired
			value.DecidedAt = timePointer(now)
		}
		if err := tx.Create(&value).Error; err != nil {
			return err
		}
		result, created = value, true
		return nil
	})
	return result, created, err
}

func (r *GormRepository) FindReviewProjection(ctx context.Context, projectionID string) (ReviewProjection, error) {
	var value ReviewProjection
	err := r.db.WithContext(ctx).Where("projection_id = ?", projectionID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) FindReviewProjectionByTaskID(ctx context.Context, reviewTaskID uint64) (ReviewProjection, error) {
	var value ReviewProjection
	err := r.db.WithContext(ctx).Where("review_task_id = ?", reviewTaskID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) TransitionReviewProjection(ctx context.Context, projectionID string, expectedVersion uint64, from, to ProjectionStatus, reviewTaskID *uint64, now time.Time) (ReviewProjection, error) {
	var result ReviewProjection
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		projection, err := loadReviewProjection(tx, projectionID, true)
		if err != nil {
			return err
		}
		if projection.Status == to {
			if to == ProjectionSubmitted && (!sameUint64Pointer(projection.ReviewTaskID, reviewTaskID) || reviewTaskID == nil) {
				return ErrReviewTaskConflict
			}
			result = projection
			return nil
		}
		if projection.RowVersion != expectedVersion {
			return ErrConcurrentUpdate
		}
		if projection.Status != from || !CanTransitionReviewProjection(from, to) {
			return ErrInvalidTransition
		}
		if from == ProjectionPending {
			if to != ProjectionSubmitted || reviewTaskID == nil || *reviewTaskID == 0 || projection.ReviewTaskID != nil {
				return ErrReviewInvariant
			}
		} else if reviewTaskID != nil && !sameUint64Pointer(projection.ReviewTaskID, reviewTaskID) {
			return ErrReviewTaskConflict
		}

		if _, err := lockReviewAsset(tx, projection.AssetID); err != nil {
			return err
		}
		updates := map[string]any{
			"status": to, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		}
		if to == ProjectionSubmitted {
			updates["review_task_id"] = *reviewTaskID
			updates["submitted_at"] = now
		}
		if to == ProjectionReviewing {
			updates["reviewing_at"] = now
		}
		if err := casUpdate(tx.Model(&ReviewProjection{}).
			Where("projection_id = ? AND row_version = ? AND status = ?", projection.ProjectionID, expectedVersion, from).
			Updates(updates)); err != nil {
			return err
		}
		projection.Status, projection.RowVersion, projection.UpdatedAt = to, projection.RowVersion+1, now
		if to == ProjectionSubmitted {
			projection.ReviewTaskID = uint64ReviewPointer(*reviewTaskID)
			projection.SubmittedAt = timePointer(now)
		}
		if to == ProjectionReviewing {
			projection.ReviewingAt = timePointer(now)
		}
		result = projection
		return nil
	})
	return result, err
}

func (r *GormRepository) ApplyReviewDecision(ctx context.Context, input ApplyReviewDecisionInput, now time.Time) (ReviewProjection, error) {
	var result ReviewProjection
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		projection, err := loadReviewProjection(tx, input.ProjectionID, true)
		if err != nil {
			return err
		}
		existing, found, err := loadReviewDecision(tx, input.ReviewTaskID, input.DecisionVersion)
		if err != nil {
			return err
		}
		if found {
			if existing.ProjectionID != input.ProjectionID || existing.Decision != input.Decision ||
				existing.PayloadHash != input.PayloadHash || existing.ResultCode != input.ResultCode ||
				projection.Status != input.Decision || projection.AppliedDecisionVersion < input.DecisionVersion {
				return ErrDecisionConflict
			}
			result = projection
			return nil
		}
		if projection.RowVersion != input.ExpectedRowVersion {
			return ErrConcurrentUpdate
		}
		if projection.Status != input.ExpectedStatus || !CanTransitionReviewProjection(input.ExpectedStatus, input.Decision) {
			return ErrInvalidTransition
		}
		if projection.ReviewTaskID == nil || *projection.ReviewTaskID != input.ReviewTaskID {
			return ErrReviewTaskConflict
		}
		if input.DecisionVersion <= projection.AppliedDecisionVersion {
			return ErrStaleDecision
		}

		if _, err := lockReviewAsset(tx, projection.AssetID); err != nil {
			return err
		}
		decision := ReviewDecision{
			ProjectionID: projection.ProjectionID, ReviewTaskID: input.ReviewTaskID,
			DecisionVersion: input.DecisionVersion, Decision: input.Decision,
			PayloadHash: input.PayloadHash, ResultCode: input.ResultCode, ReceivedAt: now,
		}
		if err := tx.Create(&decision).Error; err != nil {
			return err
		}
		if err := casUpdate(tx.Model(&ReviewProjection{}).
			Where("projection_id = ? AND row_version = ? AND status = ?", projection.ProjectionID, input.ExpectedRowVersion, input.ExpectedStatus).
			Updates(map[string]any{
				"status": input.Decision, "applied_decision_version": input.DecisionVersion,
				"last_decision_hash": input.PayloadHash, "failure_code": input.ResultCode,
				"decided_at": now, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})); err != nil {
			return err
		}
		projection.Status = input.Decision
		projection.AppliedDecisionVersion = input.DecisionVersion
		projection.LastDecisionHash = stringPointer(input.PayloadHash)
		projection.FailureCode = input.ResultCode
		projection.DecidedAt = timePointer(now)
		projection.RowVersion++
		projection.UpdatedAt = now
		result = projection
		return nil
	})
	return result, err
}

func loadReviewProjection(tx *gorm.DB, projectionID string, lock bool) (ReviewProjection, error) {
	query := tx
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var value ReviewProjection
	err := query.Where("projection_id = ?", projectionID).First(&value).Error
	return value, normalizeNotFound(err)
}

func loadReviewProjectionByAsset(tx *gorm.DB, assetID uint64, lock bool) (ReviewProjection, bool, error) {
	query := tx
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var value ReviewProjection
	err := query.Where("asset_id = ?", assetID).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ReviewProjection{}, false, nil
	}
	return value, err == nil, err
}

func loadReviewDecision(tx *gorm.DB, taskID, decisionVersion uint64) (ReviewDecision, bool, error) {
	var value ReviewDecision
	err := tx.Where("review_task_id = ? AND decision_version = ?", taskID, decisionVersion).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ReviewDecision{}, false, nil
	}
	return value, err == nil, err
}

func lockReviewAsset(tx *gorm.DB, assetID uint64) (Asset, error) {
	var asset Asset
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&asset, assetID).Error; err != nil {
		return Asset{}, normalizeNotFound(err)
	}
	if asset.TechnicalStatus != TechnicalFinalReady || asset.StorageStage != StorageStageFinal ||
		asset.FinalKey == nil || *asset.FinalKey == "" || asset.FinalVersionID == nil || *asset.FinalVersionID == "" {
		return Asset{}, ErrReviewInvariant
	}
	return asset, nil
}

func sameUint64Pointer(left, right *uint64) bool {
	return left != nil && right != nil && *left == *right
}

func uint64ReviewPointer(value uint64) *uint64 { return &value }

var _ ReviewProjectionRepository = (*GormRepository)(nil)
