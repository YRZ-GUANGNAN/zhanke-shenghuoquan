package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

type ReviewProjectionCommands interface {
	Submit(context.Context, string, uint64, uint64) (ReviewProjection, error)
	StartReview(context.Context, string, uint64) (ReviewProjection, error)
	RecordDecision(context.Context, ApplyReviewDecisionInput) (ReviewProjection, error)
}

type MediaConsumerConfig struct {
	LeaseTTL    time.Duration
	RetryDelay  time.Duration
	MaxAttempts uint32
	Now         func() time.Time
}

type MediaConsumer struct {
	repository MediaConsumerRepository
	projection ReviewProjectionCommands
	config     MediaConsumerConfig
}

func NewMediaConsumer(repository MediaConsumerRepository, projection ReviewProjectionCommands, config MediaConsumerConfig) (*MediaConsumer, error) {
	if repository == nil || projection == nil {
		return nil, ErrInvalidRecord
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
	return &MediaConsumer{repository: repository, projection: projection, config: config}, nil
}

func (c *MediaConsumer) ConsumeBatch(ctx context.Context, limit int) (ConsumerBatchResult, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var result ConsumerBatchResult
	for result.Claimed < limit {
		now := c.config.Now().UTC()
		delivery, found, err := c.repository.ClaimNextConsumerEvent(ctx, now, c.config.LeaseTTL)
		if err != nil {
			return result, err
		}
		if !found {
			break
		}
		result.Claimed++
		order, err := c.repository.ClassifyConsumerOrder(ctx, delivery)
		if err != nil {
			return result, err
		}
		switch order {
		case ConsumerOrderDuplicate:
			if err := c.repository.CompleteConsumerEvent(ctx, delivery, now); err != nil {
				return result, err
			}
			result.Duplicates++
			continue
		case ConsumerOrderOutOfOrder:
			status, err := c.repository.FailConsumerEvent(ctx, delivery, "out_of_order", now.Add(c.config.RetryDelay), c.config.MaxAttempts, false, now)
			if err != nil {
				return result, err
			}
			addConsumerFailure(&result, status)
			continue
		case ConsumerOrderConflict:
			status, err := c.repository.FailConsumerEvent(ctx, delivery, "event_conflict", now, c.config.MaxAttempts, true, now)
			if err != nil {
				return result, err
			}
			addConsumerFailure(&result, status)
			continue
		case ConsumerOrderReady:
		default:
			return result, ErrConsumerConflict
		}

		err = c.dispatch(ctx, delivery.Event)
		if err == nil {
			if err := c.repository.CompleteConsumerEvent(ctx, delivery, now); err != nil {
				return result, err
			}
			result.Succeeded++
			continue
		}
		permanent := consumerFailurePermanent(err)
		status, failErr := c.repository.FailConsumerEvent(ctx, delivery, consumerErrorCode(err), now.Add(c.config.RetryDelay), c.config.MaxAttempts, permanent, now)
		if failErr != nil {
			return result, failErr
		}
		addConsumerFailure(&result, status)
	}
	return result, nil
}

func (c *MediaConsumer) dispatch(ctx context.Context, event OutboxEvent) error {
	payload, err := decodeConsumerPayload(event.Payload)
	if err != nil {
		return err
	}
	if event.AggregateID != payload.ProjectionID || !validConsumerPayloadForOperation(event.Operation, payload) {
		return ErrConsumerPayload
	}
	switch event.Operation {
	case ConsumerOperationSubmit:
		_, err = c.projection.Submit(ctx, payload.ProjectionID, payload.ReviewTaskID, payload.ExpectedRowVersion)
	case ConsumerOperationReviewing:
		_, err = c.projection.StartReview(ctx, payload.ProjectionID, payload.ExpectedRowVersion)
	case ConsumerOperationDecision:
		_, err = c.projection.RecordDecision(ctx, ApplyReviewDecisionInput{
			ProjectionID: payload.ProjectionID, ReviewTaskID: payload.ReviewTaskID,
			DecisionVersion: payload.DecisionVersion, ExpectedStatus: payload.ExpectedStatus,
			ExpectedRowVersion: payload.ExpectedRowVersion, Decision: payload.Decision,
			PayloadHash: payload.PayloadHash, ResultCode: payload.ResultCode,
		})
	default:
		return ErrConsumerPayload
	}
	return err
}

func validConsumerPayloadForOperation(operation string, value ConsumerEventPayload) bool {
	switch operation {
	case ConsumerOperationSubmit:
		return value.ReviewTaskID > 0 && value.ExpectedStatus == "" && value.DecisionVersion == 0 &&
			value.Decision == "" && value.PayloadHash == "" && value.ResultCode == ""
	case ConsumerOperationReviewing:
		return value.ReviewTaskID == 0 && value.ExpectedStatus == "" && value.DecisionVersion == 0 &&
			value.Decision == "" && value.PayloadHash == "" && value.ResultCode == ""
	case ConsumerOperationDecision:
		return value.ReviewTaskID > 0 && value.DecisionVersion > 0 &&
			(value.ExpectedStatus == ProjectionSubmitted || value.ExpectedStatus == ProjectionReviewing) &&
			value.Decision.Decision() && validSHA256(strings.ToLower(strings.TrimSpace(value.PayloadHash)))
	default:
		return false
	}
}

func decodeConsumerPayload(raw JSONDocument) (ConsumerEventPayload, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var value ConsumerEventPayload
	if err := decoder.Decode(&value); err != nil {
		return ConsumerEventPayload{}, ErrConsumerPayload
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ConsumerEventPayload{}, ErrConsumerPayload
	}
	if !ValidULID(value.ProjectionID) || value.ExpectedRowVersion == 0 {
		return ConsumerEventPayload{}, ErrConsumerPayload
	}
	return value, nil
}

func consumerFailurePermanent(err error) bool {
	return errors.Is(err, ErrConsumerPayload) || errors.Is(err, ErrInvalidRecord) || errors.Is(err, ErrInvalidULID) ||
		errors.Is(err, ErrInvalidTransition) || errors.Is(err, ErrReviewTaskConflict) ||
		errors.Is(err, ErrDecisionConflict) || errors.Is(err, ErrStaleDecision) || errors.Is(err, ErrReviewInvariant)
}

func consumerErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrConsumerPayload):
		return "invalid_payload"
	case errors.Is(err, ErrConcurrentUpdate):
		return "stale_row_version"
	case errors.Is(err, ErrInvalidTransition):
		return "invalid_transition"
	case errors.Is(err, ErrReviewTaskConflict):
		return "review_task_conflict"
	case errors.Is(err, ErrDecisionConflict):
		return "decision_conflict"
	case errors.Is(err, ErrStaleDecision):
		return "stale_decision"
	case errors.Is(err, ErrNotFound):
		return "projection_not_found"
	default:
		name := strings.TrimSpace(err.Error())
		if name == "" {
			return "consumer_error"
		}
		return "consumer_error"
	}
}

func addConsumerFailure(result *ConsumerBatchResult, status ConsumerStatus) {
	if status == ConsumerDeadletter {
		result.Deadletters++
	} else {
		result.Retried++
	}
}

var _ ReviewProjectionCommands = (*ReviewProjectionService)(nil)
