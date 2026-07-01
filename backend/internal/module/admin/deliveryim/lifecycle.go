package deliveryim

import (
	"context"

	"campus-circle/backend/internal/module/delivery"
)

type Lifecycle struct{ service *delivery.ConversationService }

func NewLifecycle(service *delivery.ConversationService) (*Lifecycle, error) {
	if service == nil {
		return nil, delivery.ErrInvalidRecord
	}
	return &Lifecycle{service: service}, nil
}

func (l *Lifecycle) AfterClaim(ctx context.Context, order delivery.Order) error {
	_, err := l.service.EnsureForClaim(ctx, order)
	return err
}

func (l *Lifecycle) BeforeRelease(ctx context.Context, riderUserID uint64, orderID string) error {
	return l.service.RevokeBeforeRelease(ctx, riderUserID, orderID)
}
