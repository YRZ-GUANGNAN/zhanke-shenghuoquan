package community

import (
	"errors"
)

var ErrInvalidSearchFilters = errors.New("搜索筛选条件无效")

type SearchFilters struct {
	Sort        string `json:"sort"`
	ContentType string `json:"contentType"`
	WithinDays  int    `json:"withinDays"`
}

func validateSearchFilters(value SearchFilters) error {
	switch value.Sort {
	case "", "relevance", "recent", "likes", "comments", "favorites":
	default:
		return ErrInvalidSearchFilters
	}
	switch value.ContentType {
	case "", "all", "image", "text":
	default:
		return ErrInvalidSearchFilters
	}
	switch value.WithinDays {
	case 0, 1, 7, 180:
	default:
		return ErrInvalidSearchFilters
	}
	return nil
}
