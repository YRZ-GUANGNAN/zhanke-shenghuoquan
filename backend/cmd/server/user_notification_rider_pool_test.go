package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/notification"
	"campus-circle/backend/internal/module/user"
)

// Approval and an active account are not evidence of duty or capacity. Without
// an injected authoritative store, a template and allowlist cannot broadcast.
func TestRiderPoolNotificationRequiresAuthoritativeAvailability(t *testing.T) {
	now := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	for _, status := range []delivery.Status{
		delivery.StatusPendingFeePayment, delivery.StatusReadyForClaim,
		delivery.StatusClaimed, delivery.StatusCancelled,
	} {
		t.Run(string(status), func(t *testing.T) {
			users := &riderPoolActiveUsers{}
			resolver := &orderNotificationRecipients{
				orders: notificationOrdersStub{order: delivery.Order{
					DeliveryOrderPublicID: notificationTestOrderID,
					MerchantUserID:        1, Status: status, RowVersion: 2,
				}},
				users: users, now: func() time.Time { return now },
				template: &orderNotificationTemplate{
					id: "reviewed-order-template", fields: map[string]string{"thing1": "status"},
					users: map[uint64]bool{1: true, 2: true},
				},
			}
			recipients, err := resolver.ResolveRecipients(context.Background(), riderPoolEvent(now))
			if err != nil || len(recipients) != 0 {
				t.Fatalf("missing availability facts must skip broadcast: recipients=%d err=%v", len(recipients), err)
			}
			if users.accountReads != 0 || users.preferenceReads != 0 {
				t.Fatal("account activity or preferences were used as rider availability")
			}
		})
	}
}

func TestRiderPoolSubscribeGuardRejectsPreviouslyCreatedTaskWithoutAvailability(t *testing.T) {
	now := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	event := riderPoolEvent(now)
	resolver := &orderNotificationRecipients{
		now: func() time.Time { return now },
		template: &orderNotificationTemplate{
			id: "reviewed-order-template", users: map[uint64]bool{2: true},
		},
	}
	lookupCalls := 0
	guard := orderSubscribeGuard(riderPoolEventReader{event: event}, resolver,
		func(context.Context, uint64, string) (string, error) {
			lookupCalls++
			return "must-not-be-used", nil
		}, "test-app")
	item := notification.DeliveryItem{
		EventPublicID: event.EventPublicID, RecipientUserID: 2,
		Purpose: notification.PurposeOrder, TemplateKey: "order",
		TemplateSnapshot: json.RawMessage(`{"templateId":"reviewed-order-template","page":"/pages/delivery-rider/index","data":{"thing1":{"value":"有新的配送订单"}}}`),
	}
	openid, err := guard(context.Background(), item, notification.TemplateSnapshot{TemplateID: "reviewed-order-template"})
	if !errors.Is(err, notification.ErrNotificationIneligible) || openid != "" || lookupCalls != 0 {
		t.Fatalf("old task bypassed current eligibility: openidResolved=%t calls=%d err=%v", openid != "", lookupCalls, err)
	}
}

func riderPoolEvent(now time.Time) notification.Event {
	expires := now.Add(10 * time.Minute)
	return notification.Event{
		EventPublicID: notificationTestOrderID, EventType: delivery.EventRiderPoolOpened,
		AggregateType: "delivery_order", AggregatePublicID: notificationTestOrderID,
		AggregateVersion: 2, PayloadVersion: 1, OccurredAt: now, ExpiresAt: &expires,
		Payload: json.RawMessage(`{"orderPublicId":"` + notificationTestOrderID + `","deliveryFeeCents":150}`),
	}
}

type riderPoolEventReader struct{ event notification.Event }

func (r riderPoolEventReader) FindEvent(context.Context, string) (notification.Event, error) {
	return r.event, nil
}

type riderPoolActiveUsers struct{ accountReads, preferenceReads int }

func (r *riderPoolActiveUsers) Get(_ context.Context, id uint) (user.User, error) {
	r.accountReads++
	return user.User{ID: id, Status: user.StatusActive}, nil
}

func (r *riderPoolActiveUsers) Settings(context.Context, uint) (user.SettingsView, error) {
	r.preferenceReads++
	return user.SettingsView{OrderNotice: true}, nil
}

type riderPoolAvailabilityStub struct {
	delivery.RiderAvailabilityRepository
	candidates []delivery.RiderAvailabilitySnapshot
	current    map[uint64]delivery.RiderAvailabilitySnapshot
	readErr    map[uint64]error
	listErr    error
	listCalls  int
	reads      []uint64
}

