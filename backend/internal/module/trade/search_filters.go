package trade

import (
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrInvalidSearchFilters = errors.New("搜索筛选条件无效，请检查分类或价格范围")

type SearchFilters struct {
	Sort     string `json:"sort"`
	Category string `json:"category"`
	MinPrice *int64 `json:"minPrice,omitempty"`
	MaxPrice *int64 `json:"maxPrice,omitempty"`
}

func validateSearchFilters(value SearchFilters) error {
	switch value.Sort {
	case "", "relevance", "recent", "price_asc", "price_desc":
	default:
		return ErrInvalidSearchFilters
	}
	if utf8.RuneCountInString(value.Category) > 64 || strings.ContainsAny(value.Category, "\r\n\x00") {
		return ErrInvalidSearchFilters
	}
	for _, price := range []*int64{value.MinPrice, value.MaxPrice} {
		if price != nil && (*price < 0 || *price > 9999999999) {
			return ErrInvalidSearchFilters
		}
	}
	if value.MinPrice != nil && value.MaxPrice != nil && *value.MinPrice > *value.MaxPrice {
		return ErrInvalidSearchFilters
	}
	return nil
}
