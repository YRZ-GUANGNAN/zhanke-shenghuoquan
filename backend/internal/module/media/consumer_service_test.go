package media

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type consumerRepositoryStub struct {
	delivery ConsumerDelivery
	found    bool
	order    ConsumerOrder
	complete int
	failed   int
	status   ConsumerStatus
}

func (s *consumerRepositoryStub) ClaimNextConsumerEvent(context.Context, time.Time, time.Duration) (ConsumerDelivery, bool, error) {
	if !s.found {
		return ConsumerDelivery{}, false, nil
	}
	s.found = false
	return s.delivery, true, nil
}
func (s *consumerRepositoryStub) ClassifyConsumerOrder(context.Context, ConsumerDelivery) (ConsumerOrder, error) {
	return s.order, nil
}
func (s *consumerRepositoryStub) CompleteConsumerEvent(context.Context, ConsumerDelivery, time.Time) error {
	s.complete++
	return nil
}
func (s *consumerRepositoryStub) FailConsumerEvent(context.Context, ConsumerDelivery, string, time.Time, uint32, bool, time.Time) (ConsumerStatus, error) {
	s.failed++
	return s.status, nil
}

type reviewProjectionCommandsStub struct {
	submitCalls    int
	reviewingCalls int
	decisionCalls  int
	err            error
}

func (s *reviewProjectionCommandsStub) Submit(context.Context, string, uint64, uint64) (ReviewProjection, error) {
	s.submitCalls++
	return ReviewProjection{}, s.err
}
func (s *reviewProjectionCommandsStub) StartReview(context.Context, string, uint64) (ReviewProjection, error) {
	s.reviewingCalls++
	return ReviewProjection{}, s.err
}
func (s *reviewProjectionCommandsStub) RecordDecision(context.Context, ApplyReviewDecisionInput) (ReviewProjection, error) {
	s.decisionCalls++
	return ReviewProjection{}, s.err
}

func TestMediaConsumerDispatchesOnlyThroughReviewProjectionServiceBoundary(t *testing.T) {
	projectionID, _ := NewULID()
	payload := JSONDocument(`{"projectionId":"` + projectionID + `","reviewTaskId":7001,"expectedRowVersion":1}`)
	repository := &consumerRepositoryStub{
		found: true, order: ConsumerOrderReady,
		delivery: consumerStubDelivery(payload, ConsumerOperationSubmit),
	}
	commands := &reviewProjectionCommandsStub{}
	consumer, _ := NewMediaConsumer(repository, commands, MediaConsumerConfig{})
	result, err := consumer.ConsumeBatch(context.Background(), 1)
	if err != nil || result.Succeeded != 1 || commands.submitCalls != 1 || repository.complete != 1 {
		t.Fatalf("result=%#v submit=%d complete=%d err=%v", result, commands.submitCalls, repository.complete, err)
	}
}

func TestMediaConsumerDuplicateDoesNotInvokeProjectionService(t *testing.T) {
	repository := &consumerRepositoryStub{found: true, order: ConsumerOrderDuplicate, delivery: consumerStubDelivery(JSONDocument(`{}`), ConsumerOperationSubmit)}
	commands := &reviewProjectionCommandsStub{}
	consumer, _ := NewMediaConsumer(repository, commands, MediaConsumerConfig{})
	result, err := consumer.ConsumeBatch(context.Background(), 1)
	if err != nil || result.Duplicates != 1 || repository.complete != 1 || commands.submitCalls+commands.reviewingCalls+commands.decisionCalls != 0 {
		t.Fatalf("result=%#v commands=%#v err=%v", result, commands, err)
	}
}

