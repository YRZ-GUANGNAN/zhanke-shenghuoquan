package deliverypayment

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/pkg/response"
)

type Handler struct {
	funds         *delivery.FundsService
	provider      Provider
	currentUserID func(*gin.Context) uint64
	currentOpenID func(*gin.Context) string
}

func NewHandler(
	funds *delivery.FundsService,
	provider Provider,
	currentUserID func(*gin.Context) uint64,
	currentOpenID func(*gin.Context) string,
) *Handler {
	return &Handler{funds: funds, provider: provider, currentUserID: currentUserID, currentOpenID: currentOpenID}
}

func (h *Handler) RegisterMerchant(group *gin.RouterGroup) {
	group.GET("/wallet", h.wallet)
	group.GET("/wallet/ledger", h.ledger)
	group.POST("/wallet/recharges", h.recharge)
	group.GET("/payments/:id", h.payment)
	group.POST("/orders/:id/pay-with-wallet", h.payWithWallet)
	group.POST("/orders/:id/pay-with-wechat", h.payWithWechat)
	group.POST("/orders/:id/refunds", h.merchantRefund)
}

func (h *Handler) RegisterAdmin(group *gin.RouterGroup) {
	group.POST("/orders/:id/refunds", h.adminRefund)
}

func (h *Handler) RegisterWebhook(group *gin.RouterGroup) {
	group.POST("/wechat/notify", h.paymentNotify)
	group.POST("/wechat/refund-notify", h.refundNotify)
}

func (h *Handler) wallet(c *gin.Context) {
	value, err := h.funds.Wallet(c.Request.Context(), h.currentUserID(c))
	if err != nil {
		h.write(c, nil, err)
		return
	}
	response.OK(c, walletView(value))
}

func (h *Handler) ledger(c *gin.Context) {
	page, ok := parsePage(c)
	if !ok {
		return
	}
	values, total, err := h.funds.Ledger(c.Request.Context(), h.currentUserID(c), page)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	items := make([]LedgerView, 0, len(values))
	for _, value := range values {
		items = append(items, ledgerView(value))
	}
	response.OK(c, Page[LedgerView]{Items: items, Page: page.Page, PageSize: page.Size, Total: total})
}

func (h *Handler) payment(c *gin.Context) {
	value, err := h.funds.Payment(c.Request.Context(), h.currentUserID(c), c.Param("id"))
	if err != nil {
		h.write(c, nil, err)
		return
	}
	response.OK(c, paymentView(value, nil))
}

