// Package navigation is business domain D. It contains jump configuration only.
package navigation

import "time"

type OperatingStatus string

const (
	SystemKeyExternalTakeoutMerchants                 = "external_takeout_merchants"
	OperatingStatusOpen               OperatingStatus = "open"
	OperatingStatusResting            OperatingStatus = "resting"
)

type Category struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:64"`
	SystemKey *string   `json:"systemKey,omitempty" gorm:"type:varchar(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin;size:64;uniqueIndex:uq_navigation_categories_system_key"`
	Sort      int       `json:"sort" gorm:"index"`
	Enabled   bool      `json:"enabled" gorm:"default:true"`
	Items     []Item    `json:"items"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type Item struct {
	ID              uint            `json:"id" gorm:"primaryKey"`
	CategoryID      uint            `json:"categoryId" gorm:"index"`
	Title           string          `json:"title" gorm:"size:64"`
	Subtitle        string          `json:"subtitle" gorm:"size:120"`
	IconText        string          `json:"iconText" gorm:"size:12"`
	AppID           string          `json:"appId" gorm:"size:80"`
	Path            string          `json:"path" gorm:"size:240"`
	Color           string          `json:"color" gorm:"size:24"`
	Sort            int             `json:"sort" gorm:"index"`
	Enabled         bool            `json:"enabled" gorm:"default:true"`
	OperatingStatus OperatingStatus `json:"operatingStatus" gorm:"type:varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin;size:16;not null;default:open"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       time.Time       `json:"updatedAt"`
}

func (Category) TableName() string { return "navigation_categories" }
func (Item) TableName() string     { return "navigation_items" }
