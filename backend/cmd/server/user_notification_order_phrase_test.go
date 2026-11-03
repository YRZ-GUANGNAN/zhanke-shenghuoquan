package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/notification"
)

func TestOrderNotificationPhraseCoversEveryNotifiableTransition(t *testing.T) {
	states := []delivery.Status{delivery.StatusPendingFeePayment, delivery.StatusPaymentFailed, delivery.StatusReadyForClaim,
		delivery.StatusClaimed, delivery.StatusToPickup, delivery.StatusPickedUp, delivery.StatusDelivering, delivery.StatusDelivered, delivery.StatusCancelled}
	labels := map[delivery.Status]struct{ inApp, wechat string }{
		delivery.StatusPaymentFailed: {"配送费支付失败", "支付失败"},
		delivery.StatusReadyForClaim: {"配送单等待接单", "待接单"},
		delivery.StatusClaimed:       {"骑手已接单", "已接单"},
		delivery.StatusToPickup:      {"骑手正在前往取货", "取货中"},
		delivery.StatusPickedUp:      {"骑手已取货", "已取货"},
		delivery.StatusDelivering:    {"配送中", "配送中"},
		delivery.StatusDelivered:     {"配送已完成", "已送达"},
		delivery.StatusCancelled:     {"配送单已取消", "已取消"},
	}
	for _, from := range states {
		for _, to := range states {
			if !delivery.CanTransition(from, to) {
				continue
			}
			t.Run(string(from)+"/"+string(to), func(t *testing.T) {
				now := time.Date(2026, 11, 3, 1, 2, 0, 0, time.UTC)
				payload, _ := json.Marshal(delivery.OrderNotificationPayload{OrderPublicID: notificationTestOrderID, MerchantUserID: 1, From: from, To: to})
				event := notification.Event{EventPublicID: notificationTestOrderID, EventType: delivery.EventOrderStatusChanged,
					AggregateType: "delivery_order", AggregatePublicID: notificationTestOrderID, AggregateVersion: 3, PayloadVersion: 1, Payload: payload, OccurredAt: now}
				resolver := &orderNotificationRecipients{orders: notificationOrdersStub{order: delivery.Order{MerchantUserID: 1, Status: to, RowVersion: 3}},
					users: notificationUsersStub{}, now: func() time.Time { return now },
					template: &orderNotificationTemplate{id: "reviewed-order-status-template", fields: map[string]string{
						"character_string1": "orderId", "phrase3": "status", "time4": "time", "thing5": "status",
					}, users: map[uint64]bool{1: true}}}
				recipients, err := resolver.ResolveRecipients(context.Background(), event)
				if to == delivery.StatusPendingFeePayment {
					if err != nil || len(recipients) != 0 {
						t.Fatalf("unpaid retry must not produce a notification: count=%d err=%v", len(recipients), err)
					}
					return
				}
				if err != nil || len(recipients) != 1 || !recipients[0].AllowWechat {
					t.Fatalf("notifiable transition rejected: count=%d err=%v", len(recipients), err)
				}
				want := labels[to]
				recipient := recipients[0]
				if recipient.Title != want.inApp || recipient.Summary != "请进入订单查看最新进度" {
					t.Fatal("in-app notification was changed")
				}
				snapshot, err := notification.DecodeSubscribeSnapshot(recipient.WechatTemplateSnapshot)
				if err != nil {
					t.Fatal(err)
				}
				if snapshot.Data["phrase3"].Value != want.wechat || snapshot.Data["thing5"].Value != want.inApp ||
					snapshot.Data["character_string1"].Value != notificationTestOrderID || snapshot.Data["time4"].Value != "2026-11-03 09:02" {
					t.Fatalf("incorrect reviewed snapshot: %+v", snapshot.Data)
				}
				if utf8.RuneCountInString(want.wechat) > 5 {
					t.Fatal("phrase exceeds the WeChat limit")
				}
				for _, r := range want.wechat {
					if !unicode.Is(unicode.Han, r) {
						t.Fatal("phrase contains non-Han characters")
					}
				}
				item := notification.DeliveryItem{EventPublicID: event.EventPublicID, RecipientUserID: recipient.UserID, Purpose: recipient.Purpose,
					TemplateKey: "order", TemplateSnapshot: recipient.WechatTemplateSnapshot}
				lookups := 0
				guard := orderSubscribeGuard(&notificationTemplateEventsStub{event: event}, resolver,
					func(context.Context, uint64, string) (string, error) { lookups++; return "test-openid", nil }, "test-app")
				if openid, err := guard(context.Background(), item, snapshot); err != nil || openid != "test-openid" || lookups != 1 {
					t.Fatalf("guard disagreed with snapshot: calls=%d err=%v", lookups, err)
				}
				snapshot.Data["phrase3"] = notification.SubscribeTemplateValue{Value: "已退款"}
				if openid, err := guard(context.Background(), item, snapshot); openid != "" || !errors.Is(err, notification.ErrNotificationIneligible) || lookups != 1 {
					t.Fatal("guard accepted a changed phrase")
				}
			})
		}
	}
}

