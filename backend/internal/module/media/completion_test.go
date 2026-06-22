package media

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"sync"
	"testing"
	"time"
)

func TestCompleteUploadNormalAndIdempotent(t *testing.T) {
	repository, storage, service := newCompletionUnitFixture(t)
	first, err := service.CompleteUpload(context.Background(), repository.work.Bundle.Asset.OwnerID, repository.work.Bundle.Ticket.TicketID)
	if err != nil {
		t.Fatal(err)
	}
	if first.TechnicalStatus != TechnicalFinalReady || first.StorageStage != StorageStageFinal || first.FinalKey == nil || first.FinalVersionID == nil {
		t.Fatalf("asset was not finalized: %#v", first)
	}
	second, err := service.CompleteUpload(context.Background(), first.OwnerID, repository.work.Bundle.Ticket.TicketID)
	if err != nil {
		t.Fatal(err)
	}
	if second.FinalKey == nil || *second.FinalKey != *first.FinalKey || second.FinalVersionID == nil || *second.FinalVersionID != *first.FinalVersionID {
		t.Fatal("idempotent replay changed final identity")
	}
	if storage.promoteCalls != 1 {
		t.Fatalf("duplicate CompleteUpload copied %d times", storage.promoteCalls)
	}
}

func TestCompleteUploadDeclarationMismatchFailsClosed(t *testing.T) {
	repository, storage, service := newCompletionUnitFixture(t)
	storage.staging.Size++
	asset, err := service.CompleteUpload(context.Background(), repository.work.Bundle.Asset.OwnerID, repository.work.Bundle.Ticket.TicketID)
	if !errors.Is(err, ErrCompletionValidation) {
		t.Fatalf("expected validation failure, got %v", err)
	}
	if asset.TechnicalStatus != TechnicalDeletePending || repository.work.Bundle.Ticket.Status != TicketCancelled || repository.work.Bundle.Reservation.Status != ReservationReleased {
		t.Fatalf("failure was not closed: asset=%s ticket=%s reservation=%s", asset.TechnicalStatus, repository.work.Bundle.Ticket.Status, repository.work.Bundle.Reservation.Status)
	}
	if storage.promoteCalls != 0 {
		t.Fatal("invalid staging object was promoted")
	}
}

func TestCompleteUploadRecoversAmbiguousCopyWithoutSecondFinal(t *testing.T) {
	repository, storage, service := newCompletionUnitFixture(t)
	storage.ambiguousCopyOnce = true
	if _, err := service.CompleteUpload(context.Background(), repository.work.Bundle.Asset.OwnerID, repository.work.Bundle.Ticket.TicketID); err == nil {
		t.Fatal("ambiguous CopyObject unexpectedly returned success")
	}
	asset, err := service.CompleteUpload(context.Background(), repository.work.Bundle.Asset.OwnerID, repository.work.Bundle.Ticket.TicketID)
	if err != nil {
		t.Fatal(err)
	}
	if asset.TechnicalStatus != TechnicalFinalReady || storage.promoteCalls != 1 {
		t.Fatalf("recovery did not reconcile the existing final: status=%s copies=%d", asset.TechnicalStatus, storage.promoteCalls)
	}
}

func TestCompleteUploadPropagatesCASConflict(t *testing.T) {
	repository, _, service := newCompletionUnitFixture(t)
	repository.claimErr = ErrConcurrentUpdate
	if _, err := service.CompleteUpload(context.Background(), repository.work.Bundle.Asset.OwnerID, repository.work.Bundle.Ticket.TicketID); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("expected CAS conflict, got %v", err)
	}
}

type completionRepositoryStub struct {
	mu       sync.Mutex
	work     CompletionWork
	claimErr error
}

