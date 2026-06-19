// Package navigation is business domain D. It contains jump configuration only.
package navigation

import "time"

type Category struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:64"`
	Sort      int       `json:"sort" gorm:"index"`
	Enabled   bool      `json:"enabled" gorm:"default:true"`
	Items     []Item    `json:"items"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type Item struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	CategoryID uint      `json:"categoryId" gorm:"index"`
	Title      string    `json:"title" gorm:"size:64"`
	Subtitle   string    `json:"subtitle" gorm:"size:120"`
	IconText   string    `json:"iconText" gorm:"size:12"`
	AppID      string    `json:"appId" gorm:"size:80"`
	Path       string    `json:"path" gorm:"size:240"`
	Color      string    `json:"color" gorm:"size:24"`
	Sort       int       `json:"sort" gorm:"index"`
	Enabled    bool      `json:"enabled" gorm:"default:true"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (Category) TableName() string { return "navigation_categories" }
func (Item) TableName() string     { return "navigation_items" }
