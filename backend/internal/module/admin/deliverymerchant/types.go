package deliverymerchant

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

type OrderIntakeView struct {
	ManualAvailable   bool                      `json:"manualAvailable"`
	ExternalAvailable bool                      `json:"externalAvailable"`
	Platforms         []OrderIntakePlatformView `json:"platforms"`
}

type OrderIntakePlatformView struct {
	Platform  delivery.SourcePlatform `json:"platform"`
	Label     string                  `json:"label"`
	Connected bool                    `json:"connected"`
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

func orderView(value delivery.Order) OrderView {
	return OrderView{
		DeliveryOrderPublicID: value.DeliveryOrderPublicID,
		SourcePlatform:        value.SourcePlatform.Effective(), PickupCode: value.PickupCode,
		SourceOrderID: value.SourceOrderID, SourceShopID: value.SourceShopID,
		MerchantRequestID: value.MerchantRequestID, MerchantUserID: value.MerchantUserID,
		RiderUserID: value.RiderUserID, PickupContactName: value.PickupContactName,
		PickupContactPhone: value.PickupContactPhone, PickupAddress: value.PickupAddress,
		RecipientName: value.RecipientName, RecipientPhone: value.RecipientPhone,
		DeliveryAddress: value.DeliveryAddress, Remark: value.Remark,
		DeliveryFeeCents: value.DeliveryFeeCents, RiderFeeCents: value.RiderFeeCents,
		PlatformCutCents: value.PlatformCutCents, Status: value.Status,
		RowVersion:      value.RowVersion,
		ReadyForClaimAt: value.ReadyForClaimAt, ClaimedAt: value.ClaimedAt,
		ToPickupAt: value.ToPickupAt, PickedUpAt: value.PickedUpAt,
		DeliveringAt: value.DeliveringAt, DeliveredAt: value.DeliveredAt,
		PaymentFailedAt: value.PaymentFailedAt, CancelledAt: value.CancelledAt,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}
