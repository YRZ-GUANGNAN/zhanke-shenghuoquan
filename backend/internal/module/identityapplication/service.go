package identityapplication

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"regexp"
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
	ErrInvalidInput        = errors.New("invalid identity application input")
	ErrInvalidRole         = errors.New("invalid identity application role")
	ErrApplicationNotFound = errors.New("identity application not found")
	ErrApplicationPending  = errors.New("identity application is pending")
	ErrApplicationApproved = errors.New("identity application is approved")
	ErrVersionConflict     = errors.New("identity application version conflict")
	ErrDecisionConflict    = errors.New("identity application decision conflict")
	ErrSelfReview          = errors.New("identity application self review is forbidden")

	mainlandPhonePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)
)

type Service struct {
	repo        Repository
	media       ApplicationMedia
	now         func() time.Time
	provisioner ApprovalProvisioner
}

type ApprovalProvisioner interface {
	ProvisionApproved(context.Context, *gorm.DB, ApprovedIdentity) error
}

type ApprovalProvisionerFunc func(context.Context, *gorm.DB, ApprovedIdentity) error

func (f ApprovalProvisionerFunc) ProvisionApproved(ctx context.Context, tx *gorm.DB, value ApprovedIdentity) error {
	return f(ctx, tx, value)
}

func NewService(repo Repository, applicationMedia ApplicationMedia, now func() time.Time, provisioners ...ApprovalProvisioner) *Service {
	if now == nil {
		now = time.Now
	}
	var provisioner ApprovalProvisioner
	if len(provisioners) > 0 {
		provisioner = provisioners[0]
	}
	return &Service{repo: repo, media: applicationMedia, now: now, provisioner: provisioner}
}

