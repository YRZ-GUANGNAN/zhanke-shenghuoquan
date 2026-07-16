package miniappflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
)

type ticketStub struct {
	bundle        coremedia.UploadBundle
	authorization coremedia.UploadAuthorization
	createErr     error
	markCalls     int
}

func (s *ticketStub) CreateTicket(context.Context, coremedia.CreateUploadTicketInput) (coremedia.UploadBundle, bool, error) {
	return s.bundle, true, s.createErr
}
func (s *ticketStub) IssueAuthorization(context.Context, uint64, string, uint64) (coremedia.UploadAuthorization, error) {
	return s.authorization, nil
}
func (s *ticketStub) MarkUploaded(context.Context, uint64, string, uint64) (coremedia.UploadBundle, error) {
	s.markCalls++
	s.bundle.Ticket.Status = coremedia.TicketUploaded
	s.bundle.Ticket.RowVersion++
	return s.bundle, nil
}

type bundleStub struct{ bundle coremedia.UploadBundle }

func (s *bundleStub) FindUploadBundle(context.Context, string, uint64) (coremedia.UploadBundle, error) {
	return s.bundle, nil
}

type completionStub struct {
	asset coremedia.Asset
	calls int
}

func (s *completionStub) CompleteUpload(context.Context, uint64, string) (coremedia.Asset, error) {
	s.calls++
	return s.asset, nil
}

type assetStub struct{ calls int }

func (s *assetStub) TransitionTechnical(context.Context, uint64, uint64, coremedia.TechnicalStatus, coremedia.TechnicalStatus) error {
	s.calls++
	return nil
}

type projectionStub struct {
	value                   coremedia.ReviewProjection
	findByReviewTaskIDCalls int
}

func (s *projectionStub) EnsureProjection(context.Context, string) (coremedia.ReviewProjection, bool, error) {
	return s.value, true, nil
}
func (s *projectionStub) Find(context.Context, string) (coremedia.ReviewProjection, error) {
	return s.value, nil
}
func (s *projectionStub) FindByReviewTaskID(context.Context, uint64) (coremedia.ReviewProjection, error) {
	s.findByReviewTaskIDCalls++
	return s.value, nil
}

type outboxStub struct{ values []*coremedia.OutboxEvent }

func (s *outboxStub) AppendOutbox(_ context.Context, value *coremedia.OutboxEvent) error {
	s.values = append(s.values, value)
	return nil
}

func (s *outboxStub) last() *coremedia.OutboxEvent {
	if len(s.values) == 0 {
		return nil
	}
	return s.values[len(s.values)-1]
}

type consumerStub struct {
	result    coremedia.ConsumerBatchResult
	err       error
	onConsume func()
	calls     int
}

func (s *consumerStub) ConsumeBatch(context.Context, int) (coremedia.ConsumerBatchResult, error) {
	s.calls++
	if s.onConsume != nil {
		s.onConsume()
	}
	return s.result, s.err
}

type statusStub struct{ fact StatusFact }

func (s *statusStub) FindStatus(context.Context, uint64, string) (StatusFact, error) {
	return s.fact, nil
}
func (s *statusStub) ListStatus(context.Context, uint64, int, int) ([]StatusFact, int64, error) {
	return []StatusFact{s.fact}, 1, nil
}

type wechatReviewSubmitterStub struct {
	input     WechatReviewSubmitInput
	submitted bool
	response  WechatReviewSubmission
	err       error
}

func (s *wechatReviewSubmitterStub) SubmitMediaReview(_ context.Context, input WechatReviewSubmitInput) (WechatReviewSubmission, error) {
	s.input = input
	s.submitted = true
	if s.err != nil {
		return WechatReviewSubmission{}, s.err
	}
	return s.response, nil
}

