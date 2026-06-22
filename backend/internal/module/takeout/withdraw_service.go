package takeout

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"
)

type WithdrawService struct {
	repository WithdrawRepository
	now        func() time.Time
}

func NewWithdrawService(repository WithdrawRepository, now func() time.Time) (*WithdrawService, error) {
	if repository == nil {
		return nil, ErrInvalidRecord
	}
	if now == nil {
		now = time.Now
	}
	return &WithdrawService{repository: repository, now: now}, nil
}

func (s *WithdrawService) CreateWithdrawRequest(ctx context.Context, riderUserID, amountCents uint64) (WithdrawRequest, error) {
	publicID, err := NewULID()
	if err != nil {
		return WithdrawRequest{}, err
	}
	now := s.now().UTC()
	value := WithdrawRequest{
		WithdrawPublicID: publicID, RiderUserID: riderUserID, AmountCents: amountCents,
		Status: WithdrawPending, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := value.ValidateNew(); err != nil {
		return WithdrawRequest{}, err
	}
	if err := s.repository.CreateWithdrawRequest(ctx, &value); err != nil {
		return WithdrawRequest{}, err
	}
	return value, nil
}

func (s *WithdrawService) MarkWithdrawPaid(ctx context.Context, publicID string, expectedRowVersion uint64) (WithdrawRequest, error) {
	if !ValidULID(strings.TrimSpace(publicID)) {
		return WithdrawRequest{}, ErrInvalidULID
	}
	if expectedRowVersion == 0 {
		return WithdrawRequest{}, ErrInvalidRecord
	}
	return s.repository.MarkWithdrawPaid(ctx, publicID, expectedRowVersion, s.now().UTC())
}

func (s *WithdrawService) RejectWithdraw(ctx context.Context, publicID string, expectedRowVersion uint64, reason string) (WithdrawRequest, error) {
	reason = strings.TrimSpace(reason)
	if !ValidULID(strings.TrimSpace(publicID)) {
		return WithdrawRequest{}, ErrInvalidULID
	}
	if expectedRowVersion == 0 || reason == "" || utf8.RuneCountInString(reason) > 256 {
		return WithdrawRequest{}, ErrInvalidRecord
	}
	return s.repository.RejectWithdraw(ctx, publicID, expectedRowVersion, reason, s.now().UTC())
}

func (s *WithdrawService) ListWithdrawRequests(ctx context.Context, status WithdrawStatus, page, size int) ([]WithdrawRequest, int64, error) {
	if status != "" && !status.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	page, size = normalizeAdminPage(page, size)
	return s.repository.ListWithdrawRequests(ctx, status, page, size)
}

func (s *WithdrawService) FindWithdrawByID(ctx context.Context, publicID string) (WithdrawRequest, error) {
	if !ValidULID(strings.TrimSpace(publicID)) {
		return WithdrawRequest{}, ErrInvalidULID
	}
	return s.repository.FindWithdrawByID(ctx, publicID)
}
