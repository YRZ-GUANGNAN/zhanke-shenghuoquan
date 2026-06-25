package takeoutrider

import (
	"context"
	"errors"
	"strings"

	"campus-circle/backend/internal/module/takeout"
)

var (
	ErrRiderRegistrationRequired = errors.New("rider registration required")
	ErrForbiddenFulfillment      = errors.New("fulfillment does not belong to current rider")
	ErrInsufficientBalance       = errors.New("insufficient rider withdrawable balance")
)

type Service struct {
	riders      *takeout.RiderService
	orders      *takeout.OrderService
	withdrawals *takeout.WithdrawService
	repository  Repository
}

func NewService(
	riders *takeout.RiderService,
	orders *takeout.OrderService,
	withdrawals *takeout.WithdrawService,
	repository Repository,
) (*Service, error) {
	if riders == nil || orders == nil || withdrawals == nil || repository == nil {
		return nil, takeout.ErrInvalidRecord
	}
	return &Service{riders: riders, orders: orders, withdrawals: withdrawals, repository: repository}, nil
}

func (s *Service) FindRider(ctx context.Context, riderUserID uint64) (takeout.Rider, error) {
	return s.riders.FindRider(ctx, riderUserID)
}

func (s *Service) RegisterRider(ctx context.Context, input takeout.RegisterRiderInput) (takeout.Rider, error) {
	existing, err := s.riders.FindRider(ctx, input.RiderUserID)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, takeout.ErrNotFound) {
		return takeout.Rider{}, err
	}
	return s.riders.RegisterRider(ctx, input)
}

func (s *Service) ListClaimableOrders(ctx context.Context, riderUserID uint64) ([]OrderDetail, error) {
	if _, err := s.requireRider(ctx, riderUserID); err != nil {
		return nil, err
	}
	orders, err := s.riders.ListClaimableOrders(ctx)
	if err != nil {
		return nil, err
	}
	return s.orderDetails(ctx, orders)
}

func (s *Service) ClaimOrder(ctx context.Context, riderUserID uint64, orderPublicID string, expectedRowVersion uint64) (FulfillmentDetail, error) {
	if _, err := s.requireRider(ctx, riderUserID); err != nil {
		return FulfillmentDetail{}, err
	}
	value, err := s.riders.ClaimOrder(ctx, riderUserID, strings.TrimSpace(orderPublicID), expectedRowVersion)
	if err != nil {
		return FulfillmentDetail{}, err
	}
	return s.fulfillmentDetail(ctx, value)
}

func (s *Service) ListFulfillments(ctx context.Context, riderUserID uint64, status takeout.FulfillmentStatus, page, size int) ([]FulfillmentDetail, int64, error) {
	if status != "" && !status.Valid() {
		return nil, 0, takeout.ErrInvalidRecord
	}
	if _, err := s.requireRider(ctx, riderUserID); err != nil {
		return nil, 0, err
	}
	values, total, err := s.repository.ListFulfillmentsByRider(ctx, riderUserID, status, page, size)
	if err != nil {
		return nil, 0, err
	}
	result := make([]FulfillmentDetail, 0, len(values))
	for _, value := range values {
		detail, err := s.fulfillmentDetail(ctx, value)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, detail)
	}
	return result, total, nil
}

func (s *Service) FindFulfillment(ctx context.Context, riderUserID uint64, fulfillmentPublicID string) (FulfillmentDetail, error) {
	value, err := s.ownedFulfillment(ctx, riderUserID, fulfillmentPublicID)
	if err != nil {
		return FulfillmentDetail{}, err
	}
	return s.fulfillmentDetail(ctx, value)
}

func (s *Service) StartToStore(ctx context.Context, riderUserID uint64, fulfillmentPublicID string, expectedRowVersion uint64) (FulfillmentDetail, error) {
	if _, err := s.ownedFulfillment(ctx, riderUserID, fulfillmentPublicID); err != nil {
		return FulfillmentDetail{}, err
	}
	value, err := s.riders.StartToStore(ctx, fulfillmentPublicID, expectedRowVersion)
	if err != nil {
		return FulfillmentDetail{}, err
	}
	return s.fulfillmentDetail(ctx, value)
}

func (s *Service) MarkPicked(ctx context.Context, riderUserID uint64, fulfillmentPublicID string, expectedRowVersion uint64) (FulfillmentDetail, error) {
	if _, err := s.ownedFulfillment(ctx, riderUserID, fulfillmentPublicID); err != nil {
		return FulfillmentDetail{}, err
	}
	value, err := s.riders.MarkPicked(ctx, fulfillmentPublicID, expectedRowVersion)
	if err != nil {
		return FulfillmentDetail{}, err
	}
	return s.fulfillmentDetail(ctx, value)
}

func (s *Service) StartToUser(ctx context.Context, riderUserID uint64, fulfillmentPublicID string, expectedRowVersion uint64) (FulfillmentDetail, error) {
	if _, err := s.ownedFulfillment(ctx, riderUserID, fulfillmentPublicID); err != nil {
		return FulfillmentDetail{}, err
	}
	value, err := s.riders.StartToUser(ctx, fulfillmentPublicID, expectedRowVersion)
	if err != nil {
		return FulfillmentDetail{}, err
	}
	return s.fulfillmentDetail(ctx, value)
}

func (s *Service) Complete(ctx context.Context, riderUserID uint64, fulfillmentPublicID string, expectedRowVersion uint64) (FulfillmentDetail, error) {
	if _, err := s.ownedFulfillment(ctx, riderUserID, fulfillmentPublicID); err != nil {
		return FulfillmentDetail{}, err
	}
	value, err := s.riders.CompleteFulfillment(ctx, fulfillmentPublicID, expectedRowVersion)
	if err != nil {
		return FulfillmentDetail{}, err
	}
	return s.fulfillmentDetail(ctx, value)
}