func (h *Handler) recharge(c *gin.Context) {
	var req struct {
		AmountCents     uint64 `json:"amountCents"`
		ClientRequestID string `json:"clientRequestId"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.funds.StartRecharge(
		c.Request.Context(), h.currentUserID(c), req.AmountCents,
		req.ClientRequestID, h.provider.Provider(),
	)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	params, err := h.issuePrepay(c, &value, "校园配送钱包充值")
	if err != nil {
		h.write(c, nil, err)
		return
	}
	result := FundsResponse{Payment: pointer(paymentView(value.Attempt, params))}
	if value.Wallet != nil {
		result.Wallet = pointer(walletView(*value.Wallet))
	}
	if value.Ledger != nil {
		result.Ledger = pointer(ledgerView(*value.Ledger))
	}
	response.OK(c, result)
}

func (h *Handler) payWithWallet(c *gin.Context) {
	var req struct {
		ExpectedRowVersion uint64 `json:"expectedRowVersion"`
		IdempotencyKey     string `json:"idempotencyKey"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.funds.PayOrderWithWallet(
		c.Request.Context(), h.currentUserID(c), c.Param("id"),
		req.ExpectedRowVersion, req.IdempotencyKey,
	)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	response.OK(c, fundsResponse(value))
}

func (h *Handler) payWithWechat(c *gin.Context) {
	var req struct {
		ExpectedRowVersion uint64 `json:"expectedRowVersion"`
		ClientRequestID    string `json:"clientRequestId"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	value, err := h.funds.StartOrderPayment(
		c.Request.Context(), h.currentUserID(c), c.Param("id"),
		req.ExpectedRowVersion, req.ClientRequestID, h.provider.Provider(),
	)
	if err != nil {
		h.write(c, nil, err)
		return
	}
	params, err := h.issuePrepay(c, &value, "校园配送服务费")
	if err != nil {
		h.write(c, nil, err)
		return
	}
	result := FundsResponse{Payment: pointer(paymentView(value.Attempt, params))}
	if value.Order != nil {
		result.Order = pointer(orderView(*value.Order))
	}
	response.OK(c, result)
}

func (h *Handler) issuePrepay(c *gin.Context, value *delivery.PaymentStartResult, description string) (*JSAPIParameter, error) {
	if value.Attempt.Provider == delivery.PaymentMock {
		return nil, nil
	}
	if value.Attempt.Status == delivery.PaymentPrepayIssued && value.Attempt.PrepayID != nil {
		return h.provider.ClientParams(*value.Attempt.PrepayID)
	}
	prepay, err := h.provider.CreatePrepay(c.Request.Context(), PrepayRequest{
		Description: description, OutTradeNo: value.Attempt.OutTradeNo,
		AmountCents: value.Attempt.AmountCents, OpenID: h.currentOpenID(c),
	})
	if err != nil {
		return nil, err
	}
	value.Attempt, err = h.funds.MarkPrepayIssued(
		c.Request.Context(), value.Attempt.PaymentPublicID,
		value.Attempt.RowVersion, prepay.PrepayID,
	)
	if err != nil {
		return nil, err
	}
	return &prepay.JSAPI, nil
}

func (h *Handler) merchantRefund(c *gin.Context) {
	h.refund(c, false)
}

func (h *Handler) adminRefund(c *gin.Context) {
	h.refund(c, true)
}

func (h *Handler) refund(c *gin.Context, admin bool) {
	var req struct {
		ExpectedRowVersion uint64 `json:"expectedRowVersion"`
		IdempotencyKey     string `json:"idempotencyKey"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	var (
		value delivery.RefundResult
		err   error
	)
	if admin {
		value, err = h.funds.RequestAdminRefund(c.Request.Context(), c.Param("id"), req.ExpectedRowVersion, req.IdempotencyKey)
	} else {
		value, err = h.funds.RequestMerchantRefund(
			c.Request.Context(), h.currentUserID(c), c.Param("id"),
			req.ExpectedRowVersion, req.IdempotencyKey,
		)
	}
	if err != nil {
		h.write(c, nil, err)
		return
	}
	if value.Refund.Source == delivery.SettlementWechat &&
		value.Refund.Status != delivery.RefundSucceeded {
		attempt, findErr := h.funds.Payment(
			c.Request.Context(), value.Settlement.MerchantUserID, *value.Settlement.PaymentPublicID,
		)
		if findErr != nil {
			h.write(c, nil, findErr)
			return
		}
		providerResult, providerErr := h.provider.CreateRefund(c.Request.Context(), RefundRequest{
			OutTradeNo: attempt.OutTradeNo, OutRefundNo: value.Refund.OutRefundNo,
			AmountCents: value.Refund.AmountCents, Reason: "配送服务取消退款",
		})
		success := providerErr == nil && providerResult.Status == "SUCCESS"
		reason := ""
		if providerErr != nil {
			reason = providerErr.Error()
		} else if !success {
			reason = "wechat refund accepted; awaiting callback"
		}
		value, err = h.funds.CompleteRefund(
			c.Request.Context(), value.Refund.OutRefundNo,
			providerResult.WechatRefundID, success, reason,
		)
		if err != nil {
			h.write(c, nil, err)
			return
		}
	}
	response.OK(c, refundResponse(value))
}

func (h *Handler) paymentNotify(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid payment notification")
		return
	}
	value, err := h.provider.ParsePaymentNotification(c.Request, body)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid payment notification")
		return
	}
	_, _, err = h.funds.ConfirmWechatPayment(c.Request.Context(), delivery.PaymentConfirmation{
		OutTradeNo: value.OutTradeNo, WechatTransactionID: value.WechatTransactionID,
		AmountCents: value.AmountCents, RawBody: body, PaidAt: value.PaidAt,
	})
	if err != nil {
		h.write(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) refundNotify(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid refund notification")
		return
	}
	value, err := h.provider.ParseRefundNotification(c.Request, body)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid refund notification")
		return
	}
	if _, err := h.funds.CompleteRefund(
		c.Request.Context(), value.OutRefundNo, value.WechatRefundID,
		value.Success, value.Reason,
	); err != nil {
		h.write(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) write(c *gin.Context, value any, err error) {
	switch {
	case err == nil:
		response.OK(c, value)
	case errors.Is(err, delivery.ErrForbiddenOrder), errors.Is(err, delivery.ErrNotFound),
		errors.Is(err, delivery.ErrPaymentForbidden):
		response.Fail(c, http.StatusNotFound, "delivery funds record not found")
	case errors.Is(err, delivery.ErrRefundNotAllowed):
		response.Fail(c, http.StatusForbidden, err.Error())
	case errors.Is(err, delivery.ErrWalletInsufficient):
		response.Fail(c, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, ErrProviderNotConfigured):
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, ErrProviderRejected):
		response.Fail(c, http.StatusBadGateway, err.Error())
	case errors.Is(err, delivery.ErrConcurrentUpdate), errors.Is(err, delivery.ErrInvalidTransition),
		errors.Is(err, delivery.ErrIdempotencyConflict), errors.Is(err, delivery.ErrAlreadySettled),
		errors.Is(err, delivery.ErrPaymentNotPending), errors.Is(err, delivery.ErrPaymentAmount):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, delivery.ErrInvalidRecord), errors.Is(err, delivery.ErrInvalidULID),
		errors.Is(err, ErrInvalidNotification):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		response.Fail(c, http.StatusInternalServerError, "delivery funds operation failed")
	}
}

