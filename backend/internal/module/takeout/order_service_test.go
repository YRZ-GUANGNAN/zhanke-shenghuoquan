package takeout

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

type orderRepositoryStub struct {
	createdOrder Order
	createdItems []OrderItem
	config       FeeConfig
	transition   Order
	transitionTo OrderStatus
}

func (s *orderRepositoryStub) CreateOrder(_ context.Context, order *Order, items []OrderItem) error {
	s.createdOrder = *order
	s.createdItems = append([]OrderItem(nil), items...)
	return nil
}
func (s *orderRepositoryStub) FindOrder(context.Context, string) (Order, error) {
	return s.createdOrder, nil
}
func (s *orderRepositoryStub) ListOrderItems(context.Context, string) ([]OrderItem, error) {
	return append([]OrderItem(nil), s.createdItems...), nil
}
func (s *orderRepositoryStub) TransitionOrderStatus(_ context.Context, _ string, _ uint64, _, to OrderStatus, _ *string, _ *uint64, _ time.Time) (Order, error) {
	s.transitionTo = to
	value := s.transition
	value.Status = to
	return value, nil
}
func (s *orderRepositoryStub) CompleteDelivery(context.Context, string, uint64, string, time.Time) (Order, error) {
	value := s.transition
	value.Status = OrderCompleted
	return value, nil
}
func (s *orderRepositoryStub) GetActiveFeeConfig(context.Context) (FeeConfig, error) {
	return s.config, nil
}
func (s *orderRepositoryStub) UpdateFeeConfig(_ context.Context, rider, cut, _ uint64, at time.Time) (FeeConfig, error) {
	s.config.RiderFeeCents, s.config.PlatformCutCents, s.config.UpdatedAt = rider, cut, at
	return s.config, nil
}
func (s *orderRepositoryStub) FindEarningByOrder(context.Context, string) (RiderEarningRecord, error) {
	return RiderEarningRecord{}, ErrNotFound
}
func (s *orderRepositoryStub) MarkSettled(context.Context, string, uint64, time.Time) (RiderEarningRecord, error) {
	return RiderEarningRecord{SettlementStatus: SettlementSettled}, nil
}

type catalogReaderStub struct {
	dishes map[string]Dish
	err    error
}

func (s catalogReaderStub) FindDish(_ context.Context, publicID string) (Dish, error) {
	if s.err != nil {
		return Dish{}, s.err
	}
	value, ok := s.dishes[publicID]
	if !ok {
		return Dish{}, ErrNotFound
	}
	return value, nil
}