func (s *Service) ListAdmin(ctx context.Context, filter AdminListFilter) (AdminListView, error) {
	if s == nil || s.repo == nil {
		return AdminListView{}, errors.New("identity application repository is required")
	}
	if filter.Role != "" && !filter.Role.Valid() {
		return AdminListView{}, errors.Join(ErrInvalidInput, ErrInvalidRole)
	}
	if filter.Status != "" && !filter.Status.Valid() {
		return AdminListView{}, invalidInput("invalid application status")
	}
	if filter.Page < 1 || filter.PageSize < 1 || filter.PageSize > 100 {
		return AdminListView{}, invalidInput("invalid pagination")
	}
	values, total, err := s.repo.ListAdmin(ctx, filter)
	if err != nil {
		return AdminListView{}, err
	}
	items := make([]ApplicationView, len(values))
	for index := range values {
		items[index] = toApplicationView(values[index])
	}
	return AdminListView{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func (s *Service) GetAdmin(ctx context.Context, applicationPublicID string) (ApplicationDetail, error) {
	applicationPublicID = strings.TrimSpace(applicationPublicID)
	if !validULID(applicationPublicID) {
		return ApplicationDetail{}, invalidInput("invalid application id")
	}
	if s == nil || s.repo == nil {
		return ApplicationDetail{}, errors.New("identity application repository is required")
	}
	detail, err := s.repo.FindDetail(ctx, applicationPublicID)
	if err != nil {
		return ApplicationDetail{}, err
	}
	if s.media == nil {
		return ApplicationDetail{}, errors.New("identity application media is required")
	}
	proofs, err := s.media.ResolveAdminProofs(ctx, applicationPublicID, detail.AssetPublicIDs)
	if err != nil {
		return ApplicationDetail{}, err
	}
	detail.Proofs = proofs
	return detail, nil
}

func (s *Service) Approve(ctx context.Context, reviewerUserID uint64, applicationPublicID string, expectedRowVersion uint64) (ApplicationView, error) {
	if s == nil || s.repo == nil {
		return ApplicationView{}, errors.New("identity application repository is required")
	}
	input, err := normalizeDecisionInput(reviewerUserID, applicationPublicID, expectedRowVersion, "", false, s.now)
	if err != nil {
		return ApplicationView{}, err
	}
	commit := func(commitContext context.Context, tx *gorm.DB, approved ApprovedIdentity) error {
		if approved.Role != RoleRider {
			return nil
		}
		if s.provisioner == nil {
			return errors.New("approved rider provisioner is required")
		}
		return s.provisioner.ProvisionApproved(commitContext, tx, approved)
	}
	value, err := s.repo.Approve(ctx, input, commit)
	if err != nil {
		return ApplicationView{}, err
	}
	return toApplicationView(value), nil
}

func (s *Service) Reject(ctx context.Context, reviewerUserID uint64, applicationPublicID string, expectedRowVersion uint64, reason string) (ApplicationView, error) {
	if s == nil || s.repo == nil {
		return ApplicationView{}, errors.New("identity application repository is required")
	}
	input, err := normalizeDecisionInput(reviewerUserID, applicationPublicID, expectedRowVersion, reason, true, s.now)
	if err != nil {
		return ApplicationView{}, err
	}
	value, err := s.repo.Reject(ctx, input)
	if err != nil {
		return ApplicationView{}, err
	}
	return toApplicationView(value), nil
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
		return DecisionInput{}, invalidInput("reviewer, application id, and expected version are required")
	}
	if requireReason {
		if reason == "" || utf8.RuneCountInString(reason) > 500 {
			return DecisionInput{}, invalidInput("review reason must contain 1-500 characters")
		}
	} else if reason != "" {
		return DecisionInput{}, invalidInput("approval reason must be empty")
	}
	if now == nil {
		now = time.Now
	}
	return DecisionInput{
		ReviewerUserID: reviewerUserID, ApplicationPublicID: applicationPublicID,
		ExpectedRowVersion: expectedRowVersion, Reason: reason, DecidedAt: now().UTC(),
	}, nil
}

func (s *Service) ListMine(ctx context.Context, userID uint64) ([]ApplicationView, error) {
	if userID == 0 {
		return nil, invalidInput("user id is required")
	}
	if s == nil || s.repo == nil {
		return nil, errors.New("identity application repository is required")
	}
	values, err := s.repo.ListCurrent(ctx, userID)
	if err != nil {
		return nil, err
	}
	views := make([]ApplicationView, len(values))
	for index := range values {
		views[index] = toApplicationView(values[index])
	}
	return views, nil
}

func (s *Service) GetMine(ctx context.Context, userID uint64, role Role) (ApplicationView, error) {
	if userID == 0 {
		return ApplicationView{}, invalidInput("user id is required")
	}
	if !role.Valid() {
		return ApplicationView{}, errors.Join(ErrInvalidInput, ErrInvalidRole)
	}
	if s == nil || s.repo == nil {
		return ApplicationView{}, errors.New("identity application repository is required")
	}
	value, found, err := s.repo.FindCurrent(ctx, userID, role)
	if err != nil {
		return ApplicationView{}, err
	}
	if !found {
		return ApplicationView{}, ErrApplicationNotFound
	}
	return toApplicationView(value), nil
}

func (s *Service) Submit(ctx context.Context, userID uint64, role Role, input SubmitInput) (ApplicationView, error) {
	if userID == 0 {
		return ApplicationView{}, invalidInput("user id is required")
	}
	if !role.Valid() {
		return ApplicationView{}, errors.Join(ErrInvalidInput, ErrInvalidRole)
	}
	if s == nil || s.repo == nil {
		return ApplicationView{}, errors.New("identity application repository is required")
	}

	current, found, err := s.repo.FindCurrent(ctx, userID, role)
	if err != nil {
		return ApplicationView{}, err
	}
	if found {
		switch current.Status {
		case StatusPending:
			return ApplicationView{}, ErrApplicationPending
		case StatusApproved:
			return ApplicationView{}, ErrApplicationApproved
		case StatusRejected:
			// A rejected application is the only state that may be resubmitted.
		default:
			return ApplicationView{}, fmt.Errorf("invalid stored identity application status %q", current.Status)
		}
	}

	normalized, err := normalizeSubmitInput(userID, role, input)
	if err != nil {
		return ApplicationView{}, err
	}
	now := s.now().UTC()
	var value Application
	eventType := EventSubmitted
	expectedRowVersion := uint64(0)

	if !found {
		if normalized.ExpectedRowVersion != 0 {
			return ApplicationView{}, ErrVersionConflict
		}
		publicID, err := newULIDAt(now, rand.Reader)
		if err != nil {
			return ApplicationView{}, fmt.Errorf("generate identity application id: %w", err)
		}
		value = Application{
			ApplicationPublicID: publicID,
			UserID:              userID,
			Role:                role,
			Status:              StatusPending,
			RealName:            normalized.RealName,
			Phone:               normalized.Phone,
			MerchantName:        normalized.MerchantName,
			MerchantAddress:     normalized.MerchantAddress,
			SubmittedAt:         now,
			RowVersion:          1,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
	} else {
		if normalized.ExpectedRowVersion == 0 || normalized.ExpectedRowVersion != current.RowVersion {
			return ApplicationView{}, ErrVersionConflict
		}

		current.Status = StatusPending
		current.RealName = normalized.RealName
		current.Phone = normalized.Phone
		current.MerchantName = normalized.MerchantName
		current.MerchantAddress = normalized.MerchantAddress
		current.ReviewReason = ""
		current.ReviewerUserID = nil
		current.ReviewedAt = nil
		current.SubmittedAt = now
		current.RowVersion++
		current.UpdatedAt = now
		value = current
		eventType = EventResubmitted
		expectedRowVersion = normalized.ExpectedRowVersion
	}

	if s.media == nil {
		return ApplicationView{}, errors.New("identity application media is required")
	}
	var reservation MediaReservation
	if found {
		reservation, err = s.media.ReserveForApplication(ctx, userID, value.ApplicationPublicID, normalized.AssetPublicIDs)
	} else {
		reservation, err = s.media.Reserve(ctx, userID, normalized.AssetPublicIDs)
	}
	if err != nil {
		return ApplicationView{}, mapProofSubmissionMediaError(err)
	}
	commit := func(commitContext context.Context, tx *gorm.DB) error {
		return mapProofSubmissionMediaError(s.media.EnsureCommittedInTransaction(
			commitContext, tx, userID, reservation, value.ApplicationPublicID,
		))
	}
	if err := s.repo.SaveSubmission(ctx, &value, normalized.AssetPublicIDs, eventType, expectedRowVersion, commit); err != nil {
		return ApplicationView{}, s.cancelMediaAfterFailure(ctx, userID, reservation, err)
	}
	return toApplicationView(value), nil
}

func (s *Service) cancelMediaAfterFailure(ctx context.Context, userID uint64, reservation MediaReservation, primary error) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mediaCancellationTimeout)
	defer cancel()
	if cancelErr := s.media.Cancel(cleanupCtx, userID, reservation); cancelErr != nil {
		return errors.Join(primary, fmt.Errorf("cancel identity application media: %w", cancelErr))
	}
	return primary
}

