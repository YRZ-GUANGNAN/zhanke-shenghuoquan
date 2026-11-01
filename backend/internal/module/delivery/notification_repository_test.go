package delivery

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestOrderNotificationSnapshotContainsNoContactData(t *testing.T) {
	before := Order{DeliveryOrderPublicID: "01J00000000000000000000811", MerchantUserID: 42,
		RowVersion: 7, Status: StatusReadyForClaim, DeliveryFeeCents: 150,
		PickupAddress: "private-pickup", RecipientPhone: "private-phone", Remark: "private-note"}
	rider := uint64(61)
	after := before
	after.Status, after.RowVersion, after.RiderUserID = StatusClaimed, 8, &rider
	event, err := orderNotification(before, after, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !ValidULID(event.EventPublicID) || event.EventType != EventOrderStatusChanged || event.AggregateVersion != 8 {
		t.Fatal("invalid event coordinate")
	}
	var payload OrderNotificationPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.MerchantUserID != 42 || payload.RiderUserID == nil || *payload.RiderUserID != 61 || payload.From != StatusReadyForClaim {
		t.Fatal("wrong recipient facts")
	}
	for _, value := range []string{"private-pickup", "private-phone", "private-note"} {
		if strings.Contains(string(event.Payload), value) {
			t.Fatal("contact data leaked")
		}
	}
}

func TestOrderNotificationPortPreservesTransactionAndFailure(t *testing.T) {
	before := Order{DeliveryOrderPublicID: "01J00000000000000000000811", MerchantUserID: 42, RowVersion: 1, Status: StatusPendingFeePayment}
	after := before
	after.Status, after.RowVersion = StatusReadyForClaim, 2
	tx := &gorm.DB{}
	want := errors.New("event persistence failed")
	calls := 0
	appendEvent := func(got *gorm.DB, event NotificationEvent) error {
		calls++
		if got != tx {
			t.Fatal("appender received a different transaction")
		}
		return want
	}
	if err := appendOrderNotificationTx(tx, appendEvent, before, after, time.Now()); !errors.Is(err, want) || calls != 1 {
		t.Fatal("event failure must abort caller transaction")
	}
	if err := appendOrderNotificationTx(nil, appendEvent, before, after, time.Now()); !errors.Is(err, ErrInvalidRecord) {
		t.Fatal("nil transaction allowed")
	}
	if err := appendOrderNotificationTx(nil, nil, Order{}, Order{}, time.Time{}); err != nil {
		t.Fatal("disabled rollout changed legacy behavior")
	}
	if _, err := orderNotification(after, after, time.Now()); err == nil {
		t.Fatal("replayed state must not create a new event")
	}
}

func TestOrderNotificationRetainsFormerRiderOnRelease(t *testing.T) {
	rider := uint64(61)
	before := Order{DeliveryOrderPublicID: "01J00000000000000000000811", MerchantUserID: 42, RowVersion: 7, Status: StatusClaimed, RiderUserID: &rider}
	after := before
	after.Status, after.RowVersion, after.RiderUserID = StatusReadyForClaim, 8, nil
	event, err := orderNotification(before, after, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var payload OrderNotificationPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.RiderUserID != nil || payload.PreviousRiderUserID == nil || *payload.PreviousRiderUserID != rider {
		t.Fatal("release recipient lost")
	}
}

func TestFinancialNotificationPayloadIsMetadataOnlyAndVersioned(t *testing.T) {
	now := time.Now().UTC()
	event, err := newMetadataNotificationEvent(EventRiderIncomeRecorded,
		"01J00000000000000000000811", 3, RiderIncomeNotificationPayload{
			EarningPublicID: "01J00000000000000000000812", OrderPublicID: "01J00000000000000000000813",
			RiderUserID: 61, AmountCents: 120, SettlementStatus: string(RiderEarningPending)}, now)
	if err != nil || event.AggregateVersion != 3 || event.EventType != EventRiderIncomeRecorded {
		t.Fatalf("invalid financial event: %#v %v", event, err)
	}
	for _, forbidden := range []string{"openid", "phone", "address", "secret", "paymentReference", "note"} {
		if strings.Contains(strings.ToLower(string(event.Payload)), strings.ToLower(forbidden)) {
			t.Fatalf("sensitive key leaked: %s", forbidden)
		}
	}
}

func TestFinancialNotificationPortPreservesTransactionAndDisabledPath(t *testing.T) {
	tx := &gorm.DB{}
	want := errors.New("event persistence failed")
	event, err := newMetadataNotificationEvent(EventRiderWithdrawalChanged,
		"01J00000000000000000000811", 1, RiderWithdrawalNotificationPayload{
			WithdrawPublicID: "01J00000000000000000000812", RiderUserID: 61, AmountCents: 100, Status: string(RiderWithdrawalProcessing)}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	appendEvent := func(got *gorm.DB, _ NotificationEvent) error {
		calls++
		if got != tx {
			t.Fatal("appender received a different transaction")
		}
		return want
	}
	if !errors.Is(appendMetadataNotificationTx(tx, appendEvent, event), want) || calls != 1 {
		t.Fatal("financial event failure must abort caller transaction")
	}
	if appendMetadataNotificationTx(nil, nil, NotificationEvent{}) != nil {
		t.Fatal("disabled rollout changed legacy behavior")
	}
	if appendFinancialNotificationTx(nil, nil, "", "", 0, nil, time.Time{}) != nil {
		t.Fatal("disabled rollout must skip financial event generation")
	}
}

func TestRefundNotificationOnlyAppendsOnVisibleStatusChange(t *testing.T) {
	before := Refund{RefundPublicID: "01J00000000000000000000811", DeliveryOrderPublicID: "01J00000000000000000000812",
		MerchantUserID: 42, AmountCents: 150, Status: RefundRetryPending, RowVersion: 3}
	after := before
	after.RowVersion++
	calls := 0
	appendEvent := func(_ *gorm.DB, event NotificationEvent) error {
		calls++
		if event.AggregateVersion != after.RowVersion || event.AggregatePublicID != before.RefundPublicID {
			t.Fatal("notification is not bound to business version")
		}
		return nil
	}
	tx := &gorm.DB{}
	if err := appendRefundNotificationTx(tx, appendEvent, &before, after, time.Now()); err != nil || calls != 0 {
		t.Fatalf("repeated processing generated notification: calls=%d err=%v", calls, err)
	}
	after.Status = RefundSucceeded
	if err := appendRefundNotificationTx(tx, appendEvent, &before, after, time.Now()); err != nil || calls != 1 {
		t.Fatalf("success event missing: calls=%d err=%v", calls, err)
	}
	after.RowVersion++
	if err := appendRefundNotificationTx(tx, appendEvent, &before, after, time.Now()); !errors.Is(err, ErrInvalidRecord) || calls != 1 {
		t.Fatal("unbound version accepted")
	}
}
