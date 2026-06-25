package takeoutrider

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/pkg/response"
)

type Handler struct {
	service       *Service
	currentUserID func(*gin.Context) uint64
}

func NewHandler(service *Service, currentUserID func(*gin.Context) uint64) *Handler {
	return &Handler{service: service, currentUserID: currentUserID}
}

func (h *Handler) Register(group *gin.RouterGroup) {
	group.GET("/me", h.me)
	group.POST("/register", h.register)
	group.GET("/claimable-orders", h.claimableOrders)
	group.POST("/orders/:orderPublicId/claim", h.claimOrder)
	group.GET("/fulfillments", h.fulfillments)
	group.GET("/fulfillments/:fulfillmentPublicId", h.fulfillment)
	group.PATCH("/fulfillments/:fulfillmentPublicId/to-store", h.startToStore)
	group.PATCH("/fulfillments/:fulfillmentPublicId/picked", h.markPicked)
	group.PATCH("/fulfillments/:fulfillmentPublicId/to-user", h.startToUser)
	group.PATCH("/fulfillments/:fulfillmentPublicId/complete", h.complete)
	group.GET("/wallet", h.wallet)
	group.GET("/withdrawals", h.withdrawals)
	group.POST("/withdrawals", h.createWithdrawal)
}

func (h *Handler) me(c *gin.Context) {
	value, err := h.service.FindRider(c.Request.Context(), h.currentUserID(c))
	if errors.Is(err, takeout.ErrNotFound) {
		response.OK(c, MeView{Registered: false})
		return
	}
	if err != nil {
		h.write(c, nil, err)
		return
	}
	view := riderView(value)
	response.OK(c, MeView{Registered: true, Rider: &view})
}

