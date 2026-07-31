package trade

import (
	"context"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
)

func TestProductCommentReviewTargetDelegates(t *testing.T) {
	repo := &productCommentMemoryRepo{
		comments: map[uint]ProductComment{
			7: {ID: 7, ProductID: 2, UserID: 3, ReviewStatus: review.StatusSubmitted},
		},
	}
	target := NewProductCommentReviewTarget(repo)
	if target.ObjectType() != ProductCommentAggregateType {
		t.Fatalf("object type=%q", target.ObjectType())
	}
	if err := target.SetReviewStatus(context.Background(), 7, review.StatusApproved); err != nil {
		t.Fatal(err)
	}
	if repo.comments[7].ReviewStatus != review.StatusApproved {
		t.Fatalf("status=%q", repo.comments[7].ReviewStatus)
	}
}
