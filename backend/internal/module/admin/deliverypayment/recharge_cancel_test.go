package deliverypayment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"github.com/gin-gonic/gin"
)

func TestRechargeCancellationReconciler(t *testing.T) {
	for _, tc := range []struct {
		name, state                             string
		cancelled                               bool
		providerErr, closeErr, updateErr        error
		expectClose, expectPaid, expectTerminal bool
	}{
		{name: "requested unpaid closes immediately", state: "NOTPAY", cancelled: true, expectClose: true, expectTerminal: true},
		{name: "ordinary unpaid keeps global grace period", state: "NOTPAY"},
		{name: "user paying remains pending", state: "USERPAYING", cancelled: true},
		{name: "query timeout remains pending", cancelled: true, providerErr: errors.New("timeout")},
		{name: "close timeout remains pending", state: "NOTPAY", cancelled: true, closeErr: errors.New("timeout"), expectClose: true},
		{name: "payment succeeded is credited instead of closed", state: "SUCCESS", cancelled: true, expectPaid: true},
		{name: "already closed confirmed by provider", state: "CLOSED", cancelled: true, expectTerminal: true},
		{name: "callback CAS wins", state: "NOTPAY", cancelled: true, expectClose: true, expectTerminal: true, updateErr: delivery.ErrVerificationLease},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
			until := now.Add(30 * time.Second)
			payer := "cancel-payer"
			attempt := delivery.PaymentAttempt{ID: 1, PaymentPublicID: "01K00000000000000000000000", OutTradeNo: "CANCEL-ORIGINAL", Purpose: delivery.PaymentWalletRecharge, Provider: delivery.PaymentWechat, Status: delivery.PaymentPrepayIssued, AmountCents: 123, PayerOpenID: &payer, CreatedAt: now.Add(-time.Second), VerifyLeaseUntil: &until, RowVersion: 3}
			if tc.cancelled {
				attempt.CancelRequestedAt = &now
			}
			funds := &reconciliationFundsFake{paymentLease: delivery.PaymentVerificationLease{Attempt: attempt}, paymentFound: true, paymentUpdateErr: tc.updateErr}
			provider := &reconciliationProviderFake{paymentResult: PaymentQueryResult{State: tc.state, OutTradeNo: attempt.OutTradeNo, AmountCents: 123, PayerOpenID: payer, WechatTransactionID: "TX-CANCEL", PaidAt: &now, RawBody: []byte(`{"fixture":"verified"}`)}, paymentErr: tc.providerErr, closeErr: tc.closeErr}
			r, err := NewReconciler(funds, provider, ReconcilerConfig{Now: func() time.Time { return now }})
			if err != nil {
				t.Fatal(err)
			}
			if err := r.ReconcilePayment(context.Background(), attempt.PaymentPublicID); err != nil {
				t.Fatal(err)
			}
			if (provider.closeTradeNo != "") != tc.expectClose {
				t.Fatalf("close=%q", provider.closeTradeNo)
			}
			if (funds.confirmation != nil) != tc.expectPaid {
				t.Fatalf("confirmation=%+v", funds.confirmation)
			}
			terminal := funds.paymentUpdate != nil && funds.paymentUpdate.Status != ""
			if terminal != tc.expectTerminal {
				t.Fatalf("update=%+v", funds.paymentUpdate)
			}
			if provider.prepayCalls != 0 {
				t.Fatal("cancellation created a payment")
			}
			if !tc.expectPaid && !tc.expectTerminal && (funds.paymentUpdate == nil || funds.paymentUpdate.NextVerifyAt == nil) {
				t.Fatal("pending cancellation lost background retry")
			}
		})
	}
}

func TestRechargeCancellationDoesNotCloseAfterLeaseOrContextEnds(t *testing.T) {
	for _, cancelContext := range []bool{false, true} {
		now := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
		until := now.Add(30 * time.Second)
		payer := "payer"
		funds := &reconciliationFundsFake{paymentFound: true, paymentLease: delivery.PaymentVerificationLease{Attempt: delivery.PaymentAttempt{ID: 1, RowVersion: 2, Purpose: delivery.PaymentWalletRecharge, PayerOpenID: &payer, CancelRequestedAt: &now, VerifyLeaseUntil: &until}}}
		ctx, cancel := context.WithCancel(context.Background())
		provider := &reconciliationProviderFake{paymentResult: PaymentQueryResult{State: "NOTPAY"}, paymentQueryHook: func(context.Context) {
			if cancelContext {
				cancel()
			} else {
				now = until
			}
		}}
		r, _ := NewReconciler(funds, provider, ReconcilerConfig{Now: func() time.Time { return now }})
		err := r.ReconcilePayment(ctx, "01K00000000000000000000000")
		cancel()
		if cancelContext && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if provider.closeTradeNo != "" || funds.paymentUpdate != nil {
			t.Fatal("expired operation still closed or updated payment")
		}
	}
}

type rechargeCancelRepositoryFake struct {
	delivery.FundsRepository
	delivery.FundsOrderReader
	value         delivery.PaymentAttempt
	err           error
	merchantID    uint64
	publicID      string
	requestID     string
	lookupMissing bool
}

func (f *rechargeCancelRepositoryFake) RequestRechargeCancellationByRequest(_ context.Context, candidate *delivery.PaymentAttempt, _ time.Time) (delivery.PaymentAttempt, error) {
	f.merchantID = candidate.MerchantUserID
	f.requestID = candidate.ClientRequestID
	if candidate.Provider != delivery.PaymentWechat {
		panic("cancellation must never create mock credits")
	}
	return f.value, f.err
}
func (f *rechargeCancelRepositoryFake) FindRechargeByRequestOwned(_ context.Context, merchantID uint64, requestID string, _ uint64) (*delivery.PaymentAttempt, error) {
	f.merchantID = merchantID
	f.requestID = requestID
	if f.lookupMissing {
		return nil, f.err
	}
	return &f.value, f.err
}

