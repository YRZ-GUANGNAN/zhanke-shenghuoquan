// Package trade is business domain C: second-hand products and seller contact entry.
package trade

import (
	"campus-circle/backend/internal/module/admin/review"
	"gorm.io/gorm"
	"time"
)

type ProductStatus string

const (
	ReviewStatusTakenDown review.Status = "taken_down"

	StatusSelling  ProductStatus = "selling"
	StatusReserved ProductStatus = "reserved"
	StatusSold     ProductStatus = "sold"
)

type Product struct {
	ID                uint           `json:"id" gorm:"primaryKey"`
	SellerID          uint           `json:"sellerId" gorm:"index"`
	ClientRequestID   *string        `json:"-" gorm:"column:client_request_id;type:char(26)"`
	CreateRequestHash []byte         `json:"-" gorm:"column:create_request_hash;type:binary(32)"`
	Title             string         `json:"title" gorm:"size:160"`
	Description       string         `json:"description" gorm:"type:text"`
	Category          string         `json:"category" gorm:"size:64;index"`
	Price             int64          `json:"price"`
	OriginalPrice     int64          `json:"originalPrice"`
	Condition         string         `json:"condition" gorm:"column:condition_label;size:64"`
	Campus            string         `json:"campus" gorm:"size:80;index"`
	Status            ProductStatus  `json:"status" gorm:"size:24;index;default:selling"`
	ReviewStatus      review.Status  `json:"reviewStatus" gorm:"size:24;index;default:submitted"`
	Images            []ProductImage `json:"images"`
	FavoriteCount     int64          `json:"favoriteCount"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
	DeletedAt         gorm.DeletedAt `json:"-" gorm:"index"`
}
type ProductImage struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	ProductID uint      `json:"-" gorm:"uniqueIndex:idx_product_image_sort"`
	AssetID   *uint64   `json:"-" gorm:"index"`
	URL       string    `json:"url" gorm:"size:512"`
	Sort      int       `json:"sort" gorm:"uniqueIndex:idx_product_image_sort"`
	CreatedAt time.Time `json:"createdAt"`
}
type ProductFavorite struct {
	ProductID uint `gorm:"primaryKey"`
	UserID    uint `gorm:"primaryKey"`
	CreatedAt time.Time
}

type SearchOperation string

const (
	SearchUpsert SearchOperation = "upsert"
	SearchDelete SearchOperation = "delete"
)

type SearchOutbox struct {
	ID          uint            `gorm:"primaryKey"`
	ProductID   uint            `gorm:"index;not null"`
	Operation   SearchOperation `gorm:"size:16;not null"`
	Attempts    int             `gorm:"not null;default:0"`
	AvailableAt time.Time       `gorm:"index;not null"`
	ProcessedAt *time.Time      `gorm:"index"`
	LastError   string          `gorm:"size:512"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (SearchOutbox) TableName() string { return "trade_search_outbox" }
