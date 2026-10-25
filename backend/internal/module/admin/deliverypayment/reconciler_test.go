package deliverypayment

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
)

type reconciliationFundsFake struct {
	paymentLease  delivery.PaymentVerificationLease
	paymentFound  bool
	refundLease   delivery.RefundVerificationLease
	refundFound   bool
	paymentUpdate *delivery.PaymentVerificationUpdate
	refundUpdate  *delivery.RefundVerificationUpdate
	confirmation  *delivery.PaymentConfirmation
	observation   *delivery.RefundObservation
}

func (f *reconciliationFundsFake) ClaimPaymentVerification(
	context.Context, string, time.Time, time.Duration,
) (delivery.PaymentVerificationLease, bool, error) {
	return f.paymentLease, f.paymentFound, nil
}

func (f *reconciliationFundsFake) ClaimNextPaymentVerification(
	context.Context, time.Time, time.Duration,
) (delivery.PaymentVerificationLease, bool, error) {
	if !f.paymentFound {
		return delivery.PaymentVerificationLease{}, false, nil
	}
	f.paymentFound = false
	return f.paymentLease, true, nil
}

func (f *reconciliationFundsFake) UpdatePaymentVerification(
	_ context.Context,
	_ delivery.PaymentVerificationLease,
	update delivery.PaymentVerificationUpdate,
) error {
	f.paymentUpdate = &update
	return nil
}

func (f *reconciliationFundsFake) ConfirmWechatPayment(
	_ context.Context,
	input delivery.PaymentConfirmation,
) (delivery.FundsResult, bool, error) {
	f.confirmation = &input
	return delivery.FundsResult{}, false, nil
}

func (f *reconciliationFundsFake) ClaimRefundVerification(
	context.Context, string, time.Time, time.Duration,
) (delivery.RefundVerificationLease, bool, error) {
	return f.refundLease, f.refundFound, nil
}

func (f *reconciliationFundsFake) ClaimNextRefundVerification(
	context.Context, time.Time, time.Duration,
) (delivery.RefundVerificationLease, bool, error) {
	if !f.refundFound {
		return delivery.RefundVerificationLease{}, false, nil
	}
	f.refundFound = false
	return f.refundLease, true, nil
}

func (f *reconciliationFundsFake) UpdateRefundVerification(
	_ context.Context,
	_ delivery.RefundVerificationLease,
	update delivery.RefundVerificationUpdate,
) error {
	f.refundUpdate = &update
	return nil
}

func (f *reconciliationFundsFake) ObserveRefund(
	_ context.Context,
	input delivery.RefundObservation,
) (delivery.RefundResult, error) {
	f.observation = &input
	return delivery.RefundResult{}, nil
}

type reconciliationProviderFake struct {
	paymentResult PaymentQueryResult
	paymentErr    error
	refundResult  RefundQueryResult
	refundErr     error
	paymentQuery  *PaymentQueryRequest
	refundQuery   *RefundQueryRequest
	prepayCalls   int
	closeTradeNo  string
	closeErr      error
}

func (*reconciliationProviderFake) Provider() delivery.PaymentProvider { return delivery.PaymentWechat }
func (p *reconciliationProviderFake) CreatePrepay(context.Context, PrepayRequest) (PrepayResult, error) {
	p.prepayCalls++
	return PrepayResult{}, errors.New("reconciler must not create a second payment")
}
func (*reconciliationProviderFake) ClientParams(string) (*JSAPIParameter, error) {
	return nil, errors.New("unused")
}
func (*reconciliationProviderFake) ParsePaymentNotification(*http.Request, []byte) (PaymentNotificationResult, error) {
	return PaymentNotificationResult{}, errors.New("unused")
}
func (p *reconciliationProviderFake) QueryPayment(_ context.Context, input PaymentQueryRequest) (PaymentQueryResult, error) {
	p.paymentQuery = &input
	return p.paymentResult, p.paymentErr
}
func (p *reconciliationProviderFake) ClosePayment(_ context.Context, outTradeNo string) error {
	p.closeTradeNo = outTradeNo
	return p.closeErr
}
func (*reconciliationProviderFake) CreateRefund(context.Context, RefundRequest) (RefundProviderResult, error) {
	return RefundProviderResult{}, errors.New("unused")
}
func (*reconciliationProviderFake) ParseRefundNotification(*http.Request, []byte) (RefundNotificationResult, error) {
	return RefundNotificationResult{}, errors.New("unused")
}
func (p *reconciliationProviderFake) QueryRefund(_ context.Context, input RefundQueryRequest) (RefundQueryResult, error) {
	p.refundQuery = &input
	return p.refundResult, p.refundErr
}

