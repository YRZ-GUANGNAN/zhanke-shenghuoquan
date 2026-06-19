package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

type TencentCOS struct {
	bucket, region, appID string
	secretID, secretKey   string
	stsClient             *sts.Client
	cosClient             *cos.Client
}

func NewTencentCOS(secretID, secretKey, bucket, region, appID string) (*TencentCOS, error) {
	if secretID == "" || secretKey == "" || bucket == "" || region == "" || appID == "" {
		return nil, errors.New("COS credentials, bucket, region and app id are required")
	}
	credential := common.NewCredential(secretID, secretKey)
	clientProfile := profile.NewClientProfile()
	stsClient, err := sts.NewClient(credential, "", clientProfile)
	if err != nil {
		return nil, err
	}
	bucketURL, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", bucket, region))
	if err != nil {
		return nil, err
	}
	cosClient := cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Timeout:   15 * time.Second,
		Transport: &cos.AuthorizationTransport{SecretID: secretID, SecretKey: secretKey},
	})
	return &TencentCOS{bucket: bucket, region: region, appID: appID, secretID: secretID, secretKey: secretKey, stsClient: stsClient, cosClient: cosClient}, nil
}

func (c *TencentCOS) Issue(ctx context.Context, objectKey string, ttl time.Duration) (TemporaryCredentials, error) {
	statement := map[string]any{
		"version": "2.0",
		"statement": []map[string]any{{
			"effect":   "allow",
			"action":   []string{"name/cos:PutObject"},
			"resource": []string{fmt.Sprintf("qcs::cos:%s:uid/%s:%s/%s", c.region, c.appID, c.bucket, strings.TrimLeft(objectKey, "/"))},
		}},
	}
	policy, err := json.Marshal(statement)
	if err != nil {
		return TemporaryCredentials{}, err
	}
	duration := uint64(ttl / time.Second)
	request := sts.NewGetFederationTokenRequest()
	name := "campusmedia"
	encodedPolicy := url.QueryEscape(string(policy))
	request.Name, request.Policy, request.DurationSeconds = &name, &encodedPolicy, &duration
	response, err := c.stsClient.GetFederationTokenWithContext(ctx, request)
	if err != nil {
		return TemporaryCredentials{}, err
	}
	if response == nil || response.Response == nil || response.Response.Credentials == nil || response.Response.ExpiredTime == nil {
		return TemporaryCredentials{}, errors.New("STS returned incomplete credentials")
	}
	credentials := response.Response.Credentials
	if credentials.TmpSecretId == nil || credentials.TmpSecretKey == nil || credentials.Token == nil {
		return TemporaryCredentials{}, errors.New("STS returned incomplete secret fields")
	}
	return TemporaryCredentials{
		SecretID: *credentials.TmpSecretId, SecretKey: *credentials.TmpSecretKey, SessionToken: *credentials.Token,
		StartTime: time.Now().Add(-60 * time.Second).Unix(), ExpiredTime: int64(*response.Response.ExpiredTime),
	}, nil
}

func (c *TencentCOS) Stat(ctx context.Context, objectKey string) (ObjectInfo, error) {
	response, err := c.cosClient.Object.Head(ctx, objectKey, nil)
	if err != nil {
		return ObjectInfo{}, err
	}
	if response.Body != nil {
		defer response.Body.Close()
	}
	size, err := strconv.ParseInt(response.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("invalid COS content length: %w", err)
	}
	return ObjectInfo{Size: size, ContentType: response.Header.Get("Content-Type")}, nil
}

func (c *TencentCOS) ReadPrefix(ctx context.Context, objectKey string, length int64) ([]byte, error) {
	response, err := c.cosClient.Object.Get(ctx, objectKey, &cos.ObjectGetOptions{Range: fmt.Sprintf("bytes=0-%d", length-1)})
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	return io.ReadAll(io.LimitReader(response.Body, length))
}

func (c *TencentCOS) SignedGetURL(ctx context.Context, objectKey string, ttl time.Duration) (string, error) {
	value, err := c.cosClient.Object.GetPresignedURL(ctx, http.MethodGet, objectKey, c.secretID, c.secretKey, ttl, nil)
	if err != nil {
		return "", err
	}
	return value.String(), nil
}

func (c *TencentCOS) Delete(ctx context.Context, objectKey string) error {
	response, err := c.cosClient.Object.Delete(ctx, objectKey)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err != nil && response != nil && response.StatusCode == http.StatusNotFound {
		return nil
	}
	return err
}

var _ CredentialIssuer = (*TencentCOS)(nil)
var _ ObjectStore = (*TencentCOS)(nil)
