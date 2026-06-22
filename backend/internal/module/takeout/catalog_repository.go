package takeout

import (
	"context"
	"time"
)

type CatalogRepository interface {
	CreateCategory(context.Context, *DishCategory) error
	FindCategory(context.Context, string) (DishCategory, error)
	RenameCategory(context.Context, string, string, uint64, string, time.Time) (DishCategory, error)
	ReorderCategory(context.Context, string, string, uint64, int32, time.Time) (DishCategory, error)

	CreateDish(context.Context, *Dish) error
	FindDish(context.Context, string) (Dish, error)
	UpdateDish(context.Context, string, string, uint64, DishStatus, dishDetails, time.Time) (Dish, error)
	TransitionDishStatus(context.Context, string, string, uint64, DishStatus, DishStatus, time.Time) (Dish, error)
}

var _ CatalogRepository = (*GormRepository)(nil)
