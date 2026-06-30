package delivery

import (
	"context"
	"strings"
	"time"
)

const maxRechargeCents uint64 = 10_000_000

type FundsOrderReader interface {
	FindOwnedByMerchant(context.Context, string, uint64) (Order, error)
	Find(context.Context, string) (Order, error)
}

type FundsService struct {
	repository FundsRepository
	orders     FundsOrderReader
	now        func() time.Time
}

func NewFundsService(repository FundsRepository, orders FundsOrderReader, now func() time.Time) (*FundsService, error) {
	if repository == nil || orders == nil {
		return nil, ErrInvalidRecord
	}
	if now == nil {
		now = time.Now
	}
	return &FundsService{repository: repository, orders: orders, now: now}, nil
}

type PaymentStartResult struct {
	Attempt PaymentAttempt
	Order   *Order
	Wallet  *MerchantWallet
	Ledger  *WalletLedger
}

func (s *FundsService) Wallet(ctx context.Context, merchantUserID uint64) (MerchantWallet, error) {
	if merchantUserID == 0 {
		return MerchantWallet{}, ErrInvalidRecord
	}
	return s.repository.Wallet(ctx, merchantUserID, s.now().UTC())
}

func (s *FundsService) Ledger(ctx context.Context, merchantUserID uint64, page Page) ([]WalletLedger, int64, error) {
	return s.repository.ListLedger(ctx, merchantUserID, page)
}

func (s *FundsService) Payment(ctx context.Context, merchantUserID uint64, paymentPublicID string) (PaymentAttempt, error) {
	if merchantUserID == 0 || !ValidULID(strings.TrimSpace(paymentPublicID)) {
		return PaymentAttempt{}, ErrInvalidRecord
	}
	return s.repository.FindPaymentAttemptOwned(ctx, strings.TrimSpace(paymentPublicID), merchantUserID)
}

func (s *FundsService) PayOrderWithWallet(
	ctx context.Context,
	merchantUserID uint64,
	orderPublicID string,
	expectedRowVersion uint64,
	idempotencyKey string,
) (FundsResult, error) {
	if err := validateFundsCommand(merchantUserID, orderPublicID, expectedRowVersion, idempotencyKey); err != nil {
		return FundsResult{}, err
	}
	return s.repository.PayOrderWithWallet(
		ctx, merchantUserID, strings.TrimSpace(orderPublicID), expectedRowVersion,
		strings.TrimSpace(idempotencyKey), s.now().UTC(),
	)
}

func (s *FundsService) StartOrderPayment(
	ctx context.Context,
	merchantUserID uint64,
	orderPublicID string,
	expectedRowVersion uint64,
	clientRequestID string,
	provider PaymentProvider,
) (PaymentStartResult, error) {
	if err := validateFundsCommand(merchantUserID, orderPublicID, expectedRowVersion, clientRequestID); err != nil {
		return PaymentStartResult{}, err
	}
	order, err := s.orders.FindOwnedByMerchant(ctx, strings.TrimSpace(orderPublicID), merchantUserID)
	if err != nil {
		return PaymentStartResult{}, err
	}
	if order.Status != StatusPendingFeePayment || order.RowVersion != expectedRowVersion {
		return PaymentStartResult{}, ErrPaymentNotPending
	}
	attempt, err := s.newAttempt(
		merchantUserID, PaymentOrderFee, &order.DeliveryOrderPublicID,
		provider, order.DeliveryFeeCents, clientRequestID,
	)
	if err != nil {
		return PaymentStartResult{}, err
	}
	if provider == PaymentMock {
		now := s.now().UTC()
		traceID, err := NewULID()
		if err != nil {
			return PaymentStartResult{}, err
		}
		attempt.Status = PaymentPaid
		attempt.MockTraceID = &traceID
		attempt.PaidAt = &now
		result, err := s.repository.PayOrderWithMock(ctx, &attempt, expectedRowVersion, now)
		if err != nil {
			return PaymentStartResult{}, err
		}
		return PaymentStartResult{Attempt: *result.Attempt, Order: result.Order}, nil
	}
	created, err := s.repository.CreatePaymentAttempt(ctx, &attempt)
	if err != nil {
		return PaymentStartResult{}, err
	}
	return PaymentStartResult{Attempt: created, Order: &order}, nil
}

func (s *FundsService) StartRecharge(
	ctx context.Context,
	merchantUserID uint64,
	amountCents uint64,
	clientRequestID string,
	provider PaymentProvider,
) (PaymentStartResult, error) {
	if merchantUserID == 0 || amountCents == 0 || amountCents > maxRechargeCents ||
		!ValidULID(strings.TrimSpace(clientRequestID)) {
		return PaymentStartResult{}, ErrInvalidRecord
	}
	attempt, err := s.newAttempt(
		merchantUserID, PaymentWalletRecharge, nil, provider, amountCents, clientRequestID,
	)
	if err != nil {
		return PaymentStartResult{}, err
	}
	if provider == PaymentMock {
		now := s.now().UTC()
		traceID, err := NewULID()
		if err != nil {
			return PaymentStartResult{}, err
		}
		attempt.Status = PaymentPaid
		attempt.MockTraceID = &traceID
		attempt.PaidAt = &now
		result, err := s.repository.CreditMockRecharge(ctx, &attempt, now)
		if err != nil {
			return PaymentStartResult{}, err
		}
		return PaymentStartResult{Attempt: *result.Attempt, Wallet: result.Wallet, Ledger: result.Ledger}, nil
	}
	created, err := s.repository.CreatePaymentAttempt(ctx, &attempt)
	return PaymentStartResult{Attempt: created}, err
}

