package mediaconsole

import (
	"context"
	"regexp"
	"time"
)

var ulidPattern = regexp.MustCompile(`^[0-7][0-9A-HJKMNP-TV-Z]{25}$`)

type QueryService struct{ repository QueryRepository }

func NewQueryService(repository QueryRepository) (*QueryService, error) {
	if repository == nil {
		return nil, ErrInvalidQuery
	}
	return &QueryService{repository: repository}, nil
}

func (s *QueryService) Summary(ctx context.Context) (Summary, error) {
	return s.repository.Summary(ctx)
}

func (s *QueryService) Assets(ctx context.Context, filter ListFilter) (Page[AssetRow], error) {
	filter, err := normalizeFilter(filter)
	if err != nil || !oneOf(filter.Status, "", "pending_upload", "uploaded", "validating", "promoting", "final_ready", "submitted", "reviewing", "approved", "rejected", "review_failed", "delete_pending", "delete_deadletter", "deleted") ||
		!oneOf(filter.ReviewStatus, "", "pending", "submitted", "reviewing", "approved", "rejected", "review_failed") ||
		!oneOf(filter.Purpose, "", "post", "product", "avatar") {
		return Page[AssetRow]{}, ErrInvalidQuery
	}
	items, total, err := s.repository.ListAssets(ctx, filter)
	return Page[AssetRow]{Items: nonNil(items), Page: filter.Page, PageSize: filter.PageSize, Total: total}, err
}

func (s *QueryService) Asset(ctx context.Context, assetID string) (AssetDetail, error) {
	if !ulidPattern.MatchString(assetID) {
		return AssetDetail{}, ErrInvalidQuery
	}
	return s.repository.AssetDetail(ctx, assetID)
}

func (s *QueryService) Decisions(ctx context.Context, filter ListFilter) (Page[DecisionRow], error) {
	filter, err := normalizeFilter(filter)
	if err != nil || !oneOf(filter.Status, "", "approved", "rejected", "review_failed") {
		return Page[DecisionRow]{}, ErrInvalidQuery
	}
	items, total, err := s.repository.ListDecisions(ctx, filter)
	return Page[DecisionRow]{Items: nonNil(items), Page: filter.Page, PageSize: filter.PageSize, Total: total}, err
}

func (s *QueryService) Events(ctx context.Context, filter ListFilter) (Page[EventRow], error) {
	filter, err := normalizeFilter(filter)
	if err != nil || !oneOf(filter.Status, "", "unconsumed", "pending", "processing", "retry_wait", "succeeded", "deadletter") ||
		!oneOf(filter.Operation, "", "ReviewProjectionSubmitted", "ReviewProjectionReviewing", "ReviewProjectionDecision") {
		return Page[EventRow]{}, ErrInvalidQuery
	}
	items, total, err := s.repository.ListEvents(ctx, filter)
	return Page[EventRow]{Items: nonNil(items), Page: filter.Page, PageSize: filter.PageSize, Total: total}, err
}

func (s *QueryService) GCJobs(ctx context.Context, filter ListFilter) (Page[GCJobRow], error) {
	filter, err := normalizeFilter(filter)
	if err != nil || !oneOf(filter.Status, "", "pending", "processing", "retry_wait", "succeeded", "deadletter") ||
		!oneOf(filter.Kind, "", "object", "auxiliary") || !oneOf(filter.Purpose, "", "staging", "final") {
		return Page[GCJobRow]{}, ErrInvalidQuery
	}
	items, total, err := s.repository.ListGCJobs(ctx, filter)
	return Page[GCJobRow]{Items: nonNil(items), Page: filter.Page, PageSize: filter.PageSize, Total: total}, err
}

func (s *QueryService) Deadletters(ctx context.Context, filter ListFilter) (DeadletterView, error) {
	filter.Status = "deadletter"
	consumer, err := s.Events(ctx, filter)
	if err != nil {
		return DeadletterView{}, err
	}
	filter.Operation = ""
	gc, err := s.GCJobs(ctx, filter)
	if err != nil {
		return DeadletterView{}, err
	}
	return DeadletterView{Consumer: consumer, GC: gc}, nil
}

func normalizeFilter(filter ListFilter) (ListFilter, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	if filter.AssetID != "" && !ulidPattern.MatchString(filter.AssetID) {
		return ListFilter{}, ErrInvalidQuery
	}
	if filter.TaskID != nil && *filter.TaskID == 0 {
		return ListFilter{}, ErrInvalidQuery
	}
	if filter.From != nil && filter.To != nil {
		if !filter.From.Before(*filter.To) || filter.To.Sub(*filter.From) > 366*24*time.Hour {
			return ListFilter{}, ErrInvalidQuery
		}
	}
	return filter, nil
}

func oneOf(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func nonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}
