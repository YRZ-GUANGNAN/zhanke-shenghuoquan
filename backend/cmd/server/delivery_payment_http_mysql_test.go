package main

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/user"
)

const d3Fence = "d3-delivery-payment"

func TestD3DeliveryPaymentRealHTTPMySQLAuthAndGate(t *testing.T) {
	db := openD2ServerMySQL(t)
	assertD2ServerSchema(t, db)
	ensureD3ServerSchema(t, db)
	cleanupD3Server(t, db)
	t.Cleanup(func() { cleanupD3Server(t, db) })

	ctx := context.Background()
	auth := user.NewService(
		user.NewRepository(db), "d3-test-secret-that-is-longer-than-thirty-two-characters",
		"", "", true, user.WithAppEnv("dev"),
	)
	merchant, err := auth.WechatLogin(ctx, user.LoginInput{Code: d3Fence + "-merchant", Nickname: "D3 Merchant"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := auth.WechatLogin(ctx, user.LoginInput{Code: d3Fence + "-other", Nickname: "D3 Other"})
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := newDeliveryRuntime(config.Config{
		AppEnv: "dev", EnableDelivery: true, EnableDeliveryPayment: true,
		AllowMockDeliveryPayment: true,
	}, db)
	if err != nil {
		t.Fatal(err)
	}
	if runtime == nil || runtime.funds == nil || runtime.paymentProvider == nil ||
		runtime.allowMockConfirm {
		t.Fatal("D-3 runtime gate or independent mock isolation was not applied")
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerDeliveryRoutes(runtime, router.Group("/api/v1"), auth)
	server := httptest.NewServer(router)
	defer server.Close()

	status, _ := d2HTTPJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/v1/delivery/merchant/wallet", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("wallet without token status=%d", status)
	}
	rechargeID, _ := delivery.NewULID()
	status, body := d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/delivery/merchant/wallet/recharges", merchant.Token,
		map[string]any{"amountCents": 500, "clientRequestId": rechargeID})
	if status != http.StatusOK || !strings.Contains(string(body.Data), `"availableBalanceCents":500`) {
		t.Fatalf("mock recharge status=%d body=%s message=%s", status, body.Data, body.Message)
	}

	orderRequest, _ := delivery.NewULID()
	created := createD2HTTPOrder(t, server, merchant, orderRequest)
	idempotencyKey, _ := delivery.NewULID()
	status, body = d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/delivery/merchant/orders/"+created.DeliveryOrderPublicID+"/pay-with-wallet",
		merchant.Token, map[string]any{
			"expectedRowVersion": created.RowVersion, "idempotencyKey": idempotencyKey,
		})
	if status != http.StatusOK || !strings.Contains(string(body.Data), `"ready_for_claim"`) ||
		!strings.Contains(string(body.Data), `"availableBalanceCents":350`) ||
		!strings.Contains(string(body.Data), `"deliveryOrderPublicId"`) ||
		strings.Contains(string(body.Data), `"DeliveryOrderPublicID"`) {
		t.Fatalf("wallet payment status=%d body=%s message=%s", status, body.Data, body.Message)
	}
	status, _ = d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/delivery/merchant/orders/"+created.DeliveryOrderPublicID+"/pay-with-wallet",
		other.Token, map[string]any{
			"expectedRowVersion": created.RowVersion, "idempotencyKey": idempotencyKey,
		})
	if status != http.StatusNotFound {
		t.Fatalf("cross-merchant wallet payment status=%d", status)
	}
	status, _ = d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/delivery/payments/wechat/notify", "", map[string]any{})
	if status == http.StatusNotFound {
		t.Fatal("enabled delivery payment webhook route was not mounted")
	}

	disabled, err := newDeliveryRuntime(config.Config{
		AppEnv: "dev", EnableDelivery: true, EnableDeliveryPayment: false,
	}, db)
	if err != nil {
		t.Fatal(err)
	}
	if disabled == nil || disabled.funds != nil || disabled.paymentProvider != nil {
		t.Fatal("closed delivery payment gate constructed funds runtime")
	}
	disabledRouter := gin.New()
	registerDeliveryRoutes(disabled, disabledRouter.Group("/api/v1"), auth)
	disabledServer := httptest.NewServer(disabledRouter)
	defer disabledServer.Close()
	status, _ = d2HTTPJSON(t, disabledServer.Client(), http.MethodGet,
		disabledServer.URL+"/api/v1/delivery/merchant/wallet", merchant.Token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("disabled wallet route status=%d", status)
	}
	status, _ = d2HTTPJSON(t, disabledServer.Client(), http.MethodPost,
		disabledServer.URL+"/api/v1/delivery/payments/wechat/notify", "", map[string]any{})
	if status != http.StatusNotFound {
		t.Fatalf("disabled payment callback route status=%d", status)
	}
	t.Log("D-3 HTTP=PASS auth+owner_fence+mock_recharge+wallet_pay+gate_404+callback_gate")
}

