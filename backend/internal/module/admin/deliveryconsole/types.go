package deliveryconsole

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

type SummaryView struct {
	Orders                 int64  `json:"orders"`
	PendingFeePayment      int64  `json:"pendingFeePayment"`
	ReadyForClaim          int64  `json:"readyForClaim"`
	InProgress             int64  `json:"inProgress"`
	Delivered              int64  `json:"delivered"`
	Cancelled              int64  `json:"cancelled"`
	TodayOrders            int64  `json:"todayOrders"`
	TodayDelivered         int64  `json:"todayDelivered"`
	MerchantWalletCents    uint64 `json:"merchantWalletCents"`
	RiderWalletCents       uint64 `json:"riderWalletCents"`
	PendingRiderFeeCents   uint64 `json:"pendingRiderFeeCents"`
	PaidOfflineCents       uint64 `json:"paidOfflineCents"`
	PlatformFeeIncomeCents uint64 `json:"platformFeeIncomeCents"`
}

type FeeConfigView struct {
	ConfigScope      string                   `json:"configScope"`
	DeliveryFeeCents uint64                   `json:"deliveryFeeCents"`
	PlatformCutCents uint64                   `json:"platformCutCents"`
	Status           delivery.FeeConfigStatus `json:"status"`
	RowVersion       uint64                   `json:"rowVersion"`
	CreatedAt        time.Time                `json:"createdAt"`
	UpdatedAt        time.Time                `json:"updatedAt"`
}

type MerchantFeeOverrideView struct {
	MerchantUserID   uint64                   `json:"merchantUserId"`
	DeliveryFeeCents uint64                   `json:"deliveryFeeCents"`
	PlatformCutCents uint64                   `json:"platformCutCents"`
	Status           delivery.FeeConfigStatus `json:"status"`
	RowVersion       uint64                   `json:"rowVersion"`
	CreatedAt        time.Time                `json:"createdAt"`
	UpdatedAt        time.Time                `json:"updatedAt"`
}

type OrderView struct {
	DeliveryOrderPublicID string                  `json:"deliveryOrderPublicId"`
	MerchantRequestID     *string                 `json:"merchantRequestId,omitempty"`
	SourcePlatform        delivery.SourcePlatform `json:"sourcePlatform"`
	PickupCode            string                  `json:"pickupCode"`
	SourceOrderID         *string                 `json:"sourceOrderId,omitempty"`
	SourceShopID          *string                 `json:"sourceShopId,omitempty"`
	MerchantUserID        uint64                  `json:"merchantUserId"`
	RiderUserID           *uint64                 `json:"riderUserId,omitempty"`
	PickupContactName     string                  `json:"pickupContactName"`
	PickupContactPhone    string                  `json:"pickupContactPhone"`
	PickupAddress         string                  `json:"pickupAddress"`
	RecipientName         string                  `json:"recipientName"`
	RecipientPhone        string                  `json:"recipientPhone"`
	DeliveryAddress       string                  `json:"deliveryAddress"`
	Remark                string                  `json:"remark"`
	DeliveryFeeCents      uint64                  `json:"deliveryFeeCents"`
	RiderFeeCents         uint64                  `json:"riderFeeCents"`
	PlatformCutCents      uint64                  `json:"platformCutCents"`
	Status                delivery.Status         `json:"status"`
	RowVersion            uint64                  `json:"rowVersion"`
	ReadyForClaimAt       *time.Time              `json:"readyForClaimAt"`
	ClaimedAt             *time.Time              `json:"claimedAt"`
	ToPickupAt            *time.Time              `json:"toPickupAt"`
	PickedUpAt            *time.Time              `json:"pickedUpAt"`
	DeliveringAt          *time.Time              `json:"deliveringAt"`
	DeliveredAt           *time.Time              `json:"deliveredAt"`
	PaymentFailedAt       *time.Time              `json:"paymentFailedAt"`
	CancelledAt           *time.Time              `json:"cancelledAt"`
	CreatedAt             time.Time               `json:"createdAt"`
	UpdatedAt             time.Time               `json:"updatedAt"`
}

