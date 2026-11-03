package deliveryrider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/takeout"
	"github.com/gin-gonic/gin"
)

func TestWalletWithdrawalCapabilityMatchesServerWriteGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, enabled := range []bool{false, true} {
		handler := NewHandler(nil, nil, nil, func(*gin.Context) uint64 { return 77 }).WithWalletReader(walletReaderStub{}).WithWithdrawals(&withdrawalRepositoryStub{})
		// Test-only opt-in; production still has no setter for the write gate.
		handler.withdrawalWritesReady = enabled
		handler.identity = todayStatsIdentityStub{rider: takeout.Rider{RiderUserID: 77, Status: takeout.RiderActive}}
		router := gin.New()
		handler.Register(router.Group("/rider"))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/rider/wallet", nil))
		var response struct {
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		value, exists := response.Data["withdrawalEnabled"]
		if recorder.Code != http.StatusOK || !exists || value != enabled {
			t.Fatalf("gate=%t capability=%v exists=%t body=%s", enabled, value, exists, recorder.Body.String())
		}
	}
}

type walletReaderStub struct{}

func (walletReaderStub) ReadRiderWalletSnapshot(context.Context, uint64) (delivery.RiderWalletSnapshot, error) {
	now := time.Date(2026, 10, 25, 1, 2, 3, 0, time.UTC)
	return delivery.RiderWalletSnapshot{
		WalletState: delivery.RiderWalletReady, WalletPublicID: "01J0000000000000000000D6B1",
		AvailableBalanceCents: 1200, ReservedCents: 200, UnverifiedCents: 300, UpdatedAt: &now,
	}, nil
}

func (walletReaderStub) ListRiderEarningsByRider(context.Context, uint64, delivery.Page) ([]delivery.RiderEarningRecord, int64, error) {
	return []delivery.RiderEarningRecord{{EarningPublicID: "01J0000000000000000000D6B2", DeliveryOrderPublicID: "01J0000000000000000000D6B3", RiderFeeCents: 100, SettlementStatus: delivery.RiderEarningPending}}, 1, nil
}

func (walletReaderStub) ListRiderLedgerByRider(context.Context, uint64, delivery.Page) ([]delivery.RiderLedger, int64, error) {
	return []delivery.RiderLedger{{LedgerPublicID: "01J0000000000000000000D6B4", Direction: delivery.RiderLedgerCredit, EntryType: delivery.RiderLedgerIncome, AmountCents: 100, BalanceAfterCents: 1200}}, 1, nil
}

func (walletReaderStub) ListRiderPayoutQueueByRider(context.Context, uint64, delivery.Page) ([]delivery.RiderPayoutQueueReadItem, int64, error) {
	return []delivery.RiderPayoutQueueReadItem{{PayoutQueuePublicID: "01J0000000000000000000D6B5", EarningPublicID: "01J0000000000000000000D6B2", DeliveryOrderPublicID: "01J0000000000000000000D6B3", AmountCents: 100, Status: "queued"}}, 1, nil
}

func TestWalletQueriesUseSessionRiderAndWhitelistDTO(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil, nil, func(*gin.Context) uint64 { return 77 }).WithWalletReader(walletReaderStub{})
	handler.identity = todayStatsIdentityStub{rider: takeout.Rider{RiderUserID: 77, Status: takeout.RiderActive}}
	router := gin.New()
	handler.Register(router.Group("/rider"))
	for _, path := range []string{"/rider/wallet", "/rider/wallet/earnings?page=1&pageSize=20", "/rider/wallet/ledger?page=1&pageSize=20", "/rider/wallet/payout-queue?page=1&pageSize=20"} {
		recorder := httptest.NewRecorder()
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path+separator+"riderUserId=999", nil))
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
		body := recorder.Body.String()
		for _, forbidden := range []string{"operatorUserId", "idempotencyKey", "sourceEvidenceHash", "openid", "providerPayload"} {
			if strings.Contains(strings.ToLower(body), strings.ToLower(forbidden)) {
				t.Fatalf("%s leaked %s: %s", path, forbidden, body)
			}
		}
	}
}
