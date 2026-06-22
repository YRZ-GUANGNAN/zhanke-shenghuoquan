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

type projectionStub struct{ value coremedia.ReviewProjection }

func (s *projectionStub) EnsureProjection(context.Context, string) (coremedia.ReviewProjection, bool, error) {
	return s.value, true, nil
}

type outboxStub struct{ value *coremedia.OutboxEvent }

func (s *outboxStub) AppendOutbox(_ context.Context, value *coremedia.OutboxEvent) error {
	s.value = value
	return nil
}

type consumerStub struct {
	result coremedia.ConsumerBatchResult
	err    error
}

func (s *consumerStub) ConsumeBatch(context.Context, int) (coremedia.ConsumerBatchResult, error) {
	return s.result, s.err
}

type statusStub struct{ fact StatusFact }

func (s *statusStub) FindStatus(context.Context, uint64, string) (StatusFact, error) {
	return s.fact, nil
}
func (s *statusStub) ListStatus(context.Context, uint64, int, int) ([]StatusFact, int64, error) {
	return []StatusFact{s.fact}, 1, nil
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
	if outbox.value == nil || outbox.value.Operation != coremedia.ConsumerOperationSubmit || outbox.value.AggregateID != projectionID {
		t.Fatalf("review submission event missing: %#v", outbox.value)
	}
	var payload coremedia.ConsumerEventPayload
	if err := json.Unmarshal(outbox.value.Payload, &payload); err != nil || payload.ReviewTaskID != 41 || payload.ExpectedRowVersion != 1 {
		t.Fatalf("payload=%#v err=%v", payload, err)
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
