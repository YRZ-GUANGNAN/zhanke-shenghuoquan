package deliveryconsole

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/delivery"
)

type fakeQueries struct {
	summary SummaryView
	order   delivery.Order
}

func (f fakeQueries) Summary(context.Context, time.Time) (SummaryView, error) { return f.summary, nil }
func (f fakeQueries) ListOrders(context.Context, ListFilter) ([]delivery.Order, int64, error) {
	return []delivery.Order{f.order}, 1, nil
}
func (f fakeQueries) Order(context.Context, string) (delivery.Order, error) { return f.order, nil }
func (f fakeQueries) OrderFunds(context.Context, string) (OrderFundsSnapshot, error) {
	return OrderFundsSnapshot{Order: f.order}, nil
}
func (f fakeQueries) ListMerchantFeeOverrides(context.Context, ListFilter) ([]delivery.MerchantFeeOverride, int64, error) {
	return nil, 0, nil
}
func (f fakeQueries) MerchantFeeOverride(context.Context, uint64) (delivery.MerchantFeeOverride, error) {
	return delivery.MerchantFeeOverride{}, delivery.ErrNotFound
}
func (f fakeQueries) DisableMerchantFeeOverride(context.Context, uint64, uint64, time.Time) (delivery.MerchantFeeOverride, error) {
	return delivery.MerchantFeeOverride{}, delivery.ErrConcurrentUpdate
}
func (f fakeQueries) ListMerchantWallets(context.Context, ListFilter) ([]delivery.MerchantWallet, int64, error) {
	return nil, 0, nil
}
func (f fakeQueries) MerchantWallet(context.Context, uint64) (delivery.MerchantWallet, error) {
	return delivery.MerchantWallet{}, delivery.ErrNotFound
}
func (f fakeQueries) ListMerchantLedger(context.Context, uint64, ListFilter) ([]delivery.WalletLedger, int64, error) {
	return nil, 0, nil
}
func (f fakeQueries) ListRiderWallets(context.Context, ListFilter) ([]delivery.RiderWallet, int64, error) {
	return nil, 0, nil
}
func (f fakeQueries) RiderWallet(context.Context, uint64) (delivery.RiderWallet, error) {
	return delivery.RiderWallet{}, delivery.ErrNotFound
}
func (f fakeQueries) ListRiderLedger(context.Context, uint64, ListFilter) ([]delivery.RiderLedger, int64, error) {
	return nil, 0, nil
}
func (f fakeQueries) ListRiderEarnings(context.Context, ListFilter) ([]delivery.RiderEarningRecord, int64, error) {
	return nil, 0, nil
}
func (f fakeQueries) RiderEarning(context.Context, string) (delivery.RiderEarningRecord, error) {
	return delivery.RiderEarningRecord{}, delivery.ErrNotFound
}
func (f fakeQueries) ListRiderPayouts(context.Context, ListFilter) ([]delivery.RiderPayoutRecord, int64, error) {
	return nil, 0, nil
}
func (f fakeQueries) ListRefunds(context.Context, ListFilter) ([]delivery.Refund, int64, error) {
	return nil, 0, nil
}
func (f fakeQueries) Refund(context.Context, string) (delivery.Refund, error) {
	return delivery.Refund{}, delivery.ErrNotFound
}

type fakeFees struct{}

func (fakeFees) FeePolicyForMerchant(context.Context, uint64) (delivery.FeePolicy, error) {
	return delivery.FeePolicy{}, nil
}
func (fakeFees) GlobalFeeConfig(context.Context) (delivery.DeliveryFeeConfig, error) {
	return delivery.DeliveryFeeConfig{ConfigScope: "global", DeliveryFeeCents: 150, PlatformCutCents: 50, Status: delivery.FeeConfigActive, RowVersion: 1}, nil
}
func (fakeFees) UpdateGlobalFeeConfig(context.Context, uint64, uint64, uint64, time.Time) (delivery.DeliveryFeeConfig, error) {
	return delivery.DeliveryFeeConfig{ConfigScope: "global", DeliveryFeeCents: 160, PlatformCutCents: 60, Status: delivery.FeeConfigActive, RowVersion: 2}, nil
}
func (fakeFees) UpsertMerchantFeeOverride(context.Context, uint64, uint64, uint64, *uint64, time.Time) (delivery.MerchantFeeOverride, error) {
	return delivery.MerchantFeeOverride{MerchantUserID: 123, DeliveryFeeCents: 200, PlatformCutCents: 80, Status: delivery.FeeConfigActive, RowVersion: 1}, nil
}

