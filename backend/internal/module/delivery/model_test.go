package delivery

import (
	"bytes"
	"math"
	"testing"
	"time"
)

func TestOrderValidateNewAndMoneySnapshot(t *testing.T) {
	now := time.Date(2026, 6, 29, 1, 2, 3, 0, time.UTC)
	order := validNewOrder(t, now)
	if err := order.ValidateNew(); err != nil {
		t.Fatal(err)
	}
	order.DeliveryFeeCents++
	if err := order.ValidateNew(); err != ErrInvalidRecord {
		t.Fatalf("mismatched fee snapshot returned %v", err)
	}
	order = validNewOrder(t, now)
	order.PlatformCutCents = math.MaxUint64
	if err := order.ValidateNew(); err != ErrInvalidRecord {
		t.Fatalf("overflowing fee snapshot returned %v", err)
	}
}

func TestDeliveryULID(t *testing.T) {
	at := time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC)
	value, err := NewULIDAt(at, bytes.NewReader(make([]byte, 10)))
	if err != nil {
		t.Fatal(err)
	}
	if !ValidULID(value) || len(value) != 26 {
		t.Fatalf("invalid generated ULID %q", value)
	}
	for _, invalid := range []string{"", value[:25], "8" + value[1:], value[:25] + "I"} {
		if ValidULID(invalid) {
			t.Errorf("accepted invalid ULID %q", invalid)
		}
	}
}

func validNewOrder(t *testing.T, now time.Time) Order {
	t.Helper()
	publicID, err := NewULIDAt(now, bytes.NewReader(make([]byte, 10)))
	if err != nil {
		t.Fatal(err)
	}
	return Order{
		DeliveryOrderPublicID: publicID,
		MerchantUserID:        991001,
		PickupContactName:     "Merchant",
		PickupContactPhone:    "13800000001",
		PickupAddress:         "Campus Gate 1",
		RecipientName:         "Student",
		RecipientPhone:        "13800000002",
		DeliveryAddress:       "Dormitory 1-101",
		Remark:                "D-1 test",
		DeliveryFeeCents:      150,
		RiderFeeCents:         100,
		PlatformCutCents:      50,
		Status:                StatusPendingFeePayment,
		RowVersion:            1,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}
