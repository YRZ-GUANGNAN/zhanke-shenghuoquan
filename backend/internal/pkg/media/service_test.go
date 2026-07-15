package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"campus-circle/backend/internal/module/admin/review"
)

type memoryRepo struct {
	mu            sync.Mutex
	nextID        uint64
	assets        map[uint64]Asset
	byKey         map[string]uint64
	byTrace       map[string]uint64
	findPublicErr error
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{nextID: 1, assets: map[uint64]Asset{}, byKey: map[string]uint64{}, byTrace: map[string]uint64{}}
}

func (r *memoryRepo) CreateOrFind(_ context.Context, value *Asset) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := string(rune(value.OwnerUserID)) + value.IdempotencyKey
	if id, ok := r.byKey[key]; ok {
		*value = r.assets[id]
		return false, nil
	}
	value.ID = r.nextID
	r.nextID++
	r.assets[value.ID], r.byKey[key] = *value, value.ID
	return true, nil
}
func (r *memoryRepo) RefreshUploadExpiry(_ context.Context, id uint64, expires time.Time) error {
	return r.update(id, func(value *Asset) error {
		if value.Status != StatusPendingUpload || value.DeleteRequestedAt != nil {
			return ErrInvalidState
		}
		value.UploadExpiresAt = expires
		return nil
	})
}
func (r *memoryRepo) FindOwned(_ context.Context, publicID string, ownerID uint) (Asset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, value := range r.assets {
		if value.PublicID == publicID && value.OwnerUserID == ownerID {
			return value, nil
		}
	}
	return Asset{}, ErrNotFound
}
func (r *memoryRepo) FindPublic(_ context.Context, publicID string) (Asset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.findPublicErr != nil {
		return Asset{}, r.findPublicErr
	}
	for _, value := range r.assets {
		if value.PublicID == publicID && value.Status == StatusApproved && value.ReferenceType != "" && value.DeleteRequestedAt == nil {
			return value, nil
		}
	}
	return Asset{}, ErrNotFound
}

type fallbackContentResolver struct {
	calls int
	value string
	err   error
}

