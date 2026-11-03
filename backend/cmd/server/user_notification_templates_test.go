package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/featureflags"
	"campus-circle/backend/internal/module/im"
	"campus-circle/backend/internal/module/notification"
	"campus-circle/backend/internal/module/user"
)

func TestNotificationTemplateMappingRejectsAmbiguousOrUnsupportedFields(t *testing.T) {
	for _, kind := range []string{"order", "private_message"} {
		for _, test := range []struct{ name, fields string }{
			{"duplicate key", `{"thing1":"status","thing1":"time"}`},
			{"escaped duplicate key", `{"thing1":"status","thi\u006eg1":"time"}`},
			{"unknown field type", `{"unknown1":"status"}`},
			{"missing field number", `{"thing":"status"}`},
			{"field suffix is not a number", `{"thing1evil":"status"}`},
			{"status in time field", `{"time1":"status"}`},
			{"status in number field", `{"number1":"status"}`},
			{"time in date-only field", `{"date1":"time"}`},
			{"unknown semantic", `{"thing1":"messageText"}`},
			{"empty fields", `{}`},
			{"null fields", `null`},
			{"array fields", `[]`},
			{"null semantic", `{"thing1":null}`},
			{"non-string semantic", `{"thing1":true}`},
			{"trailing JSON", `{"thing1":"status"}{}`},
		} {
			t.Run(kind+"/"+test.name, func(t *testing.T) {
				cfg := config.Config{EnableWechatSubscribeNotifications: true, NotificationTestUserIDs: "12"}
				var err error
				if kind == "order" {
					cfg.WechatOrderTemplateID, cfg.WechatOrderTemplateFields = "reviewed-template", test.fields
					_, err = configuredOrderNotificationTemplate(cfg)
				} else {
					cfg.WechatMessageTemplateID, cfg.WechatMessageTemplateFields = "reviewed-template", test.fields
					_, err = configuredPrivateMessageNotificationTemplate(cfg)
				}
				if err == nil {
					t.Fatal("invalid mapping accepted before real provider call")
				}
			})
		}
	}
	cfg := config.Config{EnableWechatSubscribeNotifications: true, WechatMessageTemplateID: "reviewed-template", WechatMessageTemplateFields: `{"number1":"unreadCount"}`, NotificationTestUserIDs: "12"}
	if _, err := configuredPrivateMessageNotificationTemplate(cfg); err == nil {
		t.Fatal("metadata-only unread label is text, not a verified numeric count")
	}
}

