package notification

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type fakeEventStore struct {
	lease       EventLease
	found       bool
	claimErr    error
	failed      []string
	failStatus  EventStatus
	completed   []EventStatus
	inbox       []InboxItem
	deliveries  []DeliveryItem
	completedAt time.Time
	failedAt    time.Time
	nextAttempt time.Time
	failMessage string
	completeErr error
	failErr     error
}

func (f *fakeEventStore) ClaimNextEvent(context.Context, time.Time, time.Duration) (EventLease, bool, error) {
	if f.claimErr != nil {
		return EventLease{}, false, f.claimErr
	}
	if !f.found {
		return EventLease{}, false, nil
	}
	f.found = false
	return f.lease, true, nil
}
func (f *fakeEventStore) ExpandAndComplete(_ context.Context, _ EventLease, inbox []InboxItem, deliveries []DeliveryItem, status EventStatus, now time.Time) error {
	if f.completeErr != nil {
		return f.completeErr
	}
	f.inbox = append(f.inbox, inbox...)
	f.deliveries = append(f.deliveries, deliveries...)
	f.completed = append(f.completed, status)
	f.completedAt = now
	return nil
}
func (f *fakeEventStore) FailEvent(_ context.Context, lease EventLease, code, message string, next time.Time, max uint32, now time.Time) (EventStatus, error) {
	if f.failErr != nil {
		return "", f.failErr
	}
	f.failed = append(f.failed, code)
	f.failedAt = now
	f.nextAttempt = next
	f.failMessage = message
	if lease.Event.AttemptCount >= max {
		f.failStatus = EventDeadletter
		return EventDeadletter, nil
	}
	f.failStatus = EventRetryWait
	return EventRetryWait, nil
}

type fakeRecipients struct {
	items []Recipient
	err   error
}

func (f fakeRecipients) ResolveRecipients(context.Context, Event) ([]Recipient, error) {
	return f.items, f.err
}

func testEvent(t *testing.T) Event {
	t.Helper()
	return Event{ID: 1, EventPublicID: "01J00000000000000000000001", EventType: "OrderChanged", AggregateType: "delivery_order", AggregatePublicID: "01J00000000000000000000002", AggregateVersion: 3, PayloadVersion: 1, Payload: json.RawMessage(`{"status":"ready"}`), Status: EventProcessing, RowVersion: 2, AttemptCount: 1, OccurredAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now()}
}

func testLease(t *testing.T) EventLease {
	t.Helper()
	token := "01J00000000000000000000003"
	expires := time.Now().Add(time.Minute)
	e := testEvent(t)
	e.LeaseToken, e.LeaseExpiresAt = &token, &expires
	return EventLease{Event: e, LeaseToken: token}
}

func TestExpanderCreatesInAppAndAuthorizedWechatOnly(t *testing.T) {
	store := &fakeEventStore{lease: testLease(t), found: true}
	recipient := Recipient{UserID: 9, Purpose: PurposeOrder, Title: "订单更新", Summary: "订单状态已更新", AggregateType: "delivery_order", AggregatePublicID: "01J00000000000000000000002", AggregateVersion: 3, TargetPath: "/pages/delivery/order?id=01J00000000000000000000002", AllowWechat: true, WechatTemplateKey: "order_status", WechatTemplateSnapshot: json.RawMessage(`{"templateId":"reviewed-template","page":"/pages/delivery/order","data":{"thing1":{"value":"订单更新"}}}`)}
	seen, err := NewExpander(store, fakeRecipients{items: []Recipient{recipient}}).ExpandOnce(context.Background(), time.Now())
	if err != nil || !seen || len(store.inbox) != 1 || len(store.deliveries) != 2 || store.completed[0] != EventExpanded {
		t.Fatalf("seen=%v err=%v inbox=%d deliveries=%d completed=%v", seen, err, len(store.inbox), len(store.deliveries), store.completed)
	}
	if store.deliveries[0].Channel != ChannelInApp || store.deliveries[1].Channel != ChannelWechatSubscribe {
		t.Fatalf("channels=%v,%v", store.deliveries[0].Channel, store.deliveries[1].Channel)
	}
}

