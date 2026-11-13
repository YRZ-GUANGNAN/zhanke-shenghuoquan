package notification

import (
	"context"
	"errors"
	"time"
)

type OfficialAccountStore interface {
	ClaimWechatOfficialDelivery(context.Context, time.Time, time.Duration) (DeliveryLease, bool, error)
	CompleteDelivery(context.Context, DeliveryLease, DeliveryStatus, time.Time) error
	FailDelivery(context.Context, DeliveryLease, string, string, time.Time, uint32, time.Time) (DeliveryStatus, error)
}

type OfficialAccountGuard func(context.Context, DeliveryItem, TemplateSnapshot) (uint64, error)

type OfficialAccountWorker struct {
	Store       OfficialAccountStore
	Sender      *OfficialAccountSender
	Guard       OfficialAccountGuard
	Lookup      func(context.Context, uint64) (string, error)
	MarkFollow  func(context.Context, uint64, string) error
	Now         func() time.Time
	MaxAttempts uint32
}

func NewOfficialAccountWorker(
	store OfficialAccountStore,
	sender *OfficialAccountSender,
	guard OfficialAccountGuard,
	lookup func(context.Context, uint64) (string, error),
) *OfficialAccountWorker {
	return &OfficialAccountWorker{Store: store, Sender: sender, Guard: guard, Lookup: lookup, Now: time.Now, MaxAttempts: 5}
}

func (w *OfficialAccountWorker) RunOnce(ctx context.Context) (bool, error) {
	if w == nil || w.Store == nil || w.Sender == nil || w.Guard == nil || w.Lookup == nil || w.Now == nil || w.MaxAttempts == 0 {
		return false, ErrInvalidRecord
	}
	lease, found, err := w.Store.ClaimWechatOfficialDelivery(ctx, w.Now().UTC(), time.Minute)
	if err != nil || !found {
		return found, err
	}
	now := w.Now().UTC()
	if lease.Item.ExpiresAt != nil && !lease.Item.ExpiresAt.After(now) {
		return true, w.Store.CompleteDelivery(ctx, lease, DeliverySkipped, now)
	}
	snapshot, err := DecodeSubscribeSnapshot(lease.Item.TemplateSnapshot)
	if err != nil {
		return true, w.Store.CompleteDelivery(ctx, lease, DeliveryBlocked, now)
	}
	userID, err := w.Guard(ctx, lease.Item, snapshot)
	if errors.Is(err, ErrNotificationIneligible) {
		return true, w.Store.CompleteDelivery(ctx, lease, DeliverySkipped, now)
	}
	if err != nil {
		return true, w.retry(ctx, lease, "eligibility_lookup_failed", now)
	}
	openid, err := w.Lookup(ctx, userID)
	if errors.Is(err, ErrOfficialAccountNotBound) || openid == "" {
		return true, w.Store.CompleteDelivery(ctx, lease, DeliverySkipped, now)
	}
	if err != nil {
		return true, w.retry(ctx, lease, "official_account_lookup_failed", now)
	}
	result, sendErr := w.Sender.Send(ctx, SubscribeMessage{
		TaskID: lease.Item.DeliveryPublicID, OpenID: openid, TemplateID: snapshot.TemplateID,
		Page: snapshot.Page, Data: snapshot.Data,
	})
	if result.Status == SendStatusAccepted && sendErr == nil {
		if w.MarkFollow != nil {
			_ = w.MarkFollow(ctx, userID, "following")
		}
		return true, w.Store.CompleteDelivery(ctx, lease, DeliveryAccepted, now)
	}
	if result.ErrorCode == "not_following" && w.MarkFollow != nil {
		_ = w.MarkFollow(ctx, userID, "not_following")
	}
	code := result.ErrorCode
	if code == "" {
		code = "provider_failed"
	}
	if result.ErrorCode == "configuration_missing" || result.ErrorCode == "invalid_payload" {
		return true, w.Store.CompleteDelivery(ctx, lease, DeliveryBlocked, now)
	}
	return true, w.retry(ctx, lease, code, now)
}

func (w *OfficialAccountWorker) retry(ctx context.Context, lease DeliveryLease, code string, now time.Time) error {
	next := now.Add(time.Duration(1<<minWorkerAttempt(lease.Item.AttemptCount)) * 15 * time.Second)
	_, err := w.Store.FailDelivery(ctx, lease, code, "official account delivery failed", next, w.MaxAttempts, now)
	return err
}

func (w *OfficialAccountWorker) Run(ctx context.Context) error {
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
