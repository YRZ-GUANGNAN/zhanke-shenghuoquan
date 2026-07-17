package navigation

import (
	"context"
	"errors"
	"strings"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var (
	ErrNotFound        = errors.New("navigation entry not found")
	ErrDuplicateTarget = errors.New("duplicate navigation target")
)

type Repository interface {
	Tree(context.Context, bool) ([]Category, error)
	FindCategory(context.Context, uint) (Category, bool, error)
	FindCategoryBySystemKey(context.Context, string) (Category, bool, error)
	FindItem(context.Context, uint) (Item, bool, error)
	ListItemsByCategory(context.Context, uint) ([]Item, error)
	DuplicateTarget(context.Context, uint, string, string, uint) (bool, error)
	CreateCategory(context.Context, *Category) error
	UpdateCategory(context.Context, uint, map[string]any) error
	DeleteCategory(context.Context, uint) error
	CreateItem(context.Context, *Item) error
	UpdateItem(context.Context, uint, map[string]any) error
	DeleteItem(context.Context, uint) error
	UpdateItemInCategory(context.Context, uint, uint, map[string]any) error
	DeleteItemInCategory(context.Context, uint, uint) error
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) Tree(ctx context.Context, enabledOnly bool) ([]Category, error) {
	values := make([]Category, 0)
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

func (r *GormRepository) FindCategory(ctx context.Context, id uint) (Category, bool, error) {
	var value Category
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Category{}, false, nil
	}
	return value, err == nil, err
}

func (r *GormRepository) FindCategoryBySystemKey(ctx context.Context, systemKey string) (Category, bool, error) {
	var value Category
	err := r.db.WithContext(ctx).Where("system_key = ?", systemKey).Take(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Category{}, false, nil
	}
	return value, err == nil, err
}

func (r *GormRepository) FindItem(ctx context.Context, id uint) (Item, bool, error) {
	var value Item
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Item{}, false, nil
	}
	return value, err == nil, err
}

func (r *GormRepository) ListItemsByCategory(ctx context.Context, categoryID uint) ([]Item, error) {
	values := make([]Item, 0)
	err := r.db.WithContext(ctx).
		Where("category_id = ?", categoryID).
		Order("sort ASC, id ASC").
		Find(&values).Error
	return values, err
}

func (r *GormRepository) DuplicateTarget(ctx context.Context, categoryID uint, appID, path string, excludeID uint) (bool, error) {
	query := r.db.WithContext(ctx).Model(&Item{}).
		Where("category_id = ? AND app_id = ? AND path = ?", categoryID, appID, path)
	if excludeID != 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *GormRepository) CreateCategory(ctx context.Context, value *Category) error {
	return mapNavigationWriteError(r.db.WithContext(ctx).Create(value).Error)
}

func (r *GormRepository) UpdateCategory(ctx context.Context, id uint, changes map[string]any) error {
	filtered := filterNavigationChanges(changes, categoryUpdateColumns)
	if len(filtered) == 0 {
		return r.categoryExists(ctx, id)
	}
	result := r.db.WithContext(ctx).Model(&Category{}).
		Where("id = ?", id).
		Updates(filtered)
	return r.resolveCategoryUpdateResult(ctx, result, id)
}