func TestCreateUploadReturnsOnlyBoundSinglePUTContract(t *testing.T) {
	assetID, ticketID := testULID(t), testULID(t)
	now := time.Now().UTC()
	tickets := &ticketStub{
		bundle: coremedia.UploadBundle{
			Asset:  coremedia.Asset{AssetPublicID: assetID},
			Ticket: coremedia.UploadTicket{TicketID: ticketID, Status: coremedia.TicketCreated, RowVersion: 1},
		},
		authorization: coremedia.UploadAuthorization{
			TicketID: ticketID, AssetPublicID: assetID, UploadURL: "https://example.cos/upload?signature=redacted",
			RequiredHeaders: coremedia.RequiredUploadHeaders{
				ContentType: "image/png", ContentMD5: "1B2M2Y8AsgTpgAmY7PhCfg==", ContentLength: 68, ForbidOverwrite: "true",
			},
			ExpiresAt: now.Add(time.Minute),
		},
	}
	service := newStubService(t, tickets, nil)
	value, err := service.CreateUpload(context.Background(), CreateUploadInput{
		OwnerID: 7, Purpose: coremedia.PurposePost, SizeBytes: 68,
		ContentMD5: "1B2M2Y8AsgTpgAmY7PhCfg==", ContentType: "image/png", IdempotencyKey: "attempt-1",
	})
	if err != nil || value.State != StateWaitingUpload || value.Headers.ContentLength != 68 {
		t.Fatalf("value=%#v err=%v", value, err)
	}
	raw, _ := json.Marshal(value)
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"secretid", "secretkey", "sessioncredential", "sessiontoken", "tmpsecret"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("sensitive credential field leaked: %s", forbidden)
		}
	}
}

func TestCreateUploadDoesNotReissueAuthorization(t *testing.T) {
	tickets := &ticketStub{bundle: coremedia.UploadBundle{Ticket: coremedia.UploadTicket{Status: coremedia.TicketAuthorizationIssued}}}
	service := newStubService(t, tickets, nil)
	_, err := service.CreateUpload(context.Background(), CreateUploadInput{OwnerID: 7})
	if !errors.Is(err, ErrAuthorizationConsumed) {
		t.Fatalf("expected consumed authorization, got %v", err)
	}
}

