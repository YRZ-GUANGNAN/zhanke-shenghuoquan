package takeout

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

type PaymentService struct {
	repository PaymentRepository
	orders     *OrderService
	now        func() time.Time
}

func NewPaymentService(repository PaymentRepository, orders *OrderService, now func() time.Time) (*PaymentService, error) {
	if repository == nil || orders == nil {
		return nil, ErrInvalidRecord
	}
	if now == nil {
		now = time.Now
	}
	return &PaymentService{repository: repository, orders: orders, now: now}, nil
}

type PaymentAttemptResult struct {
	Attempt PaymentAttempt
	Order   Order
}

func (s *PaymentService) CreateWechatPaymentAttempt(ctx context.Context, buyerUserID uint64, orderPublicID string) (PaymentAttemptResult, error) {
	order, err := s.orderForPayment(ctx, buyerUserID, orderPublicID)
	if err != nil {
		return PaymentAttemptResult{}, err
	}
	if latest, findErr := s.repository.FindLatestPaymentAttemptByOrder(ctx, order.OrderPublicID, PaymentProviderWechat); findErr == nil {
		if latest.AmountCents == order.TotalAmountCents && (latest.Status == PaymentCreated || latest.Status == PaymentPrepayIssued) {
			return PaymentAttemptResult{Attempt: latest, Order: order}, nil
		}
	} else if findErr != nil && !errors.Is(findErr, ErrNotFound) {
		return PaymentAttemptResult{}, findErr
	}
	attempt, err := s.newPaymentAttempt(order, PaymentProviderWechat, PaymentCreated, nil, nil, nil)
	if err != nil {
		return PaymentAttemptResult{}, err
	}
	if err := attempt.ValidateNew(); err != nil {
		return PaymentAttemptResult{}, err
	}
	if err := s.repository.CreatePaymentAttempt(ctx, &attempt); err != nil {
		return PaymentAttemptResult{}, err
	}
	return PaymentAttemptResult{Attempt: attempt, Order: order}, nil
}

func (s *PaymentService) MarkPrepayIssued(ctx context.Context, paymentPublicID string, expectedRowVersion uint64, prepayID string) (PaymentAttempt, error) {
	if !ValidULID(strings.TrimSpace(paymentPublicID)) || expectedRowVersion == 0 ||
		strings.TrimSpace(prepayID) == "" || len(strings.TrimSpace(prepayID)) > 128 {
		return PaymentAttempt{}, ErrInvalidRecord
	}
	return s.repository.MarkPaymentAttemptPrepayIssued(ctx, strings.TrimSpace(paymentPublicID), expectedRowVersion, strings.TrimSpace(prepayID), s.now().UTC())
}

func (s *PaymentService) PayWithLocalMock(ctx context.Context, buyerUserID uint64, orderPublicID string) (PaymentAttemptResult, error) {
	order, err := s.orders.FindOrder(ctx, strings.TrimSpace(orderPublicID))
	if err != nil {
		return PaymentAttemptResult{}, err
	}
	if order.BuyerUserID != buyerUserID {
		return PaymentAttemptResult{}, ErrPaymentForbidden
	}
	if order.Status == OrderPaid {
		latest, findErr := s.repository.FindLatestPaymentAttemptByOrder(ctx, order.OrderPublicID, PaymentProviderMock)
		if findErr != nil {
			return PaymentAttemptResult{}, findErr
		}
		if latest.Status != PaymentPaid {
			return PaymentAttemptResult{}, ErrInvalidRecord
		}
		return PaymentAttemptResult{Attempt: latest, Order: order}, nil
	}
	if order.Status != OrderPendingPayment {
		return PaymentAttemptResult{}, ErrPaymentNotPending
	}
	traceID, err := NewULID()
	if err != nil {
		return PaymentAttemptResult{}, err
	}
	payNo := "MOCK-" + traceID
	paidAt := s.now().UTC()
	attempt, err := s.newPaymentAttempt(order, PaymentProviderMock, PaymentPaid, nil, &payNo, &traceID)
	if err != nil {
		return PaymentAttemptResult{}, err
	}
	attempt.PaidAt = &paidAt
	attempt.UpdatedAt = paidAt
	if err := attempt.ValidateNew(); err != nil {
		return PaymentAttemptResult{}, err
	}
	attempt, order, err = s.repository.CreatePaidPaymentAttemptAndMarkOrderPaid(ctx, &attempt, order.RowVersion)
	if err != nil {
		return PaymentAttemptResult{}, err
	}
	return PaymentAttemptResult{Attempt: attempt, Order: order}, nil
}

