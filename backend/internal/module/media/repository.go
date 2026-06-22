package media

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNotFound         = errors.New("media record not found")
	ErrConcurrentUpdate = errors.New("media record changed concurrently")
)

// Repository is the only boundary allowed to access Media Domain tables.
// It deliberately exposes no A/B/C model or database handle.
type Repository interface {
	CreateAsset(context.Context, *Asset) error
	FindAsset(context.Context, string) (Asset, error)
	CASAssetTechnicalStatus(context.Context, uint64, uint64, TechnicalStatus, TechnicalStatus) error
	CASAssetStorageStage(context.Context, uint64, uint64, StorageStage, StorageStage) error
	CASAssetReviewProjection(context.Context, uint64, uint64, uint64, ReviewStatus, time.Time) error

	CreateReference(context.Context, *AssetReference) error
	FindReference(context.Context, uint64) (AssetReference, error)
	CASReferenceStatus(context.Context, uint64, uint64, ReferenceStatus, ReferenceStatus) error
	RebuildActiveRefCount(context.Context, uint64) (uint64, error)

	CreateUploadTicket(context.Context, *UploadTicket) error
	FindUploadTicket(context.Context, string) (UploadTicket, error)
	CASTicketStatus(context.Context, string, uint64, TicketStatus, TicketStatus) error

	CreateQuotaReservation(context.Context, *QuotaReservation) error
	FindQuotaReservation(context.Context, string) (QuotaReservation, error)
	CASQuotaReservationStatus(context.Context, string, uint64, ReservationStatus, ReservationStatus) error

	AppendOutbox(context.Context, *OutboxEvent) error
	PendingOutbox(context.Context, int) ([]OutboxEvent, error)
	MarkOutboxProcessed(context.Context, string, uint64, time.Time) error
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) CreateAsset(ctx context.Context, value *Asset) error {
	return r.db.WithContext(ctx).Create(value).Error
}

func (r *GormRepository) FindAsset(ctx context.Context, publicID string) (Asset, error) {
	var value Asset
	err := r.db.WithContext(ctx).Where("asset_public_id = ?", publicID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) CASAssetTechnicalStatus(ctx context.Context, id, version uint64, from, to TechnicalStatus) error {
	return casUpdate(r.db.WithContext(ctx).Model(&Asset{}).
		Where("id = ? AND row_version = ? AND technical_status = ?", id, version, from).
		Updates(map[string]any{"technical_status": to, "row_version": gorm.Expr("row_version + 1")}))
}

func (r *GormRepository) CASAssetStorageStage(ctx context.Context, id, version uint64, from, to StorageStage) error {
	return casUpdate(r.db.WithContext(ctx).Model(&Asset{}).
		Where("id = ? AND row_version = ? AND storage_stage = ?", id, version, from).
		Updates(map[string]any{"storage_stage": to, "row_version": gorm.Expr("row_version + 1")}))
}

func (r *GormRepository) CASAssetReviewProjection(ctx context.Context, id, version, decisionVersion uint64, decision ReviewStatus, decidedAt time.Time) error {
	return casUpdate(r.db.WithContext(ctx).Model(&Asset{}).
		Where("id = ? AND row_version = ? AND applied_decision_version < ?", id, version, decisionVersion).
		Updates(map[string]any{
			"review_status": decision, "projected_decision": decision,
			"applied_decision_version": decisionVersion, "projected_decided_at": decidedAt,
			"row_version": gorm.Expr("row_version + 1"),
		}))
}

func (r *GormRepository) CreateReference(ctx context.Context, value *AssetReference) error {
	return r.db.WithContext(ctx).Create(value).Error
}

func (r *GormRepository) FindReference(ctx context.Context, id uint64) (AssetReference, error) {
	var value AssetReference
	err := r.db.WithContext(ctx).First(&value, id).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) CASReferenceStatus(ctx context.Context, id, version uint64, from, to ReferenceStatus) error {
	return casUpdate(r.db.WithContext(ctx).Model(&AssetReference{}).
		Where("id = ? AND row_version = ? AND status = ?", id, version, from).
		Updates(map[string]any{"status": to, "row_version": gorm.Expr("row_version + 1")}))
}

// RebuildActiveRefCount derives the cache exclusively from reference facts.
// GC must still inspect reference rows and must never trust this cache alone.
func (r *GormRepository) RebuildActiveRefCount(ctx context.Context, assetID uint64) (uint64, error) {
	var count int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var asset Asset
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id", "row_version").First(&asset, assetID).Error; err != nil {
			return normalizeNotFound(err)
		}
		if err := tx.Model(&AssetReference{}).Where("asset_id = ? AND status = ?", assetID, ReferenceActive).Count(&count).Error; err != nil {
			return err
		}
		return tx.Model(&Asset{}).Where("id = ?", assetID).Updates(map[string]any{
			"active_ref_count": uint64(count), "row_version": gorm.Expr("row_version + 1"),
		}).Error
	})
	return uint64(count), err
}

func (r *GormRepository) CreateUploadTicket(ctx context.Context, value *UploadTicket) error {
	return r.db.WithContext(ctx).Create(value).Error
}

func (r *GormRepository) FindUploadTicket(ctx context.Context, ticketID string) (UploadTicket, error) {
	var value UploadTicket
	err := r.db.WithContext(ctx).Where("ticket_id = ?", ticketID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) CASTicketStatus(ctx context.Context, ticketID string, version uint64, from, to TicketStatus) error {
	updates := map[string]any{"status": to, "row_version": gorm.Expr("row_version + 1")}
	if from == TicketCreated && to == TicketAuthorizationIssued {
		updates["credential_issue_count"] = gorm.Expr("credential_issue_count + 1")
	}
	return casUpdate(r.db.WithContext(ctx).Model(&UploadTicket{}).
		Where("ticket_id = ? AND row_version = ? AND status = ?", ticketID, version, from).
		Updates(updates))
}

func (r *GormRepository) CreateQuotaReservation(ctx context.Context, value *QuotaReservation) error {
	return r.db.WithContext(ctx).Create(value).Error
}

func (r *GormRepository) FindQuotaReservation(ctx context.Context, reservationID string) (QuotaReservation, error) {
	var value QuotaReservation
	err := r.db.WithContext(ctx).Where("reservation_id = ?", reservationID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) CASQuotaReservationStatus(ctx context.Context, reservationID string, version uint64, from, to ReservationStatus) error {
	return casUpdate(r.db.WithContext(ctx).Model(&QuotaReservation{}).
		Where("reservation_id = ? AND row_version = ? AND status = ?", reservationID, version, from).
		Updates(map[string]any{"status": to, "row_version": gorm.Expr("row_version + 1")}))
}

func (r *GormRepository) AppendOutbox(ctx context.Context, value *OutboxEvent) error {
	return r.db.WithContext(ctx).Create(value).Error
}

func (r *GormRepository) PendingOutbox(ctx context.Context, limit int) ([]OutboxEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var values []OutboxEvent
	err := r.db.WithContext(ctx).Where("processed_at IS NULL").Order("created_at ASC, event_id ASC").Limit(limit).Find(&values).Error
	return values, err
}

func (r *GormRepository) MarkOutboxProcessed(ctx context.Context, eventID string, version uint64, processedAt time.Time) error {
	return casUpdate(r.db.WithContext(ctx).Model(&OutboxEvent{}).
		Where("event_id = ? AND row_version = ? AND processed_at IS NULL", eventID, version).
		Updates(map[string]any{"processed_at": processedAt, "row_version": gorm.Expr("row_version + 1")}))
}

func casUpdate(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrConcurrentUpdate
	}
	return nil
}

func normalizeNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