func TestExpanderFailsClosedForIncompleteWechatAndDeduplicatesRecipients(t *testing.T) {
	store := &fakeEventStore{lease: testLease(t), found: true}
	recipient := Recipient{UserID: 9, Purpose: PurposePrivateMessage, Title: "收到新消息", AggregateType: "delivery_order", AggregatePublicID: "01J00000000000000000000002", AggregateVersion: 3, TargetPath: "/pages/messages/index", AllowWechat: true, WechatTemplateKey: "", WechatTemplateSnapshot: json.RawMessage(`{"body":"private"}`)}
	_, err := NewExpander(store, fakeRecipients{items: []Recipient{recipient, recipient}}).ExpandOnce(context.Background(), time.Now())
	if err != nil || len(store.inbox) != 1 || len(store.deliveries) != 1 {
		t.Fatalf("err=%v inbox=%d deliveries=%d", err, len(store.inbox), len(store.deliveries))
	}
}

func TestExpanderSkipsExpiredEventWithoutRecipients(t *testing.T) {
	lease := testLease(t)
	expired := time.Now().Add(-time.Second)
	lease.Event.ExpiresAt = &expired
	store := &fakeEventStore{lease: lease, found: true}
	_, err := NewExpander(store, fakeRecipients{}).ExpandOnce(context.Background(), time.Now())
	if err != nil || len(store.completed) != 1 || store.completed[0] != EventSkipped || len(store.inbox) != 0 {
		t.Fatalf("err=%v completed=%v inbox=%d", err, store.completed, len(store.inbox))
	}
}

func TestExpanderResolverFailureIsRetryableAndLeaseErrorPropagates(t *testing.T) {
	store := &fakeEventStore{lease: testLease(t), found: true}
	want := errors.New("resolver unavailable")
	_, err := NewExpander(store, fakeRecipients{err: want}).ExpandOnce(context.Background(), time.Now())
	if err != nil || len(store.failed) != 1 || store.failed[0] != "recipient_resolve_failed" {
		t.Fatalf("err=%v failed=%v", err, store.failed)
	}
	store = &fakeEventStore{lease: testLease(t), found: true, completeErr: ErrConcurrentUpdate}
	_, err = NewExpander(store, fakeRecipients{}).ExpandOnce(context.Background(), time.Now())
	if !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("expected lease error, got %v", err)
	}
}

func TestExpanderUsesFreshCommitTimeAndRejectsResolverCoordinateMixup(t *testing.T) {
	store := &fakeEventStore{lease: testLease(t), found: true}
	initial := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	fresh := initial.Add(2 * time.Minute)
	expander := NewExpander(store, fakeRecipients{items: []Recipient{{UserID: 9, Purpose: PurposeOrder, Title: "订单更新", AggregateType: "other", AggregatePublicID: "01J00000000000000000000002", AggregateVersion: 3}}})
	expander.Now = func() time.Time { return fresh }
	_, err := expander.ExpandOnce(context.Background(), initial)
	if err != nil || len(store.failed) != 1 || !store.failedAt.Equal(fresh) {
		t.Fatalf("err=%v failed=%v at=%v", err, store.failed, store.failedAt)
	}
}

func TestExpanderPassesAttemptLimitForDeadletter(t *testing.T) {
	lease := testLease(t)
	lease.Event.AttemptCount = 2
	store := &fakeEventStore{lease: lease, found: true}
	expander := NewExpander(store, fakeRecipients{err: errors.New("temporary")})
	expander.MaxAttempts = 2
	_, err := expander.ExpandOnce(context.Background(), time.Now())
	if err != nil || store.failStatus != EventDeadletter {
		t.Fatalf("err=%v status=%s", err, store.failStatus)
	}
}

