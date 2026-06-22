package miniappflow

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
	cos "github.com/tencentyun/cos-go-sdk-v5"
)

func TestMiniAppUploadFlowAgainstRealCOSAndMySQL84(t *testing.T) {
	if os.Getenv("MEDIA_MODULE10_RUN_REAL_E2E") != "1" {
		t.Skip("set MEDIA_MODULE10_RUN_REAL_E2E=1 for the real MiniApp media E2E")
	}
	db := openModule10MySQL(t)
	secretID := module10Env("MEDIA_MODULE10_COS_SECRET_ID", "MEDIA_MODULE4_COS_SECRET_ID", "MEDIA_MODULE3_COS_SECRET_ID", "TENCENT_COS_SECRET_ID", "COS_SECRET_ID")
	secretKey := module10Env("MEDIA_MODULE10_COS_SECRET_KEY", "MEDIA_MODULE4_COS_SECRET_KEY", "MEDIA_MODULE3_COS_SECRET_KEY", "TENCENT_COS_SECRET_KEY", "COS_SECRET_KEY")
	bucket := module10Env("MEDIA_MODULE10_COS_BUCKET", "MEDIA_MODULE4_COS_BUCKET", "MEDIA_MODULE3_COS_BUCKET", "TENCENT_COS_BUCKET", "COS_BUCKET")
	region := module10Env("MEDIA_MODULE10_COS_REGION", "MEDIA_MODULE4_COS_REGION", "MEDIA_MODULE3_COS_REGION", "TENCENT_COS_REGION", "COS_REGION")
	appID := module10Env("MEDIA_MODULE10_COS_APP_ID", "MEDIA_MODULE4_COS_APP_ID", "MEDIA_MODULE3_COS_APP_ID", "COS_APP_ID")
	if appID == "" && strings.Contains(bucket, "-") {
		appID = bucket[strings.LastIndex(bucket, "-")+1:]
	}
	if secretID == "" || secretKey == "" || bucket == "" || region == "" || appID == "" {
		t.Fatal("real Module 10 COS credentials are incomplete")
	}
	bucket = resolveModule10Bucket(t, secretID, secretKey, bucket, region, appID)

	ownerID := uint64(99_001_010)
	cleanupModule10Owner(t, db, ownerID)
	t.Cleanup(func() { cleanupModule10Owner(t, db, ownerID) })
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	authorizer, err := coremedia.NewTencentUploadAuthorizer(coremedia.TencentUploadAuthorizerConfig{
		SecretID: secretID, SecretKey: secretKey, Bucket: bucket, Region: region, AppID: appID, STSTTL: 15 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := coremedia.NewTencentCompletionStore(coremedia.TencentCompletionStoreConfig{
		SecretID: secretID, SecretKey: secretKey, Bucket: bucket, Region: region,
	})
	if err != nil {
		t.Fatal(err)
	}
	repository := coremedia.NewRepository(db)
	tickets, err := coremedia.NewUploadTicketService(repository, authorizer, coremedia.UploadTicketServiceConfig{Environment: "acceptance"})
	if err != nil {
		t.Fatal(err)
	}
	completion, err := coremedia.NewCompletionService(repository, store, coremedia.CompletionServiceConfig{Environment: "acceptance"})
	if err != nil {
		t.Fatal(err)
	}
	projections, err := coremedia.NewReviewProjectionService(repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := coremedia.NewMediaConsumer(repository, projections, coremedia.MediaConsumerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	application, err := NewService(
		tickets, repository, completion, coremedia.NewService(repository), projections,
		repository, consumer, NewStatusRepository(db), nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	payload := module10PNG(t)
	digest := md5.Sum(payload)
	session, err := application.CreateUpload(ctx, CreateUploadInput{
		OwnerID: ownerID, Purpose: coremedia.PurposePost, SizeBytes: uint64(len(payload)),
		ContentMD5: base64.StdEncoding.EncodeToString(digest[:]), ContentType: "image/png",
		IdempotencyKey: fmt.Sprintf("module10-real-%d", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatalf("real STS ticket/authorization: %v", err)
	}
	bundle, err := repository.FindUploadBundle(ctx, session.TicketID, ownerID)
	if err != nil || bundle.Asset.StagingKey == nil {
		t.Fatalf("load real staging identity: %v", err)
	}
	stagingKey := *bundle.Asset.StagingKey
	finalKey := ""
	cosClient := module10COSClient(t, secretID, secretKey, bucket, region)
	t.Cleanup(func() {
		cleanupModule10COSKey(t, cosClient, stagingKey)
		if finalKey != "" {
			cleanupModule10COSKey(t, cosClient, finalKey)
		}
	})

	putModule10Object(t, ctx, session, payload)
	first, err := application.CompleteUpload(ctx, ownerID, session.TicketID)
	if err != nil {
		t.Fatalf("real completion/projection/consumer: %v", err)
	}
	asset, err := repository.FindAsset(ctx, session.AssetID)
	if err != nil || asset.FinalKey == nil || asset.FinalVersionID == nil {
		t.Fatalf("fixed final object identity missing: asset=%#v err=%v", asset, err)
	}
	finalKey = *asset.FinalKey
	if first.State != StateReviewPending || first.ReviewStatus != string(coremedia.ProjectionSubmitted) || first.CanUse {
		t.Fatalf("unexpected server-owned final state: %#v", first)
	}
	second, err := application.CompleteUpload(ctx, ownerID, session.TicketID)
	if err != nil || second.AssetID != first.AssetID || second.State != first.State {
		t.Fatalf("idempotent complete changed result: first=%#v second=%#v err=%v", first, second, err)
	}
	if _, err := application.Status(ctx, ownerID+1, session.AssetID); err == nil {
		t.Fatal("cross-owner media status was exposed")
	}
	var outboxCount, inboxCount int64
	if err := db.Table("media_outbox").Where("aggregate_id IN (SELECT projection_id FROM media_review_projections WHERE asset_id = ?)", asset.ID).Count(&outboxCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Table("media_event_inbox").Where("aggregate_id IN (SELECT projection_id FROM media_review_projections WHERE asset_id = ?)", asset.ID).Count(&inboxCount).Error; err != nil {
		t.Fatal(err)
	}
	if outboxCount != 1 || inboxCount != 1 {
		t.Fatalf("review delivery is not idempotent: outbox=%d inbox=%d", outboxCount, inboxCount)
	}
}

func putModule10Object(t *testing.T, ctx context.Context, session UploadSession, payload []byte) {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, session.UploadURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.ContentLength = int64(len(payload))
	request.Header.Set("Content-Type", session.Headers.ContentType)
	request.Header.Set("Content-MD5", session.Headers.ContentMD5)
	request.Header.Set("x-cos-forbid-overwrite", session.Headers.ForbidOverwrite)
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		t.Fatalf("real COS PUT status=%d body=%q", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if response.Header.Get("x-cos-version-id") == "" {
		t.Fatal("real COS PUT returned no fixed version ID")
	}
}

func module10PNG(t *testing.T) []byte {
	t.Helper()
	value := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	value.Set(0, 0, color.NRGBA{R: 32, G: 128, B: 224, A: 255})
	value.Set(1, 0, color.NRGBA{R: 224, G: 128, B: 32, A: 255})
	value.Set(0, 1, color.NRGBA{R: 64, G: 192, B: 96, A: 255})
	value.Set(1, 1, color.NRGBA{R: 192, G: 64, B: 160, A: 255})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, value); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func module10COSClient(t *testing.T, secretID, secretKey, bucket, region string) *cos.Client {
	t.Helper()
	bucketURL, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", bucket, region))
	if err != nil {
		t.Fatal(err)
	}
	return cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Timeout:   30 * time.Second,
		Transport: &cos.AuthorizationTransport{SecretID: secretID, SecretKey: secretKey},
	})
}

func resolveModule10Bucket(t *testing.T, secretID, secretKey, configured, region, appID string) string {
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
			t.Logf("resolved acceptance bucket fingerprint %s", redactModule10(candidate.Name))
			return candidate.Name
		}
	}
	t.Fatalf("configured bucket fingerprint %s is not visible in region %s", redactModule10(configured), region)
	return ""
}

func redactModule10(value string) string {
	if len(value) <= 8 {
		return "***"
	}
	return value[:4] + "..." + value[len(value)-4:]
}

func cleanupModule10COSKey(t *testing.T, client *cos.Client, key string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	versions, _, err := client.Bucket.GetObjectVersions(ctx, &cos.BucketGetObjectVersionsOptions{Prefix: key})
	if err != nil {
		t.Errorf("list COS cleanup versions: %v", err)
		return
	}
	for _, version := range versions.Version {
		if version.Key == key {
			if _, err := client.Object.Delete(ctx, key, &cos.ObjectDeleteOptions{VersionId: version.VersionId}); err != nil {
				t.Errorf("delete COS version: %v", err)
			}
		}
	}
	for _, marker := range versions.DeleteMarker {
		if marker.Key == key {
			if _, err := client.Object.Delete(ctx, key, &cos.ObjectDeleteOptions{VersionId: marker.VersionId}); err != nil {
				t.Errorf("delete COS marker: %v", err)
			}
		}
	}
	after, _, err := client.Bucket.GetObjectVersions(ctx, &cos.BucketGetObjectVersionsOptions{Prefix: key})
	if err != nil {
		t.Errorf("verify COS cleanup: %v", err)
		return
	}
	for _, version := range after.Version {
		if version.Key == key {
			t.Errorf("COS version residue remains")
		}
	}
	for _, marker := range after.DeleteMarker {
		if marker.Key == key {
			t.Errorf("COS delete marker residue remains")
		}
	}
}

func module10Env(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}
