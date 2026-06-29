package delivery

import (
	"context"
	"strings"
	"time"
)

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository, now func() time.Time) (*Service, error) {
	if repository == nil {
		return nil, ErrInvalidRecord
	}
	if now == nil {
		now = time.Now
	}
	return &Service{repository: repository, now: now}, nil
}

type Contact struct {
	Name    string
	Phone   string
	Address string
}

type CreateOrderInput struct {
	MerchantUserID   uint64
	Pickup           Contact
	Recipient        Contact
	Remark           string
	DeliveryFeeCents uint64
	RiderFeeCents    uint64
	PlatformCutCents uint64
}

func (s *Service) CreateOrder(ctx context.Context, input CreateOrderInput) (Order, error) {
	publicID, err := NewULID()
	if err != nil {
		return Order{}, err
	}
	now := s.now().UTC()
	order := Order{
		DeliveryOrderPublicID: publicID,
		MerchantUserID:        input.MerchantUserID,
		PickupContactName:     strings.TrimSpace(input.Pickup.Name),
		PickupContactPhone:    strings.TrimSpace(input.Pickup.Phone),
		PickupAddress:         strings.TrimSpace(input.Pickup.Address),
		RecipientName:         strings.TrimSpace(input.Recipient.Name),
		RecipientPhone:        strings.TrimSpace(input.Recipient.Phone),
		DeliveryAddress:       strings.TrimSpace(input.Recipient.Address),
		Remark:                strings.TrimSpace(input.Remark),
		DeliveryFeeCents:      input.DeliveryFeeCents,
		RiderFeeCents:         input.RiderFeeCents,
		PlatformCutCents:      input.PlatformCutCents,
		Status:                StatusPendingFeePayment,
		RowVersion:            1,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := order.ValidateNew(); err != nil {
		return Order{}, err
	}
	if err := s.repository.Create(ctx, &order); err != nil {
		return Order{}, err
	}
	return order, nil
}

func (s *Service) MarkFeePaid(ctx context.Context, publicID string, expectedRowVersion uint64) (Order, error) {
	return s.transition(ctx, publicID, expectedRowVersion, StatusPendingFeePayment, StatusReadyForClaim, nil)
}

func (s *Service) MarkPaymentFailed(ctx context.Context, publicID string, expectedRowVersion uint64) (Order, error) {
	return s.transition(ctx, publicID, expectedRowVersion, StatusPendingFeePayment, StatusPaymentFailed, nil)
}

func (s *Service) RetryFeePayment(ctx context.Context, publicID string, expectedRowVersion uint64) (Order, error) {
	return s.transition(ctx, publicID, expectedRowVersion, StatusPaymentFailed, StatusPendingFeePayment, nil)
}

func (s *Service) AssignRider(ctx context.Context, publicID string, expectedRowVersion, riderUserID uint64) (Order, error) {
	if riderUserID == 0 {
		return Order{}, ErrRiderRequired
	}
	return s.transition(ctx, publicID, expectedRowVersion, StatusReadyForClaim, StatusClaimed, &riderUserID)
}

func (s *Service) StartToPickup(ctx context.Context, publicID string, expectedRowVersion, riderUserID uint64) (Order, error) {
	return s.riderTransition(ctx, publicID, expectedRowVersion, riderUserID, StatusClaimed, StatusToPickup)
}

func (s *Service) ReleaseToPool(
	ctx context.Context,
	publicID string,
	expectedStatus Status,
	expectedRowVersion, riderUserID uint64,
) (Order, error) {
	if expectedStatus != StatusClaimed && expectedStatus != StatusToPickup {
		return Order{}, ErrInvalidTransition
	}
	return s.riderTransition(ctx, publicID, expectedRowVersion, riderUserID, expectedStatus, StatusReadyForClaim)
}

func (s *Service) MarkPickedUp(ctx context.Context, publicID string, expectedRowVersion, riderUserID uint64) (Order, error) {
	return s.riderTransition(ctx, publicID, expectedRowVersion, riderUserID, StatusToPickup, StatusPickedUp)
}

func (s *Service) StartDelivering(ctx context.Context, publicID string, expectedRowVersion, riderUserID uint64) (Order, error) {
	return s.riderTransition(ctx, publicID, expectedRowVersion, riderUserID, StatusPickedUp, StatusDelivering)
}

func (s *Service) MarkDelivered(ctx context.Context, publicID string, expectedRowVersion, riderUserID uint64) (Order, error) {
	return s.riderTransition(ctx, publicID, expectedRowVersion, riderUserID, StatusDelivering, StatusDelivered)
}

func (s *Service) Cancel(ctx context.Context, publicID string, expectedStatus Status, expectedRowVersion uint64) (Order, error) {
	if expectedStatus != StatusPendingFeePayment && expectedStatus != StatusPaymentFailed &&
		expectedStatus != StatusReadyForClaim && expectedStatus != StatusClaimed &&
		expectedStatus != StatusToPickup {
		return Order{}, ErrInvalidTransition
	}
	return s.transition(ctx, publicID, expectedRowVersion, expectedStatus, StatusCancelled, nil)
}

func (s *Service) FindOrder(ctx context.Context, publicID string) (Order, error) {
	publicID = strings.TrimSpace(publicID)
	if !ValidULID(publicID) {
		return Order{}, ErrInvalidULID
	}
	return s.repository.Find(ctx, publicID)
}

func (s *Service) riderTransition(
	ctx context.Context,
	publicID string,
	expectedRowVersion, riderUserID uint64,
	from, to Status,
) (Order, error) {
	if riderUserID == 0 {
		return Order{}, ErrRiderRequired
	}
	return s.transition(ctx, publicID, expectedRowVersion, from, to, &riderUserID)
}

func (s *Service) transition(
	ctx context.Context,
	publicID string,
	expectedRowVersion uint64,
	from, to Status,
	riderUserID *uint64,
) (Order, error) {
	publicID = strings.TrimSpace(publicID)
	if !ValidULID(publicID) {
		return Order{}, ErrInvalidULID
	}
	if expectedRowVersion == 0 {
		return Order{}, ErrInvalidRecord
	}
	if !from.Valid() || !to.Valid() || !CanTransition(from, to) {
		return Order{}, ErrInvalidTransition
	}
	return s.repository.Transition(ctx, publicID, expectedRowVersion, from, to, riderUserID, s.now().UTC())
}
