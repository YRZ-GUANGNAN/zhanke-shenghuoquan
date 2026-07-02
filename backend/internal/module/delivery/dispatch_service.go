package delivery

import (
	"context"
	"strings"
	"time"
)

type FeePolicy struct {
	DeliveryFeeCents uint64
	RiderFeeCents    uint64
	PlatformCutCents uint64
}

func (p FeePolicy) Valid() bool {
	total, ok := checkedAddAmount(p.RiderFeeCents, p.PlatformCutCents)
	return ok && p.DeliveryFeeCents > 0 && total == p.DeliveryFeeCents
}

type DispatchService struct {
	repository  DispatchRepository
	orders      *Service
	eligibility RiderEligibility
	feeProvider FeePolicyProvider
	now         func() time.Time
}

type FeePolicyProvider interface {
	FeePolicyForMerchant(context.Context, uint64) (FeePolicy, error)
}

func NewDispatchService(
	repository DispatchRepository,
	orders *Service,
	eligibility RiderEligibility,
	feePolicy FeePolicy,
	now func() time.Time,
) (*DispatchService, error) {
	if !feePolicy.Valid() {
		return nil, ErrInvalidRecord
	}
	return NewDispatchServiceWithFeeProvider(repository, orders, eligibility, staticFeePolicyProvider{policy: feePolicy}, now)
}

func NewDispatchServiceWithFeeProvider(
	repository DispatchRepository,
	orders *Service,
	eligibility RiderEligibility,
	feeProvider FeePolicyProvider,
	now func() time.Time,
) (*DispatchService, error) {
	if repository == nil || orders == nil || eligibility == nil || feeProvider == nil {
		return nil, ErrInvalidRecord
	}
	if now == nil {
		now = time.Now
	}
	return &DispatchService{
		repository: repository, orders: orders, eligibility: eligibility,
		feeProvider: feeProvider, now: now,
	}, nil
}

type staticFeePolicyProvider struct{ policy FeePolicy }

func (p staticFeePolicyProvider) FeePolicyForMerchant(context.Context, uint64) (FeePolicy, error) {
	return p.policy, nil
}

type CreateDispatchInput struct {
	MerchantUserID    uint64
	MerchantRequestID string
	Pickup            Contact
	Recipient         Contact
	Remark            string
}

