// Package deliveryconsole exposes administrator-only delivery operations.
// Delivery money rules remain in internal/module/delivery.
package deliveryconsole

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/pkg/response"
)

type Handler struct {
	queries       QueryRepository
	fees          delivery.FeeConfigRepository
	riderWallets  delivery.RiderWalletRepository
	currentUserID func(*gin.Context) uint64
	now           func() time.Time
}

func NewHandler(
	queries QueryRepository,
	fees delivery.FeeConfigRepository,
	riderWallets delivery.RiderWalletRepository,
	currentUserID func(*gin.Context) uint64,
	now func() time.Time,
) *Handler {
	if now == nil {
		now = time.Now
	}
	return &Handler{
		queries: queries, fees: fees, riderWallets: riderWallets,
		currentUserID: currentUserID, now: now,
	}
}

func (h *Handler) Register(group *gin.RouterGroup) {
	group.GET("/summary", h.summary)
	group.GET("/orders", h.orders)
	group.GET("/orders/:id/funds", h.orderFunds)
	group.GET("/orders/:id", h.order)
	group.GET("/fee-config", h.feeConfig)
	group.PUT("/fee-config", h.updateFeeConfig)
	group.GET("/merchant-fee-overrides", h.merchantFeeOverrides)
	group.GET("/merchant-fee-overrides/:merchantUserId", h.merchantFeeOverride)
	group.PUT("/merchant-fee-overrides/:merchantUserId", h.upsertMerchantFeeOverride)
	group.PATCH("/merchant-fee-overrides/:merchantUserId/disable", h.disableMerchantFeeOverride)
	group.GET("/merchant-wallets", h.merchantWallets)
	group.GET("/merchant-wallets/:merchantUserId/ledger", h.merchantLedger)
	group.GET("/merchant-wallets/:merchantUserId", h.merchantWallet)
	group.GET("/rider-wallets", h.riderWalletList)
	group.GET("/rider-wallets/:riderUserId/ledger", h.riderLedger)
	group.GET("/rider-wallets/:riderUserId", h.riderWallet)
	group.GET("/rider-earnings", h.riderEarnings)
	group.POST("/rider-earnings/:id/pay-offline", h.payOffline)
	group.GET("/rider-earnings/:id", h.riderEarning)
	group.GET("/rider-payouts", h.riderPayouts)
	group.GET("/refunds", h.refunds)
	group.GET("/refunds/:id", h.refund)
}

func (h *Handler) summary(c *gin.Context) {
	value, err := h.queries.Summary(c.Request.Context(), h.now().UTC())
	h.write(c, value, err)
}

func (h *Handler) orders(c *gin.Context) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	values, total, err := h.queries.ListOrders(c.Request.Context(), filter)
	h.write(c, Page[OrderView]{Items: mapValues(values, orderView), Page: filter.Page, PageSize: filter.PageSize, Total: total}, err)
}

func (h *Handler) order(c *gin.Context) {
	if !delivery.ValidULID(c.Param("id")) {
		response.Fail(c, http.StatusBadRequest, "invalid delivery order id")
		return
	}
	value, err := h.queries.Order(c.Request.Context(), c.Param("id"))
	h.write(c, orderView(value), err)
}

func (h *Handler) orderFunds(c *gin.Context) {
	if !delivery.ValidULID(c.Param("id")) {
		response.Fail(c, http.StatusBadRequest, "invalid delivery order id")
		return
	}
	value, err := h.queries.OrderFunds(c.Request.Context(), c.Param("id"))
	h.write(c, orderFundsView(value), err)
}

func (h *Handler) feeConfig(c *gin.Context) {
	value, err := h.fees.GlobalFeeConfig(c.Request.Context())
	h.write(c, feeConfigView(value), err)
}

