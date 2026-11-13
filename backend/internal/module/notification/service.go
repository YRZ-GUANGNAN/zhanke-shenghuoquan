package notification

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

var (
	ErrResolverUnavailable = errors.New("notification recipient resolver unavailable")
	ErrNoRecipients        = errors.New("notification has no eligible recipients")
)

type Recipient struct {
	UserID                   uint64
	Purpose                  NotificationPurpose
	Title                    string
	Summary                  string
	AggregateType            string
	AggregatePublicID        string
	AggregateVersion         uint64
	TargetPath               string
	AllowWechat              bool
	WechatTemplateKey        string
	WechatTemplateSnapshot   json.RawMessage
	AllowOfficial            bool
	OfficialTemplateKey      string
	OfficialTemplateSnapshot json.RawMessage
}

type RecipientResolver interface {
	ResolveRecipients(context.Context, Event) ([]Recipient, error)
}

type Expander struct {
	Events      EventStore
	Recipients  RecipientResolver
	WecomGroups WecomGroupFormatter
	Lease       time.Duration
	MaxAttempts uint32
	Now         func() time.Time
	RetryAfter  time.Duration
}

type ExpanderOption func(*Expander)

func WithWecomGroupFormatter(formatter WecomGroupFormatter) ExpanderOption {
	return func(expander *Expander) { expander.WecomGroups = formatter }
}

func NewExpander(events EventStore, recipients RecipientResolver, options ...ExpanderOption) *Expander {
	expander := &Expander{Events: events, Recipients: recipients, Lease: 30 * time.Second, MaxAttempts: 5, RetryAfter: 30 * time.Second, Now: time.Now}
	for _, option := range options {
		option(expander)
	}
	return expander
}

func (e *Expander) ExpandOnce(ctx context.Context, now time.Time) (bool, error) {
	if e == nil || e.Events == nil || e.Recipients == nil || now.IsZero() || e.Lease <= 0 || e.MaxAttempts == 0 || e.RetryAfter <= 0 {
		return false, ErrInvalidRecord
	}
	lease, found, err := e.Events.ClaimNextEvent(ctx, now, e.Lease)
	if err != nil || !found {
		return found, err
	}
	if lease.Event.AttemptCount > e.MaxAttempts {
		return true, e.fail(ctx, lease, "attempts_exhausted", now)
	}
	if lease.Event.ExpiresAt != nil && !lease.Event.ExpiresAt.After(now) {
		return true, e.complete(ctx, lease, nil, nil, EventSkipped, now)
	}
	recipients, err := e.Recipients.ResolveRecipients(ctx, lease.Event)
	if err != nil {
		return true, e.fail(ctx, lease, "recipient_resolve_failed", now)
	}
	commitNow := e.clock(now)
	if lease.Event.ExpiresAt != nil && !lease.Event.ExpiresAt.After(commitNow) {
		return true, e.complete(ctx, lease, nil, nil, EventSkipped, commitNow)
	}
	inbox, deliveries, buildErr := e.build(lease.Event, recipients, commitNow)
	if buildErr != nil {
		return true, e.fail(ctx, lease, "recipient_payload_rejected", commitNow)
	}
	status := EventExpanded
	if len(inbox) == 0 && len(deliveries) == 0 {
		status = EventSkipped
	}
	return true, e.complete(ctx, lease, inbox, deliveries, status, commitNow)
}

func (e *Expander) fail(ctx context.Context, lease EventLease, code string, now time.Time) error {
	now = e.clock(now)
	delay := e.RetryAfter
	for n := uint32(1); n < lease.Event.AttemptCount && delay < 30*time.Minute; n++ {
		delay *= 2
	}
	if delay > 30*time.Minute || delay < 0 {
		delay = 30 * time.Minute
	}
	next := now.Add(delay)
	_, err := e.Events.FailEvent(ctx, lease, code, "recipient expansion failed", next, e.MaxAttempts, now)
	return err
}

func (e *Expander) clock(previous time.Time) time.Time {
	if e.Now != nil {
		if fresh := e.Now(); fresh.After(previous) {
			return fresh
		}
	}
	return previous
}

func (e *Expander) complete(ctx context.Context, lease EventLease, inbox []InboxItem, deliveries []DeliveryItem, status EventStatus, now time.Time) error {
	now = e.clock(now)
	if !eventLeaseValid(lease, now) {
		return ErrConcurrentUpdate
	}
	if lease.Event.ExpiresAt != nil && !lease.Event.ExpiresAt.After(now) {
		inbox, deliveries, status = nil, nil, EventSkipped
	}
	if err := e.Events.ExpandAndComplete(ctx, lease, inbox, deliveries, status, now); err != nil {
		if errors.Is(err, ErrConcurrentUpdate) {
			return err
		}
		// A failed transaction rolled back all channel rows. Move the still-owned
		// event to bounded retry/deadletter instead of waiting forever in processing.
		if failErr := e.fail(ctx, lease, "expansion_commit_failed", now); failErr != nil {
			return errors.Join(err, failErr)
		}
		return err
	}
	return nil
}

