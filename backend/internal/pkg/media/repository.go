package media

import (
	"context"
	"errors"
	"time"

	"campus-circle/backend/internal/pkg/dbtx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNotFound     = errors.New("media asset not found")
	ErrForbidden    = errors.New("media asset forbidden")
	ErrInvalidState = errors.New("invalid media asset state")
	ErrInvalidAsset = errors.New("invalid media asset")
	ErrAlreadyBound = errors.New("media asset already bound")
)

type Repository interface {
	CreateOrFind(context.Context, *Asset) (bool, error)
	RefreshUploadExpiry(context.Context, uint64, time.Time) error
	FindOwned(context.Context, string, uint) (Asset, error)
	FindPublic(context.Context, string) (Asset, error)
	MarkUploaded(context.Context, uint64, int64, string, int, int, string) error
	SetReviewTask(context.Context, uint64, uint64) error
	SetReviewStatus(context.Context, uint64, Status) error
	SetReviewTrace(context.Context, uint64, string) error
	MarkReviewFailure(context.Context, uint64, int, string, time.Time) error
	ApplyReviewResult(context.Context, string, string, int) (Asset, error)
	Reserve(context.Context, uint, Purpose, []string, string, time.Time) ([]Asset, error)
	Commit(context.Context, uint, string, string, uint64, []string) error
	CancelReservation(context.Context, uint, string) error
	RequestDeleteOwned(context.Context, string, uint) error
	RequestDeleteReference(context.Context, string, uint64, []uint64) error
	PendingReviews(context.Context, int, time.Time) ([]Asset, error)
	PendingDeletes(context.Context, int) ([]Asset, error)
	ExpiredGarbage(context.Context, int, time.Time) ([]Asset, error)
	MarkStorageDeleted(context.Context, uint64) error
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) CreateOrFind(ctx context.Context, value *Asset) (bool, error) {
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(value)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 1 {
			created = true
			return appendAudit(tx, *value, "", value.Status, "upload_created", "owner", value.OwnerUserID, "direct upload ticket created")
		}
		return tx.Where("owner_user_id = ? AND idempotency_key = ?", value.OwnerUserID, value.IdempotencyKey).First(value).Error
	})
	return created, err
}

func (r *GormRepository) RefreshUploadExpiry(ctx context.Context, id uint64, expires time.Time) error {
	result := r.db.WithContext(ctx).Model(&Asset{}).
		Where("id = ? AND status = ? AND delete_requested_at IS NULL", id, StatusPendingUpload).
		Update("upload_expires_at", expires)
	if result.Error == nil && result.RowsAffected == 0 {
		return ErrInvalidState
	}
	return result.Error
}

func (r *GormRepository) FindOwned(ctx context.Context, publicID string, ownerID uint) (Asset, error) {
	var value Asset
	err := r.db.WithContext(ctx).Where("public_id = ? AND owner_user_id = ?", publicID, ownerID).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Asset{}, ErrNotFound
	}
	return value, err
}

func (r *GormRepository) FindPublic(ctx context.Context, publicID string) (Asset, error) {
	var value Asset
	err := r.db.WithContext(ctx).Where("public_id = ? AND status = ? AND reference_type <> '' AND delete_requested_at IS NULL AND storage_deleted_at IS NULL", publicID, StatusApproved).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Asset{}, ErrNotFound
	}
	return value, err
}

func (r *GormRepository) MarkUploaded(ctx context.Context, id uint64, size int64, mime string, width, height int, openID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		value, err := lockAsset(tx, id)
		if err != nil {
			return err
		}
		if value.Status != StatusPendingUpload {
			return ErrInvalidState
		}
		if err := tx.Model(&Asset{}).Where("id = ? AND status = ?", id, StatusPendingUpload).
			Updates(map[string]any{"status": StatusUploaded, "verified_size": size, "verified_mime": mime, "width": width, "height": height, "wechat_open_id": openID, "last_error": "", "review_available_at": time.Now()}).Error; err != nil {
			return err
		}
		return appendAudit(tx, value, value.Status, StatusUploaded, "upload_completed", "owner", value.OwnerUserID, "COS object verified")
	})
}

func (r *GormRepository) SetReviewTask(ctx context.Context, id, taskID uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		value, err := lockAsset(tx, id)
		if err != nil {
			return err
		}
		if value.Status != StatusUploaded {
			return ErrInvalidState
		}
		if err := tx.Model(&Asset{}).Where("id = ?", id).Updates(map[string]any{"status": StatusSubmitted, "review_task_id": taskID}).Error; err != nil {
			return err
		}
		return appendAudit(tx, value, value.Status, StatusSubmitted, "review_submitted", "system", 0, "unified review task created")
	})
}

