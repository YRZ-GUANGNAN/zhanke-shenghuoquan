// Package takeoutrider exposes the miniapp rider transport adapter for
// domain G. Domain rules remain in internal/module/takeout.
package takeoutrider

import (
	"time"

	"campus-circle/backend/internal/module/takeout"
)

type Page[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type MeView struct {
	Registered bool       `json:"registered"`
	Rider      *RiderView `json:"rider,omitempty"`
}

type RiderView struct {
	RiderUserID uint64              `json:"riderUserId"`
	Status      takeout.RiderStatus `json:"status"`
	RealName    string              `json:"realName"`
	Phone       string              `json:"phone"`
	RowVersion  uint64              `json:"rowVersion"`
	CreatedAt   time.Time           `json:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`
}

type OrderSummaryView struct {
	OrderPublicID    string              `json:"orderPublicId"`
	BuyerUserID      uint64              `json:"buyerUserId"`
	MerchantPublicID string              `json:"merchantPublicId"`
	MerchantName     string              `json:"merchantName,omitempty"`
	RiderUserID      *uint64             `json:"riderUserId,omitempty"`
	Status           takeout.OrderStatus `json:"status"`
	ItemsAmountCents uint64              `json:"itemsAmountCents"`
	DeliveryFeeCents uint64              `json:"deliveryFeeCents"`
	TotalAmountCents uint64              `json:"totalAmountCents"`
	ContactName      string              `json:"contactName"`
	ContactPhone     string              `json:"contactPhone"`
	AddressBuilding  string              `json:"addressBuilding"`
	AddressRoom      string              `json:"addressRoom"`
	Remark           string              `json:"remark"`
	PaidAt           *time.Time          `json:"paidAt,omitempty"`
	AcceptedAt       *time.Time          `json:"acceptedAt,omitempty"`
	PreparingAt      *time.Time          `json:"preparingAt,omitempty"`
	ReadyAt          *time.Time          `json:"readyAt,omitempty"`
	DeliveringAt     *time.Time          `json:"deliveringAt,omitempty"`
	CompletedAt      *time.Time          `json:"completedAt,omitempty"`
	CancelledAt      *time.Time          `json:"cancelledAt,omitempty"`
	RefundedAt       *time.Time          `json:"refundedAt,omitempty"`
	RowVersion       uint64              `json:"rowVersion"`
	CreatedAt        time.Time           `json:"createdAt"`
	UpdatedAt        time.Time           `json:"updatedAt"`
}

type OrderItemView struct {
	DishPublicID           string `json:"dishPublicId"`
	DishNameSnapshot       string `json:"dishNameSnapshot"`
	UnitPriceCentsSnapshot uint64 `json:"unitPriceCentsSnapshot"`
	Quantity               uint32 `json:"quantity"`
	LineAmountCents        uint64 `json:"lineAmountCents"`
}

type OrderDetailView struct {
	Order OrderSummaryView `json:"order"`
	Items []OrderItemView  `json:"items"`
}

type FulfillmentView struct {
	FulfillmentPublicID string                    `json:"fulfillmentPublicId"`
	OrderPublicID       string                    `json:"orderPublicId"`
	RiderUserID         uint64                    `json:"riderUserId"`
	Status              takeout.FulfillmentStatus `json:"status"`
	AssignedAt          *time.Time                `json:"assignedAt,omitempty"`
	ToStoreAt           *time.Time                `json:"toStoreAt,omitempty"`
	PickedAt            *time.Time                `json:"pickedAt,omitempty"`
	ToUserAt            *time.Time                `json:"toUserAt,omitempty"`
	DeliveredAt         *time.Time                `json:"deliveredAt,omitempty"`
	RowVersion          uint64                    `json:"rowVersion"`
	CreatedAt           time.Time                 `json:"createdAt"`
	UpdatedAt           time.Time                 `json:"updatedAt"`
}

type FulfillmentDetailView struct {
	Fulfillment FulfillmentView  `json:"fulfillment"`
	Order       OrderSummaryView `json:"order"`
	Items       []OrderItemView  `json:"items"`
}

type EarningView struct {
	EarningPublicID  string                   `json:"earningPublicId"`
	OrderPublicID    string                   `json:"orderPublicId"`
	RiderUserID      uint64                   `json:"riderUserId"`
	RiderFeeCents    uint64                   `json:"riderFeeCents"`
	PlatformCutCents uint64                   `json:"platformCutCents"`
	SettlementStatus takeout.SettlementStatus `json:"settlementStatus"`
	SettledAt        *time.Time               `json:"settledAt,omitempty"`
	RowVersion       uint64                   `json:"rowVersion"`
	CreatedAt        time.Time                `json:"createdAt"`
	UpdatedAt        time.Time                `json:"updatedAt"`
}

type WalletView struct {
	RiderUserID            uint64        `json:"riderUserId"`
	PendingSettlementCents uint64        `json:"pendingSettlementCents"`
	PendingWithdrawCents   uint64        `json:"pendingWithdrawCents"`
	AvailableWithdrawCents uint64        `json:"availableWithdrawCents"`
	SettledCents           uint64        `json:"settledCents"`
	RecentEarnings         []EarningView `json:"recentEarnings"`
	RecentEarningsPage     int           `json:"recentEarningsPage"`
	RecentEarningsPageSize int           `json:"recentEarningsPageSize"`
	RecentEarningsTotal    int64         `json:"recentEarningsTotal"`
}

type WithdrawView struct {
	WithdrawPublicID string                 `json:"withdrawPublicId"`
	RiderUserID      uint64                 `json:"riderUserId"`
	AmountCents      uint64                 `json:"amountCents"`
	Status           takeout.WithdrawStatus `json:"status"`
	Note             string                 `json:"note"`
	PaidAt           *time.Time             `json:"paidAt,omitempty"`
	RowVersion       uint64                 `json:"rowVersion"`
	CreatedAt        time.Time              `json:"createdAt"`
	UpdatedAt        time.Time              `json:"updatedAt"`
}

func riderView(value takeout.Rider) RiderView {
	return RiderView{
		RiderUserID: value.RiderUserID, Status: value.Status, RealName: value.RealName, Phone: value.Phone,
		RowVersion: value.RowVersion, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func orderSummaryView(value takeout.Order, merchantName string) OrderSummaryView {
	return OrderSummaryView{
		OrderPublicID: value.OrderPublicID, BuyerUserID: value.BuyerUserID, MerchantPublicID: value.MerchantPublicID,
		MerchantName: merchantName, RiderUserID: value.RiderUserID, Status: value.Status,
		ItemsAmountCents: value.ItemsAmountCents, DeliveryFeeCents: value.DeliveryFeeCents,
		TotalAmountCents: value.TotalAmountCents, ContactName: value.ContactName, ContactPhone: value.ContactPhone,
		AddressBuilding: value.AddressBuilding, AddressRoom: value.AddressRoom, Remark: value.Remark,
		PaidAt: value.PaidAt, AcceptedAt: value.AcceptedAt, PreparingAt: value.PreparingAt, ReadyAt: value.ReadyAt,
		DeliveringAt: value.DeliveringAt, CompletedAt: value.CompletedAt, CancelledAt: value.CancelledAt,
		RefundedAt: value.RefundedAt, RowVersion: value.RowVersion, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func orderItemView(value takeout.OrderItem) OrderItemView {
	return OrderItemView{
		DishPublicID: value.DishPublicID, DishNameSnapshot: value.DishNameSnapshot,
		UnitPriceCentsSnapshot: value.UnitPriceCentsSnapshot, Quantity: value.Quantity,
		LineAmountCents: value.LineAmountCents,
	}
}

func fulfillmentView(value takeout.Fulfillment) FulfillmentView {
	return FulfillmentView{
		FulfillmentPublicID: value.FulfillmentPublicID, OrderPublicID: value.OrderPublicID,
		RiderUserID: value.RiderUserID, Status: value.Status, AssignedAt: value.AssignedAt,
		ToStoreAt: value.ToStoreAt, PickedAt: value.PickedAt, ToUserAt: value.ToUserAt,
		DeliveredAt: value.DeliveredAt, RowVersion: value.RowVersion, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func earningView(value takeout.RiderEarningRecord) EarningView {
	return EarningView{
		EarningPublicID: value.EarningPublicID, OrderPublicID: value.OrderPublicID, RiderUserID: value.RiderUserID,
		RiderFeeCents: value.RiderFeeCents, PlatformCutCents: value.PlatformCutCents,
		SettlementStatus: value.SettlementStatus, SettledAt: value.SettledAt, RowVersion: value.RowVersion,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func withdrawView(value takeout.WithdrawRequest) WithdrawView {
	return WithdrawView{
		WithdrawPublicID: value.WithdrawPublicID, RiderUserID: value.RiderUserID, AmountCents: value.AmountCents,
		Status: value.Status, Note: value.Note, PaidAt: value.PaidAt, RowVersion: value.RowVersion,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func mapValues[A, B any](values []A, convert func(A) B) []B {
	result := make([]B, 0, len(values))
	for _, value := range values {
		result = append(result, convert(value))
	}
	return result
}