func TestRechargeByRequestEndpointsNeverIssuePrepay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, missing := range []bool{false, true} {
		repo := &rechargeCancelRepositoryFake{lookupMissing: missing, value: delivery.PaymentAttempt{PaymentPublicID: "01K00000000000000000000000", Purpose: delivery.PaymentWalletRecharge, Provider: delivery.PaymentWechat, Status: delivery.PaymentClosed, AmountCents: 123}}
		funds, _ := delivery.NewFundsService(repo, repo, nil)
		provider := &terminalGuardProvider{}
		h := NewHandler(funds, provider, func(*gin.Context) uint64 { return 42 }, func(*gin.Context) string { return "payer" })
		r := gin.New()
		h.RegisterMerchant(r.Group("/delivery/merchant"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/delivery/merchant/wallet/recharges/by-request/01K00000000000000000000001?amountCents=123", nil))
		var response struct {
			Data *PaymentView `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if w.Code != http.StatusOK || (response.Data == nil) != missing {
			t.Fatalf("lookup status=%d body=%s", w.Code, w.Body)
		}
		w = httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/delivery/merchant/wallet/recharges/close", strings.NewReader(`{"clientRequestId":"01K00000000000000000000001","amountCents":123}`))
		request.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, request)
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if w.Code != http.StatusOK || response.Data == nil || response.Data.Status != delivery.PaymentClosed || response.Data.Wechat != nil || repo.merchantID != 42 || repo.requestID != "01K00000000000000000000001" || provider.prepayCalls != 0 {
			t.Fatalf("close status=%d body=%s", w.Code, w.Body)
		}
	}
}

func (f *rechargeCancelRepositoryFake) RequestRechargeCancellation(_ context.Context, id string, merchantID uint64, _ time.Time) (delivery.PaymentAttempt, error) {
	f.merchantID = merchantID
	f.publicID = id
	return f.value, f.err
}
func (f *rechargeCancelRepositoryFake) FindPaymentAttemptOwned(context.Context, string, uint64) (delivery.PaymentAttempt, error) {
	return f.value, f.err
}

func TestRechargeCancelEndpointReturnsPaymentWithoutPrepay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, status := range []delivery.PaymentStatus{delivery.PaymentPrepayIssued, delivery.PaymentPaid, delivery.PaymentClosed} {
		now := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
		repo := &rechargeCancelRepositoryFake{value: delivery.PaymentAttempt{PaymentPublicID: "01K00000000000000000000000", Purpose: delivery.PaymentWalletRecharge, Provider: delivery.PaymentWechat, Status: status, AmountCents: 123, CancelRequestedAt: &now}}
		funds, _ := delivery.NewFundsService(repo, repo, func() time.Time { return now })
		provider := &terminalGuardProvider{}
		h := NewHandler(funds, provider, func(*gin.Context) uint64 { return 42 }, func(*gin.Context) string { return "payer" })
		r := gin.New()
		h.RegisterMerchant(r.Group("/delivery/merchant"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/delivery/merchant/wallet/recharges/01K00000000000000000000000/close", nil))
		var response struct {
			Data PaymentView `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if w.Code != http.StatusOK || response.Data.Status != status || response.Data.PaymentPublicID != repo.value.PaymentPublicID || response.Data.CancelRequestedAt == nil || response.Data.Wechat != nil || repo.merchantID != 42 || provider.prepayCalls != 0 {
			t.Fatalf("status=%d body=%s", w.Code, w.Body)
		}
	}
}

func TestIssuePrepayDoesNotReopenCancelledRecharge(t *testing.T) {
	now := time.Now()
	provider := &terminalGuardProvider{}
	h := &Handler{provider: provider}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	for _, status := range []delivery.PaymentStatus{delivery.PaymentCreated, delivery.PaymentPrepayIssued} {
		value := delivery.PaymentStartResult{Attempt: delivery.PaymentAttempt{Provider: delivery.PaymentWechat, Purpose: delivery.PaymentWalletRecharge, Status: status, CancelRequestedAt: &now}}
		params, err := h.issuePrepay(ctx, &value, "recharge")
		if err != nil || params != nil || provider.prepayCalls != 0 {
			t.Fatalf("params=%+v err=%v", params, err)
		}
	}
}

func TestRechargeCancelEndpointRejectsMissingOwnershipAndInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		id            string
		repositoryErr error
		status        int
	}{
		{id: "invalid", status: http.StatusBadRequest},
		{id: "01K00000000000000000000000", repositoryErr: delivery.ErrPaymentForbidden, status: http.StatusNotFound},
	} {
		repo := &rechargeCancelRepositoryFake{err: tc.repositoryErr}
		funds, _ := delivery.NewFundsService(repo, repo, nil)
		provider := &terminalGuardProvider{}
		h := NewHandler(funds, provider, func(*gin.Context) uint64 { return 42 }, func(*gin.Context) string { return "payer" })
		r := gin.New()
		h.RegisterMerchant(r.Group("/delivery/merchant"))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/delivery/merchant/wallet/recharges/"+tc.id+"/close", nil))
		if w.Code != tc.status || provider.prepayCalls != 0 {
			t.Fatalf("status=%d body=%s", w.Code, w.Body)
		}
		if tc.id == "invalid" && repo.publicID != "" {
			t.Fatal("invalid ID reached storage")
		}
	}
}
