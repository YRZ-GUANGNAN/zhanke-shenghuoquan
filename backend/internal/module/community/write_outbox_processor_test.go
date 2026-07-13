package community

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"campus-circle/backend/internal/module/admin/review"
)

type communityProcessorRepo struct {
	events      []WriteOutbox
	completed   int
	failed      int
	deadletters int
	completeErr error
	retryAt     time.Time
}

func (r *communityProcessorRepo) FindPostByRequest(context.Context, uint, string) (Post, bool, error) {
	return Post{}, false, nil
}
func (r *communityProcessorRepo) FindCommentByRequest(context.Context, uint, string) (Comment, bool, error) {
	return Comment{}, false, nil
}
func (r *communityProcessorRepo) CreatePostOrReplay(context.Context, *Post, *WriteOutbox) (bool, error) {
	return false, errors.New("not implemented by processor fake")
}
func (r *communityProcessorRepo) CreateCommentOrReplay(context.Context, *Comment, *WriteOutbox) (bool, error) {
	return false, errors.New("not implemented by processor fake")
}
func (r *communityProcessorRepo) ClaimNextWriteOutbox(_ context.Context, now time.Time, _ time.Duration) (WriteOutboxDelivery, bool, error) {
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
func (r *communityProcessorRepo) CompleteWriteOutbox(context.Context, WriteOutboxDelivery, time.Time) error {
	if r.completeErr != nil {
		return r.completeErr
	}
	r.completed++
	return nil
}
func (r *communityProcessorRepo) FailWriteOutbox(_ context.Context, delivery WriteOutboxDelivery, _ string, retryAt time.Time, maxAttempts uint32, _ time.Time) (WriteOutboxStatus, error) {
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
func (r *communityProcessorRepo) DeadletterWriteOutbox(context.Context, WriteOutboxDelivery, string, time.Time) error {
	r.deadletters++
	return nil
}

type communityProcessorMedia struct {
	calls   int
	effects map[string]bool
	err     error
}

func (m *communityProcessorMedia) EnsureCommitted(_ context.Context, _ uint, token, referenceType string, referenceID uint64, assets []string) error {
	m.calls++
	if m.err != nil {
		return m.err
	}
	key := fmt.Sprintf("%s/%s/%d/%v", token, referenceType, referenceID, assets)
	m.effects[key] = true
	return nil
}

type communityProcessorReviews struct {
	calls    int
	effects  map[string]bool
	failOnce bool
}

func (r *communityProcessorReviews) EnsureSubmitted(_ context.Context, submission review.Submission) (review.Task, error) {
	r.calls++
	if r.failOnce {
		r.failOnce = false
		return review.Task{}, errors.New("temporary review outage")
	}
	key := fmt.Sprintf("%s/%d", submission.ObjectType, submission.ObjectID)
	r.effects[key] = true
	return review.Task{ObjectType: submission.ObjectType, ObjectID: submission.ObjectID}, nil
}

func TestCommunityWriteOutboxProcessorRetriesReviewWithoutDuplicatingMedia(t *testing.T) {
	token := "reservation-1"
	repo := &communityProcessorRepo{events: []WriteOutbox{{
		ID: 1, AggregateType: "post", AggregateID: 11, OwnerUserID: 7,
		EventType: "community_post_created", ReservationToken: &token,
		Payload: `{"version":"community-post-created:v1","referenceType":"post","referenceIdSource":"aggregateId","orderedAssetPublicIds":["asset-a","asset-b"],"reviewTargetType":"post","reviewRisk":"low"}`,
		Status:  WriteOutboxPending, RowVersion: 1,
	}}}
	media := &communityProcessorMedia{effects: map[string]bool{}}
	reviews := &communityProcessorReviews{effects: map[string]bool{}, failOnce: true}
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

func TestCommunityWriteOutboxProcessorDeadlettersPermanentInconsistency(t *testing.T) {
	repo := &communityProcessorRepo{events: []WriteOutbox{{
		ID: 2, AggregateType: "post", AggregateID: 12, OwnerUserID: 7,
		EventType: "community_post_created", Payload: `{"version":"wrong"}`, Status: WriteOutboxPending, RowVersion: 1,
	}}}
	processor := NewWriteOutboxProcessor(repo, &communityProcessorMedia{effects: map[string]bool{}}, &communityProcessorReviews{effects: map[string]bool{}})
	if processed, err := processor.DrainOne(context.Background()); err != nil || !processed {
		t.Fatalf("drain: processed=%v err=%v", processed, err)
	}
	if repo.deadletters != 1 || repo.failed != 0 || repo.completed != 0 {
		t.Fatalf("invalid payload was not deadlettered: %+v", repo)
	}
}

func TestCommunityWriteOutboxProcessorPropagatesLeaseCASAndHandlesEmptyQueue(t *testing.T) {
	repo := &communityProcessorRepo{events: []WriteOutbox{{
		ID: 3, AggregateType: "comment", AggregateID: 13, OwnerUserID: 7,
		EventType: "community_comment_created",
		Payload:   `{"version":"community-comment-created:v1","referenceType":"","referenceIdSource":"aggregateId","orderedAssetPublicIds":[],"reviewTargetType":"comment","reviewRisk":"low"}`,
		Status:    WriteOutboxPending, RowVersion: 1,
	}}, completeErr: ErrWriteOutboxLease}
	processor := NewWriteOutboxProcessor(repo, nil, &communityProcessorReviews{effects: map[string]bool{}})
	if processed, err := processor.DrainOne(context.Background()); !processed || !errors.Is(err, ErrWriteOutboxLease) {
		t.Fatalf("lease CAS was not propagated: processed=%v err=%v", processed, err)
	}
	repo.completeErr = nil
	if processed, err := processor.DrainOne(context.Background()); err != nil || processed {
		t.Fatalf("empty queue is not safe: processed=%v err=%v", processed, err)
	}
}
