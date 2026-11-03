package deliveryrider

import (
	"time"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/takeout"
)

type Page[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type RiderView struct {
	RiderUserID uint64              `json:"riderUserId"`
	Status      takeout.RiderStatus `json:"status"`
	RealName    string              `json:"realName"`
	Phone       string              `json:"phone"`
	RowVersion  uint64              `json:"rowVersion"`
}

type MeView struct {
	Registered bool       `json:"registered"`
	Message    string     `json:"message,omitempty"`
	Rider      *RiderView `json:"rider,omitempty"`
}

type TodayStatsView struct {
	Date             string `json:"date"`
	Timezone         string `json:"timezone"`
	TodayOrders      int64  `json:"todayOrders"`
	TodayIncomeCents uint64 `json:"todayIncomeCents"`
	Definition       string `json:"definition"`
}

type RiderWalletView struct {
	WithdrawalEnabled      bool                      `json:"withdrawalEnabled"`
	WalletState            delivery.RiderWalletState `json:"walletState"`
	WalletPublicID         string                    `json:"walletPublicId,omitempty"`
	AvailableBalanceCents  uint64                    `json:"availableBalanceCents"`
	ReservedCents          uint64                    `json:"reservedCents"`
	UnverifiedCents        uint64                    `json:"unverifiedCents"`
	PendingSettlementCents uint64                    `json:"pendingSettlementCents"`
	NextSettlementAt       *time.Time                `json:"nextSettlementAt,omitempty"`
	UpdatedAt              *time.Time                `json:"updatedAt,omitempty"`
}

type RiderWalletEarningView struct {
	EarningPublicID       string                      `json:"earningPublicId"`
	DeliveryOrderPublicID string                      `json:"deliveryOrderPublicId"`
	AmountCents           uint64                      `json:"amountCents"`
	SettlementStatus      delivery.RiderEarningStatus `json:"settlementStatus"`
	SettleAt              *time.Time                  `json:"settleAt,omitempty"`
	SettledAt             *time.Time                  `json:"settledAt,omitempty"`
	CreatedAt             time.Time                   `json:"createdAt"`
	PaidOfflineAt         *time.Time                  `json:"paidOfflineAt,omitempty"`
}

type RiderWalletLedgerView struct {
	LedgerPublicID     string                        `json:"ledgerPublicId"`
	Direction          delivery.RiderLedgerDirection `json:"direction"`
	EntryType          delivery.RiderLedgerEntryType `json:"entryType"`
	AmountCents        uint64                        `json:"amountCents"`
	BalanceBeforeCents uint64                        `json:"balanceBeforeCents"`
	BalanceAfterCents  uint64                        `json:"balanceAfterCents"`
	ReferencePublicID  string                        `json:"referencePublicId,omitempty"`
	CreatedAt          time.Time                     `json:"createdAt"`
}

type RiderWalletQueueView struct {
	PayoutQueuePublicID   string    `json:"payoutQueuePublicId"`
	EarningPublicID       string    `json:"earningPublicId"`
	DeliveryOrderPublicID string    `json:"deliveryOrderPublicId"`
	AmountCents           uint64    `json:"amountCents"`
	Status                string    `json:"status"`
	ReasonCode            string    `json:"reasonCode,omitempty"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

type RiderWithdrawalView struct {
	WithdrawPublicID string                         `json:"withdrawPublicId"`
	RiderUserID      uint64                         `json:"riderUserId"`
	AmountCents      uint64                         `json:"amountCents"`
	Status           delivery.RiderWithdrawalStatus `json:"status"`
	Reason           string                         `json:"reason,omitempty"`
	RowVersion       uint64                         `json:"rowVersion"`
	ProcessedAt      *time.Time                     `json:"processedAt,omitempty"`
	CreatedAt        time.Time                      `json:"createdAt"`
	UpdatedAt        time.Time                      `json:"updatedAt"`
}

func riderWalletView(value delivery.RiderWalletSnapshot) RiderWalletView {
	return RiderWalletView{
		WalletState: value.WalletState, WalletPublicID: value.WalletPublicID,
		AvailableBalanceCents: value.AvailableBalanceCents, ReservedCents: value.ReservedCents,
		UnverifiedCents: value.UnverifiedCents, UpdatedAt: value.UpdatedAt,
		PendingSettlementCents: value.PendingSettlementCents,
		NextSettlementAt:       value.NextSettlementAt,
	}
}

func riderWalletEarningView(value delivery.RiderEarningRecord) RiderWalletEarningView {
	var settleAt *time.Time
	if !value.SettleAt.IsZero() {
		settleAt = &value.SettleAt
	}
	return RiderWalletEarningView{
		EarningPublicID: value.EarningPublicID, DeliveryOrderPublicID: value.DeliveryOrderPublicID,
		AmountCents: value.RiderFeeCents, SettlementStatus: value.SettlementStatus,
		SettleAt: settleAt, SettledAt: value.SettledAt,
		CreatedAt: value.CreatedAt, PaidOfflineAt: value.PaidOfflineAt,
	}
}

func riderWalletLedgerView(value delivery.RiderLedger) RiderWalletLedgerView {
	return RiderWalletLedgerView{
		LedgerPublicID: value.LedgerPublicID, Direction: value.Direction, EntryType: value.EntryType,
		AmountCents: value.AmountCents, BalanceBeforeCents: value.BalanceBeforeCents,
		BalanceAfterCents: value.BalanceAfterCents, ReferencePublicID: value.ReferencePublicID,
		CreatedAt: value.CreatedAt,
	}
}

func riderWalletQueueView(value delivery.RiderPayoutQueueReadItem) RiderWalletQueueView {
	return RiderWalletQueueView{
		PayoutQueuePublicID: value.PayoutQueuePublicID, EarningPublicID: value.EarningPublicID,
		DeliveryOrderPublicID: value.DeliveryOrderPublicID, AmountCents: value.AmountCents,
		Status: value.Status, ReasonCode: value.ReasonCode, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func riderWithdrawalView(value delivery.RiderWithdrawal) RiderWithdrawalView {
	return RiderWithdrawalView{
		WithdrawPublicID: value.WithdrawPublicID,
		RiderUserID:      value.RiderUserID,
		AmountCents:      value.AmountCents,
		Status:           value.Status,
		Reason:           value.Reason,
		RowVersion:       value.RowVersion,
		ProcessedAt:      value.ProcessedAt,
		CreatedAt:        value.CreatedAt,
		UpdatedAt:        value.UpdatedAt,
	}
}

type ClaimableOrderView struct {
	DeliveryOrderPublicID string                  `json:"deliveryOrderPublicId"`
	SourcePlatform        delivery.SourcePlatform `json:"sourcePlatform"`
	PickupCode            string                  `json:"pickupCode"`
	RouteSummary          string                  `json:"routeSummary"`
	DeliveryFeeCents      uint64                  `json:"deliveryFeeCents"`
	Status                delivery.Status         `json:"status"`
	RowVersion            uint64                  `json:"rowVersion"`
	CreatedAt             time.Time               `json:"createdAt"`
}

type AssignedOrderView struct {
	DeliveryOrderPublicID string                  `json:"deliveryOrderPublicId"`
	SourcePlatform        delivery.SourcePlatform `json:"sourcePlatform"`
	PickupCode            string                  `json:"pickupCode"`
	SourceOrderID         *string                 `json:"sourceOrderId,omitempty"`
	SourceShopID          *string                 `json:"sourceShopId,omitempty"`
	MerchantUserID        uint64                  `json:"merchantUserId"`
	RiderUserID           uint64                  `json:"riderUserId"`
	PickupContactName     string                  `json:"pickupContactName"`
	PickupContactPhone    string                  `json:"pickupContactPhone"`
	PickupAddress         string                  `json:"pickupAddress"`
	RecipientName         string                  `json:"recipientName"`
	RecipientPhone        string                  `json:"recipientPhone"`
	DeliveryAddress       string                  `json:"deliveryAddress"`
	Remark                string                  `json:"remark"`
	DeliveryFeeCents      uint64                  `json:"deliveryFeeCents"`
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

func riderView(value takeout.Rider) RiderView {
	return RiderView{
		RiderUserID: value.RiderUserID, Status: value.Status,
		RealName: value.RealName, Phone: value.Phone, RowVersion: value.RowVersion,
	}
}

func claimableView(value delivery.Order) ClaimableOrderView {
	return ClaimableOrderView{
		DeliveryOrderPublicID: value.DeliveryOrderPublicID,
		SourcePlatform:        value.SourcePlatform.Effective(), PickupCode: value.PickupCode,
		RouteSummary: "校园配送单", DeliveryFeeCents: value.DeliveryFeeCents,
		Status: value.Status, RowVersion: value.RowVersion, CreatedAt: value.CreatedAt,
	}
}

func assignedView(value delivery.Order) AssignedOrderView {
	riderID := uint64(0)
	if value.RiderUserID != nil {
		riderID = *value.RiderUserID
	}
	return AssignedOrderView{
		DeliveryOrderPublicID: value.DeliveryOrderPublicID,
		SourcePlatform:        value.SourcePlatform.Effective(), PickupCode: value.PickupCode,
		SourceOrderID: value.SourceOrderID, SourceShopID: value.SourceShopID,
		MerchantUserID: value.MerchantUserID, RiderUserID: riderID,
		PickupContactName: value.PickupContactName, PickupContactPhone: value.PickupContactPhone,
		PickupAddress: value.PickupAddress, RecipientName: value.RecipientName,
		RecipientPhone: value.RecipientPhone, DeliveryAddress: value.DeliveryAddress,
		Remark: value.Remark, DeliveryFeeCents: value.DeliveryFeeCents,
		Status: value.Status, RowVersion: value.RowVersion,
		ReadyForClaimAt: value.ReadyForClaimAt, ClaimedAt: value.ClaimedAt,
		ToPickupAt: value.ToPickupAt, PickedUpAt: value.PickedUpAt,
		DeliveringAt: value.DeliveringAt, DeliveredAt: value.DeliveredAt,
		PaymentFailedAt: value.PaymentFailedAt, CancelledAt: value.CancelledAt,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}
