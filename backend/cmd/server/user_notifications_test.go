package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/notification"
	"campus-circle/backend/internal/module/user"
	"github.com/gin-gonic/gin"
)

const notificationTestOrderID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

type notificationOrdersStub struct{ order delivery.Order }

func (s notificationOrdersStub) Find(context.Context, string) (delivery.Order, error) {
	return s.order, nil
}

type notificationUsersStub struct {
	inactive uint64
	optedOut uint64
}

func (s notificationUsersStub) Get(_ context.Context, id uint) (user.User, error) {
	status := user.StatusActive
	if uint64(id) == s.inactive {
		status = user.StatusClosed
	}
	return user.User{ID: id, Status: status}, nil
}
func (s notificationUsersStub) Settings(_ context.Context, id uint) (user.SettingsView, error) {
	return user.SettingsView{OrderNotice: uint64(id) != s.optedOut}, nil
}

func TestOrderNotificationRecipientsAcceptReassign(t *testing.T) {
	previous, next := uint64(2), uint64(3)
	fact := delivery.OrderNotificationPayload{OrderPublicID: notificationTestOrderID, MerchantUserID: 1,
		RiderUserID: &next, PreviousRiderUserID: &previous,
		From: delivery.StatusDelivering, To: delivery.StatusDelivering}
	payload, _ := json.Marshal(fact)
	event := notification.Event{EventType: delivery.EventOrderReassigned, AggregateType: "delivery_order",
		AggregatePublicID: notificationTestOrderID, AggregateVersion: 6, PayloadVersion: 1, Payload: payload}
	order := delivery.Order{MerchantUserID: 1, RiderUserID: &next, Status: delivery.StatusDelivering, RowVersion: 6}
	resolver := orderNotificationRecipients{orders: notificationOrdersStub{order}, users: notificationUsersStub{},
		template: &orderNotificationTemplate{id: "reviewed-order-status-template",
			fields: map[string]string{"phrase3": "status", "thing5": "status"},
			users:  map[uint64]bool{1: true, 2: true, 3: true}}}
	recipients, err := resolver.ResolveRecipients(context.Background(), event)
	if err != nil || len(recipients) != 3 {
		t.Fatalf("reassign recipients len=%d err=%v", len(recipients), err)
	}
	for _, recipient := range recipients {
		if recipient.Title != "订单已改派" {
			t.Fatalf("reassign title=%q", recipient.Title)
		}
		snapshot, err := notification.DecodeSubscribeSnapshot(recipient.WechatTemplateSnapshot)
		if err != nil || snapshot.Data["phrase3"].Value != "已改派" || snapshot.Data["thing5"].Value != "订单已改派" {
			t.Fatalf("reassign snapshot: %+v err=%v", snapshot.Data, err)
		}
	}
	// 又被改派（当前骑手已不是事件里的新骑手）→ 旧事件静默跳过。
	other := uint64(4)
	order.RiderUserID = &other
	resolver.orders = notificationOrdersStub{order}
	if got, err := resolver.ResolveRecipients(context.Background(), event); err != nil || len(got) != 0 {
		t.Fatal("stale reassign event was emitted")
	}
	// 订单只是推进了状态（骑手没变）→ 改派通知仍然有效。
	advanced := delivery.Order{MerchantUserID: 1, RiderUserID: &next, Status: delivery.StatusToPickup, RowVersion: 7}
	resolver.orders = notificationOrdersStub{advanced}
	if got, err := resolver.ResolveRecipients(context.Background(), event); err != nil || len(got) != 3 {
		t.Fatalf("reassign after state advance must still notify: len=%d err=%v", len(got), err)
	}
	// 订单已终结（取消/送达）→ 改派通知失去意义，跳过。
	advanced.Status, advanced.RowVersion = delivery.StatusCancelled, 8
	resolver.orders = notificationOrdersStub{advanced}
	if got, err := resolver.ResolveRecipients(context.Background(), event); err != nil || len(got) != 0 {
		t.Fatal("reassign on terminal order was emitted")
	}
	// From!=To、骑手未变或缺骑手都属于非法载荷，必须拒绝。
	for _, mutate := range []func(*delivery.OrderNotificationPayload){
		func(f *delivery.OrderNotificationPayload) { f.To = delivery.StatusPickedUp },
		func(f *delivery.OrderNotificationPayload) { f.PreviousRiderUserID = &next },
		func(f *delivery.OrderNotificationPayload) { f.RiderUserID = nil },
	} {
		bad := fact
		mutate(&bad)
		badPayload, _ := json.Marshal(bad)
		badEvent := event
		badEvent.Payload = badPayload
		if got, err := resolver.ResolveRecipients(context.Background(), badEvent); err == nil || len(got) != 0 {
			t.Fatal("invalid reassign payload was accepted")
		}
	}
}