func (s *FundsService) MarkPrepayIssued(
	ctx context.Context,
	paymentPublicID string,
	expectedRowVersion uint64,
	prepayID string,
) (PaymentAttempt, error) {
	if !ValidULID(strings.TrimSpace(paymentPublicID)) || expectedRowVersion == 0 || strings.TrimSpace(prepayID) == "" {
		return PaymentAttempt{}, ErrInvalidRecord
	}
	return s.repository.MarkPaymentPrepayIssued(
		ctx, strings.TrimSpace(paymentPublicID), expectedRowVersion,
		strings.TrimSpace(prepayID), s.now().UTC(),
	)
}

func (s *FundsService) ConfirmWechatPayment(ctx context.Context, input PaymentConfirmation) (FundsResult, bool, error) {
	input.OutTradeNo = strings.TrimSpace(input.OutTradeNo)
	input.WechatTransactionID = strings.TrimSpace(input.WechatTransactionID)
	if input.OutTradeNo == "" || input.WechatTransactionID == "" || input.AmountCents == 0 || len(input.RawBody) == 0 {
		return FundsResult{}, false, ErrInvalidRecord
	}
	if input.PaidAt.IsZero() {
		input.PaidAt = s.now().UTC()
	} else {
		input.PaidAt = input.PaidAt.UTC()
	}
	return s.repository.ConfirmWechatPayment(ctx, input)
}

func (s *FundsService) RequestMerchantRefund(
	ctx context.Context,
	merchantUserID uint64,
	orderPublicID string,
	expectedRowVersion uint64,
	idempotencyKey string,
) (RefundResult, error) {
	return s.requestRefund(ctx, merchantUserID, orderPublicID, expectedRowVersion, idempotencyKey, false)
}

func (s *FundsService) RequestAdminRefund(
	ctx context.Context,
	orderPublicID string,
	expectedRowVersion uint64,
	idempotencyKey string,
) (RefundResult, error) {
	orderPublicID = strings.TrimSpace(orderPublicID)
	if !ValidULID(orderPublicID) {
		return RefundResult{}, ErrInvalidULID
	}
	order, err := s.orders.Find(ctx, orderPublicID)
	if err != nil {
		return RefundResult{}, err
	}
	return s.requestRefund(ctx, order.MerchantUserID, orderPublicID, expectedRowVersion, idempotencyKey, true)
}

func (s *FundsService) requestRefund(
	ctx context.Context,
	merchantUserID uint64,
	orderPublicID string,
	expectedRowVersion uint64,
	idempotencyKey string,
	admin bool,
) (RefundResult, error) {
	if err := validateFundsCommand(merchantUserID, orderPublicID, expectedRowVersion, idempotencyKey); err != nil {
		return RefundResult{}, err
	}
	return s.repository.RequestRefund(ctx, RefundRequest{
		MerchantUserID: merchantUserID, OrderPublicID: strings.TrimSpace(orderPublicID),
		IdempotencyKey: strings.TrimSpace(idempotencyKey), Admin: admin,
		ExpectedRowVersion: expectedRowVersion, Now: s.now().UTC(),
	})
}

func (s *FundsService) CompleteRefund(
	ctx context.Context,
	outRefundNo, wechatRefundID string,
	success bool,
	reason string,
) (RefundResult, error) {
	if strings.TrimSpace(outRefundNo) == "" {
		return RefundResult{}, ErrInvalidRecord
	}
	return s.repository.CompleteRefund(
		ctx, strings.TrimSpace(outRefundNo), strings.TrimSpace(wechatRefundID),
		success, strings.TrimSpace(reason), s.now().UTC(),
	)
}

func (s *FundsService) newAttempt(
	merchantUserID uint64,
	purpose PaymentPurpose,
	orderPublicID *string,
	provider PaymentProvider,
	amountCents uint64,
	clientRequestID string,
) (PaymentAttempt, error) {
	publicID, err := NewULID()
	if err != nil {
		return PaymentAttempt{}, err
	}
	suffix, err := NewULID()
	if err != nil {
		return PaymentAttempt{}, err
	}
	now := s.now().UTC()
	value := PaymentAttempt{
		PaymentPublicID: publicID, MerchantUserID: merchantUserID, Purpose: purpose,
		DeliveryOrderPublicID: orderPublicID, Provider: provider, Status: PaymentCreated,
		AmountCents: amountCents, Currency: "CNY", ClientRequestID: strings.TrimSpace(clientRequestID),
		OutTradeNo: "D3" + suffix, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := validatePaymentAttempt(value); err != nil {
		return PaymentAttempt{}, err
	}
	return value, nil
}

func validateFundsCommand(merchantUserID uint64, orderPublicID string, expectedRowVersion uint64, idempotencyKey string) error {
	if merchantUserID == 0 || !ValidULID(strings.TrimSpace(orderPublicID)) ||
		expectedRowVersion == 0 || !ValidULID(strings.TrimSpace(idempotencyKey)) {
		return ErrInvalidRecord
	}
	return nil
}