func (r *GormRepository) SetReviewStatus(ctx context.Context, id uint64, status Status) error {
	allowed := map[Status][]Status{
		StatusReviewing: {StatusSubmitted},
		StatusApproved:  {StatusReviewing},
		StatusRejected:  {StatusPendingUpload, StatusReviewing},
	}
	from, ok := allowed[status]
	if !ok {
		return ErrInvalidState
	}
	return dbtx.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		value, err := lockAsset(tx, id)
		if err != nil {
			return err
		}
		if !statusIn(value.Status, from) {
			return ErrInvalidState
		}
		if err := tx.Model(&Asset{}).Where("id = ? AND status = ?", id, value.Status).Update("status", status).Error; err != nil {
			return err
		}
		return appendAudit(tx, value, value.Status, status, "review_"+string(status), "system", 0, "media review state changed")
	})
}

func (r *GormRepository) SetReviewTrace(ctx context.Context, id uint64, trace string) error {
	result := r.db.WithContext(ctx).Model(&Asset{}).Where("id = ? AND status = ? AND review_trace_id IS NULL", id, StatusReviewing).
		Updates(map[string]any{"review_trace_id": trace, "last_error": ""})
	if result.Error == nil && result.RowsAffected == 0 {
		return ErrInvalidState
	}
	return result.Error
}

func (r *GormRepository) MarkReviewFailure(ctx context.Context, id uint64, attempts int, message string, available time.Time) error {
	return r.db.WithContext(ctx).Model(&Asset{}).Where("id = ?", id).Updates(map[string]any{
		"review_attempts": attempts, "last_error": message, "review_available_at": available,
	}).Error
}

func (r *GormRepository) ApplyReviewResult(ctx context.Context, trace, suggest string, label int) (Asset, error) {
	var value Asset
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("review_trace_id = ?", trace).First(&value).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if value.Status == StatusApproved || value.Status == StatusRejected {
			return nil
		}
		if value.Status != StatusReviewing {
			return ErrInvalidState
		}
		return tx.Model(&Asset{}).Where("id = ?", value.ID).Updates(map[string]any{"review_suggest": suggest, "review_label": label}).Error
	})
	return value, err
}

func (r *GormRepository) Reserve(ctx context.Context, ownerID uint, purpose Purpose, publicIDs []string, token string, expires time.Time) ([]Asset, error) {
	if len(publicIDs) == 0 {
		return nil, nil
	}
	var values []Asset
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("public_id IN ?", publicIDs).Find(&values).Error; err != nil {
			return err
		}
		if len(values) != len(publicIDs) {
			return ErrInvalidAsset
		}
		now := time.Now()
		for _, value := range values {
			if value.OwnerUserID != ownerID || value.Purpose != purpose || value.Status != StatusApproved || value.DeleteRequestedAt != nil || value.ReferenceType != "" {
				return ErrInvalidAsset
			}
			if value.ReservationToken != "" && value.ReservationExpiresAt != nil && value.ReservationExpiresAt.After(now) {
				return ErrAlreadyBound
			}
		}
		return tx.Model(&Asset{}).Where("public_id IN ?", publicIDs).Updates(map[string]any{"reservation_token": token, "reservation_expires_at": expires}).Error
	})
	return values, err
}

func (r *GormRepository) Commit(ctx context.Context, ownerID uint, token, referenceType string, referenceID uint64, orderedIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for index, publicID := range orderedIDs {
			var value Asset
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("public_id = ?", publicID).First(&value).Error; err != nil {
				return err
			}
			result := tx.Model(&Asset{}).Where("id = ? AND owner_user_id = ? AND reservation_token = ? AND status = ? AND reference_type = ''", value.ID, ownerID, token, StatusApproved).
				Updates(map[string]any{"reference_type": referenceType, "reference_id": referenceID, "reference_sort": index, "reservation_token": "", "reservation_expires_at": nil})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrInvalidState
			}
			if err := appendAudit(tx, value, value.Status, value.Status, "reference_bound", "owner", ownerID, referenceType); err != nil {
				return err
			}
		}
		// Replacing a product image set or avatar is atomic inside the media
		// boundary: once the new references bind, superseded objects are queued
		// for storage deletion in the same transaction.
		var superseded []Asset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("reference_type = ? AND reference_id = ? AND public_id NOT IN ? AND delete_requested_at IS NULL", referenceType, referenceID, orderedIDs).
			Find(&superseded).Error; err != nil {
			return err
		}
		if len(superseded) == 0 {
			return nil
		}
		now := time.Now()
		ids := make([]uint64, 0, len(superseded))
		for _, value := range superseded {
			ids = append(ids, value.ID)
		}
		if err := tx.Model(&Asset{}).Where("id IN ?", ids).Updates(map[string]any{"status": StatusDeleted, "delete_requested_at": now}).Error; err != nil {
			return err
		}
		for _, value := range superseded {
			if err := appendAudit(tx, value, value.Status, StatusDeleted, "reference_replaced", "owner", ownerID, referenceType); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *GormRepository) FindActiveReferenceSet(ctx context.Context, ownerID uint, referenceType string, referenceID uint64) ([]Asset, error) {
	var values []Asset
	err := r.db.WithContext(ctx).
		Where(
			"owner_user_id = ? AND reference_type = ? AND reference_id = ? AND status = ? AND delete_requested_at IS NULL AND storage_deleted_at IS NULL",
			ownerID, referenceType, referenceID, StatusApproved,
		).
		Order("reference_sort ASC").
		Order("id ASC").
		Find(&values).Error
	return values, err
}

