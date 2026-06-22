package reviewtarget

import (
	"context"
	"errors"
	"fmt"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/takeout"
)

const (
	MerchantObjectType = "takeout_merchant"
	DishObjectType     = "takeout_dish"
)

var ErrUnsupportedReviewStatus = errors.New("unsupported takeout review status")

type MerchantService interface {
	FindMerchantByInternalID(context.Context, uint64) (takeout.Merchant, error)
	Approve(context.Context, string, uint64) (takeout.Merchant, error)
	Reject(context.Context, string, uint64) (takeout.Merchant, error)
}

type CatalogService interface {
	FindDishByInternalID(context.Context, uint64) (takeout.Dish, error)
	PutOnShelf(context.Context, string, string, takeout.DishStatus, uint64) (takeout.Dish, error)
}

type MerchantTarget struct{ service MerchantService }
type DishTarget struct{ service CatalogService }

func NewTargets(merchants MerchantService, catalog CatalogService) []review.Target {
	return []review.Target{MerchantTarget{service: merchants}, DishTarget{service: catalog}}
}

func (MerchantTarget) ObjectType() string { return MerchantObjectType }

func (t MerchantTarget) SetReviewStatus(ctx context.Context, objectID uint, status review.Status) error {
	value, err := t.service.FindMerchantByInternalID(ctx, uint64(objectID))
	if err != nil {
		return fmt.Errorf("find takeout merchant review target: %w", err)
	}
	switch status {
	case review.StatusReviewing:
		if value.Status != takeout.MerchantPendingReview {
			return takeout.ErrInvalidTransition
		}
		return nil
	case review.StatusApproved:
		_, err = t.service.Approve(ctx, value.MerchantPublicID, value.RowVersion)
	case review.StatusRejected:
		_, err = t.service.Reject(ctx, value.MerchantPublicID, value.RowVersion)
	default:
		return ErrUnsupportedReviewStatus
	}
	if err != nil {
		return fmt.Errorf("apply takeout merchant review decision: %w", err)
	}
	return nil
}

func (DishTarget) ObjectType() string { return DishObjectType }

func (t DishTarget) SetReviewStatus(ctx context.Context, objectID uint, status review.Status) error {
	value, err := t.service.FindDishByInternalID(ctx, uint64(objectID))
	if err != nil {
		return fmt.Errorf("find takeout dish review target: %w", err)
	}
	switch status {
	case review.StatusReviewing, review.StatusRejected:
		if value.Status != takeout.DishDraft {
			return takeout.ErrInvalidTransition
		}
		return nil
	case review.StatusApproved:
		_, err = t.service.PutOnShelf(ctx, value.DishPublicID, value.MerchantPublicID, takeout.DishDraft, value.RowVersion)
	default:
		return ErrUnsupportedReviewStatus
	}
	if err != nil {
		return fmt.Errorf("apply takeout dish review decision: %w", err)
	}
	return nil
}

var _ review.Target = MerchantTarget{}
var _ review.Target = DishTarget{}
