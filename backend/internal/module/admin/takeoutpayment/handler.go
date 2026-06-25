package takeoutpayment

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/pkg/response"
)

type Handler struct {
	payments      *takeout.PaymentService
	provider      Provider
	currentUserID func(*gin.Context) uint64
	currentOpenID func(*gin.Context) string
}

func NewHandler(
	payments *takeout.PaymentService,
	provider Provider,
	currentUserID func(*gin.Context) uint64,
	currentOpenID func(*gin.Context) string,
) *Handler {
	return &Handler{payments: payments, provider: provider, currentUserID: currentUserID, currentOpenID: currentOpenID}
}

func (h *Handler) RegisterBuyer(group *gin.RouterGroup) {
	group.POST("/orders/:orderPublicId/pay", h.payOrder)
}

func (h *Handler) RegisterWebhook(group *gin.RouterGroup) {
	group.POST("/wechat/notify", h.wechatNotify)
}

func (h *Handler) payOrder(c *gin.Context) {
	switch h.provider.Provider() {
	case takeout.PaymentProviderMock:
		result, err := h.payments.PayWithLocalMock(c.Request.Context(), h.currentUserID(c), c.Param("orderPublicId"))
		h.write(c, paymentResponse("mock", result, nil), err)
	case takeout.PaymentProviderWechat:
		result, err := h.payments.CreateWechatPaymentAttempt(c.Request.Context(), h.currentUserID(c), c.Param("orderPublicId"))
		if err != nil {
			h.write(c, nil, err)
			return
		}
		var params *JSAPIPaymentParameter
		if result.Attempt.Status == takeout.PaymentPrepayIssued && result.Attempt.PrepayID != nil {
			params, err = h.provider.ClientParams(*result.Attempt.PrepayID)
		} else {
			prepay, providerErr := h.provider.CreatePrepay(c.Request.Context(), PrepayRequest{
				Description: encodeDescription("校园外卖订单 " + result.Order.OrderPublicID),
				OutTradeNo:  result.Attempt.OutTradeNo,
				AmountCents: result.Order.TotalAmountCents,
				OpenID:      h.currentOpenID(c),
			})
			if providerErr != nil {
				h.write(c, nil, providerErr)
				return
			}
			result.Attempt, err = h.payments.MarkPrepayIssued(c.Request.Context(), result.Attempt.PaymentPublicID, result.Attempt.RowVersion, prepay.PrepayID)
			params = &prepay.JSAPI
		}
		h.write(c, paymentResponse("wechat", result, params), err)
	default:
		h.write(c, nil, ErrProviderNotConfigured)
	}
}

func (h *Handler) wechatNotify(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid payment notification")
		return
	}
	notification, err := h.provider.ParseNotification(c.Request, body)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "invalid payment notification")
		return
	}
	_, _, err = h.payments.ConfirmProviderPayment(c.Request.Context(), takeout.PaymentConfirmationInput{
		Provider:            takeout.PaymentProviderWechat,
		OutTradeNo:          notification.OutTradeNo,
		WechatTransactionID: notification.WechatTransactionID,
		AmountCents:         notification.AmountCents,
		RawBody:             body,
		PaidAt:              notification.PaidAt,
	})
	if err != nil {
		h.write(c, nil, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) write(c *gin.Context, value any, err error) {
	switch {
	case err == nil:
		response.OK(c, value)
	case errors.Is(err, takeout.ErrNotFound), errors.Is(err, takeout.ErrPaymentForbidden):
		response.Fail(c, http.StatusNotFound, "takeout payment record not found")
	case errors.Is(err, ErrProviderNotConfigured):
		response.Fail(c, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, ErrProviderRejected):
		response.Fail(c, http.StatusBadGateway, err.Error())
	case errors.Is(err, takeout.ErrPaymentNotPending), errors.Is(err, takeout.ErrConcurrentUpdate),
		errors.Is(err, takeout.ErrInvalidTransition), errors.Is(err, takeout.ErrPaymentAmount):
		response.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, takeout.ErrInvalidRecord), errors.Is(err, takeout.ErrInvalidULID), errors.Is(err, ErrInvalidNotification):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		response.Fail(c, http.StatusInternalServerError, "takeout payment operation failed")
	}
}
