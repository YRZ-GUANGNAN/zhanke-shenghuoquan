package takeout

import (
	"math"
	"testing"
	"time"
)

func TestOrderModelsAndIntegerAmountGuards(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	orderID := "01J00000000000000000000101"
	merchantID := "01J00000000000000000000102"
	dishID := "01J00000000000000000000103"
	order := Order{
		OrderPublicID: orderID, BuyerUserID: 1, MerchantPublicID: merchantID,
		Status: OrderPendingPayment, ItemsAmountCents: 2000, DeliveryFeeCents: 150,
		TotalAmountCents: 2150, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := order.ValidateNew(); err != nil {
		t.Fatalf("valid order rejected: %v", err)
	}
	order.TotalAmountCents++
	if err := order.ValidateNew(); err == nil {
		t.Fatal("mismatched total accepted")
	}
	item := OrderItem{
		OrderPublicID: orderID, DishPublicID: dishID, DishNameSnapshot: "Rice",
		UnitPriceCentsSnapshot: 1000, Quantity: 2, LineAmountCents: 2000, CreatedAt: now,
	}
	if err := item.ValidateNew(); err != nil {
		t.Fatalf("valid item rejected: %v", err)
	}
	item.LineAmountCents++
	if err := item.ValidateNew(); err == nil {
		t.Fatal("mismatched line amount accepted")
	}
	if _, ok := checkedMultiplyAmount(math.MaxUint64, 2); ok {
		t.Fatal("multiplication overflow accepted")
	}
	if _, ok := checkedAddAmount(math.MaxUint64, 1); ok {
		t.Fatal("addition overflow accepted")
	}
	earning := RiderEarningRecord{
		EarningPublicID: "01J00000000000000000000104", OrderPublicID: orderID, RiderUserID: 2,
		RiderFeeCents: 100, PlatformCutCents: 50, SettlementStatus: SettlementPending,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := earning.ValidateNew(); err != nil {
		t.Fatalf("valid earning rejected: %v", err)
	}
}
