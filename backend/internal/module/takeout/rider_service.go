package takeout

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"
)

type RiderService struct {
	repository RiderRepository
	orderOps   RiderOrderOps
	now        func() time.Time
}

func NewRiderService(repository RiderRepository, orderOps RiderOrderOps, now func() time.Time) (*RiderService, error) {
	if repository == nil || orderOps == nil {
		return nil, ErrInvalidRecord
	}
	if now == nil {
		now = time.Now
	}
	return &RiderService{repository: repository, orderOps: orderOps, now: now}, nil
}

type RegisterRiderInput struct {
	RiderUserID uint64
	RealName    string
	Phone       string
}

func (s *RiderService) RegisterRider(ctx context.Context, input RegisterRiderInput) (Rider, error) {
	now := s.now().UTC()
	value := Rider{
		RiderUserID: input.RiderUserID, Status: RiderActive,
		RealName: strings.TrimSpace(input.RealName), Phone: strings.TrimSpace(input.Phone),
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := value.ValidateNew(); err != nil {
		return Rider{}, err
	}
	if err := s.repository.CreateRider(ctx, &value); err != nil {
		return Rider{}, err
	}
	return value, nil
}

func (s *RiderService) SetActive(ctx context.Context, riderUserID, expectedRowVersion uint64) (Rider, error) {
	return s.transitionRider(ctx, riderUserID, expectedRowVersion, RiderResting, RiderActive)
}

func (s *RiderService) SetResting(ctx context.Context, riderUserID, expectedRowVersion uint64) (Rider, error) {
	return s.transitionRider(ctx, riderUserID, expectedRowVersion, RiderActive, RiderResting)
}

func (s *RiderService) Suspend(ctx context.Context, riderUserID, expectedRowVersion uint64, expectedStatus RiderStatus) (Rider, error) {
	if expectedStatus != RiderActive && expectedStatus != RiderResting {
		return Rider{}, ErrInvalidTransition
	}
	return s.transitionRider(ctx, riderUserID, expectedRowVersion, expectedStatus, RiderSuspended)
}

func (s *RiderService) Unsuspend(ctx context.Context, riderUserID, expectedRowVersion uint64) (Rider, error) {
	return s.transitionRider(ctx, riderUserID, expectedRowVersion, RiderSuspended, RiderActive)
}

func (s *RiderService) FindRider(ctx context.Context, riderUserID uint64) (Rider, error) {
	if riderUserID == 0 {
		return Rider{}, ErrInvalidRecord
	}
	return s.repository.FindRider(ctx, riderUserID)
}

func (s *RiderService) GetRiderConfig(ctx context.Context) (RiderConfig, error) {
	return s.repository.GetRiderConfig(ctx)
}

func (s *RiderService) UpdateRiderConfig(ctx context.Context, maxInFlight uint32, expectedRowVersion uint64) (RiderConfig, error) {
	if maxInFlight == 0 || expectedRowVersion == 0 {
		return RiderConfig{}, ErrInvalidRecord
	}
	return s.repository.UpdateRiderConfig(ctx, maxInFlight, expectedRowVersion, s.now().UTC())
}

func (s *RiderService) ListClaimableOrders(ctx context.Context) ([]Order, error) {
	return s.orderOps.ListClaimableOrders(ctx)
}

func (s *RiderService) ClaimOrder(
	ctx context.Context,
	riderUserID uint64,
	orderPublicID string,
	expectedOrderRowVersion uint64,
) (Fulfillment, error) {
	if riderUserID == 0 || !ValidULID(strings.TrimSpace(orderPublicID)) || expectedOrderRowVersion == 0 {
		return Fulfillment{}, ErrInvalidRecord
	}
	fulfillmentPublicID, err := NewULID()
	if err != nil {
		return Fulfillment{}, err
	}
	return s.repository.ClaimOrder(ctx, riderUserID, orderPublicID, expectedOrderRowVersion,
		fulfillmentPublicID, s.orderOps, s.now().UTC())
}

func (s *RiderService) StartToStore(ctx context.Context, fulfillmentPublicID string, expectedRowVersion uint64) (Fulfillment, error) {
	return s.transitionFulfillment(ctx, fulfillmentPublicID, expectedRowVersion, FulfillmentAssigned, FulfillmentToStore)
}

func (s *RiderService) MarkPicked(ctx context.Context, fulfillmentPublicID string, expectedRowVersion uint64) (Fulfillment, error) {
	return s.transitionFulfillment(ctx, fulfillmentPublicID, expectedRowVersion, FulfillmentToStore, FulfillmentPicked)
}

func (s *RiderService) StartToUser(ctx context.Context, fulfillmentPublicID string, expectedRowVersion uint64) (Fulfillment, error) {
	return s.transitionFulfillment(ctx, fulfillmentPublicID, expectedRowVersion, FulfillmentPicked, FulfillmentToUser)
}

func (s *RiderService) CompleteFulfillment(ctx context.Context, fulfillmentPublicID string, expectedRowVersion uint64) (Fulfillment, error) {
	if err := validateFulfillmentMutation(fulfillmentPublicID, expectedRowVersion); err != nil {
		return Fulfillment{}, err
	}
	return s.repository.CompleteFulfillment(ctx, fulfillmentPublicID, expectedRowVersion, s.orderOps, s.now().UTC())
}

func (s *RiderService) FindFulfillmentByOrder(ctx context.Context, orderPublicID string) (Fulfillment, error) {
	if !ValidULID(strings.TrimSpace(orderPublicID)) {
		return Fulfillment{}, ErrInvalidULID
	}
	return s.repository.FindFulfillmentByOrder(ctx, orderPublicID)
}

func (s *RiderService) transitionRider(
	ctx context.Context,
	riderUserID, expectedRowVersion uint64,
	from, to RiderStatus,
) (Rider, error) {
	if riderUserID == 0 || expectedRowVersion == 0 {
		return Rider{}, ErrInvalidRecord
	}
	if !from.Valid() || !to.Valid() || !CanTransitionRider(from, to) {
		return Rider{}, ErrInvalidTransition
	}
	return s.repository.TransitionRiderStatus(ctx, riderUserID, expectedRowVersion, from, to, s.now().UTC())
}

func (s *RiderService) transitionFulfillment(
	ctx context.Context,
	fulfillmentPublicID string,
	expectedRowVersion uint64,
	from, to FulfillmentStatus,
) (Fulfillment, error) {
	if err := validateFulfillmentMutation(fulfillmentPublicID, expectedRowVersion); err != nil {
		return Fulfillment{}, err
	}
	if !from.Valid() || !to.Valid() || !CanTransitionFulfillment(from, to) || to == FulfillmentDelivered {
		return Fulfillment{}, ErrInvalidTransition
	}
	return s.repository.TransitionFulfillmentStatus(ctx, fulfillmentPublicID, expectedRowVersion, from, to, s.now().UTC())
}

func validateFulfillmentMutation(publicID string, expectedRowVersion uint64) error {
	if !ValidULID(strings.TrimSpace(publicID)) {
		return ErrInvalidULID
	}
	if expectedRowVersion == 0 {
		return ErrInvalidRecord
	}
	return nil
}

func validRiderProfile(realName, phone string) bool {
	return utf8.RuneCountInString(strings.TrimSpace(realName)) <= 64 && utf8.RuneCountInString(strings.TrimSpace(phone)) <= 32
}
