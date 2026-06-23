package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	cos "github.com/tencentyun/cos-go-sdk-v5"
)

var (
	ErrCompletionInProgress       = errors.New("media upload completion is already in progress")
	ErrCompletionInvariant        = errors.New("media upload completion invariant violated")
	ErrCompletionValidation       = errors.New("media upload content validation failed")
	ErrCompletionAttemptsExceeded = errors.New("media upload completion attempts exceeded")
	ErrObjectNotFound             = errors.New("media object not found")
	ErrObjectAlreadyExists        = errors.New("media object already exists")
)

type CompletionPhase string

const (
	CompletionValidating       CompletionPhase = "validating"
	CompletionPromoting        CompletionPhase = "promoting"
	CompletionFinalReady       CompletionPhase = "final_ready"
	CompletionValidationFailed CompletionPhase = "validation_failed"
)

type UploadCompletion struct {
	TicketID             string          `gorm:"column:ticket_id;size:26;primaryKey"`
	AssetID              uint64          `gorm:"column:asset_id;not null;uniqueIndex:uq_media_completion_asset"`
	Phase                CompletionPhase `gorm:"column:phase;size:24;not null"`
	StagingVersionID     *string         `gorm:"column:staging_version_id;size:128"`
	StagingETag          *string         `gorm:"column:staging_etag;size:128"`
	StagingLastModified  *time.Time      `gorm:"column:staging_last_modified"`
	TrustedSHA256        *string         `gorm:"column:trusted_sha256;size:64"`
	VerifiedMIME         *string         `gorm:"column:verified_mime;size:64"`
	VerifiedSize         *uint64         `gorm:"column:verified_size"`
	VerifiedWidth        *uint32         `gorm:"column:verified_width"`
	VerifiedHeight       *uint32         `gorm:"column:verified_height"`
	FinalKey             *string         `gorm:"column:final_key;size:512"`
	FinalVersionID       *string         `gorm:"column:final_version_id;size:128"`
	FinalETag            *string         `gorm:"column:final_etag;size:128"`
	FinalCRC64           *string         `gorm:"column:final_crc64;size:64"`
	LeaseToken           *string         `gorm:"column:lease_token;size:26"`
	LeaseExpiresAt       *time.Time      `gorm:"column:lease_expires_at"`
	ProcessingDeadlineAt time.Time       `gorm:"column:processing_deadline_at;not null"`
	ValidationAttempts   uint32          `gorm:"column:validation_attempts;not null"`
	PromoteAttempts      uint32          `gorm:"column:promote_attempts;not null"`
	LastErrorCode        string          `gorm:"column:last_error_code;size:64;not null"`
	CompletedAt          *time.Time      `gorm:"column:completed_at"`
	RowVersion           uint64          `gorm:"column:row_version;not null"`
	CreatedAt            time.Time       `gorm:"column:created_at;not null"`
	UpdatedAt            time.Time       `gorm:"column:updated_at;not null"`
}

func (UploadCompletion) TableName() string { return "media_upload_completions" }

type CompletionWork struct {
	Bundle        UploadBundle
	Completion    UploadCompletion
	LeaseToken    string
	ShouldProcess bool
}

type CompletionObject struct {
	Key          string
	VersionID    string
	ETag         string
	ContentType  string
	Size         uint64
	CRC64        string
	LastModified time.Time
	Metadata     map[string]string
}

type PromoteObjectRequest struct {
	SourceKey       string
	SourceVersionID string
	SourceETag      string
	DestinationKey  string
	AssetPublicID   string
	TicketID        string
	TrustedSHA256   string
	VerifiedMIME    string
	VerifiedSize    uint64
}

type CompletionObjectStore interface {
	HeadObject(context.Context, string, string) (CompletionObject, error)
	OpenObject(context.Context, string, string) (io.ReadCloser, error)
	PromoteObject(context.Context, PromoteObjectRequest) (CompletionObject, error)
}

type CompletionValidation struct {
	StagingVersionID    string
	StagingETag         string
	StagingLastModified time.Time
	TrustedSHA256       string
	VerifiedMIME        string
	VerifiedSize        uint64
	Width               uint32
	Height              uint32
	FinalKey            string
}

type TencentCompletionStoreConfig struct {
	SecretID  string
	SecretKey string
	Bucket    string
	Region    string
}

type TencentCompletionStore struct {
	bucket, region      string
	secretID, secretKey string
	cosClient           *cos.Client
}

func NewTencentCompletionStore(config TencentCompletionStoreConfig) (*TencentCompletionStore, error) {
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
	client := cos.NewClient(&cos.BaseURL{BucketURL: bucketURL}, &http.Client{
		Timeout:   30 * time.Second,
		Transport: &cos.AuthorizationTransport{SecretID: config.SecretID, SecretKey: config.SecretKey},
	})
	return &TencentCompletionStore{
		bucket: config.Bucket, region: config.Region,
		secretID: config.SecretID, secretKey: config.SecretKey,
		cosClient: client,
	}, nil
}

type signedGetObjectOptions struct {
	VersionID string `url:"versionId,omitempty" header:"-"`
}

