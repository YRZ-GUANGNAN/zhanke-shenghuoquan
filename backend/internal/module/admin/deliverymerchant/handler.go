package deliverymerchant

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/pkg/response"
)

type Handler struct {
	service          *delivery.DispatchService
	currentUserID    func(*gin.Context) uint64
	allowMockConfirm bool
}

func NewHandler(
	service *delivery.DispatchService,
	currentUserID func(*gin.Context) uint64,
	allowMockConfirm bool,
) *Handler {
	return &Handler{service: service, currentUserID: currentUserID, allowMockConfirm: allowMockConfirm}
}

func (h *Handler) Register(group *gin.RouterGroup) {
	group.POST("/orders", h.create)
	group.GET("/orders", h.list)
	group.GET("/orders/:id", h.detail)
	group.PATCH("/orders/:id/cancel", h.cancel)
	if h.allowMockConfirm {
		group.POST("/orders/:id/mock-confirm-fee", h.mockConfirmFee)
	}
}

func (h *Handler) create(c *gin.Context) {
	var req struct {
		ClientRequestID string `json:"clientRequestId"`
		PickupName      string `json:"pickupName"`
		PickupPhone     string `json:"pickupPhone"`
		PickupAddress   string `json:"pickupAddress"`
		RecipientName   string `json:"recipientName"`
		RecipientPhone  string `json:"recipientPhone"`
		DeliveryAddress string `json:"deliveryAddress"`
		Remark          string `json:"remark"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	requestID := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if requestID == "" {
		requestID = strings.TrimSpace(req.ClientRequestID)
	}
	value, err := h.service.CreateOrder(c.Request.Context(), delivery.CreateDispatchInput{
		MerchantUserID: h.currentUserID(c), MerchantRequestID: requestID,
		Pickup:    delivery.Contact{Name: req.PickupName, Phone: req.PickupPhone, Address: req.PickupAddress},
		Recipient: delivery.Contact{Name: req.RecipientName, Phone: req.RecipientPhone, Address: req.DeliveryAddress},
		Remark:    req.Remark,
	})
	if err != nil {
		h.write(c, nil, err)
		return
	}
	response.Created(c, orderView(value))
}

func (h *Handler) list(c *gin.Context) {
	page, ok := parsePage(c)
	if !ok {
		return
	}
	statuses, ok := merchantStatusesForGroup(c.Query("statusGroup"))
	if !ok {
		response.Fail(c, http.StatusBadRequest, "invalid status group")
		return
	}
	values, total, err := h.service.MerchantOrders(c.Request.Context(), h.currentUserID(c), statuses, page)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	items := make([]OrderView, 0, len(values))
	for _, value := range values {
		items = append(items, orderView(value))
	}
	response.OK(c, Page[OrderView]{Items: items, Page: page.Page, PageSize: page.Size, Total: total})
}

func merchantStatusesForGroup(group string) ([]delivery.Status, bool) {
	switch strings.TrimSpace(group) {
	case "":
		return nil, true
	case "pending":
		return []delivery.Status{
			delivery.StatusPendingFeePayment,
			delivery.StatusPaymentFailed,
			delivery.StatusReadyForClaim,
		}, true
	case "delivering":
		return []delivery.Status{
			delivery.StatusClaimed,
			delivery.StatusToPickup,
			delivery.StatusPickedUp,
			delivery.StatusDelivering,
		}, true
	case "completed":
		return []delivery.Status{delivery.StatusDelivered, delivery.StatusCancelled}, true
	default:
		return nil, false
	}
}

func (h *Handler) detail(c *gin.Context) {
	value, err := h.service.MerchantOrder(c.Request.Context(), h.currentUserID(c), c.Param("id"))
	h.write(c, orderView(value), err)
}

func (h *Handler) cancel(c *gin.Context) {
	var req struct {
		ExpectedStatus     delivery.Status `json:"expectedStatus"`
		ExpectedRowVersion uint64          `json:"expectedRowVersion"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.service.CancelMerchantOrder(
		c.Request.Context(), h.currentUserID(c), c.Param("id"),
		req.ExpectedStatus, req.ExpectedRowVersion,
	)
	h.write(c, orderView(value), err)
}

func (h *Handler) mockConfirmFee(c *gin.Context) {
	var req struct {
		ExpectedRowVersion uint64 `json:"expectedRowVersion"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, _, err := h.service.ConfirmMockFee(
		c.Request.Context(), h.currentUserID(c), c.Param("id"), req.ExpectedRowVersion,
	)
	h.write(c, orderView(value), err)
}

func (h *Handler) write(c *gin.Context, value any, err error) {
	switch {
	case err == nil:
		response.OK(c, value)
	case errors.Is(err, delivery.ErrForbiddenOrder), errors.Is(err, delivery.ErrNotFound):
		response.Fail(c, http.StatusNotFound, "delivery order not found")
	case errors.Is(err, delivery.ErrConcurrentUpdate), errors.Is(err, delivery.ErrInvalidTransition),
		errors.Is(err, delivery.ErrIdempotencyConflict):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, delivery.ErrInvalidRecord), errors.Is(err, delivery.ErrInvalidULID),
		errors.Is(err, delivery.ErrAmountOverflow):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		response.Fail(c, http.StatusInternalServerError, "delivery merchant operation failed")
	}
}

func parsePage(c *gin.Context) (delivery.Page, bool) {
	page, err := strconv.Atoi(defaultQuery(c, "page", "1"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return delivery.Page{}, false
	}
	size, err := strconv.Atoi(defaultQuery(c, "pageSize", "20"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return delivery.Page{}, false
	}
	value := delivery.Page{Page: page, Size: size}
	if !value.Valid() {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return delivery.Page{}, false
	}
	return value, true
}

func defaultQuery(c *gin.Context, key, fallback string) string {
	if value := c.Query(key); value != "" {
		return value
	}
	return fallback
}
