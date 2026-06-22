package media

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

func TestTencentAuthorizerRequiresFullBucketName(t *testing.T) {
	_, err := NewTencentUploadAuthorizer(TencentUploadAuthorizerConfig{
		SecretID: "id", SecretKey: "key", Bucket: "media", Region: "ap-guangzhou", AppID: "1234567890",
	})
	if err == nil {
		t.Fatal("bucket without AppID suffix was accepted")
	}
}

func TestExactPutPolicyContainsOneKeyAndNoForbiddenAction(t *testing.T) {
	key := "staging/test/owner/asset/nonce.webp"
	raw, err := buildExactPutPolicy("ap-guangzhou", "1234567890", "media-1234567890", key)
	if err != nil {
		t.Fatal(err)
	}
	var policy struct {
		Statement []struct {
			Action   []string `json:"action"`
			Resource []string `json:"resource"`
		} `json:"statement"`
	}
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	if len(policy.Statement) != 1 || !reflect.DeepEqual(policy.Statement[0].Action, []string{"name/cos:PutObject"}) || len(policy.Statement[0].Resource) != 1 || !strings.HasSuffix(policy.Statement[0].Resource[0], "/"+key) {
		t.Fatalf("policy is not exact single-key PutObject: %s", raw)
	}
	for _, forbidden := range []string{"GetObject", "DeleteObject", "ListBucket", "CopyObject", "ACL", "Multipart", "InitiateMultipartUpload", "UploadPart"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("policy contains forbidden action %s: %s", forbidden, raw)
		}
	}
}

func TestUploadAuthorizationDTOContainsNoCredentialField(t *testing.T) {
	typeOfDTO := reflect.TypeOf(UploadAuthorization{})
	for index := 0; index < typeOfDTO.NumField(); index++ {
		name := strings.ToLower(typeOfDTO.Field(index).Name)
		for _, forbidden := range []string{"secret", "credential", "sessiontoken", "tmpsecret"} {
			if strings.Contains(name, forbidden) {
				t.Fatalf("authorization DTO exposes sensitive field %s", typeOfDTO.Field(index).Name)
			}
		}
	}
}

func TestExactSinglePUTPresignUsesTemporaryCredentialWithoutLeakingSecret(t *testing.T) {
	bucketURL, _ := url.Parse("https://media-1234567890.cos.ap-guangzhou.myqcloud.com")
	authorizer := &TencentUploadAuthorizer{
		bucket: "media-1234567890", region: "ap-guangzhou", appID: "1234567890",
		cosClient: cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{}),
		credentialIssuer: func(context.Context, string, string) (temporaryCredential, error) {
			return temporaryCredential{
				secretID: "temporary-id", secretKey: "DO-NOT-LEAK-TEMPORARY-SECRET", token: "temporary-token",
				expiresAt: time.Now().Add(15 * time.Minute),
			}, nil
		},
	}
	authorization, err := authorizer.IssueSinglePUT(context.Background(), AuthorizationRequest{
		TicketID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", ObjectKey: "staging/test/owner/asset/nonce.webp",
		Size: 1024, MD5: "1B2M2Y8AsgTpgAmY7PhCfg==", ContentType: "image/webp", TTL: 2 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(authorization.UploadURL, "DO-NOT-LEAK") {
		t.Fatal("temporary SecretKey leaked into authorization")
	}
	if authorization.RequiredHeaders.ContentLength != 1024 || authorization.RequiredHeaders.ForbidOverwrite != "true" {
		t.Fatalf("authorization did not bind required headers: %#v", authorization)
	}
	if ttl := time.Until(authorization.ExpiresAt); ttl <= 0 || ttl > 2*time.Minute+time.Second {
		t.Fatalf("authorization TTL escaped 120-second boundary: %s", ttl)
	}
}

func TestSignedURLValidationRequiresExactKeyTokenAndHeaders(t *testing.T) {
	value, err := url.Parse("https://media-1234567890.cos.ap-guangzhou.myqcloud.com/staging/test/key.webp?q-header-list=content-length%3Bcontent-md5%3Bcontent-type%3Bhost%3Bx-cos-forbid-overwrite&q-signature=sig&x-cos-security-token=token")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateSignedURL(value, "media-1234567890", "ap-guangzhou", "staging/test/key.webp"); err != nil {
		t.Fatal(err)
	}
	value.RawQuery = "q-header-list=content-type%3Bhost&q-signature=sig&x-cos-security-token=token"
	if err := validateSignedURL(value, "media-1234567890", "ap-guangzhou", "staging/test/key.webp"); err == nil {
		t.Fatal("authorization missing signed length and MD5 was accepted")
	}
}