func (r *fallbackContentResolver) ContentURLByMediaID(context.Context, string) (string, error) {
	r.calls++
	return r.value, r.err
}
func (r *memoryRepo) MarkUploaded(_ context.Context, id uint64, size int64, mime string, width, height int, openID string) error {
	return r.update(id, func(value *Asset) error {
		if value.Status != StatusPendingUpload {
			return ErrInvalidState
		}
		value.Status, value.VerifiedSize, value.VerifiedMIME, value.Width, value.Height, value.WechatOpenID = StatusUploaded, size, mime, width, height, openID
		return nil
	})
}
func (r *memoryRepo) SetReviewTask(_ context.Context, id, taskID uint64) error {
	return r.update(id, func(value *Asset) error { value.Status, value.ReviewTaskID = StatusSubmitted, taskID; return nil })
}
func (r *memoryRepo) SetReviewStatus(_ context.Context, id uint64, status Status) error {
	return r.update(id, func(value *Asset) error { value.Status = status; return nil })
}
func (r *memoryRepo) SetReviewTrace(_ context.Context, id uint64, trace string) error {
	return r.update(id, func(value *Asset) error { value.ReviewTraceID = &trace; r.byTrace[trace] = id; return nil })
}
func (r *memoryRepo) MarkReviewFailure(_ context.Context, id uint64, attempts int, message string, available time.Time) error {
	return r.update(id, func(value *Asset) error {
		value.ReviewAttempts, value.LastError, value.ReviewAvailableAt = attempts, message, available
		return nil
	})
}
func (r *memoryRepo) ApplyReviewResult(_ context.Context, trace, suggest string, label int) (Asset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, ok := r.byTrace[trace]
	if !ok {
		return Asset{}, ErrNotFound
	}
	value := r.assets[id]
	value.ReviewSuggest, value.ReviewLabel = suggest, label
	r.assets[id] = value
	return value, nil
}
func (r *memoryRepo) Reserve(_ context.Context, ownerID uint, purpose Purpose, ids []string, token string, expires time.Time) ([]Asset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	values := make([]Asset, 0, len(ids))
	for _, publicID := range ids {
		found := false
		for id, value := range r.assets {
			if value.PublicID == publicID {
				if value.OwnerUserID != ownerID || value.Purpose != purpose || value.Status != StatusApproved || value.ReferenceType != "" {
					return nil, ErrInvalidAsset
				}
				value.ReservationToken, value.ReservationExpiresAt = token, &expires
				r.assets[id], values, found = value, append(values, value), true
			}
		}
		if !found {
			return nil, ErrInvalidAsset
		}
	}
	return values, nil
}
func (r *memoryRepo) Commit(_ context.Context, ownerID uint, token, refType string, refID uint64, ordered []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := make(map[string]bool, len(ordered))
	for index, publicID := range ordered {
		kept[publicID] = true
		for id, value := range r.assets {
			if value.PublicID == publicID && value.OwnerUserID == ownerID && value.ReservationToken == token {
				value.ReferenceType, value.ReferenceID, value.ReferenceSort, value.ReservationToken = refType, refID, index, ""
				value.ReservationExpiresAt = nil
				r.assets[id] = value
			}
		}
	}
	for id, value := range r.assets {
		if value.ReferenceType == refType && value.ReferenceID == refID && !kept[value.PublicID] && value.DeleteRequestedAt == nil {
			now := time.Now()
			value.Status, value.DeleteRequestedAt = StatusDeleted, &now
			r.assets[id] = value
		}
	}
	return nil
}
func (r *memoryRepo) CancelReservation(_ context.Context, ownerID uint, token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, value := range r.assets {
		if value.OwnerUserID == ownerID && value.ReservationToken == token {
			value.ReservationToken = ""
			value.ReservationExpiresAt = nil
			r.assets[id] = value
		}
	}
	return nil
}
func (r *memoryRepo) RequestDeleteOwned(_ context.Context, publicID string, ownerID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, value := range r.assets {
		if value.PublicID == publicID && value.OwnerUserID == ownerID && value.ReferenceType == "" {
			now := time.Now()
			value.Status = StatusDeleted
			value.DeleteRequestedAt = &now
			r.assets[id] = value
			return nil
		}
	}
	return ErrForbidden
}
func (r *memoryRepo) RequestDeleteReference(_ context.Context, refType string, refID uint64, keep []uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := map[uint64]bool{}
	for _, id := range keep {
		kept[id] = true
	}
	for id, value := range r.assets {
		if value.ReferenceType == refType && value.ReferenceID == refID && !kept[id] {
			now := time.Now()
			value.Status = StatusDeleted
			value.DeleteRequestedAt = &now
			r.assets[id] = value
		}
	}
	return nil
}
func (r *memoryRepo) PendingReviews(_ context.Context, limit int, now time.Time) ([]Asset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Asset
	for _, v := range r.assets {
		if len(out) < limit && v.ReviewTraceID == nil && !v.ReviewAvailableAt.After(now) && (v.Status == StatusUploaded || v.Status == StatusSubmitted || v.Status == StatusReviewing) {
			out = append(out, v)
		}
	}
	return out, nil
}
func (r *memoryRepo) PendingDeletes(_ context.Context, limit int) ([]Asset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Asset
	for _, v := range r.assets {
		if len(out) < limit && v.DeleteRequestedAt != nil && v.StorageDeletedAt == nil {
			out = append(out, v)
		}
	}
	return out, nil
}
func (r *memoryRepo) ExpiredGarbage(_ context.Context, limit int, cutoff time.Time) ([]Asset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Asset
	for _, v := range r.assets {
		if len(out) < limit && v.ReferenceType == "" && v.DeleteRequestedAt == nil && v.UploadExpiresAt.Before(cutoff) {
			out = append(out, v)
		}
	}
	return out, nil
}
func (r *memoryRepo) MarkStorageDeleted(_ context.Context, id uint64) error {
	return r.update(id, func(v *Asset) error {
		now := time.Now()
		v.StorageDeletedAt = &now
		v.Status = StatusDeleted
		return nil
	})
}
func (r *memoryRepo) update(id uint64, fn func(*Asset) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.assets[id]
	if !ok {
		return ErrNotFound
	}
	if err := fn(&value); err != nil {
		return err
	}
	r.assets[id] = value
	return nil
}

