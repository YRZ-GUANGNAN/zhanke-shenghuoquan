package media

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

func TestMediaDisplaySignedGetAgainstRealCOS(t *testing.T) {
	if os.Getenv("MEDIA_DISPLAY_RUN_REAL_COS") != "1" {
		t.Skip("set MEDIA_DISPLAY_RUN_REAL_COS=1 to run real COS fixed-version display GET acceptance")
	}
	_ = godotenv.Load("../../../.env")
	secretID := module4Env("MEDIA_DISPLAY_COS_SECRET_ID", "MEDIA_MODULE4_COS_SECRET_ID", "TENCENT_COS_SECRET_ID", "COS_SECRET_ID")
	secretKey := module4Env("MEDIA_DISPLAY_COS_SECRET_KEY", "MEDIA_MODULE4_COS_SECRET_KEY", "TENCENT_COS_SECRET_KEY", "COS_SECRET_KEY")
	bucket := module4Env("MEDIA_DISPLAY_COS_BUCKET", "MEDIA_MODULE4_COS_BUCKET", "TENCENT_COS_BUCKET", "COS_BUCKET")
	region := module4Env("MEDIA_DISPLAY_COS_REGION", "MEDIA_MODULE4_COS_REGION", "TENCENT_COS_REGION", "COS_REGION")
	appID := module4Env("MEDIA_DISPLAY_COS_APP_ID", "MEDIA_MODULE4_COS_APP_ID", "COS_APP_ID")
	if appID == "" && strings.Contains(bucket, "-") {
		appID = bucket[strings.LastIndex(bucket, "-")+1:]
	}
	if secretID == "" || secretKey == "" || bucket == "" || region == "" || appID == "" {
		t.Skip("real display COS credentials are not configured")
	}
	bucket = resolveModule4TestBucket(t, secretID, secretKey, bucket, region, appID)
	store, err := NewTencentCompletionStore(TencentCompletionStoreConfig{SecretID: secretID, SecretKey: secretKey, Bucket: bucket, Region: region})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	key := fmt.Sprintf("final/display-acceptance/%d.txt", time.Now().UnixNano())
	payload := "display-fixed-version-acceptance"
	t.Cleanup(func() { cleanupCompletionCOSKey(t, store, key) })
	response, err := store.cosClient.Object.Put(ctx, key, strings.NewReader(payload), nil)
	if err != nil {
		t.Fatalf("real COS PUT display object: %v", err)
	}
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	versionID := ""
	if response != nil {
		versionID = response.Header.Get("x-cos-version-id")
	}
	if versionID == "" {
		t.Fatal("real COS bucket versioning is required for display fixed-version GET")
	}
	signedURL, err := store.SignedGetURL(ctx, key, versionID, MediaDisplayURLTTL)
	if err != nil {
		t.Fatalf("real COS SignedGetURL: %v", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, signedURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("real COS signed GET: %v", err)
	}
	defer result.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(result.Body, 1024))
	if result.StatusCode != http.StatusOK || string(body) != payload {
		t.Fatalf("real COS signed GET status=%d body=%q", result.StatusCode, string(body))
	}
	t.Logf("PASS real COS fixed-version display GET key=%s version=%s", redactVersion(key), redactVersion(versionID))
}
