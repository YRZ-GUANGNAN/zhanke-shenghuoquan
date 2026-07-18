package trade

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"campus-circle/backend/internal/module/admin/review"
)

var (
	ErrNotFound          = errors.New("product not found")
	ErrForbidden         = errors.New("product does not belong to current user")
	ErrConflict          = errors.New("product changed concurrently or is not editable")
	ErrProductSold       = errors.New("sold product does not accept this operation")
	ErrSearchRequired    = errors.New("keyword search must use Meilisearch")
	ErrFavoriteInvariant = errors.New("product favorite count invariant violated")
)

type ListQuery struct {
	Keyword, Category string
	Status            ProductStatus
	Page, PageSize    int
}

type Repository interface {
	Create(context.Context, *Product) error
	Update(context.Context, *Product, []ProductImage) error
	Delete(context.Context, uint, uint) error
	Find(context.Context, uint, bool) (Product, error)
	FindForIndex(context.Context, uint) (Product, error)
	List(context.Context, ListQuery, bool) ([]Product, int64, error)
	ListByIDs(context.Context, []uint, ListQuery) ([]Product, error)
	ListSeller(context.Context, uint, int, int) ([]Product, int64, error)
	SetStatus(context.Context, uint, uint, ProductStatus, ProductStatus) error
	ToggleFavorite(context.Context, uint, uint, bool) (bool, error)
	IsFavorited(context.Context, uint, uint) (bool, error)
	SetFavorite(context.Context, uint, uint, bool, bool) (bool, error)
	SetReviewStatus(context.Context, uint, review.Status) error
	AdminTakedown(context.Context, uint) error
	QueueSearchRebuild(context.Context) error
	PendingSearchEvents(context.Context, int) ([]SearchOutbox, error)
	MarkSearchEventProcessed(context.Context, uint) error
	MarkSearchEventFailed(context.Context, uint, int, string) error
	Count(context.Context) (int64, int64, error)
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) SearchProducts(ctx context.Context, keyword string, page, size int) ([]Product, int64, error) {
	like, prefix := "%"+keyword+"%", keyword+"%"
	query := r.db.WithContext(ctx).Model(&Product{}).
		Where("review_status = ? AND status = ?", review.StatusApproved, StatusSelling).
		Where("title LIKE ? OR description LIKE ? OR category LIKE ?", like, like, like)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	values := make([]Product, 0, size)
	rank := clause.Expr{SQL: `CASE
		WHEN title = ? THEN 0
		WHEN title LIKE ? THEN 1
		WHEN title LIKE ? THEN 2
		WHEN category = ? THEN 3
		WHEN category LIKE ? THEN 4
		ELSE 5 END`, Vars: []any{keyword, prefix, like, keyword, like}}
	err := query.Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort ASC") }).
		Order(rank).Order("created_at DESC").Order("favorite_count DESC").Order("id DESC").
		Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) Create(ctx context.Context, value *Product) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		images := value.Images
		value.Images = nil
		if err := tx.Create(value).Error; err != nil {
			return err
		}
		for i := range images {
			images[i].ProductID, images[i].Sort = value.ID, i
		}
		if len(images) > 0 {
			if err := tx.Create(&images).Error; err != nil {
				return err
			}
		}
		value.Images = images
		return enqueueSearch(tx, value.ID, SearchDelete)
	})
}

func (r *GormRepository) Update(ctx context.Context, value *Product, images []ProductImage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Product{}).
			Where("id = ? AND seller_id = ? AND status <> ?", value.ID, value.SellerID, StatusSold).
			Updates(map[string]any{
				"title": value.Title, "description": value.Description, "category": value.Category,
				"price": value.Price, "original_price": value.OriginalPrice, "condition_label": value.Condition,
				"campus": value.Campus, "review_status": review.StatusSubmitted,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrConflict
		}
		if err := tx.Where("product_id = ?", value.ID).Delete(&ProductImage{}).Error; err != nil {
			return err
		}
		for i := range images {
			images[i].ProductID, images[i].Sort = value.ID, i
		}
		if len(images) > 0 {
			if err := tx.Create(&images).Error; err != nil {
				return err
			}
		}
		return enqueueSearch(tx, value.ID, SearchDelete)
	})
}

