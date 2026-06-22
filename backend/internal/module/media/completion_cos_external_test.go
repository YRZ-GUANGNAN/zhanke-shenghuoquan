package media

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	cos "github.com/tencentyun/cos-go-sdk-v5"
)

func TestUploadCompletionAgainstRealCOS(t *testing.T) {
	if os.Getenv("MEDIA_MODULE4_RUN_REAL_COS") != "1" {
		t.Skip("set MEDIA_MODULE4_RUN_REAL_COS=1 to run the destructive real COS completion acceptance")
	}
	_ = godotenv.Load("../../../.env")
	secretID := module4Env("MEDIA_MODULE4_COS_SECRET_ID", "MEDIA_MODULE3_COS_SECRET_ID", "TENCENT_COS_SECRET_ID")
	secretKey := module4Env("MEDIA_MODULE4_COS_SECRET_KEY", "MEDIA_MODULE3_COS_SECRET_KEY", "TENCENT_COS_SECRET_KEY")
	bucket := module4Env("MEDIA_MODULE4_COS_BUCKET", "MEDIA_MODULE3_COS_BUCKET", "TENCENT_COS_BUCKET")
	region := module4Env("MEDIA_MODULE4_COS_REGION", "MEDIA_MODULE3_COS_REGION", "TENCENT_COS_REGION")
	appID := module4Env("MEDIA_MODULE4_COS_APP_ID", "MEDIA_MODULE3_COS_APP_ID")
	if appID == "" && strings.Contains(bucket, "-") {
		appID = bucket[strings.LastIndex(bucket, "-")+1:]
	}
	if secretID == "" || secretKey == "" || bucket == "" || region == "" || appID == "" {
		t.Skip("real Module 4 Tencent COS credentials are not configured")
	}
	bucket = resolveModule4TestBucket(t, secretID, secretKey, bucket, region, appID)
	db := openModule4MySQL(t)
	ownerID := uint64(99_000_404)
	cleanupCompletionOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupCompletionOwner(t, db, ownerID) })

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	authorizer, err := NewTencentUploadAuthorizer(TencentUploadAuthorizerConfig{
		SecretID: secretID, SecretKey: secretKey, Bucket: bucket, Region: region, AppID: appID, STSTTL: 15 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewTencentCompletionStore(TencentCompletionStoreConfig{SecretID: secretID, SecretKey: secretKey, Bucket: bucket, Region: region})
	if err != nil {
		t.Fatal(err)
	}
	versioning, _, err := store.cosClient.Bucket.GetVersioning(context.Background())
	if err != nil {
		t.Fatalf("real COS GetVersioning: %v", err)
	}
	versioningStatus := ""
	if versioning != nil {
		versioningStatus = versioning.Status
	}
	if versioningStatus != "Enabled" {
		t.Fatalf("real COS bucket versioning is %q; Module 4 requires Enabled", versioningStatus)
	}
	repository := NewRepository(db)
	now := time.Now().UTC()
	uploads, err := NewUploadTicketService(repository, authorizer, UploadTicketServiceConfig{Environment: "acceptance", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	payload := unitPNG(t)
	digest := md5.Sum(payload)
	bundle, created, err := uploads.CreateTicket(ctx, CreateUploadTicketInput{
		OwnerID: ownerID, Purpose: PurposePost, ExpectedSize: uint64(len(payload)),
		ExpectedMD5: base64.StdEncoding.EncodeToString(digest[:]), ExpectedContentType: "image/png",
		IdempotencyKey: fmt.Sprintf("module4-real-%d", now.UnixNano()),
	})
	if err != nil || !created {
		t.Fatalf("create real upload ticket created=%v err=%v", created, err)
	}
	stagingKey := *bundle.Asset.StagingKey
	var finalKey string
	t.Cleanup(func() {
		cleanupCompletionCOSKey(t, store, stagingKey)
		if finalKey != "" {
			cleanupCompletionCOSKey(t, store, finalKey)
		}
	})
	authorization, err := uploads.IssueAuthorization(ctx, ownerID, bundle.Ticket.TicketID, 1)
	if err != nil {
		t.Fatalf("real GetFederationToken/presign: %v", err)
	}
	if err := putRealCompletionObject(ctx, authorization, payload); err != nil {
		t.Fatal(err)
	}
	if _, err := uploads.MarkUploaded(ctx, ownerID, bundle.Ticket.TicketID, 2); err != nil {
		t.Fatal(err)
	}
	if err := NewService(repository).TransitionTechnical(ctx, bundle.Asset.ID, 1, TechnicalPendingUpload, TechnicalUploaded); err != nil {
		t.Fatal(err)
	}
	completion, err := NewCompletionService(repository, store, CompletionServiceConfig{Environment: "acceptance"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := completion.CompleteUpload(ctx, ownerID, bundle.Ticket.TicketID)
	if err != nil {
		t.Fatalf("real HEAD/GET/Copy completion: %v", err)
	}
	if first.FinalKey == nil || first.FinalVersionID == nil || first.TechnicalStatus != TechnicalFinalReady || first.StorageStage != StorageStageFinal {
		t.Fatalf("real completion did not finalize asset: %#v", first)
	}
	finalKey = *first.FinalKey
	second, err := completion.CompleteUpload(ctx, ownerID, bundle.Ticket.TicketID)
	if err != nil {
		t.Fatalf("real idempotent completion: %v", err)
	}
	if second.FinalKey == nil || second.FinalVersionID == nil || *second.FinalKey != finalKey || *second.FinalVersionID != *first.FinalVersionID {
		t.Fatal("real idempotent completion changed final identity")
	}
	final, err := store.HeadObject(ctx, finalKey, *first.FinalVersionID)
	if err != nil || final.Size != uint64(len(payload)) || final.ContentType != "image/png" {
		t.Fatalf("real final HEAD mismatch: %#v err=%v", final, err)
	}
	versions, _, err := store.cosClient.Bucket.GetObjectVersions(ctx, &cos.BucketGetObjectVersionsOptions{Prefix: finalKey})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, version := range versions.Version {
		if version.Key == finalKey {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("idempotent completion created %d final versions", count)
	}

	// Real failure injection: CopyObject is actually executed against COS, then
	// its successful response is discarded once to emulate a lost response.
	recoveryBundle, recoveryCreated, err := uploads.CreateTicket(ctx, CreateUploadTicketInput{
		OwnerID: ownerID, Purpose: PurposePost, ExpectedSize: uint64(len(payload)),
		ExpectedMD5: base64.StdEncoding.EncodeToString(digest[:]), ExpectedContentType: "image/png",
		IdempotencyKey: fmt.Sprintf("module4-real-recovery-%d", now.UnixNano()),
	})
	if err != nil || !recoveryCreated {
		t.Fatalf("create recovery ticket created=%v err=%v", recoveryCreated, err)
	}
	recoveryStagingKey := *recoveryBundle.Asset.StagingKey
	var recoveryFinalKey string
	t.Cleanup(func() {
		cleanupCompletionCOSKey(t, store, recoveryStagingKey)
		if recoveryFinalKey != "" {
			cleanupCompletionCOSKey(t, store, recoveryFinalKey)
		}
	})
	recoveryAuthorization, err := uploads.IssueAuthorization(ctx, ownerID, recoveryBundle.Ticket.TicketID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := putRealCompletionObject(ctx, recoveryAuthorization, payload); err != nil {
		t.Fatal(err)
	}
	if _, err := uploads.MarkUploaded(ctx, ownerID, recoveryBundle.Ticket.TicketID, 2); err != nil {
		t.Fatal(err)
	}
	if err := NewService(repository).TransitionTechnical(ctx, recoveryBundle.Asset.ID, 1, TechnicalPendingUpload, TechnicalUploaded); err != nil {
		t.Fatal(err)
	}
	uncertainStore := &uncertainRealCompletionStore{delegate: store, loseFirstCopyResponse: true}
	recoveryService, err := NewCompletionService(repository, uncertainStore, CompletionServiceConfig{Environment: "acceptance"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recoveryService.CompleteUpload(ctx, ownerID, recoveryBundle.Ticket.TicketID); err == nil {
		t.Fatal("lost real CopyObject response did not surface as an uncertain result")
	}
	recoveryWork, err := repository.FindCompletion(ctx, ownerID, recoveryBundle.Ticket.TicketID)
	if err != nil || recoveryWork.Completion.FinalKey == nil {
		t.Fatalf("load uncertain recovery evidence: %#v err=%v", recoveryWork.Completion, err)
	}
	recoveryFinalKey = *recoveryWork.Completion.FinalKey
	recovered, err := recoveryService.CompleteUpload(ctx, ownerID, recoveryBundle.Ticket.TicketID)
	if err != nil {
		t.Fatalf("recover real uncertain CopyObject: %v", err)
	}
	if recovered.FinalVersionID == nil || uncertainStore.promoteCalls != 1 {
		t.Fatalf("uncertain recovery copied again or lost final version: calls=%d asset=%#v", uncertainStore.promoteCalls, recovered)
	}
	if count := countCompletionCOSVersions(t, ctx, store, recoveryFinalKey); count != 1 {
		t.Fatalf("uncertain recovery created %d final versions", count)
	}
	if first.StagingVersionID == nil || recovered.StagingVersionID == nil {
		t.Fatal("fixed staging version was not persisted")
	}
	t.Logf(
		"PASS real STS/PUT/fixed HEAD/GET/Copy/final HEAD/idempotency/recovery sourceVersion=%s finalVersion=%s recoverySourceVersion=%s recoveryFinalVersion=%s finalVersions=1 recoveryFinalVersions=1 realCopyCalls=1",
		redactVersion(*first.StagingVersionID), redactVersion(*first.FinalVersionID),
		redactVersion(*recovered.StagingVersionID), redactVersion(*recovered.FinalVersionID),
	)
}

type uncertainRealCompletionStore struct {
	delegate              *TencentCompletionStore
	loseFirstCopyResponse bool
	promoteCalls          int
}

func (s *uncertainRealCompletionStore) HeadObject(ctx context.Context, key, version string) (CompletionObject, error) {
	return s.delegate.HeadObject(ctx, key, version)
}

func (s *uncertainRealCompletionStore) OpenObject(ctx context.Context, key, version string) (io.ReadCloser, error) {
	return s.delegate.OpenObject(ctx, key, version)
}

func (s *uncertainRealCompletionStore) PromoteObject(ctx context.Context, request PromoteObjectRequest) (CompletionObject, error) {
	s.promoteCalls++
	result, err := s.delegate.PromoteObject(ctx, request)
	if err == nil && s.loseFirstCopyResponse {
		s.loseFirstCopyResponse = false
		return CompletionObject{}, errors.New("injected response loss after real CopyObject success")
	}
	return result, err
}

func countCompletionCOSVersions(t *testing.T, ctx context.Context, store *TencentCompletionStore, key string) int {
	t.Helper()
	versions, _, err := store.cosClient.Bucket.GetObjectVersions(ctx, &cos.BucketGetObjectVersionsOptions{Prefix: key})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, version := range versions.Version {
		if version.Key == key {
			count++
		}
	}
	return count
}

func putRealCompletionObject(ctx context.Context, authorization UploadAuthorization, payload []byte) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, authorization.UploadURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.ContentLength = int64(len(payload))
	request.Header.Set("Content-Type", authorization.RequiredHeaders.ContentType)
	request.Header.Set("Content-MD5", authorization.RequiredHeaders.ContentMD5)
	request.Header.Set("x-cos-forbid-overwrite", authorization.RequiredHeaders.ForbidOverwrite)
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("real COS PUT status=%d body=%q", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if response.Header.Get("x-cos-version-id") == "" {
		return errors.New("real COS PUT did not return x-cos-version-id; bucket versioning is required")
	}
	return nil
}

func cleanupCompletionCOSKey(t *testing.T, store *TencentCompletionStore, key string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	versions, _, err := store.cosClient.Bucket.GetObjectVersions(ctx, &cos.BucketGetObjectVersionsOptions{Prefix: key})
	if err != nil {
		t.Errorf("list cleanup versions for key fingerprint %s: %v", redactVersion(key), err)
		return
	}
	for _, version := range versions.Version {
		if version.Key == key {
			if _, err := store.cosClient.Object.Delete(ctx, key, &cos.ObjectDeleteOptions{VersionId: version.VersionId}); err != nil {
				t.Errorf("delete cleanup version: %v", err)
			}
		}
	}
	for _, marker := range versions.DeleteMarker {
		if marker.Key == key {
			if _, err := store.cosClient.Object.Delete(ctx, key, &cos.ObjectDeleteOptions{VersionId: marker.VersionId}); err != nil {
				t.Errorf("delete cleanup marker: %v", err)
			}
		}
	}
	after, _, err := store.cosClient.Bucket.GetObjectVersions(ctx, &cos.BucketGetObjectVersionsOptions{Prefix: key})
	if err != nil {
		t.Errorf("verify cleanup versions: %v", err)
		return
	}
	for _, version := range after.Version {
		if version.Key == key {
			t.Errorf("COS cleanup residue remains for key fingerprint %s", redactVersion(key))
		}
	}
	for _, marker := range after.DeleteMarker {
		if marker.Key == key {
			t.Errorf("COS cleanup delete-marker residue remains for key fingerprint %s", redactVersion(key))
		}
	}
}

func module4Env(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func resolveModule4TestBucket(t *testing.T, secretID, secretKey, configured, region, appID string) string {
	t.Helper()
	serviceURL, err := url.Parse("https://service.cos.myqcloud.com")
	if err != nil {
		t.Fatal(err)
	}
	client := cos.NewClient(&cos.BaseURL{ServiceURL: serviceURL}, &http.Client{
		Timeout:   30 * time.Second,
		Transport: &cos.AuthorizationTransport{SecretID: secretID, SecretKey: secretKey},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, _, err := client.Service.Get(ctx)
	if err != nil {
		t.Fatalf("real COS GetService: %v", err)
	}
	for _, candidate := range result.Buckets {
		if candidate.Name == configured && candidate.Region == region {
			return configured
		}
	}
	expectedFullName := configured + "-" + appID
	for _, candidate := range result.Buckets {
		if candidate.Name == expectedFullName && candidate.Region == region {
			t.Logf("using control-plane resolved full bucket fingerprint %s", redactVersion(candidate.Name))
			return candidate.Name
		}
	}
	t.Fatalf("configured bucket fingerprint %s is not visible in region %s", redactVersion(configured), region)
	return ""
}

func redactVersion(value string) string {
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "..." + value[len(value)-4:]
}

var _ CompletionObjectStore = (*uncertainRealCompletionStore)(nil)