func (r *completionRepositoryStub) ClaimCompletion(_ context.Context, owner uint64, ticket string, now time.Time, leaseTTL, _ time.Duration) (CompletionWork, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.claimErr != nil {
		return CompletionWork{}, r.claimErr
	}
	if owner != r.work.Bundle.Asset.OwnerID || ticket != r.work.Bundle.Ticket.TicketID {
		return CompletionWork{}, ErrTicketForbidden
	}
	if r.work.Completion.Phase == CompletionFinalReady {
		value := r.work
		value.ShouldProcess = false
		return value, nil
	}
	lease, _ := NewULID()
	r.work.LeaseToken = lease
	r.work.Completion.LeaseToken = stringPointer(lease)
	r.work.Completion.LeaseExpiresAt = timePointer(now.Add(leaseTTL))
	r.work.ShouldProcess = true
	return r.work, nil
}

func (r *completionRepositoryStub) PinCompletionSource(_ context.Context, work CompletionWork, object CompletionObject, _ time.Time) (CompletionWork, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.work = work
	r.work.Completion.StagingVersionID = stringPointer(object.VersionID)
	r.work.Completion.StagingETag = stringPointer(object.ETag)
	r.work.Completion.StagingLastModified = timePointer(object.LastModified)
	r.work.Bundle.Asset.StagingVersionID = stringPointer(object.VersionID)
	r.work.Completion.RowVersion++
	r.work.Bundle.Asset.RowVersion++
	return r.work, nil
}

func (r *completionRepositoryStub) MarkCompletionPromoting(_ context.Context, work CompletionWork, validation CompletionValidation, _ time.Time) (CompletionWork, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.work = work
	r.work.Completion.Phase = CompletionPromoting
	r.work.Completion.TrustedSHA256 = stringPointer(validation.TrustedSHA256)
	r.work.Completion.VerifiedMIME = stringPointer(validation.VerifiedMIME)
	r.work.Completion.VerifiedSize = uint64Pointer(validation.VerifiedSize)
	r.work.Completion.VerifiedWidth = uint32Pointer(validation.Width)
	r.work.Completion.VerifiedHeight = uint32Pointer(validation.Height)
	r.work.Completion.FinalKey = stringPointer(validation.FinalKey)
	r.work.Bundle.Asset.TechnicalStatus = TechnicalPromoting
	r.work.Bundle.Asset.TrustedSHA256 = stringPointer(validation.TrustedSHA256)
	r.work.Bundle.Asset.VerifiedMIME = validation.VerifiedMIME
	r.work.Bundle.Asset.ByteSize = validation.VerifiedSize
	r.work.Bundle.Asset.Width, r.work.Bundle.Asset.Height = validation.Width, validation.Height
	return r.work, nil
}

func (r *completionRepositoryStub) RecordCompletionRetry(_ context.Context, _ string, _ string, _ CompletionPhase, code string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.work.Completion.LastErrorCode = code
	r.work.Completion.LeaseToken = nil
	r.work.Completion.LeaseExpiresAt = nil
	return nil
}

func (r *completionRepositoryStub) FailCompletionValidation(_ context.Context, work CompletionWork, code string, _ time.Time) (UploadBundle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.work = work
	r.work.Completion.Phase = CompletionValidationFailed
	r.work.Completion.LastErrorCode = code
	r.work.Bundle.Asset.TechnicalStatus = TechnicalDeletePending
	r.work.Bundle.Ticket.Status = TicketCancelled
	r.work.Bundle.Reservation.Status = ReservationReleased
	return r.work.Bundle, nil
}

func (r *completionRepositoryStub) FinalizeCompletion(_ context.Context, work CompletionWork, final CompletionObject, _ time.Time) (UploadBundle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.work = work
	r.work.Completion.Phase = CompletionFinalReady
	r.work.Completion.FinalVersionID = stringPointer(final.VersionID)
	r.work.Bundle.Asset.TechnicalStatus = TechnicalFinalReady
	r.work.Bundle.Asset.StorageStage = StorageStageFinal
	r.work.Bundle.Asset.FinalKey = stringPointer(final.Key)
	r.work.Bundle.Asset.FinalVersionID = stringPointer(final.VersionID)
	r.work.Bundle.Ticket.Status = TicketConsumed
	r.work.Bundle.Reservation.Status = ReservationCommitted
	return r.work.Bundle, nil
}

func (r *completionRepositoryStub) FindCompletion(context.Context, uint64, string) (CompletionWork, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.work, nil
}

