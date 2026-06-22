package media

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

// gate0PutSignature contains every request attribute that the Final design
// requires COS to authenticate at the upload boundary. The STS token is a URL
// parameter so the client never needs to re-sign the request.
type gate0PutSignature struct {
	SessionToken    string `url:"x-cos-security-token,omitempty" header:"-"`
	ContentType     string `header:"Content-Type" url:"-"`
	ContentMD5      string `header:"Content-MD5" url:"-"`
	ContentLength   int64  `header:"Content-Length" url:"-"`
	ForbidOverwrite string `header:"x-cos-forbid-overwrite" url:"-"`
}

func TestGate0TencentCOSUploadBoundary(t *testing.T) {
	secretID := os.Getenv("TEST_COS_SECRET_ID")
	secretKey := os.Getenv("TEST_COS_SECRET_KEY")
	bucket := os.Getenv("TEST_COS_BUCKET")
	region := os.Getenv("TEST_COS_REGION")
	appID := os.Getenv("TEST_COS_APP_ID")
	if secretID == "" || secretKey == "" || bucket == "" || region == "" || appID == "" {
		t.Skip("real Tencent COS Gate-0 credentials are not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	adapter, err := NewTencentCOS(secretID, secretKey, bucket, region, appID)
	if err != nil {
		t.Fatal(err)
	}

	key := fmt.Sprintf("staging/gate0/%d/exact.png", time.Now().UnixNano())
	forbiddenKey := strings.Replace(key, "/exact.png", "/other.png", 1)
	finalKey := strings.Replace(key, "staging/gate0/", "final/gate0/", 1)
	for _, cleanupKey := range []string{key, forbiddenKey, finalKey} {
		cleanupKey := cleanupKey
		defer func() { _ = adapter.Delete(context.Background(), cleanupKey) }()
	}

	credential, err := adapter.Issue(ctx, key, 15*time.Minute)
	if err != nil {
		t.Fatalf("issue exact-key STS: %v", err)
	}
	bucketURL, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", bucket, region))
	if err != nil {
		t.Fatal(err)
	}
	temporary := cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Timeout: 20 * time.Second,
		Transport: &cos.AuthorizationTransport{
			SecretID: credential.SecretID, SecretKey: credential.SecretKey, SessionToken: credential.SessionToken,
		},
	})

	payload := []byte("gate-0-single-put-payload")
	digest := md5.Sum(payload)
	contentMD5 := base64.StdEncoding.EncodeToString(digest[:])
	signature := &gate0PutSignature{
		SessionToken: credential.SessionToken, ContentType: "image/png", ContentMD5: contentMD5,
		ContentLength: int64(len(payload)), ForbidOverwrite: "true",
	}
	presigned, err := adapter.cosClient.Object.GetPresignedURL(ctx, http.MethodPut, key, credential.SecretID, credential.SecretKey, 2*time.Minute, signature)
	if err != nil {
		t.Fatalf("presign exact PUT: %v", err)
	}
	headerList := presigned.Query().Get("q-header-list")
	for _, required := range []string{"content-length", "content-md5", "content-type", "host", "x-cos-forbid-overwrite"} {
		if !strings.Contains(headerList, required) {
			t.Fatalf("presigned PUT does not bind required header %q; signed headers=%q", required, headerList)
		}
	}
	if presigned.Query().Get("x-cos-security-token") == "" {
		t.Fatal("presigned PUT is missing the STS session token")
	}

	status, body, err := gate0Put(ctx, presigned, payload, "image/png", contentMD5, true)
	if err != nil {
		t.Fatalf("exact PUT request: %v", err)
	}
	if status < 200 || status >= 300 {
		t.Fatalf("exact PUT status=%d body=%s", status, body)
	}
	t.Logf("PASS exact single PUT: status=%d", status)

	status, _, err = gate0Put(ctx, presigned, append(payload, '!'), "image/png", contentMD5, true)
	if err != nil {
		t.Fatalf("length-negative request failed before COS response: %v", err)
	}
	if status >= 200 && status < 300 {
		t.Fatal("Content-Length mismatch was accepted")
	}
	t.Logf("PASS Content-Length mismatch rejected: status=%d", status)

	wrongDigest := md5.Sum([]byte("different"))
	status, _, err = gate0Put(ctx, presigned, payload, "image/png", base64.StdEncoding.EncodeToString(wrongDigest[:]), true)
	if err != nil {
		t.Fatal(err)
	}
	if status >= 200 && status < 300 {
		t.Fatal("Content-MD5 mismatch was accepted")
	}
	t.Logf("PASS Content-MD5 mismatch rejected: status=%d", status)

	status, _, err = gate0Put(ctx, presigned, payload, "image/jpeg", contentMD5, true)
	if err != nil {
		t.Fatal(err)
	}
	if status >= 200 && status < 300 {
		t.Fatal("Content-Type mismatch was accepted")
	}
	t.Logf("PASS Content-Type mismatch rejected: status=%d", status)

	status, _, err = gate0Put(ctx, presigned, payload, "image/png", contentMD5, true)
	if err != nil {
		t.Fatal(err)
	}
	if status >= 200 && status < 300 {
		t.Fatal("overwrite using the original signed PUT was accepted")
	}
	t.Logf("PASS overwrite rejected: status=%d", status)

	putOptions := &cos.ObjectPutOptions{ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{ContentType: "image/png"}}
	if _, err := temporary.Object.Put(ctx, forbiddenKey, bytes.NewReader(payload), putOptions); err == nil {
		t.Fatal("exact-key STS wrote another staging key")
	}
	t.Log("PASS STS cannot write another staging key")
	if _, err := temporary.Object.Put(ctx, finalKey, bytes.NewReader(payload), putOptions); err == nil {
		t.Fatal("staging STS wrote a final key")
	}
	t.Log("PASS STS cannot write final key")
	if response, err := temporary.Object.Get(ctx, key, nil); err == nil {
		if response != nil && response.Body != nil {
			response.Body.Close()
		}
		t.Fatal("PutObject-only STS read the object")
	}
	t.Log("PASS STS has no GetObject permission")

	result, _, multipartErr := temporary.Object.InitiateMultipartUpload(ctx, key, nil)
	if multipartErr == nil {
		if result != nil && result.UploadID != "" {
			_, _ = adapter.cosClient.Object.AbortMultipartUpload(context.Background(), key, result.UploadID, nil)
		}
		t.Fatal("PutObject-only STS initiated multipart upload")
	}
	t.Log("PASS multipart initiation rejected")
}

func gate0Put(ctx context.Context, signedURL *url.URL, payload []byte, contentType, contentMD5 string, forbidOverwrite bool) (int, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, signedURL.String(), bytes.NewReader(payload))
	if err != nil {
		return 0, "", err
	}
	request.ContentLength = int64(len(payload))
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Content-MD5", contentMD5)
	if forbidOverwrite {
		request.Header.Set("x-cos-forbid-overwrite", "true")
	}
	client := &http.Client{Timeout: 20 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return 0, "", err
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 512))
	return response.StatusCode, strings.TrimSpace(string(data)), nil
}
