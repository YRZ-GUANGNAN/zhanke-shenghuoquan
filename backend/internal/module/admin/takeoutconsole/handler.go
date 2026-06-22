// Package takeoutconsole exposes the administrator transport adapter for
// domain G. Domain rules remain in internal/module/takeout.
package takeoutconsole

import (
	"errors"
	"net/http"
	"strconv"

	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	merchants *takeout.MerchantService
	catalog   *takeout.CatalogService
	orders    *takeout.OrderService
	riders    *takeout.RiderService
	queries   *takeout.AdminQueryService
	withdraws *takeout.WithdrawService
}

func NewHandler(
	merchants *takeout.MerchantService,
	catalog *takeout.CatalogService,
	orders *takeout.OrderService,
	riders *takeout.RiderService,
	queries *takeout.AdminQueryService,
	withdraws *takeout.WithdrawService,
) *Handler {
	return &Handler{merchants: merchants, catalog: catalog, orders: orders, riders: riders, queries: queries, withdraws: withdraws}
}

func (h *Handler) Register(group *gin.RouterGroup) {
	group.GET("/merchants", h.listMerchants)
	group.GET("/merchants/:id", h.merchant)
	group.PATCH("/merchants/:id/suspend", h.suspendMerchant)
	group.PATCH("/merchants/:id/unsuspend", h.unsuspendMerchant)
	group.GET("/categories", h.listCategories)
	group.GET("/categories/:id", h.category)
	group.GET("/dishes", h.listDishes)
	group.GET("/dishes/:id", h.dish)
	group.GET("/orders", h.listOrders)
	group.GET("/orders/:id", h.order)
	group.GET("/riders", h.listRiders)
	group.PATCH("/riders/:id/status", h.riderStatus)
	group.GET("/fee-config", h.feeConfig)
	group.PUT("/fee-config", h.updateFeeConfig)
	group.GET("/rider-config", h.riderConfig)
	group.PUT("/rider-config", h.updateRiderConfig)
	group.GET("/settlement-report", h.settlementReport)
	group.GET("/withdraw-requests", h.listWithdraws)
	group.GET("/withdraw-requests/:id", h.withdraw)
	group.PATCH("/withdraw-requests/:id/pay", h.payWithdraw)
	group.PATCH("/withdraw-requests/:id/reject", h.rejectWithdraw)
}

func (h *Handler) listMerchants(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	values, total, err := h.queries.Merchants(c.Request.Context(), takeout.MerchantStatus(c.Query("status")), page, size)
	h.write(c, Page[MerchantView]{mapValues(values, merchantView), page, size, total}, err)
}

func (h *Handler) merchant(c *gin.Context) {
	value, err := h.merchants.FindMerchant(c.Request.Context(), c.Param("id"))
	h.write(c, merchantView(value), err)
}

