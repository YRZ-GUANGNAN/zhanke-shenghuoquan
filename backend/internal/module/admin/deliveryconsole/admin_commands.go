package deliveryconsole

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/pkg/response"
)

// orderCommands is the administrator write surface over delivery orders.
type orderCommands interface {
	AdminReassignRider(context.Context, string, uint64, uint64, uint64, string) (delivery.Order, error)
	AdminCancelOrder(context.Context, string, uint64, uint64, string) (delivery.Order, error)
}

// WithOrderCommands enables the order command routes (reassign and cancel).
func (h *Handler) WithOrderCommands(value orderCommands) *Handler {
	h.commands = value
	return h
}

// RefundCommands 暴露管理员退款（退款会在同一事务内取消订单并退回配送费）。
type RefundCommands interface {
	RequestAdminRefund(ctx context.Context, orderPublicID string, expectedRowVersion uint64, idempotencyKey string) (delivery.RefundResult, error)
}

// WithRefundCommands enables the administrator refund route.
func (h *Handler) WithRefundCommands(value RefundCommands) *Handler {
	h.refundCommands = value
	return h
}

func (h *Handler) refundOrder(c *gin.Context) {
	if !delivery.ValidULID(c.Param("id")) {
		response.Fail(c, http.StatusBadRequest, "invalid delivery order id")
		return
	}
	if h.refundCommands == nil {
		response.Fail(c, http.StatusServiceUnavailable, "delivery refunds unavailable")
		return
	}
	var request struct {
		RowVersion     uint64 `json:"rowVersion"`
		IdempotencyKey string `json:"idempotencyKey"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.RowVersion == 0 || strings.TrimSpace(request.IdempotencyKey) == "" {
		response.Fail(c, http.StatusBadRequest, "rowVersion and idempotencyKey are required")
		return
	}
	result, err := h.refundCommands.RequestAdminRefund(c.Request.Context(), c.Param("id"), request.RowVersion, strings.TrimSpace(request.IdempotencyKey))
	switch {
	case err == nil:
		response.OK(c, gin.H{"refund": result.Refund.RefundPublicID, "order": result.Order.DeliveryOrderPublicID, "status": string(result.Order.Status)})
	case errors.Is(err, delivery.ErrRefundNotAllowed), errors.Is(err, delivery.ErrInvalidTransition):
		response.Fail(c, http.StatusConflict, "该订单当前状态不允许退款（未支付、已送达或已取消）")
	case errors.Is(err, delivery.ErrConcurrentUpdate):
		response.Fail(c, http.StatusConflict, "数据已被他人修改，请刷新后重试")
	case errors.Is(err, delivery.ErrNotFound):
		response.Fail(c, http.StatusNotFound, "配送单不存在")
	default:
		response.Fail(c, http.StatusInternalServerError, "退款发起失败")
	}
}

func (h *Handler) reassignOrder(c *gin.Context) {
	if !delivery.ValidULID(c.Param("id")) {
		response.Fail(c, http.StatusBadRequest, "invalid delivery order id")
		return
	}
	if h.commands == nil {
		response.Fail(c, http.StatusServiceUnavailable, "delivery order commands unavailable")
		return
	}
	var request struct {
		RiderUserID uint64 `json:"riderUserId"`
		RowVersion  uint64 `json:"rowVersion"`
		Reason      string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.RiderUserID == 0 || request.RowVersion == 0 {
		response.Fail(c, http.StatusBadRequest, "riderUserId, rowVersion and reason are required")
		return
	}
	actor := h.actorUserID(c)
	value, err := h.commands.AdminReassignRider(c.Request.Context(), c.Param("id"), request.RowVersion, request.RiderUserID, actor, request.Reason)
	h.write(c, orderView(value), err)
}

func (h *Handler) cancelOrder(c *gin.Context) {
	if !delivery.ValidULID(c.Param("id")) {
		response.Fail(c, http.StatusBadRequest, "invalid delivery order id")
		return
	}
	if h.commands == nil {
		response.Fail(c, http.StatusServiceUnavailable, "delivery order commands unavailable")
		return
	}
	var request struct {
		RowVersion uint64 `json:"rowVersion"`
		Reason     string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.RowVersion == 0 {
		response.Fail(c, http.StatusBadRequest, "rowVersion and reason are required")
		return
	}
	actor := h.actorUserID(c)
	value, err := h.commands.AdminCancelOrder(c.Request.Context(), c.Param("id"), request.RowVersion, actor, request.Reason)
	h.write(c, orderView(value), err)
}

func (h *Handler) actorUserID(c *gin.Context) uint64 {
	if h.currentUserID == nil {
		return 0
	}
	return h.currentUserID(c)
}

// ---------------------------------------------------------------------------
// Rider administration: candidate list, duty/suspension state and in-flight
// orders. Rider identity lives in takeout_riders; the duty flag lives in
// delivery_rider_availability. Statistics are aggregated from delivery_orders.
// ---------------------------------------------------------------------------

type RiderFilter struct {
	Status   string
	Query    string
	Page     int
	PageSize int
}

type RiderRow struct {
	RiderUserID      uint64    `json:"riderUserId"`
	Status           string    `json:"status"`
	DutyState        string    `json:"dutyState"`
	RealName         string    `json:"realName"`
	Phone            string    `json:"phone"`
	RowVersion       uint64    `json:"rowVersion"`
	InFlight         int64     `json:"inFlight"`
	TodayOrders      int64     `json:"todayOrders"`
	TodayIncomeCents int64     `json:"todayIncomeCents"`
	AccountStatus    string    `json:"accountStatus"`
	GrantActive      bool      `json:"grantActive"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type RiderAdminRepository interface {
	ListRiders(context.Context, RiderFilter) ([]RiderRow, int64, error)
	RiderOrders(context.Context, uint64) ([]delivery.Order, error)
	SetRiderStatus(context.Context, uint64, string, uint64, uint64, string, time.Time) (RiderRow, error)
}

// WithRiderAdmin enables the rider management routes.
func (h *Handler) WithRiderAdmin(value RiderAdminRepository) *Handler {
	h.riderAdmin = value
	return h
}

func (h *Handler) riders(c *gin.Context) {
	if h.riderAdmin == nil {
		response.Fail(c, http.StatusServiceUnavailable, "rider administration unavailable")
		return
	}
	page, err := strconv.Atoi(defaultQuery(c, "page", "1"))
	if err != nil || page < 1 {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return
	}
	pageSize, err := strconv.Atoi(defaultQuery(c, "pageSize", "20"))
	if err != nil || pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	filter := RiderFilter{Status: strings.TrimSpace(c.Query("status")), Query: strings.TrimSpace(c.Query("query")), Page: page, PageSize: pageSize}
	values, total, err := h.riderAdmin.ListRiders(c.Request.Context(), filter)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "rider list failed")
		return
	}
	response.OK(c, Page[RiderRow]{Items: values, Page: filter.Page, PageSize: filter.PageSize, Total: total})
}

