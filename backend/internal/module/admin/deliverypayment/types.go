package deliverypayment

import (
	"time"

	"campus-circle/backend/internal/module/delivery"
)

type Page[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type WalletView struct {
	WalletPublicID        string    `json:"walletPublicId"`
	AvailableBalanceCents uint64    `json:"availableBalanceCents"`
	RowVersion            uint64    `json:"rowVersion"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

type LedgerView struct {
	LedgerPublicID     string                   `json:"ledgerPublicId"`
	Direction          delivery.LedgerDirection `json:"direction"`
	EntryType          delivery.LedgerEntryType `json:"entryType"`
	AmountCents        uint64                   `json:"amountCents"`
	BalanceBeforeCents uint64                   `json:"balanceBeforeCents"`
	BalanceAfterCents  uint64                   `json:"balanceAfterCents"`
	ReferenceType      string                   `json:"referenceType"`
	ReferencePublicID  string                   `json:"referencePublicId"`
	CreatedAt          time.Time                `json:"createdAt"`
}

type PaymentView struct {
	PaymentPublicID       string                   `json:"paymentPublicId"`
	Purpose               delivery.PaymentPurpose  `json:"purpose"`
	DeliveryOrderPublicID *string                  `json:"deliveryOrderPublicId,omitempty"`
	Provider              delivery.PaymentProvider `json:"provider"`
	Status                delivery.PaymentStatus   `json:"status"`
	AmountCents           uint64                   `json:"amountCents"`
	OutTradeNo            string                   `json:"outTradeNo"`
	RowVersion            uint64                   `json:"rowVersion"`
	PaidAt                *time.Time               `json:"paidAt,omitempty"`
	Wechat                *JSAPIParameter          `json:"wechat,omitempty"`
}

type SettlementView struct {
	SettlementPublicID    string                    `json:"settlementPublicId"`
	DeliveryOrderPublicID string                    `json:"deliveryOrderPublicId"`
	Source                delivery.SettlementSource `json:"source"`
	AmountCents           uint64                    `json:"amountCents"`
	Status                delivery.SettlementStatus `json:"status"`
	PaidAt                time.Time                 `json:"paidAt"`
	RefundedAt            *time.Time                `json:"refundedAt,omitempty"`
}

type RefundView struct {
	RefundPublicID        string                    `json:"refundPublicId"`
	DeliveryOrderPublicID string                    `json:"deliveryOrderPublicId"`
	Source                delivery.SettlementSource `json:"source"`
	AmountCents           uint64                    `json:"amountCents"`
	Status                delivery.RefundStatus     `json:"status"`
	OutRefundNo           string                    `json:"outRefundNo"`
	FailureReason         string                    `json:"failureReason,omitempty"`
	CreatedAt             time.Time                 `json:"createdAt"`
	UpdatedAt             time.Time                 `json:"updatedAt"`
}

type OrderView struct {
	DeliveryOrderPublicID string          `json:"deliveryOrderPublicId"`
	DeliveryFeeCents      uint64          `json:"deliveryFeeCents"`
	Status                delivery.Status `json:"status"`
	RowVersion            uint64          `json:"rowVersion"`
}

type FundsResponse struct {
	Wallet     *WalletView     `json:"wallet,omitempty"`
	Ledger     *LedgerView     `json:"ledger,omitempty"`
	Payment    *PaymentView    `json:"payment,omitempty"`
	Settlement *SettlementView `json:"settlement,omitempty"`
	Order      *OrderView      `json:"order,omitempty"`
	Refund     *RefundView     `json:"refund,omitempty"`
}

type OrderFundsView struct {
	DeliveryOrderPublicID string          `json:"deliveryOrderPublicId"`
	Settlement            *SettlementView `json:"settlement"`
	Refund                *RefundView     `json:"refund"`
	Payment               *PaymentView    `json:"payment"`
}

func walletView(value delivery.MerchantWallet) WalletView {
	return WalletView{
		WalletPublicID: value.WalletPublicID, AvailableBalanceCents: value.AvailableBalanceCents,
		RowVersion: value.RowVersion, UpdatedAt: value.UpdatedAt,
	}
}

func ledgerView(value delivery.WalletLedger) LedgerView {
	return LedgerView{
		LedgerPublicID: value.LedgerPublicID, Direction: value.Direction, EntryType: value.EntryType,
		AmountCents: value.AmountCents, BalanceBeforeCents: value.BalanceBeforeCents,
		BalanceAfterCents: value.BalanceAfterCents, ReferenceType: value.ReferenceType,
		ReferencePublicID: value.ReferencePublicID, CreatedAt: value.CreatedAt,
	}
}

func paymentView(value delivery.PaymentAttempt, params *JSAPIParameter) PaymentView {
	return PaymentView{
		PaymentPublicID: value.PaymentPublicID, Purpose: value.Purpose,
		DeliveryOrderPublicID: value.DeliveryOrderPublicID, Provider: value.Provider,
		Status: value.Status, AmountCents: value.AmountCents, OutTradeNo: value.OutTradeNo,
		RowVersion: value.RowVersion, PaidAt: value.PaidAt, Wechat: params,
	}
}

func settlementView(value delivery.FeeSettlement) SettlementView {
	return SettlementView{
		SettlementPublicID:    value.SettlementPublicID,
		DeliveryOrderPublicID: value.DeliveryOrderPublicID, Source: value.Source,
		AmountCents: value.AmountCents, Status: value.Status,
		PaidAt: value.PaidAt, RefundedAt: value.RefundedAt,
	}
}

func refundView(value delivery.Refund) RefundView {
	return RefundView{
		RefundPublicID: value.RefundPublicID, DeliveryOrderPublicID: value.DeliveryOrderPublicID,
		Source: value.Source, AmountCents: value.AmountCents, Status: value.Status,
		OutRefundNo: value.OutRefundNo, FailureReason: value.FailureReason,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func orderView(value delivery.Order) OrderView {
	return OrderView{
		DeliveryOrderPublicID: value.DeliveryOrderPublicID,
		DeliveryFeeCents:      value.DeliveryFeeCents,
		Status:                value.Status,
		RowVersion:            value.RowVersion,
	}
}

func orderFundsView(value delivery.OrderFundsSnapshot) OrderFundsView {
	result := OrderFundsView{DeliveryOrderPublicID: value.Order.DeliveryOrderPublicID}
	if value.Settlement != nil {
		result.Settlement = pointer(settlementView(*value.Settlement))
	}
	if value.Refund != nil {
		result.Refund = pointer(refundView(*value.Refund))
	}
	if value.Payment != nil {
		result.Payment = pointer(paymentView(*value.Payment, nil))
	}
	return result
}
