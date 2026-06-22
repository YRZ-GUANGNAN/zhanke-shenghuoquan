package media

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	sts "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/sts/v20180813"
	cos "github.com/tencentyun/cos-go-sdk-v5"
)

type TencentUploadAuthorizerConfig struct {
	SecretID  string
	SecretKey string
	Bucket    string
	Region    string
	AppID     string
	STSTTL    time.Duration
}

type TencentUploadAuthorizer struct {
	bucket, region, appID string
	stsTTL                time.Duration
	stsClient             *sts.Client
	cosClient             *cos.Client
	credentialIssuer      func(context.Context, string, string) (temporaryCredential, error)
}

type exactSinglePUTSignature struct {
	SessionToken    string `url:"x-cos-security-token,omitempty" header:"-"`
	ContentType     string `header:"Content-Type" url:"-"`
	ContentMD5      string `header:"Content-MD5" url:"-"`
	ContentLength   int64  `header:"Content-Length" url:"-"`
	ForbidOverwrite string `header:"x-cos-forbid-overwrite" url:"-"`
}

type temporaryCredential struct {
	secretID, secretKey, token string
	expiresAt                  time.Time
}

func NewTencentUploadAuthorizer(config TencentUploadAuthorizerConfig) (*TencentUploadAuthorizer, error) {
	config.SecretID = strings.TrimSpace(config.SecretID)
	config.SecretKey = strings.TrimSpace(config.SecretKey)
	config.Bucket = strings.TrimSpace(config.Bucket)
	config.Region = strings.TrimSpace(config.Region)
	config.AppID = strings.TrimSpace(config.AppID)
	if config.SecretID == "" || config.SecretKey == "" || config.Bucket == "" || config.Region == "" || config.AppID == "" {
		return nil, errors.New("COS SecretId, SecretKey, full bucket, region and AppID are required")
	}
	if !strings.HasSuffix(config.Bucket, "-"+config.AppID) {
		return nil, errors.New("COS bucket must include the matching AppID suffix")
	}
	if config.STSTTL < 15*time.Minute {
		config.STSTTL = 15 * time.Minute
	}
	credential := common.NewCredential(config.SecretID, config.SecretKey)
	clientProfile := profile.NewClientProfile()
	stsClient, err := sts.NewClient(credential, config.Region, clientProfile)
	if err != nil {
		return nil, err
	}
	bucketURL, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", config.Bucket, config.Region))
	if err != nil {
		return nil, err
	}
	cosClient := cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{Timeout: 15 * time.Second})
	authorizer := &TencentUploadAuthorizer{
		bucket: config.Bucket, region: config.Region, appID: config.AppID,
		stsTTL: config.STSTTL, stsClient: stsClient, cosClient: cosClient,
	}
	authorizer.credentialIssuer = authorizer.issueExactKeyCredential
	return authorizer, nil
}

func (a *TencentUploadAuthorizer) IssueSinglePUT(ctx context.Context, request AuthorizationRequest) (UploadAuthorization, error) {
	if err := validateAuthorizationRequest(request); err != nil {
		return UploadAuthorization{}, err
	}
	if a.credentialIssuer == nil {
		return UploadAuthorization{}, errors.New("temporary credential issuer is not configured")
	}
	credential, err := a.credentialIssuer(ctx, request.TicketID, request.ObjectKey)
	if err != nil {
		return UploadAuthorization{}, err
	}
	ttl := min(request.TTL, time.Until(credential.expiresAt)-time.Minute)
	if ttl <= 0 || ttl > 2*time.Minute {
		return UploadAuthorization{}, errors.New("temporary credential lifetime cannot satisfy upload authorization")
	}
	options := &exactSinglePUTSignature{
		SessionToken: credential.token, ContentType: request.ContentType, ContentMD5: request.MD5,
		ContentLength: int64(request.Size), ForbidOverwrite: "true",
	}
	signedURL, err := a.cosClient.Object.GetPresignedURL(ctx, http.MethodPut, request.ObjectKey, credential.secretID, credential.secretKey, ttl, options)
	if err != nil {
		return UploadAuthorization{}, err
	}
	if err := validateSignedURL(signedURL, a.bucket, a.region, request.ObjectKey); err != nil {
		return UploadAuthorization{}, err
	}
	return UploadAuthorization{
		UploadURL: signedURL.String(),
		RequiredHeaders: RequiredUploadHeaders{
			ContentType: request.ContentType, ContentMD5: request.MD5,
			ContentLength: request.Size, ForbidOverwrite: "true",
		},
		ExpiresAt: time.Now().UTC().Add(ttl),
	}, nil
}