func (s *DispatchService) CreateOrder(ctx context.Context, input CreateDispatchInput) (Order, error) {
	requestID := strings.TrimSpace(input.MerchantRequestID)
	if input.MerchantUserID == 0 || !ValidULID(requestID) {
		return Order{}, ErrInvalidRecord
	}
	publicID, err := NewULID()
	if err != nil {
		return Order{}, err
	}
	feePolicy, err := s.feeProvider.FeePolicyForMerchant(ctx, input.MerchantUserID)
	if err != nil {
		return Order{}, err
	}
	if !feePolicy.Valid() {
		return Order{}, ErrInvalidRecord
	}
	now := s.now().UTC()
	order := Order{
		DeliveryOrderPublicID: publicID, MerchantUserID: input.MerchantUserID,
		MerchantRequestID:  &requestID,
		PickupContactName:  strings.TrimSpace(input.Pickup.Name),
		PickupContactPhone: strings.TrimSpace(input.Pickup.Phone),
		PickupAddress:      strings.TrimSpace(input.Pickup.Address),
		RecipientName:      strings.TrimSpace(input.Recipient.Name),
		RecipientPhone:     strings.TrimSpace(input.Recipient.Phone),
		DeliveryAddress:    strings.TrimSpace(input.Recipient.Address),
		Remark:             strings.TrimSpace(input.Remark),
		DeliveryFeeCents:   feePolicy.DeliveryFeeCents,
		RiderFeeCents:      feePolicy.RiderFeeCents,
		PlatformCutCents:   feePolicy.PlatformCutCents,
		Status:             StatusPendingFeePayment, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := order.ValidateNew(); err != nil {
		return Order{}, err
	}
	return s.repository.CreateIdempotent(ctx, &order)
}

func (s *DispatchService) MerchantOrder(ctx context.Context, merchantUserID uint64, publicID string) (Order, error) {
	if err := validateActorAndOrder(merchantUserID, publicID); err != nil {
		return Order{}, err
	}
	return s.repository.FindOwnedByMerchant(ctx, strings.TrimSpace(publicID), merchantUserID)
}

func (s *DispatchService) MerchantOrders(ctx context.Context, merchantUserID uint64, page Page) ([]Order, int64, error) {
	if merchantUserID == 0 || !page.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	return s.repository.ListByMerchantUserID(ctx, merchantUserID, page)
}

func (s *DispatchService) CancelMerchantOrder(
	ctx context.Context,
	merchantUserID uint64,
	publicID string,
	expectedStatus Status,
	expectedRowVersion uint64,
) (Order, error) {
	if _, err := s.MerchantOrder(ctx, merchantUserID, publicID); err != nil {
		return Order{}, err
	}
	return s.orders.Cancel(ctx, strings.TrimSpace(publicID), expectedStatus, expectedRowVersion)
}

func (s *DispatchService) ConfirmMockFee(
	ctx context.Context,
	merchantUserID uint64,
	publicID string,
	expectedRowVersion uint64,
) (Order, OutboxEvent, error) {
	if err := validateActorAndOrder(merchantUserID, publicID); err != nil || expectedRowVersion == 0 {
		return Order{}, OutboxEvent{}, ErrInvalidRecord
	}
	return s.repository.TransitionToReadyAndEnqueue(
		ctx, strings.TrimSpace(publicID), &merchantUserID, expectedRowVersion,
		StatusPendingFeePayment, nil, s.now().UTC(),
	)
}

func (s *DispatchService) ClaimableOrders(ctx context.Context, riderUserID uint64, page Page) ([]Order, int64, error) {
	if err := s.requireActive(ctx, riderUserID); err != nil {
		return nil, 0, err
	}
	if !page.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	return s.repository.ListClaimable(ctx, page)
}

func (s *DispatchService) ClaimOrder(
	ctx context.Context,
	riderUserID uint64,
	publicID string,
	expectedRowVersion uint64,
) (Order, error) {
	if err := s.requireActive(ctx, riderUserID); err != nil {
		return Order{}, err
	}
	return s.orders.AssignRider(ctx, strings.TrimSpace(publicID), expectedRowVersion, riderUserID)
}

func (s *DispatchService) RiderOrders(ctx context.Context, riderUserID uint64, page Page) ([]Order, int64, error) {
	if err := s.requireActive(ctx, riderUserID); err != nil {
		return nil, 0, err
	}
	if !page.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	return s.repository.ListByRiderUserID(ctx, riderUserID, page)
}

func (s *DispatchService) RiderOrder(ctx context.Context, riderUserID uint64, publicID string) (Order, error) {
	if err := s.requireActive(ctx, riderUserID); err != nil {
		return Order{}, err
	}
	if err := validateActorAndOrder(riderUserID, publicID); err != nil {
		return Order{}, err
	}
	return s.repository.FindAssignedToRider(ctx, strings.TrimSpace(publicID), riderUserID)
}

func (s *DispatchService) StartToPickup(ctx context.Context, riderUserID uint64, publicID string, expectedRowVersion uint64) (Order, error) {
	return s.riderTransition(ctx, riderUserID, publicID, expectedRowVersion, s.orders.StartToPickup)
}

func (s *DispatchService) MarkPickedUp(ctx context.Context, riderUserID uint64, publicID string, expectedRowVersion uint64) (Order, error) {
	return s.riderTransition(ctx, riderUserID, publicID, expectedRowVersion, s.orders.MarkPickedUp)
}

func (s *DispatchService) StartDelivering(ctx context.Context, riderUserID uint64, publicID string, expectedRowVersion uint64) (Order, error) {
	return s.riderTransition(ctx, riderUserID, publicID, expectedRowVersion, s.orders.StartDelivering)
}

func (s *DispatchService) MarkDelivered(ctx context.Context, riderUserID uint64, publicID string, expectedRowVersion uint64) (Order, error) {
	return s.riderTransition(ctx, riderUserID, publicID, expectedRowVersion, s.orders.MarkDelivered)
}

func (s *DispatchService) ReleaseToPool(
	ctx context.Context,
	riderUserID uint64,
	publicID string,
	expectedStatus Status,
	expectedRowVersion uint64,
) (Order, OutboxEvent, error) {
	if err := s.requireActive(ctx, riderUserID); err != nil {
		return Order{}, OutboxEvent{}, err
	}
	if _, err := s.repository.FindAssignedToRider(ctx, strings.TrimSpace(publicID), riderUserID); err != nil {
		return Order{}, OutboxEvent{}, err
	}
	return s.repository.TransitionToReadyAndEnqueue(
		ctx, strings.TrimSpace(publicID), nil, expectedRowVersion,
		expectedStatus, &riderUserID, s.now().UTC(),
	)
}

func (s *DispatchService) riderTransition(
	ctx context.Context,
	riderUserID uint64,
	publicID string,
	expectedRowVersion uint64,
	transition func(context.Context, string, uint64, uint64) (Order, error),
) (Order, error) {
	if err := s.requireActive(ctx, riderUserID); err != nil {
		return Order{}, err
	}
	if _, err := s.repository.FindAssignedToRider(ctx, strings.TrimSpace(publicID), riderUserID); err != nil {
		return Order{}, err
	}
	return transition(ctx, strings.TrimSpace(publicID), expectedRowVersion, riderUserID)
}

func (s *DispatchService) requireActive(ctx context.Context, riderUserID uint64) error {
	if riderUserID == 0 {
		return ErrRiderNotRegistered
	}
	return s.eligibility.RequireActive(ctx, riderUserID)
}

func validateActorAndOrder(actorUserID uint64, publicID string) error {
	if actorUserID == 0 || !ValidULID(strings.TrimSpace(publicID)) {
		return ErrInvalidRecord
	}
	return nil
}