func TestNotificationTemplateMappingPreservesSupportedReviewedTypes(t *testing.T) {
	cfg := config.Config{EnableWechatSubscribeNotifications: true, EnableNotificationSending: true,
		WechatOrderTemplateID: "reviewed-order-template", WechatOrderTemplateFields: `{"character_string1":"orderId","thing2":"status","time3":"time"}`,
		WechatMessageTemplateID: "reviewed-private-template", WechatMessageTemplateFields: `{"phrase1":"status","thing2":"unreadCount","time3":"time"}`,
		NotificationTestUserIDs: "12"}
	if _, err := configuredOrderNotificationTemplate(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := configuredPrivateMessageNotificationTemplate(cfg); err != nil {
		t.Fatal(err)
	}
	// A thing field is free-form text and can hold the formatted 16-char time.
	cfg.WechatMessageTemplateFields = `{"thing1":"time"}`
	if _, err := configuredPrivateMessageNotificationTemplate(cfg); err != nil {
		t.Fatal(err)
	}
	cfg.WechatOrderTemplateFields = `{"phrase1":"status"}`
	if _, err := configuredOrderNotificationTemplate(cfg); err != nil {
		t.Fatalf("reviewed short order status must fit the phrase field: %v", err)
	}
}

type notificationTemplateEventsStub struct {
	event notification.Event
	err   error
}

func (s *notificationTemplateEventsStub) FindEvent(context.Context, string) (notification.Event, error) {
	return s.event, s.err
}

type privateTemplateQuietUsers struct{ privateNotificationUsersFake }

func (privateTemplateQuietUsers) Settings(context.Context, uint) (user.SettingsView, error) {
	return user.SettingsView{SocialNotice: true, QuietHours: true}, nil
}

type privateTemplateGuardFixture struct {
	resolver    *privateNotificationRecipients
	events      notificationEventReader
	item        notification.DeliveryItem
	snapshot    notification.TemplateSnapshot
	lookup      notification.OpenIDLookup
	appID       string
	lookupCalls int
}

func newPrivateTemplateGuardFixture(t *testing.T) *privateTemplateGuardFixture {
	t.Helper()
	featureflags.Configure(false)
	t.Cleanup(func() { featureflags.Configure(false) })
	now := time.Now().UTC()
	identity := privateNotificationIdentityFixture()
	relation, err := identity.ResolveC2CRelation(context.Background(), 11, 12)
	if err != nil {
		t.Fatal(err)
	}
	event, err := notification.BuildPrivateMessageEvent(notification.IMCallback{SDKAppID: 1400000000,
		ConversationType: notification.IMCallbackC2C, ProviderMessageID: "provider-private", SenderAccount: "sender",
		RecipientAccount: "recipient", OccurredAt: now}, relation)
	if err != nil {
		t.Fatal(err)
	}
	resolver := &privateNotificationRecipients{identity: identity, users: privateNotificationUsersFake{}, now: func() time.Time { return now },
		template: &privateMessageNotificationTemplate{id: "reviewed-private-template", fields: map[string]string{"thing1": "status", "time2": "time"}, users: map[uint64]bool{12: true}}}
	recipients, err := resolver.ResolveRecipients(context.Background(), event)
	if err != nil || len(recipients) != 1 || !recipients[0].AllowWechat {
		t.Fatalf("fixture recipients=%+v err=%v", recipients, err)
	}
	item := notification.DeliveryItem{EventPublicID: event.EventPublicID, RecipientUserID: 12,
		Purpose: notification.PurposePrivateMessage, TemplateKey: "private_message", TemplateSnapshot: recipients[0].WechatTemplateSnapshot}
	snapshot, err := notification.DecodeSubscribeSnapshot(item.TemplateSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	f := &privateTemplateGuardFixture{resolver: resolver, events: &notificationTemplateEventsStub{event: event}, item: item, snapshot: snapshot, appID: "test-app"}
	f.lookup = func(_ context.Context, id uint64, appID string) (string, error) {
		f.lookupCalls++
		if id != 12 || appID != "test-app" {
			t.Fatal("unexpected identity lookup coordinate")
		}
		return "test-openid", nil
	}
	return f
}

func TestPrivateMessageSubscribeGuardRejectsChangedEligibility(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*privateTemplateGuardFixture)
	}{
		{"missing resolver", func(f *privateTemplateGuardFixture) { f.resolver = nil }},
		{"missing template", func(f *privateTemplateGuardFixture) { f.resolver.template = nil }},
		{"not allowlisted", func(f *privateTemplateGuardFixture) { delete(f.resolver.template.users, 12) }},
		{"wrong purpose", func(f *privateTemplateGuardFixture) { f.item.Purpose = notification.PurposePayment }},
		{"wrong recipient", func(f *privateTemplateGuardFixture) { f.item.RecipientUserID = 13 }},
		{"wrong template key", func(f *privateTemplateGuardFixture) { f.item.TemplateKey = "order" }},
		{"wrong template ID", func(f *privateTemplateGuardFixture) { f.snapshot.TemplateID = "retired-template" }},
		{"changed decoded page", func(f *privateTemplateGuardFixture) { f.snapshot.Page = "/pages/secondhand-chat/index?relationId=99" }},
		{"changed decoded value", func(f *privateTemplateGuardFixture) {
			f.snapshot.Data["thing1"] = notification.SubscribeTemplateValue{Value: "changed"}
		}},
		{"changed persisted snapshot", func(f *privateTemplateGuardFixture) {
			f.item.TemplateSnapshot = json.RawMessage(strings.Replace(string(f.item.TemplateSnapshot), "relationId=4", "relationId=99", 1))
		}},
		{"missing event reader", func(f *privateTemplateGuardFixture) { f.events = nil }},
		{"missing identity reader", func(f *privateTemplateGuardFixture) { f.resolver.identity = nil }},
		{"missing users", func(f *privateTemplateGuardFixture) { f.resolver.users = nil }},
		{"missing OpenID lookup", func(f *privateTemplateGuardFixture) { f.lookup = nil }},
		{"missing app ID", func(f *privateTemplateGuardFixture) { f.appID = "" }},
		{"expired event", func(f *privateTemplateGuardFixture) {
			now := f.resolver.now()
			f.events.(*notificationTemplateEventsStub).event.ExpiresAt = &now
		}},
		{"wrong event identity", func(f *privateTemplateGuardFixture) {
			f.events.(*notificationTemplateEventsStub).event.EventPublicID = "different-event"
		}},
		{"preference opted out", func(f *privateTemplateGuardFixture) { f.resolver.users = privateNotificationUsersFake{optedOut: true} }},
		{"quiet hours", func(f *privateTemplateGuardFixture) {
			f.resolver.users = privateTemplateQuietUsers{}
			now := f.resolver.now()
			f.resolver.now = func() time.Time { return time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 0, 0, time.UTC) }
		}},
		{"blocked sender", func(f *privateTemplateGuardFixture) {
			a := privateNotificationIdentityFixture()
			a.users = privateNotificationUsersFake{blocked: true}
			f.resolver.identity = a
		}},
		{"inactive sender", func(f *privateTemplateGuardFixture) {
			a := privateNotificationIdentityFixture()
			a.users = privateNotificationUsersFake{inactive: 11}
			f.resolver.identity = a
		}},
		{"inactive recipient", func(f *privateTemplateGuardFixture) {
			a := privateNotificationIdentityFixture()
			a.users = privateNotificationUsersFake{inactive: 12}
			f.resolver.identity = a
		}},
		{"relationship changed", func(f *privateTemplateGuardFixture) {
			a := privateNotificationIdentityFixture()
			a.im = imNotificationReaderFake{found: true, relation: im.ConversationContext{RelationID: 99, ReferenceType: "user", BuyerID: 11, SellerID: 12}}
			f.resolver.identity = a
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			f := newPrivateTemplateGuardFixture(t)
			test.mutate(f)
			guard := privateMessageSubscribeGuard(f.events, f.resolver, f.lookup, f.appID)
			openid, err := guard(context.Background(), f.item, f.snapshot)
			if openid != "" || !errors.Is(err, notification.ErrNotificationIneligible) || f.lookupCalls != 0 {
				t.Fatalf("ineligible notification resolved identity: calls=%d err=%v", f.lookupCalls, err)
			}
		})
	}
}