func (a *TencentUploadAuthorizer) issueExactKeyCredential(ctx context.Context, ticketID, objectKey string) (temporaryCredential, error) {
	policy, err := buildExactPutPolicy(a.region, a.appID, a.bucket, objectKey)
	if err != nil {
		return temporaryCredential{}, err
	}
	duration := uint64(a.stsTTL / time.Second)
	name := "media-" + strings.ToLower(ticketID[len(ticketID)-12:])
	encodedPolicy := url.QueryEscape(string(policy))
	request := sts.NewGetFederationTokenRequest()
	request.Name, request.Policy, request.DurationSeconds = &name, &encodedPolicy, &duration
	response, err := a.stsClient.GetFederationTokenWithContext(ctx, request)
	if err != nil {
		return temporaryCredential{}, err
	}
	if response == nil || response.Response == nil || response.Response.Credentials == nil || response.Response.ExpiredTime == nil {
		return temporaryCredential{}, errors.New("STS returned incomplete credentials")
	}
	credentials := response.Response.Credentials
	if credentials.TmpSecretId == nil || credentials.TmpSecretKey == nil || credentials.Token == nil {
		return temporaryCredential{}, errors.New("STS returned incomplete secret fields")
	}
	return temporaryCredential{
		secretID: *credentials.TmpSecretId, secretKey: *credentials.TmpSecretKey, token: *credentials.Token,
		expiresAt: time.Unix(int64(*response.Response.ExpiredTime), 0).UTC(),
	}, nil
}

func buildExactPutPolicy(region, appID, bucket, objectKey string) ([]byte, error) {
	resource := fmt.Sprintf("qcs::cos:%s:uid/%s:%s/%s", region, appID, bucket, strings.TrimLeft(objectKey, "/"))
	return json.Marshal(map[string]any{
		"version": "2.0",
		"statement": []map[string]any{{
			"effect": "allow", "action": []string{"name/cos:PutObject"}, "resource": []string{resource},
		}},
	})
}

func validateAuthorizationRequest(request AuthorizationRequest) error {
	if !ValidULID(request.TicketID) || request.Size == 0 || request.Size > 10<<20 || request.TTL <= 0 || request.TTL > 2*time.Minute {
		return ErrInvalidUploadDeclaration
	}
	if !strings.HasPrefix(request.ObjectKey, "staging/") || strings.Contains(request.ObjectKey, "..") || strings.ContainsAny(request.ObjectKey, "\r\n\\") {
		return ErrInvalidUploadDeclaration
	}
	if len(request.MD5) != 24 {
		return ErrInvalidUploadDeclaration
	}
	decodedMD5, err := base64.StdEncoding.DecodeString(request.MD5)
	if err != nil || len(decodedMD5) != 16 || base64.StdEncoding.EncodeToString(decodedMD5) != request.MD5 {
		return ErrInvalidUploadDeclaration
	}
	switch request.ContentType {
	case "image/jpeg", "image/png", "image/webp":
		return nil
	default:
		return ErrInvalidUploadDeclaration
	}
}

func validateSignedURL(value *url.URL, bucket, region, objectKey string) error {
	if value == nil || value.Scheme != "https" || value.Host != fmt.Sprintf("%s.cos.%s.myqcloud.com", bucket, region) {
		return errors.New("presigned URL target mismatch")
	}
	decodedPath, err := url.PathUnescape(strings.TrimPrefix(value.EscapedPath(), "/"))
	if err != nil || decodedPath != objectKey {
		return errors.New("presigned URL object key mismatch")
	}
	headerList := value.Query().Get("q-header-list")
	for _, required := range []string{"content-length", "content-md5", "content-type", "host", "x-cos-forbid-overwrite"} {
		if !containsSignedHeader(headerList, required) {
			return fmt.Errorf("presigned URL does not bind %s", required)
		}
	}
	if value.Query().Get("x-cos-security-token") == "" || value.Query().Get("q-signature") == "" {
		return errors.New("presigned URL is missing temporary token or signature")
	}
	return nil
}

func containsSignedHeader(list, expected string) bool {
	for _, value := range strings.Split(list, ";") {
		if value == expected {
			return true
		}
	}
	return false
}

func (h RequiredUploadHeaders) AsMap() map[string]string {
	return map[string]string{
		"Content-Type": h.ContentType, "Content-MD5": h.ContentMD5,
		"Content-Length": strconv.FormatUint(h.ContentLength, 10), "x-cos-forbid-overwrite": h.ForbidOverwrite,
	}
}

var _ UploadAuthorizer = (*TencentUploadAuthorizer)(nil)
