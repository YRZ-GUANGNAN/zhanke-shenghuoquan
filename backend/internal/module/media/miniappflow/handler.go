package miniappflow

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service       *Service
	display       displayContentService
	currentUserID func(*gin.Context) uint64
}

type displayContentService interface {
	ContentURL(context.Context, coremedia.Purpose, string, uint16) (string, error)
}

func NewHandler(service *Service, currentUserID func(*gin.Context) uint64, display ...displayContentService) *Handler {
	handler := &Handler{service: service, currentUserID: currentUserID}
	if len(display) > 0 {
		handler.display = display[0]
	}
	return handler
}

// Register exposes only the Module 10 mini-program orchestration contract.
// Authentication and muted-user checks are attached by the composition root.
func (h *Handler) Register(group *gin.RouterGroup) {
	group.POST("/uploads", h.createUpload)
	group.POST("/uploads/:ticketId/complete", h.completeUpload)
	group.GET("/assets/:assetId", h.status)
	group.GET("/uploads", h.history)
	group.GET("/display/:referenceType/:referenceId/:slot/content", h.displayContent)
	group.GET("/resolve/:referenceType/:referenceId/:slot", h.resolveContentURL)
}

func (h *Handler) createUpload(c *gin.Context) {
	var request struct {
		Purpose        coremedia.Purpose `json:"purpose" binding:"required"`
		SizeBytes      uint64            `json:"sizeBytes" binding:"required"`
		ContentMD5     string            `json:"contentMD5" binding:"required"`
		ContentType    string            `json:"contentType" binding:"required"`
		IdempotencyKey string            `json:"idempotencyKey" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || !request.Purpose.Valid() || strings.TrimSpace(request.IdempotencyKey) == "" {
		response.Fail(c, http.StatusBadRequest, "invalid media upload request")
		return
	}
	value, err := h.service.CreateUpload(c.Request.Context(), CreateUploadInput{
		OwnerID: h.currentUserID(c), Purpose: request.Purpose, SizeBytes: request.SizeBytes,
		ContentMD5: request.ContentMD5, ContentType: request.ContentType,
		IdempotencyKey: strings.TrimSpace(request.IdempotencyKey),
	})
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.Created(c, value)
}

func (h *Handler) completeUpload(c *gin.Context) {
	value, err := h.service.CompleteUpload(c.Request.Context(), h.currentUserID(c), c.Param("ticketId"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) status(c *gin.Context) {
	value, err := h.service.Status(c.Request.Context(), h.currentUserID(c), c.Param("assetId"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) history(c *gin.Context) {
	page, err := positivePage(c.DefaultQuery("page", "1"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return
	}
	pageSize, err := positivePage(c.DefaultQuery("pageSize", "20"))
	if err != nil || pageSize > 50 {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return
	}
	value, err := h.service.History(c.Request.Context(), h.currentUserID(c), page, pageSize)
	if err != nil {
		h.writeError(c, err)
		return
	}
	response.OK(c, value)
}

func (h *Handler) displayContent(c *gin.Context) {
	if h.display == nil {
		response.Fail(c, http.StatusNotFound, "media content not found")
		return
	}
	referenceType := coremedia.Purpose(c.Param("referenceType"))
	if !referenceType.Valid() {
		response.Fail(c, http.StatusBadRequest, "invalid media reference")
		return
	}
	referenceID := strings.TrimSpace(c.Param("referenceId"))
	slot, err := strconv.ParseUint(c.Param("slot"), 10, 16)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid media reference")
		return
	}
	value, err := h.display.ContentURL(c.Request.Context(), referenceType, referenceID, uint16(slot))
	if err != nil {
		h.writeDisplayError(c, err)
		return
	}
	c.Header("Location", value)
	c.Status(http.StatusFound)
}

func (h *Handler) resolveContentURL(c *gin.Context) {
	if h.display == nil {
		response.Fail(c, http.StatusNotFound, "media content not found")
		return
	}
	referenceType := coremedia.Purpose(c.Param("referenceType"))
	if !referenceType.Valid() {
		response.Fail(c, http.StatusBadRequest, "invalid media reference")
		return
	}
	referenceID := strings.TrimSpace(c.Param("referenceId"))
	slot, err := strconv.ParseUint(c.Param("slot"), 10, 16)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid media reference")
		return
	}
	value, err := h.display.ContentURL(c.Request.Context(), referenceType, referenceID, uint16(slot))
	if err != nil {
		h.writeDisplayError(c, err)
		return
	}
	response.OK(c, ResolvedDisplayURL{
		URL:              value,
		ExpiresInSeconds: int64(coremedia.MediaDisplayURLTTL.Seconds()),
	})
}

func positivePage(raw string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, ErrInvalidRequest
	}
	return value, nil
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidRequest), errors.Is(err, coremedia.ErrInvalidRecord),
		errors.Is(err, coremedia.ErrInvalidUploadDeclaration), errors.Is(err, ErrUploadNotStarted):
		response.Fail(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, coremedia.ErrTicketForbidden):
		response.Fail(c, http.StatusForbidden, "media upload does not belong to current user")
	case errors.Is(err, coremedia.ErrNotFound):
		response.Fail(c, http.StatusNotFound, "media upload not found")
	case errors.Is(err, ErrUploadExpired), errors.Is(err, coremedia.ErrTicketExpired):
		response.Fail(c, http.StatusGone, "upload ticket expired; create a new upload")
	case errors.Is(err, ErrUploadCancelled):
		response.Fail(c, http.StatusGone, "upload was cancelled; create a new upload")
	case errors.Is(err, coremedia.ErrQuotaExceeded):
		response.Fail(c, http.StatusTooManyRequests, "media upload quota exceeded")
	case errors.Is(err, ErrAuthorizationConsumed), errors.Is(err, coremedia.ErrAuthorizationAlreadyIssued),
		errors.Is(err, coremedia.ErrConcurrentUpdate), errors.Is(err, coremedia.ErrCompletionInProgress):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, coremedia.ErrCompletionValidation):
		response.Fail(c, http.StatusUnprocessableEntity, "uploaded image failed validation")
	case errors.Is(err, ErrSubmissionPending):
		response.Fail(c, http.StatusServiceUnavailable, "review submission is pending; retry completion")
	default:
		response.Fail(c, http.StatusInternalServerError, "media upload flow failed")
	}
}

func (h *Handler) writeDisplayError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, coremedia.ErrInvalidRecord), errors.Is(err, coremedia.ErrInvalidULID):
		response.Fail(c, http.StatusBadRequest, "invalid media reference")
	case errors.Is(err, coremedia.ErrNotFound), errors.Is(err, coremedia.ErrMediaNotVisible),
		errors.Is(err, coremedia.ErrReviewProjectionUnavailable), errors.Is(err, coremedia.ErrObjectNotFound):
		response.Fail(c, http.StatusNotFound, "media content not found")
	default:
		response.Fail(c, http.StatusInternalServerError, "media content failed")
	}
}
