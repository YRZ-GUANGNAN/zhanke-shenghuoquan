package notification

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type wecomWorkerStoreFake struct {
	lease       DeliveryLease
	found       bool
	completed   DeliveryStatus
	failed      bool
	failureNext time.Time
}

func (f *wecomWorkerStoreFake) ClaimWecomDelivery(context.Context, time.Time, time.Duration) (DeliveryLease, bool, error) {
	return f.lease, f.found, nil
}

func (f *wecomWorkerStoreFake) CompleteDelivery(_ context.Context, _ DeliveryLease, status DeliveryStatus, _ time.Time) error {
	f.completed = status
	return nil
}

func (f *wecomWorkerStoreFake) FailDelivery(_ context.Context, _ DeliveryLease, _ string, _ string, next time.Time, _ uint32, _ time.Time) (DeliveryStatus, error) {
	f.failed = true
	f.failureNext = next
	return DeliveryRetryWait, nil
}

type wecomWorkerSenderFake struct {
	status SendStatus
	err    error
	sent   bool
}

func (f *wecomWorkerSenderFake) Enabled(string) bool { return true }
func (f *wecomWorkerSenderFake) Send(context.Context, WecomGroupSnapshot) (SendStatus, error) {
	f.sent = true
	return f.status, f.err
}

func TestWecomWorkerAcceptsExpiredAndRetriesFailures(t *testing.T) {
	now := time.Date(2026, 11, 13, 2, 0, 0, 0, time.UTC)
	snapshot, _ := json.Marshal(WecomGroupSnapshot{
		Version: 1, GroupKey: "merchant_ops", Title: "商家订单", Summary: "待接单",
		AggregateType: "delivery_order", AggregatePublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		AggregateVersion: 2, OccurredAt: now,
	})
	token := "lease-token"
	expires := now.Add(time.Minute)
	lease := DeliveryLease{LeaseToken: token, Item: DeliveryItem{
		ID: 1, DeliveryPublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAW", EventPublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		RecipientUserID: WecomMerchantGroupRecipientID, Channel: ChannelWecom, Purpose: PurposeOrder,
		TemplateKey: "merchant_ops", TemplateSnapshot: snapshot, PayloadVersion: 1,
		Status: DeliveryProcessing, AttemptCount: 1, RowVersion: 2, LeaseToken: &token, LeaseExpiresAt: &expires,
	}}
	store := &wecomWorkerStoreFake{lease: lease, found: true}
	sender := &wecomWorkerSenderFake{status: SendStatusAccepted}
	worker := NewWecomWorker(store, sender)
	worker.Now = func() time.Time { return now }
	if found, err := worker.RunOnce(context.Background()); err != nil || !found || store.completed != DeliveryAccepted || !sender.sent {
		t.Fatalf("accepted group delivery found=%v completed=%s sent=%v err=%v", found, store.completed, sender.sent, err)
	}

	store = &wecomWorkerStoreFake{lease: lease, found: true}
	sender = &wecomWorkerSenderFake{status: SendStatusUnknown, err: errors.New("transport")}
	worker = NewWecomWorker(store, sender)
	worker.Now = func() time.Time { return now }
	if found, err := worker.RunOnce(context.Background()); err != nil || !found || !store.failed || store.failureNext.IsZero() {
		t.Fatalf("failed group delivery found=%v failed=%v next=%v err=%v", found, store.failed, store.failureNext, err)
	}

	expired := lease
	expired.Item.ExpiresAt = ptrTime(now)
	store = &wecomWorkerStoreFake{lease: expired, found: true}
	sender = &wecomWorkerSenderFake{status: SendStatusAccepted}
	worker = NewWecomWorker(store, sender)
	worker.Now = func() time.Time { return now }
	if _, err := worker.RunOnce(context.Background()); err != nil || store.completed != DeliverySkipped || sender.sent {
		t.Fatalf("expired group event completed=%s sent=%v err=%v", store.completed, sender.sent, err)
	}
}

func ptrTime(value time.Time) *time.Time { return &value }