func (h *Handler) register(c *gin.Context) {
	var req struct {
		RealName string `json:"realName"`
		Phone    string `json:"phone"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.service.RegisterRider(c.Request.Context(), takeout.RegisterRiderInput{
		RiderUserID: h.currentUserID(c), RealName: req.RealName, Phone: req.Phone,
	})
	h.writeCreated(c, riderView(value), err)
}

func (h *Handler) claimableOrders(c *gin.Context) {
	values, err := h.service.ListClaimableOrders(c.Request.Context(), h.currentUserID(c))
	if err != nil {
		h.write(c, nil, err)
		return
	}
	merchantNames, err := h.service.merchantNames(c.Request.Context(), values)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	views := make([]OrderDetailView, 0, len(values))
	for _, value := range values {
		views = append(views, OrderDetailView{
			Order: orderSummaryView(value.Order, merchantNames[value.Order.MerchantPublicID]),
			Items: mapValues(value.Items, orderItemView),
		})
	}
	response.OK(c, Page[OrderDetailView]{Items: views, Page: 1, PageSize: len(views), Total: int64(len(views))})
}

type rowVersionRequest struct {
	ExpectedRowVersion uint64 `json:"expectedRowVersion"`
}

func (h *Handler) claimOrder(c *gin.Context) {
	var req rowVersionRequest
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.service.ClaimOrder(c.Request.Context(), h.currentUserID(c), c.Param("orderPublicId"), req.ExpectedRowVersion)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	h.write(c, h.fulfillmentDetailView(c, value), nil)
}

func (h *Handler) fulfillments(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	values, total, err := h.service.ListFulfillments(c.Request.Context(), h.currentUserID(c), takeout.FulfillmentStatus(c.Query("status")), page, size)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	merchantNames, err := h.service.merchantNamesForFulfillments(c.Request.Context(), values)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	views := make([]FulfillmentDetailView, 0, len(values))
	for _, value := range values {
		views = append(views, h.fulfillmentDetailViewWithMerchant(value, merchantNames[value.Order.MerchantPublicID]))
	}
	response.OK(c, Page[FulfillmentDetailView]{Items: views, Page: page, PageSize: size, Total: total})
}

func (h *Handler) fulfillment(c *gin.Context) {
	value, err := h.service.FindFulfillment(c.Request.Context(), h.currentUserID(c), c.Param("fulfillmentPublicId"))
	if err != nil {
		h.write(c, nil, err)
		return
	}
	h.write(c, h.fulfillmentDetailView(c, value), nil)
}

func (h *Handler) startToStore(c *gin.Context) {
	h.transitionFulfillment(c, h.service.StartToStore)
}

func (h *Handler) markPicked(c *gin.Context) {
	h.transitionFulfillment(c, h.service.MarkPicked)
}

func (h *Handler) startToUser(c *gin.Context) {
	h.transitionFulfillment(c, h.service.StartToUser)
}

func (h *Handler) complete(c *gin.Context) {
	h.transitionFulfillment(c, h.service.Complete)
}

func (h *Handler) transitionFulfillment(
	c *gin.Context,
	transition func(context.Context, uint64, string, uint64) (FulfillmentDetail, error),
) {
	var req rowVersionRequest
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := transition(c.Request.Context(), h.currentUserID(c), c.Param("fulfillmentPublicId"), req.ExpectedRowVersion)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	h.write(c, h.fulfillmentDetailView(c, value), nil)
}

func (h *Handler) wallet(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	value, err := h.service.Wallet(c.Request.Context(), h.currentUserID(c), page, size)
	h.write(c, value, err)
}

func (h *Handler) withdrawals(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	values, total, err := h.service.ListWithdrawals(c.Request.Context(), h.currentUserID(c), takeout.WithdrawStatus(c.Query("status")), page, size)
	h.write(c, Page[WithdrawView]{Items: mapValues(values, withdrawView), Page: page, PageSize: size, Total: total}, err)
}

func (h *Handler) createWithdrawal(c *gin.Context) {
	var req struct {
		AmountCents uint64 `json:"amountCents"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.service.CreateWithdrawal(c.Request.Context(), h.currentUserID(c), req.AmountCents)
	h.writeCreated(c, withdrawView(value), err)
}

func (h *Handler) fulfillmentDetailView(c *gin.Context, value FulfillmentDetail) FulfillmentDetailView {
	names, err := h.service.repository.MerchantNames(c.Request.Context(), []string{value.Order.MerchantPublicID})
	if err != nil {
		return h.fulfillmentDetailViewWithMerchant(value, "")
	}
	return h.fulfillmentDetailViewWithMerchant(value, names[value.Order.MerchantPublicID])
}

func (h *Handler) fulfillmentDetailViewWithMerchant(value FulfillmentDetail, merchantName string) FulfillmentDetailView {
	return FulfillmentDetailView{
		Fulfillment: fulfillmentView(value.Fulfillment),
		Order:       orderSummaryView(value.Order, merchantName),
		Items:       mapValues(value.Items, orderItemView),
	}
}

func pagination(c *gin.Context) (int, int, bool) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return 0, 0, false
	}
	size, err := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if err != nil || size < 1 || size > 100 {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return 0, 0, false
	}
	return page, size, true
}

func (h *Handler) writeCreated(c *gin.Context, value any, err error) {
	if err != nil {
		h.write(c, nil, err)
		return
	}
	response.Created(c, value)
}

func (h *Handler) write(c *gin.Context, value any, err error) {
	switch {
	case err == nil:
		response.OK(c, value)
	case errors.Is(err, takeout.ErrNotFound), errors.Is(err, ErrForbiddenFulfillment):
		response.Fail(c, http.StatusNotFound, "takeout rider record not found")
	case errors.Is(err, ErrRiderRegistrationRequired):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, takeout.ErrRiderUnavailable):
		response.Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, takeout.ErrConcurrentUpdate), errors.Is(err, takeout.ErrInvalidTransition),
		errors.Is(err, takeout.ErrInFlightLimit), errors.Is(err, takeout.ErrOrderAlreadyClaimed),
		errors.Is(err, ErrInsufficientBalance):
		status := http.StatusConflict
		if errors.Is(err, ErrInsufficientBalance) {
			status = http.StatusUnprocessableEntity
		}
		response.Fail(c, status, err.Error())
	case errors.Is(err, takeout.ErrInvalidRecord), errors.Is(err, takeout.ErrInvalidULID), errors.Is(err, takeout.ErrAmountOverflow):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		response.Fail(c, http.StatusInternalServerError, "takeout rider operation failed")
	}
}
