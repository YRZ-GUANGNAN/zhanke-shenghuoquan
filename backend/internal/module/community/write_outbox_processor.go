package community

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"campus-circle/backend/internal/module/admin/review"
)

var ErrPermanentWriteOutboxFailure = errors.New("community write outbox permanent failure")

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

type communityWriteOutboxPayload struct {
	Version               string   `json:"version"`
	ReferenceType         string   `json:"referenceType"`
	ReferenceIDSource     string   `json:"referenceIdSource"`
	OrderedAssetPublicIDs []string `json:"orderedAssetPublicIds"`
	ReviewTargetType      string   `json:"reviewTargetType"`
	ReviewRisk            string   `json:"reviewRisk"`
}

func (p *WriteOutboxProcessor) deliver(ctx context.Context, event WriteOutbox) error {
	payload, err := validateCommunityWriteOutbox(event)
	if err != nil {
		return err
	}
	ownerID := uint(event.OwnerUserID)
	objectID := uint(event.AggregateID)
	if uint64(ownerID) != event.OwnerUserID || uint64(objectID) != event.AggregateID {
		return permanentCommunityOutboxError("owner or aggregate id overflows uint")
	}
	if len(payload.OrderedAssetPublicIDs) > 0 {
		if p.media == nil {
			return errors.New("community write outbox media ensurer is unavailable")
		}
		if err := p.media.EnsureCommitted(ctx, ownerID, *event.ReservationToken, payload.ReferenceType, event.AggregateID, append([]string(nil), payload.OrderedAssetPublicIDs...)); err != nil {
			return fmt.Errorf("ensure community media committed: %w", err)
		}
	}
	if p.reviews == nil {
		return errors.New("community write outbox review ensurer is unavailable")
	}
	_, err = p.reviews.EnsureSubmitted(ctx, review.Submission{
		ObjectType: payload.ReviewTargetType, ObjectID: objectID, SubmitterID: ownerID, Risk: payload.ReviewRisk,
	})
	if err != nil {
		return fmt.Errorf("ensure community review submitted: %w", err)
	}
	return nil
}

func validateCommunityWriteOutbox(event WriteOutbox) (communityWriteOutboxPayload, error) {
	var payload communityWriteOutboxPayload
	if event.AggregateID == 0 || event.OwnerUserID == 0 || !json.Valid([]byte(event.Payload)) {
		return payload, permanentCommunityOutboxError("invalid coordinates or JSON payload")
	}
	if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
		return payload, permanentCommunityOutboxError("decode payload: %v", err)
	}
	if payload.ReferenceIDSource != "aggregateId" || !validOrderedCommunityAssetIDs(payload.OrderedAssetPublicIDs) {
		return payload, permanentCommunityOutboxError("invalid reference coordinates or ordered asset ids")
	}
	switch event.EventType {
	case "community_post_created":
		if event.AggregateType != "post" || payload.Version != "community-post-created:v1" || payload.ReferenceType != "post" || payload.ReviewTargetType != "post" {
			return payload, permanentCommunityOutboxError("post event and payload disagree")
		}
	case "community_comment_created":
		if event.AggregateType != "comment" || payload.Version != "community-comment-created:v1" || payload.ReferenceType != "" || payload.ReviewTargetType != "comment" || len(payload.OrderedAssetPublicIDs) != 0 {
			return payload, permanentCommunityOutboxError("comment event and payload disagree")
		}
	default:
		return payload, permanentCommunityOutboxError("unsupported event type %q", event.EventType)
	}
	if len(payload.OrderedAssetPublicIDs) > 0 {
		if event.ReservationToken == nil || *event.ReservationToken == "" {
			return payload, permanentCommunityOutboxError("media assets require a reservation token")
		}
	} else if event.ReservationToken != nil && *event.ReservationToken != "" {
		return payload, permanentCommunityOutboxError("reservation token has no media assets")
	}
	return payload, nil
}

func validOrderedCommunityAssetIDs(values []string) bool {
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

func permanentCommunityOutboxError(format string, args ...any) error {
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
