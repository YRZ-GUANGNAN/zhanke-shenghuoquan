package trade

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"campus-circle/backend/internal/module/admin/review"
)

type tradeProcessorRepo struct {
	events      []WriteOutbox
	completed   int
	failed      int
	deadletters int
	completeErr error
	retryAt     time.Time
}

func (r *tradeProcessorRepo) FindProductByRequest(context.Context, uint, string) (Product, bool, error) {
	return Product{}, false, nil
}
func (r *tradeProcessorRepo) CreateProductOrReplay(context.Context, *Product, *WriteOutbox) (bool, error) {
	return false, errors.New("not implemented by processor fake")
}
func (r *tradeProcessorRepo) ClaimNextWriteOutbox(_ context.Context, now time.Time, _ time.Duration) (WriteOutboxDelivery, bool, error) {
	if len(r.events) == 0 {
		return WriteOutboxDelivery{}, false, nil
	}
	event := r.events[0]
	r.events = r.events[1:]
	event.Status = WriteOutboxProcessing
	event.AttemptCount++
	event.RowVersion++
	lease := fmt.Sprintf("lease-%d", event.AttemptCount)
	event.LeaseToken = &lease
	expires := now.Add(time.Minute)
	event.LeaseExpiresAt = &expires
	return WriteOutboxDelivery{Event: event, LeaseToken: lease}, true, nil
}
func (r *tradeProcessorRepo) CompleteWriteOutbox(context.Context, WriteOutboxDelivery, time.Time) error {
	if r.completeErr != nil {
		return r.completeErr
	}
	r.completed++
	return nil
}
func (r *tradeProcessorRepo) FailWriteOutbox(_ context.Context, delivery WriteOutboxDelivery, _ string, retryAt time.Time, maxAttempts uint32, _ time.Time) (WriteOutboxStatus, error) {
	r.failed++
	r.retryAt = retryAt
	if delivery.Event.AttemptCount >= maxAttempts {
		r.deadletters++
		return WriteOutboxDeadletter, nil
	}
	event := delivery.Event
	event.Status = WriteOutboxRetryWait
	event.RowVersion++
	event.LeaseToken = nil
	event.LeaseExpiresAt = nil
	r.events = append(r.events, event)
	return WriteOutboxRetryWait, nil
}
func (r *tradeProcessorRepo) DeadletterWriteOutbox(context.Context, WriteOutboxDelivery, string, time.Time) error {
	r.deadletters++
	return nil
}

type tradeProcessorMedia struct {
	calls   int
	effects map[string]bool
	err     error
}

func (m *tradeProcessorMedia) EnsureCommitted(_ context.Context, _ uint, token, referenceType string, referenceID uint64, assets []string) error {
	m.calls++
	if m.err != nil {
		return m.err
	}
	key := fmt.Sprintf("%s/%s/%d/%v", token, referenceType, referenceID, assets)
	m.effects[key] = true
	return nil
}

type tradeProcessorReviews struct {
	calls    int
	effects  map[string]bool
	failOnce bool
}

func (r *tradeProcessorReviews) EnsureSubmitted(_ context.Context, submission review.Submission) (review.Task, error) {
	r.calls++
	if r.failOnce {
		r.failOnce = false
		return review.Task{}, errors.New("temporary review outage")
	}
	key := fmt.Sprintf("%s/%d", submission.ObjectType, submission.ObjectID)
	r.effects[key] = true
	return review.Task{ObjectType: submission.ObjectType, ObjectID: submission.ObjectID}, nil
}

