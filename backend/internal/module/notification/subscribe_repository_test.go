package notification

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"
)

func newSubscribeSQLFixture(t *testing.T) (*GormRepository, *eventSQLState, DeliveryLease) {
	t.Helper()
	events, state := newEventSQLRepository(t, testLease(t))
	expires := state.now.Add(time.Minute)
	token := "01J00000000000000000000011"
	item := DeliveryItem{ID: 11, DeliveryPublicID: "01J00000000000000000000012", EventPublicID: "01J00000000000000000000001", RecipientUserID: 9, Channel: ChannelWechatSubscribe, Purpose: PurposeOrder, TemplateKey: "order", Status: DeliveryProcessing, RowVersion: 3, LeaseToken: &token, LeaseExpiresAt: &expires}
	state.tables["delivery_notification_deliveries"] = []map[string]driver.Value{{"id": int64(item.ID), "delivery_public_id": item.DeliveryPublicID, "event_public_id": item.EventPublicID, "recipient_user_id": int64(item.RecipientUserID), "channel": string(item.Channel), "purpose": string(item.Purpose), "template_key": item.TemplateKey, "status": string(item.Status), "row_version": int64(item.RowVersion), "lease_token": token, "lease_expires_at": expires, "subscription_id": nil, "subscription_nonce": nil, "expires_at": nil}}
	state.tables["notification_subscriptions"] = []map[string]driver.Value{subscriptionSQLRow(20, state.now)}
	return events.notifications, state, DeliveryLease{Item: item, LeaseToken: token}
}

func subscriptionSQLRow(id int64, now time.Time) map[string]driver.Value {
	return map[string]driver.Value{"id": id, "user_id": int64(9), "app_id": "wx-test", "openid": "private-openid", "template_key": "order", "authorization_nonce": strings.Repeat("a", 32) + authorizationTemplateTag("wx-test", "order", "reviewed-template"), "status": "accepted", "remaining_quota": int64(1), "reserved_quota": int64(0), "reservation_nonce": nil, "reserved_at": nil, "expires_at": nil, "row_version": int64(1), "invalidation_reason": authorizationReported + "accepted", "created_at": now, "updated_at": now}
}

func reserveTestSubscription(t *testing.T, repo *GormRepository, state *eventSQLState, lease DeliveryLease) DeliveryLease {
	t.Helper()
	reserved, err := repo.ReserveForDelivery(context.Background(), lease, "wx-test", "private-openid", "reviewed-template", state.now)
	if err != nil {
		t.Fatal(err)
	}
	return reserved
}

func TestReserveForDeliveryAtomicallyBindsQuotaAndLease(t *testing.T) {
	repo, state, lease := newSubscribeSQLFixture(t)
	reserved := reserveTestSubscription(t, repo, state, lease)
	if reserved.Item.SubscriptionID == nil || *reserved.Item.SubscriptionID != 20 || reserved.Item.RowVersion != lease.Item.RowVersion+1 {
		t.Fatalf("bad returned lease %+v", reserved.Item)
	}
	row := state.tables["notification_subscriptions"][0]
	if row["remaining_quota"] != int64(0) || row["reserved_quota"] != int64(1) || row["reservation_nonce"] != lease.Item.DeliveryPublicID {
		t.Fatalf("bad quota %+v", row)
	}
	locks := 0
	for _, query := range state.queries {
		if strings.Contains(query, "FOR UPDATE") {
			locks++
		}
	}
	if locks != 2 {
		t.Fatalf("delivery/subscription not both locked: count=%d", locks)
	}
	if _, err := repo.ReserveForDelivery(context.Background(), lease, "wx-test", "private-openid", "reviewed-template", state.now); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("old lease reused: %v", err)
	}
}

func TestReserveForDeliveryRejectsIdentityTemplateAndStaleLease(t *testing.T) {
	for _, test := range []struct {
		name             string
		mutate           func(*eventSQLState, *DeliveryLease)
		openid, template string
		want             error
	}{
		{name: "openid", openid: "other-openid", template: "reviewed-template", want: ErrSubscriptionEmpty},
		{name: "template-tag", openid: "private-openid", template: "new-template", want: ErrSubscriptionEmpty},
		{name: "row-version", openid: "private-openid", template: "reviewed-template", want: ErrConcurrentUpdate, mutate: func(_ *eventSQLState, l *DeliveryLease) { l.Item.RowVersion++ }},
		{name: "database-expiry", openid: "private-openid", template: "reviewed-template", want: ErrConcurrentUpdate, mutate: func(s *eventSQLState, l *DeliveryLease) { s.now = l.Item.LeaseExpiresAt.Add(time.Second) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, state, lease := newSubscribeSQLFixture(t)
			clientNow := state.now
			if test.mutate != nil {
				test.mutate(state, &lease)
			}
			_, err := repo.ReserveForDelivery(context.Background(), lease, "wx-test", test.openid, test.template, clientNow)
			if !errors.Is(err, test.want) {
				t.Fatalf("err=%v want=%v", err, test.want)
			}
			if state.tables["notification_subscriptions"][0]["remaining_quota"] != int64(1) {
				t.Fatal("failed precondition consumed quota")
			}
		})
	}
}

