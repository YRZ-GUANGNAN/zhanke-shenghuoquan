package admin

import (
	"campus-circle/backend/internal/module/admin/review"
	"context"
	"errors"
	"strings"
	"unicode/utf8"
)

var ErrReportIdempotencyConflict = errors.New("report idempotency key payload conflict")

type Service struct {
	repo    Repository
	reviews *review.Service
}

func NewService(repo Repository, reviews *review.Service) *Service {
	return &Service{repo: repo, reviews: reviews}
}
func (s *Service) SubmitReport(ctx context.Context, reporterID uint, targetType string, targetID uint, reason, detail, clientRequestID string) (Report, error) {
	targetType = strings.TrimSpace(targetType)
	reason = strings.TrimSpace(reason)
	detail = strings.TrimSpace(detail)
	clientRequestID = strings.TrimSpace(clientRequestID)
	if reporterID == 0 || targetID == 0 || targetType == "" || reason == "" {
		return Report{}, errors.New("target and reason are required")
	}
	if clientRequestID == "" || len(clientRequestID) > 96 {
		return Report{}, errors.New("valid client request id is required")
	}
	if utf8.RuneCountInString(targetType) > 32 || utf8.RuneCountInString(reason) > 240 {
		return Report{}, errors.New("report fields exceed maximum length")
	}
	expected := Report{ReporterID: reporterID, TargetType: targetType, TargetID: targetID, Reason: reason, Detail: detail, ReviewStatus: review.StatusSubmitted}
	expected.ClientRequestID = &clientRequestID
	value := expected
	err := s.repo.WithinTransaction(ctx, func(txCtx context.Context, txRepo Repository) error {
		created, err := txRepo.CreateReportOrReplay(txCtx, &value)
		if err != nil {
			return err
		}
		if !created && !sameReportPayload(value, expected) {
			return ErrReportIdempotencyConflict
		}
		_, err = s.reviews.EnsureSubmitted(txCtx, review.Submission{
			ObjectType: "report", ObjectID: value.ID, SubmitterID: reporterID, Risk: reason,
		})
		return err
	})
	if err != nil {
		return Report{}, err
	}
	return value, nil
}

func sameReportPayload(left, right Report) bool {
	return left.ReporterID == right.ReporterID && left.TargetType == right.TargetType &&
		left.TargetID == right.TargetID && left.Reason == right.Reason && left.Detail == right.Detail
}
func (s *Service) Reports(ctx context.Context, page, size int) ([]Report, int64, error) {
	page, size = normalize(page, size)
	return s.repo.ListReports(ctx, page, size)
}
func (s *Service) ObjectType() string { return "report" }
func (s *Service) SetReviewStatus(ctx context.Context, id uint, status review.Status) error {
	return s.repo.SetReportStatus(ctx, id, status)
}
func normalize(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