func (s *riderPoolAvailabilityStub) ListOnDutyRiderAvailability(context.Context, time.Time) ([]delivery.RiderAvailabilitySnapshot, error) {
	s.listCalls++
	return s.candidates, s.listErr
}

func (s *riderPoolAvailabilityStub) ReadRiderAvailability(_ context.Context, id uint64) (delivery.RiderAvailabilitySnapshot, error) {
	s.reads = append(s.reads, id)
	return s.current[id], s.readErr[id]
}

func riderPoolSnapshot(id uint64, now time.Time) delivery.RiderAvailabilitySnapshot {
	return delivery.RiderAvailabilitySnapshot{RiderUserID: id, State: delivery.RiderOnDuty,
		Scope: delivery.RiderAvailabilityScopeCampus, InFlight: 1, MaxInFlight: 3, RowVersion: 1, UpdatedAt: now}
}

func riderPoolResolver(now time.Time) (*orderNotificationRecipients, *riderPoolAvailabilityStub) {
	snapshot := riderPoolSnapshot(2, now)
	store := &riderPoolAvailabilityStub{candidates: []delivery.RiderAvailabilitySnapshot{snapshot},
		current: map[uint64]delivery.RiderAvailabilitySnapshot{2: snapshot}, readErr: map[uint64]error{}}
	return &orderNotificationRecipients{
		orders: notificationOrdersStub{order: delivery.Order{DeliveryOrderPublicID: notificationTestOrderID,
			MerchantUserID: 1, Status: delivery.StatusReadyForClaim, RowVersion: 2, DeliveryFeeCents: 150}},
		users: notificationUsersStub{}, now: func() time.Time { return now }, availability: store,
		template: &orderNotificationTemplate{id: "reviewed-order-template", users: map[uint64]bool{2: true},
			fields: map[string]string{"phrase1": "status", "thing2": "status", "character_string3": "orderId", "time4": "time"}},
	}, store
}

func TestRiderPoolNotificationBroadcastUsesFreshAvailabilityAndReviewedTemplate(t *testing.T) {
	now := time.Date(2026, 11, 4, 8, 0, 0, 0, time.UTC)
	resolver, store := riderPoolResolver(now)
	third := riderPoolSnapshot(3, now)
	store.candidates = append(store.candidates, store.candidates[0], third)
	store.current[3] = third
	event := riderPoolEvent(now)
	got, err := resolver.ResolveRecipients(context.Background(), event)
	if err != nil || len(got) != 2 || len(store.reads) != 2 {
		t.Fatalf("deduplicated fresh candidates: count=%d reads=%v err=%v", len(got), store.reads, err)
	}
	for _, recipient := range got {
		if recipient.Title != "有新的校园配送单" || recipient.TargetPath != "/pages/delivery-rider/index" || recipient.Purpose != notification.PurposeOrder || recipient.AggregateVersion != event.AggregateVersion {
			t.Fatalf("unexpected rider pool recipient: %+v", recipient)
		}
		if recipient.UserID == 3 && (recipient.AllowWechat || len(recipient.WechatTemplateSnapshot) != 0) {
			t.Fatal("rider outside reviewed allowlist received external notification")
		}
		if recipient.UserID == 2 {
			snapshot, err := notification.DecodeSubscribeSnapshot(recipient.WechatTemplateSnapshot)
			if err != nil || !recipient.AllowWechat || snapshot.Data["phrase1"].Value != "待接单" || snapshot.Data["thing2"].Value != "有新的校园配送单" || snapshot.Data["character_string3"].Value != notificationTestOrderID {
				t.Fatalf("invalid reviewed rider pool template: %+v err=%v", snapshot, err)
			}
		}
	}
}

func TestRiderPoolNotificationSuppressesUnavailableAndObsoleteFacts(t *testing.T) {
	now := time.Date(2026, 11, 4, 8, 0, 0, 0, time.UTC)
	for _, scenario := range []string{"off duty", "full", "lowered capacity", "revoked", "unregistered", "inactive account", "opted out", "claimed", "cancelled", "unpaid", "assigned", "new version", "expired"} {
		t.Run(scenario, func(t *testing.T) {
			resolver, store := riderPoolResolver(now)
			event := riderPoolEvent(now)
			current := store.current[2]
			order := resolver.orders.(notificationOrdersStub).order
			switch scenario {
			case "off duty":
				current.State = delivery.RiderOffDuty
			case "full":
				current.InFlight = current.MaxInFlight
			case "lowered capacity":
				current.MaxInFlight = 1
				current.InFlight = 2
			case "revoked":
				store.readErr[2] = delivery.ErrRiderUnavailable
			case "unregistered":
				store.readErr[2] = delivery.ErrRiderNotRegistered
			case "inactive account":
				resolver.users = notificationUsersStub{inactive: 2}
			case "opted out":
				resolver.users = notificationUsersStub{optedOut: 2}
			case "claimed":
				order.Status = delivery.StatusClaimed
			case "cancelled":
				order.Status = delivery.StatusCancelled
			case "unpaid":
				order.Status = delivery.StatusPendingFeePayment
			case "assigned":
				id := uint64(3)
				order.RiderUserID = &id
			case "new version":
				order.RowVersion++
			case "expired":
				event.OccurredAt = now.Add(-10 * time.Minute)
				event.ExpiresAt = &now
			}
			store.current[2] = current
			resolver.orders = notificationOrdersStub{order: order}
			got, err := resolver.ResolveRecipients(context.Background(), event)
			if err != nil || len(got) != 0 {
				t.Fatalf("ineligible broadcast count=%d err=%v", len(got), err)
			}
		})
	}
}