func TestCompleteUploadUsesFrozenServicesAndEnqueuesReview(t *testing.T) {
	assetID, ticketID, projectionID := testULID(t), testULID(t), testULID(t)
	now := time.Now().UTC()
	bundle := coremedia.UploadBundle{
		Asset:  coremedia.Asset{ID: 41, AssetPublicID: assetID, OwnerID: 7, StorageStage: coremedia.StorageStageStaging, TechnicalStatus: coremedia.TechnicalPendingUpload, RowVersion: 1},
		Ticket: coremedia.UploadTicket{TicketID: ticketID, OwnerID: 7, Status: coremedia.TicketAuthorizationIssued, RowVersion: 2},
	}
	tickets := &ticketStub{bundle: bundle}
	completion := &completionStub{asset: coremedia.Asset{
		ID: 41, AssetPublicID: assetID, OwnerID: 7, StorageStage: coremedia.StorageStageFinal,
		TechnicalStatus: coremedia.TechnicalFinalReady,
	}}
	assets := &assetStub{}
	projection := &projectionStub{value: coremedia.ReviewProjection{ProjectionID: projectionID, AssetID: 41, Status: coremedia.ProjectionPending, RowVersion: 1}}
	outbox := &outboxStub{}
	consumer := &consumerStub{result: coremedia.ConsumerBatchResult{Claimed: 1, Succeeded: 1}}
	statuses := &statusStub{fact: StatusFact{
		AssetID: 41, AssetPublicID: assetID, OwnerID: 7, Purpose: coremedia.PurposePost,
		StorageStage: coremedia.StorageStageFinal, TechnicalStatus: coremedia.TechnicalFinalReady,
		TicketStatus: coremedia.TicketConsumed, TicketID: ticketID, Projection: pointer(coremedia.ProjectionSubmitted), UpdatedAt: now,
	}}
	service, err := NewService(tickets, &bundleStub{bundle: bundle}, completion, assets, projection, outbox, consumer, statuses, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	value, err := service.CompleteUpload(context.Background(), 7, ticketID)
	if err != nil || value.State != StateReviewPending || value.CanUse || tickets.markCalls != 1 || assets.calls != 1 || completion.calls != 1 {
		t.Fatalf("value=%#v marks=%d assets=%d completion=%d err=%v", value, tickets.markCalls, assets.calls, completion.calls, err)
	}
	submission := outbox.last()
	if submission == nil || submission.Operation != coremedia.ConsumerOperationSubmit || submission.AggregateID != projectionID {
		t.Fatalf("review submission event missing: %#v", submission)
	}
	var payload coremedia.ConsumerEventPayload
	if err := json.Unmarshal(submission.Payload, &payload); err != nil || payload.ReviewTaskID != 41 || payload.ExpectedRowVersion != 1 {
		t.Fatalf("payload=%#v err=%v", payload, err)
	}
}

func TestCompleteUploadLocalMockAutoApprovesThroughProjectionEvents(t *testing.T) {
	assetID, ticketID, projectionID := testULID(t), testULID(t), testULID(t)
	now := time.Now().UTC()
	bundle := coremedia.UploadBundle{
		Asset:  coremedia.Asset{ID: 41, AssetPublicID: assetID, OwnerID: 7, StorageStage: coremedia.StorageStageStaging, TechnicalStatus: coremedia.TechnicalPendingUpload, RowVersion: 1},
		Ticket: coremedia.UploadTicket{TicketID: ticketID, OwnerID: 7, Status: coremedia.TicketAuthorizationIssued, RowVersion: 2},
	}
	tickets := &ticketStub{bundle: bundle}
	completion := &completionStub{asset: coremedia.Asset{
		ID: 41, AssetPublicID: assetID, OwnerID: 7, StorageStage: coremedia.StorageStageFinal,
		TechnicalStatus: coremedia.TechnicalFinalReady,
	}}
	projection := &projectionStub{value: coremedia.ReviewProjection{ProjectionID: projectionID, AssetID: 41, Status: coremedia.ProjectionPending, RowVersion: 1}}
	outbox := &outboxStub{}
	consumer := &consumerStub{result: coremedia.ConsumerBatchResult{Claimed: 1, Succeeded: 1}, onConsume: func() {
		last := outbox.last()
		if last == nil {
			return
		}
		switch last.Operation {
		case coremedia.ConsumerOperationSubmit:
			projection.value.Status = coremedia.ProjectionSubmitted
			projection.value.ReviewTaskID = pointer(uint64(41))
			projection.value.RowVersion = 2
		case coremedia.ConsumerOperationReviewing:
			projection.value.Status = coremedia.ProjectionReviewing
			projection.value.RowVersion = 3
		case coremedia.ConsumerOperationDecision:
			projection.value.Status = coremedia.ProjectionApproved
			projection.value.AppliedDecisionVersion = 1
			projection.value.RowVersion = 4
		}
	}}
	statuses := &statusStub{fact: StatusFact{
		AssetID: 41, AssetPublicID: assetID, OwnerID: 7, Purpose: coremedia.PurposeDish,
		StorageStage: coremedia.StorageStageFinal, TechnicalStatus: coremedia.TechnicalFinalReady,
		TicketStatus: coremedia.TicketConsumed, TicketID: ticketID, Projection: pointer(coremedia.ProjectionApproved), UpdatedAt: now,
	}}
	service, err := NewService(
		tickets, &bundleStub{bundle: bundle}, completion, &assetStub{}, projection,
		outbox, consumer, statuses, func() time.Time { return now }, WithLocalMockAutoApproval(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	value, err := service.CompleteUpload(context.Background(), 7, ticketID)
	if err != nil || value.State != StateApproved || !value.CanUse {
		t.Fatalf("value=%#v err=%v", value, err)
	}
	if len(outbox.values) != 3 {
		t.Fatalf("expected submit+reviewing+decision events, got %d", len(outbox.values))
	}
	if outbox.values[1].Operation != coremedia.ConsumerOperationReviewing || outbox.values[1].AggregateVersion != 2 {
		t.Fatalf("unexpected reviewing event: %#v", outbox.values[1])
	}
	var decision coremedia.ConsumerEventPayload
	if err := json.Unmarshal(outbox.values[2].Payload, &decision); err != nil {
		t.Fatal(err)
	}
	if outbox.values[2].Operation != coremedia.ConsumerOperationDecision || outbox.values[2].AggregateVersion != 3 ||
		decision.ExpectedStatus != coremedia.ProjectionReviewing || decision.Decision != coremedia.ProjectionApproved ||
		decision.DecisionVersion != 1 || !strings.EqualFold(decision.PayloadHash, localMockDecisionHash(projectionID, 41)) {
		t.Fatalf("unexpected decision payload=%#v event=%#v", decision, outbox.values[2])
	}
}

func TestCompleteUploadWithOpenIDSubmitsRealWechatReview(t *testing.T) {
	assetID, ticketID, projectionID := testULID(t), testULID(t), testULID(t)
	finalKey, finalVersion := "final/media.webp", "version-1"
	now := time.Now().UTC()
	bundle := coremedia.UploadBundle{
		Asset:  coremedia.Asset{ID: 41, AssetPublicID: assetID, OwnerID: 7, StorageStage: coremedia.StorageStageStaging, TechnicalStatus: coremedia.TechnicalPendingUpload, RowVersion: 1},
		Ticket: coremedia.UploadTicket{TicketID: ticketID, OwnerID: 7, Status: coremedia.TicketAuthorizationIssued, RowVersion: 2},
	}
	tickets := &ticketStub{bundle: bundle}
	completion := &completionStub{asset: coremedia.Asset{
		ID: 41, AssetPublicID: assetID, OwnerID: 7, Purpose: coremedia.PurposeDish,
		StorageStage: coremedia.StorageStageFinal, TechnicalStatus: coremedia.TechnicalFinalReady,
		FinalKey: &finalKey, FinalVersionID: &finalVersion,
	}}
	projection := &projectionStub{value: coremedia.ReviewProjection{ProjectionID: projectionID, AssetID: 41, Status: coremedia.ProjectionPending, RowVersion: 1}}
	outbox := &outboxStub{}
	consumer := &consumerStub{result: coremedia.ConsumerBatchResult{Claimed: 1, Succeeded: 1}, onConsume: func() {
		last := outbox.last()
		if last == nil {
			return
		}
		switch last.Operation {
		case coremedia.ConsumerOperationSubmit:
			projection.value.Status = coremedia.ProjectionSubmitted
			projection.value.ReviewTaskID = pointer(uint64(9001))
			projection.value.RowVersion = 2
		case coremedia.ConsumerOperationReviewing:
			projection.value.Status = coremedia.ProjectionReviewing
			projection.value.RowVersion = 3
		}
	}}
	statuses := &statusStub{fact: StatusFact{
		AssetID: 41, AssetPublicID: assetID, OwnerID: 7, Purpose: coremedia.PurposeDish,
		StorageStage: coremedia.StorageStageFinal, TechnicalStatus: coremedia.TechnicalFinalReady,
		TicketStatus: coremedia.TicketConsumed, TicketID: ticketID, Projection: pointer(coremedia.ProjectionReviewing), UpdatedAt: now,
	}}
	wechat := &wechatReviewSubmitterStub{response: WechatReviewSubmission{TraceID: "trace-9001", ReviewTaskID: 9001}}
	service, err := NewService(
		tickets, &bundleStub{bundle: bundle}, completion, &assetStub{}, projection,
		outbox, consumer, statuses, func() time.Time { return now }, WithWechatReviewSubmitter(wechat),
	)
	if err != nil {
		t.Fatal(err)
	}
	value, err := service.CompleteUploadWithOpenID(context.Background(), 7, "openid-7", ticketID)
	if err != nil || value.State != StateReviewPending || !wechat.submitted {
		t.Fatalf("value=%#v submitted=%v err=%v", value, wechat.submitted, err)
	}
	if wechat.input.OpenID != "openid-7" || wechat.input.FinalKey != finalKey || wechat.input.FinalVersionID != finalVersion {
		t.Fatalf("unexpected wechat input: %#v", wechat.input)
	}
	if len(outbox.values) != 2 || outbox.values[0].Operation != coremedia.ConsumerOperationSubmit ||
		outbox.values[1].Operation != coremedia.ConsumerOperationReviewing {
		t.Fatalf("expected submit+reviewing events, got %#v", outbox.values)
	}
	var submit coremedia.ConsumerEventPayload
	if err := json.Unmarshal(outbox.values[0].Payload, &submit); err != nil || submit.ReviewTaskID != 9001 {
		t.Fatalf("submit payload=%#v err=%v", submit, err)
	}
}

func TestApplyWechatReviewResultRecordsDecision(t *testing.T) {
	traceID := "wechat-trace-decision"
	reviewTaskID, err := WechatReviewTaskID(traceID)
	if err != nil {
		t.Fatal(err)
	}
	projectionID := testULID(t)
	projection := &projectionStub{value: coremedia.ReviewProjection{
		ProjectionID: projectionID, AssetID: 41, Status: coremedia.ProjectionReviewing,
		ReviewTaskID: pointer(reviewTaskID), RowVersion: 3,
	}}
	outbox := &outboxStub{}
	consumer := &consumerStub{result: coremedia.ConsumerBatchResult{Claimed: 1, Succeeded: 1}, onConsume: func() {
		if last := outbox.last(); last != nil && last.Operation == coremedia.ConsumerOperationDecision {
			projection.value.Status = coremedia.ProjectionApproved
			projection.value.AppliedDecisionVersion = 1
			projection.value.RowVersion = 4
		}
	}}
	service, err := NewService(
		&ticketStub{}, &bundleStub{}, &completionStub{}, &assetStub{}, projection,
		outbox, consumer, &statusStub{}, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ApplyWechatReviewResult(context.Background(), traceID, "pass", 100); err != nil {
		t.Fatal(err)
	}
	if len(outbox.values) != 1 || outbox.values[0].Operation != coremedia.ConsumerOperationDecision {
		t.Fatalf("expected one decision event, got %#v", outbox.values)
	}
	var decision coremedia.ConsumerEventPayload
	if err := json.Unmarshal(outbox.values[0].Payload, &decision); err != nil {
		t.Fatal(err)
	}
	if decision.Decision != coremedia.ProjectionApproved || decision.ReviewTaskID != reviewTaskID ||
		decision.ExpectedStatus != coremedia.ProjectionReviewing || decision.ExpectedRowVersion != 3 ||
		decision.DecisionVersion != 1 || decision.PayloadHash == "" {
		t.Fatalf("unexpected decision payload: %#v", decision)
	}
}

func TestApplyWechatReviewFailureRecordsDownloadErrorFromSubmitted(t *testing.T) {
	testApplyWechatReviewFailureRecordsDecision(
		t, coremedia.ProjectionSubmitted, wechatMediaDownloadFailureCode,
	)
}

func TestApplyWechatReviewFailureRecordsPlatformErrorFromReviewing(t *testing.T) {
	testApplyWechatReviewFailureRecordsDecision(
		t, coremedia.ProjectionReviewing, wechatMediaPlatformFailureCode,
	)
}

func testApplyWechatReviewFailureRecordsDecision(
	t *testing.T,
	startingStatus coremedia.ProjectionStatus,
	failureCode string,
) {
	t.Helper()
	traceID := "wechat-failure-" + string(startingStatus)
	reviewTaskID, err := WechatReviewTaskID(traceID)
	if err != nil {
		t.Fatal(err)
	}
	projection := &projectionStub{value: coremedia.ReviewProjection{
		ProjectionID: testULID(t), AssetID: 41, Status: startingStatus,
		ReviewTaskID: pointer(reviewTaskID), RowVersion: 3,
	}}
	outbox := &outboxStub{}
	consumer := &consumerStub{
		result: coremedia.ConsumerBatchResult{Claimed: 1, Succeeded: 1},
		onConsume: func() {
			projection.value.Status = coremedia.ProjectionReviewFailed
			projection.value.AppliedDecisionVersion = 1
			projection.value.RowVersion = 4
		},
	}
	service, err := NewService(
		&ticketStub{}, &bundleStub{}, &completionStub{}, &assetStub{}, projection,
		outbox, consumer, &statusStub{}, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ApplyWechatReviewFailure(context.Background(), traceID, failureCode); err != nil {
		t.Fatal(err)
	}
	if len(outbox.values) != 1 || outbox.values[0].Operation != coremedia.ConsumerOperationDecision {
		t.Fatalf("expected one decision event, got %#v", outbox.values)
	}
	var decision coremedia.ConsumerEventPayload
	if err := json.Unmarshal(outbox.values[0].Payload, &decision); err != nil {
		t.Fatal(err)
	}
	if decision.Decision != coremedia.ProjectionReviewFailed ||
		decision.ResultCode != failureCode ||
		decision.ReviewTaskID != reviewTaskID ||
		decision.ExpectedStatus != startingStatus ||
		decision.ExpectedRowVersion != 3 ||
		decision.DecisionVersion != 1 || decision.PayloadHash == "" {
		t.Fatalf("unexpected failure decision: %#v", decision)
	}

	if err := service.ApplyWechatReviewFailure(context.Background(), traceID, failureCode); err != nil {
		t.Fatal(err)
	}
	if len(outbox.values) != 1 || consumer.calls != 1 {
		t.Fatalf("duplicate callback appended or consumed work: outbox=%d consumer=%d", len(outbox.values), consumer.calls)
	}
}

func TestApplyWechatReviewFailureIgnoresTerminalProjection(t *testing.T) {
	t.Parallel()
	for _, status := range []coremedia.ProjectionStatus{
		coremedia.ProjectionApproved,
		coremedia.ProjectionRejected,
		coremedia.ProjectionReviewFailed,
	} {
		status := status
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()
			traceID := "wechat-terminal-failure-" + string(status)
			reviewTaskID, err := WechatReviewTaskID(traceID)
			if err != nil {
				t.Fatal(err)
			}
			projection := &projectionStub{value: coremedia.ReviewProjection{
				ProjectionID: testULID(t), AssetID: 41, Status: status,
				ReviewTaskID: pointer(reviewTaskID), AppliedDecisionVersion: 1, RowVersion: 4,
			}}
			outbox := &outboxStub{}
			consumer := &consumerStub{}
			service, err := NewService(
				&ticketStub{}, &bundleStub{}, &completionStub{}, &assetStub{}, projection,
				outbox, consumer, &statusStub{}, nil,
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := service.ApplyWechatReviewFailure(
				context.Background(), traceID, wechatMediaPlatformFailureCode,
			); err != nil {
				t.Fatal(err)
			}
			if len(outbox.values) != 0 || consumer.calls != 0 {
				t.Fatalf("terminal callback appended or consumed work: outbox=%d consumer=%d", len(outbox.values), consumer.calls)
			}
		})
	}
}

func TestApplyWechatReviewFailureRejectsUnknownReason(t *testing.T) {
	t.Parallel()
	projection := &projectionStub{}
	outbox := &outboxStub{}
	consumer := &consumerStub{}
	service, err := NewService(
		&ticketStub{}, &bundleStub{}, &completionStub{}, &assetStub{}, projection,
		outbox, consumer, &statusStub{}, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ApplyWechatReviewFailure(
		context.Background(), "trace-unknown-code", "-1008",
	); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
	if projection.findByReviewTaskIDCalls != 0 || len(outbox.values) != 0 || consumer.calls != 0 {
		t.Fatalf(
			"invalid reason reached dependencies: finds=%d outbox=%d consumer=%d",
			projection.findByReviewTaskIDCalls, len(outbox.values), consumer.calls,
		)
	}
}

func TestNewServiceRejectsLocalMockAndRealWechatTogether(t *testing.T) {
	_, err := NewService(
		&ticketStub{}, &bundleStub{}, &completionStub{}, &assetStub{}, &projectionStub{},
		&outboxStub{}, &consumerStub{}, &statusStub{}, nil,
		WithLocalMockAutoApproval(true),
		WithWechatReviewSubmitter(&wechatReviewSubmitterStub{}),
	)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected invalid request, got %v", err)
	}
}

func TestServerOwnedStatusProjection(t *testing.T) {
	base := StatusFact{AssetPublicID: testULID(t), TicketID: testULID(t), Purpose: coremedia.PurposeAvatar, UpdatedAt: time.Now()}
	tests := []struct {
		name     string
		mutate   func(*StatusFact)
		state    ClientState
		canUse   bool
		recovery RecoveryAction
	}{
		{"waiting", func(v *StatusFact) { v.TicketStatus = coremedia.TicketAuthorizationIssued }, StateWaitingUpload, false, RecoveryComplete},
		{"expired", func(v *StatusFact) { v.TicketStatus = coremedia.TicketExpired }, StateExpired, false, RecoveryNewUpload},
		{"pending review", func(v *StatusFact) {
			v.TicketStatus = coremedia.TicketConsumed
			v.TechnicalStatus = coremedia.TechnicalFinalReady
		}, StateReviewPending, false, RecoveryPoll},
		{"approved", func(v *StatusFact) {
			v.TicketStatus = coremedia.TicketConsumed
			v.TechnicalStatus = coremedia.TechnicalFinalReady
			v.Projection = pointer(coremedia.ProjectionApproved)
		}, StateApproved, true, RecoveryNone},
		{"rejected", func(v *StatusFact) {
			v.TicketStatus = coremedia.TicketConsumed
			v.TechnicalStatus = coremedia.TechnicalFinalReady
			v.Projection = pointer(coremedia.ProjectionRejected)
		}, StateRejected, false, RecoveryNewUpload},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := base
			test.mutate(&value)
			projected := projectStatus(value)
			if projected.State != test.state || projected.CanUse != test.canUse || projected.Recovery != test.recovery {
				t.Fatalf("projected=%#v", projected)
			}
		})
	}
}

func newStubService(t *testing.T, tickets *ticketStub, statuses *statusStub) *Service {
	t.Helper()
	if statuses == nil {
		statuses = &statusStub{}
	}
	value, err := NewService(
		tickets, &bundleStub{}, &completionStub{}, &assetStub{}, &projectionStub{},
		&outboxStub{}, &consumerStub{}, statuses, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func testULID(t *testing.T) string {
	t.Helper()
	value, err := coremedia.NewULID()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func pointer[T any](value T) *T { return &value }
