package admin

import (
	"campus-circle/backend/internal/module/admin/review"
	"context"
	"errors"
	"gorm.io/gorm"
)

var ErrReportNotFound = errors.New("report not found")

type Repository interface {
	CreateReport(context.Context, *Report) error
	ListReports(context.Context, int, int) ([]Report, int64, error)
	SetReportStatus(context.Context, uint, review.Status) error
}
type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }
func (r *GormRepository) CreateReport(ctx context.Context, v *Report) error {
	return r.db.WithContext(ctx).Create(v).Error
}
func (r *GormRepository) ListReports(ctx context.Context, page, size int) ([]Report, int64, error) {
	var values []Report
	var total int64
	query := r.db.WithContext(ctx).Model(&Report{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}
func (r *GormRepository) SetReportStatus(ctx context.Context, id uint, status review.Status) error {
	result := r.db.WithContext(ctx).Model(&Report{}).Where("id = ?", id).Update("review_status", status)
	if result.Error == nil && result.RowsAffected == 0 {
		return ErrReportNotFound
	}
	return result.Error
}
