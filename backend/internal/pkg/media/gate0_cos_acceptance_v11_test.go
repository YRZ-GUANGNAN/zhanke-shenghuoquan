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

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

type gate0V11PutSignature struct {
	SessionToken    string `url:"x-cos-security-token,omitempty" header:"-"`
	ContentType     string `header:"Content-Type" url:"-"`
	ContentMD5      string `header:"Content-MD5" url:"-"`
	ContentLength   int64  `header:"Content-Length" url:"-"`
	ForbidOverwrite string `header:"x-cos-forbid-overwrite" url:"-"`
}

type gate0V11Authorization struct {
	URL        *url.URL
	Credential TemporaryCredentials
}

func TestGate0TencentCOSUploadBoundaryV11(t *testing.T) {
	secretID := os.Getenv("TEST_COS_SECRET_ID")
	secretKey := os.Getenv("TEST_COS_SECRET_KEY")
	bucket := os.Getenv("TEST_COS_BUCKET")
	region := os.Getenv("TEST_COS_REGION")
	appID := os.Getenv("TEST_COS_APP_ID")
	if secretID == "" || secretKey == "" || bucket == "" || region == "" || appID == "" {
		t.Skip("real Tencent COS Gate-0 credentials are not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	adapter, err := NewTencentCOS(secretID, secretKey, bucket, region, appID)
	if err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("staging/gate0-v11/%d/", time.Now().UnixNano())
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cleanupCancel()
		if err := gate0V11Cleanup(cleanupCtx, adapter, prefix); err != nil {
			t.Errorf("Gate-0 cleanup failed: %v", err)
		}
	}()

	payload := []byte("gate-0-v11-single-put-payload")
	contentMD5 := gate0V11MD5(payload)

	lengthKey := prefix + "length.png"
	lengthAuth := gate0V11Presign(t, ctx, adapter, lengthKey, payload, "image/png")
	status, _, err := gate0V11Put(ctx, lengthAuth.URL, append(payload, '!'), "image/png", contentMD5)
	gate0V11RequireRejectedStrict(t, "Content-Length tamper", status, err)
	gate0V11RequireAbsent(t, ctx, adapter, lengthKey)

	md5Key := prefix + "md5.png"
	md5Auth := gate0V11Presign(t, ctx, adapter, md5Key, payload, "image/png")
	status, _, err = gate0V11Put(ctx, md5Auth.URL, payload, "image/png", gate0V11MD5([]byte("different")))
	gate0V11RequireRejectedStrict(t, "Content-MD5 tamper", status, err)
	gate0V11RequireAbsent(t, ctx, adapter, md5Key)

	typeKey := prefix + "type.png"
	typeAuth := gate0V11Presign(t, ctx, adapter, typeKey, payload, "image/png")
	status, _, err = gate0V11Put(ctx, typeAuth.URL, payload, "image/jpeg", contentMD5)
	gate0V11RequireRejectedStrict(t, "Content-Type tamper", status, err)
	gate0V11RequireAbsent(t, ctx, adapter, typeKey)

	exactKey := prefix + "exact.png"
	exactAuth := gate0V11Presign(t, ctx, adapter, exactKey, payload, "image/png")
	status, body, err := gate0V11Put(ctx, exactAuth.URL, payload, "image/png", contentMD5)
	if err != nil || status < 200 || status >= 300 {
		t.Fatalf("exact staging PUT failed: status=%d err=%v body=%s", status, err, body)
	}
	t.Logf("PASS exact staging Key single PUT: status=%d", status)
	status, _, err = gate0V11Put(ctx, exactAuth.URL, payload, "image/png", contentMD5)
	gate0V11RequireRejectedStrict(t, "signed PUT overwrite", status, err)

	temporary := gate0V11TemporaryClient(t, bucket, region, exactAuth.Credential)
	otherKey := prefix + "other.png"
	finalKey := strings.Replace(prefix, "staging/gate0-v11/", "final/gate0-v11/", 1) + "forbidden.png"
	putOpt := &cos.ObjectPutOptions{ObjectPutHeaderOptions: &cos.ObjectPutHeaderOptions{ContentType: "image/png"}}
	response, err := temporary.Object.Put(ctx, otherKey, bytes.NewReader(payload), putOpt)
	gate0V11RequireCOSDenied(t, "other staging Key PutObject", response, err)
	response, err = temporary.Object.Put(ctx, finalKey, bytes.NewReader(payload), putOpt)
	gate0V11RequireCOSDenied(t, "final Key PutObject", response, err)

	getResponse, err := temporary.Object.Get(ctx, exactKey, nil)
	if getResponse != nil && getResponse.Body != nil {
		getResponse.Body.Close()
	}
	gate0V11RequireCOSDenied(t, "GetObject", getResponse, err)
	deleteResponse, err := temporary.Object.Delete(ctx, exactKey)
	gate0V11RequireCOSDenied(t, "DeleteObject", deleteResponse, err)
	_, listResponse, err := temporary.Bucket.Get(ctx, &cos.BucketGetOptions{Prefix: prefix})
	gate0V11RequireCOSDenied(t, "ListBucket", listResponse, err)

	initResult, initResponse, err := temporary.Object.InitiateMultipartUpload(ctx, exactKey, nil)
	if err == nil && initResult != nil && initResult.UploadID != "" {
		_, _ = adapter.cosClient.Object.AbortMultipartUpload(context.Background(), exactKey, initResult.UploadID)
	}
	gate0V11RequireCOSDenied(t, "multipart initiate", initResponse, err)

	fakeUploadID := "gate0-v11-forbidden-upload-id"
	partResponse, err := temporary.Object.UploadPart(ctx, exactKey, fakeUploadID, 1, bytes.NewReader(payload), nil)
	gate0V11RequireCOSDenied(t, "multipart upload part", partResponse, err)
	_, partsResponse, err := temporary.Object.ListParts(ctx, exactKey, fakeUploadID, nil)
	gate0V11RequireCOSDenied(t, "multipart list parts", partsResponse, err)
	_, completeResponse, err := temporary.Object.CompleteMultipartUpload(ctx, exactKey, fakeUploadID, gate0V11CompleteOptions())
	gate0V11RequireCOSDenied(t, "multipart complete", completeResponse, err)
	abortResponse, err := temporary.Object.AbortMultipartUpload(ctx, exactKey, fakeUploadID)
	gate0V11RequireCOSDenied(t, "multipart abort", abortResponse, err)
	_, multipartListResponse, err := temporary.Bucket.ListMultipartUploads(ctx, &cos.ListMultipartUploadsOptions{Prefix: prefix})
	gate0V11RequireCOSDenied(t, "multipart list uploads", multipartListResponse, err)

	if err := gate0V11Cleanup(ctx, adapter, prefix); err != nil {
		t.Fatalf("cleanup Gate-0 objects: %v", err)
	}
	gate0V11RequirePrefixEmpty(t, ctx, adapter, prefix)
}

func gate0V11Presign(t *testing.T, ctx context.Context, adapter *TencentCOS, key string, payload []byte, contentType string) gate0V11Authorization {
	t.Helper()
	credential, err := adapter.Issue(ctx, key, 15*time.Minute)
	if err != nil {
		t.Fatalf("GetFederationToken for %s: %v", key, err)
	}
	opt := &gate0V11PutSignature{
		SessionToken: credential.SessionToken, ContentType: contentType, ContentMD5: gate0V11MD5(payload),
		ContentLength: int64(len(payload)), ForbidOverwrite: "true",
	}
	presigned, err := adapter.cosClient.Object.GetPresignedURL(ctx, http.MethodPut, key, credential.SecretID, credential.SecretKey, 2*time.Minute, opt)
	if err != nil {
		t.Fatalf("presign %s: %v", key, err)
	}
	headerList := presigned.Query().Get("q-header-list")
	for _, required := range []string{"content-length", "content-md5", "content-type", "host", "x-cos-forbid-overwrite"} {
		if !strings.Contains(headerList, required) {
			t.Fatalf("presigned request is missing signed header %s; list=%s", required, headerList)
		}
	}
	if presigned.Query().Get("x-cos-security-token") == "" {
		t.Fatal("presigned request is missing x-cos-security-token")
	}
	return gate0V11Authorization{URL: presigned, Credential: credential}
}

func gate0V11TemporaryClient(t *testing.T, bucket, region string, credential TemporaryCredentials) *cos.Client {
	t.Helper()
	bucketURL, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", bucket, region))
	if err != nil {
		t.Fatal(err)
	}
	return cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Timeout: 20 * time.Second,
		Transport: &cos.AuthorizationTransport{
			SecretID: credential.SecretID, SecretKey: credential.SecretKey, SessionToken: credential.SessionToken,
		},
	})
}