func (e *Expander) build(event Event, recipients []Recipient, now time.Time) ([]InboxItem, []DeliveryItem, error) {
	type coordinate struct {
		UserID  uint64
		Purpose NotificationPurpose
	}
	seen := make(map[coordinate]Recipient)
	inbox := make([]InboxItem, 0, len(recipients))
	deliveries := make([]DeliveryItem, 0, len(recipients)*3)
	for _, recipient := range recipients {
		if recipient.UserID == 0 || !safeText(string(recipient.Purpose), 48, true) || !safeText(recipient.Title, 128, true) || !safeText(recipient.Summary, 256, false) || !safeText(recipient.AggregateType, 32, true) || !safeText(recipient.AggregatePublicID, 128, true) || recipient.AggregateVersion == 0 || !safeText(recipient.TargetPath, 256, false) {
			return nil, nil, ErrInvalidRecord
		}
		if recipient.AggregateType != event.AggregateType || recipient.AggregatePublicID != event.AggregatePublicID || recipient.AggregateVersion != event.AggregateVersion {
			return nil, nil, ErrInvalidRecord
		}
		key := coordinate{recipient.UserID, recipient.Purpose}
		if existing, ok := seen[key]; ok {
			if !sameRecipient(existing, recipient) {
				return nil, nil, ErrIdempotencyConflict
			}
			continue
		}
		seen[key] = recipient
		inboxID, err := newID()
		if err != nil {
			return nil, nil, err
		}
		item := InboxItem{InboxPublicID: inboxID, EventPublicID: event.EventPublicID, RecipientUserID: recipient.UserID, Purpose: recipient.Purpose, Title: recipient.Title, Summary: recipient.Summary, AggregateType: recipient.AggregateType, AggregatePublicID: recipient.AggregatePublicID, AggregateVersion: recipient.AggregateVersion, TargetPath: recipient.TargetPath, Status: DeliveryPending, RowVersion: 1, CreatedAt: now, UpdatedAt: now}
		inbox = append(inbox, item)
		inAppID, err := newID()
		if err != nil {
			return nil, nil, err
		}
		deliveries = append(deliveries, DeliveryItem{DeliveryPublicID: inAppID, EventPublicID: event.EventPublicID, RecipientUserID: recipient.UserID, Channel: ChannelInApp, Purpose: recipient.Purpose, TemplateSnapshot: json.RawMessage(`{}`), PayloadVersion: 1, Status: DeliveryPending, RowVersion: 1, ExpiresAt: event.ExpiresAt, CreatedAt: now, UpdatedAt: now})
		if recipient.AllowWechat && safeText(recipient.WechatTemplateKey, 64, true) && validWechatTemplateSnapshot(recipient.WechatTemplateSnapshot) {
			wechatID, err := newID()
			if err != nil {
				return nil, nil, err
			}
			deliveries = append(deliveries, DeliveryItem{DeliveryPublicID: wechatID, EventPublicID: event.EventPublicID, RecipientUserID: recipient.UserID, Channel: ChannelWechatSubscribe, Purpose: recipient.Purpose, TemplateKey: recipient.WechatTemplateKey, TemplateSnapshot: recipient.WechatTemplateSnapshot, PayloadVersion: 1, Status: DeliveryPending, RowVersion: 1, ExpiresAt: event.ExpiresAt, CreatedAt: now, UpdatedAt: now})
		}
		if recipient.AllowOfficial && safeText(recipient.OfficialTemplateKey, 64, true) && validWechatTemplateSnapshot(recipient.OfficialTemplateSnapshot) {
			officialID, err := newID()
			if err != nil {
				return nil, nil, err
			}
			deliveries = append(deliveries, DeliveryItem{DeliveryPublicID: officialID, EventPublicID: event.EventPublicID, RecipientUserID: recipient.UserID, Channel: ChannelWechatOfficial, Purpose: recipient.Purpose, TemplateKey: recipient.OfficialTemplateKey, TemplateSnapshot: recipient.OfficialTemplateSnapshot, PayloadVersion: 1, Status: DeliveryPending, RowVersion: 1, ExpiresAt: event.ExpiresAt, CreatedAt: now, UpdatedAt: now})
		}
	}
	if e.WecomGroups != nil {
		groups, err := e.WecomGroups(event)
		if err != nil {
			return nil, nil, ErrInvalidRecord
		}
		seenGroups := map[string]bool{}
		for _, group := range groups {
			if !validWecomGroupMessage(group) || seenGroups[group.GroupKey] {
				return nil, nil, ErrInvalidRecord
			}
			seenGroups[group.GroupKey] = true
			recipientID, ok := wecomGroupRecipientID(group.GroupKey)
			if !ok {
				return nil, nil, ErrInvalidRecord
			}
			snapshot, err := json.Marshal(WecomGroupSnapshot{
				Version: 1, GroupKey: group.GroupKey, Title: group.Title, Summary: group.Summary,
				AggregateType: event.AggregateType, AggregatePublicID: event.AggregatePublicID,
				AggregateVersion: event.AggregateVersion, OccurredAt: event.OccurredAt,
			})
			if err != nil {
				return nil, nil, err
			}
			deliveryID, err := newID()
			if err != nil {
				return nil, nil, err
			}
			deliveries = append(deliveries, DeliveryItem{
				DeliveryPublicID: deliveryID, EventPublicID: event.EventPublicID,
				RecipientUserID: recipientID, Channel: ChannelWecom, Purpose: PurposeOrder,
				TemplateKey: group.GroupKey, TemplateSnapshot: snapshot, PayloadVersion: 1,
				Status: DeliveryPending, RowVersion: 1, ExpiresAt: event.ExpiresAt,
				CreatedAt: now, UpdatedAt: now,
			})
		}
	}
	return inbox, deliveries, nil
}