func TestRiderPoolNotificationRejectsMalformedFactsAndRetriesLookupFailures(t *testing.T) {
	now := time.Date(2026, 11, 4, 8, 0, 0, 0, time.UTC)
	for _, scenario := range []string{"missing expiry", "long ttl", "future event", "missing occurrence", "wrong public id", "wrong fee", "zero version", "invalid current", "read failed", "list failed"} {
		t.Run(scenario, func(t *testing.T) {
			resolver, store := riderPoolResolver(now)
			event := riderPoolEvent(now)
			switch scenario {
			case "missing expiry":
				event.ExpiresAt = nil
			case "long ttl":
				later := now.Add(time.Hour)
				event.ExpiresAt = &later
			case "future event":
				event.OccurredAt = now.Add(time.Second)
			case "missing occurrence":
				event.OccurredAt = time.Time{}
			case "wrong public id":
				event.Payload = json.RawMessage(`{"orderPublicId":"wrong","deliveryFeeCents":150}`)
			case "wrong fee":
				event.Payload = json.RawMessage(`{"orderPublicId":"` + notificationTestOrderID + `","deliveryFeeCents":151}`)
			case "zero version":
				event.AggregateVersion = 0
			case "invalid current":
				current := store.current[2]
				current.Scope = "unknown"
				store.current[2] = current
			case "read failed":
				store.readErr[2] = errors.New("database unavailable")
			case "list failed":
				store.listErr = errors.New("database unavailable")
			}
			got, err := resolver.ResolveRecipients(context.Background(), event)
			if err == nil || len(got) != 0 || errors.Is(err, notification.ErrNotificationIneligible) {
				t.Fatalf("invalid fact or transient failure acknowledged: count=%d err=%v", len(got), err)
			}
		})
	}
}

func TestRiderPoolSubscribeGuardRechecksEligibilityBeforeSending(t *testing.T) {
	now := time.Date(2026, 11, 4, 8, 0, 0, 0, time.UTC)
	for _, scenario := range []string{"eligible", "stopped", "capacity filled", "claimed", "expired", "preference revoked", "allowlist removed"} {
		t.Run(scenario, func(t *testing.T) {
			resolver, store := riderPoolResolver(now)
			event := riderPoolEvent(now)
			recipients, err := resolver.ResolveRecipients(context.Background(), event)
			if err != nil || len(recipients) != 1 {
				t.Fatalf("fixture: count=%d err=%v", len(recipients), err)
			}
			recipient := recipients[0]
			item := notification.DeliveryItem{EventPublicID: event.EventPublicID, RecipientUserID: recipient.UserID,
				Purpose: recipient.Purpose, TemplateKey: recipient.WechatTemplateKey, TemplateSnapshot: recipient.WechatTemplateSnapshot}
			snapshot, err := notification.DecodeSubscribeSnapshot(item.TemplateSnapshot)
			if err != nil {
				t.Fatal(err)
			}
			current := store.current[2]
			switch scenario {
			case "stopped":
				current.State = delivery.RiderOffDuty
			case "capacity filled":
				current.InFlight = current.MaxInFlight
			case "claimed":
				order := resolver.orders.(notificationOrdersStub).order
				order.Status = delivery.StatusClaimed
				resolver.orders = notificationOrdersStub{order: order}
			case "expired":
				resolver.now = func() time.Time { return now.Add(10 * time.Minute) }
			case "preference revoked":
				resolver.users = notificationUsersStub{optedOut: 2}
			case "allowlist removed":
				delete(resolver.template.users, 2)
			}
			store.current[2] = current
			lookups := 0
			guard := orderSubscribeGuard(riderPoolEventReader{event: event}, resolver,
				func(context.Context, uint64, string) (string, error) { lookups++; return "test-openid", nil }, "test-app")
			openid, err := guard(context.Background(), item, snapshot)
			if scenario == "eligible" {
				if err != nil || openid != "test-openid" || lookups != 1 || len(store.reads) != 2 {
					t.Fatalf("eligible guard failed: lookups=%d reads=%v err=%v", lookups, store.reads, err)
				}
			} else if !errors.Is(err, notification.ErrNotificationIneligible) || openid != "" || lookups != 0 {
				t.Fatalf("stale task was sent: lookups=%d err=%v", lookups, err)
			}
		})
	}
}

