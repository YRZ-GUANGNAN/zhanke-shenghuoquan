package media

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"campus-circle/backend/internal/module/admin/review"
	_ "golang.org/x/image/webp"
)

const (
	uploadTTL      = 10 * time.Minute
	reservationTTL = 15 * time.Minute
	garbageTTL     = 24 * time.Hour
)

type CredentialIssuer interface {
	Issue(context.Context, string, time.Duration) (TemporaryCredentials, error)
}

type ObjectStore interface {
	Stat(context.Context, string) (ObjectInfo, error)
	ReadPrefix(context.Context, string, int64) ([]byte, error)
	SignedGetURL(context.Context, string, time.Duration) (string, error)
	Delete(context.Context, string) error
}

type SafetyChecker interface {
	Submit(context.Context, string, string) (string, error)
}

type ReviewFlow interface {
	Submit(context.Context, review.Submission) (review.Task, error)
	Start(context.Context, uint, uint) (review.Task, error)
	Decide(context.Context, uint, uint, review.Status, string) (review.Task, error)
}

type Gateway interface {
	Reserve(context.Context, uint, Purpose, []string) (Reservation, error)
	Commit(context.Context, uint, Reservation, string, uint64) error
	Cancel(context.Context, uint, Reservation) error
	DeleteReference(context.Context, string, uint64, []uint64) error
}

type Service struct {
	repo       Repository
	issuer     CredentialIssuer
	storage    ObjectStore
	safety     SafetyChecker
	reviews    ReviewFlow
	bucket     string
	region     string
	publicBase string
	now        func() time.Time
}

func NewService(repo Repository, issuer CredentialIssuer, storage ObjectStore, safety SafetyChecker, reviews ReviewFlow, bucket, region, publicBase string) *Service {
	return &Service{repo: repo, issuer: issuer, storage: storage, safety: safety, reviews: reviews, bucket: bucket, region: region, publicBase: strings.TrimRight(publicBase, "/"), now: time.Now}
}

func (s *Service) CreateUpload(ctx context.Context, input CreateUploadInput) (UploadTicket, error) {
	maxBytes, extension, err := validateUpload(input)
	if err != nil {
		return UploadTicket{}, err
	}
	publicID, err := randomID()
	if err != nil {
		return UploadTicket{}, err
	}
	now := s.now()
	value := Asset{
		PublicID: publicID, OwnerUserID: input.OwnerUserID, Purpose: input.Purpose,
		ObjectKey:    fmt.Sprintf("media/%s/%d/%s/%s%s", input.Purpose, input.OwnerUserID, now.Format("2006-01"), publicID, extension),
		DeclaredMIME: normalizeMIME(input.MIMEType), DeclaredSize: input.SizeBytes, Status: StatusPendingUpload,
		IdempotencyKey: input.IdempotencyKey, UploadExpiresAt: now.Add(uploadTTL), ReviewAvailableAt: now,
	}
	created, err := s.repo.CreateOrFind(ctx, &value)
	if err != nil {
		return UploadTicket{}, err
	}
	if value.OwnerUserID != input.OwnerUserID || value.Purpose != input.Purpose || value.DeclaredMIME != normalizeMIME(input.MIMEType) || value.DeclaredSize != input.SizeBytes {
		return UploadTicket{}, errors.New("idempotency key was already used for another upload")
	}
	if value.Status != StatusPendingUpload || value.DeleteRequestedAt != nil {
		return UploadTicket{}, ErrInvalidState
	}
	// A failed direct upload may be retried with the same idempotency key. Refresh
	// the database deadline so a newly issued STS credential is actually usable.
	if !created {
		value.UploadExpiresAt = now.Add(uploadTTL)
		if err := s.repo.RefreshUploadExpiry(ctx, value.ID, value.UploadExpiresAt); err != nil {
			return UploadTicket{}, err
		}
	}
	credential, err := s.issuer.Issue(ctx, value.ObjectKey, 15*time.Minute)
	if err != nil {
		return UploadTicket{}, fmt.Errorf("issue COS credentials: %w", err)
	}
	return UploadTicket{AssetID: value.PublicID, Bucket: s.bucket, Region: s.region, Key: value.ObjectKey, MIMEType: value.DeclaredMIME, MaxBytes: maxBytes, Credential: credential}, nil
}

