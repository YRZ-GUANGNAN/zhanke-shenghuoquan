package media

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestTencentUploadAuthorizerAgainstRealSTS(t *testing.T) {
	config := TencentUploadAuthorizerConfig{
		SecretID: os.Getenv("MEDIA_MODULE3_COS_SECRET_ID"), SecretKey: os.Getenv("MEDIA_MODULE3_COS_SECRET_KEY"),
		Bucket: os.Getenv("MEDIA_MODULE3_COS_BUCKET"), Region: os.Getenv("MEDIA_MODULE3_COS_REGION"),
		AppID: os.Getenv("MEDIA_MODULE3_COS_APP_ID"), STSTTL: 15 * time.Minute,
	}
	if config.SecretID == "" || config.SecretKey == "" || config.Bucket == "" || config.Region == "" || config.AppID == "" {
		t.Skip("real Module 3 Tencent STS credentials are not configured")
	}
	authorizer, err := NewTencentUploadAuthorizer(config)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	authorization, err := authorizer.IssueSinglePUT(ctx, AuthorizationRequest{
		TicketID:  "01ARZ3NDEKTSV4RRFFQ69G5FAV",
		ObjectKey: "staging/module3-acceptance/0000/owner/01ARZ3NDEKTSV4RRFFQ69G5FAV/00000000000000000000000000000000.webp",
		Size:      1024, MD5: "1B2M2Y8AsgTpgAmY7PhCfg==", ContentType: "image/webp", TTL: 2 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if authorization.UploadURL == "" || authorization.RequiredHeaders.ForbidOverwrite != "true" {
		t.Fatal("real STS did not produce exact single-PUT authorization")
	}
	parsed, err := url.Parse(authorization.UploadURL)
	if err != nil {
		t.Fatal(err)
	}
	headerList := parsed.Query().Get("q-header-list")
	for _, required := range []string{"content-length", "content-md5", "content-type", "host", "x-cos-forbid-overwrite"} {
		if !containsSignedHeader(headerList, required) {
			t.Fatalf("real STS authorization is missing signed header %s", required)
		}
	}
	if parsed.Query().Get("x-cos-security-token") == "" || strings.Contains(authorization.UploadURL, config.SecretKey) {
		t.Fatal("real STS authorization is missing token or leaked long-term SecretKey")
	}
	if ttl := time.Until(authorization.ExpiresAt); ttl <= 0 || ttl > 2*time.Minute+time.Second {
		t.Fatalf("real authorization TTL escaped 120-second boundary: %s", ttl)
	}
}