func ensureD3ServerSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if db.Migrator().HasTable(&delivery.MerchantWallet{}) {
		return
	}
	file, err := os.Open(filepath.Clean("../../migrations/025_delivery_payment_wallet.sql"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var statement strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		statement.WriteString(line)
		statement.WriteByte('\n')
		if !strings.HasSuffix(strings.TrimSpace(line), ";") {
			continue
		}
		query := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(statement.String()), ";"))
		statement.Reset()
		if query != "" {
			if err := db.Exec(query).Error; err != nil {
				t.Fatalf("apply migration 025: %v", err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func cleanupD3Server(t *testing.T, db *gorm.DB) {
	t.Helper()
	var userIDs []uint
	if err := db.Unscoped().Model(&user.User{}).
		Where("open_id LIKE ?", "mock_openid_"+d3Fence+"%").Pluck("id", &userIDs).Error; err != nil {
		t.Errorf("list D-3 HTTP users: %v", err)
	}
	if len(userIDs) == 0 {
		return
	}
	var orderIDs []string
	if err := db.Model(&delivery.Order{}).Where("merchant_user_id IN ?", userIDs).
		Pluck("delivery_order_public_id", &orderIDs).Error; err != nil {
		t.Errorf("list D-3 HTTP orders: %v", err)
	}
	if len(orderIDs) > 0 {
		if err := db.Where("aggregate_public_id IN ?", orderIDs).Delete(&delivery.OutboxEvent{}).Error; err != nil {
			t.Errorf("cleanup D-3 HTTP outbox: %v", err)
		}
	}
	if db.Migrator().HasTable(&delivery.MerchantWallet{}) {
		if err := db.Where("out_trade_no LIKE ?", "D3%").Delete(&delivery.PaymentNotification{}).Error; err != nil {
			t.Errorf("cleanup D-3 HTTP notifications: %v", err)
		}
		for _, value := range []any{
			&delivery.Refund{}, &delivery.FeeSettlement{}, &delivery.PaymentAttempt{},
			&delivery.WalletLedger{}, &delivery.MerchantWallet{},
		} {
			if err := db.Where("merchant_user_id IN ?", userIDs).Delete(value).Error; err != nil {
				t.Errorf("cleanup D-3 HTTP %T: %v", value, err)
			}
		}
	}
	if len(orderIDs) > 0 {
		if err := db.Where("delivery_order_public_id IN ?", orderIDs).Delete(&delivery.Order{}).Error; err != nil {
			t.Errorf("cleanup D-3 HTTP orders: %v", err)
		}
	}
	if err := db.Unscoped().Where("id IN ?", userIDs).Delete(&user.User{}).Error; err != nil {
		t.Errorf("cleanup D-3 HTTP users: %v", err)
	}
}

func decodeD3Data(t *testing.T, body d2Envelope, target any) {
	t.Helper()
	if err := json.Unmarshal(body.Data, target); err != nil {
		t.Fatal(err)
	}
}
