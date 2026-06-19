package navigation

import (
	"context"
	"errors"
	"gorm.io/gorm"
)

var ErrNotFound = errors.New("navigation entry not found")

type Repository interface {
	Tree(context.Context, bool) ([]Category, error)
	CreateCategory(context.Context, *Category) error
	UpdateCategory(context.Context, *Category) error
	DeleteCategory(context.Context, uint) error
	CreateItem(context.Context, *Item) error
	UpdateItem(context.Context, *Item) error
	DeleteItem(context.Context, uint) error
}
type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }
func (r *GormRepository) Tree(ctx context.Context, enabledOnly bool) ([]Category, error) {
	var values []Category
	query := r.db.WithContext(ctx)
	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}
	err := query.Preload("Items", func(db *gorm.DB) *gorm.DB {
		if enabledOnly {
			db = db.Where("enabled = ?", true)
		}
		return db.Order("sort ASC, id ASC")
	}).Order("sort ASC, id ASC").Find(&values).Error
	return values, err
}
func (r *GormRepository) CreateCategory(ctx context.Context, v *Category) error {
	return r.db.WithContext(ctx).Create(v).Error
}
func (r *GormRepository) UpdateCategory(ctx context.Context, v *Category) error {
	result := r.db.WithContext(ctx).Model(&Category{}).Where("id = ?", v.ID).Updates(v)
	if result.Error == nil && result.RowsAffected == 0 {
		return ErrNotFound
	}
	return result.Error
}
func (r *GormRepository) DeleteCategory(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("category_id = ?", id).Delete(&Item{}).Error; err != nil {
			return err
		}
		result := tx.Delete(&Category{}, id)
		if result.Error == nil && result.RowsAffected == 0 {
			return ErrNotFound
		}
		return result.Error
	})
}
func (r *GormRepository) CreateItem(ctx context.Context, v *Item) error {
	return r.db.WithContext(ctx).Create(v).Error
}
func (r *GormRepository) UpdateItem(ctx context.Context, v *Item) error {
	result := r.db.WithContext(ctx).Model(&Item{}).Where("id = ?", v.ID).Updates(v)
	if result.Error == nil && result.RowsAffected == 0 {
		return ErrNotFound
	}
	return result.Error
}
func (r *GormRepository) DeleteItem(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&Item{}, id)
	if result.Error == nil && result.RowsAffected == 0 {
		return ErrNotFound
	}
	return result.Error
}