func (r *GormRepository) Delete(ctx context.Context, id, sellerID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("id = ? AND seller_id = ? AND status <> ?", id, sellerID, StatusSold).Delete(&Product{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrConflict
		}
		return enqueueSearch(tx, id, SearchDelete)
	})
}

func (r *GormRepository) Find(ctx context.Context, id uint, publicOnly bool) (Product, error) {
	var value Product
	query := r.db.WithContext(ctx).Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort ASC") })
	if publicOnly {
		query = query.Where("review_status = ?", review.StatusApproved)
	}
	err := query.First(&value, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Product{}, ErrNotFound
	}
	return value, err
}

func (r *GormRepository) FindForIndex(ctx context.Context, id uint) (Product, error) {
	var value Product
	err := r.db.WithContext(ctx).Unscoped().First(&value, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Product{}, ErrNotFound
	}
	return value, err
}

func (r *GormRepository) List(ctx context.Context, q ListQuery, publicOnly bool) ([]Product, int64, error) {
	if q.Keyword != "" {
		return nil, 0, ErrSearchRequired
	}
	var values []Product
	var total int64
	query := r.db.WithContext(ctx).Model(&Product{})
	if publicOnly {
		query = query.Where("review_status = ?", review.StatusApproved)
	}
	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}
	if q.Category != "" {
		query = query.Where("category = ?", q.Category)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort ASC") }).
		Order("created_at DESC, id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) ListByIDs(ctx context.Context, ids []uint, q ListQuery) ([]Product, error) {
	if len(ids) == 0 {
		return []Product{}, nil
	}
	var values []Product
	query := r.db.WithContext(ctx).Where("id IN ? AND review_status = ?", ids, review.StatusApproved)
	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}
	if q.Category != "" {
		query = query.Where("category = ?", q.Category)
	}
	if err := query.Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort ASC") }).Find(&values).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]Product, len(values))
	for _, value := range values {
		byID[value.ID] = value
	}
	ordered := make([]Product, 0, len(values))
	for _, id := range ids {
		if value, ok := byID[id]; ok {
			ordered = append(ordered, value)
		}
	}
	return ordered, nil
}

func (r *GormRepository) ListSeller(ctx context.Context, sellerID uint, page, size int) ([]Product, int64, error) {
	var values []Product
	var total int64
	query := r.db.WithContext(ctx).Model(&Product{}).Where("seller_id = ?", sellerID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort ASC") }).
		Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) SetStatus(ctx context.Context, id, sellerID uint, from, to ProductStatus) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Product{}).
			Where("id = ? AND seller_id = ? AND status = ? AND review_status = ?", id, sellerID, from, review.StatusApproved).
			Update("status", to)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrConflict
		}
		return enqueueSearch(tx, id, SearchUpsert)
	})
}

func (r *GormRepository) ToggleFavorite(ctx context.Context, productID, userID uint, allowCreate bool) (bool, error) {
	favorited := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var product Product
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&product, productID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		var value ProductFavorite
		err := tx.Where("product_id = ? AND user_id = ?", productID, userID).First(&value).Error
		if err == nil {
			if err := tx.Delete(&value).Error; err != nil {
				return err
			}
			if err := tx.Model(&Product{}).Where("id = ?", productID).
				UpdateColumn("favorite_count", gorm.Expr("GREATEST(favorite_count - 1, 0)")).Error; err != nil {
				return err
			}
			return enqueueSearch(tx, productID, SearchUpsert)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if !allowCreate || product.Status == StatusSold {
			return ErrProductSold
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ProductFavorite{ProductID: productID, UserID: userID})
		if result.Error != nil {
			return result.Error
		}
		favorited = true
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.Model(&Product{}).Where("id = ?", productID).
			UpdateColumn("favorite_count", gorm.Expr("favorite_count + 1")).Error; err != nil {
			return err
		}
		return enqueueSearch(tx, productID, SearchUpsert)
	})
	return favorited, err
}

func (r *GormRepository) IsFavorited(ctx context.Context, productID, userID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&ProductFavorite{}).
		Where("product_id = ? AND user_id = ?", productID, userID).Limit(1).Count(&count).Error
	return count > 0, err
}