func TestReconcilerConfirmsOriginalPaymentThroughDomainEntry(t *testing.T) {
	now := time.Date(2026, 10, 25, 8, 0, 0, 0, time.UTC)
	paidAt := now.Add(-time.Minute)
	payer := "openid-merchant"
	funds := &reconciliationFundsFake{
		paymentFound: true,
		paymentLease: delivery.PaymentVerificationLease{Attempt: delivery.PaymentAttempt{
			PaymentPublicID: "01K00000000000000000000000", OutTradeNo: "D3ORIGINAL",
			AmountCents: 150, PayerOpenID: &payer, VerifyAttemptCount: 1, RowVersion: 3,
		}},
	}
	provider := &reconciliationProviderFake{paymentResult: PaymentQueryResult{
		OutTradeNo: "D3ORIGINAL", WechatTransactionID: "WXTX-1", PayerOpenID: payer,
		State: "SUCCESS", AmountCents: 150, PaidAt: &paidAt,
		RawBody: []byte(`{"trade_state":"SUCCESS"}`),
	}}
	reconciler, err := NewReconciler(funds, provider, ReconcilerConfig{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if err := reconciler.ReconcilePayment(context.Background(), funds.paymentLease.Attempt.PaymentPublicID); err != nil {
		t.Fatal(err)
	}
	if provider.paymentQuery == nil || provider.paymentQuery.OutTradeNo != "D3ORIGINAL" ||
		provider.paymentQuery.PayerOpenID != payer || provider.paymentQuery.AmountCents != 150 {
		t.Fatalf("payment query=%#v", provider.paymentQuery)
	}
	if provider.prepayCalls != 0 {
		t.Fatalf("created a replacement payment %d times", provider.prepayCalls)
	}
	if funds.confirmation == nil || funds.confirmation.OutTradeNo != "D3ORIGINAL" ||
		funds.confirmation.PayerOpenID != payer || funds.confirmation.ProviderStatus != "SUCCESS" ||
		string(funds.confirmation.RawBody) != `{"trade_state":"SUCCESS"}` {
		t.Fatalf("confirmation=%#v", funds.confirmation)
	}
}

func TestReconcilerBacksOffUnknownPaymentAndPausesConfigurationFailure(t *testing.T) {
	now := time.Date(2026, 10, 25, 8, 30, 0, 0, time.UTC)
	payer := "openid-merchant"
	baseLease := delivery.PaymentVerificationLease{Attempt: delivery.PaymentAttempt{
		PaymentPublicID: "01K00000000000000000000000", OutTradeNo: "D3SAME",
		AmountCents: 150, PayerOpenID: &payer, VerifyAttemptCount: 2, RowVersion: 4,
	}}
	for _, test := range []struct {
		name        string
		providerErr error
		wantCode    string
		wantRetry   bool
	}{
		{name: "network unknown", providerErr: errors.New("temporary network failure"), wantCode: "provider_unavailable", wantRetry: true},
		{name: "configuration", providerErr: ErrProviderNotConfigured, wantCode: "provider_not_configured", wantRetry: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			funds := &reconciliationFundsFake{paymentFound: true, paymentLease: baseLease}
			provider := &reconciliationProviderFake{paymentErr: test.providerErr}
			reconciler, err := NewReconciler(funds, provider, ReconcilerConfig{
				Now: func() time.Time { return now }, RetryBase: 5 * time.Second,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := reconciler.ReconcilePayment(context.Background(), baseLease.Attempt.PaymentPublicID); err != nil {
				t.Fatal(err)
			}
			if funds.paymentUpdate == nil || funds.paymentUpdate.LastErrorCode != test.wantCode {
				t.Fatalf("update=%#v", funds.paymentUpdate)
			}
			if (funds.paymentUpdate.NextVerifyAt != nil) != test.wantRetry {
				t.Fatalf("nextVerifyAt=%v wantRetry=%v", funds.paymentUpdate.NextVerifyAt, test.wantRetry)
			}
			if provider.prepayCalls != 0 || baseLease.Attempt.OutTradeNo != "D3SAME" {
				t.Fatal("unknown result changed the original payment identity")
			}
		})
	}
}

func TestReconcilerAppliesRefundQueryWithLeaseVersion(t *testing.T) {
	now := time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)
	outTradeNo := "D3PAID"
	funds := &reconciliationFundsFake{
		refundFound: true,
		refundLease: delivery.RefundVerificationLease{Refund: delivery.Refund{
			RefundPublicID: "01K00000000000000000000001", OutTradeNo: &outTradeNo,
			OutRefundNo: "D3R-ORIGINAL", AmountCents: 150, RowVersion: 7,
		}},
	}
	provider := &reconciliationProviderFake{refundResult: RefundQueryResult{
		OutTradeNo: outTradeNo, OutRefundNo: "D3R-ORIGINAL", WechatRefundID: "WXR-1",
		State: "PROCESSING", AmountCents: 150, TotalCents: 150,
	}}
	reconciler, err := NewReconciler(funds, provider, ReconcilerConfig{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if err := reconciler.ReconcileRefund(context.Background(), funds.refundLease.Refund.RefundPublicID); err != nil {
		t.Fatal(err)
	}
	if provider.refundQuery == nil || provider.refundQuery.OutRefundNo != "D3R-ORIGINAL" ||
		provider.refundQuery.OutTradeNo != outTradeNo {
		t.Fatalf("refund query=%#v", provider.refundQuery)
	}
	if funds.observation == nil || funds.observation.ExpectedRowVersion != 7 ||
		funds.observation.OutTradeNo != outTradeNo || funds.observation.ProviderStatus != delivery.RefundProviderProcessing {
		t.Fatalf("observation=%#v", funds.observation)
	}
}

func TestReconcilerQueriesThenClosesExpiredUnpaidOriginalOrder(t *testing.T) {
	now := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	payer := "openid-merchant"
	funds := &reconciliationFundsFake{
		paymentFound: true,
		paymentLease: delivery.PaymentVerificationLease{Attempt: delivery.PaymentAttempt{
			PaymentPublicID: "01K00000000000000000000000", OutTradeNo: "D3EXPIRED",
			AmountCents: 150, PayerOpenID: &payer, VerifyAttemptCount: 3, RowVersion: 5,
			CreatedAt: now.Add(-31 * time.Minute),
		}},
	}
	provider := &reconciliationProviderFake{paymentResult: PaymentQueryResult{
		OutTradeNo: "D3EXPIRED", State: "NOTPAY",
	}}
	reconciler, err := NewReconciler(funds, provider, ReconcilerConfig{
		Now: func() time.Time { return now }, CloseAfter: 30 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := reconciler.ReconcilePayment(context.Background(), funds.paymentLease.Attempt.PaymentPublicID); err != nil {
		t.Fatal(err)
	}
	if provider.paymentQuery == nil || provider.closeTradeNo != "D3EXPIRED" {
		t.Fatalf("query=%#v closeTradeNo=%q", provider.paymentQuery, provider.closeTradeNo)
	}
	if funds.paymentUpdate == nil || funds.paymentUpdate.Status != delivery.PaymentClosed ||
		funds.paymentUpdate.ProviderStatus != "CLOSED" {
		t.Fatalf("update=%#v", funds.paymentUpdate)
	}
}

func TestReconcilerRunStopsWhenServiceContextIsCancelled(t *testing.T) {
	funds := &reconciliationFundsFake{}
	provider := &reconciliationProviderFake{}
	reconciler, err := NewReconciler(funds, provider, ReconcilerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- reconciler.Run(ctx, time.Hour) }()
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("run returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("reconciler did not stop after cancellation")
	}
}