func TestTradeWriteOutboxProcessorRetriesReviewWithoutDuplicatingMedia(t *testing.T) {
	token := "reservation-1"
	repo := &tradeProcessorRepo{events: []WriteOutbox{{
		ID: 1, AggregateType: "product", AggregateID: 21, OwnerUserID: 8,
		EventType: "trade_product_created", ReservationToken: &token,
		Payload: `{"version":"trade-product-create:v1","orderedAssetPublicIds":["asset-a","asset-b"],"reviewObjectType":"product","reviewSubmitterId":8,"reviewRisk":"low"}`,
		Status:  WriteOutboxPending, RowVersion: 1,
	}}}
	media := &tradeProcessorMedia{effects: map[string]bool{}}
	reviews := &tradeProcessorReviews{effects: map[string]bool{}, failOnce: true}
	processor := NewWriteOutboxProcessor(repo, media, reviews)
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	processor.now = func() time.Time { return now }

	if processed, err := processor.DrainOne(context.Background()); err != nil || !processed {
		t.Fatalf("first drain: processed=%v err=%v", processed, err)
	}
	if repo.failed != 1 || repo.completed != 0 || !repo.retryAt.Equal(now.Add(time.Second)) {
		t.Fatalf("unexpected retry state: %+v", repo)
	}
	if processed, err := processor.DrainOne(context.Background()); err != nil || !processed {
		t.Fatalf("second drain: processed=%v err=%v", processed, err)
	}
	if repo.completed != 1 || repo.deadletters != 0 {
		t.Fatalf("unexpected completion state: %+v", repo)
	}
	if media.calls != 2 || len(media.effects) != 1 {
		t.Fatalf("media ensure was not idempotent: calls=%d effects=%d", media.calls, len(media.effects))
	}
	if reviews.calls != 2 || len(reviews.effects) != 1 {
		t.Fatalf("review ensure was not idempotent: calls=%d effects=%d", reviews.calls, len(reviews.effects))
	}
}

func TestTradeWriteOutboxProcessorDeadlettersPermanentFailure(t *testing.T) {
	token := "reservation-2"
	repo := &tradeProcessorRepo{events: []WriteOutbox{{
		ID: 2, AggregateType: "product", AggregateID: 22, OwnerUserID: 8,
		EventType: "trade_product_created", ReservationToken: &token,
		Payload: `{"version":"trade-product-create:v1","orderedAssetPublicIds":["asset-a"],"reviewObjectType":"product","reviewSubmitterId":8,"reviewRisk":"low"}`,
		Status:  WriteOutboxPending, RowVersion: 1,
	}}}
	media := &tradeProcessorMedia{effects: map[string]bool{}, err: fmt.Errorf("%w: asset list mismatch", ErrPermanentWriteOutboxFailure)}
	processor := NewWriteOutboxProcessor(repo, media, &tradeProcessorReviews{effects: map[string]bool{}})
	if processed, err := processor.DrainOne(context.Background()); err != nil || !processed {
		t.Fatalf("drain: processed=%v err=%v", processed, err)
	}
	if repo.deadletters != 1 || repo.failed != 0 || repo.completed != 0 {
		t.Fatalf("permanent error was not deadlettered: %+v", repo)
	}
}

func TestTradeWriteOutboxProcessorPropagatesLeaseCASAndHandlesEmptyQueue(t *testing.T) {
	repo := &tradeProcessorRepo{events: []WriteOutbox{{
		ID: 3, AggregateType: "product", AggregateID: 23, OwnerUserID: 8,
		EventType: "trade_product_created",
		Payload:   `{"version":"trade-product-create:v1","orderedAssetPublicIds":[],"reviewObjectType":"product","reviewSubmitterId":8,"reviewRisk":"low"}`,
		Status:    WriteOutboxPending, RowVersion: 1,
	}}, completeErr: ErrWriteOutboxLease}
	processor := NewWriteOutboxProcessor(repo, nil, &tradeProcessorReviews{effects: map[string]bool{}})
	if processed, err := processor.DrainOne(context.Background()); !processed || !errors.Is(err, ErrWriteOutboxLease) {
		t.Fatalf("lease CAS was not propagated: processed=%v err=%v", processed, err)
	}
	repo.completeErr = nil
	if processed, err := processor.DrainOne(context.Background()); err != nil || processed {
		t.Fatalf("empty queue is not safe: processed=%v err=%v", processed, err)
	}
}