func TestPrivateMessageSubscribeGuardAcceptsEligibleSnapshotAndOptionalClock(t *testing.T) {
	for _, nilClock := range []bool{false, true} {
		f := newPrivateTemplateGuardFixture(t)
		if nilClock {
			f.resolver.now = nil
		}
		guard := privateMessageSubscribeGuard(f.events, f.resolver, f.lookup, f.appID)
		if openid, err := guard(context.Background(), f.item, f.snapshot); err != nil || openid != "test-openid" || f.lookupCalls != 1 {
			t.Fatalf("eligible guard nilClock=%v calls=%d err=%v", nilClock, f.lookupCalls, err)
		}
	}
}

func TestPrivateMessageSubscribeGuardLookupFailuresRemainRetriable(t *testing.T) {
	for _, source := range []string{"event", "relation", "preferences", "openid"} {
		t.Run(source, func(t *testing.T) {
			f := newPrivateTemplateGuardFixture(t)
			failure := errors.New("temporary service outage")
			switch source {
			case "event":
				f.events.(*notificationTemplateEventsStub).err = failure
			case "relation":
				a := privateNotificationIdentityFixture()
				a.im = imNotificationReaderFake{relationErr: failure}
				f.resolver.identity = a
			case "preferences":
				f.resolver.users = privateNotificationUsersFake{settingsErr: failure}
			case "openid":
				f.lookup = func(context.Context, uint64, string) (string, error) { return "", failure }
			}
			guard := privateMessageSubscribeGuard(f.events, f.resolver, f.lookup, f.appID)
			if openid, err := guard(context.Background(), f.item, f.snapshot); openid != "" || err == nil || errors.Is(err, notification.ErrNotificationIneligible) {
				t.Fatalf("transient failure incorrectly acknowledged: %v", err)
			}
		})
	}
}