func TestOrderNotificationPhraseRejectsUnknownTitlesWithoutTruncation(t *testing.T) {
	template := &orderNotificationTemplate{id: "reviewed-order-status-template", fields: map[string]string{"phrase3": "status"}}
	for _, title := range []string{"", "未知", "已接单", "骑手正在前往取货请稍候", "配送费支付失败未知", "收到新私信"} {
		if raw, err := template.snapshot(notification.Event{OccurredAt: time.Now()}, notification.Recipient{Title: title}); len(raw) != 0 || !errors.Is(err, notification.ErrSubscribePayload) {
			t.Fatalf("unknown title %q was accepted or truncated: err=%v", title, err)
		}
	}
}

func TestOrderNotificationPhraseDoesNotExpandEventEligibility(t *testing.T) {
	for _, scenario := range []struct {
		name, eventType string
		to              delivery.Status
		wantError       bool
	}{
		{"pool is not a rider broadcast", delivery.EventRiderPoolOpened, delivery.StatusReadyForClaim, false},
		{"unknown event", "UnknownEvent", delivery.StatusReadyForClaim, true},
		{"private event", notification.EventPrivateMessageReceived, delivery.StatusReadyForClaim, true},
		{"unknown status", delivery.EventOrderStatusChanged, delivery.Status("unknown"), true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			payload, _ := json.Marshal(delivery.OrderNotificationPayload{OrderPublicID: notificationTestOrderID, MerchantUserID: 1,
				From: delivery.StatusPendingFeePayment, To: scenario.to})
			event := notification.Event{EventType: scenario.eventType, AggregateType: "delivery_order", AggregatePublicID: notificationTestOrderID,
				AggregateVersion: 2, PayloadVersion: 1, Payload: payload}
			resolver := &orderNotificationRecipients{orders: notificationOrdersStub{order: delivery.Order{MerchantUserID: 1, Status: scenario.to, RowVersion: 2}},
				users: notificationUsersStub{}, template: &orderNotificationTemplate{id: "reviewed-order-status-template",
					fields: map[string]string{"phrase3": "status"}, users: map[uint64]bool{1: true}}}
			recipients, err := resolver.ResolveRecipients(context.Background(), event)
			if len(recipients) != 0 || (scenario.wantError && !errors.Is(err, notification.ErrInvalidRecord)) || (!scenario.wantError && err != nil) {
				t.Fatalf("unexpected notification eligibility: count=%d err=%v", len(recipients), err)
			}
		})
	}
}

func TestPrivateMessagePhraseRemainsMetadataOnly(t *testing.T) {
	template := &privateMessageNotificationTemplate{id: "reviewed-private-template",
		fields: map[string]string{"phrase4": "status", "thing1": "unreadCount", "time3": "time"}}
	raw, err := template.snapshot(notification.Event{OccurredAt: time.Date(2026, 11, 3, 1, 2, 0, 0, time.UTC)},
		notification.Recipient{Title: "收到新私信", TargetPath: "/pages/secondhand-chat/index?relationId=1"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := notification.DecodeSubscribeSnapshot(raw)
	if err != nil || snapshot.Data["phrase4"].Value != "收到新私信" || snapshot.Data["thing1"].Value != "有新消息" || snapshot.Data["time3"].Value != "2026-11-03 09:02" {
		t.Fatalf("private notification changed: %+v err=%v", snapshot.Data, err)
	}
}
