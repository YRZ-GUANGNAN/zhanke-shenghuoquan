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
	const miniAppID = "wx0123456789abcdef"
	format := deliveryWecomGroupFormatter(nil, func() bool { return true }, func() bool { return true }, miniAppID)
	groups, err := format(event)
	if err != nil || len(groups) != 2 || groups[0].GroupKey != "merchant_ops" || groups[1].GroupKey != "rider_ops" {
		t.Fatalf("routing groups=%+v err=%v", groups, err)
	}
	for _, group := range groups {
		if strings.Contains(group.Summary, "13800000000") || strings.Contains(group.Summary, "Dorm") ||
			!strings.Contains(group.Title, "骑手正在前往取货") {
			t.Fatalf("unsafe or incorrect group message: %+v", group)
		}
		if group.Card == nil || group.Card.ActionAppID != miniAppID || group.Card.ActionURL != wecomOrderActionURL {
			t.Fatalf("group card must carry miniapp jump: %+v", group.Card)
		}
		wantPage := wecomMerchantOrderPagePath
		if group.GroupKey == "rider_ops" {
			wantPage = wecomRiderOrderPagePath
		}
		if group.Card.ActionPagePath != wantPage {
			t.Fatalf("group %s pagepath=%q want %q", group.GroupKey, group.Card.ActionPagePath, wantPage)
		}
		// 卡片层级对齐京东/美团式：标题即状态，副标题带平台与短单号，
		// 条目不再重复状态，只留单号/起点/终点/时间。
		wantEmphasis := "配送费"
		if group.GroupKey == "rider_ops" {
			wantEmphasis = "赏金"
		}
		if group.Card.Title != "骑手正在前往取货" ||
			group.Card.Desc != "小程序独立订单 · 单号 01ARZ3…5FAV" ||
			group.Card.EmphasisDesc != wantEmphasis {
			t.Fatalf("card head=%+v", group.Card)
		}
		wantKeys := []string{"单号", "起点", "终点", "时间"}
		if len(group.Card.Items) != len(wantKeys) {
			t.Fatalf("card items=%+v", group.Card.Items)
		}
		for index, key := range wantKeys {
			if group.Card.Items[index].Key != key {
				t.Fatalf("card item[%d]=%+v want key %q", index, group.Card.Items[index], key)
			}
		}
		// 完整单号必须保留在条目里（副标题只放短号），值班人员需要复制回后台检索。
		if group.Card.Items[0].Value != notificationTestOrderID {
			t.Fatalf("card item 单号=%q want full id %q", group.Card.Items[0].Value, notificationTestOrderID)
		}
	}
	merchantOnly, err := deliveryWecomGroupFormatter(nil, func() bool { return true }, func() bool { return false }, "")(event)
	if err != nil || len(merchantOnly) != 1 || merchantOnly[0].GroupKey != "merchant_ops" {
		t.Fatalf("merchant-only routing=%+v err=%v", merchantOnly, err)
	}
}

func TestShortOrderID(t *testing.T) {
	cases := map[string]string{
		"01ARZ3NDEKTSV4RRFFQ69G5FAV": "01ARZ3…5FAV",
		"SHORT-ID":                   "SHORT-ID",
		"":                           "",
	}
	for input, want := range cases {
		if got := shortOrderID(input); got != want {
			t.Fatalf("shortOrderID(%q)=%q want %q", input, got, want)
		}
	}
}

func TestDeliveryWecomFormatterSendsReassignCards(t *testing.T) {
	now := time.Date(2026, 11, 18, 4, 30, 0, 0, time.UTC)
	previous, next := uint64(2), uint64(3)
	payload, _ := json.Marshal(delivery.OrderNotificationPayload{
		OrderPublicID: notificationTestOrderID, MerchantUserID: 1,
		RiderUserID: &next, PreviousRiderUserID: &previous,
		From: delivery.StatusDelivering, To: delivery.StatusDelivering,
		DeliveryFeeCents: 400, RiderFeeCents: 350, SourcePlatform: "meituan",
		PickupAddress: "东门菜鸟驿站", DeliveryAddress: "12栋305",
	})
	event := notification.Event{EventType: delivery.EventOrderReassigned, AggregateType: "delivery_order",
		AggregatePublicID: notificationTestOrderID, AggregateVersion: 6, PayloadVersion: 1, Payload: payload, OccurredAt: now}
	groups, err := deliveryWecomGroupFormatter(nil, func() bool { return true }, func() bool { return true }, "")(event)
	if err != nil || len(groups) != 2 {
		t.Fatalf("reassign routing groups=%+v err=%v", groups, err)
	}
	for _, group := range groups {
		// 改派卡与状态卡同一套层级：标题=动作，副标题=平台+短单号，条目不重复状态。
		if group.Card == nil || group.Card.Title != "订单已改派" ||
			group.Card.Desc != "美团订单 · 单号 01ARZ3…5FAV" {
			t.Fatalf("reassign card head: %+v", group.Card)
		}
		if len(group.Card.Items) != 4 || group.Card.Items[1].Value != "东门菜鸟驿站" ||
			group.Card.Items[2].Value != "12栋305" {
			t.Fatalf("reassign card items: %+v", group.Card.Items)
		}
	}
	// 状态变化、骑手未变或缺骑手都不是合法改派，必须拒收。
	for _, mutate := range []func(*delivery.OrderNotificationPayload){
		func(f *delivery.OrderNotificationPayload) { f.To = delivery.StatusPickedUp },
		func(f *delivery.OrderNotificationPayload) { f.PreviousRiderUserID = &next },
		func(f *delivery.OrderNotificationPayload) { f.RiderUserID = nil },
	} {
		bad := delivery.OrderNotificationPayload{
			OrderPublicID: notificationTestOrderID, MerchantUserID: 1,
			RiderUserID: &next, PreviousRiderUserID: &previous,
			From: delivery.StatusDelivering, To: delivery.StatusDelivering,
		}
		mutate(&bad)
		badPayload, _ := json.Marshal(bad)
		badEvent := event
		badEvent.Payload = badPayload
		if result, err := deliveryWecomGroupFormatter(nil, func() bool { return true }, func() bool { return true }, "")(badEvent); err == nil || len(result) != 0 {
			t.Fatalf("invalid reassign payload accepted: %+v err=%v", result, err)
		}
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
		if groups, err := deliveryWecomGroupFormatter(nil, func() bool { return true }, func() bool { return true }, "")(event); err == nil || len(groups) != 0 {
			t.Fatalf("invalid payload accepted: groups=%+v err=%v", groups, err)
		}
	}
}