func (r *GormRepository) CancelReservation(ctx context.Context, ownerID uint, token string) error {
	return r.db.WithContext(ctx).Model(&Asset{}).Where("owner_user_id = ? AND reservation_token = ? AND reference_type = ''", ownerID, token).
		Updates(map[string]any{"reservation_token": "", "reservation_expires_at": nil}).Error
}

func (r *GormRepository) RequestDeleteOwned(ctx context.Context, publicID string, ownerID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var value Asset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("public_id = ? AND owner_user_id = ? AND reference_type = ''", publicID, ownerID).First(&value).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrForbidden
			}
			return err
		}
		if value.DeleteRequestedAt != nil && value.Status == StatusDeleted {
			return nil
		}
		now := time.Now()
		if err := tx.Model(&Asset{}).Where("id = ?", value.ID).Updates(map[string]any{"status": StatusDeleted, "delete_requested_at": now, "reservation_token": "", "reservation_expires_at": nil}).Error; err != nil {
			return err
		}
		return appendAudit(tx, value, value.Status, StatusDeleted, "delete_requested", "owner", ownerID, "unbound media deletion requested")
	})
}

func (r *GormRepository) RequestDeleteReference(ctx context.Context, referenceType string, referenceID uint64, keep []uint64) error {
	return dbtx.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("reference_type = ? AND reference_id = ? AND delete_requested_at IS NULL", referenceType, referenceID)
		if len(keep) > 0 {
			query = query.Where("id NOT IN ?", keep)
		}
		var values []Asset
		if err := query.Find(&values).Error; err != nil || len(values) == 0 {
			return err
		}
		ids := make([]uint64, 0, len(values))
		for _, value := range values {
			ids = append(ids, value.ID)
		}
		now := time.Now()
		if err := tx.Model(&Asset{}).Where("id IN ?", ids).Updates(map[string]any{"status": StatusDeleted, "delete_requested_at": now}).Error; err != nil {
			return err
		}
		for _, value := range values {
			if err := appendAudit(tx, value, value.Status, StatusDeleted, "reference_deleted", "business", 0, referenceType); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *GormRepository) PendingReviews(ctx context.Context, limit int, now time.Time) ([]Asset, error) {
	var values []Asset
	err := r.db.WithContext(ctx).Where("status IN ? AND review_trace_id IS NULL AND review_available_at <= ?", []Status{StatusUploaded, StatusSubmitted, StatusReviewing}, now).
		Order("review_available_at ASC, id ASC").Limit(limit).Find(&values).Error
	return values, err
}

func (r *GormRepository) PendingDeletes(ctx context.Context, limit int) ([]Asset, error) {
	var values []Asset
	err := r.db.WithContext(ctx).Where("delete_requested_at IS NOT NULL AND storage_deleted_at IS NULL").Order("delete_requested_at ASC, id ASC").Limit(limit).Find(&values).Error
	return values, err
}

func (r *GormRepository) ExpiredGarbage(ctx context.Context, limit int, cutoff time.Time) ([]Asset, error) {
	var values []Asset
	err := r.db.WithContext(ctx).Where("reference_type = '' AND delete_requested_at IS NULL AND ((status = ? AND upload_expires_at < ?) OR (status IN ? AND updated_at < ?))",
		StatusPendingUpload, cutoff, []Status{StatusUploaded, StatusSubmitted, StatusReviewing, StatusRejected, StatusApproved}, cutoff.Add(-garbageTTL)).Order("id ASC").Limit(limit).Find(&values).Error
	return values, err
}

func (r *GormRepository) MarkStorageDeleted(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		value, err := lockAsset(tx, id)
		if err != nil {
			return err
		}
		if value.StorageDeletedAt != nil {
			return nil
		}
		now := time.Now()
		if err := tx.Model(&Asset{}).Where("id = ? AND storage_deleted_at IS NULL", id).
			Updates(map[string]any{"storage_deleted_at": now, "status": StatusDeleted}).Error; err != nil {
			return err
		}
		return appendAudit(tx, value, value.Status, StatusDeleted, "storage_deleted", "system", 0, "COS object deleted")
	})
}

func lockAsset(tx *gorm.DB, id uint64) (Asset, error) {
	var value Asset
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&value, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Asset{}, ErrNotFound
	}
	return value, err
}

func appendAudit(tx *gorm.DB, value Asset, from, to Status, action, actorType string, actorID uint, detail string) error {
	return tx.Create(&AssetAudit{AssetID: value.ID, FromStatus: from, ToStatus: to, Action: action, ActorType: actorType, ActorID: actorID, Detail: detail}).Error
}

func statusIn(value Status, allowed []Status) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