func TestExpanderCommitFailureSchedulesBoundedRetryWithoutLeakingError(t *testing.T) {
	lease := testLease(t)
	lease.Event.AttemptCount = 3
	store := &fakeEventStore{lease: lease, found: true, completeErr: errors.New("private-sql-credential")}
	now := time.Now().UTC()
	expander := NewExpander(store, fakeRecipients{})
	expander.Now = func() time.Time { return now }
	_, err := expander.ExpandOnce(context.Background(), now)
	if err == nil || len(store.failed) != 1 || store.failed[0] != "expansion_commit_failed" || store.failStatus != EventRetryWait {
		t.Fatalf("err=%v failed=%v status=%v", err, store.failed, store.failStatus)
	}
	if !store.nextAttempt.Equal(now.Add(2*time.Minute)) || store.failMessage != "recipient expansion failed" {
		t.Fatalf("next=%v safeMessage=%q", store.nextAttempt, store.failMessage)
	}
}

func TestExpanderRechecksExpiryAfterResolver(t *testing.T) {
	lease := testLease(t)
	initial := time.Now().UTC()
	expires := initial.Add(time.Second)
	lease.Event.ExpiresAt = &expires
	store := &fakeEventStore{lease: lease, found: true}
	expander := NewExpander(store, fakeRecipients{items: []Recipient{{UserID: 9, Purpose: PurposeOrder, Title: "订单更新", AggregateType: lease.Event.AggregateType, AggregatePublicID: lease.Event.AggregatePublicID, AggregateVersion: lease.Event.AggregateVersion}}})
	expander.Now = func() time.Time { return expires.Add(time.Second) }
	_, err := expander.ExpandOnce(context.Background(), initial)
	if err != nil || len(store.inbox) != 0 || len(store.deliveries) != 0 || len(store.completed) != 1 || store.completed[0] != EventSkipped {
		t.Fatalf("err=%v inbox=%d channels=%d completed=%v", err, len(store.inbox), len(store.deliveries), store.completed)
	}
}

func TestExpanderRejectsExpiredLeaseBeforeCommit(t *testing.T) {
	lease := testLease(t)
	store := &fakeEventStore{lease: lease, found: true}
	expander := NewExpander(store, fakeRecipients{})
	expander.Now = func() time.Time { return lease.Event.LeaseExpiresAt.Add(time.Second) }
	_, err := expander.ExpandOnce(context.Background(), time.Now())
	if !errors.Is(err, ErrConcurrentUpdate) || len(store.completed) != 0 {
		t.Fatalf("err=%v completed=%v", err, store.completed)
	}
}

func TestExpanderRecipientCoordinateDedupIsExactAndConflictAware(t *testing.T) {
	event := testEvent(t)
	a := Recipient{UserID: 0x110000, Purpose: PurposeOrder, Title: "订单更新", AggregateType: event.AggregateType, AggregatePublicID: event.AggregatePublicID, AggregateVersion: event.AggregateVersion}
	b := a
	b.UserID++
	expander := &Expander{}
	inbox, _, err := expander.build(event, []Recipient{a, b, a}, time.Now())
	if err != nil || len(inbox) != 2 {
		t.Fatalf("large ids collided: count=%d err=%v", len(inbox), err)
	}
	b = a
	b.Summary = "冲突文案"
	if _, _, err := expander.build(event, []Recipient{a, b}, time.Now()); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting recipient silently ignored: %v", err)
	}
}

func TestExpanderWechatSnapshotUsesSenderValidation(t *testing.T) {
	for _, raw := range []string{
		`{}`,
		`{"page":"/pages/messages/index","data":{"thing1":{"value":"新消息"}}}`,
		`{"templateId":"reviewed","page":"https://external.test","data":{"thing1":{"value":"新消息"}}}`,
		`{"templateId":"reviewed","page":"/pages/messages/index","data":{}}`,
		`{"templateId":"reviewed","page":"/pages/messages/index","data":{"phrase1":{"value":"超过五字限制"}}}`,
		`{"templateId":"reviewed","page":"/pages/messages/index","touser":"private-id","data":{"thing1":{"value":"新消息"}}}`,
	} {
		if validWechatTemplateSnapshot(json.RawMessage(raw)) {
			t.Fatalf("unsafe/incomplete snapshot accepted: %s", raw)
		}
	}
	if !validWechatTemplateSnapshot(json.RawMessage(`{"templateId":"reviewed","page":"/pages/messages/index","data":{"thing1":{"value":"新消息"}}}`)) {
		t.Fatal("complete reviewed snapshot rejected")
	}
}
