// Package takeoutmerchant exposes the miniapp merchant transport adapter for
// domain G. Domain rules remain in internal/module/takeout.
package takeoutmerchant

import (
	"time"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/takeout"
)

type Page[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type MeView struct {
	Onboarded bool          `json:"onboarded"`
	Merchant  *MerchantView `json:"merchant,omitempty"`
}

type OnboardingView struct {
	Merchant MerchantView `json:"merchant"`
	Review   ReviewView   `json:"review"`
}

type ReviewView struct {
	TaskID     uint          `json:"taskId"`
	ObjectType string        `json:"objectType"`
	ObjectID   uint          `json:"objectId"`
	Status     review.Status `json:"status"`
}

type MerchantView struct {
	MerchantPublicID  string                 `json:"merchantPublicId"`
	OwnerUserID       uint64                 `json:"ownerUserId"`
	Name              string                 `json:"name"`
	ContactPhone      string                 `json:"contactPhone"`
	Location          string                 `json:"location"`
	BusinessHours     string                 `json:"businessHours"`
	LogoAssetPublicID *string                `json:"logoAssetPublicId,omitempty"`
	Status            takeout.MerchantStatus `json:"status"`
	RowVersion        uint64                 `json:"rowVersion"`
	CreatedAt         time.Time              `json:"createdAt"`
	UpdatedAt         time.Time              `json:"updatedAt"`
}

type CategoryView struct {
	CategoryPublicID string    `json:"categoryPublicId"`
	MerchantPublicID string    `json:"merchantPublicId"`
	Name             string    `json:"name"`
	SortOrder        int32     `json:"sortOrder"`
	RowVersion       uint64    `json:"rowVersion"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type DishView struct {
	DishPublicID       string             `json:"dishPublicId"`
	MerchantPublicID   string             `json:"merchantPublicId"`
	CategoryPublicID   *string            `json:"categoryPublicId,omitempty"`
	Name               string             `json:"name"`
	Description        string             `json:"description"`
	PriceCents         uint64             `json:"priceCents"`
	ImageAssetPublicID *string            `json:"imageAssetPublicId,omitempty"`
	Status             takeout.DishStatus `json:"status"`
	SortOrder          int32              `json:"sortOrder"`
	RowVersion         uint64             `json:"rowVersion"`
	CreatedAt          time.Time          `json:"createdAt"`
	UpdatedAt          time.Time          `json:"updatedAt"`
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
	PayNo            *string             `json:"payNo,omitempty"`
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

func merchantView(v takeout.Merchant) MerchantView {
	return MerchantView{v.MerchantPublicID, v.OwnerUserID, v.Name, v.ContactPhone, v.Location, v.BusinessHours, v.LogoAssetPublicID, v.Status, v.RowVersion, v.CreatedAt, v.UpdatedAt}
}

func categoryView(v takeout.DishCategory) CategoryView {
	return CategoryView{v.CategoryPublicID, v.MerchantPublicID, v.Name, v.SortOrder, v.RowVersion, v.CreatedAt, v.UpdatedAt}
}

func dishView(v takeout.Dish) DishView {
	return DishView{v.DishPublicID, v.MerchantPublicID, v.CategoryPublicID, v.Name, v.Description, v.PriceCents, v.ImageAssetPublicID, v.Status, v.SortOrder, v.RowVersion, v.CreatedAt, v.UpdatedAt}
}

func orderView(v takeout.Order) OrderView {
	return OrderView{
		OrderPublicID: v.OrderPublicID, BuyerUserID: v.BuyerUserID, MerchantPublicID: v.MerchantPublicID,
		RiderUserID: v.RiderUserID, Status: v.Status, ItemsAmountCents: v.ItemsAmountCents,
		DeliveryFeeCents: v.DeliveryFeeCents, TotalAmountCents: v.TotalAmountCents,
		ContactName: v.ContactName, ContactPhone: v.ContactPhone, AddressBuilding: v.AddressBuilding,
		AddressRoom: v.AddressRoom, Remark: v.Remark, PayNo: v.PayNo,
		PaidAt: v.PaidAt, AcceptedAt: v.AcceptedAt, PreparingAt: v.PreparingAt, ReadyAt: v.ReadyAt,
		DeliveringAt: v.DeliveringAt, CompletedAt: v.CompletedAt, CancelledAt: v.CancelledAt,
		RefundedAt: v.RefundedAt, RowVersion: v.RowVersion, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt,
	}
}

func orderItemView(v takeout.OrderItem) OrderItemView {
	return OrderItemView{v.DishPublicID, v.DishNameSnapshot, v.UnitPriceCentsSnapshot, v.Quantity, v.LineAmountCents}
}

func reviewView(v review.Task) ReviewView {
	return ReviewView{TaskID: v.ID, ObjectType: v.ObjectType, ObjectID: v.ObjectID, Status: v.Status}
}

func mapValues[A, B any](values []A, convert func(A) B) []B {
	result := make([]B, 0, len(values))
	for _, value := range values {
		result = append(result, convert(value))
	}
	return result
}