func TestReserveForDeliveryRollsBackQuotaIfLeaseBindingFails(t *testing.T) {
	repo, state, lease := newSubscribeSQLFixture(t)
	state.failDeliveryUpdate = true
	if _, err := repo.ReserveForDelivery(context.Background(), lease, "wx-test", "private-openid", "reviewed-template", state.now); err == nil {
		t.Fatal("expected binding failure")
	}
	row := state.tables["notification_subscriptions"][0]
	if row["remaining_quota"] != int64(1) || row["reserved_quota"] != int64(0) || row["reservation_nonce"] != nil {
		t.Fatalf("reservation leaked after rollback: %+v", row)
	}
}

func TestFinishSubscribeConsumesAcceptedAndUnknownWithoutRefund(t *testing.T) {
	for _, status := range []DeliveryStatus{DeliveryAccepted, DeliveryUnknown} {
		t.Run(string(status), func(t *testing.T) {
			repo, state, lease := newSubscribeSQLFixture(t)
			reserved := reserveTestSubscription(t, repo, state, lease)
			err := repo.FinishSubscribe(context.Background(), reserved, SubscribeOutcome{Status: status, QuotaAction: "consume"}, state.now)
			if err != nil {
				t.Fatal(err)
			}
			row := state.tables["notification_subscriptions"][0]
			delivery := state.tables["delivery_notification_deliveries"][0]
			if row["status"] != "exhausted" || row["remaining_quota"] != int64(0) || row["reserved_quota"] != int64(0) || row["reservation_nonce"] != nil || delivery["status"] != string(status) {
				t.Fatalf("bad completion subscription=%+v delivery=%+v", row, delivery)
			}
			if err := repo.FinishSubscribe(context.Background(), reserved, SubscribeOutcome{Status: status, QuotaAction: "consume"}, state.now); !errors.Is(err, ErrConcurrentUpdate) {
				t.Fatalf("stale completion allowed: %v", err)
			}
		})
	}
}

func TestFinishSubscribeExplicitRejectionReleasesOrInvalidatesQuota(t *testing.T) {
	for _, action := range []string{"release", "invalidate"} {
		t.Run(action, func(t *testing.T) {
			repo, state, lease := newSubscribeSQLFixture(t)
			reserved := reserveTestSubscription(t, repo, state, lease)
			if err := repo.FinishSubscribe(context.Background(), reserved, SubscribeOutcome{Status: DeliveryBlocked, Code: "provider_rejected", QuotaAction: action}, state.now); err != nil {
				t.Fatal(err)
			}
			row := state.tables["notification_subscriptions"][0]
			if action == "release" && (row["remaining_quota"] != int64(1) || row["status"] != "accepted") {
				t.Fatalf("explicit no-send not released %+v", row)
			}
			if action == "invalidate" && (row["remaining_quota"] != int64(0) || row["status"] != "invalid") {
				t.Fatalf("provider invalidation not preserved %+v", row)
			}
		})
	}
}

func TestFinishSubscribeFailureRollsBackQuotaAndDelivery(t *testing.T) {
	repo, state, lease := newSubscribeSQLFixture(t)
	reserved := reserveTestSubscription(t, repo, state, lease)
	state.failDeliveryUpdate = true
	if err := repo.FinishSubscribe(context.Background(), reserved, SubscribeOutcome{Status: DeliveryAccepted, QuotaAction: "consume"}, state.now); err == nil {
		t.Fatal("expected delivery CAS/write failure")
	}
	row := state.tables["notification_subscriptions"][0]
	delivery := state.tables["delivery_notification_deliveries"][0]
	if row["status"] != "accepted" || row["remaining_quota"] != int64(0) || row["reserved_quota"] != int64(1) || delivery["status"] != string(DeliveryProcessing) {
		t.Fatalf("partial finish committed subscription=%+v delivery=%+v", row, delivery)
	}
}