func (s *Service) Wallet(ctx context.Context, riderUserID uint64, page, size int) (WalletView, error) {
	if _, err := s.requireRider(ctx, riderUserID); err != nil {
		return WalletView{}, err
	}
	facts, err := s.repository.WalletFacts(ctx, riderUserID)
	if err != nil {
		return WalletView{}, err
	}
	earnings, total, err := s.repository.ListEarningsByRider(ctx, riderUserID, page, size)
	if err != nil {
		return WalletView{}, err
	}
	return WalletView{
		RiderUserID: riderUserID, PendingSettlementCents: facts.PendingSettlementCents,
		PendingWithdrawCents: facts.PendingWithdrawCents, AvailableWithdrawCents: facts.AvailableCents,
		SettledCents: facts.SettledCents, RecentEarnings: mapValues(earnings, earningView),
		RecentEarningsPage: page, RecentEarningsPageSize: size, RecentEarningsTotal: total,
	}, nil
}

func (s *Service) ListWithdrawals(ctx context.Context, riderUserID uint64, status takeout.WithdrawStatus, page, size int) ([]takeout.WithdrawRequest, int64, error) {
	if status != "" && !status.Valid() {
		return nil, 0, takeout.ErrInvalidRecord
	}
	if _, err := s.requireRider(ctx, riderUserID); err != nil {
		return nil, 0, err
	}
	return s.repository.ListWithdrawalsByRider(ctx, riderUserID, status, page, size)
}

func (s *Service) CreateWithdrawal(ctx context.Context, riderUserID, amountCents uint64) (takeout.WithdrawRequest, error) {
	if amountCents == 0 {
		return takeout.WithdrawRequest{}, takeout.ErrInvalidRecord
	}
	if _, err := s.requireRider(ctx, riderUserID); err != nil {
		return takeout.WithdrawRequest{}, err
	}
	facts, err := s.repository.WalletFacts(ctx, riderUserID)
	if err != nil {
		return takeout.WithdrawRequest{}, err
	}
	if amountCents > facts.AvailableCents {
		return takeout.WithdrawRequest{}, ErrInsufficientBalance
	}
	return s.withdrawals.CreateWithdrawRequest(ctx, riderUserID, amountCents)
}

func (s *Service) requireRider(ctx context.Context, riderUserID uint64) (takeout.Rider, error) {
	value, err := s.riders.FindRider(ctx, riderUserID)
	if errors.Is(err, takeout.ErrNotFound) {
		return takeout.Rider{}, ErrRiderRegistrationRequired
	}
	return value, err
}

func (s *Service) ownedFulfillment(ctx context.Context, riderUserID uint64, fulfillmentPublicID string) (takeout.Fulfillment, error) {
	if _, err := s.requireRider(ctx, riderUserID); err != nil {
		return takeout.Fulfillment{}, err
	}
	value, err := s.repository.FindFulfillment(ctx, strings.TrimSpace(fulfillmentPublicID))
	if err != nil {
		return takeout.Fulfillment{}, err
	}
	if value.RiderUserID != riderUserID {
		return takeout.Fulfillment{}, ErrForbiddenFulfillment
	}
	return value, nil
}

type OrderDetail struct {
	Order takeout.Order
	Items []takeout.OrderItem
}

type FulfillmentDetail struct {
	Fulfillment takeout.Fulfillment
	Order       takeout.Order
	Items       []takeout.OrderItem
}

func (s *Service) orderDetails(ctx context.Context, orders []takeout.Order) ([]OrderDetail, error) {
	result := make([]OrderDetail, 0, len(orders))
	for _, value := range orders {
		items, err := s.orders.ListOrderItems(ctx, value.OrderPublicID)
		if err != nil {
			return nil, err
		}
		result = append(result, OrderDetail{Order: value, Items: items})
	}
	return result, nil
}

func (s *Service) fulfillmentDetail(ctx context.Context, fulfillment takeout.Fulfillment) (FulfillmentDetail, error) {
	order, err := s.orders.FindOrder(ctx, fulfillment.OrderPublicID)
	if err != nil {
		return FulfillmentDetail{}, err
	}
	items, err := s.orders.ListOrderItems(ctx, order.OrderPublicID)
	if err != nil {
		return FulfillmentDetail{}, err
	}
	return FulfillmentDetail{Fulfillment: fulfillment, Order: order, Items: items}, nil
}

func (s *Service) merchantNames(ctx context.Context, details []OrderDetail) (map[string]string, error) {
	ids := uniqueMerchantIDs(details)
	return s.repository.MerchantNames(ctx, ids)
}

func (s *Service) merchantNamesForFulfillments(ctx context.Context, details []FulfillmentDetail) (map[string]string, error) {
	ids := make([]string, 0, len(details))
	seen := map[string]struct{}{}
	for _, detail := range details {
		if _, ok := seen[detail.Order.MerchantPublicID]; ok {
			continue
		}
		seen[detail.Order.MerchantPublicID] = struct{}{}
		ids = append(ids, detail.Order.MerchantPublicID)
	}
	return s.repository.MerchantNames(ctx, ids)
}

func uniqueMerchantIDs(details []OrderDetail) []string {
	ids := make([]string, 0, len(details))
	seen := map[string]struct{}{}
	for _, detail := range details {
		if _, ok := seen[detail.Order.MerchantPublicID]; ok {
			continue
		}
		seen[detail.Order.MerchantPublicID] = struct{}{}
		ids = append(ids, detail.Order.MerchantPublicID)
	}
	return ids
}