func TestOrderServiceCreatesServerCalculatedSnapshots(t *testing.T) {
	now := time.Date(2026, 6, 22, 13, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	merchant := "01J00000000000000000000111"
	dishOne := "01J00000000000000000000112"
	dishTwo := "01J00000000000000000000113"
	repository := &orderRepositoryStub{config: FeeConfig{
		ConfigScope: "global", RiderFeeCents: 100, PlatformCutCents: 50, RowVersion: 1, UpdatedAt: now,
	}}
	reader := catalogReaderStub{dishes: map[string]Dish{
		dishOne: {DishPublicID: dishOne, MerchantPublicID: merchant, Name: "Rice", PriceCents: 1200, Status: DishOnShelf},
		dishTwo: {DishPublicID: dishTwo, MerchantPublicID: merchant, Name: "Soup", PriceCents: 500, Status: DishOnShelf},
	}}
	service, err := NewOrderService(repository, reader, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	order, err := service.CreateOrder(context.Background(), CreateOrderInput{
		BuyerUserID: 9, MerchantPublicID: merchant,
		Items:   []OrderItemRequest{{DishPublicID: dishOne, Quantity: 2}, {DishPublicID: dishTwo, Quantity: 3}},
		Address: OrderAddress{ContactName: "Buyer", ContactPhone: "10086", AddressBuilding: "A", AddressRoom: "101"},
		Remark:  "less salt",
	})
	if err != nil {
		t.Fatal(err)
	}
	if order.ItemsAmountCents != 3900 || order.DeliveryFeeCents != 150 || order.TotalAmountCents != 4050 ||
		order.Status != OrderPendingPayment || order.RowVersion != 1 || len(repository.createdItems) != 2 {
		t.Fatalf("unexpected server calculation order=%#v items=%#v", order, repository.createdItems)
	}
	if repository.createdItems[0].DishNameSnapshot != "Rice" || repository.createdItems[0].UnitPriceCentsSnapshot != 1200 ||
		repository.createdItems[0].LineAmountCents != 2400 || !order.CreatedAt.Equal(now.UTC()) {
		t.Fatalf("snapshot/UTC mismatch order=%#v item=%#v", order, repository.createdItems[0])
	}
}

func TestOrderServiceRejectsUnavailableDuplicateAndOverflow(t *testing.T) {
	now := time.Now().UTC()
	merchant := "01J00000000000000000000121"
	dishID := "01J00000000000000000000122"
	repository := &orderRepositoryStub{config: FeeConfig{ConfigScope: "global", RowVersion: 1, UpdatedAt: now}}
	base := Dish{DishPublicID: dishID, MerchantPublicID: merchant, Name: "Dish", PriceCents: 100, Status: DishOnShelf}
	reader := catalogReaderStub{dishes: map[string]Dish{dishID: base}}
	service, _ := NewOrderService(repository, reader, func() time.Time { return now })
	ctx := context.Background()
	input := CreateOrderInput{BuyerUserID: 1, MerchantPublicID: merchant, Items: []OrderItemRequest{{DishPublicID: dishID, Quantity: 1}}}

	reader.dishes[dishID] = Dish{DishPublicID: dishID, MerchantPublicID: merchant, Name: "Dish", PriceCents: 100, Status: DishDraft}
	service.catalogReader = reader
	if _, err := service.CreateOrder(ctx, input); !errors.Is(err, ErrDishUnavailable) {
		t.Fatalf("draft dish returned %v", err)
	}
	reader.dishes[dishID] = base
	service.catalogReader = reader
	duplicate := input
	duplicate.Items = append(duplicate.Items, duplicate.Items[0])
	if _, err := service.CreateOrder(ctx, duplicate); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("duplicate dish returned %v", err)
	}
	overflowDish := base
	overflowDish.PriceCents = math.MaxUint64
	reader.dishes[dishID] = overflowDish
	service.catalogReader = reader
	input.Items[0].Quantity = 2
	if _, err := service.CreateOrder(ctx, input); !errors.Is(err, ErrAmountOverflow) {
		t.Fatalf("overflow returned %v", err)
	}
	service.catalogReader = catalogReaderStub{err: ErrNotFound}
	input.Items[0].Quantity = 1
	if _, err := service.CreateOrder(ctx, input); !errors.Is(err, ErrDishUnavailable) {
		t.Fatalf("missing dish returned %v", err)
	}
}

func TestOrderServiceValidatesTransitionInputs(t *testing.T) {
	now := time.Now().UTC()
	repository := &orderRepositoryStub{
		config:     FeeConfig{ConfigScope: "global", RowVersion: 1, UpdatedAt: now},
		transition: Order{OrderPublicID: "01J00000000000000000000131", RowVersion: 1},
	}
	service, _ := NewOrderService(repository, catalogReaderStub{dishes: map[string]Dish{}}, func() time.Time { return now })
	ctx := context.Background()
	if _, err := service.MarkPaid(ctx, repository.transition.OrderPublicID, 1, ""); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("empty pay no returned %v", err)
	}
	if _, err := service.Refund(ctx, repository.transition.OrderPublicID, OrderPreparing, 1); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("illegal refund source returned %v", err)
	}
	if _, err := service.StartDelivering(ctx, repository.transition.OrderPublicID, 1, 0); !errors.Is(err, ErrRiderRequired) {
		t.Fatalf("zero rider returned %v", err)
	}
	if _, err := NewOrderService(nil, catalogReaderStub{}, nil); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("nil repository returned %v", err)
	}
}