func (s *TencentCompletionStore) SignedGetURL(ctx context.Context, key, versionID string, ttl time.Duration) (string, error) {
	key = strings.TrimSpace(key)
	versionID = strings.TrimSpace(versionID)
	if key == "" || versionID == "" || ttl <= 0 {
		return "", ErrCompletionInvariant
	}
	value, err := s.cosClient.Object.GetPresignedURL(ctx, http.MethodGet, key, s.secretID, s.secretKey, ttl, &signedGetObjectOptions{VersionID: versionID})
	if err != nil {
		return "", err
	}
	return value.String(), nil
}

func (s *TencentCompletionStore) HeadObject(ctx context.Context, key, versionID string) (CompletionObject, error) {
	var response *cos.Response
	var err error
	if versionID == "" {
		response, err = s.cosClient.Object.Head(ctx, key, nil)
	} else {
		response, err = s.cosClient.Object.Head(ctx, key, nil, versionID)
	}
	if err != nil {
		return CompletionObject{}, normalizeCompletionCOSError(err)
	}
	if response == nil {
		return CompletionObject{}, ErrCompletionInvariant
	}
	if response.Body != nil {
		defer response.Body.Close()
	}
	size, err := strconv.ParseUint(response.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return CompletionObject{}, fmt.Errorf("invalid COS Content-Length: %w", err)
	}
	lastModified, err := http.ParseTime(response.Header.Get("Last-Modified"))
	if err != nil {
		return CompletionObject{}, fmt.Errorf("invalid COS Last-Modified: %w", err)
	}
	object := CompletionObject{
		Key: key, VersionID: response.Header.Get("x-cos-version-id"),
		ETag: normalizeETag(response.Header.Get("ETag")), ContentType: response.Header.Get("Content-Type"),
		Size: size, CRC64: response.Header.Get("x-cos-hash-crc64ecma"), LastModified: lastModified.UTC(),
		Metadata: map[string]string{},
	}
	for name, values := range response.Header {
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "x-cos-meta-") && len(values) > 0 {
			object.Metadata[strings.TrimPrefix(lower, "x-cos-meta-")] = values[0]
		}
	}
	if object.VersionID == "" {
		return CompletionObject{}, errors.New("COS versioning is required for media completion")
	}
	return object, nil
}

func (s *TencentCompletionStore) OpenObject(ctx context.Context, key, versionID string) (io.ReadCloser, error) {
	if versionID == "" {
		return nil, ErrCompletionInvariant
	}
	response, err := s.cosClient.Object.Get(ctx, key, nil, versionID)
	if err != nil {
		return nil, normalizeCompletionCOSError(err)
	}
	return response.Body, nil
}

func (s *TencentCompletionStore) PromoteObject(ctx context.Context, request PromoteObjectRequest) (CompletionObject, error) {
	if request.SourceVersionID == "" || request.SourceETag == "" || request.DestinationKey == "" {
		return CompletionObject{}, ErrCompletionInvariant
	}
	headers := make(http.Header)
	headers.Set("x-cos-forbid-overwrite", "true")
	headers.Set("x-cos-meta-media-asset-id", request.AssetPublicID)
	headers.Set("x-cos-meta-media-ticket-id", request.TicketID)
	headers.Set("x-cos-meta-media-sha256", request.TrustedSHA256)
	headers.Set("x-cos-meta-media-source-version", request.SourceVersionID)
	headers.Set("x-cos-meta-media-verified-size", strconv.FormatUint(request.VerifiedSize, 10))
	options := &cos.ObjectCopyOptions{ObjectCopyHeaderOptions: &cos.ObjectCopyHeaderOptions{
		ContentType: request.VerifiedMIME, XCosMetadataDirective: "Replaced",
		XCosCopySourceIfMatch: quoteETag(request.SourceETag), XOptionHeader: &headers,
	}}
	source := fmt.Sprintf("%s.cos.%s.myqcloud.com/%s", s.bucket, s.region, strings.TrimLeft(request.SourceKey, "/"))
	result, response, err := s.cosClient.Object.Copy(ctx, request.DestinationKey, source, options, request.SourceVersionID)
	if err != nil {
		return CompletionObject{}, normalizeCompletionCOSError(err)
	}
	versionID := ""
	if result != nil {
		versionID = result.VersionId
	}
	if versionID == "" && response != nil {
		versionID = response.Header.Get("x-cos-version-id")
	}
	if versionID == "" {
		return CompletionObject{}, errors.New("COS CopyObject did not return a final version")
	}
	return s.HeadObject(ctx, request.DestinationKey, versionID)
}

func normalizeCompletionCOSError(err error) error {
	if err == nil {
		return nil
	}
	if cos.IsNotFoundError(err) {
		return errors.Join(ErrObjectNotFound, err)
	}
	var responseError *cos.ErrorResponse
	if errors.As(err, &responseError) && responseError.Response != nil {
		if responseError.Response.StatusCode == http.StatusConflict || responseError.Response.StatusCode == http.StatusPreconditionFailed ||
			responseError.Code == "ObjectAlreadyExists" || responseError.Code == "FileAlreadyExists" {
			return errors.Join(ErrObjectAlreadyExists, err)
		}
	}
	return err
}

func normalizeETag(value string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(value), `"`))
}
func quoteETag(value string) string { return `"` + normalizeETag(value) + `"` }

var _ CompletionObjectStore = (*TencentCompletionStore)(nil)
