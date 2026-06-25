// Package takeoutpayment exposes the G-4 payment transport adapter.
// Domain rules and payment records remain in internal/module/takeout.
package takeoutpayment

import (
	"time"

	"campus-circle/backend/internal/module/takeout"
)

type PaymentResponse struct {
	Mode            string                 `json:"mode"`
	PaymentPublicID string                 `json:"paymentPublicId"`
	OrderPublicID   string                 `json:"orderPublicId"`
	AmountCents     uint64                 `json:"amountCents"`
	OutTradeNo      string                 `json:"outTradeNo"`
	Status          takeout.PaymentStatus  `json:"status"`
	Order           PaymentOrderView       `json:"order"`
	Wechat          *JSAPIPaymentParameter `json:"wechat,omitempty"`
}

type PaymentOrderView struct {
	OrderPublicID    string              `json:"orderPublicId"`
	Status           takeout.OrderStatus `json:"status"`
	TotalAmountCents uint64              `json:"totalAmountCents"`
	RowVersion       uint64              `json:"rowVersion"`
	PaidAt           *time.Time          `json:"paidAt,omitempty"`
}

type JSAPIPaymentParameter struct {
	TimeStamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
	Package   string `json:"package"`
	SignType  string `json:"signType"`
	PaySign   string `json:"paySign"`
}

func paymentResponse(mode string, value takeout.PaymentAttemptResult, wechat *JSAPIPaymentParameter) PaymentResponse {
	return PaymentResponse{
		Mode:            mode,
		PaymentPublicID: value.Attempt.PaymentPublicID,
		OrderPublicID:   value.Order.OrderPublicID,
		AmountCents:     value.Attempt.AmountCents,
		OutTradeNo:      value.Attempt.OutTradeNo,
		Status:          value.Attempt.Status,
		Order: PaymentOrderView{
			OrderPublicID: value.Order.OrderPublicID, Status: value.Order.Status,
			TotalAmountCents: value.Order.TotalAmountCents, RowVersion: value.Order.RowVersion,
			PaidAt: value.Order.PaidAt,
		},
		Wechat: wechat,
	}
}
