package admin

import (
	"context"
	"errors"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/pkg/dbtx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrReportNotFound = errors.New("report not found")

type Repository interface {
	CreateReportOrReplay(context.Context, *Report) (bool, error)
	ListReports(context.Context, int, int) ([]Report, int64, error)
	SetReportStatus(context.Context, uint, review.Status) error
	WithinTransaction(context.Context, func(context.Context, Repository) error) error
}
type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }
func (r *GormRepository) WithinTransaction(ctx context.Context, fn func(context.Context, Repository) error) error {
	if tx, ok := dbtx.FromContext(ctx); ok {
		return fn(ctx, &GormRepository{db: tx})
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := dbtx.WithDB(ctx, tx)
		return fn(txCtx, &GormRepository{db: tx})
	})
}
func (r *GormRepository) CreateReportOrReplay(ctx context.Context, v *Report) (bool, error) {
	if v.ClientRequestID == nil {
		return false, errors.New("report client request id is required")
	}
	candidate := *v
	result := dbtx.DB(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "reporter_id"}, {Name: "client_request_id"}},
		DoNothing: true,
	}).Create(&candidate)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 1 {
		*v = candidate
		return true, nil
	}
	var existing Report
	if err := dbtx.DB(ctx, r.db).
		Where("reporter_id = ? AND client_request_id = ?", v.ReporterID, *v.ClientRequestID).
		First(&existing).Error; err != nil {
		return false, err
	}
	*v = existing
	return false, nil
}
func (r *GormRepository) ListReports(ctx context.Context, page, size int) ([]Report, int64, error) {
	var values []Report
	var total int64
	query := dbtx.DB(ctx, r.db).Model(&Report{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}
func (r *GormRepository) SetReportStatus(ctx context.Context, id uint, status review.Status) error {
	result := dbtx.DB(ctx, r.db).Model(&Report{}).Where("id = ?", id).Update("review_status", status)
	if result.Error == nil && result.RowsAffected == 0 {
		return ErrReportNotFound
	}
	return result.Error
}