func (s *Service) Complete(ctx context.Context, ownerID uint, openID, publicID string) (Asset, error) {
	value, err := s.repo.FindOwned(ctx, publicID, ownerID)
	if err != nil {
		return Asset{}, err
	}
	if value.Status == StatusApproved || value.Status == StatusRejected || value.Status == StatusReviewing || value.Status == StatusSubmitted {
		return value, nil
	}
	if value.Status != StatusPendingUpload || s.now().After(value.UploadExpiresAt.Add(5*time.Minute)) {
		return Asset{}, ErrInvalidState
	}
	info, err := s.storage.Stat(ctx, value.ObjectKey)
	if err != nil {
		return Asset{}, fmt.Errorf("stat COS object: %w", err)
	}
	prefix, err := s.storage.ReadPrefix(ctx, value.ObjectKey, 1<<20)
	if err != nil {
		return Asset{}, fmt.Errorf("inspect COS object: %w", err)
	}
	detectionPrefix := prefix
	if len(detectionPrefix) > 512 {
		detectionPrefix = detectionPrefix[:512]
	}
	verifiedMIME := normalizeMIME(http.DetectContentType(detectionPrefix))
	imageConfig, format, decodeErr := image.DecodeConfig(bytes.NewReader(prefix))
	decodedMIME := map[string]string{"jpeg": "image/jpeg", "png": "image/png", "webp": "image/webp"}[strings.ToLower(format)]
	maxBytes := maxSize(value.Purpose)
	maxEdge := maxImageEdge(value.Purpose)
	if info.Size <= 0 || info.Size > maxBytes || info.Size != value.DeclaredSize || normalizeMIME(info.ContentType) != value.DeclaredMIME ||
		verifiedMIME != value.DeclaredMIME || decodedMIME != value.DeclaredMIME || !allowedMIME(verifiedMIME) || decodeErr != nil ||
		imageConfig.Width <= 0 || imageConfig.Height <= 0 || imageConfig.Width > maxEdge || imageConfig.Height > maxEdge {
		_ = s.repo.SetReviewStatus(ctx, value.ID, StatusRejected)
		_ = s.repo.RequestDeleteOwned(ctx, value.PublicID, value.OwnerUserID)
		return Asset{}, errors.New("uploaded object does not match declared image")
	}
	if err := s.repo.MarkUploaded(ctx, value.ID, info.Size, verifiedMIME, imageConfig.Width, imageConfig.Height, openID); err != nil && !errors.Is(err, ErrInvalidState) {
		return Asset{}, err
	}
	value, err = s.repo.FindOwned(ctx, publicID, ownerID)
	if err != nil {
		return Asset{}, err
	}
	if err := s.advanceReview(ctx, value, openID); err != nil {
		return s.repo.FindOwned(ctx, publicID, ownerID)
	}
	return s.repo.FindOwned(ctx, publicID, ownerID)
}

func (s *Service) advanceReview(ctx context.Context, value Asset, openID string) error {
	if value.Status == StatusUploaded {
		task, err := s.reviews.Submit(ctx, review.Submission{ObjectType: "media_asset", ObjectID: uint(value.ID), SubmitterID: value.OwnerUserID, Risk: "wechat image safety pending"})
		if err != nil {
			return s.retryReview(ctx, value, err)
		}
		if err := s.repo.SetReviewTask(ctx, value.ID, uint64(task.ID)); err != nil {
			return err
		}
		value.Status, value.ReviewTaskID = StatusSubmitted, uint64(task.ID)
	}
	if value.Status == StatusSubmitted {
		if _, err := s.reviews.Start(ctx, uint(value.ReviewTaskID), 0); err != nil {
			return s.retryReview(ctx, value, err)
		}
		value.Status = StatusReviewing
	}
	if value.Status != StatusReviewing || value.ReviewTraceID != nil {
		return nil
	}
	url, err := s.storage.SignedGetURL(ctx, value.ObjectKey, 10*time.Minute)
	if err != nil {
		return s.retryReview(ctx, value, err)
	}
	trace, err := s.safety.Submit(ctx, openID, url)
	if err != nil {
		return s.retryReview(ctx, value, err)
	}
	if trace == "" {
		return s.retryReview(ctx, value, errors.New("wechat safety returned empty trace id"))
	}
	return s.repo.SetReviewTrace(ctx, value.ID, trace)
}

