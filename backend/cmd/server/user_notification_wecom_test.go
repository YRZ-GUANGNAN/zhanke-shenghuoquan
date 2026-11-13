package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/notification"
)

func TestDeliveryWecomFormatterRoutesOrderMetadataByGroup(t *testing.T) {
	now := time.Date(2026, 11, 13, 1, 2, 0, 0, time.UTC)
	payload, _ := json.Marshal(delivery.OrderNotificationPayload{
		OrderPublicID: notificationTestOrderID, MerchantUserID: 1,
		From: delivery.StatusClaimed, To: delivery.StatusToPickup,
	})
	event := notification.Event{
		EventPublicID: notificationTestOrderID, EventType: delivery.EventOrderStatusChanged,
		AggregateType: "delivery_order", AggregatePublicID: notificationTestOrderID,
		AggregateVersion: 3, PayloadVersion: 1, Payload: payload, OccurredAt: now,
	}
	format := deliveryWecomGroupFormatter(true, true)
	groups, err := format(event)
	if err != nil || len(groups) != 2 || groups[0].GroupKey != "merchant_ops" || groups[1].GroupKey != "rider_ops" {
		t.Fatalf("routing groups=%+v err=%v", groups, err)
	}
	for _, group := range groups {
		if strings.Contains(group.Summary, "13800000000") || strings.Contains(group.Summary, "Dorm") ||
			!strings.Contains(group.Title, "骑手正在前往取货") {
			t.Fatalf("unsafe or incorrect group message: %+v", group)
		}
	}
	merchantOnly, err := deliveryWecomGroupFormatter(true, false)(event)
	if err != nil || len(merchantOnly) != 1 || merchantOnly[0].GroupKey != "merchant_ops" {
		t.Fatalf("merchant-only routing=%+v err=%v", merchantOnly, err)
	}
}

func TestDeliveryWecomFormatterRejectsUnknownPayloads(t *testing.T) {
	now := time.Date(2026, 11, 13, 1, 2, 0, 0, time.UTC)
	valid, _ := json.Marshal(delivery.OrderNotificationPayload{
		OrderPublicID: notificationTestOrderID, MerchantUserID: 1,
		From: delivery.StatusClaimed, To: delivery.StatusToPickup,
	})
	for _, payload := range [][]byte{
		append(valid, []byte(`{"extra":true}`)...),
		[]byte(`{"orderPublicId":"wrong","merchantUserId":1,"from":"claimed","to":"to_pickup"}`),
		[]byte(`not-json`),
	} {
		event := notification.Event{EventType: delivery.EventOrderStatusChanged, AggregateType: "delivery_order",
			AggregatePublicID: notificationTestOrderID, PayloadVersion: 1, Payload: payload, OccurredAt: now}
		if groups, err := deliveryWecomGroupFormatter(true, true)(event); err == nil || len(groups) != 0 {
			t.Fatalf("invalid payload accepted: groups=%+v err=%v", groups, err)
		}
	}
}
