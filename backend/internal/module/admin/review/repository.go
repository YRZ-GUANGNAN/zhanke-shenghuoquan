package review

import (
	"context"
	"errors"

	"campus-circle/backend/internal/pkg/dbtx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrNotFound = errors.New("review task not found")

type Repository interface {
	Submit(context.Context, *Task) error
	EnsureSubmitted(context.Context, *Task) error
	Find(context.Context, uint) (Task, error)
	FindForUpdate(context.Context, uint) (Task, error)
	Save(context.Context, *Task) error
	List(context.Context, Status, int, int) ([]Task, int64, error)
	CountPending(context.Context) (int64, error)
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
func (r *GormRepository) Submit(ctx context.Context, value *Task) error {
	err := dbtx.DB(ctx, r.db).Where("object_type = ? AND object_id = ?", value.ObjectType, value.ObjectID).Assign(map[string]any{
		"submitter_id": value.SubmitterID, "risk": value.Risk, "status": StatusSubmitted,
		"decision_source": DecisionSourceManual, "reason": "", "operator_id": 0,
	}).FirstOrCreate(value).Error
	return err
}
func (r *GormRepository) EnsureSubmitted(ctx context.Context, value *Task) error {
	candidate := *value
	if err := dbtx.DB(ctx, r.db).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "object_type"}, {Name: "object_id"}},
		DoNothing: true,
	}).Create(&candidate).Error; err != nil {
		return err
	}
	existing, err := r.findByTarget(ctx, value.ObjectType, value.ObjectID)
	if err != nil {
		return err
	}
	*value = existing
	return nil
}
func (r *GormRepository) Find(ctx context.Context, id uint) (Task, error) {
	var value Task
	err := dbtx.DB(ctx, r.db).First(&value, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Task{}, ErrNotFound
	}
	return value, err
}
func (r *GormRepository) FindForUpdate(ctx context.Context, id uint) (Task, error) {
	var value Task
	err := dbtx.DB(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).First(&value, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Task{}, ErrNotFound
	}
	return value, err
}
func (r *GormRepository) findByTarget(ctx context.Context, objectType string, objectID uint) (Task, error) {
	var value Task
	err := dbtx.DB(ctx, r.db).Where("object_type = ? AND object_id = ?", objectType, objectID).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Task{}, ErrNotFound
	}
	return value, err
}
func (r *GormRepository) Save(ctx context.Context, value *Task) error {
	return dbtx.DB(ctx, r.db).Save(value).Error
}
func (r *GormRepository) List(ctx context.Context, status Status, page, size int) ([]Task, int64, error) {
	var values []Task
	var total int64
	query := dbtx.DB(ctx, r.db).Model(&Task{})
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
	err := dbtx.DB(ctx, r.db).Model(&Task{}).Where("status IN ?", []Status{StatusSubmitted, StatusReviewing}).Count(&count).Error
	return count, err
}