func TestRiderPoolSubscribeGuardOnlyReadsCurrentRecipient(t *testing.T) {
	now := time.Date(2026, 11, 6, 8, 0, 0, 0, time.UTC)
	resolver, store := riderPoolResolver(now)
	for id := uint64(3); id <= 102; id++ {
		candidate := riderPoolSnapshot(id, now)
		store.candidates = append(store.candidates, candidate)
		store.current[id] = candidate
	}
	event := riderPoolEvent(now)
	recipients, err := resolver.ResolveRecipients(context.Background(), event)
	if err != nil || len(recipients) != 101 || store.listCalls != 1 {
		t.Fatalf("initial expansion must scan the duty pool once: count=%d scans=%d err=%v", len(recipients), store.listCalls, err)
	}
	recipient := recipients[0]
	item := notification.DeliveryItem{EventPublicID: event.EventPublicID, RecipientUserID: recipient.UserID,
		Purpose: recipient.Purpose, TemplateKey: recipient.WechatTemplateKey, TemplateSnapshot: recipient.WechatTemplateSnapshot}
	snapshot, err := notification.DecodeSubscribeSnapshot(item.TemplateSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	store.reads = nil
	store.listErr = errors.New("sending must not scan the duty pool")
	lookupCalls := 0
	guard := orderSubscribeGuard(riderPoolEventReader{event: event}, resolver,
		func(_ context.Context, id uint64, _ string) (string, error) {
			lookupCalls++
			if id != item.RecipientUserID {
				t.Fatalf("looked up another recipient: %d", id)
			}
			return "test-openid", nil
		}, "test-app")
	openid, err := guard(context.Background(), item, snapshot)
	if err != nil || openid != "test-openid" || lookupCalls != 1 || store.listCalls != 1 ||
		len(store.reads) != 1 || store.reads[0] != item.RecipientUserID {
		t.Fatalf("send guard must only recheck its recipient: scans=%d reads=%v lookups=%d err=%v", store.listCalls, store.reads, lookupCalls, err)
	}
}

type riderPoolChangingOrder struct {
	delivery.Order
	reads int
}

func (s *riderPoolChangingOrder) Find(context.Context, string) (delivery.Order, error) {
	s.reads++
	if s.reads > 1 {
		s.Status = delivery.StatusClaimed
		s.RowVersion++
	}
	return s.Order, nil
}

func TestRiderPoolNotificationRechecksOrderAfterCandidateScan(t *testing.T) {
	now := time.Date(2026, 11, 4, 8, 0, 0, 0, time.UTC)
	resolver, store := riderPoolResolver(now)
	orders := &riderPoolChangingOrder{Order: resolver.orders.(notificationOrdersStub).order}
	resolver.orders = orders
	got, err := resolver.ResolveRecipients(context.Background(), riderPoolEvent(now))
	if err != nil || len(got) != 0 || len(store.reads) != 1 || orders.reads != 2 {
		t.Fatalf("claim during candidate scan was ignored: count=%d reads=%d err=%v", len(got), orders.reads, err)
	}
}

type riderPoolQuietUsers struct{ notificationUsersStub }

func (riderPoolQuietUsers) Settings(context.Context, uint) (user.SettingsView, error) {
	return user.SettingsView{OrderNotice: true, QuietHours: true}, nil
}

func TestRiderPoolNotificationQuietHoursPreserveInAppOnly(t *testing.T) {
	for _, hour := range []int{6, 7, 22, 23} {
		t.Run(time.Date(2026, 11, 4, hour, 0, 0, 0, time.UTC).Format("15"), func(t *testing.T) {
			now := time.Date(2026, 11, 4, hour, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
			resolver, _ := riderPoolResolver(now)
			resolver.users = riderPoolQuietUsers{}
			got, err := resolver.ResolveRecipients(context.Background(), riderPoolEvent(now))
			if err != nil || len(got) != 1 || got[0].AllowWechat != (hour >= 7 && hour < 23) {
				t.Fatalf("quiet-hour policy changed: count=%d err=%v", len(got), err)
			}
		})
	}
}
