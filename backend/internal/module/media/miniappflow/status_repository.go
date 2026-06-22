package miniappflow

import (
	"context"
	"errors"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
	"gorm.io/gorm"
)

type StatusRepository interface {
	FindStatus(context.Context, uint64, string) (StatusFact, error)
	ListStatus(context.Context, uint64, int, int) ([]StatusFact, int64, error)
}

type GormStatusRepository struct{ db *gorm.DB }

func NewStatusRepository(db *gorm.DB) *GormStatusRepository { return &GormStatusRepository{db: db} }

type statusRow struct {
	AssetID         uint64                      `gorm:"column:asset_id"`
	AssetPublicID   string                      `gorm:"column:asset_public_id"`
	OwnerID         uint64                      `gorm:"column:owner_id"`
	Purpose         coremedia.Purpose           `gorm:"column:purpose"`
	StorageStage    coremedia.StorageStage      `gorm:"column:storage_stage"`
	TechnicalStatus coremedia.TechnicalStatus   `gorm:"column:technical_status"`
	AssetReview     coremedia.ReviewStatus      `gorm:"column:asset_review"`
	TicketStatus    coremedia.TicketStatus      `gorm:"column:ticket_status"`
	TicketID        string                      `gorm:"column:ticket_id"`
	Projection      *coremedia.ProjectionStatus `gorm:"column:projection_status"`
	UpdatedAt       time.Time                   `gorm:"column:updated_at"`
}

func (r *GormStatusRepository) FindStatus(ctx context.Context, ownerID uint64, assetPublicID string) (StatusFact, error) {
	if ownerID == 0 || !coremedia.ValidULID(assetPublicID) {
		return StatusFact{}, ErrInvalidRequest
	}
	var row statusRow
	err := statusQuery(r.db.WithContext(ctx)).
		Where("a.owner_id = ? AND a.asset_public_id = ?", ownerID, assetPublicID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return StatusFact{}, coremedia.ErrNotFound
	}
	return factFromRow(row), err
}

func (r *GormStatusRepository) ListStatus(ctx context.Context, ownerID uint64, page, pageSize int) ([]StatusFact, int64, error) {
	if ownerID == 0 || page < 1 || pageSize < 1 || pageSize > 50 {
		return nil, 0, ErrInvalidRequest
	}
	var total int64
	if err := r.db.WithContext(ctx).Model(&coremedia.Asset{}).Where("owner_id = ?", ownerID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []statusRow
	err := statusQuery(r.db.WithContext(ctx)).Where("a.owner_id = ?", ownerID).
		Order("a.updated_at DESC, a.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	values := make([]StatusFact, 0, len(rows))
	for _, row := range rows {
		values = append(values, factFromRow(row))
	}
	return values, total, nil
}

func statusQuery(db *gorm.DB) *gorm.DB {
	return db.Table("media_assets AS a").
		Select(`a.id AS asset_id, a.asset_public_id, a.owner_id, a.purpose, a.storage_stage,
			a.technical_status, a.review_status AS asset_review, a.updated_at,
			t.ticket_id, t.status AS ticket_status, p.status AS projection_status`).
		Joins("JOIN media_upload_tickets AS t ON t.asset_id = a.id").
		Joins("LEFT JOIN media_review_projections AS p ON p.asset_id = a.id")
}

func factFromRow(row statusRow) StatusFact {
	return StatusFact{
		AssetID: row.AssetID, AssetPublicID: row.AssetPublicID, OwnerID: row.OwnerID,
		Purpose: row.Purpose, StorageStage: row.StorageStage, TechnicalStatus: row.TechnicalStatus,
		AssetReview: row.AssetReview, TicketStatus: row.TicketStatus, TicketID: row.TicketID, Projection: row.Projection,
		UpdatedAt: row.UpdatedAt,
	}
}
