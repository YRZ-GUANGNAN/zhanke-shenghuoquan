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
	paymentLease     delivery.PaymentVerificationLease
	paymentFound     bool
	refundLease      delivery.RefundVerificationLease
	refundFound      bool
	paymentUpdate    *delivery.PaymentVerificationUpdate
	paymentUpdateErr error
	refundUpdate     *delivery.RefundVerificationUpdate
	confirmation     *delivery.PaymentConfirmation
	observation      *delivery.RefundObservation
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
	return f.paymentUpdateErr
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
	paymentResult    PaymentQueryResult
	paymentErr       error
	refundResult     RefundQueryResult
	refundErr        error
	paymentQuery     *PaymentQueryRequest
	refundQuery      *RefundQueryRequest
	prepayCalls      int
	refundCreates    int
	refundCreate     RefundProviderResult
	refundCreateErr  error
	refundRequest    *RefundRequest
	refundQueryHook  func(context.Context)
	refundCreateHook func(context.Context)
	closeTradeNo     string
	closeErr         error
	paymentQueryHook func(context.Context)
	closeHook        func(context.Context)
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
func (p *reconciliationProviderFake) QueryPayment(ctx context.Context, input PaymentQueryRequest) (PaymentQueryResult, error) {
	p.paymentQuery = &input
	if p.paymentQueryHook != nil {
		p.paymentQueryHook(ctx)
	}
	return p.paymentResult, p.paymentErr
}
func (p *reconciliationProviderFake) ClosePayment(ctx context.Context, outTradeNo string) error {
	p.closeTradeNo = outTradeNo
	if p.closeHook != nil {
		p.closeHook(ctx)
	}
	return p.closeErr
}
func (p *reconciliationProviderFake) CreateRefund(ctx context.Context, input RefundRequest) (RefundProviderResult, error) {
	p.refundCreates++
	p.refundRequest = &input
	if p.refundCreateHook != nil {
		p.refundCreateHook(ctx)
	}
	return p.refundCreate, p.refundCreateErr
}
func (*reconciliationProviderFake) ParseRefundNotification(*http.Request, []byte) (RefundNotificationResult, error) {
	return RefundNotificationResult{}, errors.New("unused")
}
func (p *reconciliationProviderFake) QueryRefund(ctx context.Context, input RefundQueryRequest) (RefundQueryResult, error) {
	p.refundQuery = &input
	if p.refundQueryHook != nil {
		p.refundQueryHook(ctx)
	}
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

func TestReconcilerReusesRefundNumberWhenProviderHasNoRefund(t *testing.T) {
	now := time.Date(2026, 10, 25, 9, 30, 0, 0, time.UTC)
	leaseUntil := now.Add(30 * time.Second)
	outTradeNo := "D3PAID"
	funds := &reconciliationFundsFake{
		refundFound: true,
		refundLease: delivery.RefundVerificationLease{Refund: delivery.Refund{
			RefundPublicID: "01K00000000000000000000001", OutTradeNo: &outTradeNo,
			OutRefundNo: "D3R-ORIGINAL", AmountCents: 150, RowVersion: 7,
			Source: delivery.SettlementWechat, Status: delivery.RefundPending, VerifyAttemptCount: 1,
			VerifyLeaseUntil: &leaseUntil,
		}},
	}
	provider := &reconciliationProviderFake{
		refundErr:    ErrRefundNotFound,
		refundCreate: RefundProviderResult{WechatRefundID: "WXR-RETRY", Status: "PROCESSING"},
	}
	reconciler, err := NewReconciler(funds, provider, ReconcilerConfig{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if err := reconciler.ReconcileRefund(context.Background(), funds.refundLease.Refund.RefundPublicID); err != nil {
		t.Fatal(err)
	}
	if provider.refundCreates != 1 || provider.refundRequest == nil {
		t.Fatalf("refund creates=%d request=%#v", provider.refundCreates, provider.refundRequest)
	}
	if provider.refundRequest.OutTradeNo != outTradeNo || provider.refundRequest.OutRefundNo != "D3R-ORIGINAL" ||
		provider.refundRequest.AmountCents != 150 {
		t.Fatalf("refund request=%#v", provider.refundRequest)
	}
	if funds.observation == nil || funds.observation.OutRefundNo != "D3R-ORIGINAL" ||
		funds.observation.WechatRefundID != "WXR-RETRY" ||
		funds.observation.ProviderStatus != delivery.RefundProviderProcessing {
		t.Fatalf("observation=%#v", funds.observation)
	}
}

func TestReconcilerStopsRefundResubmissionAfterBound(t *testing.T) {
	now := time.Date(2026, 10, 25, 9, 45, 0, 0, time.UTC)
	leaseUntil := now.Add(30 * time.Second)
	outTradeNo := "D3PAID"
	funds := &reconciliationFundsFake{
		refundFound: true,
		refundLease: delivery.RefundVerificationLease{Refund: delivery.Refund{
			RefundPublicID: "01K00000000000000000000001", OutTradeNo: &outTradeNo,
			OutRefundNo: "D3R-ORIGINAL", AmountCents: 150, RowVersion: 7,
			Source: delivery.SettlementWechat, Status: delivery.RefundPending, VerifyAttemptCount: 4,
			VerifyLeaseUntil: &leaseUntil,
		}},
	}
	provider := &reconciliationProviderFake{refundErr: ErrRefundNotFound}
	reconciler, err := NewReconciler(funds, provider, ReconcilerConfig{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if err := reconciler.ReconcileRefund(context.Background(), funds.refundLease.Refund.RefundPublicID); err != nil {
		t.Fatal(err)
	}
	if provider.refundCreates != 0 || funds.refundUpdate == nil || funds.refundUpdate.LastErrorCode != "provider_not_found" {
		t.Fatalf("creates=%d update=%#v", provider.refundCreates, funds.refundUpdate)
	}
	if funds.refundUpdate.NextVerifyAt != nil {
		t.Fatal("exhausted submission budget must pause automatic retries")
	}
}

func TestReconcilerDoesNotResubmitKnownOrUntrustedRefunds(t *testing.T) {
	now := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(30 * time.Second)
	outTradeNo, knownRefundID := "D3PAID", "WXKNOWN"
	base := delivery.Refund{
		RefundPublicID: "01K00000000000000000000001", OutTradeNo: &outTradeNo,
		OutRefundNo: "D3R-ORIGINAL", AmountCents: 150, RowVersion: 7,
		Source: delivery.SettlementWechat, Status: delivery.RefundPending, VerifyAttemptCount: 1,
		VerifyLeaseUntil: &leaseUntil,
	}
	for _, test := range []struct {
		name     string
		change   func(*delivery.Refund)
		queryErr error
	}{
		{name: "known provider refund", change: func(refund *delivery.Refund) { refund.WechatRefundID = &knownRefundID }, queryErr: ErrRefundNotFound},
		{name: "already processing", change: func(refund *delivery.Refund) { refund.ProviderStatus = "PROCESSING" }, queryErr: ErrRefundNotFound},
		{name: "terminal local status", change: func(refund *delivery.Refund) { refund.Status = delivery.RefundSucceeded }, queryErr: ErrRefundNotFound},
		{name: "wallet source", change: func(refund *delivery.Refund) { refund.Source = delivery.SettlementWallet }, queryErr: ErrRefundNotFound},
		{name: "no lease attempt", change: func(refund *delivery.Refund) { refund.VerifyAttemptCount = 0 }, queryErr: ErrRefundNotFound},
		{name: "unknown network result", queryErr: errors.New("connection timeout")},
		{name: "untrusted result", queryErr: ErrUntrustedProviderResponse},
		{name: "generic rejection", queryErr: ErrProviderRejected},
	} {
		t.Run(test.name, func(t *testing.T) {
			refund := base
			if test.change != nil {
				test.change(&refund)
			}
			funds := &reconciliationFundsFake{refundFound: true, refundLease: delivery.RefundVerificationLease{Refund: refund}}
			provider := &reconciliationProviderFake{refundErr: test.queryErr}
			reconciler, err := NewReconciler(funds, provider, ReconcilerConfig{Now: func() time.Time { return now }})
			if err != nil {
				t.Fatal(err)
			}
			if err := reconciler.ReconcileRefund(context.Background(), refund.RefundPublicID); err != nil {
				t.Fatal(err)
			}
			if provider.refundCreates != 0 || funds.observation != nil {
				t.Fatalf("known/untrusted refund resubmitted: creates=%d observation=%#v", provider.refundCreates, funds.observation)
			}
		})
	}
}

func TestReconcilerQueriesThenClosesExpiredUnpaidOriginalOrder(t *testing.T) {
	now := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	leaseUntil := now.Add(30 * time.Second)
	payer := "openid-merchant"
	funds := &reconciliationFundsFake{
		paymentFound: true,
		paymentLease: delivery.PaymentVerificationLease{Attempt: delivery.PaymentAttempt{
			PaymentPublicID: "01K00000000000000000000000", OutTradeNo: "D3EXPIRED",
			AmountCents: 150, PayerOpenID: &payer, VerifyAttemptCount: 3, RowVersion: 5,
			CreatedAt: now.Add(-31 * time.Minute), VerifyLeaseUntil: &leaseUntil,
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

func pendingRefundLease(now time.Time, attempt uint32) delivery.RefundVerificationLease {
	outTradeNo := "D3PAID"
	leaseUntil := now.Add(30 * time.Second)
	return delivery.RefundVerificationLease{Refund: delivery.Refund{
		RefundPublicID: "01K00000000000000000000001", OutTradeNo: &outTradeNo,
		OutRefundNo: "D3R-ORIGINAL", AmountCents: 150, RowVersion: 7,
		Source: delivery.SettlementWechat, Status: delivery.RefundPending,
		VerifyAttemptCount: attempt, VerifyLeaseUntil: &leaseUntil,
	}}
}

func TestReconcilerMissingRefundRecoveryRequiresLiveLease(t *testing.T) {
	for _, test := range []struct {
		name          string
		missingLease  bool
		queryDuration time.Duration
	}{
		{name: "missing lease", missingLease: true},
		{name: "lease expires at query completion", queryDuration: 30 * time.Second},
		{name: "lease already expired", queryDuration: 31 * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			now := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
			lease := pendingRefundLease(now, 1)
			if test.missingLease {
				lease.Refund.VerifyLeaseUntil = nil
			}
			funds := &reconciliationFundsFake{refundFound: true, refundLease: lease}
			provider := &reconciliationProviderFake{
				refundErr:       ErrRefundNotFound,
				refundCreate:    RefundProviderResult{WechatRefundID: "WXR-1", Status: "PROCESSING"},
				refundQueryHook: func(context.Context) { now = now.Add(test.queryDuration) },
			}
			reconciler, err := NewReconciler(funds, provider, ReconcilerConfig{Now: func() time.Time { return now }})
			if err != nil {
				t.Fatal(err)
			}
			if err := reconciler.ReconcileRefund(context.Background(), lease.Refund.RefundPublicID); err != nil {
				t.Fatal(err)
			}
			if provider.refundCreates != 0 || funds.observation != nil || funds.refundUpdate != nil {
				t.Fatalf("expired lease performed write: creates=%d observation=%#v update=%#v", provider.refundCreates, funds.observation, funds.refundUpdate)
			}
		})
	}
}

func TestReconcilerRefundRecoveryUsesRemainingLeaseTimeout(t *testing.T) {
	now := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	lease := pendingRefundLease(now, 1)
	funds := &reconciliationFundsFake{refundFound: true, refundLease: lease}
	provider := &reconciliationProviderFake{
		refundErr:       ErrRefundNotFound,
		refundCreate:    RefundProviderResult{WechatRefundID: "WXR-1", Status: "PROCESSING"},
		refundQueryHook: func(context.Context) { now = now.Add(29 * time.Second) },
		refundCreateHook: func(ctx context.Context) {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > time.Second {
				t.Errorf("create deadline exceeds remaining lease: deadline=%v", deadline)
			}
		},
	}
	reconciler, err := NewReconciler(funds, provider, ReconcilerConfig{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if err := reconciler.ReconcileRefund(context.Background(), lease.Refund.RefundPublicID); err != nil {
		t.Fatal(err)
	}
	if provider.refundCreates != 1 {
		t.Fatalf("creates=%d", provider.refundCreates)
	}
}

func TestReconcilerRefundRecoveryReusesPersistentBudget(t *testing.T) {
	now := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
	for attempt := uint32(1); attempt <= 4; attempt++ {
		// Each iteration constructs a new worker, as after a process restart.
		lease := pendingRefundLease(now, attempt)
		funds := &reconciliationFundsFake{refundFound: true, refundLease: lease}
		provider := &reconciliationProviderFake{refundErr: ErrRefundNotFound, refundCreateErr: errors.New("connection lost after submission")}
		reconciler, err := NewReconciler(funds, provider, ReconcilerConfig{Now: func() time.Time { return now }})
		if err != nil {
			t.Fatal(err)
		}
		if err := reconciler.ReconcileRefund(context.Background(), lease.Refund.RefundPublicID); err != nil {
			t.Fatal(err)
		}
		if funds.observation != nil || funds.refundUpdate == nil {
			t.Fatalf("attempt=%d observation=%#v update=%#v", attempt, funds.observation, funds.refundUpdate)
		}
		if attempt <= 3 {
			if provider.refundCreates != 1 || provider.refundRequest.OutRefundNo != "D3R-ORIGINAL" || provider.refundRequest.OutTradeNo != "D3PAID" || provider.refundRequest.AmountCents != 150 ||
				funds.refundUpdate.NextVerifyAt == nil || funds.refundUpdate.LastErrorCode != "provider_unavailable" {
				t.Fatalf("attempt=%d creates=%d request=%#v update=%#v", attempt, provider.refundCreates, provider.refundRequest, funds.refundUpdate)
			}
		} else if provider.refundCreates != 0 || funds.refundUpdate.NextVerifyAt != nil || funds.refundUpdate.LastErrorCode != "provider_not_found" {
			t.Fatalf("restarted worker reset submission budget: creates=%d update=%#v", provider.refundCreates, funds.refundUpdate)
		}
	}
}

func TestReconcilerRejectsMalformedRefundCreationObservation(t *testing.T) {
	for _, result := range []RefundProviderResult{
		{Status: "PROCESSING"},
		{WechatRefundID: "WXR-1", Status: "UNKNOWN"},
	} {
		now := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
		lease := pendingRefundLease(now, 1)
		funds := &reconciliationFundsFake{refundFound: true, refundLease: lease}
		provider := &reconciliationProviderFake{refundErr: ErrRefundNotFound, refundCreate: result}
		reconciler, err := NewReconciler(funds, provider, ReconcilerConfig{Now: func() time.Time { return now }})
		if err != nil {
			t.Fatal(err)
		}
		if err := reconciler.ReconcileRefund(context.Background(), lease.Refund.RefundPublicID); err != nil {
			t.Fatal(err)
		}
		if funds.observation != nil || funds.refundUpdate == nil || funds.refundUpdate.LastErrorCode != "untrusted_provider_response" || funds.refundUpdate.NextVerifyAt == nil {
			t.Fatalf("malformed creation observed: result=%#v observation=%#v update=%#v", result, funds.observation, funds.refundUpdate)
		}
	}
}

func TestReconcilerRefundRecoveryDoesNotWriteAfterCancellation(t *testing.T) {
	for _, stage := range []string{"query", "create"} {
		t.Run(stage, func(t *testing.T) {
			now := time.Date(2026, 10, 25, 10, 0, 0, 0, time.UTC)
			lease := pendingRefundLease(now, 1)
			funds := &reconciliationFundsFake{refundFound: true, refundLease: lease}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			provider := &reconciliationProviderFake{refundErr: ErrRefundNotFound, refundCreate: RefundProviderResult{WechatRefundID: "WXR-1", Status: "PROCESSING"}}
			if stage == "query" {
				provider.refundQueryHook = func(context.Context) { cancel() }
			} else {
				provider.refundCreateHook = func(context.Context) { cancel() }
			}
			reconciler, err := NewReconciler(funds, provider, ReconcilerConfig{Now: func() time.Time { return now }})
			if err != nil {
				t.Fatal(err)
			}
			if err := reconciler.ReconcileRefund(ctx, lease.Refund.RefundPublicID); !errors.Is(err, context.Canceled) {
				t.Fatalf("error=%v", err)
			}
			if funds.observation != nil || funds.refundUpdate != nil || (stage == "query" && provider.refundCreates != 0) {
				t.Fatalf("cancellation performed write: creates=%d observation=%#v update=%#v", provider.refundCreates, funds.observation, funds.refundUpdate)
			}
		})
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