func normalizeSubmitInput(userID uint64, role Role, input SubmitInput) (SubmitInput, error) {
	if userID == 0 {
		return SubmitInput{}, invalidInput("user id is required")
	}
	if !role.Valid() {
		return SubmitInput{}, errors.Join(ErrInvalidInput, ErrInvalidRole)
	}

	input.RealName = strings.TrimSpace(input.RealName)
	input.Phone = strings.TrimSpace(input.Phone)
	input.MerchantName = strings.TrimSpace(input.MerchantName)
	input.MerchantAddress = strings.TrimSpace(input.MerchantAddress)
	if input.RealName == "" || utf8.RuneCountInString(input.RealName) > 64 {
		return SubmitInput{}, invalidInput("real name must contain 1-64 characters")
	}
	if !mainlandPhonePattern.MatchString(input.Phone) {
		return SubmitInput{}, invalidInput("phone must be an 11-digit mainland mobile number")
	}
	if role == RoleMerchant {
		if input.MerchantName == "" || utf8.RuneCountInString(input.MerchantName) > 120 {
			return SubmitInput{}, invalidInput("merchant name must contain 1-120 characters")
		}
		if input.MerchantAddress == "" || utf8.RuneCountInString(input.MerchantAddress) > 240 {
			return SubmitInput{}, invalidInput("merchant address must contain 1-240 characters")
		}
	} else {
		input.MerchantName = ""
		input.MerchantAddress = ""
	}

	if len(input.AssetPublicIDs) < 1 || len(input.AssetPublicIDs) > 6 {
		return SubmitInput{}, invalidInput("proof asset count must be between 1 and 6")
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