func (h *Handler) suspendMerchant(c *gin.Context) {
	var req struct {
		ExpectedStatus     takeout.MerchantStatus `json:"expectedStatus"`
		ExpectedRowVersion uint64                 `json:"expectedRowVersion"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.merchants.Suspend(c.Request.Context(), c.Param("id"), req.ExpectedStatus, req.ExpectedRowVersion)
	h.write(c, merchantView(value), err)
}

func (h *Handler) unsuspendMerchant(c *gin.Context) {
	var req struct {
		ExpectedRowVersion uint64 `json:"expectedRowVersion"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.merchants.Unsuspend(c.Request.Context(), c.Param("id"), req.ExpectedRowVersion)
	h.write(c, merchantView(value), err)
}

func (h *Handler) listCategories(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	values, total, err := h.queries.Categories(c.Request.Context(), c.Query("merchantId"), page, size)
	h.write(c, Page[CategoryView]{mapValues(values, categoryView), page, size, total}, err)
}

func (h *Handler) category(c *gin.Context) {
	value, err := h.catalog.FindCategory(c.Request.Context(), c.Param("id"))
	h.write(c, categoryView(value), err)
}

func (h *Handler) listDishes(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	values, total, err := h.queries.Dishes(c.Request.Context(), c.Query("merchantId"), takeout.DishStatus(c.Query("status")), page, size)
	h.write(c, Page[DishView]{mapValues(values, dishView), page, size, total}, err)
}

func (h *Handler) dish(c *gin.Context) {
	value, err := h.catalog.FindDish(c.Request.Context(), c.Param("id"))
	h.write(c, dishView(value), err)
}

func (h *Handler) listOrders(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	values, total, err := h.queries.Orders(c.Request.Context(), takeout.OrderStatus(c.Query("status")), page, size)
	h.write(c, Page[OrderView]{mapValues(values, orderView), page, size, total}, err)
}

func (h *Handler) order(c *gin.Context) {
	value, err := h.orders.FindOrder(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.write(c, nil, err)
		return
	}
	items, err := h.orders.ListOrderItems(c.Request.Context(), c.Param("id"))
	h.write(c, OrderDetailView{Order: orderView(value), Items: mapValues(items, orderItemView)}, err)
}

func (h *Handler) listRiders(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	values, total, err := h.queries.Riders(c.Request.Context(), takeout.RiderStatus(c.Query("status")), page, size)
	h.write(c, Page[RiderView]{mapValues(values, riderView), page, size, total}, err)
}

func (h *Handler) riderStatus(c *gin.Context) {
	riderUserID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || riderUserID == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid rider id")
		return
	}
	var req struct {
		Status             takeout.RiderStatus `json:"status"`
		ExpectedStatus     takeout.RiderStatus `json:"expectedStatus"`
		ExpectedRowVersion uint64              `json:"expectedRowVersion"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	var value takeout.Rider
	switch req.Status {
	case takeout.RiderActive:
		if req.ExpectedStatus == takeout.RiderSuspended {
			value, err = h.riders.Unsuspend(c.Request.Context(), riderUserID, req.ExpectedRowVersion)
		} else if req.ExpectedStatus == takeout.RiderResting {
			value, err = h.riders.SetActive(c.Request.Context(), riderUserID, req.ExpectedRowVersion)
		} else {
			err = takeout.ErrInvalidTransition
		}
	case takeout.RiderResting:
		if req.ExpectedStatus != takeout.RiderActive {
			err = takeout.ErrInvalidTransition
		} else {
			value, err = h.riders.SetResting(c.Request.Context(), riderUserID, req.ExpectedRowVersion)
		}
	case takeout.RiderSuspended:
		value, err = h.riders.Suspend(c.Request.Context(), riderUserID, req.ExpectedRowVersion, req.ExpectedStatus)
	default:
		err = takeout.ErrInvalidRecord
	}
	h.write(c, riderView(value), err)
}

func (h *Handler) feeConfig(c *gin.Context) {
	value, err := h.orders.GetActiveFeeConfig(c.Request.Context())
	h.write(c, FeeConfigView{value.RiderFeeCents, value.PlatformCutCents, value.RowVersion, value.UpdatedAt}, err)
}

func (h *Handler) updateFeeConfig(c *gin.Context) {
	var req struct {
		RiderFeeCents      uint64 `json:"riderFeeCents"`
		PlatformCutCents   uint64 `json:"platformCutCents"`
		ExpectedRowVersion uint64 `json:"expectedRowVersion"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.orders.UpdateFeeConfig(c.Request.Context(), req.RiderFeeCents, req.PlatformCutCents, req.ExpectedRowVersion)
	h.write(c, FeeConfigView{value.RiderFeeCents, value.PlatformCutCents, value.RowVersion, value.UpdatedAt}, err)
}

func (h *Handler) riderConfig(c *gin.Context) {
	value, err := h.riders.GetRiderConfig(c.Request.Context())
	h.write(c, RiderConfigView{value.MaxInFlightPerRider, value.RowVersion, value.UpdatedAt}, err)
}

func (h *Handler) updateRiderConfig(c *gin.Context) {
	var req struct {
		MaxInFlightPerRider uint32 `json:"maxInFlightPerRider"`
		ExpectedRowVersion  uint64 `json:"expectedRowVersion"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.riders.UpdateRiderConfig(c.Request.Context(), req.MaxInFlightPerRider, req.ExpectedRowVersion)
	h.write(c, RiderConfigView{value.MaxInFlightPerRider, value.RowVersion, value.UpdatedAt}, err)
}

func (h *Handler) settlementReport(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	values, total, err := h.queries.SettlementReport(c.Request.Context(), page, size)
	views := mapValues(values, func(v takeout.SettlementSummary) SettlementView {
		return SettlementView{v.RiderUserID, v.PendingCount, v.RiderFeeCents, v.PlatformCutCents}
	})
	h.write(c, Page[SettlementView]{views, page, size, total}, err)
}

func (h *Handler) listWithdraws(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	values, total, err := h.withdraws.ListWithdrawRequests(c.Request.Context(), takeout.WithdrawStatus(c.Query("status")), page, size)
	h.write(c, Page[WithdrawView]{mapValues(values, withdrawView), page, size, total}, err)
}

func (h *Handler) withdraw(c *gin.Context) {
	value, err := h.withdraws.FindWithdrawByID(c.Request.Context(), c.Param("id"))
	h.write(c, withdrawView(value), err)
}

func (h *Handler) payWithdraw(c *gin.Context) {
	var req struct {
		ExpectedRowVersion uint64 `json:"expectedRowVersion"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.withdraws.MarkWithdrawPaid(c.Request.Context(), c.Param("id"), req.ExpectedRowVersion)
	h.write(c, withdrawView(value), err)
}

func (h *Handler) rejectWithdraw(c *gin.Context) {
	var req struct {
		ExpectedRowVersion uint64 `json:"expectedRowVersion"`
		Reason             string `json:"reason"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.withdraws.RejectWithdraw(c.Request.Context(), c.Param("id"), req.ExpectedRowVersion, req.Reason)
	h.write(c, withdrawView(value), err)
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

func (h *Handler) write(c *gin.Context, value any, err error) {
	switch {
	case err == nil:
		response.OK(c, value)
	case errors.Is(err, takeout.ErrNotFound):
		response.Fail(c, http.StatusNotFound, "takeout record not found")
	case errors.Is(err, takeout.ErrConcurrentUpdate), errors.Is(err, takeout.ErrInvalidTransition), errors.Is(err, takeout.ErrInFlightLimit), errors.Is(err, takeout.ErrOrderAlreadyClaimed):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, takeout.ErrInvalidRecord), errors.Is(err, takeout.ErrInvalidULID), errors.Is(err, takeout.ErrAmountOverflow):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		response.Fail(c, http.StatusInternalServerError, "takeout admin operation failed")
	}
}
