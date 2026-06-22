package media

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	cos "github.com/tencentyun/cos-go-sdk-v5"
)

// TestEnableModule4COSVersioning is an explicit, one-purpose infrastructure
// gate action. It never runs during ordinary test suites.
func TestEnableModule4COSVersioning(t *testing.T) {
	if os.Getenv("MEDIA_MODULE4_ENABLE_VERSIONING") != "1" {
		t.Skip("set MEDIA_MODULE4_ENABLE_VERSIONING=1 for the authorized versioning gate action")
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
		t.Fatal("real Module 4 Tencent COS credentials are not configured")
	}
	bucket = resolveModule4TestBucket(t, secretID, secretKey, bucket, region, appID)
	store, err := NewTencentCompletionStore(TencentCompletionStoreConfig{SecretID: secretID, SecretKey: secretKey, Bucket: bucket, Region: region})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	current, _, err := store.cosClient.Bucket.GetVersioning(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if current != nil && current.Status == "Enabled" {
		t.Log("PASS real COS versioning already Enabled")
		return
	}
	if _, err := store.cosClient.Bucket.PutVersioning(ctx, &cos.BucketPutVersionOptions{Status: "Enabled"}); err != nil {
		t.Fatalf("enable real COS versioning: %v", err)
	}
	for attempt := 0; attempt < 10; attempt++ {
		confirmed, _, err := store.cosClient.Bucket.GetVersioning(ctx)
		if err == nil && confirmed != nil && confirmed.Status == "Enabled" {
			t.Log("PASS real COS versioning changed to Enabled and was read back")
			return
		}
		time.Sleep(time.Second)
	}
	t.Fatal("real COS GetVersioning did not converge to Enabled")
}
