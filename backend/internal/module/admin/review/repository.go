package review

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("review task not found")

type Repository interface {
	Submit(context.Context, *Task) error
	Find(context.Context, uint) (Task, error)
	Save(context.Context, *Task) error
	List(context.Context, Status, int, int) ([]Task, int64, error)
	CountPending(context.Context) (int64, error)
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }
func (r *GormRepository) Submit(ctx context.Context, value *Task) error {
	err := r.db.WithContext(ctx).Where("object_type = ? AND object_id = ?", value.ObjectType, value.ObjectID).Assign(map[string]any{
		"submitter_id": value.SubmitterID, "risk": value.Risk, "status": StatusSubmitted, "reason": "", "operator_id": 0,
	}).FirstOrCreate(value).Error
	return err
}
func (r *GormRepository) Find(ctx context.Context, id uint) (Task, error) {
	var value Task
	err := r.db.WithContext(ctx).First(&value, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Task{}, ErrNotFound
	}
	return value, err
}
func (r *GormRepository) Save(ctx context.Context, value *Task) error {
	return r.db.WithContext(ctx).Save(value).Error
}
func (r *GormRepository) List(ctx context.Context, status Status, page, size int) ([]Task, int64, error) {
	var values []Task
	var total int64
	query := r.db.WithContext(ctx).Model(&Task{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}
func (r *GormRepository) CountPending(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&Task{}).Where("status IN ?", []Status{StatusSubmitted, StatusReviewing}).Count(&count).Error
	return count, err
}
