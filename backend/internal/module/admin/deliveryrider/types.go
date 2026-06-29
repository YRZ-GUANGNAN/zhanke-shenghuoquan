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

type ClaimableOrderView struct {
	DeliveryOrderPublicID string          `json:"deliveryOrderPublicId"`
	RouteSummary          string          `json:"routeSummary"`
	DeliveryFeeCents      uint64          `json:"deliveryFeeCents"`
	Status                delivery.Status `json:"status"`
	RowVersion            uint64          `json:"rowVersion"`
	CreatedAt             time.Time       `json:"createdAt"`
}

type AssignedOrderView struct {
	DeliveryOrderPublicID string          `json:"deliveryOrderPublicId"`
	MerchantUserID        uint64          `json:"merchantUserId"`
	RiderUserID           uint64          `json:"riderUserId"`
	PickupContactName     string          `json:"pickupContactName"`
	PickupContactPhone    string          `json:"pickupContactPhone"`
	PickupAddress         string          `json:"pickupAddress"`
	RecipientName         string          `json:"recipientName"`
	RecipientPhone        string          `json:"recipientPhone"`
	DeliveryAddress       string          `json:"deliveryAddress"`
	Remark                string          `json:"remark"`
	DeliveryFeeCents      uint64          `json:"deliveryFeeCents"`
	Status                delivery.Status `json:"status"`
	RowVersion            uint64          `json:"rowVersion"`
	ReadyForClaimAt       *time.Time      `json:"readyForClaimAt"`
	ClaimedAt             *time.Time      `json:"claimedAt"`
	ToPickupAt            *time.Time      `json:"toPickupAt"`
	PickedUpAt            *time.Time      `json:"pickedUpAt"`
	DeliveringAt          *time.Time      `json:"deliveringAt"`
	DeliveredAt           *time.Time      `json:"deliveredAt"`
	PaymentFailedAt       *time.Time      `json:"paymentFailedAt"`
	CancelledAt           *time.Time      `json:"cancelledAt"`
	CreatedAt             time.Time       `json:"createdAt"`
	UpdatedAt             time.Time       `json:"updatedAt"`
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
		RouteSummary:          "校园配送单", DeliveryFeeCents: value.DeliveryFeeCents,
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
		MerchantUserID:        value.MerchantUserID, RiderUserID: riderID,
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
