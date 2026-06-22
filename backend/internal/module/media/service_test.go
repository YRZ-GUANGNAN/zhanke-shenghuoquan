package media

import (
	"context"
	"testing"
	"time"
)

type repositoryStub struct {
	createdAsset       *Asset
	technicalCalls     int
	referenceCountCall int
}

func (r *repositoryStub) CreateAsset(_ context.Context, value *Asset) error {
	copy := *value
	r.createdAsset = &copy
	value.ID = 1
	return nil
}
func (r *repositoryStub) FindAsset(context.Context, string) (Asset, error) { return Asset{}, nil }
func (r *repositoryStub) CASAssetTechnicalStatus(context.Context, uint64, uint64, TechnicalStatus, TechnicalStatus) error {
	r.technicalCalls++
	return nil
}
func (*repositoryStub) CASAssetStorageStage(context.Context, uint64, uint64, StorageStage, StorageStage) error {
	return nil
}
func (*repositoryStub) CASAssetReviewProjection(context.Context, uint64, uint64, uint64, ReviewStatus, time.Time) error {
	return nil
}
func (*repositoryStub) CreateReference(context.Context, *AssetReference) error { return nil }
func (*repositoryStub) FindReference(context.Context, uint64) (AssetReference, error) {
	return AssetReference{}, nil
}
func (*repositoryStub) CASReferenceStatus(context.Context, uint64, uint64, ReferenceStatus, ReferenceStatus) error {
	return nil
}
func (r *repositoryStub) RebuildActiveRefCount(context.Context, uint64) (uint64, error) {
	r.referenceCountCall++
	return 2, nil
}
func (*repositoryStub) CreateUploadTicket(context.Context, *UploadTicket) error { return nil }
func (*repositoryStub) FindUploadTicket(context.Context, string) (UploadTicket, error) {
	return UploadTicket{}, nil
}
func (*repositoryStub) CASTicketStatus(context.Context, string, uint64, TicketStatus, TicketStatus) error {
	return nil
}
func (*repositoryStub) CreateQuotaReservation(context.Context, *QuotaReservation) error { return nil }
func (*repositoryStub) FindQuotaReservation(context.Context, string) (QuotaReservation, error) {
	return QuotaReservation{}, nil
}
func (*repositoryStub) CASQuotaReservationStatus(context.Context, string, uint64, ReservationStatus, ReservationStatus) error {
	return nil
}
func (*repositoryStub) AppendOutbox(context.Context, *OutboxEvent) error          { return nil }
func (*repositoryStub) PendingOutbox(context.Context, int) ([]OutboxEvent, error) { return nil, nil }
func (*repositoryStub) MarkOutboxProcessed(context.Context, string, uint64, time.Time) error {
	return nil
}

func TestCreateAssetRecordSetsOnlyModuleTwoInitialState(t *testing.T) {
	repository := &repositoryStub{}
	service := NewService(repository)
	expires := time.Now().Add(30 * time.Minute)
	value, err := service.CreateAssetRecord(context.Background(), NewAsset{OwnerID: 7, Purpose: PurposePost, IdempotencyKey: "request-1", UploadExpiresAt: &expires})
	if err != nil {
		t.Fatal(err)
	}
	if repository.createdAsset == nil || value.ID != 1 || !ValidULID(value.AssetPublicID) {
		t.Fatalf("asset was not persisted correctly: %#v", value)
	}
	if value.StorageStage != StorageStageStaging || value.ReviewStatus != ReviewPending || value.TechnicalStatus != TechnicalPendingUpload || value.RowVersion != 1 {
		t.Fatalf("unexpected initial state: %#v", value)
	}
}

func TestServiceRejectsIllegalTransitionBeforeRepository(t *testing.T) {
	repository := &repositoryStub{}
	service := NewService(repository)
	err := service.TransitionTechnical(context.Background(), 1, 1, TechnicalPendingUpload, TechnicalApproved)
	if err != ErrInvalidTransition {
		t.Fatalf("expected invalid transition, got %v", err)
	}
	if repository.technicalCalls != 0 {
		t.Fatal("repository was called for an illegal transition")
	}
}

func TestActiveRefCountCanOnlyBeRebuiltThroughReferenceFacts(t *testing.T) {
	repository := &repositoryStub{}
	service := NewService(repository)
	count, err := service.RebuildActiveRefCount(context.Background(), 9)
	if err != nil || count != 2 || repository.referenceCountCall != 1 {
		t.Fatalf("unexpected rebuild result count=%d err=%v calls=%d", count, err, repository.referenceCountCall)
	}
}

func TestJSONDocumentRejectsMalformedPayload(t *testing.T) {
	if _, err := (JSONDocument(`{"event":"ok"}`)).Value(); err != nil {
		t.Fatal(err)
	}
	if _, err := (JSONDocument(`{"event":`)).Value(); err != ErrInvalidJSON {
		t.Fatalf("expected ErrInvalidJSON, got %v", err)
	}
}
