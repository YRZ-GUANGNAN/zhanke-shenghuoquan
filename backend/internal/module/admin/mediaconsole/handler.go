package mediaconsole

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct{ service *QueryService }

func NewHandler(service *QueryService) *Handler { return &Handler{service: service} }

// Register accepts an already authenticated and admin-authorized route group.
// Module 9 intentionally exposes GET routes only.
func (h *Handler) Register(group *gin.RouterGroup) {
	group.GET("/summary", h.summary)
	group.GET("/assets", h.assets)
	group.GET("/assets/:assetId", h.asset)
	group.GET("/review-decisions", h.decisions)
	group.GET("/events", h.events)
	group.GET("/gc-jobs", h.gcJobs)
	group.GET("/deadletters", h.deadletters)
}

func (h *Handler) summary(c *gin.Context) {
	value, err := h.service.Summary(c)
	h.write(c, value, err)
}

func (h *Handler) assets(c *gin.Context) {
	filter, err := parseFilter(c)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	value, err := h.service.Assets(c, filter)
	h.write(c, value, err)
}

func (h *Handler) asset(c *gin.Context) {
	value, err := h.service.Asset(c, c.Param("assetId"))
	h.write(c, value, err)
}

func (h *Handler) decisions(c *gin.Context) {
	filter, err := parseFilter(c)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	value, err := h.service.Decisions(c, filter)
	h.write(c, value, err)
}

func (h *Handler) events(c *gin.Context) {
	filter, err := parseFilter(c)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	value, err := h.service.Events(c, filter)
	h.write(c, value, err)
}

func (h *Handler) gcJobs(c *gin.Context) {
	filter, err := parseFilter(c)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	value, err := h.service.GCJobs(c, filter)
	h.write(c, value, err)
}

func (h *Handler) deadletters(c *gin.Context) {
	filter, err := parseFilter(c)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	value, err := h.service.Deadletters(c, filter)
	h.write(c, value, err)
}

func parseFilter(c *gin.Context) (ListFilter, error) {
	page, err := positiveInt(c.DefaultQuery("page", "1"))
	if err != nil {
		return ListFilter{}, ErrInvalidQuery
	}
	pageSize, err := positiveInt(c.DefaultQuery("pageSize", "20"))
	if err != nil {
		return ListFilter{}, ErrInvalidQuery
	}
	filter := ListFilter{
		Page: page, PageSize: pageSize, AssetID: c.Query("assetId"), Status: c.Query("status"),
		ReviewStatus: c.Query("reviewStatus"), Purpose: c.Query("purpose"), Operation: c.Query("operation"), Kind: c.Query("kind"),
	}
	if raw := c.Query("taskId"); raw != "" {
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || value == 0 {
			return ListFilter{}, ErrInvalidQuery
		}
		filter.TaskID = &value
	}
	if filter.From, err = parseOptionalTime(c.Query("from")); err != nil {
		return ListFilter{}, ErrInvalidQuery
	}
	if filter.To, err = parseOptionalTime(c.Query("to")); err != nil {
		return ListFilter{}, ErrInvalidQuery
	}
	return filter, nil
}

func positiveInt(raw string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, ErrInvalidQuery
	}
	return value, nil
}

func parseOptionalTime(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	value = value.UTC()
	return &value, nil
}

func (h *Handler) write(c *gin.Context, value any, err error) {
	switch {
	case err == nil:
		response.OK(c, value)
	case errors.Is(err, ErrInvalidQuery):
		response.Fail(c, http.StatusBadRequest, "invalid media admin query")
	case errors.Is(err, ErrNotFound):
		response.Fail(c, http.StatusNotFound, "media asset not found")
	default:
		response.Fail(c, http.StatusInternalServerError, "media admin query failed")
	}
}