func TestOrderSubscribeGuardPreservesEligibleDeliveryAndRejectsMismatches(t *testing.T) {
	for _, scenario := range []string{"eligible", "nil clock", "wrong purpose", "wrong event", "expired event", "changed page", "changed state", "missing orders", "missing users", "missing event reader", "missing lookup", "missing app ID"} {
		t.Run(scenario, func(t *testing.T) {
			now := time.Now().UTC()
			expires := now.Add(time.Hour)
			payload, _ := json.Marshal(delivery.OrderNotificationPayload{OrderPublicID: notificationTestOrderID, MerchantUserID: 1, From: delivery.StatusClaimed, To: delivery.StatusToPickup})
			event := notification.Event{EventPublicID: notificationTestOrderID, EventType: delivery.EventOrderStatusChanged, AggregateType: "delivery_order", AggregatePublicID: notificationTestOrderID,
				AggregateVersion: 3, PayloadVersion: 1, Payload: payload, OccurredAt: now, ExpiresAt: &expires}
			resolver := &orderNotificationRecipients{orders: notificationOrdersStub{order: delivery.Order{MerchantUserID: 1, Status: delivery.StatusToPickup, RowVersion: 3}},
				users: notificationUsersStub{}, now: func() time.Time { return now },
				template: &orderNotificationTemplate{id: "reviewed-order-template", fields: map[string]string{"thing1": "status", "time2": "time"}, users: map[uint64]bool{1: true}}}
			recipients, err := resolver.ResolveRecipients(context.Background(), event)
			if err != nil || len(recipients) != 1 || !recipients[0].AllowWechat {
				t.Fatalf("order fixture: recipients=%+v err=%v", recipients, err)
			}
			item := notification.DeliveryItem{EventPublicID: event.EventPublicID, RecipientUserID: 1, Purpose: notification.PurposeDelivery, TemplateKey: "order", TemplateSnapshot: recipients[0].WechatTemplateSnapshot}
			snapshot, err := notification.DecodeSubscribeSnapshot(item.TemplateSnapshot)
			if err != nil {
				t.Fatal(err)
			}
			var events notificationEventReader = &notificationTemplateEventsStub{event: event}
			calls := 0
			var lookup notification.OpenIDLookup = func(context.Context, uint64, string) (string, error) { calls++; return "test-openid", nil }
			appID := "test-app"
			switch scenario {
			case "nil clock":
				resolver.now = nil
			case "wrong purpose":
				item.Purpose = notification.PurposePrivateMessage
			case "wrong event":
				events.(*notificationTemplateEventsStub).event.EventPublicID = "different-event"
			case "expired event":
				events.(*notificationTemplateEventsStub).event.ExpiresAt = &now
			case "changed page":
				snapshot.Page = "/pages/notifications/index"
			case "changed state":
				resolver.orders = notificationOrdersStub{order: delivery.Order{MerchantUserID: 1, Status: delivery.StatusCancelled, RowVersion: 4}}
			case "missing orders":
				resolver.orders = nil
			case "missing users":
				resolver.users = nil
			case "missing event reader":
				events = nil
			case "missing lookup":
				lookup = nil
			case "missing app ID":
				appID = ""
			}
			openid, err := orderSubscribeGuard(events, resolver, lookup, appID)(context.Background(), item, snapshot)
			if scenario == "eligible" || scenario == "nil clock" {
				if err != nil || openid != "test-openid" || calls != 1 {
					t.Fatalf("eligible order rejected: calls=%d err=%v", calls, err)
				}
			} else if !errors.Is(err, notification.ErrNotificationIneligible) || openid != "" || calls != 0 {
				t.Fatalf("ineligible order resolved identity: calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestNotificationSnapshotMatchesRejectsAmbiguousOrExtraFields(t *testing.T) {
	raw := json.RawMessage(`{"templateId":"approved","page":"/pages/notifications/index","data":{"thing1":{"value":"收到新私信"}}}`)
	decoded, err := notification.DecodeSubscribeSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !notificationSnapshotMatches(raw, raw, decoded) {
		t.Fatal("matching snapshot rejected")
	}
	for _, invalid := range []string{
		`{"templateId":"replaced","templateId":"approved","page":"/pages/notifications/index","data":{"thing1":{"value":"收到新私信"}}}`,
		`{"templateId":"approved","page":"/pages/notifications/index","extra":"ignored","data":{"thing1":{"value":"收到新私信"}}}`,
		`{"templateId":"approved","page":"/pages/notifications/index","data":{"thing1":{"value":"收到新私信","color":"red"}}}`,
	} {
		if notificationSnapshotMatches(raw, json.RawMessage(invalid), decoded) {
			t.Fatal("ambiguous or non-reviewed immutable payload accepted")
		}
	}
}