func (h *Handler) riderOrders(c *gin.Context) {
	if h.riderAdmin == nil {
		response.Fail(c, http.StatusServiceUnavailable, "rider administration unavailable")
		return
	}
	riderUserID, err := strconv.ParseUint(c.Param("riderUserId"), 10, 64)
	if err != nil || riderUserID == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid rider id")
		return
	}
	values, err := h.riderAdmin.RiderOrders(c.Request.Context(), riderUserID)
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "rider orders failed")
		return
	}
	items := make([]OrderView, 0, len(values))
	for _, value := range values {
		items = append(items, orderView(value))
	}
	response.OK(c, items)
}

func (h *Handler) setRiderStatus(c *gin.Context) {
	if h.riderAdmin == nil {
		response.Fail(c, http.StatusServiceUnavailable, "rider administration unavailable")
		return
	}
	riderUserID, err := strconv.ParseUint(c.Param("riderUserId"), 10, 64)
	if err != nil || riderUserID == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid rider id")
		return
	}
	var request struct {
		Status     string `json:"status"`
		RowVersion uint64 `json:"rowVersion"`
		Reason     string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.RowVersion == 0 {
		response.Fail(c, http.StatusBadRequest, "status and rowVersion are required")
		return
	}
	switch request.Status {
	case "active", "resting", "suspended":
	default:
		response.Fail(c, http.StatusBadRequest, "invalid rider status")
		return
	}
	reason := strings.TrimSpace(request.Reason)
	if reason == "" {
		response.Fail(c, http.StatusBadRequest, "rider status change requires a reason")
		return
	}
	value, err := h.riderAdmin.SetRiderStatus(c.Request.Context(), riderUserID, request.Status, request.RowVersion, h.actorUserID(c), reason, h.now())
	switch {
	case err == nil:
		response.OK(c, value)
	case errors.Is(err, ErrRiderHasActiveOrders):
		response.Fail(c, http.StatusConflict, "骑手仍有在途订单，请先转出在途订单再暂停或封禁")
	case errors.Is(err, delivery.ErrConcurrentUpdate):
		response.Fail(c, http.StatusConflict, "rider state changed, refresh and retry")
	case errors.Is(err, delivery.ErrNotFound):
		response.Fail(c, http.StatusNotFound, "rider not found")
	default:
		response.Fail(c, http.StatusInternalServerError, "rider status update failed")
	}
}
