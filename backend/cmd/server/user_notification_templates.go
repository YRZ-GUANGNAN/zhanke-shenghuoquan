package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/notification"
)

type orderNotificationTemplate struct {
	id     string
	fields map[string]string
	users  map[uint64]bool
}

// privateMessageNotificationTemplate is deliberately metadata-only. The
// WeChat subscription message never contains message text, phone numbers or
// provider identifiers; it only points the user back to the conversation.
type privateMessageNotificationTemplate struct {
	id     string
	fields map[string]string
	users  map[uint64]bool
}

var notificationTemplateField = regexp.MustCompile(`^(thing|phrase|time|character_string)[1-9][0-9]*$`)

// Only map fields whose reviewed WeChat type can represent the value we
// actually generate. In particular unreadCount is a metadata-only label, not
// an authoritative numeric unread count. Reject duplicate JSON keys instead
// of silently replacing one reviewed mapping with another.
func decodeNotificationTemplateFields(raw string, private bool) (map[string]string, error) {
	invalid := errors.New("unsupported or ambiguous notification template field mapping")
	if len(raw) > 16384 {
		return nil, invalid
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	if start, err := decoder.Token(); err != nil || start != json.Delim('{') {
		return nil, invalid
	}
	fields := map[string]string{}
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || len(key) > 32 || fields[key] != "" {
			return nil, invalid
		}
		match := notificationTemplateField.FindStringSubmatch(key)
		var semantic string
		if len(match) != 2 || decoder.Decode(&semantic) != nil {
			return nil, invalid
		}
		compatible := false
		switch semantic {
		case "status":
			compatible = match[1] == "thing" || match[1] == "phrase"
		case "time":
			compatible = match[1] == "time" || match[1] == "thing"
		case "orderId":
			compatible = !private && match[1] == "character_string"
		case "unreadCount":
			compatible = private && (match[1] == "thing" || match[1] == "phrase")
		}
		if !compatible || len(fields) >= 20 {
			return nil, invalid
		}
		fields[key] = semantic
	}
	if end, err := decoder.Token(); err != nil || end != json.Delim('}') || len(fields) == 0 {
		return nil, invalid
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, invalid
	}
	return fields, nil
}

func configuredPrivateMessageNotificationTemplate(cfg config.Config) (*privateMessageNotificationTemplate, error) {
	if !cfg.EnableWechatSubscribeNotifications || strings.TrimSpace(cfg.WechatMessageTemplateID) == "" && strings.TrimSpace(cfg.WechatMessageTemplateFields) == "" {
		return nil, nil
	}
	template := &privateMessageNotificationTemplate{id: strings.TrimSpace(cfg.WechatMessageTemplateID), users: map[uint64]bool{}}
	fields, err := decodeNotificationTemplateFields(cfg.WechatMessageTemplateFields, true)
	if template.id == "" || err != nil {
		return nil, errors.New("private message template requires approved ID and field mapping")
	}
	template.fields = fields
	for _, raw := range strings.Split(cfg.NotificationTestUserIDs, ",") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		id, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
		if err != nil || id == 0 {
			return nil, errors.New("invalid notification test recipient allowlist")
		}
		template.users[id] = true
	}
	if cfg.EnableNotificationSending && len(template.users) == 0 {
		return nil, errors.New("external notification test allowlist is empty")
	}
	_, err = template.snapshot(notification.Event{AggregatePublicID: "im-pair:1:2", OccurredAt: time.Now()}, notification.Recipient{Title: "收到新私信", TargetPath: "/pages/secondhand-chat/index?relationId=1"})
	if err != nil {
		return nil, errors.New("private message template fields do not fit approved field types")
	}
	return template, nil
}

