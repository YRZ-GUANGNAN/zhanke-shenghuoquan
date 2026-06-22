package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	cos "github.com/tencentyun/cos-go-sdk-v5"
)

func TestMediaGCDeletesOnlyFixedVersionAgainstRealCOS(t *testing.T) {
	if os.Getenv("MEDIA_MODULE8_RUN_REAL_COS") != "1" {
		t.Skip("set MEDIA_MODULE8_RUN_REAL_COS=1 for destructive fixed-version COS GC acceptance")
	}
	_ = godotenv.Load("../../../.env")
	secretID := module4Env("MEDIA_MODULE8_COS_SECRET_ID", "MEDIA_MODULE4_COS_SECRET_ID", "TENCENT_COS_SECRET_ID")
	secretKey := module4Env("MEDIA_MODULE8_COS_SECRET_KEY", "MEDIA_MODULE4_COS_SECRET_KEY", "TENCENT_COS_SECRET_KEY")
	bucket := module4Env("MEDIA_MODULE8_COS_BUCKET", "MEDIA_MODULE4_COS_BUCKET", "TENCENT_COS_BUCKET")
	region := module4Env("MEDIA_MODULE8_COS_REGION", "MEDIA_MODULE4_COS_REGION", "TENCENT_COS_REGION")
	appID := module4Env("MEDIA_MODULE8_COS_APP_ID", "MEDIA_MODULE4_COS_APP_ID", "MEDIA_MODULE3_COS_APP_ID")
	if appID == "" && strings.Contains(bucket, "-") {
		appID = bucket[strings.LastIndex(bucket, "-")+1:]
	}
	if secretID == "" || secretKey == "" || bucket == "" || region == "" || appID == "" {
		t.Skip("real Module 8 Tencent COS credentials are not configured")
	}
	bucket = resolveModule4TestBucket(t, secretID, secretKey, bucket, region, appID)
	store, err := NewTencentGCStore(TencentGCStoreConfig{SecretID: secretID, SecretKey: secretKey, Bucket: bucket, Region: region})
	if err != nil {
		t.Fatal(err)
	}
	completionStore, err := NewTencentCompletionStore(TencentCompletionStoreConfig{SecretID: secretID, SecretKey: secretKey, Bucket: bucket, Region: region})
	if err != nil {
		t.Fatal(err)
	}
	versioning, _, err := store.client.Bucket.GetVersioning(context.Background())
	if err != nil {
		t.Fatalf("real COS GetVersioning: %v", err)
	}
	if versioning == nil || versioning.Status != "Enabled" {
		t.Fatalf("real COS versioning is not Enabled: %#v", versioning)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	jobID, _ := NewULID()
	key := fmt.Sprintf("staging/module8-gc-acceptance/%s.bin", jobID)
	t.Cleanup(func() { cleanupCompletionCOSKey(t, completionStore, key) })
	version1 := putGCRealVersion(t, ctx, store, key, []byte("module8-fixed-version-one"))
	version2 := putGCRealVersion(t, ctx, store, key, []byte("module8-fixed-version-two"))
	if version1 == version2 || version1 == "" || version2 == "" {
		t.Fatalf("real COS did not create two distinct fixed versions")
	}

	deleted, err := store.DeleteVersion(ctx, key, version1)
	if err != nil {
		t.Fatalf("delete exact COS version: %v", err)
	}
	if deleted.AlreadyMissing {
		t.Fatal("existing exact version was reported missing")
	}
	if _, err := store.client.Object.Head(ctx, key, nil, version1); err == nil || !cos.IsNotFoundError(err) {
		t.Fatalf("deleted fixed version still exists or unexpected HEAD error: %v", err)
	}
	response, err := store.client.Object.Head(ctx, key, nil, version2)
	if err != nil {
		t.Fatalf("non-target version was deleted: %v", err)
	}
	if response != nil && response.Body != nil {
		response.Body.Close()
	}
	retry, err := store.DeleteVersion(ctx, key, version1)
	if err != nil {
		t.Fatalf("idempotent repeated fixed-version delete result=%#v err=%v", retry, err)
	}
	versions, _, err := store.client.Bucket.GetObjectVersions(ctx, &cos.BucketGetObjectVersionsOptions{Prefix: key})
	if err != nil {
		t.Fatal(err)
	}
	foundTarget, foundSurvivor := false, false
	for _, version := range versions.Version {
		if version.Key != key {
			continue
		}
		foundTarget = foundTarget || version.VersionId == version1
		foundSurvivor = foundSurvivor || version.VersionId == version2
	}
	if foundTarget || !foundSurvivor {
		t.Fatalf("fixed deletion target=%v survivor=%v", foundTarget, foundSurvivor)
	}
	t.Logf("PASS real COS versioning=Enabled fixedDelete=%s survivor=%s request=%s repeatedDeleteAccepted=true", redactVersion(version1), redactVersion(version2), redactVersion(deleted.ProviderRequestID))
}

func putGCRealVersion(t *testing.T, ctx context.Context, store *TencentGCStore, key string, body []byte) string {
	t.Helper()
	response, err := store.client.Object.Put(ctx, key, bytes.NewReader(body), nil)
	if err != nil {
		t.Fatal(err)
	}
	if response == nil {
		t.Fatal("real COS PUT returned no response")
	}
	if response.Body != nil {
		response.Body.Close()
	}
	versionID := response.Header.Get("x-cos-version-id")
	if versionID == "" {
		t.Fatal("real COS PUT returned no x-cos-version-id")
	}
	return versionID
}

func TestTencentGCStoreRejectsLatestAndPrefixDeletion(t *testing.T) {
	store := &TencentGCStore{}
	for _, test := range []struct{ key, version string }{{"final/a.webp", ""}, {"", "version"}} {
		if _, err := store.DeleteVersion(context.Background(), test.key, test.version); !errors.Is(err, ErrGCFixedIdentity) {
			t.Fatalf("unsafe identity key=%q version=%q returned %v", test.key, test.version, err)
		}
	}
}