func TestNotificationRolloutDisabledDoesNotRequireDatabase(t *testing.T) {
	appender, err := newDeliveryNotificationAppender(config.Config{}, nil)
	if err != nil || appender != nil {
		t.Fatal("disabled notification appender accessed database")
	}
	runtime, err := newUserNotificationRuntime(config.Config{}, nil, nil)
	if err != nil || runtime.consumer != nil {
		t.Fatal("disabled notifications spawned worker")
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	runtime.handler.Register(router.Group(""))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/notification/inbox", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated inbox: %d", response.Code)
	}
	if _, err := newDeliveryNotificationAppender(config.Config{EnableUserNotifications: true}, nil); err == nil {
		t.Fatal("enabled notifications bypassed migration preflight")
	}
}

func TestNotificationAdapterPreservesFactAndExpiresPool(t *testing.T) {
	now := time.Date(2026, 11, 1, 1, 0, 0, 0, time.UTC)
	source := delivery.NotificationEvent{EventPublicID: notificationTestOrderID, EventType: delivery.EventRiderPoolOpened,
		AggregatePublicID: notificationTestOrderID, AggregateVersion: 2, Payload: json.RawMessage(`{"orderPublicId":"` + notificationTestOrderID + `"}`), OccurredAt: now}
	event := deliveryNotificationFact(source)
	if event.AggregateType != "delivery_order" || event.RowVersion != 1 || event.PayloadVersion != 1 || event.CreatedAt != now || event.Status != notification.EventPending || event.ExpiresAt.Sub(now) != 10*time.Minute {
		t.Fatal("incomplete persisted notification fact")
	}
	source.Payload[0] = '['
	if event.Payload[0] != '{' {
		t.Fatal("mutable event payload alias")
	}
}

func TestOrderNotificationRecipientsGuardCurrentFactAndPreferences(t *testing.T) {
	rider, previous := uint64(2), uint64(3)
	fact := delivery.OrderNotificationPayload{OrderPublicID: notificationTestOrderID, MerchantUserID: 1, RiderUserID: &rider, PreviousRiderUserID: &previous, From: delivery.StatusPickedUp, To: delivery.StatusDelivering}
	payload, _ := json.Marshal(fact)
	event := notification.Event{EventType: delivery.EventOrderStatusChanged, AggregateType: "delivery_order", AggregatePublicID: notificationTestOrderID, AggregateVersion: 5, PayloadVersion: 1, Payload: payload}
	order := delivery.Order{MerchantUserID: 1, RiderUserID: &rider, Status: delivery.StatusDelivering, RowVersion: 5}
	resolver := orderNotificationRecipients{orders: notificationOrdersStub{order}, users: notificationUsersStub{inactive: 3}}
	recipients, err := resolver.ResolveRecipients(context.Background(), event)
	if err != nil || len(recipients) != 2 {
		t.Fatalf("recipient selection: len=%d err=%v", len(recipients), err)
	}
	for _, r := range recipients {
		if r.AllowWechat || strings.Contains(r.Summary, "到账") {
			t.Fatal("manufactured provider eligibility or financial result")
		}
		if r.UserID == 1 && r.TargetPath != "/pages/merchant-order-detail/index?orderId="+notificationTestOrderID {
			t.Fatal("incorrect protected merchant route")
		}
	}
	resolver.users = notificationUsersStub{inactive: 3, optedOut: 2}
	recipients, err = resolver.ResolveRecipients(context.Background(), event)
	if err != nil || len(recipients) != 1 || recipients[0].UserID != 1 {
		t.Fatal("ignored notification preference")
	}
	order.Status = delivery.StatusCancelled
	resolver.orders = notificationOrdersStub{order}
	if got, err := resolver.ResolveRecipients(context.Background(), event); err != nil || len(got) != 0 {
		t.Fatal("obsolete order notification was emitted")
	}
	event.EventType = delivery.EventRiderPoolOpened
	if got, err := resolver.ResolveRecipients(context.Background(), event); err != nil || len(got) != 0 {
		t.Fatal("rider qualification was treated as on-duty permission")
	}
}