func (h *Handler) updateFeeConfig(c *gin.Context) {
	var req struct {
		DeliveryFeeCents   uint64 `json:"deliveryFeeCents"`
		PlatformCutCents   uint64 `json:"platformCutCents"`
		ExpectedRowVersion uint64 `json:"expectedRowVersion"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.fees.UpdateGlobalFeeConfig(
		c.Request.Context(), req.DeliveryFeeCents, req.PlatformCutCents,
		req.ExpectedRowVersion, h.now().UTC(),
	)
	h.write(c, feeConfigView(value), err)
}

func (h *Handler) merchantFeeOverrides(c *gin.Context) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	values, total, err := h.queries.ListMerchantFeeOverrides(c.Request.Context(), filter)
	h.write(c, Page[MerchantFeeOverrideView]{Items: mapValues(values, merchantFeeOverrideView), Page: filter.Page, PageSize: filter.PageSize, Total: total}, err)
}

func (h *Handler) merchantFeeOverride(c *gin.Context) {
	merchantUserID, ok := h.uintParam(c, "merchantUserId")
	if !ok {
		return
	}
	value, err := h.queries.MerchantFeeOverride(c.Request.Context(), merchantUserID)
	h.write(c, merchantFeeOverrideView(value), err)
}

func (h *Handler) upsertMerchantFeeOverride(c *gin.Context) {
	merchantUserID, ok := h.uintParam(c, "merchantUserId")
	if !ok {
		return
	}
	var req struct {
		DeliveryFeeCents   uint64  `json:"deliveryFeeCents"`
		PlatformCutCents   uint64  `json:"platformCutCents"`
		ExpectedRowVersion *uint64 `json:"expectedRowVersion"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.fees.UpsertMerchantFeeOverride(
		c.Request.Context(), merchantUserID, req.DeliveryFeeCents, req.PlatformCutCents,
		req.ExpectedRowVersion, h.now().UTC(),
	)
	h.write(c, merchantFeeOverrideView(value), err)
}

func (h *Handler) disableMerchantFeeOverride(c *gin.Context) {
	merchantUserID, ok := h.uintParam(c, "merchantUserId")
	if !ok {
		return
	}
	var req struct {
		ExpectedRowVersion uint64 `json:"expectedRowVersion"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.queries.DisableMerchantFeeOverride(
		c.Request.Context(), merchantUserID, req.ExpectedRowVersion, h.now().UTC(),
	)
	h.write(c, merchantFeeOverrideView(value), err)
}

func (h *Handler) merchantWallets(c *gin.Context) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	values, total, err := h.queries.ListMerchantWallets(c.Request.Context(), filter)
	h.write(c, Page[MerchantWalletView]{Items: mapValues(values, merchantWalletView), Page: filter.Page, PageSize: filter.PageSize, Total: total}, err)
}

func (h *Handler) merchantWallet(c *gin.Context) {
	merchantUserID, ok := h.uintParam(c, "merchantUserId")
	if !ok {
		return
	}
	value, err := h.queries.MerchantWallet(c.Request.Context(), merchantUserID)
	h.write(c, merchantWalletView(value), err)
}

func (h *Handler) merchantLedger(c *gin.Context) {
	merchantUserID, ok := h.uintParam(c, "merchantUserId")
	if !ok {
		return
	}
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	values, total, err := h.queries.ListMerchantLedger(c.Request.Context(), merchantUserID, filter)
	h.write(c, Page[MerchantLedgerView]{Items: mapValues(values, merchantLedgerView), Page: filter.Page, PageSize: filter.PageSize, Total: total}, err)
}

func (h *Handler) riderWalletList(c *gin.Context) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	values, total, err := h.queries.ListRiderWallets(c.Request.Context(), filter)
	h.write(c, Page[RiderWalletView]{Items: mapValues(values, riderWalletView), Page: filter.Page, PageSize: filter.PageSize, Total: total}, err)
}

func (h *Handler) riderWallet(c *gin.Context) {
	riderUserID, ok := h.uintParam(c, "riderUserId")
	if !ok {
		return
	}
	value, err := h.queries.RiderWallet(c.Request.Context(), riderUserID)
	h.write(c, riderWalletView(value), err)
}

func (h *Handler) riderLedger(c *gin.Context) {
	riderUserID, ok := h.uintParam(c, "riderUserId")
	if !ok {
		return
	}
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	values, total, err := h.queries.ListRiderLedger(c.Request.Context(), riderUserID, filter)
	h.write(c, Page[RiderLedgerView]{Items: mapValues(values, riderLedgerView), Page: filter.Page, PageSize: filter.PageSize, Total: total}, err)
}

func (h *Handler) riderEarnings(c *gin.Context) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	values, total, err := h.queries.ListRiderEarnings(c.Request.Context(), filter)
	h.write(c, Page[RiderEarningView]{Items: mapValues(values, riderEarningView), Page: filter.Page, PageSize: filter.PageSize, Total: total}, err)
}

func (h *Handler) riderEarning(c *gin.Context) {
	if !delivery.ValidULID(c.Param("id")) {
		response.Fail(c, http.StatusBadRequest, "invalid earning id")
		return
	}
	value, err := h.queries.RiderEarning(c.Request.Context(), c.Param("id"))
	h.write(c, riderEarningView(value), err)
}

func (h *Handler) payOffline(c *gin.Context) {
	if !delivery.ValidULID(c.Param("id")) {
		response.Fail(c, http.StatusBadRequest, "invalid earning id")
		return
	}
	var req struct {
		ExpectedRowVersion uint64 `json:"expectedRowVersion"`
		IdempotencyKey     string `json:"idempotencyKey"`
		Note               string `json:"note"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.riderWallets.MarkRiderEarningPaidOffline(
		c.Request.Context(), c.Param("id"), req.ExpectedRowVersion,
		h.currentUserID(c), strings.TrimSpace(req.IdempotencyKey), req.Note, h.now().UTC(),
	)
	h.write(c, riderPayoutResultView(value), err)
}

func (h *Handler) riderPayouts(c *gin.Context) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	values, total, err := h.queries.ListRiderPayouts(c.Request.Context(), filter)
	h.write(c, Page[RiderPayoutView]{Items: mapValues(values, riderPayoutView), Page: filter.Page, PageSize: filter.PageSize, Total: total}, err)
}

func (h *Handler) refunds(c *gin.Context) {
	filter, ok := h.parseFilter(c)
	if !ok {
		return
	}
	values, total, err := h.queries.ListRefunds(c.Request.Context(), filter)
	h.write(c, Page[RefundView]{Items: mapValues(values, refundView), Page: filter.Page, PageSize: filter.PageSize, Total: total}, err)
}

func (h *Handler) refund(c *gin.Context) {
	if !delivery.ValidULID(c.Param("id")) {
		response.Fail(c, http.StatusBadRequest, "invalid refund id")
		return
	}
	value, err := h.queries.Refund(c.Request.Context(), c.Param("id"))
	h.write(c, refundView(value), err)
}

func (h *Handler) parseFilter(c *gin.Context) (ListFilter, bool) {
	page, err := strconv.Atoi(defaultQuery(c, "page", "1"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return ListFilter{}, false
	}
	pageSize, err := strconv.Atoi(defaultQuery(c, "pageSize", "20"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return ListFilter{}, false
	}
	filter := ListFilter{Page: page, PageSize: pageSize, Status: strings.TrimSpace(c.Query("status"))}
	if !h.optionalUintQuery(c, "merchantUserId", &filter.MerchantUserID) ||
		!h.optionalUintQuery(c, "riderUserId", &filter.RiderUserID) {
		return ListFilter{}, false
	}
	if !optionalTimeQuery(c, "from", &filter.From) || !optionalTimeQuery(c, "to", &filter.To) {
		response.Fail(c, http.StatusBadRequest, "invalid time filter")
		return ListFilter{}, false
	}
	if filter.Page < 1 || filter.PageSize < 1 || filter.PageSize > 100 {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return ListFilter{}, false
	}
	return filter, true
}

func (h *Handler) optionalUintQuery(c *gin.Context, key string, target *uint64) bool {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return true
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid "+key)
		return false
	}
	*target = value
	return true
}

func optionalTimeQuery(c *gin.Context, key string, target **time.Time) bool {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return true
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return false
	}
	utc := value.UTC()
	*target = &utc
	return true
}

func (h *Handler) uintParam(c *gin.Context, key string) (uint64, bool) {
	value, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil || value == 0 {
		response.Fail(c, http.StatusBadRequest, "invalid "+key)
		return 0, false
	}
	return value, true
}

func defaultQuery(c *gin.Context, key, fallback string) string {
	if value := strings.TrimSpace(c.Query(key)); value != "" {
		return value
	}
	return fallback
}

func (h *Handler) write(c *gin.Context, value any, err error) {
	switch {
	case err == nil:
		response.OK(c, value)
	case errors.Is(err, delivery.ErrNotFound):
		response.Fail(c, http.StatusNotFound, "delivery admin record not found")
	case errors.Is(err, delivery.ErrConcurrentUpdate), errors.Is(err, delivery.ErrIdempotencyConflict),
		errors.Is(err, delivery.ErrEarningSettled), errors.Is(err, delivery.ErrWalletInsufficient):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, delivery.ErrInvalidRecord), errors.Is(err, delivery.ErrInvalidULID),
		errors.Is(err, delivery.ErrAmountOverflow), errors.Is(err, ErrInvalidQuery):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		response.Fail(c, http.StatusInternalServerError, "delivery admin operation failed")
	}
}
