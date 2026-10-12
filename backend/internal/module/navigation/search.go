package navigation

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/featureflags"
	"campus-circle/backend/internal/pkg/response"
	"campus-circle/backend/internal/pkg/searchmetrics"
)

type ExternalTakeoutSearchPage struct {
	Items    []PublicItem `json:"items"`
	Page     int          `json:"page"`
	PageSize int          `json:"pageSize"`
	Total    int          `json:"total"`
}

func (s *Service) SearchExternalTakeout(ctx context.Context, keyword string, page, size int) (ExternalTakeoutSearchPage, error) {
	display, normalized, err := searchmetrics.Normalize(keyword)
	if err != nil {
		return ExternalTakeoutSearchPage{}, err
	}
	_ = display
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 20
	}
	categories, err := s.PublicTree(ctx)
	if err != nil {
		return ExternalTakeoutSearchPage{}, err
	}
	type rankedItem struct {
		item PublicItem
		rank int
	}
	values := make([]rankedItem, 0)
	for _, category := range categories {
		if category.SystemKey == nil || *category.SystemKey != SystemKeyExternalTakeoutMerchants {
			continue
		}
		for _, item := range category.Items {
			title, subtitle := strings.ToLower(item.Title), strings.ToLower(item.Subtitle)
			rank := -1
			switch {
			case title == normalized:
				rank = 0
			case strings.HasPrefix(title, normalized):
				rank = 1
			case strings.Contains(title, normalized):
				rank = 2
			case strings.Contains(subtitle, normalized):
				rank = 3
			}
			if rank >= 0 {
				values = append(values, rankedItem{item: item, rank: rank})
			}
		}
		break
	}
	sort.Slice(values, func(i, j int) bool {
		left, right := values[i], values[j]
		if left.rank != right.rank {
			return left.rank < right.rank
		}
		if left.item.OperatingStatus != right.item.OperatingStatus {
			return left.item.OperatingStatus == OperatingStatusOpen
		}
		if left.item.Sort != right.item.Sort {
			return left.item.Sort < right.item.Sort
		}
		return left.item.ID < right.item.ID
	})
	total := len(values)
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	items := make([]PublicItem, 0, end-start)
	for _, value := range values[start:end] {
		items = append(items, value.item)
	}
	return ExternalTakeoutSearchPage{Items: items, Page: page, PageSize: size, Total: total}, nil
}

type SearchHandler struct {
	service       *Service
	authRequired  gin.HandlerFunc
	currentUserID func(*gin.Context) uint
	metrics       searchmetrics.Metrics
}

func NewSearchHandler(service *Service, authRequired gin.HandlerFunc, currentUserID func(*gin.Context) uint, metrics searchmetrics.Metrics) *SearchHandler {
	return &SearchHandler{service: service, authRequired: authRequired, currentUserID: currentUserID, metrics: metrics}
}

func (h *SearchHandler) Register(group *gin.RouterGroup) {
	auth := group.Group("/external-takeout", h.authRequired)
	auth.POST("/searches", h.search)
	auth.GET("/search-history", h.history)
	auth.DELETE("/search-history", h.clearHistory)
	auth.GET("/hot-searches", h.hotSearches)
}

func (h *SearchHandler) search(c *gin.Context) {
	if !featureflags.Can(featureflags.KeyTakeout) {
		response.Fail(c, http.StatusForbidden, "外卖功能暂未开放")
		return
	}
	var request struct {
		Keyword  string `json:"keyword"`
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
	}
	if c.ShouldBindJSON(&request) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid external takeout search")
		return
	}
	keyword, err := h.metrics.Confirm(c, h.currentUserID(c), request.Keyword)
	if errors.Is(err, searchmetrics.ErrInvalidKeyword) {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "record external takeout search failed")
		return
	}
	value, err := h.service.SearchExternalTakeout(c, keyword, request.Page, request.PageSize)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "search external takeout failed")
		return
	}
	response.OK(c, value)
}

func (h *SearchHandler) history(c *gin.Context) {
	if !featureflags.Can(featureflags.KeyTakeout) {
		response.Fail(c, http.StatusForbidden, "外卖功能暂未开放")
		return
	}
	value, err := h.metrics.History(c, h.currentUserID(c))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "list external takeout search history failed")
		return
	}
	response.OK(c, value)
}

func (h *SearchHandler) clearHistory(c *gin.Context) {
	if !featureflags.Can(featureflags.KeyTakeout) {
		response.Fail(c, http.StatusForbidden, "外卖功能暂未开放")
		return
	}
	if err := h.metrics.ClearHistory(c, h.currentUserID(c)); err != nil {
		response.Fail(c, http.StatusInternalServerError, "clear external takeout search history failed")
		return
	}
	response.OK(c, gin.H{"cleared": true})
}

func (h *SearchHandler) hotSearches(c *gin.Context) {
	if !featureflags.Can(featureflags.KeyTakeout) {
		response.Fail(c, http.StatusForbidden, "外卖功能暂未开放")
		return
	}
	value, err := h.metrics.Hot(c)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "list external takeout hot searches failed")
		return
	}
	response.OK(c, value)
}
