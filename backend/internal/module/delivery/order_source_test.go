package delivery

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSourceMetadataNormalization(t *testing.T) {
	for _, test := range []struct {
		name              string
		platform          SourcePlatform
		code, order, shop string
		valid             bool
	}{
		{"legacy", "", "", "", "", true},
		{"local code", SourceLocal, " 001A ", "", "", true},
		{"manual platform", SourceTaobao, "22", "", "", true},
		{"external identity", SourceJD, "0015", "JD-AbC001", "SHOP-a", true},
		{"unicode limits", SourceMeituan, strings.Repeat("号", 16), strings.Repeat("单", 128), strings.Repeat("店", 64), true},
		{"invalid platform", "unknown", "", "", "", false},
		{"local source id", SourceLocal, "22", "order", "shop", false},
		{"order without shop", SourceJD, "15", "order", "", false},
		{"shop without order", SourceJD, "15", "", "shop", false},
		{"long pickup", SourceJD, strings.Repeat("a", 17), "", "", false},
		{"long order", SourceJD, "", strings.Repeat("a", 129), "shop", false},
		{"long shop", SourceJD, "", "order", strings.Repeat("a", 65), false},
		{"control code", SourceJD, "1\n2", "", "", false},
		{"bidi source id", SourceJD, "15", "order\u202E", "shop", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			value, err := NormalizeSourceMetadata(test.platform, test.code, test.order, test.shop)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v error=%v", test.valid, err)
			}
			if err != nil {
				return
			}
			if value.PickupCode != strings.TrimSpace(test.code) {
				t.Fatal("pickup formatting lost")
			}
			if test.order == "" && (value.SourceOrderID != nil || value.SourceShopID != nil) {
				t.Fatal("empty source identities must be NULL")
			}
			if test.order != "" && (*value.SourceOrderID != test.order || *value.SourceShopID != test.shop) {
				t.Fatal("case/leading zeros lost")
			}
		})
	}
}

type sourceRecordingRepository struct {
	DispatchRepository
	created     Order
	createCount int
}

func (r *sourceRecordingRepository) CreateIdempotent(_ context.Context, value *Order) (Order, error) {
	r.created = *value
	r.createCount++
	return *value, nil
}

func TestDispatchSourceValidationHappensBeforeCreate(t *testing.T) {
	repository := &sourceRecordingRepository{}
	orders, _ := NewService(&recordingRepository{}, nil)
	service, err := NewDispatchService(repository, orders, activeEligibility{}, FeePolicy{150, 100, 50}, nil)
	if err != nil {
		t.Fatal(err)
	}
	requestID, _ := NewULID()
	input := CreateDispatchInput{MerchantUserID: 1, MerchantRequestID: requestID,
		Pickup: Contact{"商家", "1", "取货点"}, Recipient: Contact{"顾客", "2", "送达点"},
		SourcePlatform: SourceJD, PickupCode: "0015", SourceOrderID: "JD001", SourceShopID: "S01"}
	created, err := service.CreateOrder(context.Background(), input)
	if err != nil || created.PickupCode != "0015" || created.SourcePlatform != SourceJD || *created.SourceOrderID != "JD001" || created.Status != StatusPendingFeePayment {
		t.Fatalf("created=%#v error=%v", created, err)
	}
	input.SourceShopID = ""
	if _, err := service.CreateOrder(context.Background(), input); !errors.Is(err, ErrInvalidRecord) || repository.createCount != 1 {
		t.Fatal("invalid source reached persistence")
	}
}

