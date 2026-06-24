// Package takeoutbuyer exposes the miniapp buyer transport adapter for
// domain G. Domain rules remain in internal/module/takeout.
package takeoutbuyer

import (
	"fmt"
	"time"

	"campus-circle/backend/internal/module/takeout"
)

type Page[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type MerchantView struct {
	MerchantPublicID  string                 `json:"merchantPublicId"`
	Name              string                 `json:"name"`
	ContactPhone      string                 `json:"contactPhone"`
	Location          string                 `json:"location"`
	BusinessHours     string                 `json:"businessHours"`
	LogoAssetPublicID *string                `json:"logoAssetPublicId,omitempty"`
	LogoContentURL    string                 `json:"logoContentUrl,omitempty"`
	Status            takeout.MerchantStatus `json:"status"`
	CreatedAt         time.Time              `json:"createdAt"`
	UpdatedAt         time.Time              `json:"updatedAt"`
}

type DishView struct {
	DishPublicID       string  `json:"dishPublicId"`
	MerchantPublicID   string  `json:"merchantPublicId"`
	CategoryPublicID   *string `json:"categoryPublicId,omitempty"`
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	PriceCents         uint64  `json:"priceCents"`
	ImageAssetPublicID *string `json:"imageAssetPublicId,omitempty"`
	ImageContentURL    string  `json:"imageContentUrl,omitempty"`
	SortOrder          int32   `json:"sortOrder"`
}

type OrderView struct {
	OrderPublicID    string              `json:"orderPublicId"`
	BuyerUserID      uint64              `json:"buyerUserId"`
	MerchantPublicID string              `json:"merchantPublicId"`
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
	Order OrderView       `json:"order"`
	Items []OrderItemView `json:"items"`
}

func merchantView(value takeout.Merchant) MerchantView {
	return MerchantView{
		MerchantPublicID: value.MerchantPublicID, Name: value.Name, ContactPhone: value.ContactPhone,
		Location: value.Location, BusinessHours: value.BusinessHours, LogoAssetPublicID: value.LogoAssetPublicID,
		LogoContentURL: mediaContentURL("merchant_logo", value.MerchantPublicID, value.LogoAssetPublicID),
		Status:         value.Status, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func dishView(value takeout.Dish) DishView {
	return DishView{
		DishPublicID: value.DishPublicID, MerchantPublicID: value.MerchantPublicID, CategoryPublicID: value.CategoryPublicID,
		Name: value.Name, Description: value.Description, PriceCents: value.PriceCents,
		ImageAssetPublicID: value.ImageAssetPublicID, ImageContentURL: mediaContentURL("dish", value.DishPublicID, value.ImageAssetPublicID),
		SortOrder: value.SortOrder,
	}
}

func orderView(value takeout.Order) OrderView {
	return OrderView{
		OrderPublicID: value.OrderPublicID, BuyerUserID: value.BuyerUserID, MerchantPublicID: value.MerchantPublicID,
		RiderUserID: value.RiderUserID, Status: value.Status, ItemsAmountCents: value.ItemsAmountCents,
		DeliveryFeeCents: value.DeliveryFeeCents, TotalAmountCents: value.TotalAmountCents,
		ContactName: value.ContactName, ContactPhone: value.ContactPhone, AddressBuilding: value.AddressBuilding,
		AddressRoom: value.AddressRoom, Remark: value.Remark, PaidAt: value.PaidAt, AcceptedAt: value.AcceptedAt,
		PreparingAt: value.PreparingAt, ReadyAt: value.ReadyAt, DeliveringAt: value.DeliveringAt,
		CompletedAt: value.CompletedAt, CancelledAt: value.CancelledAt, RefundedAt: value.RefundedAt,
		RowVersion: value.RowVersion, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func orderItemView(value takeout.OrderItem) OrderItemView {
	return OrderItemView{
		DishPublicID: value.DishPublicID, DishNameSnapshot: value.DishNameSnapshot,
		UnitPriceCentsSnapshot: value.UnitPriceCentsSnapshot, Quantity: value.Quantity,
		LineAmountCents: value.LineAmountCents,
	}
}

func mediaContentURL(referenceType, referenceID string, assetPublicID *string) string {
	if assetPublicID == nil || *assetPublicID == "" {
		return ""
	}
	return fmt.Sprintf("/api/v1/miniapp/media/display/%s/%s/0/content", referenceType, referenceID)
}

func mapValues[A, B any](values []A, convert func(A) B) []B {
	result := make([]B, 0, len(values))
	for _, value := range values {
		result = append(result, convert(value))
	}
	return result
}