func (t *privateMessageNotificationTemplate) snapshot(event notification.Event, recipient notification.Recipient) (json.RawMessage, error) {
	if t == nil {
		return nil, notification.ErrSubscribePayload
	}
	data := map[string]notification.SubscribeTemplateValue{}
	for field, semantic := range t.fields {
		value := ""
		switch semantic {
		case "status":
			value = recipient.Title
		case "time":
			value = event.OccurredAt.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02 15:04")
		case "unreadCount":
			value = "有新消息"
		}
		data[field] = notification.SubscribeTemplateValue{Value: value}
	}
	raw, err := json.Marshal(notification.TemplateSnapshot{TemplateID: t.id, Page: recipient.TargetPath, Data: data})
	if err != nil {
		return nil, err
	}
	if _, err := notification.DecodeSubscribeSnapshot(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func configuredOrderNotificationTemplate(cfg config.Config) (*orderNotificationTemplate, error) {
	if !cfg.EnableWechatSubscribeNotifications {
		return nil, nil
	}
	template := &orderNotificationTemplate{id: strings.TrimSpace(cfg.WechatOrderTemplateID), users: map[uint64]bool{}}
	fields, err := decodeNotificationTemplateFields(cfg.WechatOrderTemplateFields, false)
	if template.id == "" || err != nil {
		return nil, errors.New("notification order template requires approved ID and field mapping")
	}
	template.fields = fields
	for _, raw := range strings.Split(cfg.NotificationTestUserIDs, ",") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		id, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
		if err != nil || id == 0 {
			return nil, errors.New("invalid notification test recipient allowlist")
		}
		template.users[id] = true
	}
	if cfg.EnableNotificationSending && len(template.users) == 0 {
		return nil, errors.New("external notification test allowlist is empty")
	}
	// Validate the longest in-app title and public ID against the reviewed
	// mapping. Phrase fields use explicit short status labels, never truncation.
	_, err = template.snapshot(notification.Event{AggregatePublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", OccurredAt: time.Now()}, notification.Recipient{Title: "骑手正在前往取货", TargetPath: "/pages/delivery-rider/index"})
	if err != nil {
		return nil, errors.New("notification template fields do not fit approved field types")
	}
	return template, nil
}

func (t *orderNotificationTemplate) snapshot(event notification.Event, recipient notification.Recipient) (json.RawMessage, error) {
	if t == nil {
		return nil, notification.ErrSubscribePayload
	}
	data := map[string]notification.SubscribeTemplateValue{}
	for field, semantic := range t.fields {
		value := ""
		switch semantic {
		case "orderId":
			value = event.AggregatePublicID
		case "status":
			value = recipient.Title
			if strings.HasPrefix(field, "phrase") {
				var err error
				value, err = orderNotificationStatusPhrase(recipient.Title)
				if err != nil {
					return nil, err
				}
			}
		case "time":
			value = event.OccurredAt.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02 15:04")
		}
		data[field] = notification.SubscribeTemplateValue{Value: value}
	}
	raw, err := json.Marshal(notification.TemplateSnapshot{TemplateID: t.id, Page: recipient.TargetPath, Data: data})
	if err != nil {
		return nil, err
	}
	if _, err := notification.DecodeSubscribeSnapshot(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// The order resolver owns the validated state and its in-app title. Only the
// reviewed WeChat phrase field needs shorter wording; keep thing fields and
// private-message templates unchanged. Unknown titles fail closed even when
// they would fit, so a future status cannot silently become an external notice.
func orderNotificationStatusPhrase(title string) (string, error) {
	switch title {
	case "配送费支付失败":
		return "支付失败", nil
	case "配送单等待接单", "有新的校园配送单":
		return "待接单", nil
	case "骑手已接单":
		return "已接单", nil
	case "骑手正在前往取货":
		return "取货中", nil
	case "骑手已取货":
		return "已取货", nil
	case "配送中":
		return "配送中", nil
	case "配送已完成":
		return "已送达", nil
	case "配送单已取消":
		return "已取消", nil
	default:
		return "", notification.ErrSubscribePayload
	}
}

type notificationEventReader interface {
	FindEvent(context.Context, string) (notification.Event, error)
}

func orderSubscribeGuard(events notificationEventReader, resolver *orderNotificationRecipients, lookup notification.OpenIDLookup, appID string) notification.SubscribeGuard {
	return func(ctx context.Context, item notification.DeliveryItem, snapshot notification.TemplateSnapshot) (string, error) {
		if events == nil || lookup == nil || appID == "" || resolver == nil || resolver.orders == nil || resolver.users == nil || resolver.template == nil || !resolver.template.users[item.RecipientUserID] || item.TemplateKey != "order" || snapshot.TemplateID != resolver.template.id {
			return "", notification.ErrNotificationIneligible
		}
		event, err := events.FindEvent(ctx, item.EventPublicID)
		if err != nil {
			return "", errors.New("notification event lookup failed")
		}
		if event.EventPublicID != item.EventPublicID || notificationEventExpired(event, resolver.now) {
			return "", notification.ErrNotificationIneligible
		}
		recipients, err := resolver.resolveRecipients(ctx, event, item.RecipientUserID)
		if err != nil {
			return "", err
		}
		for _, recipient := range recipients {
			if recipient.UserID != item.RecipientUserID || recipient.Purpose != item.Purpose || !recipient.AllowWechat {
				continue
			}
			// Immutable snapshot must still match the reviewed live mapping.
			if !notificationSnapshotMatches(recipient.WechatTemplateSnapshot, item.TemplateSnapshot, snapshot) {
				return "", notification.ErrNotificationIneligible
			}
			return lookup(ctx, item.RecipientUserID, appID)
		}
		return "", notification.ErrNotificationIneligible
	}
}

func privateMessageSubscribeGuard(events notificationEventReader, resolver *privateNotificationRecipients, lookup notification.OpenIDLookup, appID string) notification.SubscribeGuard {
	return func(ctx context.Context, item notification.DeliveryItem, snapshot notification.TemplateSnapshot) (string, error) {
		if events == nil || lookup == nil || appID == "" || resolver == nil || resolver.identity == nil || resolver.users == nil || resolver.template == nil || !resolver.template.users[item.RecipientUserID] || item.Purpose != notification.PurposePrivateMessage || item.TemplateKey != "private_message" || snapshot.TemplateID != resolver.template.id {
			return "", notification.ErrNotificationIneligible
		}
		event, err := events.FindEvent(ctx, item.EventPublicID)
		if err != nil {
			return "", errors.New("notification event lookup failed")
		}
		if event.EventPublicID != item.EventPublicID || notificationEventExpired(event, resolver.now) {
			return "", notification.ErrNotificationIneligible
		}
		recipients, err := resolver.ResolveRecipients(ctx, event)
		if err != nil {
			return "", err
		}
		for _, recipient := range recipients {
			if recipient.UserID != item.RecipientUserID || recipient.Purpose != notification.PurposePrivateMessage || !recipient.AllowWechat {
				continue
			}
			if !notificationSnapshotMatches(recipient.WechatTemplateSnapshot, item.TemplateSnapshot, snapshot) {
				return "", notification.ErrNotificationIneligible
			}
			return lookup(ctx, item.RecipientUserID, appID)
		}
		return "", notification.ErrNotificationIneligible
	}
}

func notificationEventExpired(event notification.Event, clock func() time.Time) bool {
	now := time.Now()
	if clock != nil {
		now = clock()
	}
	return event.ExpiresAt != nil && !event.ExpiresAt.After(now)
}

func notificationSnapshotMatches(expectedRaw, persistedRaw json.RawMessage, decoded notification.TemplateSnapshot) bool {
	if notification.ValidateTemplateSnapshot(expectedRaw) != nil || notification.ValidateTemplateSnapshot(persistedRaw) != nil {
		return false
	}
	expected, expectedErr := notification.DecodeSubscribeSnapshot(expectedRaw)
	persisted, persistedErr := notification.DecodeSubscribeSnapshot(persistedRaw)
	return expectedErr == nil && persistedErr == nil && reflect.DeepEqual(expected, persisted) && reflect.DeepEqual(expected, decoded)
}

func userSubscribeGuard(events notificationEventReader, orders *orderNotificationRecipients, private *privateNotificationRecipients, lookup notification.OpenIDLookup, appID string) notification.SubscribeGuard {
	orderGuard := orderSubscribeGuard(events, orders, lookup, appID)
	privateGuard := privateMessageSubscribeGuard(events, private, lookup, appID)
	return func(ctx context.Context, item notification.DeliveryItem, snapshot notification.TemplateSnapshot) (string, error) {
		if item.TemplateKey == "private_message" {
			return privateGuard(ctx, item, snapshot)
		}
		return orderGuard(ctx, item, snapshot)
	}
}
