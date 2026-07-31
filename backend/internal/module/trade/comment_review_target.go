package trade

import (
	"context"

	"campus-circle/backend/internal/module/admin/review"
)

var _ review.Target = ProductCommentReviewTarget{}

type ProductCommentReviewTarget struct {
	repo ProductCommentRepository
}

func NewProductCommentReviewTarget(repo ProductCommentRepository) ProductCommentReviewTarget {
	return ProductCommentReviewTarget{repo: repo}
}

func (ProductCommentReviewTarget) ObjectType() string {
	return ProductCommentAggregateType
}

func (t ProductCommentReviewTarget) SetReviewStatus(ctx context.Context, id uint, status review.Status) error {
	return t.repo.SetProductCommentReviewStatus(ctx, id, status)
}