type WecomGroupMessage struct {
	GroupKey string
	Title    string
	Summary  string
}

type WecomGroupSnapshot struct {
	Version           uint32    `json:"version"`
	GroupKey          string    `json:"groupKey"`
	Title             string    `json:"title"`
	Summary           string    `json:"summary"`
	AggregateType     string    `json:"aggregateType"`
	AggregatePublicID string    `json:"aggregatePublicId"`
	AggregateVersion  uint64    `json:"aggregateVersion"`
	OccurredAt        time.Time `json:"occurredAt"`
}

type WecomGroupFormatter func(Event) ([]WecomGroupMessage, error)

func wecomGroupRecipientID(group string) (uint64, bool) {
	switch group {
	case "merchant_ops":
		return WecomMerchantGroupRecipientID, true
	case "rider_ops":
		return WecomRiderGroupRecipientID, true
	default:
		return 0, false
	}
}

func validWecomGroupMessage(message WecomGroupMessage) bool {
	_, groupValid := wecomGroupRecipientID(message.GroupKey)
	return groupValid && safeText(message.Title, 128, true) && safeText(message.Summary, 256, true)
}

func DecodeWecomGroupSnapshot(item DeliveryItem, raw json.RawMessage) (WecomGroupSnapshot, error) {
	var snapshot WecomGroupSnapshot
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if item.Channel != ChannelWecom || decoder.Decode(&snapshot) != nil || decoder.Decode(new(any)) != io.EOF ||
		snapshot.Version != 1 || snapshot.GroupKey != item.TemplateKey ||
		snapshot.AggregatePublicID == "" || snapshot.AggregateVersion == 0 || snapshot.OccurredAt.IsZero() {
		return WecomGroupSnapshot{}, ErrInvalidRecord
	}
	recipientID, ok := wecomGroupRecipientID(snapshot.GroupKey)
	if !ok || recipientID != item.RecipientUserID ||
		!validWecomGroupMessage(WecomGroupMessage{GroupKey: snapshot.GroupKey, Title: snapshot.Title, Summary: snapshot.Summary}) {
		return WecomGroupSnapshot{}, ErrInvalidRecord
	}
	return snapshot, nil
}

func validWechatTemplateSnapshot(raw json.RawMessage) bool {
	var value struct {
		TemplateID string                            `json:"templateId"`
		Page       string                            `json:"page"`
		Data       map[string]SubscribeTemplateValue `json:"data"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if !validMetadata(raw) || decoder.Decode(&value) != nil || value.Page == "" || len(value.Data) == 0 {
		return false
	}
	if decoder.Decode(new(any)) != io.EOF {
		return false
	}
	// Validate template fields using the sender's exact limits. This placeholder
	// is used only for validation and is never persisted or sent as an OpenID.
	return validateSubscribeMessage(SubscribeMessage{OpenID: "server-resolved-recipient", TemplateID: value.TemplateID, Page: value.Page, Data: value.Data}) == nil
}

func sameRecipient(left, right Recipient) bool {
	return left.UserID == right.UserID && left.Purpose == right.Purpose && left.Title == right.Title && left.Summary == right.Summary && left.AggregateType == right.AggregateType && left.AggregatePublicID == right.AggregatePublicID && left.AggregateVersion == right.AggregateVersion && left.TargetPath == right.TargetPath && left.AllowWechat == right.AllowWechat && left.WechatTemplateKey == right.WechatTemplateKey && ((len(left.WechatTemplateSnapshot) == 0 && len(right.WechatTemplateSnapshot) == 0) || sameJSON(left.WechatTemplateSnapshot, right.WechatTemplateSnapshot)) && left.AllowOfficial == right.AllowOfficial && left.OfficialTemplateKey == right.OfficialTemplateKey && ((len(left.OfficialTemplateSnapshot) == 0 && len(right.OfficialTemplateSnapshot) == 0) || sameJSON(left.OfficialTemplateSnapshot, right.OfficialTemplateSnapshot))
}