type MerchantWalletView struct {
	WalletPublicID        string    `json:"walletPublicId"`
	MerchantUserID        uint64    `json:"merchantUserId"`
	AvailableBalanceCents uint64    `json:"availableBalanceCents"`
	RowVersion            uint64    `json:"rowVersion"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

type MerchantLedgerView struct {
	LedgerPublicID     string                   `json:"ledgerPublicId"`
	WalletPublicID     string                   `json:"walletPublicId"`
	MerchantUserID     uint64                   `json:"merchantUserId"`
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
	MerchantUserID        uint64                   `json:"merchantUserId"`
	Purpose               delivery.PaymentPurpose  `json:"purpose"`
	DeliveryOrderPublicID *string                  `json:"deliveryOrderPublicId,omitempty"`
	Provider              delivery.PaymentProvider `json:"provider"`
	Status                delivery.PaymentStatus   `json:"status"`
	AmountCents           uint64                   `json:"amountCents"`
	OutTradeNo            string                   `json:"outTradeNo"`
	RowVersion            uint64                   `json:"rowVersion"`
	PaidAt                *time.Time               `json:"paidAt,omitempty"`
	CreatedAt             time.Time                `json:"createdAt"`
	UpdatedAt             time.Time                `json:"updatedAt"`
}

type SettlementView struct {
	SettlementPublicID    string                    `json:"settlementPublicId"`
	DeliveryOrderPublicID string                    `json:"deliveryOrderPublicId"`
	MerchantUserID        uint64                    `json:"merchantUserId"`
	Source                delivery.SettlementSource `json:"source"`
	AmountCents           uint64                    `json:"amountCents"`
	Status                delivery.SettlementStatus `json:"status"`
	PaymentPublicID       *string                   `json:"paymentPublicId,omitempty"`
	LedgerPublicID        *string                   `json:"ledgerPublicId,omitempty"`
	PaidAt                time.Time                 `json:"paidAt"`
	RefundedAt            *time.Time                `json:"refundedAt,omitempty"`
	RowVersion            uint64                    `json:"rowVersion"`
}

type RefundView struct {
	RefundPublicID        string                    `json:"refundPublicId"`
	SettlementPublicID    string                    `json:"settlementPublicId"`
	DeliveryOrderPublicID string                    `json:"deliveryOrderPublicId"`
	MerchantUserID        uint64                    `json:"merchantUserId"`
	Source                delivery.SettlementSource `json:"source"`
	AmountCents           uint64                    `json:"amountCents"`
	Status                delivery.RefundStatus     `json:"status"`
	OutRefundNo           string                    `json:"outRefundNo"`
	FailureReason         string                    `json:"failureReason,omitempty"`
	RowVersion            uint64                    `json:"rowVersion"`
	CreatedAt             time.Time                 `json:"createdAt"`
	UpdatedAt             time.Time                 `json:"updatedAt"`
	SucceededAt           *time.Time                `json:"succeededAt,omitempty"`
}

type OrderFundsView struct {
	Order      OrderView       `json:"order"`
	Settlement *SettlementView `json:"settlement"`
	Refund     *RefundView     `json:"refund"`
	Payment    *PaymentView    `json:"payment"`
}

type RiderWalletView struct {
	WalletPublicID        string    `json:"walletPublicId"`
	RiderUserID           uint64    `json:"riderUserId"`
	AvailableBalanceCents uint64    `json:"availableBalanceCents"`
	RowVersion            uint64    `json:"rowVersion"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

type RiderLedgerView struct {
	LedgerPublicID     string                        `json:"ledgerPublicId"`
	WalletPublicID     string                        `json:"walletPublicId"`
	RiderUserID        uint64                        `json:"riderUserId"`
	Direction          delivery.RiderLedgerDirection `json:"direction"`
	EntryType          delivery.RiderLedgerEntryType `json:"entryType"`
	AmountCents        uint64                        `json:"amountCents"`
	BalanceBeforeCents uint64                        `json:"balanceBeforeCents"`
	BalanceAfterCents  uint64                        `json:"balanceAfterCents"`
	ReferenceType      string                        `json:"referenceType"`
	ReferencePublicID  string                        `json:"referencePublicId"`
	CreatedAt          time.Time                     `json:"createdAt"`
}

type RiderEarningView struct {
	EarningPublicID       string                      `json:"earningPublicId"`
	DeliveryOrderPublicID string                      `json:"deliveryOrderPublicId"`
	RiderUserID           uint64                      `json:"riderUserId"`
	DeliveryFeeCents      uint64                      `json:"deliveryFeeCents"`
	PlatformCutCents      uint64                      `json:"platformCutCents"`
	RiderFeeCents         uint64                      `json:"riderFeeCents"`
	SettlementStatus      delivery.RiderEarningStatus `json:"settlementStatus"`
	PaidOfflineAt         *time.Time                  `json:"paidOfflineAt,omitempty"`
	RowVersion            uint64                      `json:"rowVersion"`
	CreatedAt             time.Time                   `json:"createdAt"`
	UpdatedAt             time.Time                   `json:"updatedAt"`
}

type RiderPayoutView struct {
	PayoutPublicID        string    `json:"payoutPublicId"`
	EarningPublicID       string    `json:"earningPublicId"`
	DeliveryOrderPublicID string    `json:"deliveryOrderPublicId"`
	RiderUserID           uint64    `json:"riderUserId"`
	OperatorUserID        uint64    `json:"operatorUserId"`
	AmountCents           uint64    `json:"amountCents"`
	IdempotencyKey        string    `json:"idempotencyKey"`
	Note                  string    `json:"note"`
	PaidOfflineAt         time.Time `json:"paidOfflineAt"`
	RowVersion            uint64    `json:"rowVersion"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

type RiderPayoutResultView struct {
	Wallet  RiderWalletView  `json:"wallet"`
	Earning RiderEarningView `json:"earning"`
	Ledger  RiderLedgerView  `json:"ledger"`
	Payout  RiderPayoutView  `json:"payout"`
}

func feeConfigView(value delivery.DeliveryFeeConfig) FeeConfigView {
	return FeeConfigView{
		ConfigScope: value.ConfigScope, DeliveryFeeCents: value.DeliveryFeeCents,
		PlatformCutCents: value.PlatformCutCents, Status: value.Status, RowVersion: value.RowVersion,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func merchantFeeOverrideView(value delivery.MerchantFeeOverride) MerchantFeeOverrideView {
	return MerchantFeeOverrideView{
		MerchantUserID: value.MerchantUserID, DeliveryFeeCents: value.DeliveryFeeCents,
		PlatformCutCents: value.PlatformCutCents, Status: value.Status, RowVersion: value.RowVersion,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func orderView(value delivery.Order) OrderView {
	return OrderView{
		DeliveryOrderPublicID: value.DeliveryOrderPublicID, MerchantRequestID: value.MerchantRequestID,
		SourcePlatform: value.SourcePlatform.Effective(), PickupCode: value.PickupCode,
		SourceOrderID: value.SourceOrderID, SourceShopID: value.SourceShopID,
		MerchantUserID: value.MerchantUserID, RiderUserID: value.RiderUserID,
		PickupContactName: value.PickupContactName, PickupContactPhone: value.PickupContactPhone,
		PickupAddress: value.PickupAddress, RecipientName: value.RecipientName,
		RecipientPhone: value.RecipientPhone, DeliveryAddress: value.DeliveryAddress,
		Remark: value.Remark, DeliveryFeeCents: value.DeliveryFeeCents,
		RiderFeeCents: value.RiderFeeCents, PlatformCutCents: value.PlatformCutCents,
		Status: value.Status, RowVersion: value.RowVersion,
		ReadyForClaimAt: value.ReadyForClaimAt, ClaimedAt: value.ClaimedAt,
		ToPickupAt: value.ToPickupAt, PickedUpAt: value.PickedUpAt,
		DeliveringAt: value.DeliveringAt, DeliveredAt: value.DeliveredAt,
		PaymentFailedAt: value.PaymentFailedAt, CancelledAt: value.CancelledAt,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func merchantWalletView(value delivery.MerchantWallet) MerchantWalletView {
	return MerchantWalletView{
		WalletPublicID: value.WalletPublicID, MerchantUserID: value.MerchantUserID,
		AvailableBalanceCents: value.AvailableBalanceCents, RowVersion: value.RowVersion,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func merchantLedgerView(value delivery.WalletLedger) MerchantLedgerView {
	return MerchantLedgerView{
		LedgerPublicID: value.LedgerPublicID, WalletPublicID: value.WalletPublicID,
		MerchantUserID: value.MerchantUserID, Direction: value.Direction, EntryType: value.EntryType,
		AmountCents: value.AmountCents, BalanceBeforeCents: value.BalanceBeforeCents,
		BalanceAfterCents: value.BalanceAfterCents, ReferenceType: value.ReferenceType,
		ReferencePublicID: value.ReferencePublicID, CreatedAt: value.CreatedAt,
	}
}

func paymentView(value delivery.PaymentAttempt) PaymentView {
	return PaymentView{
		PaymentPublicID: value.PaymentPublicID, MerchantUserID: value.MerchantUserID,
		Purpose: value.Purpose, DeliveryOrderPublicID: value.DeliveryOrderPublicID,
		Provider: value.Provider, Status: value.Status, AmountCents: value.AmountCents,
		OutTradeNo: value.OutTradeNo, RowVersion: value.RowVersion,
		PaidAt: value.PaidAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func settlementView(value delivery.FeeSettlement) SettlementView {
	return SettlementView{
		SettlementPublicID: value.SettlementPublicID, DeliveryOrderPublicID: value.DeliveryOrderPublicID,
		MerchantUserID: value.MerchantUserID, Source: value.Source, AmountCents: value.AmountCents,
		Status: value.Status, PaymentPublicID: value.PaymentPublicID, LedgerPublicID: value.LedgerPublicID,
		PaidAt: value.PaidAt, RefundedAt: value.RefundedAt, RowVersion: value.RowVersion,
	}
}

func refundView(value delivery.Refund) RefundView {
	return RefundView{
		RefundPublicID: value.RefundPublicID, SettlementPublicID: value.SettlementPublicID,
		DeliveryOrderPublicID: value.DeliveryOrderPublicID, MerchantUserID: value.MerchantUserID,
		Source: value.Source, AmountCents: value.AmountCents, Status: value.Status,
		OutRefundNo: value.OutRefundNo, FailureReason: value.FailureReason,
		RowVersion: value.RowVersion, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		SucceededAt: value.SucceededAt,
	}
}

func orderFundsView(value OrderFundsSnapshot) OrderFundsView {
	result := OrderFundsView{Order: orderView(value.Order)}
	if value.Settlement != nil {
		view := settlementView(*value.Settlement)
		result.Settlement = &view
	}
	if value.Refund != nil {
		view := refundView(*value.Refund)
		result.Refund = &view
	}
	if value.Payment != nil {
		view := paymentView(*value.Payment)
		result.Payment = &view
	}
	return result
}

func riderWalletView(value delivery.RiderWallet) RiderWalletView {
	return RiderWalletView{
		WalletPublicID: value.WalletPublicID, RiderUserID: value.RiderUserID,
		AvailableBalanceCents: value.AvailableBalanceCents, RowVersion: value.RowVersion,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func riderLedgerView(value delivery.RiderLedger) RiderLedgerView {
	return RiderLedgerView{
		LedgerPublicID: value.LedgerPublicID, WalletPublicID: value.WalletPublicID,
		RiderUserID: value.RiderUserID, Direction: value.Direction, EntryType: value.EntryType,
		AmountCents: value.AmountCents, BalanceBeforeCents: value.BalanceBeforeCents,
		BalanceAfterCents: value.BalanceAfterCents, ReferenceType: value.ReferenceType,
		ReferencePublicID: value.ReferencePublicID, CreatedAt: value.CreatedAt,
	}
}

func riderEarningView(value delivery.RiderEarningRecord) RiderEarningView {
	return RiderEarningView{
		EarningPublicID: value.EarningPublicID, DeliveryOrderPublicID: value.DeliveryOrderPublicID,
		RiderUserID: value.RiderUserID, DeliveryFeeCents: value.DeliveryFeeCents,
		PlatformCutCents: value.PlatformCutCents, RiderFeeCents: value.RiderFeeCents,
		SettlementStatus: value.SettlementStatus, PaidOfflineAt: value.PaidOfflineAt,
		RowVersion: value.RowVersion, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func riderPayoutView(value delivery.RiderPayoutRecord) RiderPayoutView {
	return RiderPayoutView{
		PayoutPublicID: value.PayoutPublicID, EarningPublicID: value.EarningPublicID,
		DeliveryOrderPublicID: value.DeliveryOrderPublicID, RiderUserID: value.RiderUserID,
		OperatorUserID: value.OperatorUserID, AmountCents: value.AmountCents,
		IdempotencyKey: value.IdempotencyKey, Note: value.Note,
		PaidOfflineAt: value.PaidOfflineAt, RowVersion: value.RowVersion,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func riderPayoutResultView(value delivery.RiderPayoutResult) RiderPayoutResultView {
	return RiderPayoutResultView{
		Wallet: riderWalletView(value.Wallet), Earning: riderEarningView(value.Earning),
		Ledger: riderLedgerView(value.Ledger), Payout: riderPayoutView(value.Payout),
	}
}

func mapValues[A, B any](values []A, convert func(A) B) []B {
	result := make([]B, 0, len(values))
	for _, value := range values {
		result = append(result, convert(value))
	}
	return result
}
