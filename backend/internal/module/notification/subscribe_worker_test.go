package notification

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type workerStoreFake struct {
	lease                    DeliveryLease
	found                    bool
	claimCalls, reserveCalls int
	reserveErr               error
	finishErr                error
	outcomes                 []SubscribeOutcome
	finishedLease            DeliveryLease
}

func (s *workerStoreFake) ClaimWechatDelivery(context.Context, time.Time, time.Duration) (DeliveryLease, bool, error) {
	s.claimCalls++
	found := s.found
	s.found = false
	return s.lease, found, nil
}
func (s *workerStoreFake) ReserveForDelivery(_ context.Context, lease DeliveryLease, _, _, _ string, _ time.Time) (DeliveryLease, error) {
	s.reserveCalls++
	if s.reserveErr != nil {
		return DeliveryLease{}, s.reserveErr
	}
	id := uint64(42)
	nonce := lease.Item.DeliveryPublicID
	lease.Item.SubscriptionID = &id
	lease.Item.SubscriptionNonce = &nonce
	lease.Item.RowVersion++
	return lease, nil
}
func (s *workerStoreFake) FinishSubscribe(_ context.Context, lease DeliveryLease, outcome SubscribeOutcome, _ time.Time) error {
	s.finishedLease = lease
	s.outcomes = append(s.outcomes, outcome)
	return s.finishErr
}

type workerSenderFake struct {
	enabled bool
	calls   int
	result  SubscribeSendResult
	err     error
	onSend  func()
}

func (s *workerSenderFake) Enabled() bool { return s.enabled }
func (s *workerSenderFake) Send(context.Context, SubscribeMessage) (SubscribeSendResult, error) {
	s.calls++
	if s.onSend != nil {
		s.onSend()
	}
	return s.result, s.err
}

func testSubscribeWorker(t *testing.T) (*SubscribeWorker, *workerStoreFake, *workerSenderFake) {
	t.Helper()
	now := time.Date(2026, 11, 1, 5, 0, 0, 0, time.UTC)
	expires := now.Add(time.Minute)
	token := "lease"
	snapshot, _ := json.Marshal(TemplateSnapshot{TemplateID: "approved-template", Page: "/pages/delivery-rider/index", Data: map[string]SubscribeTemplateValue{"thing1": {Value: "配送中"}}})
	store := &workerStoreFake{found: true, lease: DeliveryLease{LeaseToken: token, Item: DeliveryItem{ID: 1, DeliveryPublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Channel: ChannelWechatSubscribe, RecipientUserID: 7, TemplateKey: "order", TemplateSnapshot: snapshot, RowVersion: 2, AttemptCount: 1, LeaseToken: &token, LeaseExpiresAt: &expires}}}
	sender := &workerSenderFake{enabled: true, result: SubscribeSendResult{Status: SendStatusAccepted}}
	worker := NewSubscribeWorker(store, sender, func(context.Context, DeliveryItem, TemplateSnapshot) (string, error) { return "test-recipient", nil }, "test-app")
	worker.Now = func() time.Time { return now }
	return worker, store, sender
}