func (r *GormRepository) DeleteCategory(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := mapNavigationWriteError(tx.Where("category_id = ?", id).Delete(&Item{}).Error); err != nil {
			return err
		}
		result := tx.Delete(&Category{}, id)
		if result.Error != nil {
			return mapNavigationWriteError(result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (r *GormRepository) CreateItem(ctx context.Context, value *Item) error {
	if value.OperatingStatus == "" {
		value.OperatingStatus = OperatingStatusOpen
	}
	return mapNavigationWriteError(r.db.WithContext(ctx).Create(value).Error)
}

func (r *GormRepository) UpdateItem(ctx context.Context, id uint, changes map[string]any) error {
	filtered := filterNavigationChanges(changes, itemUpdateColumns)
	if len(filtered) == 0 {
		return r.itemExists(ctx, id)
	}
	result := r.db.WithContext(ctx).Model(&Item{}).
		Where("id = ?", id).
		Updates(filtered)
	return r.resolveItemUpdateResult(ctx, result, id, 0, false)
}

func (r *GormRepository) DeleteItem(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&Item{})
	if result.Error != nil {
		return mapNavigationWriteError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormRepository) UpdateItemInCategory(ctx context.Context, id, categoryID uint, changes map[string]any) error {
	filtered := filterNavigationChanges(changes, scopedItemUpdateColumns)
	if len(filtered) == 0 {
		return r.itemExistsInCategory(ctx, id, categoryID)
	}
	result := r.db.WithContext(ctx).Model(&Item{}).
		Where("id = ? AND category_id = ?", id, categoryID).
		Updates(filtered)
	return r.resolveItemUpdateResult(ctx, result, id, categoryID, true)
}

func (r *GormRepository) DeleteItemInCategory(ctx context.Context, id, categoryID uint) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND category_id = ?", id, categoryID).
		Delete(&Item{})
	if result.Error != nil {
		return mapNavigationWriteError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormRepository) resolveCategoryUpdateResult(ctx context.Context, result *gorm.DB, id uint) error {
	if result.Error != nil {
		return mapNavigationWriteError(result.Error)
	}
	if result.RowsAffected > 0 {
		return nil
	}
	return r.categoryExists(ctx, id)
}

func (r *GormRepository) resolveItemUpdateResult(ctx context.Context, result *gorm.DB, id, categoryID uint, scoped bool) error {
	if result.Error != nil {
		return mapNavigationWriteError(result.Error)
	}
	if result.RowsAffected > 0 {
		return nil
	}
	if scoped {
		return r.itemExistsInCategory(ctx, id, categoryID)
	}
	return r.itemExists(ctx, id)
}

func (r *GormRepository) categoryExists(ctx context.Context, id uint) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Category{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormRepository) itemExists(ctx context.Context, id uint) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Item{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormRepository) itemExistsInCategory(ctx context.Context, id, categoryID uint) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Item{}).
		Where("id = ? AND category_id = ?", id, categoryID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func mapNavigationWriteError(err error) error {
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 && duplicateTargetIndexMessage(mysqlErr.Message) {
		return ErrDuplicateTarget
	}
	return err
}

func duplicateTargetIndexMessage(message string) bool {
	const indexName = "uq_navigation_items_category_target"
	const marker = "for key "
	markerIndex := strings.LastIndex(strings.ToLower(message), marker)
	if markerIndex < 0 {
		return false
	}
	keyPart := strings.TrimSpace(message[markerIndex+len(marker):])
	if keyPart == "" {
		return false
	}

	key := keyPart
	if quote := keyPart[0]; quote == '\'' || quote == '`' || quote == '"' {
		closingIndex := strings.IndexByte(keyPart[1:], quote)
		if closingIndex < 0 {
			return false
		}
		key = keyPart[1 : closingIndex+1]
	} else if fields := strings.Fields(keyPart); len(fields) > 0 {
		key = fields[0]
	}

	parts := strings.Split(key, ".")
	if !strings.EqualFold(parts[len(parts)-1], indexName) {
		return false
	}
	return len(parts) == 1 || strings.EqualFold(parts[len(parts)-2], "navigation_items")
}

var categoryUpdateColumns = map[string]struct{}{
	"name":    {},
	"sort":    {},
	"enabled": {},
}

var itemUpdateColumns = map[string]struct{}{
	"category_id":      {},
	"title":            {},
	"subtitle":         {},
	"icon_text":        {},
	"app_id":           {},
	"path":             {},
	"color":            {},
	"sort":             {},
	"enabled":          {},
	"operating_status": {},
}

var scopedItemUpdateColumns = map[string]struct{}{
	"title":            {},
	"subtitle":         {},
	"icon_text":        {},
	"app_id":           {},
	"path":             {},
	"color":            {},
	"sort":             {},
	"enabled":          {},
	"operating_status": {},
}

func filterNavigationChanges(changes map[string]any, allowed map[string]struct{}) map[string]any {
	filtered := make(map[string]any, len(changes))
	for column, value := range changes {
		if _, ok := allowed[column]; ok {
			filtered[column] = value
		}
	}
	return filtered
}

var _ Repository = (*GormRepository)(nil)