type PaymentConfirmationInput struct {
	Provider            PaymentProvider
	OutTradeNo          string
	WechatTransactionID string
	AmountCents         uint64
	RawBody             []byte
	PaidAt              time.Time
}

func (s *PaymentService) ConfirmProviderPayment(ctx context.Context, input PaymentConfirmationInput) (PaymentAttemptResult, bool, error) {
	input.OutTradeNo = strings.TrimSpace(input.OutTradeNo)
	input.WechatTransactionID = strings.TrimSpace(input.WechatTransactionID)
	if !input.Provider.Valid() || input.OutTradeNo == "" || input.WechatTransactionID == "" || input.AmountCents == 0 || len(input.RawBody) == 0 {
		return PaymentAttemptResult{}, false, ErrInvalidRecord
	}
	paidAt := input.PaidAt.UTC()
	if paidAt.IsZero() {
		paidAt = s.now().UTC()
	}
	bodyHash := sha256.Sum256(input.RawBody)
	hashText := hex.EncodeToString(bodyHash[:])
	publicID, err := NewULID()
	if err != nil {
		return PaymentAttemptResult{}, false, err
	}
	processedAt := paidAt
	notification := PaymentNotification{
		NotificationPublicID: publicID,
		Provider:             input.Provider,
		OutTradeNo:           input.OutTradeNo,
		WechatTransactionID:  &input.WechatTransactionID,
		BodySHA256:           hashText,
		Status:               PaymentNotificationProcessed,
		ProcessedAt:          &processedAt,
		CreatedAt:            s.now().UTC(),
	}
	if err := notification.ValidateNew(); err != nil {
		return PaymentAttemptResult{}, false, err
	}
	payNo := fmt.Sprintf("%s-%s", strings.ToUpper(string(input.Provider)), input.WechatTransactionID)
	if len(payNo) > 64 {
		payNo = payNo[:64]
	}
	attempt, order, duplicate, err := s.repository.RecordPaymentNotificationAndMarkPaid(
		ctx, &notification, input.OutTradeNo, input.WechatTransactionID, payNo, input.AmountCents, paidAt,
	)
	if err != nil {
		return PaymentAttemptResult{}, duplicate, err
	}
	return PaymentAttemptResult{Attempt: attempt, Order: order}, duplicate, nil
}

func (s *PaymentService) orderForPayment(ctx context.Context, buyerUserID uint64, orderPublicID string) (Order, error) {
	if buyerUserID == 0 || !ValidULID(strings.TrimSpace(orderPublicID)) {
		return Order{}, ErrInvalidRecord
	}
	order, err := s.orders.FindOrder(ctx, strings.TrimSpace(orderPublicID))
	if err != nil {
		return Order{}, err
	}
	if order.BuyerUserID != buyerUserID {
		return Order{}, ErrPaymentForbidden
	}
	if order.Status != OrderPendingPayment {
		return Order{}, ErrPaymentNotPending
	}
	if order.TotalAmountCents == 0 {
		return Order{}, ErrInvalidRecord
	}
	return order, nil
}

func (s *PaymentService) newPaymentAttempt(order Order, provider PaymentProvider, status PaymentStatus, prepayID, payNo, mockTraceID *string) (PaymentAttempt, error) {
	paymentPublicID, err := NewULID()
	if err != nil {
		return PaymentAttempt{}, err
	}
	outTradeSuffix, err := NewULID()
	if err != nil {
		return PaymentAttempt{}, err
	}
	now := s.now().UTC()
	attempt := PaymentAttempt{
		PaymentPublicID: paymentPublicID,
		OrderPublicID:   order.OrderPublicID,
		BuyerUserID:     order.BuyerUserID,
		Provider:        provider,
		Status:          status,
		AmountCents:     order.TotalAmountCents,
		Currency:        "CNY",
		OutTradeNo:      "G4" + outTradeSuffix,
		PrepayID:        prepayID,
		PayNo:           payNo,
		MockTraceID:     mockTraceID,
		RowVersion:      1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return attempt, nil
}