func TestSubscribeWorkerNeverClaimsWhenDisabled(t *testing.T) {
	w, store, sender := testSubscribeWorker(t)
	sender.enabled = false
	if found, err := w.RunOnce(context.Background()); err != nil || found || store.claimCalls != 0 || sender.calls != 0 {
		t.Fatal("disabled worker consumed work")
	}
}
func TestSubscribeWorkerChecksConsentAndKeepsOriginalLeaseOnMissingQuota(t *testing.T) {
	w, store, sender := testSubscribeWorker(t)
	store.reserveErr = ErrSubscriptionEmpty
	if _, err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sender.calls != 0 || len(store.outcomes) != 1 || store.outcomes[0].Status != DeliveryBlocked || store.finishedLease.Item.ID != 1 || store.finishedLease.Item.RowVersion != 2 {
		t.Fatal("missing quota attempted send or lost the claimed lease")
	}
}
func TestSubscribeWorkerClassifiesProviderOutcome(t *testing.T) {
	for _, tc := range []struct {
		name, status, code string
		err                error
		want               DeliveryStatus
		quota              string
	}{
		{"accepted", SendStatusAccepted, "", nil, DeliveryAccepted, "consume"},
		{"accepted_with_error", SendStatusAccepted, "", errors.New("ambiguous"), DeliveryUnknown, "consume"},
		{"timeout", SendStatusUnknown, "timeout", ErrSubscribeUnknown, DeliveryUnknown, "consume"},
		{"rate_limit", SendStatusRejected, "rate_limited", ErrSubscribeRejected, DeliveryRetryWait, "release"},
		{"token_invalid", SendStatusRejected, "token_invalid", ErrSubscribeRejected, DeliveryRetryWait, "release"},
		{"revoked", SendStatusRejected, "subscription_rejected", ErrSubscribeRejected, DeliveryBlocked, "invalidate"},
		{"template_invalid", SendStatusRejected, "template_invalid", ErrSubscribeRejected, DeliveryBlocked, "release"},
		{"token_transport", SendStatusBlocked, "transport", ErrSubscribeBlocked, DeliveryRetryWait, "release"},
		{"unrecognized", "unexpected", "", nil, DeliveryUnknown, "consume"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, store, sender := testSubscribeWorker(t)
			sender.result = SubscribeSendResult{Status: tc.status, ErrorCode: tc.code}
			sender.err = tc.err
			if _, err := w.RunOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			if sender.calls != 1 || len(store.outcomes) != 1 || store.outcomes[0].Status != tc.want || store.outcomes[0].QuotaAction != tc.quota || !validSubscribeOutcome(store.outcomes[0]) {
				t.Fatalf("incorrect outcome: %+v", store.outcomes)
			}
			if _, err := w.RunOnce(context.Background()); err != nil || sender.calls != 1 {
				t.Fatal("terminal result was retried")
			}
		})
	}
}
func TestSubscribeWorkerIneligibleExpiredAndBoundedRetry(t *testing.T) {
	t.Run("ineligible", func(t *testing.T) {
		w, store, sender := testSubscribeWorker(t)
		w.Guard = func(context.Context, DeliveryItem, TemplateSnapshot) (string, error) {
			return "", ErrNotificationIneligible
		}
		_, _ = w.RunOnce(context.Background())
		if sender.calls != 0 || store.reserveCalls != 0 || store.outcomes[0].Status != DeliverySkipped {
			t.Fatal("ineligible user received notification")
		}
	})
	t.Run("expired", func(t *testing.T) {
		w, store, sender := testSubscribeWorker(t)
		expired := w.Now()
		store.lease.Item.ExpiresAt = &expired
		_, _ = w.RunOnce(context.Background())
		if sender.calls != 0 || store.reserveCalls != 0 || store.outcomes[0].Status != DeliverySkipped {
			t.Fatal("expired event attempted send")
		}
	})
	t.Run("deadletter", func(t *testing.T) {
		w, store, sender := testSubscribeWorker(t)
		store.lease.Item.AttemptCount = 5
		sender.result = SubscribeSendResult{Status: SendStatusRejected, ErrorCode: "rate_limited"}
		_, _ = w.RunOnce(context.Background())
		if store.outcomes[0].Status != DeliveryDeadletter || store.outcomes[0].NextAttemptAt != nil {
			t.Fatal("retry not bounded")
		}
	})
	t.Run("failed reservation", func(t *testing.T) {
		w, store, sender := testSubscribeWorker(t)
		store.reserveErr = errors.New("database unavailable")
		_, err := w.RunOnce(context.Background())
		if err == nil || sender.calls != 0 {
			t.Fatal("sent before committed reservation")
		}
	})
}
func TestSubscribeWorkerUnknownCommitFailureDoesNotResend(t *testing.T) {
	w, store, sender := testSubscribeWorker(t)
	store.finishErr = ErrConcurrentUpdate
	sender.result = SubscribeSendResult{Status: SendStatusUnknown}
	if _, err := w.RunOnce(context.Background()); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatal("lost lease silently swallowed")
	}
	if len(store.outcomes) != 1 || store.outcomes[0].QuotaAction != "consume" {
		t.Fatal("unknown response released quota")
	}
	_, _ = w.RunOnce(context.Background())
	if sender.calls != 1 {
		t.Fatal("sent again after uncertain commit")
	}
}
func TestAuthorizationTemplateTagChangesWithAppAndTemplate(t *testing.T) {
	base := authorizationTemplateTag("app", "order", "template")
	for _, tag := range []string{authorizationTemplateTag("other-app", "order", "template"), authorizationTemplateTag("app", "private_message", "template"), authorizationTemplateTag("app", "order", "new-template")} {
		if tag == base {
			t.Fatal("authorization transferable across template identity")
		}
	}
}

func TestSubscribeWorkerRechecksAccountAfterReservation(t *testing.T) {
	for _, changed := range []string{"openid", "permission", "lookup_error"} {
		t.Run(changed, func(t *testing.T) {
			w, store, sender := testSubscribeWorker(t)
			checks := 0
			w.Guard = func(context.Context, DeliveryItem, TemplateSnapshot) (string, error) {
				checks++
				if checks == 1 {
					return "old-openid", nil
				}
				if changed == "permission" {
					return "", ErrNotificationIneligible
				}
				if changed == "lookup_error" {
					return "", errors.New("lookup unavailable")
				}
				return "new-openid", nil
			}
			if _, err := w.RunOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			if checks != 2 || sender.calls != 0 || store.reserveCalls != 1 || len(store.outcomes) != 1 || store.outcomes[0].QuotaAction != "release" {
				t.Fatal("identity or preference changed during quota lock wait but message sent")
			}
		})
	}
}