type fakeRiderWallets struct{}

func (fakeRiderWallets) RiderWallet(context.Context, uint64, time.Time) (delivery.RiderWallet, error) {
	return delivery.RiderWallet{}, nil
}
func (fakeRiderWallets) FindRiderEarningByOrder(context.Context, string) (delivery.RiderEarningRecord, error) {
	return delivery.RiderEarningRecord{}, nil
}
func (fakeRiderWallets) FindRiderEarning(context.Context, string) (delivery.RiderEarningRecord, error) {
	return delivery.RiderEarningRecord{}, nil
}
func (fakeRiderWallets) MarkRiderEarningPaidOffline(context.Context, string, uint64, uint64, string, string, time.Time) (delivery.RiderPayoutResult, error) {
	return delivery.RiderPayoutResult{}, nil
}

func TestDeliveryConsoleHandlerContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	order := delivery.Order{
		DeliveryOrderPublicID: "01J0000000000000000000D6B1",
		MerchantUserID:        10,
		DeliveryFeeCents:      150,
		RiderFeeCents:         100,
		PlatformCutCents:      50,
		Status:                delivery.StatusPendingFeePayment,
		RowVersion:            1,
	}
	NewHandler(
		fakeQueries{summary: SummaryView{Orders: 1}, order: order},
		fakeFees{}, fakeRiderWallets{}, func(*gin.Context) uint64 { return 99 },
		func() time.Time { return time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC) },
	).Register(router.Group("/api/v1/admin/delivery"))

	for _, check := range []struct {
		method string
		path   string
		body   string
		want   int
	}{
		{http.MethodGet, "/api/v1/admin/delivery/summary", "", http.StatusOK},
		{http.MethodGet, "/api/v1/admin/delivery/orders?page=1&pageSize=20", "", http.StatusOK},
		{http.MethodGet, "/api/v1/admin/delivery/orders/" + order.DeliveryOrderPublicID, "", http.StatusOK},
		{http.MethodGet, "/api/v1/admin/delivery/orders/" + order.DeliveryOrderPublicID + "/funds", "", http.StatusOK},
		{http.MethodGet, "/api/v1/admin/delivery/fee-config", "", http.StatusOK},
		{http.MethodPut, "/api/v1/admin/delivery/fee-config", `{"deliveryFeeCents":160,"platformCutCents":60,"expectedRowVersion":1}`, http.StatusOK},
		{http.MethodPut, "/api/v1/admin/delivery/merchant-fee-overrides/123", `{"deliveryFeeCents":200,"platformCutCents":80}`, http.StatusOK},
		{http.MethodGet, "/api/v1/admin/delivery/orders/not-a-ulid", "", http.StatusBadRequest},
		{http.MethodGet, "/api/v1/admin/delivery/orders?page=0", "", http.StatusBadRequest},
		{http.MethodPatch, "/api/v1/admin/delivery/merchant-fee-overrides/123/disable", `{"expectedRowVersion":1}`, http.StatusConflict},
	} {
		request := httptest.NewRequest(check.method, check.path, strings.NewReader(check.body))
		if check.body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != check.want {
			t.Fatalf("%s %s status=%d want=%d body=%s", check.method, check.path, recorder.Code, check.want, recorder.Body.String())
		}
	}
}

func TestDeliveryConsoleWriteErrorMapping(t *testing.T) {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	handler := &Handler{}
	handler.write(context, nil, delivery.ErrConcurrentUpdate)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d", recorder.Code)
	}
	recorder = httptest.NewRecorder()
	context, _ = gin.CreateTestContext(recorder)
	handler.write(context, nil, delivery.ErrInvalidRecord)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("bad request status=%d", recorder.Code)
	}
	recorder = httptest.NewRecorder()
	context, _ = gin.CreateTestContext(recorder)
	handler.write(context, nil, errors.New("boom"))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("internal status=%d", recorder.Code)
	}
}

func decodeEnvelope(t *testing.T, body string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		t.Fatal(err)
	}
	return value
}
