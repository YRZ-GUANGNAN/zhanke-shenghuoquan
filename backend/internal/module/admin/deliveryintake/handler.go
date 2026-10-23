// Package deliveryintake exposes the durable intake boundary. It does not
// implement or imitate any provider's signature protocol.
package deliveryintake

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Intake interface {
	Receive(context.Context, string, delivery.SourcePlatform, delivery.ExternalOrderEnvelope) (delivery.ExternalReceipt, error)
	Receipt(context.Context, uint64, uint64) (delivery.ExternalReceipt, error)
	Retry(context.Context, uint64, uint64) error
}

type Handler struct {
	intake        Intake
	currentUserID func(*gin.Context) uint64
}

func NewHandler(intake Intake, currentUserID func(*gin.Context) uint64) *Handler {
	return &Handler{intake: intake, currentUserID: currentUserID}
}

func (h *Handler) RegisterWebhook(group *gin.RouterGroup) {
	group.POST("/:platform/:bindingKey/events", h.receive)
}

// This group must already require merchant authentication/identity.
func (h *Handler) RegisterMerchant(group *gin.RouterGroup) {
	group.GET("/external-order-receipts/:id", h.receipt)
	group.POST("/external-order-receipts/:id/retry", h.retry)
}

func (h *Handler) available(c *gin.Context) bool {
	if h.intake != nil {
		return true
	}
	response.Fail(c, http.StatusServiceUnavailable, "外部订单接口尚未配置平台授权")
	return false
}

func (h *Handler) receive(c *gin.Context) {
	platform := delivery.SourcePlatform(c.Param("platform"))
	if platform != delivery.SourceTaobao && platform != delivery.SourceJD && platform != delivery.SourceMeituan {
		response.Fail(c, http.StatusBadRequest, "unsupported platform")
		return
	}
	if !h.available(c) {
		return
	}
	if len(c.Request.URL.RawQuery) > 8192 {
		response.Fail(c, http.StatusBadRequest, "invalid query")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Fail(c, http.StatusRequestEntityTooLarge, "invalid callback body")
		return
	}
	if len(body) == 0 {
		response.Fail(c, http.StatusBadRequest, "empty callback body")
		return
	}
	value, err := h.intake.Receive(c.Request.Context(), c.Param("bindingKey"), platform, delivery.ExternalOrderEnvelope{
		Payload: body, Headers: c.Request.Header.Clone(), Method: c.Request.Method, Path: c.Request.URL.Path, RawQuery: c.Request.URL.RawQuery,
	})
	if err != nil {
		writeError(c, err)
		return
	}
	// Acknowledge durable receipt, never payment or dispatch. Official adapters
	// may need a provider-specific response encoder before production activation.
	response.OK(c, receiptView(value))
}

func receiptView(value delivery.ExternalReceipt) gin.H {
	return gin.H{"receiptId": value.ID, "status": value.Status, "orderPublicId": value.OrderPublicID, "reason": value.Reason, "duplicate": value.Duplicate}
}

func (h *Handler) scope(c *gin.Context) (uint64, uint64, bool) {
	if h.currentUserID == nil || h.currentUserID(c) == 0 {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
		return 0, 0, false
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid receipt")
		return 0, 0, false
	}
	return h.currentUserID(c), id, h.available(c)
}

func (h *Handler) receipt(c *gin.Context) {
	merchant, id, ok := h.scope(c)
	if !ok {
		return
	}
	value, err := h.intake.Receipt(c.Request.Context(), merchant, id)
	if err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, receiptView(value))
}

func (h *Handler) retry(c *gin.Context) {
	merchant, id, ok := h.scope(c)
	if !ok {
		return
	}
	if err := h.intake.Retry(c.Request.Context(), merchant, id); err != nil {
		writeError(c, err)
		return
	}
	response.OK(c, gin.H{"queued": true})
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, delivery.ErrExternalSignature):
		response.Fail(c, http.StatusUnauthorized, "invalid provider signature")
	case errors.Is(err, delivery.ErrExternalBinding), errors.Is(err, delivery.ErrNotFound):
		response.Fail(c, http.StatusNotFound, "binding or receipt unavailable")
	case errors.Is(err, delivery.ErrExternalEventConflict), errors.Is(err, delivery.ErrInvalidTransition):
		response.Fail(c, http.StatusConflict, "event state conflict")
	case errors.Is(err, delivery.ErrInvalidRecord):
		response.Fail(c, http.StatusBadRequest, "invalid event")
	default:
		response.Fail(c, http.StatusServiceUnavailable, "external intake unavailable")
	}
}
