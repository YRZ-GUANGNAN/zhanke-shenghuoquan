package takeout

import (
	"context"
	"errors"
	"testing"
	"time"
)

type riderRepositoryStub struct {
	rider       Rider
	config      RiderConfig
	fulfillment Fulfillment
	transition  FulfillmentStatus
}

func (s *riderRepositoryStub) CreateRider(_ context.Context, value *Rider) error {
	s.rider = *value
	return nil
}
func (s *riderRepositoryStub) FindRider(context.Context, uint64) (Rider, error) { return s.rider, nil }
func (s *riderRepositoryStub) TransitionRiderStatus(_ context.Context, _ uint64, _ uint64, _, to RiderStatus, at time.Time) (Rider, error) {
	s.rider.Status, s.rider.UpdatedAt = to, at
	s.rider.RowVersion++
	return s.rider, nil
}
func (s *riderRepositoryStub) GetRiderConfig(context.Context) (RiderConfig, error) {
	return s.config, nil
}
func (s *riderRepositoryStub) UpdateRiderConfig(_ context.Context, max uint32, _ uint64, at time.Time) (RiderConfig, error) {
	s.config.MaxInFlightPerRider, s.config.UpdatedAt = max, at
	s.config.RowVersion++
	return s.config, nil
}
func (s *riderRepositoryStub) ClaimOrder(_ context.Context, rider uint64, order string, _ uint64, fulfillmentID string, _ RiderOrderOps, at time.Time) (Fulfillment, error) {
	s.fulfillment = Fulfillment{FulfillmentPublicID: fulfillmentID, OrderPublicID: order, RiderUserID: rider, Status: FulfillmentAssigned, AssignedAt: &at, RowVersion: 1, CreatedAt: at, UpdatedAt: at}
	return s.fulfillment, nil
}
func (s *riderRepositoryStub) TransitionFulfillmentStatus(_ context.Context, _ string, _ uint64, _, to FulfillmentStatus, at time.Time) (Fulfillment, error) {
	s.transition = to
	s.fulfillment.Status = to
	s.fulfillment.RowVersion++
	return s.fulfillment, nil
}
func (s *riderRepositoryStub) CompleteFulfillment(_ context.Context, _ string, _ uint64, _ RiderOrderOps, at time.Time) (Fulfillment, error) {
	s.fulfillment.Status, s.fulfillment.DeliveredAt = FulfillmentDelivered, &at
	s.fulfillment.RowVersion++
	return s.fulfillment, nil
}
func (s *riderRepositoryStub) FindFulfillmentByOrder(context.Context, string) (Fulfillment, error) {
	return s.fulfillment, nil
}

type riderOrderOpsStub struct{ claimable []Order }

func (s *riderOrderOpsStub) StartDelivering(context.Context, RiderOrderUnit, string, uint64, uint64) (Order, error) {
	return Order{Status: OrderDelivering}, nil
}
func (s *riderOrderOpsStub) CompleteDelivery(context.Context, RiderOrderUnit, string) (Order, error) {
	return Order{Status: OrderCompleted}, nil
}
func (s *riderOrderOpsStub) ListClaimableOrders(context.Context) ([]Order, error) {
	return append([]Order(nil), s.claimable...), nil
}

func TestRiderServiceValidationAndDelegation(t *testing.T) {
	now := time.Date(2026, 6, 22, 17, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	repository := &riderRepositoryStub{config: RiderConfig{ConfigScope: "global", MaxInFlightPerRider: 1000, RowVersion: 1, UpdatedAt: now}}
	orderID := "01J00000000000000000000311"
	orderOps := &riderOrderOpsStub{claimable: []Order{{OrderPublicID: orderID, Status: OrderReadyForPickup}}}
	service, err := NewRiderService(repository, orderOps, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	rider, err := service.RegisterRider(context.Background(), RegisterRiderInput{RiderUserID: 9, RealName: " Rider ", Phone: " 10086 "})
	if err != nil || rider.Status != RiderActive || rider.RowVersion != 1 || rider.RealName != "Rider" || !rider.CreatedAt.Equal(now.UTC()) {
		t.Fatalf("register rider=%#v err=%v", rider, err)
	}
	fulfillment, err := service.ClaimOrder(context.Background(), rider.RiderUserID, orderID, 3)
	if err != nil || fulfillment.Status != FulfillmentAssigned || !ValidULID(fulfillment.FulfillmentPublicID) {
		t.Fatalf("claim fulfillment=%#v err=%v", fulfillment, err)
	}
	claimable, err := service.ListClaimableOrders(context.Background())
	if err != nil || len(claimable) != 1 || claimable[0].Status != OrderReadyForPickup {
		t.Fatalf("claimable=%#v err=%v", claimable, err)
	}
	if _, err := service.UpdateRiderConfig(context.Background(), 0, 1); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("zero max returned %v", err)
	}
	if _, err := service.ClaimOrder(context.Background(), 0, orderID, 1); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("zero rider returned %v", err)
	}
	if _, err := NewRiderService(nil, orderOps, nil); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("nil repository returned %v", err)
	}
}
