package notification

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// TemplateSnapshot is immutable and has no recipient or credential fields.
type TemplateSnapshot struct {
	TemplateID string                            `json:"templateId"`
	Page       string                            `json:"page"`
	Data       map[string]SubscribeTemplateValue `json:"data"`
}

func DecodeSubscribeSnapshot(raw json.RawMessage) (TemplateSnapshot, error) {
	var result TemplateSnapshot
	if !validMetadata(raw) || json.Unmarshal(raw, &result) != nil || len(result.Data) == 0 {
		return result, ErrSubscribePayload
	}
	if err := validateSubscribeMessage(SubscribeMessage{OpenID: "validation_only", TemplateID: result.TemplateID, Page: result.Page, Data: result.Data}); err != nil {
		return TemplateSnapshot{}, ErrSubscribePayload
	}
	return result, nil
}

var ErrNotificationIneligible = errors.New("notification recipient or event is no longer eligible")

// Guard rechecks live account, preference, template and business facts just
// before reserving quota. Lookup errors are retriable; ineligible is terminal.
type SubscribeGuard func(context.Context, DeliveryItem, TemplateSnapshot) (string, error)

type SubscribeOutcome struct {
	Status            DeliveryStatus
	Code              string
	ProviderMessageID string
	NextAttemptAt     *time.Time
	QuotaAction       string // consume, release, invalidate, or none
}

type SubscribeStore interface {
	ClaimWechatDelivery(context.Context, time.Time, time.Duration) (DeliveryLease, bool, error)
	ReserveForDelivery(context.Context, DeliveryLease, string, string, string, time.Time) (DeliveryLease, error)
	FinishSubscribe(context.Context, DeliveryLease, SubscribeOutcome, time.Time) error
}

type SubscribeWorker struct {
	Store       SubscribeStore
	Sender      Sender
	Guard       SubscribeGuard
	AppID       string
	Now         func() time.Time
	MaxAttempts uint32
}

func NewSubscribeWorker(store SubscribeStore, sender Sender, guard SubscribeGuard, appID string) *SubscribeWorker {
	return &SubscribeWorker{Store: store, Sender: sender, Guard: guard, AppID: appID, Now: time.Now, MaxAttempts: 5}
}

