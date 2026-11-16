package studentverification

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

const (
	ulidAlphabet             = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	mediaCancellationTimeout = 5 * time.Second
)

var (
	ErrInvalidInput        = errors.New("invalid student verification input")
	ErrApplicationNotFound = errors.New("student verification not found")
	ErrApplicationPending  = errors.New("student verification is pending")
	ErrApplicationApproved = errors.New("student verification is approved")
	ErrVersionConflict     = errors.New("student verification version conflict")
	ErrDecisionConflict    = errors.New("student verification decision conflict")
	ErrSelfReview          = errors.New("student verification self review is forbidden")
	ErrCancelNotAllowed    = errors.New("student verification cancel request is not allowed")
)

type Service struct {
	repo  Repository
	media ApplicationMedia
	now   func() time.Time
}

func NewService(repo Repository, applicationMedia ApplicationMedia, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, media: applicationMedia, now: now}
}

func (s *Service) ListAdmin(ctx context.Context, filter AdminListFilter) (AdminListView, error) {
	if s == nil || s.repo == nil {
		return AdminListView{}, errors.New("student verification repository is required")
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return AdminListView{}, invalidInput("invalid verification status")
	}
	if filter.Page < 1 || filter.PageSize < 1 || filter.PageSize > 100 {
		return AdminListView{}, invalidInput("invalid pagination")
	}
	values, total, err := s.repo.ListAdmin(ctx, filter)
	if err != nil {
		return AdminListView{}, err
	}
	items := make([]VerificationView, len(values))
	for index := range values {
		items[index] = toVerificationView(values[index])
	}
	return AdminListView{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func (s *Service) GetAdmin(ctx context.Context, applicationPublicID string) (VerificationDetail, error) {
	applicationPublicID = strings.TrimSpace(applicationPublicID)
	if !validULID(applicationPublicID) {
		return VerificationDetail{}, invalidInput("invalid verification id")
	}
	if s == nil || s.repo == nil {
		return VerificationDetail{}, errors.New("student verification repository is required")
	}
	detail, err := s.repo.FindDetail(ctx, applicationPublicID)
	if err != nil {
		return VerificationDetail{}, err
	}
	if s.media == nil {
		return VerificationDetail{}, errors.New("student verification media is required")
	}
	proofs, err := s.media.ResolveAdminProofs(ctx, applicationPublicID, detail.AssetPublicIDs)
	if err != nil {
		return VerificationDetail{}, err
	}
	detail.Proofs = proofs
	return detail, nil
}

func (s *Service) GetMine(ctx context.Context, userID uint64) (VerificationView, error) {
	if userID == 0 {
		return VerificationView{}, invalidInput("user id is required")
	}
	if s == nil || s.repo == nil {
		return VerificationView{}, errors.New("student verification repository is required")
	}
	value, found, err := s.repo.FindCurrent(ctx, userID)
	if err != nil {
		return VerificationView{}, err
	}
	if !found {
		return VerificationView{}, ErrApplicationNotFound
	}
	return toVerificationView(value), nil
}

func (s *Service) Submit(ctx context.Context, userID uint64, input SubmitInput) (VerificationView, error) {
	if userID == 0 {
		return VerificationView{}, invalidInput("user id is required")
	}
	if s == nil || s.repo == nil {
		return VerificationView{}, errors.New("student verification repository is required")
	}

	current, found, err := s.repo.FindCurrent(ctx, userID)
	if err != nil {
		return VerificationView{}, err
	}
	if found {
		switch current.Status {
		case StatusPending:
			return VerificationView{}, ErrApplicationPending
		case StatusApproved:
			return VerificationView{}, ErrApplicationApproved
		case StatusRejected, StatusCancelled:
			// Rejected or cancelled records may be resubmitted.
		default:
			return VerificationView{}, fmt.Errorf("invalid stored student verification status %q", current.Status)
		}
	}

	normalized, err := normalizeSubmitInput(userID, input)
	if err != nil {
		return VerificationView{}, err
	}
	now := s.now().UTC()
	var value Application
	eventType := EventSubmitted
	expectedRowVersion := uint64(0)

	if !found {
		if normalized.ExpectedRowVersion != 0 {
			return VerificationView{}, ErrVersionConflict
		}
		publicID, err := newULIDAt(now, rand.Reader)
		if err != nil {
			return VerificationView{}, fmt.Errorf("generate student verification id: %w", err)
		}
		value = Application{
			ApplicationPublicID: publicID,
			UserID:              userID,
			Status:              StatusPending,
			RealName:            normalized.RealName,
			StudentNo:           normalized.StudentNo,
			College:             normalized.College,
			SubmittedAt:         now,
			RowVersion:          1,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
	} else {
		if normalized.ExpectedRowVersion == 0 || normalized.ExpectedRowVersion != current.RowVersion {
			return VerificationView{}, ErrVersionConflict
		}
		current.Status = StatusPending
		current.RealName = normalized.RealName
		current.StudentNo = normalized.StudentNo
		current.College = normalized.College
		current.ReviewReason = ""
		current.ReviewerUserID = nil
		current.ReviewedAt = nil
		current.CancelRequestedAt = nil
		current.SubmittedAt = now
		current.RowVersion++
		current.UpdatedAt = now
		value = current
		eventType = EventResubmitted
		expectedRowVersion = normalized.ExpectedRowVersion
	}

	if s.media == nil {
		return VerificationView{}, errors.New("student verification media is required")
	}
	var reservation MediaReservation
	if found {
		reservation, err = s.media.ReserveForApplication(ctx, userID, value.ApplicationPublicID, normalized.AssetPublicIDs)
	} else {
		reservation, err = s.media.Reserve(ctx, userID, normalized.AssetPublicIDs)
	}
	if err != nil {
		return VerificationView{}, mapProofSubmissionMediaError(err)
	}
	commit := func(commitContext context.Context, tx *gorm.DB) error {
		return mapProofSubmissionMediaError(s.media.EnsureCommittedInTransaction(
			commitContext, tx, userID, reservation, value.ApplicationPublicID,
		))
	}
	if err := s.repo.SaveSubmission(ctx, &value, normalized.AssetPublicIDs, eventType, expectedRowVersion, commit); err != nil {
		return VerificationView{}, s.cancelMediaAfterFailure(ctx, userID, reservation, err)
	}
	return toVerificationView(value), nil
}

func (s *Service) RequestCancel(ctx context.Context, userID uint64, expectedRowVersion uint64) (VerificationView, error) {
	if userID == 0 {
		return VerificationView{}, invalidInput("user id is required")
	}
	if s == nil || s.repo == nil {
		return VerificationView{}, errors.New("student verification repository is required")
	}
	current, found, err := s.repo.FindCurrent(ctx, userID)
	if err != nil {
		return VerificationView{}, err
	}
	if !found {
		return VerificationView{}, ErrApplicationNotFound
	}
	if current.Status != StatusApproved {
		return VerificationView{}, ErrCancelNotAllowed
	}
	if current.CancelRequestedAt != nil {
		// Idempotent: a repeated cancel request keeps the first submission.
		return toVerificationView(current), nil
	}
	if expectedRowVersion == 0 || expectedRowVersion != current.RowVersion {
		return VerificationView{}, ErrVersionConflict
	}
	value, err := s.repo.RequestCancel(ctx, userID, expectedRowVersion, s.now().UTC())
	if err != nil {
		return VerificationView{}, err
	}
	return toVerificationView(value), nil
}

func (s *Service) Approve(ctx context.Context, reviewerUserID uint64, applicationPublicID string, expectedRowVersion uint64) (VerificationView, error) {
	if s == nil || s.repo == nil {
		return VerificationView{}, errors.New("student verification repository is required")
	}
	input, err := normalizeDecisionInput(reviewerUserID, applicationPublicID, expectedRowVersion, "", false, s.now)
	if err != nil {
		return VerificationView{}, err
	}
	value, err := s.repo.Approve(ctx, input)
	if err != nil {
		return VerificationView{}, err
	}
	return toVerificationView(value), nil
}

func (s *Service) Reject(ctx context.Context, reviewerUserID uint64, applicationPublicID string, expectedRowVersion uint64, reason string) (VerificationView, error) {
	if s == nil || s.repo == nil {
		return VerificationView{}, errors.New("student verification repository is required")
	}
	input, err := normalizeDecisionInput(reviewerUserID, applicationPublicID, expectedRowVersion, reason, true, s.now)
	if err != nil {
		return VerificationView{}, err
	}
	value, err := s.repo.Reject(ctx, input)
	if err != nil {
		return VerificationView{}, err
	}
	return toVerificationView(value), nil
}

func (s *Service) ApproveCancel(ctx context.Context, reviewerUserID uint64, applicationPublicID string, expectedRowVersion uint64, reason string) (VerificationView, error) {
	if s == nil || s.repo == nil {
		return VerificationView{}, errors.New("student verification repository is required")
	}
	input, err := normalizeDecisionInput(reviewerUserID, applicationPublicID, expectedRowVersion, reason, false, s.now)
	if err != nil {
		return VerificationView{}, err
	}
	value, err := s.repo.ApproveCancel(ctx, input)
	if err != nil {
		return VerificationView{}, err
	}
	return toVerificationView(value), nil
}

func normalizeDecisionInput(
	reviewerUserID uint64,
	applicationPublicID string,
	expectedRowVersion uint64,
	reason string,
	requireReason bool,
	now func() time.Time,
) (DecisionInput, error) {
	applicationPublicID = strings.TrimSpace(applicationPublicID)
	reason = strings.TrimSpace(reason)
	if reviewerUserID == 0 || !validULID(applicationPublicID) || expectedRowVersion == 0 {
		return DecisionInput{}, invalidInput("reviewer, verification id, and expected version are required")
	}
	if requireReason {
		if reason == "" || utf8.RuneCountInString(reason) > 500 {
			return DecisionInput{}, invalidInput("review reason must contain 1-500 characters")
		}
	} else if utf8.RuneCountInString(reason) > 500 {
		return DecisionInput{}, invalidInput("review reason must not exceed 500 characters")
	}
	if now == nil {
		now = time.Now
	}
	return DecisionInput{
		ReviewerUserID: reviewerUserID, ApplicationPublicID: applicationPublicID,
		ExpectedRowVersion: expectedRowVersion, Reason: reason, DecidedAt: now().UTC(),
	}, nil
}

func normalizeSubmitInput(userID uint64, input SubmitInput) (SubmitInput, error) {
	if userID == 0 {
		return SubmitInput{}, invalidInput("user id is required")
	}
	input.RealName = strings.TrimSpace(input.RealName)
	input.StudentNo = strings.TrimSpace(input.StudentNo)
	input.College = strings.TrimSpace(input.College)
	if input.RealName == "" || utf8.RuneCountInString(input.RealName) > 64 {
		return SubmitInput{}, invalidInput("real name must contain 1-64 characters")
	}
	if input.StudentNo == "" || utf8.RuneCountInString(input.StudentNo) > 64 {
		return SubmitInput{}, invalidInput("student number must contain 1-64 characters")
	}
	if utf8.RuneCountInString(input.College) > 120 {
		return SubmitInput{}, invalidInput("college must not exceed 120 characters")
	}

	if len(input.AssetPublicIDs) < 1 || len(input.AssetPublicIDs) > 3 {
		return SubmitInput{}, invalidInput("proof asset count must be between 1 and 3")
	}
	seen := make(map[string]struct{}, len(input.AssetPublicIDs))
	assetIDs := make([]string, len(input.AssetPublicIDs))
	for index, rawID := range input.AssetPublicIDs {
		assetID := strings.TrimSpace(rawID)
		if !validULID(assetID) {
			return SubmitInput{}, invalidInput("proof asset id must be a canonical ULID")
		}
		if _, duplicate := seen[assetID]; duplicate {
			return SubmitInput{}, invalidInput("proof asset ids must be unique")
		}
		seen[assetID] = struct{}{}
		assetIDs[index] = assetID
	}
	input.AssetPublicIDs = assetIDs
	return input, nil
}

func (s *Service) cancelMediaAfterFailure(ctx context.Context, userID uint64, reservation MediaReservation, primary error) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mediaCancellationTimeout)
	defer cancel()
	if cancelErr := s.media.Cancel(cleanupCtx, userID, reservation); cancelErr != nil {
		return errors.Join(primary, fmt.Errorf("cancel student verification media: %w", cancelErr))
	}
	return primary
}

func invalidInput(message string) error { return fmt.Errorf("%w: %s", ErrInvalidInput, message) }

func validULID(value string) bool {
	if len(value) != 26 || value[0] > '7' {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune(ulidAlphabet, character) {
			return false
		}
	}
	return true
}

func newULIDAt(at time.Time, randomness io.Reader) (string, error) {
	if randomness == nil {
		return "", ErrInvalidInput
	}
	milliseconds := at.UnixMilli()
	if milliseconds < 0 || milliseconds > (1<<48)-1 {
		return "", ErrInvalidInput
	}
	raw := make([]byte, 16)
	for index := 5; index >= 0; index-- {
		raw[index] = byte(milliseconds)
		milliseconds >>= 8
	}
	if _, err := io.ReadFull(randomness, raw[6:]); err != nil {
		return "", err
	}
	value := new(big.Int).SetBytes(raw)
	base := big.NewInt(32)
	remainder := new(big.Int)
	encoded := make([]byte, 26)
	for index := len(encoded) - 1; index >= 0; index-- {
		value.QuoRem(value, base, remainder)
		encoded[index] = ulidAlphabet[remainder.Int64()]
	}
	return string(encoded), nil
}
