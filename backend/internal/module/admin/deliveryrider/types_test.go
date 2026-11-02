package deliveryrider

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
)

func TestAssignedViewProjectsStageTimestampsAndKeepsMissingStagesNull(t *testing.T) {
	ready := time.Date(2026, 6, 29, 10, 1, 0, 0, time.UTC)
	claimed := ready.Add(time.Minute)
	value := delivery.Order{
		ReadyForClaimAt: &ready,
		ClaimedAt:       &claimed,
	}

	view := assignedView(value)
	if view.ReadyForClaimAt != &ready || view.ClaimedAt != &claimed {
		t.Fatalf("projected stage times=%#v", view)
	}
	if view.ToPickupAt != nil || view.PickedUpAt != nil || view.DeliveringAt != nil ||
		view.DeliveredAt != nil || view.PaymentFailedAt != nil || view.CancelledAt != nil {
		t.Fatalf("missing stages must remain nil: %#v", view)
	}

	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		`"toPickupAt":null`, `"pickedUpAt":null`, `"deliveringAt":null`,
		`"deliveredAt":null`, `"paymentFailedAt":null`, `"cancelledAt":null`,
	} {
		if !strings.Contains(string(encoded), field) {
			t.Fatalf("missing nullable field %s in %s", field, encoded)
		}
	}
}

func TestSourceMetadataPoolPrivacyAndAssignedVisibility(t *testing.T) {
	orderID, shopID := "private-source-order", "private-source-shop"
	value := delivery.Order{SourcePlatform: delivery.SourceMeituan, PickupCode: "0028", SourceOrderID: &orderID, SourceShopID: &shopID, PickupContactPhone: "private-phone", DeliveryAddress: "private-address"}
	pool, err := json.Marshal(claimableView(value))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{orderID, shopID, "sourceOrderId", "sourceShopId", "private-phone", "private-address"} {
		if strings.Contains(string(pool), forbidden) {
			t.Fatalf("pool leaked %q", forbidden)
		}
	}
	if !strings.Contains(string(pool), `"sourcePlatform":"meituan"`) || !strings.Contains(string(pool), `"pickupCode":"0028"`) {
		t.Fatal("pool lost public source label")
	}
	assigned := assignedView(value)
	if assigned.SourceOrderID != &orderID || assigned.SourceShopID != &shopID || assigned.PickupCode != "0028" {
		t.Fatal("assigned rider lost source identifiers")
	}
}

func TestRiderWalletViewProjectsSettlementFieldsAndOmitsZeroOptionalTimes(t *testing.T) {
	now := time.Date(2026, 11, 2, 16, 0, 0, 0, time.UTC)
	next := now.Add(24 * time.Hour)
	settle := now.Add(-time.Hour)
	settled := settle.Add(time.Minute)

	wallet := riderWalletView(delivery.RiderWalletSnapshot{
		WalletState:            delivery.RiderWalletReady,
		WalletPublicID:         "01ARZ3NDEKTSV4RRFFQ69G5FAA",
		AvailableBalanceCents:  1200,
		ReservedCents:          200,
		UnverifiedCents:        300,
		PendingSettlementCents: 321,
		NextSettlementAt:       &next,
		UpdatedAt:              &now,
	})
	if wallet.PendingSettlementCents != 321 || wallet.NextSettlementAt == nil || !wallet.NextSettlementAt.Equal(next) ||
		wallet.UpdatedAt == nil || !wallet.UpdatedAt.Equal(now) {
		t.Fatalf("settlement fields were not projected: %#v", wallet)
	}

	earning := riderWalletEarningView(delivery.RiderEarningRecord{
		EarningPublicID:       "01ARZ3NDEKTSV4RRFFQ69G5FAB",
		DeliveryOrderPublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAC",
		RiderFeeCents:         200,
		SettlementStatus:      delivery.RiderEarningSettled,
		SettleAt:              settle,
		SettledAt:             &settled,
		CreatedAt:             now,
	})
	if earning.SettleAt == nil || !earning.SettleAt.Equal(settle) || earning.SettledAt == nil || !earning.SettledAt.Equal(settled) {
		t.Fatalf("earning settlement fields were not projected: %#v", earning)
	}

	encoded, err := json.Marshal(struct {
		Wallet  RiderWalletView        `json:"wallet"`
		Earning RiderWalletEarningView `json:"earning"`
	}{Wallet: wallet, Earning: earning})
	if err != nil {
		t.Fatal(err)
	}
	jsonText := string(encoded)
	for _, field := range []string{`"pendingSettlementCents":321`, `"nextSettlementAt":"2026-11-03T16:00:00Z"`, `"updatedAt":"2026-11-02T16:00:00Z"`, `"settleAt":"2026-11-02T15:00:00Z"`, `"settledAt":"2026-11-02T15:01:00Z"`} {
		if !strings.Contains(jsonText, field) {
			t.Fatalf("JSON missing %s: %s", field, jsonText)
		}
	}
	for _, forbidden := range []string{"operatorUserId", "idempotencyKey", "sourceEvidenceHash", "providerPayload", "openid"} {
		if strings.Contains(jsonText, forbidden) {
			t.Fatalf("wallet response leaked sensitive field %q: %s", forbidden, jsonText)
		}
	}

	zeroWallet := riderWalletView(delivery.RiderWalletSnapshot{WalletState: delivery.RiderWalletNotCreated})
	zeroEarning := riderWalletEarningView(delivery.RiderEarningRecord{SettlementStatus: delivery.RiderEarningPending, CreatedAt: now})
	zeroEncoded, err := json.Marshal(struct {
		Wallet  RiderWalletView        `json:"wallet"`
		Earning RiderWalletEarningView `json:"earning"`
	}{Wallet: zeroWallet, Earning: zeroEarning})
	if err != nil {
		t.Fatal(err)
	}
	zeroText := string(zeroEncoded)
	for _, omitted := range []string{"nextSettlementAt", "settleAt", "settledAt"} {
		if strings.Contains(zeroText, omitted) {
			t.Fatalf("zero optional time should be omitted (%s): %s", omitted, zeroText)
		}
	}
}
