package notification

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type wecomFormatterEventStore struct{}

func (*wecomFormatterEventStore) ListPendingEvents(context.Context, time.Time, int) ([]Event, error) {
	return nil, nil
}

func TestExpanderBuildsIndependentWecomGroupDeliveries(t *testing.T) {
	now := time.Date(2026, 11, 13, 2, 0, 0, 0, time.UTC)
	event := Event{
		EventPublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", EventType: "DeliveryOrderStatusChanged",
		AggregateType: "delivery_order", AggregatePublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAW",
		AggregateVersion: 2, PayloadVersion: 1, Payload: json.RawMessage(`{}`),
		OccurredAt: now, ExpiresAt: ptrTime(now.Add(time.Hour)),
	}
	expander := NewExpander(nil, nil, WithWecomGroupFormatter(func(Event) ([]WecomGroupMessage, error) {
		return []WecomGroupMessage{
			{GroupKey: "merchant_ops", Title: "商家订单 · 待接单", Summary: "配送单等待接单"},
			{GroupKey: "rider_ops", Title: "骑手调度 · 待接单", Summary: "配送单等待接单"},
		}, nil
	}))
	inbox, deliveries, err := expander.build(event, nil, now)
	if err != nil || len(inbox) != 0 || len(deliveries) != 2 {
		t.Fatalf("group expansion inbox=%d deliveries=%d err=%v", len(inbox), len(deliveries), err)
	}
	if deliveries[0].Channel != ChannelWecom || deliveries[1].Channel != ChannelWecom ||
		deliveries[0].RecipientUserID == deliveries[1].RecipientUserID {
		t.Fatalf("group deliveries share an invalid identity: %+v", deliveries)
	}
	for _, delivery := range deliveries {
		if _, err := DecodeWecomGroupSnapshot(delivery, delivery.TemplateSnapshot); err != nil {
			t.Fatal(err)
		}
	}
}
