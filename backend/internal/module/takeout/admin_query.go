package takeout

import (
	"context"
	"strings"
)

type SettlementSummary struct {
	RiderUserID      uint64
	PendingCount     int64
	RiderFeeCents    uint64
	PlatformCutCents uint64
}

type AdminQueryRepository interface {
	ListMerchants(context.Context, MerchantStatus, int, int) ([]Merchant, int64, error)
	ListCategories(context.Context, string, int, int) ([]DishCategory, int64, error)
	ListDishes(context.Context, string, DishStatus, int, int) ([]Dish, int64, error)
	ListOrders(context.Context, OrderStatus, int, int) ([]Order, int64, error)
	ListRiders(context.Context, RiderStatus, int, int) ([]Rider, int64, error)
	ListSettlementSummaries(context.Context, int, int) ([]SettlementSummary, int64, error)
}

var _ AdminQueryRepository = (*GormRepository)(nil)

type AdminQueryService struct{ repository AdminQueryRepository }

func NewAdminQueryService(repository AdminQueryRepository) (*AdminQueryService, error) {
	if repository == nil {
		return nil, ErrInvalidRecord
	}
	return &AdminQueryService{repository: repository}, nil
}

func (s *AdminQueryService) Merchants(ctx context.Context, status MerchantStatus, page, size int) ([]Merchant, int64, error) {
	if status != "" && !status.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	page, size = normalizeAdminPage(page, size)
	return s.repository.ListMerchants(ctx, status, page, size)
}

func (s *AdminQueryService) Categories(ctx context.Context, merchantPublicID string, page, size int) ([]DishCategory, int64, error) {
	merchantPublicID = strings.TrimSpace(merchantPublicID)
	if merchantPublicID != "" && !ValidULID(merchantPublicID) {
		return nil, 0, ErrInvalidULID
	}
	page, size = normalizeAdminPage(page, size)
	return s.repository.ListCategories(ctx, merchantPublicID, page, size)
}

func (s *AdminQueryService) Dishes(ctx context.Context, merchantPublicID string, status DishStatus, page, size int) ([]Dish, int64, error) {
	merchantPublicID = strings.TrimSpace(merchantPublicID)
	if merchantPublicID != "" && !ValidULID(merchantPublicID) {
		return nil, 0, ErrInvalidULID
	}
	if status != "" && !status.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	page, size = normalizeAdminPage(page, size)
	return s.repository.ListDishes(ctx, merchantPublicID, status, page, size)
}

func (s *AdminQueryService) Orders(ctx context.Context, status OrderStatus, page, size int) ([]Order, int64, error) {
	if status != "" && !status.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	page, size = normalizeAdminPage(page, size)
	return s.repository.ListOrders(ctx, status, page, size)
}

func (s *AdminQueryService) Riders(ctx context.Context, status RiderStatus, page, size int) ([]Rider, int64, error) {
	if status != "" && !status.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	page, size = normalizeAdminPage(page, size)
	return s.repository.ListRiders(ctx, status, page, size)
}

func (s *AdminQueryService) SettlementReport(ctx context.Context, page, size int) ([]SettlementSummary, int64, error) {
	page, size = normalizeAdminPage(page, size)
	return s.repository.ListSettlementSummaries(ctx, page, size)
}

func normalizeAdminPage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