func gate0V11Put(ctx context.Context, signedURL *url.URL, payload []byte, contentType, contentMD5 string) (int, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, signedURL.String(), bytes.NewReader(payload))
	if err != nil {
		return 0, "", err
	}
	request.ContentLength = int64(len(payload))
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Content-MD5", contentMD5)
	request.Header.Set("x-cos-forbid-overwrite", "true")
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if err != nil {
		return 0, "", err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
	return response.StatusCode, strings.TrimSpace(string(body)), nil
}

func gate0V11RequireRejected(t *testing.T, label string, status int, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s did not receive a COS response: %v", label, err)
	}
	if status >= 200 && status < 300 {
		t.Fatalf("%s was accepted: status=%d", label, status)
	}
	t.Logf("PASS %s rejected: status=%d", label, status)
}

func gate0V11RequireCOSDenied(t *testing.T, label string, response *cos.Response, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s unexpectedly succeeded", label)
	}
	status := 0
	if response != nil && response.Response != nil {
		status = response.StatusCode
	}
	var cosError *cos.ErrorResponse
	if errors.As(err, &cosError) && cosError.Response != nil {
		status = cosError.Response.StatusCode
	}
	if status != http.StatusForbidden {
		t.Fatalf("%s failed for a non-policy reason: status=%d err=%v", label, status, err)
	}
	t.Logf("PASS %s denied by policy: status=%d", label, status)
}

