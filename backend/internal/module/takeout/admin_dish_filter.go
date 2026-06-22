package takeout

import (
	"context"
	"strings"
)

type adminDishFilterRepository interface {
	ListDishesFiltered(context.Context, string, string, DishStatus, int, int) ([]Dish, int64, error)
}

// DishesFiltered is an additive admin query. An empty categoryPublicID delegates
// to the frozen G-6a-1 query so existing callers retain identical behavior.
func (s *AdminQueryService) DishesFiltered(
	ctx context.Context,
	merchantPublicID, categoryPublicID string,
	status DishStatus,
	page, size int,
) ([]Dish, int64, error) {
	categoryPublicID = strings.TrimSpace(categoryPublicID)
	if categoryPublicID == "" {
		return s.Dishes(ctx, merchantPublicID, status, page, size)
	}
	merchantPublicID = strings.TrimSpace(merchantPublicID)
	if merchantPublicID != "" && !ValidULID(merchantPublicID) {
		return nil, 0, ErrInvalidULID
	}
	if !ValidULID(categoryPublicID) {
		return nil, 0, ErrInvalidULID
	}
	if status != "" && !status.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	repository, ok := s.repository.(adminDishFilterRepository)
	if !ok {
		return nil, 0, ErrInvalidRecord
	}
	page, size = normalizeAdminPage(page, size)
	return repository.ListDishesFiltered(ctx, merchantPublicID, categoryPublicID, status, page, size)
}
