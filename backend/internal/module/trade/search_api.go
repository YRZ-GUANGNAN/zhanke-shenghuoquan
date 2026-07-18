package trade

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"campus-circle/backend/internal/pkg/searchmetrics"
)

type tradeSearchRepository interface {
	SearchProducts(context.Context, uint, string, int, int) ([]Product, int64, error)
}

func (s *Service) SearchProducts(ctx context.Context, viewerID uint, keyword string, page, size int) (Page, error) {
	display, _, err := searchmetrics.Normalize(keyword)
	if err != nil {
		return Page{}, err
	}
	page, size = normalize(page, size)
	repository, ok := s.repo.(tradeSearchRepository)
	if !ok {
		return Page{}, ErrSearchUnavailable
	}
	values, total, err := repository.SearchProducts(ctx, viewerID, display, page, size)
	if err != nil {
		return Page{}, err
	}
	views, err := s.enrich(ctx, values)
	return Page{Items: views, Page: page, PageSize: size, Total: total}, err
}

type SearchHandler struct {
	service *Service
	users   *user.Service
	metrics searchmetrics.Metrics
}

func NewSearchHandler(service *Service, users *user.Service, metrics searchmetrics.Metrics) *SearchHandler {
	return &SearchHandler{service: service, users: users, metrics: metrics}
}

func (h *SearchHandler) Register(group *gin.RouterGroup) {
	auth := group.Group("", h.users.AuthRequired())
	auth.POST("/searches", h.search)
	auth.GET("/search-history", h.history)
	auth.DELETE("/search-history", h.clearHistory)
	auth.GET("/hot-searches", h.hotSearches)
}

func (h *SearchHandler) search(c *gin.Context) {
	var request struct {
		Keyword  string `json:"keyword"`
		Page     int    `json:"page"`
		PageSize int    `json:"pageSize"`
	}
	if c.ShouldBindJSON(&request) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid trade search")
		return
	}
	keyword, err := h.metrics.Confirm(c, user.CurrentUserID(c), request.Keyword)
	if errors.Is(err, searchmetrics.ErrInvalidKeyword) {
		response.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "record trade search failed")
		return
	}
	value, err := h.service.SearchProducts(c, user.CurrentUserID(c), keyword, request.Page, request.PageSize)
	write(c, value, err)
}

func (h *SearchHandler) history(c *gin.Context) {
	value, err := h.metrics.History(c, user.CurrentUserID(c))
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "list trade search history failed")
		return
	}
	response.OK(c, value)
}

func (h *SearchHandler) clearHistory(c *gin.Context) {
	if err := h.metrics.ClearHistory(c, user.CurrentUserID(c)); err != nil {
		response.Fail(c, http.StatusInternalServerError, "clear trade search history failed")
		return
	}
	response.OK(c, gin.H{"cleared": true})
}

func (h *SearchHandler) hotSearches(c *gin.Context) {
	value, err := h.metrics.Hot(c)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "list trade hot searches failed")
		return
	}
	response.OK(c, value)
}