type completionStoreStub struct {
	mu                sync.Mutex
	staging           CompletionObject
	stagingBytes      []byte
	final             *CompletionObject
	promoteCalls      int
	ambiguousCopyOnce bool
}

func (s *completionStoreStub) HeadObject(_ context.Context, key, _ string) (CompletionObject, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key == s.staging.Key {
		return s.staging, nil
	}
	if s.final != nil && key == s.final.Key {
		return *s.final, nil
	}
	return CompletionObject{}, ErrObjectNotFound
}

func (s *completionStoreStub) OpenObject(_ context.Context, key, version string) (io.ReadCloser, error) {
	if key != s.staging.Key || version != s.staging.VersionID {
		return nil, ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(s.stagingBytes)), nil
}

func (s *completionStoreStub) PromoteObject(_ context.Context, request PromoteObjectRequest) (CompletionObject, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.promoteCalls++
	value := CompletionObject{
		Key: request.DestinationKey, VersionID: "final-version-1", ETag: s.staging.ETag,
		ContentType: request.VerifiedMIME, Size: request.VerifiedSize, LastModified: time.Now().UTC(),
		Metadata: map[string]string{
			"media-asset-id": request.AssetPublicID, "media-ticket-id": request.TicketID,
			"media-sha256": request.TrustedSHA256, "media-source-version": request.SourceVersionID,
		},
	}
	s.final = &value
	if s.ambiguousCopyOnce {
		s.ambiguousCopyOnce = false
		return CompletionObject{}, errors.New("forced ambiguous CopyObject response loss")
	}
	return value, nil
}

func newCompletionUnitFixture(t *testing.T) (*completionRepositoryStub, *completionStoreStub, *CompletionService) {
	t.Helper()
	payload := unitPNG(t)
	digest := md5.Sum(payload)
	ticketID := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	assetID := "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	stagingKey := "staging/test/owner/" + assetID + "/nonce.png"
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	repository := &completionRepositoryStub{work: CompletionWork{
		Bundle: UploadBundle{
			Asset:       Asset{ID: 1, AssetPublicID: assetID, OwnerID: 42, Purpose: PurposePost, StorageStage: StorageStageStaging, TechnicalStatus: TechnicalUploaded, StagingKey: &stagingKey, RowVersion: 2, CreatedAt: now},
			Ticket:      UploadTicket{TicketID: ticketID, AssetID: 1, OwnerID: 42, ExpectedSize: uint64(len(payload)), ExpectedMD5: base64.StdEncoding.EncodeToString(digest[:]), ExpectedContentType: "image/png", Status: TicketUploaded, RowVersion: 3},
			Reservation: QuotaReservation{ReservationID: assetID, OwnerID: 42, TicketID: ticketID, Status: ReservationActive, RowVersion: 1},
		},
		Completion: UploadCompletion{TicketID: ticketID, AssetID: 1, Phase: CompletionValidating, RowVersion: 1, ProcessingDeadlineAt: now.Add(time.Hour)},
	}}
	storage := &completionStoreStub{
		stagingBytes: payload,
		staging:      CompletionObject{Key: stagingKey, VersionID: "staging-version-1", ETag: hexMD5(payload), ContentType: "image/png", Size: uint64(len(payload)), LastModified: now},
	}
	service, err := NewCompletionService(repository, storage, CompletionServiceConfig{Environment: "test", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	return repository, storage, service
}

func unitPNG(t *testing.T) []byte {
	t.Helper()
	value := image.NewRGBA(image.Rect(0, 0, 2, 2))
	value.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, value); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func hexMD5(value []byte) string {
	digest := md5.Sum(value)
	return fmtHex(digest[:])
}

func fmtHex(value []byte) string {
	const alphabet = "0123456789abcdef"
	result := make([]byte, len(value)*2)
	for index, current := range value {
		result[index*2], result[index*2+1] = alphabet[current>>4], alphabet[current&0x0f]
	}
	return string(result)
}

var _ CompletionRepository = (*completionRepositoryStub)(nil)
var _ CompletionObjectStore = (*completionStoreStub)(nil)
