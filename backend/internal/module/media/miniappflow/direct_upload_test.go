package miniappflow

import (
	"context"
	"errors"
	"testing"

	coremedia "campus-circle/backend/internal/module/media"
)

func TestDirectPurposeCompletionDoesNotSubmitCloudReview(t *testing.T) {
	for _, purpose := range []coremedia.Purpose{coremedia.PurposeAvatar, coremedia.PurposeProfileCover, coremedia.PurposeIdentityApplication, coremedia.PurposeDish, coremedia.PurposeMerchantLogo} {
		t.Run(string(purpose), func(t *testing.T) {
			assetID, ticketID, projectionID := testULID(t), testULID(t), testULID(t)
			asset := coremedia.Asset{ID: 41, OwnerID: 7, AssetPublicID: assetID, Purpose: purpose, TechnicalStatus: coremedia.TechnicalFinalReady, StorageStage: coremedia.StorageStageFinal}
			bundle := coremedia.UploadBundle{Asset: asset, Ticket: coremedia.UploadTicket{TicketID: ticketID, OwnerID: 7, Status: coremedia.TicketConsumed}}
			projection := &projectionStub{value: coremedia.ReviewProjection{ProjectionID: projectionID, AssetID: 41, Status: coremedia.ProjectionNotRequired, RowVersion: 1}}
			wechat := &wechatReviewSubmitterStub{err: errors.New("cloud service is offline")}
			outbox, consumer := &outboxStub{}, &consumerStub{}
			statuses := &statusStub{fact: StatusFact{AssetID: 41, OwnerID: 7, AssetPublicID: assetID, Purpose: purpose, TechnicalStatus: coremedia.TechnicalFinalReady, StorageStage: coremedia.StorageStageFinal, TicketID: ticketID, TicketStatus: coremedia.TicketConsumed, Projection: pointer(coremedia.ProjectionNotRequired)}}
			service, err := NewService(&ticketStub{}, &bundleStub{bundle: bundle}, &completionStub{asset: asset}, &assetStub{}, projection, outbox, consumer, statuses, nil, WithWechatReviewSubmitter(wechat))
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				got, err := service.CompleteUploadWithOpenID(context.Background(), 7, "", ticketID)
				if err != nil || got.State != StateReady || !got.CanUse || got.ReviewStatus != "not_required" {
					t.Fatalf("completion=%+v err=%v", got, err)
				}
			}
			if wechat.submitted || len(outbox.values) != 0 || consumer.calls != 0 || projection.value.ReviewTaskID != nil {
				t.Fatal("direct upload invented a cloud task or approval event")
			}
		})
	}
}