type fakeIssuer struct {
	mu   sync.Mutex
	keys []string
}

func (f *fakeIssuer) Issue(_ context.Context, key string, _ time.Duration) (TemporaryCredentials, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keys = append(f.keys, key)
	return TemporaryCredentials{SecretID: "tmp", SecretKey: "secret", SessionToken: "token", StartTime: 1, ExpiredTime: 2}, nil
}

type fakeStorage struct {
	info    ObjectInfo
	prefix  []byte
	deleted []string
}

func (f *fakeStorage) Stat(context.Context, string) (ObjectInfo, error) { return f.info, nil }
func (f *fakeStorage) ReadPrefix(context.Context, string, int64) ([]byte, error) {
	return f.prefix, nil
}
func (f *fakeStorage) SignedGetURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://cos.invalid/" + key + "?signature=secret", nil
}
func (f *fakeStorage) Delete(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}

type fakeSafety struct{ calls int }

func (f *fakeSafety) Submit(context.Context, string, string) (string, error) {
	f.calls++
	return "trace-1", nil
}

type fakeReviews struct {
	target *Service
	next   uint
	tasks  map[uint]review.Status
}

func (f *fakeReviews) Submit(_ context.Context, _ review.Submission) (review.Task, error) {
	f.next++
	if f.tasks == nil {
		f.tasks = map[uint]review.Status{}
	}
	f.tasks[f.next] = review.StatusSubmitted
	return review.Task{ID: f.next, Status: review.StatusSubmitted}, nil
}
func (f *fakeReviews) Start(ctx context.Context, id, operator uint) (review.Task, error) {
	f.tasks[id] = review.StatusReviewing
	if err := f.target.SetReviewStatus(ctx, id, review.StatusReviewing); err != nil {
		return review.Task{}, err
	}
	return review.Task{ID: id, Status: review.StatusReviewing, OperatorID: operator}, nil
}
func (f *fakeReviews) Decide(ctx context.Context, id, operator uint, status review.Status, reason string) (review.Task, error) {
	if f.tasks[id] != review.StatusReviewing {
		return review.Task{}, errors.New("not reviewing")
	}
	if err := f.target.SetReviewStatus(ctx, id, status); err != nil {
		return review.Task{}, err
	}
	f.tasks[id] = status
	return review.Task{ID: id, Status: status, OperatorID: operator, Reason: reason}, nil
}

func newTestService() (*Service, *memoryRepo, *fakeStorage, *fakeReviews, *fakeSafety) {
	repo := newMemoryRepo()
	storage := &fakeStorage{}
	reviews := &fakeReviews{}
	safety := &fakeSafety{}
	service := NewService(repo, &fakeIssuer{}, storage, safety, reviews, "bucket-1", "ap-test", "https://api.example.com")
	reviews.target = service
	return service, repo, storage, reviews, safety
}

func makeTestJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	value := image.NewRGBA(image.Rect(0, 0, width, height))
	value.Set(0, 0, color.White)
	var output bytes.Buffer
	if err := jpeg.Encode(&output, value, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestUploadReviewBindingAndControlledURL(t *testing.T) {
	service, repo, storage, _, safety := newTestService()
	jpegBytes := makeTestJPEG(t, 320, 240)
	storage.info = ObjectInfo{Size: int64(len(jpegBytes)), ContentType: "image/jpeg"}
	storage.prefix = jpegBytes
	ticket, err := service.CreateUpload(context.Background(), CreateUploadInput{OwnerUserID: 7, Purpose: PurposePostImage, FileName: "photo.jpg", MIMEType: "image/jpeg", SizeBytes: int64(len(jpegBytes)), IdempotencyKey: "same-image"})
	if err != nil {
		t.Fatal(err)
	}
	asset, err := service.Complete(context.Background(), 7, "openid-7", ticket.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if asset.Status != StatusReviewing || asset.ReviewTraceID == nil || safety.calls != 1 {
		t.Fatalf("unexpected review state: %+v calls=%d", asset, safety.calls)
	}
	if asset.Width != 320 || asset.Height != 240 {
		t.Fatalf("verified dimensions=%dx%d want=320x240", asset.Width, asset.Height)
	}
	if err := service.ApplyWechatResult(context.Background(), "trace-1", "pass", 100); err != nil {
		t.Fatal(err)
	}
	reservation, err := service.Reserve(context.Background(), 7, PurposePostImage, []string{ticket.AssetID})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(reservation.References[0].URL, "myqcloud") || !strings.Contains(reservation.References[0].URL, "/api/v1/media/assets/") {
		t.Fatalf("business layer received storage URL: %s", reservation.References[0].URL)
	}
	repo.assets[99] = Asset{ID: 99, PublicID: "superseded", OwnerUserID: 7, Purpose: PurposePostImage, Status: StatusApproved, ReferenceType: "post", ReferenceID: 42}
	if err := service.Commit(context.Background(), 7, reservation, "post", 42); err != nil {
		t.Fatal(err)
	}
	if repo.assets[99].DeleteRequestedAt == nil {
		t.Fatal("superseded reference was not atomically queued for deletion")
	}
	if _, err := service.ContentURL(context.Background(), ticket.AssetID); err != nil {
		t.Fatal(err)
	}
}

func TestContentURLFallbackUsesCoreOnlyForLegacyNotFound(t *testing.T) {
	tests := []struct {
		name          string
		legacy        *Asset
		legacyErr     error
		wantURL       string
		wantErr       error
		resolverCalls int
	}{
		{name: "core fallback", wantURL: "https://core.invalid/fixed", resolverCalls: 1},
		{name: "legacy wins", legacy: &Asset{ID: 1, PublicID: "legacy", Status: StatusApproved, ReferenceType: "product", ObjectKey: "legacy/product.webp"}, wantURL: "https://cos.invalid/legacy/product.webp?signature=secret"},
		{name: "database failure does not fallback", legacyErr: errors.New("database unavailable"), wantErr: errors.New("database unavailable")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newMemoryRepo()
			repo.findPublicErr = test.legacyErr
			if test.legacy != nil {
				repo.assets[test.legacy.ID] = *test.legacy
			}
			resolver := &fallbackContentResolver{value: "https://core.invalid/fixed"}
			service := NewService(repo, &fakeIssuer{}, &fakeStorage{}, &fakeSafety{}, &fakeReviews{},
				"bucket-1", "ap-test", "https://api.example.com", WithPublicContentResolver(resolver))
			value, err := service.ContentURL(context.Background(), "legacy")
			if test.wantErr != nil {
				if err == nil || err.Error() != test.wantErr.Error() {
					t.Fatalf("err=%v want=%v", err, test.wantErr)
				}
			} else if err != nil || value != test.wantURL {
				t.Fatalf("url=%q err=%v want=%q", value, err, test.wantURL)
			}
			if resolver.calls != test.resolverCalls {
				t.Fatalf("resolver calls=%d want=%d", resolver.calls, test.resolverCalls)
			}
		})
	}
}

func TestOversizedImageDimensionsAreRejected(t *testing.T) {
	service, _, storage, _, _ := newTestService()
	jpegBytes := makeTestJPEG(t, 1025, 1)
	storage.info = ObjectInfo{Size: int64(len(jpegBytes)), ContentType: "image/jpeg"}
	storage.prefix = jpegBytes
	ticket, err := service.CreateUpload(context.Background(), CreateUploadInput{OwnerUserID: 12, Purpose: PurposeAvatar, FileName: "wide.jpg", MIMEType: "image/jpeg", SizeBytes: int64(len(jpegBytes)), IdempotencyKey: "oversized-avatar"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Complete(context.Background(), 12, "openid", ticket.AssetID); err == nil {
		t.Fatal("oversized avatar dimensions were accepted")
	}
	asset, _ := service.Asset(context.Background(), 12, ticket.AssetID)
	if asset.DeleteRequestedAt == nil {
		t.Fatal("oversized object was not queued for deletion")
	}
}

func TestMIMEForgeryIsRejectedAndQueuedForDeletion(t *testing.T) {
	service, _, storage, _, _ := newTestService()
	storage.info = ObjectInfo{Size: 5, ContentType: "image/jpeg"}
	storage.prefix = []byte("hello")
	ticket, err := service.CreateUpload(context.Background(), CreateUploadInput{OwnerUserID: 2, Purpose: PurposeAvatar, FileName: "a.jpg", MIMEType: "image/jpeg", SizeBytes: 5, IdempotencyKey: "forged"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Complete(context.Background(), 2, "openid", ticket.AssetID); err == nil {
		t.Fatal("forged content was accepted")
	}
	asset, _ := service.Asset(context.Background(), 2, ticket.AssetID)
	if asset.DeleteRequestedAt == nil {
		t.Fatal("forged object was not queued for deletion")
	}
}

func TestConcurrentCreateIsIdempotent(t *testing.T) {
	service, repo, _, _, _ := newTestService()
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	ids := make(chan string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticket, err := service.CreateUpload(context.Background(), CreateUploadInput{OwnerUserID: 9, Purpose: PurposeProductImage, FileName: "x.webp", MIMEType: "image/webp", SizeBytes: 123, IdempotencyKey: "one"})
			errs <- err
			if err == nil {
				ids <- ticket.AssetID
			}
		}()
	}
	wg.Wait()
	close(errs)
	close(ids)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	unique := map[string]bool{}
	for id := range ids {
		unique[id] = true
	}
	if len(unique) != 1 || len(repo.assets) != 1 {
		t.Fatalf("idempotency failed ids=%d assets=%d", len(unique), len(repo.assets))
	}
}

func TestDirectUploadRetryRefreshesExpiredDeadline(t *testing.T) {
	service, repo, _, _, _ := newTestService()
	base := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return base }
	input := CreateUploadInput{OwnerUserID: 11, Purpose: PurposePostImage, FileName: "retry.jpg", MIMEType: "image/jpeg", SizeBytes: 128, IdempotencyKey: "retry-image"}
	first, err := service.CreateUpload(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return base.Add(20 * time.Minute) }
	second, err := service.CreateUpload(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if second.AssetID != first.AssetID || len(repo.assets) != 1 {
		t.Fatalf("retry created another asset: first=%s second=%s count=%d", first.AssetID, second.AssetID, len(repo.assets))
	}
	asset, err := service.Asset(context.Background(), input.OwnerUserID, first.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	want := base.Add(20*time.Minute + uploadTTL)
	if !asset.UploadExpiresAt.Equal(want) {
		t.Fatalf("upload deadline=%s want=%s", asset.UploadExpiresAt, want)
	}
}

func TestGarbageCollectorDeletesExpiredObject(t *testing.T) {
	service, repo, storage, _, _ := newTestService()
	now := time.Now()
	repo.assets[1] = Asset{ID: 1, PublicID: "expired", OwnerUserID: 3, ObjectKey: "media/expired.jpg", Status: StatusPendingUpload, UploadExpiresAt: now.Add(-time.Hour)}
	if err := service.ProcessMaintenance(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if len(storage.deleted) != 1 {
		t.Fatalf("deleted=%v", storage.deleted)
	}
	asset := repo.assets[1]
	if asset.StorageDeletedAt == nil {
		t.Fatal("storage deletion was not recorded")
	}
}

func TestWechatSignatureAndProtocol(t *testing.T) {
	if !VerifyWechatSignature("token", "1", "2", sha1Hex("1"+"2"+"token")) {
		t.Fatal("valid signature rejected")
	}
	var checked bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/token" {
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access", "expires_in": 7200})
			return
		}
		if r.URL.Path == "/wxa/media_check_async" {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			checked = body["media_url"] == "https://signed.example/image" && body["openid"] == "openid" && body["media_type"] == float64(2)
			_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0, "trace_id": "trace"})
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	client, err := NewWechatSafety("appid", "secret", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	trace, err := client.Submit(context.Background(), "openid", "https://signed.example/image")
	if err != nil || trace != "trace" || !checked {
		t.Fatalf("trace=%s checked=%v err=%v", trace, checked, err)
	}
}
