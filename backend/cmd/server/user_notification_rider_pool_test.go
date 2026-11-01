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

// Approval and an active account are not evidence of on-duty, service-range,
// or capacity eligibility. Until those facts have an authoritative source,
// even an approved template and explicit test-recipient allowlist must not
// turn RiderPoolOpened into a candidate-rider broadcast.
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