func gate0V11RequireAbsent(t *testing.T, ctx context.Context, adapter *TencentCOS, key string) {
	t.Helper()
	if _, err := adapter.Stat(ctx, key); err == nil {
		t.Fatalf("rejected request left an object at %s", key)
	}
}

func gate0V11Cleanup(ctx context.Context, adapter *TencentCOS, prefix string) error {
	uploads, _, err := adapter.cosClient.Bucket.ListMultipartUploads(ctx, &cos.ListMultipartUploadsOptions{Prefix: prefix})
	if err != nil {
		return fmt.Errorf("list multipart uploads: %w", err)
	}
	for _, upload := range uploads.Uploads {
		if _, err := adapter.cosClient.Object.AbortMultipartUpload(ctx, upload.Key, upload.UploadID); err != nil {
			return fmt.Errorf("abort multipart %s: %w", upload.Key, err)
		}
	}
	versions, _, err := adapter.cosClient.Bucket.GetObjectVersions(ctx, &cos.BucketGetObjectVersionsOptions{Prefix: prefix})
	if err != nil {
		return fmt.Errorf("list object versions: %w", err)
	}
	for _, version := range versions.Version {
		if _, err := adapter.cosClient.Object.Delete(ctx, version.Key, &cos.ObjectDeleteOptions{VersionId: version.VersionId}); err != nil {
			return fmt.Errorf("delete version %s: %w", version.Key, err)
		}
	}
	for _, marker := range versions.DeleteMarker {
		if _, err := adapter.cosClient.Object.Delete(ctx, marker.Key, &cos.ObjectDeleteOptions{VersionId: marker.VersionId}); err != nil {
			return fmt.Errorf("delete marker %s: %w", marker.Key, err)
		}
	}
	return nil
}

func gate0V11RequirePrefixEmpty(t *testing.T, ctx context.Context, adapter *TencentCOS, prefix string) {
	t.Helper()
	versions, _, err := adapter.cosClient.Bucket.GetObjectVersions(ctx, &cos.BucketGetObjectVersionsOptions{Prefix: prefix})
	if err != nil {
		t.Fatalf("verify object cleanup: %v", err)
	}
	if len(versions.Version) != 0 || len(versions.DeleteMarker) != 0 {
		t.Fatalf("Gate-0 object versions remain: versions=%d markers=%d", len(versions.Version), len(versions.DeleteMarker))
	}
	uploads, _, err := adapter.cosClient.Bucket.ListMultipartUploads(ctx, &cos.ListMultipartUploadsOptions{Prefix: prefix})
	if err != nil {
		t.Fatalf("verify multipart cleanup: %v", err)
	}
	if len(uploads.Uploads) != 0 {
		t.Fatalf("Gate-0 multipart uploads remain: %d", len(uploads.Uploads))
	}
	t.Log("PASS cleanup: no object version, delete marker, or multipart residue")
}

func gate0V11MD5(value []byte) string {
	digest := md5.Sum(value)
	return base64.StdEncoding.EncodeToString(digest[:])
}
