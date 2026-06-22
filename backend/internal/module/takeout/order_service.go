package takeout

import (
	"context"
	"errors"
	"strings"
	"time"
)

type OrderService struct {
	repository    OrderRepository
	catalogReader CatalogReader
	now           func() time.Time
}

func NewOrderService(repository OrderRepository, catalogReader CatalogReader, now func() time.Time) (*OrderService, error) {
	if repository == nil || catalogReader == nil {
		return nil, ErrInvalidRecord
	}
	if now == nil {
		now = time.Now
	}
	return &OrderService{repository: repository, catalogReader: catalogReader, now: now}, nil
}

type OrderAddress struct {
	ContactName     string
	ContactPhone    string
	AddressBuilding string
	AddressRoom     string
}

type OrderItemRequest struct {
	DishPublicID string
	Quantity     uint32
}

type CreateOrderInput struct {
	BuyerUserID      uint64
	MerchantPublicID string
	Items            []OrderItemRequest
	Address          OrderAddress
	Remark           string
}

func (s *OrderService) CreateOrder(ctx context.Context, input CreateOrderInput) (Order, error) {
	merchantPublicID := strings.TrimSpace(input.MerchantPublicID)
	if input.BuyerUserID == 0 || !ValidULID(merchantPublicID) || len(input.Items) == 0 {
		return Order{}, ErrInvalidRecord
	}

	orderPublicID, err := NewULID()
	if err != nil {
		return Order{}, err
	}
	now := s.now().UTC()
	items := make([]OrderItem, 0, len(input.Items))
	seen := make(map[string]struct{}, len(input.Items))
	var itemsAmount uint64
	for _, request := range input.Items {
		dishPublicID := strings.TrimSpace(request.DishPublicID)
		if !ValidULID(dishPublicID) || request.Quantity == 0 {
			return Order{}, ErrInvalidRecord
		}
		if _, duplicate := seen[dishPublicID]; duplicate {
			return Order{}, ErrInvalidRecord
		}
		seen[dishPublicID] = struct{}{}
		dish, findErr := s.catalogReader.FindDish(ctx, dishPublicID)
		if findErr != nil {
			if errors.Is(findErr, ErrNotFound) {
				return Order{}, ErrDishUnavailable
			}
			return Order{}, findErr
		}
		if dish.Status != DishOnShelf || dish.MerchantPublicID != merchantPublicID {
			return Order{}, ErrDishUnavailable
		}
		lineAmount, ok := checkedMultiplyAmount(dish.PriceCents, request.Quantity)
		if !ok {
			return Order{}, ErrAmountOverflow
		}
		itemsAmount, ok = checkedAddAmount(itemsAmount, lineAmount)
		if !ok {
			return Order{}, ErrAmountOverflow
		}
		item := OrderItem{
			OrderPublicID: orderPublicID, DishPublicID: dish.DishPublicID,
			DishNameSnapshot: dish.Name, UnitPriceCentsSnapshot: dish.PriceCents,
			Quantity: request.Quantity, LineAmountCents: lineAmount, CreatedAt: now,
		}
		if validateErr := item.ValidateNew(); validateErr != nil {
			return Order{}, validateErr
		}
		items = append(items, item)
	}

	feeConfig, err := s.repository.GetActiveFeeConfig(ctx)
	if err != nil {
		return Order{}, err
	}
	deliveryFee, ok := checkedAddAmount(feeConfig.RiderFeeCents, feeConfig.PlatformCutCents)
	if !ok {
		return Order{}, ErrAmountOverflow
	}
	totalAmount, ok := checkedAddAmount(itemsAmount, deliveryFee)
	if !ok {
		return Order{}, ErrAmountOverflow
	}
	order := Order{
		OrderPublicID: orderPublicID, BuyerUserID: input.BuyerUserID, MerchantPublicID: merchantPublicID,
		Status: OrderPendingPayment, ItemsAmountCents: itemsAmount, DeliveryFeeCents: deliveryFee,
		TotalAmountCents: totalAmount, ContactName: strings.TrimSpace(input.Address.ContactName),
		ContactPhone: strings.TrimSpace(input.Address.ContactPhone), AddressBuilding: strings.TrimSpace(input.Address.AddressBuilding),
		AddressRoom: strings.TrimSpace(input.Address.AddressRoom), Remark: strings.TrimSpace(input.Remark),
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := order.ValidateNew(); err != nil {
		return Order{}, err
	}
	if err := s.repository.CreateOrder(ctx, &order, items); err != nil {
		return Order{}, err
	}
	return order, nil
}

func (s *OrderService) MarkPaid(ctx context.Context, orderPublicID string, expectedRowVersion uint64, payNo string) (Order, error) {
	payNo = strings.TrimSpace(payNo)
	if payNo == "" || len(payNo) > 64 {
		return Order{}, ErrInvalidRecord
	}
	return s.transitionOrder(ctx, orderPublicID, expectedRowVersion, OrderPendingPayment, OrderPaid, &payNo, nil)
}

func (s *OrderService) Accept(ctx context.Context, orderPublicID string, expectedRowVersion uint64) (Order, error) {
	return s.transitionOrder(ctx, orderPublicID, expectedRowVersion, OrderPaid, OrderAccepted, nil, nil)
}

func (s *OrderService) StartPreparing(ctx context.Context, orderPublicID string, expectedRowVersion uint64) (Order, error) {
	return s.transitionOrder(ctx, orderPublicID, expectedRowVersion, OrderAccepted, OrderPreparing, nil, nil)
}

func (s *OrderService) MarkReady(ctx context.Context, orderPublicID string, expectedRowVersion uint64) (Order, error) {
	return s.transitionOrder(ctx, orderPublicID, expectedRowVersion, OrderPreparing, OrderReadyForPickup, nil, nil)
}

func (s *OrderService) StartDelivering(ctx context.Context, orderPublicID string, expectedRowVersion, riderUserID uint64) (Order, error) {
	if riderUserID == 0 {
		return Order{}, ErrRiderRequired
	}
	return s.transitionOrder(ctx, orderPublicID, expectedRowVersion, OrderReadyForPickup, OrderDelivering, nil, &riderUserID)
}

func (s *OrderService) CompleteDelivery(ctx context.Context, orderPublicID string, expectedRowVersion uint64) (Order, error) {
	if err := validateOrderMutation(orderPublicID, expectedRowVersion); err != nil {
		return Order{}, err
	}
	earningPublicID, err := NewULID()
	if err != nil {
		return Order{}, err
	}
	return s.repository.CompleteDelivery(ctx, orderPublicID, expectedRowVersion, earningPublicID, s.now().UTC())
}

func (s *OrderService) Cancel(ctx context.Context, orderPublicID string, expectedRowVersion uint64) (Order, error) {
	return s.transitionOrder(ctx, orderPublicID, expectedRowVersion, OrderPendingPayment, OrderCancelled, nil, nil)
}

func (s *OrderService) Refund(ctx context.Context, orderPublicID string, expectedStatus OrderStatus, expectedRowVersion uint64) (Order, error) {
	if expectedStatus != OrderPaid && expectedStatus != OrderAccepted {
		return Order{}, ErrInvalidTransition
	}
	return s.transitionOrder(ctx, orderPublicID, expectedRowVersion, expectedStatus, OrderRefunded, nil, nil)
}

func (s *OrderService) FindOrder(ctx context.Context, orderPublicID string) (Order, error) {
	if !ValidULID(orderPublicID) {
		return Order{}, ErrInvalidULID
	}
	return s.repository.FindOrder(ctx, orderPublicID)
}

func (s *OrderService) ListOrderItems(ctx context.Context, orderPublicID string) ([]OrderItem, error) {
	if !ValidULID(orderPublicID) {
		return nil, ErrInvalidULID
	}
	return s.repository.ListOrderItems(ctx, orderPublicID)
}

func (s *OrderService) GetActiveFeeConfig(ctx context.Context) (FeeConfig, error) {
	return s.repository.GetActiveFeeConfig(ctx)
}

func (s *OrderService) UpdateFeeConfig(ctx context.Context, riderFeeCents, platformCutCents, expectedRowVersion uint64) (FeeConfig, error) {
	if expectedRowVersion == 0 {
		return FeeConfig{}, ErrInvalidRecord
	}
	if _, ok := checkedAddAmount(riderFeeCents, platformCutCents); !ok {
		return FeeConfig{}, ErrAmountOverflow
	}
	return s.repository.UpdateFeeConfig(ctx, riderFeeCents, platformCutCents, expectedRowVersion, s.now().UTC())
}

func (s *OrderService) FindEarningByOrder(ctx context.Context, orderPublicID string) (RiderEarningRecord, error) {
	if !ValidULID(orderPublicID) {
		return RiderEarningRecord{}, ErrInvalidULID
	}
	return s.repository.FindEarningByOrder(ctx, orderPublicID)
}

func (s *OrderService) MarkSettled(ctx context.Context, earningPublicID string, expectedRowVersion uint64) (RiderEarningRecord, error) {
	if !ValidULID(earningPublicID) || expectedRowVersion == 0 {
		return RiderEarningRecord{}, ErrInvalidRecord
	}
	return s.repository.MarkSettled(ctx, earningPublicID, expectedRowVersion, s.now().UTC())
}

func (s *OrderService) transitionOrder(
	ctx context.Context,
	orderPublicID string,
	expectedRowVersion uint64,
	from, to OrderStatus,
	payNo *string,
	riderUserID *uint64,
) (Order, error) {
	if err := validateOrderMutation(orderPublicID, expectedRowVersion); err != nil {
		return Order{}, err
	}
	if !from.Valid() || !to.Valid() || !CanTransitionOrder(from, to) {
		return Order{}, ErrInvalidTransition
	}
	return s.repository.TransitionOrderStatus(ctx, orderPublicID, expectedRowVersion, from, to, payNo, riderUserID, s.now().UTC())
}

func validateOrderMutation(orderPublicID string, expectedRowVersion uint64) error {
	if !ValidULID(strings.TrimSpace(orderPublicID)) {
		return ErrInvalidULID
	}
	if expectedRowVersion == 0 {
		return ErrInvalidRecord
	}
	return nil
}
