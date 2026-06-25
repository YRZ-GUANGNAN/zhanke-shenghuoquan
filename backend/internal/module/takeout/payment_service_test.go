package takeout

import (
	"context"
	"errors"
	"testing"
	"time"
)

type paymentRepositoryStub struct {
	created PaymentAttempt
	latest  PaymentAttempt
	order   Order
}

func (s *paymentRepositoryStub) CreatePaymentAttempt(_ context.Context, value *PaymentAttempt) error {
	s.created = *value
	return nil
}
func (s *paymentRepositoryStub) FindPaymentAttempt(context.Context, string) (PaymentAttempt, error) {
	return PaymentAttempt{}, ErrNotFound
}
func (s *paymentRepositoryStub) FindPaymentAttemptByOutTradeNo(context.Context, string) (PaymentAttempt, error) {
	return PaymentAttempt{}, ErrNotFound
}
func (s *paymentRepositoryStub) FindLatestPaymentAttemptByOrder(context.Context, string, PaymentProvider) (PaymentAttempt, error) {
	if s.latest.PaymentPublicID == "" {
		return PaymentAttempt{}, ErrNotFound
	}
	return s.latest, nil
}
func (s *paymentRepositoryStub) MarkPaymentAttemptPrepayIssued(_ context.Context, _ string, _ uint64, prepayID string, at time.Time) (PaymentAttempt, error) {
	s.created.Status = PaymentPrepayIssued
	s.created.PrepayID = &prepayID
	s.created.RowVersion++
	s.created.UpdatedAt = at
	return s.created, nil
}
func (s *paymentRepositoryStub) CreatePaidPaymentAttemptAndMarkOrderPaid(_ context.Context, value *PaymentAttempt, _ uint64) (PaymentAttempt, Order, error) {
	s.created = *value
	s.order.Status = OrderPaid
	s.order.PayNo = value.PayNo
	s.order.PaidAt = value.PaidAt
	s.order.RowVersion++
	return *value, s.order, nil
}
func (s *paymentRepositoryStub) RecordPaymentNotificationAndMarkPaid(_ context.Context, _ *PaymentNotification, outTradeNo, transactionID, payNo string, amountCents uint64, paidAt time.Time) (PaymentAttempt, Order, bool, error) {
	if s.latest.OutTradeNo != outTradeNo {
		return PaymentAttempt{}, Order{}, false, ErrNotFound
	}
	if s.latest.AmountCents != amountCents {
		return PaymentAttempt{}, Order{}, false, ErrPaymentAmount
	}
	s.latest.Status = PaymentPaid
	s.latest.WechatTransactionID = &transactionID
	s.latest.PayNo = &payNo
	s.latest.PaidAt = &paidAt
	s.order.Status = OrderPaid
	s.order.PayNo = &payNo
	s.order.PaidAt = &paidAt
	return s.latest, s.order, false, nil
}

func TestPaymentServiceMockPayUsesServerOrderFacts(t *testing.T) {
	now := time.Date(2026, 6, 25, 9, 0, 0, 0, time.UTC)
	order := Order{
		OrderPublicID: "01J00000000000000000000301", BuyerUserID: 77, MerchantPublicID: "01J00000000000000000000302",
		Status: OrderPendingPayment, TotalAmountCents: 1234, RowVersion: 3,
	}
	orderRepo := &orderRepositoryStub{createdOrder: order}
	orders, err := NewOrderService(orderRepo, catalogReaderStub{}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	paymentRepo := &paymentRepositoryStub{order: order}
	payments, err := NewPaymentService(paymentRepo, orders, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	result, err := payments.PayWithLocalMock(context.Background(), 77, order.OrderPublicID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Order.Status != OrderPaid || result.Attempt.Provider != PaymentProviderMock ||
		result.Attempt.Status != PaymentPaid || result.Attempt.AmountCents != order.TotalAmountCents ||
		result.Attempt.PayNo == nil || result.Attempt.PaidAt == nil {
		t.Fatalf("mock payment did not use server facts: %+v", result)
	}
}

func TestPaymentServiceRejectsCrossBuyerPayment(t *testing.T) {
	now := time.Now().UTC()
	order := Order{OrderPublicID: "01J00000000000000000000311", BuyerUserID: 77, Status: OrderPendingPayment, TotalAmountCents: 1, RowVersion: 1}
	orders, _ := NewOrderService(&orderRepositoryStub{createdOrder: order}, catalogReaderStub{}, func() time.Time { return now })
	payments, _ := NewPaymentService(&paymentRepositoryStub{order: order}, orders, func() time.Time { return now })
	if _, err := payments.PayWithLocalMock(context.Background(), 88, order.OrderPublicID); !errors.Is(err, ErrPaymentForbidden) {
		t.Fatalf("cross buyer returned %v", err)
	}
}

func TestPaymentServiceConfirmsProviderNotificationIdempotently(t *testing.T) {
	now := time.Date(2026, 6, 25, 10, 0, 0, 0, time.UTC)
	attempt := PaymentAttempt{
		PaymentPublicID: "01J00000000000000000000321", OrderPublicID: "01J00000000000000000000322",
		BuyerUserID: 77, Provider: PaymentProviderWechat, Status: PaymentPrepayIssued,
		AmountCents: 1234, Currency: "CNY", OutTradeNo: "G4TESTOUTTRADE", RowVersion: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	order := Order{OrderPublicID: attempt.OrderPublicID, BuyerUserID: 77, Status: OrderPendingPayment, TotalAmountCents: 1234, RowVersion: 1}
	orders, _ := NewOrderService(&orderRepositoryStub{createdOrder: order}, catalogReaderStub{}, func() time.Time { return now })
	paymentRepo := &paymentRepositoryStub{latest: attempt, order: order}
	payments, _ := NewPaymentService(paymentRepo, orders, func() time.Time { return now })
	result, duplicate, err := payments.ConfirmProviderPayment(context.Background(), PaymentConfirmationInput{
		Provider: PaymentProviderWechat, OutTradeNo: attempt.OutTradeNo, WechatTransactionID: "4200000000001",
		AmountCents: 1234, RawBody: []byte(`{"id":"notification"}`), PaidAt: now,
	})
	if err != nil || duplicate || result.Order.Status != OrderPaid || result.Attempt.Status != PaymentPaid {
		t.Fatalf("confirmation result=%+v duplicate=%v err=%v", result, duplicate, err)
	}
}