func (s *Service) retryReview(ctx context.Context, value Asset, cause error) error {
	attempts := value.ReviewAttempts + 1
	delay := time.Duration(1<<min(attempts, 8)) * time.Second
	_ = s.repo.MarkReviewFailure(ctx, value.ID, attempts, cause.Error(), s.now().Add(delay))
	return cause
}

func (s *Service) ApplyWechatResult(ctx context.Context, trace, suggest string, label int) error {
	value, err := s.repo.ApplyReviewResult(ctx, trace, suggest, label)
	if err != nil {
		return err
	}
	if value.Status == StatusApproved || value.Status == StatusRejected {
		return nil
	}
	decision := review.StatusRejected
	if strings.EqualFold(suggest, "pass") {
		decision = review.StatusApproved
	}
	if _, err := s.reviews.Decide(ctx, uint(value.ReviewTaskID), 0, decision, fmt.Sprintf("wechat %s label=%d", suggest, label)); err != nil {
		return err
	}
	if decision == review.StatusRejected {
		return s.repo.RequestDeleteOwned(ctx, value.PublicID, value.OwnerUserID)
	}
	return nil
}

func (s *Service) Asset(ctx context.Context, ownerID uint, publicID string) (Asset, error) {
	return s.repo.FindOwned(ctx, publicID, ownerID)
}

func (s *Service) CancelUpload(ctx context.Context, ownerID uint, publicID string) error {
	return s.repo.RequestDeleteOwned(ctx, publicID, ownerID)
}

func (s *Service) ContentURL(ctx context.Context, publicID string) (string, error) {
	value, err := s.repo.FindPublic(ctx, publicID)
	if err != nil {
		return "", err
	}
	return s.storage.SignedGetURL(ctx, value.ObjectKey, 5*time.Minute)
}

func (s *Service) Reserve(ctx context.Context, ownerID uint, purpose Purpose, publicIDs []string) (Reservation, error) {
	if len(publicIDs) > maxCount(purpose) || hasDuplicates(publicIDs) {
		return Reservation{}, ErrInvalidAsset
	}
	if len(publicIDs) == 0 {
		return Reservation{}, nil
	}
	token, err := randomID()
	if err != nil {
		return Reservation{}, err
	}
	values, err := s.repo.Reserve(ctx, ownerID, purpose, publicIDs, token, s.now().Add(reservationTTL))
	if err != nil {
		return Reservation{}, err
	}
	byID := make(map[string]Asset, len(values))
	for _, value := range values {
		byID[value.PublicID] = value
	}
	refs := make([]Reference, 0, len(publicIDs))
	for _, id := range publicIDs {
		value := byID[id]
		refs = append(refs, Reference{InternalID: value.ID, PublicID: id, URL: s.stableURL(id)})
	}
	return Reservation{Token: token, References: refs}, nil
}

func (s *Service) Commit(ctx context.Context, ownerID uint, reservation Reservation, referenceType string, referenceID uint64) error {
	if ownerID == 0 || reservation.Token == "" || referenceID == 0 || len(reservation.References) == 0 ||
		(referenceType != "post" && referenceType != "product" && referenceType != "user") {
		return ErrInvalidAsset
	}
	ids := make([]string, 0, len(reservation.References))
	for _, ref := range reservation.References {
		ids = append(ids, ref.PublicID)
	}
	return s.repo.Commit(ctx, ownerID, reservation.Token, referenceType, referenceID, ids)
}