func (w *SubscribeWorker) RunOnce(ctx context.Context) (bool, error) {
	// A disabled sender must not claim tasks or consume consent.
	if w == nil || w.Sender == nil || !w.Sender.Enabled() {
		return false, nil
	}
	if w.Store == nil || w.Guard == nil || w.Now == nil || w.AppID == "" || w.MaxAttempts == 0 {
		return false, ErrInvalidRecord
	}
	lease, found, err := w.Store.ClaimWechatDelivery(ctx, w.Now().UTC(), time.Minute)
	if err != nil || !found {
		return found, err
	}
	finish := func(outcome SubscribeOutcome) (bool, error) {
		return true, w.Store.FinishSubscribe(ctx, lease, outcome, w.Now().UTC())
	}
	if lease.Item.ExpiresAt != nil && !lease.Item.ExpiresAt.After(w.Now()) {
		return finish(SubscribeOutcome{Status: DeliverySkipped, Code: "event_expired", QuotaAction: "none"})
	}
	snapshot, err := DecodeSubscribeSnapshot(lease.Item.TemplateSnapshot)
	if err != nil {
		return finish(SubscribeOutcome{Status: DeliveryBlocked, Code: "invalid_template_snapshot", QuotaAction: "none"})
	}
	openid, err := w.Guard(ctx, lease.Item, snapshot)
	if errors.Is(err, ErrNotificationIneligible) {
		return finish(SubscribeOutcome{Status: DeliverySkipped, Code: "recipient_ineligible", QuotaAction: "none"})
	}
	if err != nil {
		return finish(w.retry(lease, "eligibility_lookup_failed", "none"))
	}
	if openid == "" {
		return finish(SubscribeOutcome{Status: DeliveryBlocked, Code: "identity_unavailable", QuotaAction: "none"})
	}
	reserved, err := w.Store.ReserveForDelivery(ctx, lease, w.AppID, openid, snapshot.TemplateID, w.Now().UTC())
	if errors.Is(err, ErrSubscriptionEmpty) {
		return finish(SubscribeOutcome{Status: DeliveryBlocked, Code: "subscription_unavailable", QuotaAction: "none"})
	}
	if err != nil {
		return true, err
	} // no send until the reservation transaction committed
	lease = reserved
	// A lock wait can outlive account, preference, template or order changes.
	// Never send using an identity verified only before the reservation.
	currentOpenID, guardErr := w.Guard(ctx, lease.Item, snapshot)
	if errors.Is(guardErr, ErrNotificationIneligible) || (guardErr == nil && currentOpenID != openid) {
		return finish(SubscribeOutcome{Status: DeliverySkipped, Code: "recipient_changed", QuotaAction: "release"})
	}
	if guardErr != nil {
		return finish(w.retry(lease, "eligibility_lookup_failed", "release"))
	}
	if lease.Item.ExpiresAt != nil && !lease.Item.ExpiresAt.After(w.Now()) {
		return finish(SubscribeOutcome{Status: DeliverySkipped, Code: "event_expired", QuotaAction: "release"})
	}
	// The reserve operation can block behind another transaction. A deadline
	// bounded by the lease prevents sending after it has already expired.
	if lease.Item.LeaseExpiresAt == nil || !lease.Item.LeaseExpiresAt.After(w.Now().Add(15*time.Second)) {
		return finish(w.retry(lease, "insufficient_lease", "release"))
	}
	sendContext, cancel := context.WithTimeout(ctx, 15*time.Second)
	result, sendErr := w.Sender.Send(sendContext, SubscribeMessage{TaskID: lease.Item.DeliveryPublicID, OpenID: openid, TemplateID: snapshot.TemplateID, Page: snapshot.Page, Data: snapshot.Data})
	cancel()
	outcome := SubscribeOutcome{Status: DeliveryUnknown, Code: "provider_result_unknown", QuotaAction: "consume"}
	switch result.Status {
	case SendStatusAccepted:
		if sendErr == nil {
			outcome = SubscribeOutcome{Status: DeliveryAccepted, ProviderMessageID: result.ProviderMessageID, QuotaAction: "consume"}
		}
	case SendStatusBlocked:
		// Sender contract: blocked means no message-send request was made.
		outcome = w.retry(lease, "provider_not_sent", "release")
		if result.ErrorCode == "configuration_missing" || result.ErrorCode == "invalid_payload" {
			outcome = SubscribeOutcome{Status: DeliveryBlocked, Code: result.ErrorCode, QuotaAction: "release"}
		}
	case SendStatusRejected:
		switch result.ErrorCode {
		case "token_invalid", "rate_limited":
			outcome = w.retry(lease, result.ErrorCode, "release")
		case "subscription_rejected", "openid_invalid":
			outcome = SubscribeOutcome{Status: DeliveryBlocked, Code: result.ErrorCode, QuotaAction: "invalidate"}
		default:
			outcome = SubscribeOutcome{Status: DeliveryBlocked, Code: "provider_rejected", QuotaAction: "release"}
		}
	case SendStatusUnknown:
		outcome.Code = "provider_result_unknown"
	}
	// Finishing an accepted/unknown request must still be attempted if the
	// worker's context was cancelled. A failed commit remains unknown on lease
	// expiry and can never become an automatic second send.
	finishContext, cancelFinish := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelFinish()
	return true, w.Store.FinishSubscribe(finishContext, lease, outcome, w.Now().UTC())
}

func (w *SubscribeWorker) retry(lease DeliveryLease, code, quota string) SubscribeOutcome {
	if lease.Item.AttemptCount >= w.MaxAttempts {
		return SubscribeOutcome{Status: DeliveryDeadletter, Code: code, QuotaAction: quota}
	}
	exponent := lease.Item.AttemptCount
	if exponent > 5 {
		exponent = 5
	}
	next := w.Now().UTC().Add(time.Duration(1<<exponent) * 15 * time.Second)
	return SubscribeOutcome{Status: DeliveryRetryWait, Code: code, NextAttemptAt: &next, QuotaAction: quota}
}

func (w *SubscribeWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			_, _ = w.RunOnce(ctx)
		}
	}
}
