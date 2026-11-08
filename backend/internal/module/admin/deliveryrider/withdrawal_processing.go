package deliveryrider

import (
	"net/http"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *Handler) withdrawalCapabilities(c *gin.Context) {
	response.OK(c, gin.H{"withdrawalEnabled": h.withdrawalWritesReady, "mode": "manual", "claimRequired": true})
}

func (h *Handler) withdrawalProcessing(c *gin.Context) {
	processor, ok := h.withdrawals.(delivery.RiderWithdrawalProcessingRepository)
	if !ok {
		response.Fail(c, http.StatusServiceUnavailable, "提现处理状态暂不可用")
		return
	}
	if !delivery.ValidULID(c.Param("id")) {
		response.Fail(c, http.StatusBadRequest, "invalid withdrawal id")
		return
	}
	value, err := processor.FindRiderWithdrawal(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.write(c, nil, err)
		return
	}
	claim, err := processor.ReadRiderWithdrawalClaim(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.write(c, nil, err)
		return
	}
	status, claimKey := "unclaimed", ""
	owned := claim.ID != 0 && h.currentUserID != nil && claim.OperatorUserID == h.currentUserID(c)
	if claim.ID != 0 {
		status = string(claim.Status)
	}
	if owned {
		claimKey = claim.ClaimKey
	}
	response.OK(c, gin.H{"withdrawal": riderWithdrawalView(value), "status": status, "ownedByMe": owned,
		"claimKey": claimKey, "operatorUserId": claim.OperatorUserID, "reason": claim.Reason, "paymentReference": claim.PaymentReference})
}

func (h *Handler) processWithdrawal(c *gin.Context, kind string) {
	if !h.withdrawalWritesReady {
		response.Fail(c, http.StatusForbidden, "提现处理尚未开放，请勿据此付款或调整余额")
		return
	}
	processor, ok := h.withdrawals.(delivery.RiderWithdrawalProcessingRepository)
	if !ok || h.currentUserID == nil {
		response.Fail(c, http.StatusServiceUnavailable, "提现处理暂不可用")
		return
	}
	operator := h.currentUserID(c)
	if operator == 0 {
		response.Fail(c, http.StatusUnauthorized, "authentication required")
		return
	}
	var req struct {
		ExpectedRowVersion uint64 `json:"expectedRowVersion"`
		IdempotencyKey     string `json:"idempotencyKey"`
		ClaimKey           string `json:"claimKey"`
		Reason             string `json:"reason"`
		PaymentReference   string `json:"paymentReference"`
		ConfirmedUnpaid    bool   `json:"confirmedUnpaid"`
	}
	if c.ShouldBindJSON(&req) != nil {
		response.Fail(c, http.StatusBadRequest, "invalid request")
		return
	}
	now := h.now
	if now == nil {
		now = time.Now
	}
	value, err := processor.ProcessRiderWithdrawal(c.Request.Context(), kind, delivery.RiderWithdrawalAction{
		WithdrawPublicID: c.Param("id"), ExpectedRowVersion: req.ExpectedRowVersion, OperatorUserID: operator,
		IdempotencyKey: req.IdempotencyKey, ClaimKey: req.ClaimKey, Reason: req.Reason, PaymentReference: req.PaymentReference, ConfirmedUnpaid: req.ConfirmedUnpaid}, now())
	h.write(c, riderWithdrawalView(value), err)
}
