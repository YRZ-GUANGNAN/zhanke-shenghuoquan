package notification

import (
	"context"
	"errors"
	"time"
)

type WecomStore interface {
	ClaimWecomDelivery(context.Context, time.Time, time.Duration) (DeliveryLease, bool, error)
	CompleteDelivery(context.Context, DeliveryLease, DeliveryStatus, time.Time) error
	FailDelivery(context.Context, DeliveryLease, string, string, time.Time, uint32, time.Time) (DeliveryStatus, error)
}

type WecomWorker struct {
	Store       WecomStore
	Sender      WecomSender
	Now         func() time.Time
	MaxAttempts uint32
}

func NewWecomWorker(store WecomStore, sender WecomSender) *WecomWorker {
	return &WecomWorker{Store: store, Sender: sender, Now: time.Now, MaxAttempts: 5}
}

func (w *WecomWorker) RunOnce(ctx context.Context) (bool, error) {
	if w == nil || w.Store == nil || w.Sender == nil || w.Now == nil || w.MaxAttempts == 0 {
		return false, ErrInvalidRecord
	}
	lease, found, err := w.Store.ClaimWecomDelivery(ctx, w.Now().UTC(), time.Minute)
	if err != nil || !found {
		return found, err
	}
	now := w.Now().UTC()
	if lease.Item.ExpiresAt != nil && !lease.Item.ExpiresAt.After(now) {
		return true, w.Store.CompleteDelivery(ctx, lease, DeliverySkipped, now)
	}
	snapshot, err := DecodeWecomGroupSnapshot(lease.Item, lease.Item.TemplateSnapshot)
	if err != nil {
		return true, w.Store.CompleteDelivery(ctx, lease, DeliveryBlocked, now)
	}
	if !w.Sender.Enabled(snapshot.GroupKey) {
		return true, w.Store.CompleteDelivery(ctx, lease, DeliveryBlocked, now)
	}
	status, sendErr := w.Sender.Send(ctx, snapshot)
	if sendErr == nil && status == SendStatusAccepted {
		return true, w.Store.CompleteDelivery(ctx, lease, DeliveryAccepted, now)
	}
	code := "provider_rejected"
	if errors.Is(sendErr, ErrWecomConfiguration) {
		code = "configuration_missing"
	} else if errors.Is(sendErr, ErrWecomPayload) {
		code = "provider_failed"
	} else if status == SendStatusUnknown {
		code = "provider_result_unknown"
	}
	next := now.Add(time.Duration(1<<minWorkerAttempt(lease.Item.AttemptCount)) * 15 * time.Second)
	_, failErr := w.Store.FailDelivery(ctx, lease, code, "WeCom group delivery failed", next, w.MaxAttempts, now)
	return true, failErr
}

func (w *WecomWorker) Run(ctx context.Context) error {
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

func minWorkerAttempt(attempt uint32) uint32 {
	if attempt > 5 {
		return 5
	}
	return attempt
}
