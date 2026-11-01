package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/notification"
	"campus-circle/backend/internal/module/user"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type userNotificationRuntime struct {
	handler  *notification.Handler
	consumer *notification.Consumer
	wechat   *notification.SubscribeWorker
}

// A disabled rollout still has stable authenticated API shapes, but never
// queries notification tables or writes authorization records.
type disabledNotificationInbox struct{}

func (disabledNotificationInbox) ListInbox(context.Context, uint64, string, int) ([]notification.InboxItem, int64, error) {
	return []notification.InboxItem{}, 0, nil
}
func (disabledNotificationInbox) MarkInboxRead(context.Context, uint64, string, time.Time) (bool, error) {
	return false, nil
}

func notificationSchemaReady(db *gorm.DB) error {
	if db == nil {
		return errors.New("notification database unavailable")
	}
	for _, model := range []any{&notification.Event{}, &notification.InboxItem{}, &notification.DeliveryItem{}, &notification.Subscription{}, &notification.CallbackReceipt{}} {
		if !db.Migrator().HasTable(model) {
			return errors.New("user notifications require migration 053")
		}
	}
	return nil
}

func newDeliveryNotificationAppender(cfg config.Config, db *gorm.DB) (delivery.NotificationEventAppender, error) {
	if !cfg.EnableUserNotifications {
		return nil, nil
	}
	if err := notificationSchemaReady(db); err != nil {
		return nil, err
	}
	repo := notification.NewRepository(db)
	return func(tx *gorm.DB, fact delivery.NotificationEvent) error {
		return repo.AppendEventTx(tx, deliveryNotificationFact(fact))
	}, nil
}

func deliveryNotificationFact(fact delivery.NotificationEvent) notification.Event {
	ttl := 24 * time.Hour
	if fact.EventType == delivery.EventRiderPoolOpened {
		ttl = 10 * time.Minute
	}
	expires := fact.OccurredAt.Add(ttl)
	return notification.Event{EventPublicID: fact.EventPublicID, EventType: fact.EventType,
		AggregateType: "delivery_order", AggregatePublicID: fact.AggregatePublicID,
		AggregateVersion: fact.AggregateVersion, PayloadVersion: 1,
		Payload: append(json.RawMessage(nil), fact.Payload...), Status: notification.EventPending,
		RowVersion: 1, OccurredAt: fact.OccurredAt, ExpiresAt: &expires,
		CreatedAt: fact.OccurredAt, UpdatedAt: fact.OccurredAt}
}

func newUserNotificationRuntime(cfg config.Config, db *gorm.DB, users *user.Service) (*userNotificationRuntime, error) {
	var inbox notification.InboxRepository = disabledNotificationInbox{}
	var authorization notification.SubscriptionAuthorizationRepository
	var lookup notification.OpenIDLookup
	var consumer *notification.Consumer
	var wechat *notification.SubscribeWorker
	var templates = map[string]string{"order": "", "private_message": ""}
	if cfg.EnableUserNotifications {
		if users == nil {
			return nil, errors.New("notification identity service unavailable")
		}
		if err := notificationSchemaReady(db); err != nil {
			return nil, err
		}
		repo := notification.NewRepository(db)
		inbox, authorization = repo, repo
		lookup = func(ctx context.Context, id uint64, appID string) (string, error) {
			if appID == "" || appID != cfg.WechatAppID || uint64(uint(id)) != id {
				return "", notification.ErrSubscriptionEmpty
			}
			account, err := users.Get(ctx, uint(id))
			if err != nil || account.Status != user.StatusActive || account.ClosedAt != nil || account.DeletedAt.Valid {
				return "", notification.ErrSubscriptionEmpty
			}
			return account.OpenID, nil
		}
		template, err := configuredOrderNotificationTemplate(cfg)
		if err != nil {
			return nil, err
		}
		if template != nil {
			templates["order"] = template.id
		}
		resolver := &orderNotificationRecipients{orders: delivery.NewRepository(db), users: users, now: time.Now, template: template}
		consumer = notification.NewConsumer(notification.NewExpander(notification.NewEventRepository(db), resolver))
		if cfg.EnableWechatSubscribeNotifications && cfg.EnableNotificationSending {
			sender, err := notification.NewSubscribeSender(cfg)
			if err != nil {
				return nil, err
			}
			wechat = notification.NewSubscribeWorker(repo, sender, orderSubscribeGuard(repo, resolver, lookup, cfg.WechatAppID), cfg.WechatAppID)
		}
	}
	service := notification.NewSubscriptionService(authorization, lookup, notification.SubscriptionConfig{
		Enabled:   cfg.EnableUserNotifications && cfg.EnableWechatSubscribeNotifications,
		AppID:     cfg.WechatAppID,
		Templates: templates,
	})
	handler := notification.NewHandler(inbox, service, func(c *gin.Context) uint64 { return uint64(user.CurrentUserID(c)) })
	return &userNotificationRuntime{handler: handler, consumer: consumer, wechat: wechat}, nil
}