func parsePage(c *gin.Context) (delivery.Page, bool) {
	page, pageErr := strconv.Atoi(defaultQuery(c, "page", "1"))
	size, sizeErr := strconv.Atoi(defaultQuery(c, "pageSize", "20"))
	value := delivery.Page{Page: page, Size: size}
	if pageErr != nil || sizeErr != nil || !value.Valid() {
		response.Fail(c, http.StatusBadRequest, "invalid pagination")
		return delivery.Page{}, false
	}
	return value, true
}

func defaultQuery(c *gin.Context, key, fallback string) string {
	if value := strings.TrimSpace(c.Query(key)); value != "" {
		return value
	}
	return fallback
}

func fundsResponse(value delivery.FundsResult) FundsResponse {
	result := FundsResponse{}
	if value.Wallet != nil {
		result.Wallet = pointer(walletView(*value.Wallet))
	}
	if value.Ledger != nil {
		result.Ledger = pointer(ledgerView(*value.Ledger))
	}
	if value.Attempt != nil {
		result.Payment = pointer(paymentView(*value.Attempt, nil))
	}
	if value.Settlement != nil {
		result.Settlement = pointer(settlementView(*value.Settlement))
	}
	if value.Order != nil {
		result.Order = pointer(orderView(*value.Order))
	}
	return result
}

func refundResponse(value delivery.RefundResult) FundsResponse {
	result := FundsResponse{
		Refund: pointer(refundView(value.Refund)), Settlement: pointer(settlementView(value.Settlement)),
		Order: pointer(orderView(value.Order)),
	}
	if value.Wallet != nil {
		result.Wallet = pointer(walletView(*value.Wallet))
	}
	if value.Ledger != nil {
		result.Ledger = pointer(ledgerView(*value.Ledger))
	}
	return result
}

func pointer[T any](value T) *T { return &value }
