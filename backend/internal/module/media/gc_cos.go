package media

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

type TencentGCStoreConfig struct {
	SecretID  string
	SecretKey string
	Bucket    string
	Region    string
}

type TencentGCStore struct{ client *cos.Client }

func NewTencentGCStore(config TencentGCStoreConfig) (*TencentGCStore, error) {
	config.SecretID = strings.TrimSpace(config.SecretID)
	config.SecretKey = strings.TrimSpace(config.SecretKey)
	config.Bucket = strings.TrimSpace(config.Bucket)
	config.Region = strings.TrimSpace(config.Region)
	if config.SecretID == "" || config.SecretKey == "" || config.Bucket == "" || config.Region == "" {
		return nil, errors.New("COS service identity, full bucket and region are required")
	}
	bucketURL, err := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", config.Bucket, config.Region))
	if err != nil {
		return nil, err
	}
	return &TencentGCStore{client: cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Timeout:   30 * time.Second,
		Transport: &cos.AuthorizationTransport{SecretID: config.SecretID, SecretKey: config.SecretKey},
	})}, nil
}

func (s *TencentGCStore) DeleteVersion(ctx context.Context, key, versionID string) (GCDeleteResult, error) {
	key, versionID = strings.TrimSpace(key), strings.TrimSpace(versionID)
	if key == "" || versionID == "" {
		return GCDeleteResult{}, ErrGCFixedIdentity
	}
	response, err := s.client.Object.Delete(ctx, key, &cos.ObjectDeleteOptions{VersionId: versionID})
	if err != nil {
		if cos.IsNotFoundError(err) {
			return GCDeleteResult{AlreadyMissing: true}, nil
		}
		return GCDeleteResult{}, err
	}
	result := GCDeleteResult{}
	if response != nil {
		result.ProviderRequestID = response.Header.Get("x-cos-request-id")
		if response.Body != nil {
			response.Body.Close()
		}
	}
	return result, nil
}

var _ GCObjectStore = (*TencentGCStore)(nil)