type notificationOrderReader interface {
	Find(context.Context, string) (delivery.Order, error)
}
type notificationUserReader interface {
	Get(context.Context, uint) (user.User, error)
	Settings(context.Context, uint) (user.SettingsView, error)
}
type orderNotificationRecipients struct {
	orders   notificationOrderReader
	users    notificationUserReader
	now      func() time.Time
	template *orderNotificationTemplate
}

func (r *orderNotificationRecipients) ResolveRecipients(ctx context.Context, event notification.Event) ([]notification.Recipient, error) {
	if event.AggregateType != "delivery_order" || event.PayloadVersion != 1 {
		return nil, notification.ErrInvalidRecord
	}
	// There is no reliable on-duty/service-range/capacity fact in the current
	// rider model. Never reinterpret an approved identity as an available rider.
	if event.EventType == delivery.EventRiderPoolOpened {
		return nil, nil
	}
	if event.EventType != delivery.EventOrderStatusChanged {
		return nil, notification.ErrInvalidRecord
	}
	var fact delivery.OrderNotificationPayload
	if err := json.Unmarshal(event.Payload, &fact); err != nil || fact.OrderPublicID != event.AggregatePublicID || fact.MerchantUserID == 0 || !delivery.CanTransition(fact.From, fact.To) {
		return nil, notification.ErrInvalidRecord
	}
	current, err := r.orders.Find(ctx, event.AggregatePublicID)
	if errors.Is(err, delivery.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("notification order lookup failed")
	}
	if current.MerchantUserID != fact.MerchantUserID || current.RowVersion < event.AggregateVersion {
		return nil, notification.ErrInvalidRecord
	}
	// A delayed state notification must not announce a now-obsolete state.
	if current.Status != fact.To {
		return nil, nil
	}
	label, ok := map[delivery.Status]string{
		delivery.StatusPaymentFailed: "配送费支付失败", delivery.StatusReadyForClaim: "配送单等待接单",
		delivery.StatusClaimed: "骑手已接单", delivery.StatusToPickup: "骑手正在前往取货",
		delivery.StatusPickedUp: "骑手已取货", delivery.StatusDelivering: "配送中",
		delivery.StatusDelivered: "配送已完成", delivery.StatusCancelled: "配送单已取消",
	}[fact.To]
	if !ok {
		return nil, nil
	}
	purpose := notification.PurposeDelivery
	if fact.From == delivery.StatusPendingFeePayment || fact.From == delivery.StatusPaymentFailed {
		purpose = notification.PurposePayment
	}
	ids := []uint64{fact.MerchantUserID}
	if fact.RiderUserID != nil && current.RiderUserID != nil && *fact.RiderUserID == *current.RiderUserID {
		ids = append(ids, *fact.RiderUserID)
	}
	if fact.PreviousRiderUserID != nil {
		ids = append(ids, *fact.PreviousRiderUserID)
	}
	seen := map[uint64]bool{}
	result := make([]notification.Recipient, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] || uint64(uint(id)) != id {
			continue
		}
		seen[id] = true
		account, err := r.users.Get(ctx, uint(id))
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return nil, errors.New("notification account lookup failed")
		}
		if account.Status != user.StatusActive || account.ClosedAt != nil || account.DeletedAt.Valid {
			continue
		}
		settings, err := r.users.Settings(ctx, uint(id))
		if err != nil {
			return nil, errors.New("notification preference lookup failed")
		}
		if !settings.OrderNotice {
			continue
		}
		path := "/pages/delivery-rider/index"
		if id == fact.MerchantUserID {
			path = "/pages/merchant-order-detail/index?orderId=" + fact.OrderPublicID
		}
		recipient := notification.Recipient{UserID: id, Purpose: purpose, Title: label,
			Summary: "请进入订单查看最新进度", AggregateType: event.AggregateType, AggregatePublicID: event.AggregatePublicID,
			AggregateVersion: event.AggregateVersion, TargetPath: path}
		if r.template != nil && r.template.users[id] {
			now := time.Now()
			if r.now != nil {
				now = r.now()
			}
			hour := now.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Hour()
			if !settings.QuietHours || (hour >= 7 && hour < 23) {
				snapshot, err := r.template.snapshot(event, recipient)
				if err != nil {
					return nil, err
				}
				recipient.AllowWechat = true
				recipient.WechatTemplateKey = "order"
				recipient.WechatTemplateSnapshot = snapshot
			}
		}
		result = append(result, recipient)
	}
	return result, nil
}