func TestMediaConsumerRetryAndDeadletterClassification(t *testing.T) {
	projectionID, _ := NewULID()
	payload := JSONDocument(`{"projectionId":"` + projectionID + `","reviewTaskId":7002,"expectedRowVersion":1}`)
	tests := []struct {
		name   string
		order  ConsumerOrder
		err    error
		status ConsumerStatus
	}{
		{name: "out of order", order: ConsumerOrderOutOfOrder, status: ConsumerRetryWait},
		{name: "event conflict", order: ConsumerOrderConflict, status: ConsumerDeadletter},
		{name: "transient stale CAS", order: ConsumerOrderReady, err: ErrConcurrentUpdate, status: ConsumerRetryWait},
		{name: "permanent transition", order: ConsumerOrderReady, err: ErrInvalidTransition, status: ConsumerDeadletter},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &consumerRepositoryStub{found: true, order: test.order, status: test.status, delivery: consumerStubDelivery(payload, ConsumerOperationSubmit)}
			commands := &reviewProjectionCommandsStub{err: test.err}
			consumer, _ := NewMediaConsumer(repository, commands, MediaConsumerConfig{})
			result, err := consumer.ConsumeBatch(context.Background(), 1)
			if err != nil || repository.failed != 1 {
				t.Fatalf("result=%#v failed=%d err=%v", result, repository.failed, err)
			}
			if test.status == ConsumerDeadletter && result.Deadletters != 1 {
				t.Fatalf("deadletter result=%#v", result)
			}
			if test.status == ConsumerRetryWait && result.Retried != 1 {
				t.Fatalf("retry result=%#v", result)
			}
		})
	}
}

func TestDecodeConsumerPayloadRejectsUnknownAndTrailingData(t *testing.T) {
	projectionID, _ := NewULID()
	invalid := []JSONDocument{
		JSONDocument(`{"projectionId":"` + projectionID + `","expectedRowVersion":1,"unknown":true}`),
		JSONDocument(`{"projectionId":"` + projectionID + `","expectedRowVersion":1}{}`),
		JSONDocument(`{"projectionId":"bad","expectedRowVersion":1}`),
	}
	for index, raw := range invalid {
		if _, err := decodeConsumerPayload(raw); !errors.Is(err, ErrConsumerPayload) {
			t.Fatalf("case %d returned %v", index, err)
		}
	}
}

func TestMediaConsumerRejectsAggregateProjectionMismatch(t *testing.T) {
	projectionID, _ := NewULID()
	payload := JSONDocument(`{"projectionId":"` + projectionID + `","reviewTaskId":7003,"expectedRowVersion":1}`)
	delivery := consumerStubDelivery(payload, ConsumerOperationSubmit)
	otherAggregate, _ := NewULID()
	delivery.Event.AggregateID = otherAggregate
	delivery.Inbox.AggregateID = otherAggregate
	repository := &consumerRepositoryStub{found: true, order: ConsumerOrderReady, status: ConsumerDeadletter, delivery: delivery}
	commands := &reviewProjectionCommandsStub{}
	consumer, _ := NewMediaConsumer(repository, commands, MediaConsumerConfig{})
	result, err := consumer.ConsumeBatch(context.Background(), 1)
	if err != nil || result.Deadletters != 1 || commands.submitCalls != 0 {
		t.Fatalf("result=%#v commands=%#v err=%v", result, commands, err)
	}
}

func consumerStubDelivery(payload JSONDocument, operation string) ConsumerDelivery {
	eventID, _ := NewULID()
	aggregateID, _ := NewULID()
	var decoded ConsumerEventPayload
	if json.Unmarshal(payload, &decoded) == nil && ValidULID(decoded.ProjectionID) {
		aggregateID = decoded.ProjectionID
	}
	now := time.Now().UTC()
	event := OutboxEvent{EventID: eventID, AggregateID: aggregateID, AggregateVersion: 1, Operation: operation, Payload: payload, RowVersion: 1, CreatedAt: now, UpdatedAt: now}
	return ConsumerDelivery{
		Event:      event,
		Inbox:      ConsumerInbox{EventID: eventID, AggregateID: aggregateID, AggregateVersion: 1, Operation: operation, PayloadHash: hashConsumerPayload(payload), Status: ConsumerProcessing, AttemptCount: 1, RowVersion: 2},
		LeaseToken: "01J00000000000000000000000",
	}
}
