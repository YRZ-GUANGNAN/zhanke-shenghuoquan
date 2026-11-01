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
	UserID                 uint64
	Purpose                NotificationPurpose
	Title                  string
	Summary                string
	AggregateType          string
	AggregatePublicID      string
	AggregateVersion       uint64
	TargetPath             string
	AllowWechat            bool
	WechatTemplateKey      string
	WechatTemplateSnapshot json.RawMessage
}

type RecipientResolver interface {
	ResolveRecipients(context.Context, Event) ([]Recipient, error)
}

type Expander struct {
	Events      EventStore
	Recipients  RecipientResolver
	Lease       time.Duration
	MaxAttempts uint32
	Now         func() time.Time
	RetryAfter  time.Duration
}

func NewExpander(events EventStore, recipients RecipientResolver) *Expander {
	return &Expander{Events: events, Recipients: recipients, Lease: 30 * time.Second, MaxAttempts: 5, RetryAfter: 30 * time.Second, Now: time.Now}
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
	if len(inbox) == 0 {
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
	deliveries := make([]DeliveryItem, 0, len(recipients)*2)
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
	}
	return inbox, deliveries, nil
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
	return left.UserID == right.UserID && left.Purpose == right.Purpose && left.Title == right.Title && left.Summary == right.Summary && left.AggregateType == right.AggregateType && left.AggregatePublicID == right.AggregatePublicID && left.AggregateVersion == right.AggregateVersion && left.TargetPath == right.TargetPath && left.AllowWechat == right.AllowWechat && left.WechatTemplateKey == right.WechatTemplateKey && ((len(left.WechatTemplateSnapshot) == 0 && len(right.WechatTemplateSnapshot) == 0) || sameJSON(left.WechatTemplateSnapshot, right.WechatTemplateSnapshot))
}
