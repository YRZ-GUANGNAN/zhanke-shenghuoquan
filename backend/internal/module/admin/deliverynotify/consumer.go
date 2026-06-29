package deliverynotify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"campus-circle/backend/internal/module/delivery"
)

type ConsumerConfig struct {
	LeaseTTL    time.Duration
	RetryDelay  time.Duration
	MaxAttempts uint32
	Now         func() time.Time
}

type Consumer struct {
	repository delivery.DispatchRepository
	sender     Sender
	config     ConsumerConfig
}

type BatchResult struct {
	Claimed     int
	Succeeded   int
	Skipped     int
	Retried     int
	Deadletters int
}

func NewConsumer(
	repository delivery.DispatchRepository,
	sender Sender,
	config ConsumerConfig,
) (*Consumer, error) {
	if repository == nil || sender == nil {
		return nil, delivery.ErrInvalidRecord
	}
	if config.LeaseTTL <= 0 {
		config.LeaseTTL = 30 * time.Second
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 5 * time.Second
	}
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 5
	}
	if config.Now == nil {
		config.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Consumer{repository: repository, sender: sender, config: config}, nil
}

func (c *Consumer) ConsumeBatch(ctx context.Context, limit int) (BatchResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var result BatchResult
	for result.Claimed < limit {
		now := c.config.Now().UTC()
		item, found, err := c.repository.ClaimNextOutbox(ctx, now, c.config.LeaseTTL)
		if err != nil {
			return result, err
		}
		if !found {
			break
		}
		result.Claimed++
		if !c.sender.Enabled() {
			if err := c.repository.CompleteOutbox(ctx, item, delivery.OutboxSkipped, now); err != nil {
				return result, err
			}
			result.Skipped++
			continue
		}
		payload, err := decodePayload(item.Event)
		if err == nil {
			err = c.sender.Send(ctx, item.Event, payload)
		}
		if err == nil {
			if err := c.repository.CompleteOutbox(ctx, item, delivery.OutboxSucceeded, now); err != nil {
				return result, err
			}
			result.Succeeded++
			continue
		}
		delay := c.config.RetryDelay * time.Duration(1<<min(item.Event.AttemptCount-1, 8))
		status, failErr := c.repository.FailOutbox(
			ctx, item, notificationErrorCode(err), safeNotificationError(err),
			now.Add(delay), c.config.MaxAttempts, now,
		)
		if failErr != nil {
			return result, failErr
		}
		if status == delivery.OutboxDeadletter {
			result.Deadletters++
		} else {
			result.Retried++
		}
	}
	return result, nil
}

func (c *Consumer) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := c.ConsumeBatch(ctx, 20); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func decodePayload(event delivery.OutboxEvent) (delivery.RiderPoolOpenedPayload, error) {
	if event.EventType != delivery.EventRiderPoolOpened {
		return delivery.RiderPoolOpenedPayload{}, delivery.ErrOutboxPayload
	}
	decoder := json.NewDecoder(bytes.NewReader(event.Payload))
	decoder.DisallowUnknownFields()
	var payload delivery.RiderPoolOpenedPayload
	if err := decoder.Decode(&payload); err != nil {
		return delivery.RiderPoolOpenedPayload{}, delivery.ErrOutboxPayload
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return delivery.RiderPoolOpenedPayload{}, delivery.ErrOutboxPayload
	}
	if !delivery.ValidULID(payload.OrderPublicID) || payload.DeliveryFeeCents == 0 ||
		strings.TrimSpace(payload.RouteSummary) == "" {
		return delivery.RiderPoolOpenedPayload{}, delivery.ErrOutboxPayload
	}
	return payload, nil
}

func notificationErrorCode(err error) string {
	switch {
	case errors.Is(err, delivery.ErrOutboxPayload), errors.Is(err, ErrWebhookPayload):
		return "invalid_payload"
	case errors.Is(err, ErrWebhookConfiguration):
		return "configuration"
	default:
		return "webhook_failed"
	}
}

func safeNotificationError(err error) string {
	switch {
	case errors.Is(err, delivery.ErrOutboxPayload), errors.Is(err, ErrWebhookPayload):
		return "notification payload rejected"
	case errors.Is(err, ErrWebhookConfiguration):
		return "notification configuration rejected"
	default:
		return "notification delivery failed"
	}
}

func min(left, right uint32) uint32 {
	if left < right {
		return left
	}
	return right
}
