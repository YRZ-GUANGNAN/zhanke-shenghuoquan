package main

import (
	"context"
	"encoding/json"
	"errors"
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

func configuredPrivateMessageNotificationTemplate(cfg config.Config) (*privateMessageNotificationTemplate, error) {
	if !cfg.EnableWechatSubscribeNotifications || strings.TrimSpace(cfg.WechatMessageTemplateID) == "" && strings.TrimSpace(cfg.WechatMessageTemplateFields) == "" {
		return nil, nil
	}
	template := &privateMessageNotificationTemplate{id: strings.TrimSpace(cfg.WechatMessageTemplateID), users: map[uint64]bool{}}
	if template.id == "" || json.Unmarshal([]byte(cfg.WechatMessageTemplateFields), &template.fields) != nil || len(template.fields) == 0 {
		return nil, errors.New("private message template requires approved ID and field mapping")
	}
	for _, semantic := range template.fields {
		if semantic != "status" && semantic != "time" && semantic != "unreadCount" {
			return nil, errors.New("unsupported private message template field mapping")
		}
	}
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
	_, err := template.snapshot(notification.Event{AggregatePublicID: "im-pair:1:2", OccurredAt: time.Now()}, notification.Recipient{Title: "收到新私信", TargetPath: "/pages/secondhand-chat/index?relationId=1"})
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
	if template.id == "" || json.Unmarshal([]byte(cfg.WechatOrderTemplateFields), &template.fields) != nil || len(template.fields) == 0 {
		return nil, errors.New("notification order template requires approved ID and field mapping")
	}
	for _, semantic := range template.fields {
		if semantic != "orderId" && semantic != "status" && semantic != "time" {
			return nil, errors.New("unsupported notification template field mapping")
		}
	}
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
	// Validate the longest currently supported state label and public ID, not
	// invented field keys or truncated notification text.
	_, err := template.snapshot(notification.Event{AggregatePublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", OccurredAt: time.Now()}, notification.Recipient{Title: "骑手正在前往取货", TargetPath: "/pages/delivery-rider/index"})
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

type notificationEventReader interface {
	FindEvent(context.Context, string) (notification.Event, error)
}

func orderSubscribeGuard(events notificationEventReader, resolver *orderNotificationRecipients, lookup notification.OpenIDLookup, appID string) notification.SubscribeGuard {
	return func(ctx context.Context, item notification.DeliveryItem, snapshot notification.TemplateSnapshot) (string, error) {
		if resolver == nil || resolver.template == nil || !resolver.template.users[item.RecipientUserID] || item.TemplateKey != "order" || snapshot.TemplateID != resolver.template.id {
			return "", notification.ErrNotificationIneligible
		}
		event, err := events.FindEvent(ctx, item.EventPublicID)
		if err != nil {
			return "", errors.New("notification event lookup failed")
		}
		if event.ExpiresAt != nil && !event.ExpiresAt.After(resolver.now()) {
			return "", notification.ErrNotificationIneligible
		}
		recipients, err := resolver.ResolveRecipients(ctx, event)
		if err != nil {
			return "", err
		}
		for _, recipient := range recipients {
			if recipient.UserID != item.RecipientUserID || recipient.Purpose != item.Purpose || !recipient.AllowWechat {
				continue
			}
			// Immutable snapshot must still match the reviewed live mapping.
			var expected, actual any
			if json.Unmarshal(recipient.WechatTemplateSnapshot, &expected) != nil || json.Unmarshal(item.TemplateSnapshot, &actual) != nil {
				return "", notification.ErrNotificationIneligible
			}
			expectedJSON, _ := json.Marshal(expected)
			actualJSON, _ := json.Marshal(actual)
			if string(expectedJSON) != string(actualJSON) {
				return "", notification.ErrNotificationIneligible
			}
			return lookup(ctx, item.RecipientUserID, appID)
		}
		return "", notification.ErrNotificationIneligible
	}
}

func privateMessageSubscribeGuard(events notificationEventReader, resolver *privateNotificationRecipients, lookup notification.OpenIDLookup, appID string) notification.SubscribeGuard {
	return func(ctx context.Context, item notification.DeliveryItem, snapshot notification.TemplateSnapshot) (string, error) {
		if resolver == nil || resolver.template == nil || !resolver.template.users[item.RecipientUserID] || item.TemplateKey != "private_message" || snapshot.TemplateID != resolver.template.id {
			return "", notification.ErrNotificationIneligible
		}
		event, err := events.FindEvent(ctx, item.EventPublicID)
		if err != nil {
			return "", errors.New("notification event lookup failed")
		}
		if event.ExpiresAt != nil && !event.ExpiresAt.After(resolver.now()) {
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
			var expected, actual any
			if json.Unmarshal(recipient.WechatTemplateSnapshot, &expected) != nil || json.Unmarshal(item.TemplateSnapshot, &actual) != nil {
				return "", notification.ErrNotificationIneligible
			}
			expectedJSON, _ := json.Marshal(expected)
			actualJSON, _ := json.Marshal(actual)
			if string(expectedJSON) != string(actualJSON) {
				return "", notification.ErrNotificationIneligible
			}
			return lookup(ctx, item.RecipientUserID, appID)
		}
		return "", notification.ErrNotificationIneligible
	}
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
