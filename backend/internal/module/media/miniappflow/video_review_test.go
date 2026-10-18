package miniappflow

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
)

type videoReviewStub struct {
	job   coremedia.VideoReviewJob
	calls int
}

func (r *videoReviewStub) ReviewVideo(context.Context, coremedia.Asset) (coremedia.VideoReviewJob, error) {
	r.calls++
	return r.job, nil
}

func TestVideoReviewUsesCompleteProjectionLifecycle(t *testing.T) {
	asset := coremedia.Asset{AssetPublicID: testULID(t), ID: 41, Purpose: coremedia.PurposePost, VerifiedMIME: "video/mp4", TechnicalStatus: coremedia.TechnicalFinalReady}
	projection := &projectionStub{value: coremedia.ReviewProjection{ProjectionID: testULID(t), AssetID: 41, Status: coremedia.ProjectionPending, RowVersion: 1}}
	outbox, consumer := &outboxStub{}, &consumerStub{}
	reviewer := &videoReviewStub{job: coremedia.VideoReviewJob{AssetPublicID: asset.AssetPublicID, JobID: "cos-job", Decision: coremedia.ProjectionApproved}}
	consumer.onConsume = func() {
		var payload coremedia.ConsumerEventPayload
		if err := json.Unmarshal(outbox.last().Payload, &payload); err != nil {
			t.Fatal(err)
		}
		switch outbox.last().Operation {
		case coremedia.ConsumerOperationSubmit:
			if projection.value.Status != coremedia.ProjectionPending {
				t.Fatal("submission reordered")
			}
			projection.value.Status, projection.value.ReviewTaskID = coremedia.ProjectionSubmitted, &payload.ReviewTaskID
		case coremedia.ConsumerOperationReviewing:
			if projection.value.Status != coremedia.ProjectionSubmitted {
				t.Fatal("reviewing reordered")
			}
			projection.value.Status = coremedia.ProjectionReviewing
		case coremedia.ConsumerOperationDecision:
			if projection.value.Status != coremedia.ProjectionReviewing || payload.ExpectedStatus != coremedia.ProjectionReviewing {
				t.Fatal("approval bypassed reviewing")
			}
			projection.value.Status = payload.Decision
		default:
			t.Fatal("unexpected projection operation")
		}
		projection.value.RowVersion++
	}
	s := &Service{videoReview: reviewer, projections: projection, outbox: outbox, consumer: consumer, now: time.Now}
	if err := s.advanceVideoReview(context.Background(), asset); err != nil {
		t.Fatal(err)
	}
	if projection.value.Status != coremedia.ProjectionApproved || len(outbox.values) != 3 {
		t.Fatal("video did not complete projection")
	}
	if err := s.advanceVideoReview(context.Background(), asset); err != nil || reviewer.calls != 1 {
		t.Fatal("terminal review did not replay")
	}
}

func TestVideoReviewDoesNotBindAnUnsubmittedJob(t *testing.T) {
	outbox := &outboxStub{}
	s := &Service{videoReview: &videoReviewStub{job: coremedia.VideoReviewJob{Decision: coremedia.ProjectionPending}}, projections: &projectionStub{value: coremedia.ReviewProjection{Status: coremedia.ProjectionPending}}, outbox: outbox}
	if err := s.advanceVideoReview(context.Background(), coremedia.Asset{}); err != nil || len(outbox.values) != 0 {
		t.Fatal("unsubmitted cloud job bound to projection")
	}
	s.videoReview = nil
	if err := s.advanceVideoReview(context.Background(), coremedia.Asset{}); !errors.Is(err, coremedia.ErrVideoUnavailable) {
		t.Fatal("unconfigured video review was bypassed")
	}
}

func TestVideoStatusRemainsReadableWhenNewUploadsDisabled(t *testing.T) {
	approved := coremedia.ProjectionApproved
	s := &Service{status: &statusStub{fact: StatusFact{MIMEType: "video/mp4", TechnicalStatus: coremedia.TechnicalFinalReady, StorageStage: coremedia.StorageStageFinal, TicketStatus: coremedia.TicketConsumed, Purpose: coremedia.PurposePost, Projection: &approved}}}
	value, err := s.Status(context.Background(), 1, testULID(t))
	if err != nil || !value.CanUse || value.State != StateApproved || value.Message != "视频审核通过，可以使用" {
		t.Fatalf("approved video unavailable: %+v %v", value, err)
	}
	if _, err := s.CreateUpload(context.Background(), CreateUploadInput{OwnerID: 1, ContentType: "video/mp4"}); !errors.Is(err, coremedia.ErrVideoUnavailable) {
		t.Fatalf("disabled upload not rejected: %v", err)
	}
}