func (r *GormRepository) SetFavorite(ctx context.Context, productID, userID uint, desired, allowCreate bool) (bool, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var product Product
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&product, productID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if product.ReviewStatus != review.StatusApproved {
			return ErrNotFound
		}
		if desired {
			var existing int64
			if err := tx.Model(&ProductFavorite{}).
				Where("product_id = ? AND user_id = ?", productID, userID).
				Limit(1).Count(&existing).Error; err != nil {
				return err
			}
			if existing > 0 {
				return nil
			}
			if !allowCreate || product.Status == StatusSold {
				return ErrProductSold
			}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&ProductFavorite{ProductID: productID, UserID: userID})
			if result.Error != nil || result.RowsAffected == 0 {
				return result.Error
			}
			if err := requireFavoriteUpdate(tx.Model(&Product{}).Where("id = ?", productID).
				UpdateColumn("favorite_count", gorm.Expr("favorite_count + 1"))); err != nil {
				return err
			}
			return enqueueSearch(tx, productID, SearchUpsert)
		}
		result := tx.Where("product_id = ? AND user_id = ?", productID, userID).Delete(&ProductFavorite{})
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		if err := requireFavoriteUpdate(tx.Model(&Product{}).Where("id = ? AND favorite_count > 0", productID).
			UpdateColumn("favorite_count", gorm.Expr("favorite_count - 1"))); err != nil {
			return err
		}
		return enqueueSearch(tx, productID, SearchUpsert)
	})
	return desired, err
}

func requireFavoriteUpdate(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrFavoriteInvariant
	}
	return nil
}

func (r *GormRepository) SetReviewStatus(ctx context.Context, id uint, status review.Status) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&Product{}).Where("id = ?", id).Update("review_status", status)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		operation := SearchDelete
		if status == review.StatusApproved {
			operation = SearchUpsert
		}
		return enqueueSearch(tx, id, operation)
	})
}

func (r *GormRepository) AdminTakedown(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var value Product
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&value, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if value.ReviewStatus == ReviewStatusTakenDown {
			return nil
		}
		if err := tx.Model(&Product{}).Where("id = ?", id).Update("review_status", ReviewStatusTakenDown).Error; err != nil {
			return err
		}
		return enqueueSearch(tx, id, SearchDelete)
	})
}

func enqueueSearch(tx *gorm.DB, productID uint, operation SearchOperation) error {
	return tx.Create(&SearchOutbox{ProductID: productID, Operation: operation, AvailableAt: time.Now()}).Error
}

func (r *GormRepository) QueueSearchRebuild(ctx context.Context) error {
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO trade_search_outbox(product_id, operation, attempts, available_at, created_at, updated_at)
		SELECT id, CASE WHEN review_status = 'approved' AND deleted_at IS NULL THEN 'upsert' ELSE 'delete' END,
		       0, NOW(3), NOW(3), NOW(3)
		FROM products`).Error
}

func (r *GormRepository) PendingSearchEvents(ctx context.Context, limit int) ([]SearchOutbox, error) {
	var values []SearchOutbox
	err := r.db.WithContext(ctx).Where("processed_at IS NULL AND available_at <= ?", time.Now()).
		Order("id ASC").Limit(limit).Find(&values).Error
	return values, err
}

func (r *GormRepository) MarkSearchEventProcessed(ctx context.Context, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&SearchOutbox{}).Where("id = ? AND processed_at IS NULL", id).
		Updates(map[string]any{"processed_at": &now, "last_error": ""}).Error
}

func (r *GormRepository) MarkSearchEventFailed(ctx context.Context, id uint, attempts int, message string) error {
	if len(message) > 512 {
		message = message[:512]
	}
	delay := time.Duration(1<<min(attempts, 8)) * time.Second
	return r.db.WithContext(ctx).Model(&SearchOutbox{}).Where("id = ? AND processed_at IS NULL").
		Updates(map[string]any{"attempts": gorm.Expr("attempts + 1"), "available_at": time.Now().Add(delay), "last_error": message}).Error
}

func (r *GormRepository) Count(ctx context.Context) (int64, int64, error) {
	var total, today int64
	if err := r.db.WithContext(ctx).Model(&Product{}).Count(&total).Error; err != nil {
		return 0, 0, err
	}
	start := time.Now().Truncate(24 * time.Hour)
	err := r.db.WithContext(ctx).Model(&Product{}).Where("created_at >= ?", start).Count(&today).Error
	return total, today, err
}
