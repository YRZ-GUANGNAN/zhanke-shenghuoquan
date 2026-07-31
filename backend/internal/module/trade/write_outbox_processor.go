package trade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"campus-circle/backend/internal/module/admin/review"
)

var ErrPermanentWriteOutboxFailure = errors.New("trade write outbox permanent failure")

type MediaEnsurer interface {
	EnsureCommitted(context.Context, uint, string, string, uint64, []string) error
}

type ReviewEnsurer interface {
	EnsureSubmitted(context.Context, review.Submission) (review.Task, error)
}

type WriteOutboxProcessor struct {
	repo         WriteContractRepository
	media        MediaEnsurer
	reviews      ReviewEnsurer
	now          func() time.Time
	leaseTTL     time.Duration
	maxAttempts  uint32
	retryBase    time.Duration
	retryMax     time.Duration
	pollInterval time.Duration
}

func NewWriteOutboxProcessor(repo WriteContractRepository, media MediaEnsurer, reviews ReviewEnsurer) *WriteOutboxProcessor {
	return &WriteOutboxProcessor{
		repo: repo, media: media, reviews: reviews, now: func() time.Time { return time.Now().UTC() },
		leaseTTL: time.Minute, maxAttempts: 8, retryBase: time.Second, retryMax: 5 * time.Minute,
		pollInterval: time.Second,
	}
}

func (p *WriteOutboxProcessor) DrainOne(ctx context.Context) (bool, error) {
	if p == nil || p.repo == nil || p.now == nil || p.leaseTTL <= 0 || p.maxAttempts == 0 {
		return false, ErrWriteContractInvalid
	}
	now := p.now().UTC()
	delivery, found, err := p.repo.ClaimNextWriteOutbox(ctx, now, p.leaseTTL)
	if err != nil || !found {
		return found, err
	}
	if err := p.deliver(ctx, delivery.Event); err != nil {
		finishedAt := p.now().UTC()
		if errors.Is(err, ErrPermanentWriteOutboxFailure) {
			return true, p.repo.DeadletterWriteOutbox(ctx, delivery, err.Error(), finishedAt)
		}
		_, failErr := p.repo.FailWriteOutbox(ctx, delivery, err.Error(), finishedAt.Add(p.retryDelay(delivery.Event.AttemptCount)), p.maxAttempts, finishedAt)
		return true, failErr
	}
	return true, p.repo.CompleteWriteOutbox(ctx, delivery, p.now().UTC())
}

func (p *WriteOutboxProcessor) Run(ctx context.Context) error {
	if p == nil || p.pollInterval <= 0 {
		return ErrWriteContractInvalid
	}
	for {
		processed, err := p.DrainOne(ctx)
		if err != nil {
			return err
		}
		if processed {
			continue
		}
		timer := time.NewTimer(p.pollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

type tradeWriteOutboxPayload struct {
	Version               string   `json:"version"`
	OrderedAssetPublicIDs []string `json:"orderedAssetPublicIds"`
	ReviewObjectType      string   `json:"reviewObjectType"`
	ReviewSubmitterID     uint     `json:"reviewSubmitterId"`
	ReviewRisk            string   `json:"reviewRisk"`
}

func (p *WriteOutboxProcessor) deliver(ctx context.Context, event WriteOutbox) error {
	payload, err := validateTradeWriteOutbox(event)
	if err != nil {
		return err
	}
	ownerID := uint(event.OwnerUserID)
	objectID := uint(event.AggregateID)
	if uint64(ownerID) != event.OwnerUserID || uint64(objectID) != event.AggregateID || payload.ReviewSubmitterID != ownerID {
		return permanentTradeOutboxError("owner, submitter, or aggregate id disagree")
	}
	if len(payload.OrderedAssetPublicIDs) > 0 {
		if p.media == nil {
			return errors.New("trade write outbox media ensurer is unavailable")
		}
		if err := p.media.EnsureCommitted(ctx, ownerID, *event.ReservationToken, "product", event.AggregateID, append([]string(nil), payload.OrderedAssetPublicIDs...)); err != nil {
			return fmt.Errorf("ensure trade media committed: %w", err)
		}
	}
	if p.reviews == nil {
		return errors.New("trade write outbox review ensurer is unavailable")
	}
	_, err = p.reviews.EnsureSubmitted(ctx, review.Submission{
		ObjectType: payload.ReviewObjectType, ObjectID: objectID, SubmitterID: ownerID, Risk: payload.ReviewRisk,
	})
	if err != nil {
		return fmt.Errorf("ensure trade review submitted: %w", err)
	}
	return nil
}

func validateTradeWriteOutbox(event WriteOutbox) (tradeWriteOutboxPayload, error) {
	var payload tradeWriteOutboxPayload
	if event.AggregateID == 0 || event.OwnerUserID == 0 || !json.Valid([]byte(event.Payload)) {
		return payload, permanentTradeOutboxError("invalid event coordinates or JSON payload")
	}
	switch event.EventType {
	case "trade_product_created":
		return validateProductCreatedEvent(event)
	case ProductCommentCreatedEventType:
		return validateProductCommentCreatedEvent(event)
	default:
		return payload, permanentTradeOutboxError("unsupported trade write event")
	}
}

func validateProductCreatedEvent(event WriteOutbox) (tradeWriteOutboxPayload, error) {
	var payload tradeWriteOutboxPayload
	if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
		return payload, permanentTradeOutboxError("decode payload: %v", err)
	}
	if event.AggregateType != "product" ||
		payload.Version != "trade-product-create:v1" ||
		payload.ReviewObjectType != "product" ||
		!validOrderedTradeAssetIDs(payload.OrderedAssetPublicIDs) {
		return payload, permanentTradeOutboxError("product event and payload disagree")
	}
	if len(payload.OrderedAssetPublicIDs) > 0 {
		if event.ReservationToken == nil || *event.ReservationToken == "" {
			return payload, permanentTradeOutboxError("media assets require a reservation token")
		}
	} else if event.ReservationToken != nil && *event.ReservationToken != "" {
		return payload, permanentTradeOutboxError("reservation token has no media assets")
	}
	return payload, nil
}

func validateProductCommentCreatedEvent(event WriteOutbox) (tradeWriteOutboxPayload, error) {
	var payload tradeWriteOutboxPayload
	if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
		return payload, permanentTradeOutboxError("decode payload: %v", err)
	}
	if event.AggregateType != ProductCommentAggregateType ||
		payload.Version != "trade-product-comment:v1" ||
		payload.ReviewObjectType != ProductCommentAggregateType ||
		len(payload.OrderedAssetPublicIDs) != 0 ||
		event.ReservationToken != nil {
		return payload, permanentTradeOutboxError("product comment event and payload disagree")
	}
	return payload, nil
}

func validOrderedTradeAssetIDs(values []string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			return false
		}
		if _, exists := seen[value]; exists {
			return false
		}
		seen[value] = struct{}{}
	}
	return true
}

func permanentTradeOutboxError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrPermanentWriteOutboxFailure, fmt.Sprintf(format, args...))
}

func (p *WriteOutboxProcessor) retryDelay(attempt uint32) time.Duration {
	delay := p.retryBase
	if delay <= 0 {
		delay = time.Second
	}
	maximum := p.retryMax
	if maximum < delay {
		maximum = delay
	}
	for current := uint32(1); current < attempt && delay < maximum; current++ {
		if delay > maximum/2 {
			return maximum
		}
		delay *= 2
	}
	if delay > maximum {
		return maximum
	}
	return delay
}
