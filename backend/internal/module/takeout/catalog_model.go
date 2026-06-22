package takeout

import (
	"strings"
	"time"
	"unicode/utf8"
)

type DishStatus string

const (
	DishDraft    DishStatus = "draft"
	DishOnShelf  DishStatus = "on_shelf"
	DishSoldOut  DishStatus = "sold_out"
	DishOffShelf DishStatus = "off_shelf"
	DishRemoved  DishStatus = "removed"
)

func (s DishStatus) Valid() bool {
	switch s {
	case DishDraft, DishOnShelf, DishSoldOut, DishOffShelf, DishRemoved:
		return true
	default:
		return false
	}
}

func (s DishStatus) Terminal() bool { return s == DishRemoved }

type DishCategory struct {
	ID               uint64    `gorm:"column:id;primaryKey"`
	CategoryPublicID string    `gorm:"column:category_public_id;size:26;not null;uniqueIndex:uq_takeout_category_public_id"`
	MerchantPublicID string    `gorm:"column:merchant_public_id;size:26;not null;index:idx_takeout_category_merchant"`
	Name             string    `gorm:"column:name;size:64;not null"`
	SortOrder        int32     `gorm:"column:sort_order;not null"`
	RowVersion       uint64    `gorm:"column:row_version;not null"`
	CreatedAt        time.Time `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time `gorm:"column:updated_at;not null"`
}

func (DishCategory) TableName() string { return "takeout_dish_categories" }

func (c DishCategory) ValidateNew() error {
	if !ValidULID(c.CategoryPublicID) || !ValidULID(c.MerchantPublicID) ||
		strings.TrimSpace(c.Name) == "" || utf8.RuneCountInString(c.Name) > 64 ||
		c.RowVersion != 1 || c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	return nil
}

type Dish struct {
	ID                 uint64     `gorm:"column:id;primaryKey"`
	DishPublicID       string     `gorm:"column:dish_public_id;size:26;not null;uniqueIndex:uq_takeout_dish_public_id"`
	MerchantPublicID   string     `gorm:"column:merchant_public_id;size:26;not null;index:idx_takeout_dish_merchant"`
	CategoryPublicID   *string    `gorm:"column:category_public_id;size:26"`
	Name               string     `gorm:"column:name;size:128;not null"`
	Description        string     `gorm:"column:description;size:512;not null"`
	PriceCents         uint64     `gorm:"column:price_cents;not null"`
	ImageAssetPublicID *string    `gorm:"column:image_asset_public_id;size:26"`
	Status             DishStatus `gorm:"column:status;size:24;not null;index:idx_takeout_dish_status"`
	SortOrder          int32      `gorm:"column:sort_order;not null"`
	RowVersion         uint64     `gorm:"column:row_version;not null"`
	CreatedAt          time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;not null"`
}

func (Dish) TableName() string { return "takeout_dishes" }

func (d Dish) ValidateNew() error {
	if !ValidULID(d.DishPublicID) || !ValidULID(d.MerchantPublicID) || !validDishDetails(dishDetailsFromDish(d)) ||
		d.Status != DishDraft || d.RowVersion != 1 || d.CreatedAt.IsZero() || d.UpdatedAt.IsZero() {
		return ErrInvalidRecord
	}
	return nil
}

type dishDetails struct {
	CategoryPublicID   *string
	Name               string
	Description        string
	PriceCents         uint64
	ImageAssetPublicID *string
	SortOrder          int32
}

func validDishDetails(value dishDetails) bool {
	if strings.TrimSpace(value.Name) == "" || utf8.RuneCountInString(value.Name) > 128 ||
		utf8.RuneCountInString(value.Description) > 512 || value.PriceCents == 0 {
		return false
	}
	if value.CategoryPublicID != nil && !ValidULID(*value.CategoryPublicID) {
		return false
	}
	if value.ImageAssetPublicID != nil && !ValidULID(*value.ImageAssetPublicID) {
		return false
	}
	return true
}

func dishDetailsFromDish(value Dish) dishDetails {
	return dishDetails{
		CategoryPublicID: value.CategoryPublicID, Name: value.Name, Description: value.Description,
		PriceCents: value.PriceCents, ImageAssetPublicID: value.ImageAssetPublicID, SortOrder: value.SortOrder,
	}
}
