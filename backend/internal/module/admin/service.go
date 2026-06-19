package admin

import (
	"campus-circle/backend/internal/module/admin/review"
	"context"
	"errors"
)

type Service struct {
	repo    Repository
	reviews *review.Service
}

func NewService(repo Repository, reviews *review.Service) *Service {
	return &Service{repo: repo, reviews: reviews}
}
func (s *Service) SubmitReport(ctx context.Context, reporterID uint, targetType string, targetID uint, reason, detail string) (Report, error) {
	if reporterID == 0 || targetID == 0 || targetType == "" || reason == "" {
		return Report{}, errors.New("target and reason are required")
	}
	value := Report{ReporterID: reporterID, TargetType: targetType, TargetID: targetID, Reason: reason, Detail: detail, ReviewStatus: review.StatusSubmitted}
	if err := s.repo.CreateReport(ctx, &value); err != nil {
		return Report{}, err
	}
	if _, err := s.reviews.Submit(ctx, review.Submission{ObjectType: "report", ObjectID: value.ID, SubmitterID: reporterID, Risk: reason}); err != nil {
		return Report{}, err
	}
	return value, nil
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