func (s *Service) Cancel(ctx context.Context, ownerID uint, reservation Reservation) error {
	if reservation.Token == "" {
		return nil
	}
	return s.repo.CancelReservation(ctx, ownerID, reservation.Token)
}

func (s *Service) DeleteReference(ctx context.Context, referenceType string, referenceID uint64, keep []uint64) error {
	return s.repo.RequestDeleteReference(ctx, referenceType, referenceID, keep)
}

func (s *Service) SetReviewStatus(ctx context.Context, objectID uint, status review.Status) error {
	mapped := Status(status)
	if mapped != StatusReviewing && mapped != StatusApproved && mapped != StatusRejected {
		return ErrInvalidState
	}
	return s.repo.SetReviewStatus(ctx, uint64(objectID), mapped)
}

func (s *Service) ObjectType() string { return "media_asset" }

func (s *Service) ProcessMaintenance(ctx context.Context, limit int) error {
	now := s.now()
	reviews, err := s.repo.PendingReviews(ctx, limit, now)
	if err != nil {
		return err
	}
	for _, value := range reviews {
		_ = s.advanceReview(ctx, value, value.WechatOpenID)
	}
	garbage, err := s.repo.ExpiredGarbage(ctx, limit, now)
	if err != nil {
		return err
	}
	for _, value := range garbage {
		_ = s.repo.RequestDeleteOwned(ctx, value.PublicID, value.OwnerUserID)
	}
	deletes, err := s.repo.PendingDeletes(ctx, limit)
	if err != nil {
		return err
	}
	for _, value := range deletes {
		if err := s.storage.Delete(ctx, value.ObjectKey); err != nil {
			_ = s.repo.MarkReviewFailure(ctx, value.ID, value.ReviewAttempts+1, err.Error(), now.Add(time.Minute))
			continue
		}
		_ = s.repo.MarkStorageDeleted(ctx, value.ID)
	}
	return nil
}

func (s *Service) RunMaintenance(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := s.ProcessMaintenance(ctx, 100); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Service) stableURL(publicID string) string {
	return s.publicBase + "/api/v1/media/assets/" + publicID + "/content"
}

func validateUpload(input CreateUploadInput) (int64, string, error) {
	if input.OwnerUserID == 0 || input.IdempotencyKey == "" || len(input.IdempotencyKey) > 96 {
		return 0, "", ErrInvalidAsset
	}
	if input.Purpose != PurposePostImage && input.Purpose != PurposeProductImage && input.Purpose != PurposeAvatar {
		return 0, "", ErrInvalidAsset
	}
	mimeType := normalizeMIME(input.MIMEType)
	if !allowedMIME(mimeType) || input.SizeBytes <= 0 || input.SizeBytes > maxSize(input.Purpose) {
		return 0, "", ErrInvalidAsset
	}
	ext := strings.ToLower(filepath.Ext(input.FileName))
	expected := map[string][]string{"image/jpeg": {".jpg", ".jpeg"}, "image/png": {".png"}, "image/webp": {".webp"}}[mimeType]
	if !contains(expected, ext) {
		return 0, "", ErrInvalidAsset
	}
	return maxSize(input.Purpose), map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[mimeType], nil
}

func maxSize(purpose Purpose) int64 {
	if purpose == PurposeAvatar {
		return 5 << 20
	}
	return 10 << 20
}

func maxImageEdge(purpose Purpose) int {
	if purpose == PurposeAvatar {
		return 1024
	}
	return 1920
}

func maxCount(purpose Purpose) int {
	if purpose == PurposeAvatar {
		return 1
	}
	return 9
}

func allowedMIME(value string) bool {
	return value == "image/jpeg" || value == "image/png" || value == "image/webp"
}

func normalizeMIME(value string) string {
	value, _, _ = mime.ParseMediaType(strings.ToLower(strings.TrimSpace(value)))
	return value
}

func randomID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func hasDuplicates(values []string) bool {
	seen := map[string]struct{}{}
	for _, value := range values {
		if value == "" {
			return true
		}
		if _, ok := seen[value]; ok {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var _ review.Target = (*Service)(nil)
var _ Gateway = (*Service)(nil)
