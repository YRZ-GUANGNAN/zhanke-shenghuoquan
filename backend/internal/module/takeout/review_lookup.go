package takeout

import (
	"context"
	"errors"
)

var ErrReviewLookupUnavailable = errors.New("takeout review lookup is unavailable")

type merchantInternalIDReader interface {
	FindMerchantByInternalID(context.Context, uint64) (Merchant, error)
}

type dishInternalIDReader interface {
	FindDishByInternalID(context.Context, uint64) (Dish, error)
}

// FindMerchantByInternalID exposes the minimum read needed by the admin review
// adapter. Public business flows continue to address merchants by ULID.
func (s *MerchantService) FindMerchantByInternalID(ctx context.Context, id uint64) (Merchant, error) {
	if id == 0 {
		return Merchant{}, ErrInvalidRecord
	}
	reader, ok := s.repository.(merchantInternalIDReader)
	if !ok {
		return Merchant{}, ErrReviewLookupUnavailable
	}
	return reader.FindMerchantByInternalID(ctx, id)
}

// FindDishByInternalID exposes the minimum read needed by the admin review
// adapter. Public business flows continue to address dishes by ULID.
func (s *CatalogService) FindDishByInternalID(ctx context.Context, id uint64) (Dish, error) {
	if id == 0 {
		return Dish{}, ErrInvalidRecord
	}
	reader, ok := s.repository.(dishInternalIDReader)
	if !ok {
		return Dish{}, ErrReviewLookupUnavailable
	}
	return reader.FindDishByInternalID(ctx, id)
}