func TestCreateIdempotencyComparesSourceAndPreservesServerFeeSnapshot(t *testing.T) {
	base := validNewOrder(t, time.Now())
	requestID, _ := NewULID()
	base.MerchantRequestID = &requestID
	base.SourcePlatform = SourceJD
	base.PickupCode = "0015"
	orderID, shopID := "ORDER-Abc", "SHOP-Abc"
	base.SourceOrderID, base.SourceShopID = &orderID, &shopID
	for _, change := range []func(*Order){
		func(v *Order) { v.SourcePlatform = SourceMeituan },
		func(v *Order) { v.PickupCode = "15" },
		func(v *Order) { s := "ORDER-abc"; v.SourceOrderID = &s },
		func(v *Order) { s := "SHOP-abc"; v.SourceShopID = &s },
		func(v *Order) { v.RecipientPhone = "changed" },
	} {
		changed := base
		change(&changed)
		if sameCreateSnapshot(base, changed) {
			t.Fatal("changed merchant request was accepted as a retry")
		}
	}
	repriced := base
	repriced.DeliveryFeeCents, repriced.RiderFeeCents, repriced.PlatformCutCents = 200, 130, 70
	if !sameCreateSnapshot(base, repriced) {
		t.Fatal("server fee update must not change merchant request identity")
	}
}

func TestExternalAdapterNormalizationRequiresBoundShopAndNeverCreatesOrder(t *testing.T) {
	orderID, shopID := "Order-001", "Shop-A"
	event := ExternalOrderEvent{EventID: "evt-001", EventType: ExternalOrderCreated, OccurredAt: time.Now(), Source: SourceMetadata{SourcePlatform: SourceTaobao, PickupCode: "22", SourceOrderID: &orderID, SourceShopID: &shopID}}
	normalized, err := NormalizeExternalOrderEvent(SourceTaobao, ExternalShopBinding{MerchantUserID: 99, Platform: SourceTaobao, SourceShopID: shopID}, event)
	if err != nil || normalized.Source.SourceOrderID == nil || *normalized.Source.SourceOrderID != orderID {
		t.Fatalf("normalized=%#v err=%v", normalized, err)
	}
	if _, err := NormalizeExternalOrderEvent(SourceJD, ExternalShopBinding{MerchantUserID: 99, Platform: SourceTaobao, SourceShopID: shopID}, event); !errors.Is(err, ErrInvalidRecord) {
		t.Fatal("adapter/platform mismatch accepted")
	}
	if _, err := NormalizeExternalOrderEvent(SourceTaobao, ExternalShopBinding{MerchantUserID: 99, Platform: SourceTaobao, SourceShopID: "Other-Shop"}, event); !errors.Is(err, ErrInvalidRecord) {
		t.Fatal("unbound shop accepted")
	}
	event.Source.SourcePlatform = SourceJD
	if _, err := NormalizeExternalOrderEvent(SourceTaobao, ExternalShopBinding{MerchantUserID: 99, Platform: SourceTaobao, SourceShopID: shopID}, event); !errors.Is(err, ErrInvalidRecord) {
		t.Fatal("event platform mismatch accepted")
	}
	event.Source.SourcePlatform = SourceLocal
	if _, err := NormalizeExternalOrderEvent(SourceLocal, ExternalShopBinding{MerchantUserID: 99, Platform: SourceLocal, SourceShopID: shopID}, event); !errors.Is(err, ErrInvalidRecord) {
		t.Fatal("manual local source accepted as a verified external adapter")
	}
}

func TestExternalContentRequiresCompleteCreateAndBoundsOptionalUpdates(t *testing.T) {
	e := ExternalOrderEvent{EventType: ExternalOrderCreated, Pickup: Contact{Name: "商家", Phone: "13000000001", Address: "校门"}, Recipient: Contact{Name: "同学", Phone: "13000000002", Address: "宿舍"}}
	if !validExternalContent(e) {
		t.Fatal("valid snapshot rejected")
	}
	e.Recipient.Address = ""
	if validExternalContent(e) {
		t.Fatal("missing destination accepted")
	}
	e.EventType = ExternalOrderCancelled
	if !validExternalContent(e) {
		t.Fatal("cancellation must not require unchanged addresses")
	}
	e.Remark = string([]byte{0xff})
	if validExternalContent(e) {
		t.Fatal("invalid UTF-8 accepted")
	}
}
