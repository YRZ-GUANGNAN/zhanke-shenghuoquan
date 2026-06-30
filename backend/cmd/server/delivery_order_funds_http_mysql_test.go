package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/admin/deliverypayment"
	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/user"
)

func TestD3OrderFundsReadOnlyRealHTTPMySQL(t *testing.T) {
	db := openD2ServerMySQL(t)
	assertD2ServerSchema(t, db)
	ensureD3ServerSchema(t, db)
	cleanupD3Server(t, db)

	ctx := context.Background()
	auth := user.NewService(
		user.NewRepository(db), "d3-readonly-secret-that-is-longer-than-thirty-two-characters",
		"", "", true, user.WithAppEnv("dev"),
	)
	merchant, err := auth.WechatLogin(ctx, user.LoginInput{
		Code: d3Fence + "-readonly-merchant", Nickname: "D3 Readonly Merchant",
	})
	if err != nil {
		t.Fatal(err)
	}
	other, err := auth.WechatLogin(ctx, user.LoginInput{
		Code: d3Fence + "-readonly-other", Nickname: "D3 Readonly Other",
	})
	if err != nil {
		t.Fatal(err)
	}
	merchantUserID := uint64(merchant.User.ID)
	otherUserID := uint64(other.User.ID)
	wechatTransactionID := "wx-d3-readonly-funds"
	t.Cleanup(func() {
		cleanupD3Server(t, db)
		assertD3ReadonlyResidueZero(t, db, merchantUserID, otherUserID, wechatTransactionID)
	})

	runtime, err := newDeliveryRuntime(config.Config{
		AppEnv: "dev", EnableDelivery: true, EnableDeliveryPayment: true,
		AllowMockDeliveryPayment: true,
	}, db)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerDeliveryRoutes(runtime, router.Group("/api/v1"), auth)
	server := httptest.NewServer(router)
	defer server.Close()

	unpaidRequestID, _ := delivery.NewULID()
	unpaid := createD2HTTPOrder(t, server, merchant, unpaidRequestID)
	fundsURL := server.URL + "/api/v1/delivery/merchant/orders/" +
		unpaid.DeliveryOrderPublicID + "/funds"
	status, _ := d2HTTPJSON(t, server.Client(), http.MethodGet, fundsURL, "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("funds without token status=%d", status)
	}
	status, body := d2HTTPJSON(t, server.Client(), http.MethodGet, fundsURL, merchant.Token, nil)
	unpaidFunds := decodeD3OrderFunds(t, body)
	if status != http.StatusOK || unpaidFunds.DeliveryOrderPublicID != unpaid.DeliveryOrderPublicID ||
		unpaidFunds.Settlement != nil || unpaidFunds.Refund != nil || unpaidFunds.Payment != nil {
		t.Fatalf("unpaid funds status=%d result=%#v body=%s", status, unpaidFunds, body.Data)
	}
	rawUnpaid := string(body.Data)
	for _, expected := range []string{`"settlement":null`, `"refund":null`, `"payment":null`} {
		if !strings.Contains(rawUnpaid, expected) {
			t.Fatalf("unpaid funds omitted explicit empty state %s: %s", expected, rawUnpaid)
		}
	}
	status, _ = d2HTTPJSON(t, server.Client(), http.MethodGet, fundsURL, other.Token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("cross-merchant funds status=%d", status)
	}
	missingOrderID, _ := delivery.NewULID()
	status, _ = d2HTTPJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/v1/delivery/merchant/orders/"+missingOrderID+"/funds",
		merchant.Token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("missing order funds status=%d", status)
	}

	rechargeID, _ := delivery.NewULID()
	status, body = d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/delivery/merchant/wallet/recharges", merchant.Token,
		map[string]any{"amountCents": 500, "clientRequestId": rechargeID})
	if status != http.StatusOK {
		t.Fatalf("mock recharge status=%d body=%s message=%s", status, body.Data, body.Message)
	}
	walletOrderRequestID, _ := delivery.NewULID()
	walletOrder := createD2HTTPOrder(t, server, merchant, walletOrderRequestID)
	walletPaymentKey, _ := delivery.NewULID()
	status, body = d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/delivery/merchant/orders/"+walletOrder.DeliveryOrderPublicID+"/pay-with-wallet",
		merchant.Token, map[string]any{
			"expectedRowVersion": walletOrder.RowVersion,
			"idempotencyKey":     walletPaymentKey,
		})
	if status != http.StatusOK {
		t.Fatalf("wallet payment status=%d body=%s message=%s", status, body.Data, body.Message)
	}
	var paidWithWallet deliverypayment.FundsResponse
	decodeD3Data(t, body, &paidWithWallet)
	if paidWithWallet.Order == nil {
		t.Fatal("wallet payment response omitted order")
	}

	walletFundsURL := server.URL + "/api/v1/delivery/merchant/orders/" +
		walletOrder.DeliveryOrderPublicID + "/funds"
	status, body = d2HTTPJSON(t, server.Client(), http.MethodGet, walletFundsURL, merchant.Token, nil)
	walletFunds := decodeD3OrderFunds(t, body)
	if status != http.StatusOK || walletFunds.Settlement == nil ||
		walletFunds.Settlement.Status != delivery.SettlementPaid ||
		walletFunds.Settlement.Source != delivery.SettlementWallet ||
		walletFunds.Settlement.AmountCents != 150 ||
		walletFunds.Refund != nil || walletFunds.Payment != nil {
		t.Fatalf("wallet funds status=%d result=%#v body=%s", status, walletFunds, body.Data)
	}
	before := snapshotD3FundsReadState(t, db, merchantUserID, walletOrder.DeliveryOrderPublicID)
	for range 3 {
		status, _ = d2HTTPJSON(t, server.Client(), http.MethodGet, walletFundsURL, merchant.Token, nil)
		if status != http.StatusOK {
			t.Fatalf("repeated funds status=%d", status)
		}
	}
	after := snapshotD3FundsReadState(t, db, merchantUserID, walletOrder.DeliveryOrderPublicID)
	if before != after {
		t.Fatalf("GET funds mutated data\nbefore=%s\nafter=%s", before, after)
	}

	walletRefundKey, _ := delivery.NewULID()
	status, body = d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/delivery/merchant/orders/"+walletOrder.DeliveryOrderPublicID+"/refunds",
		merchant.Token, map[string]any{
			"expectedRowVersion": paidWithWallet.Order.RowVersion,
			"idempotencyKey":     walletRefundKey,
		})
	if status != http.StatusOK {
		t.Fatalf("wallet refund status=%d body=%s message=%s", status, body.Data, body.Message)
	}
	status, body = d2HTTPJSON(t, server.Client(), http.MethodGet, walletFundsURL, merchant.Token, nil)
	refundedWalletFunds := decodeD3OrderFunds(t, body)
	if status != http.StatusOK || refundedWalletFunds.Settlement == nil ||
		refundedWalletFunds.Settlement.Status != delivery.SettlementRefunded ||
		refundedWalletFunds.Refund == nil ||
		refundedWalletFunds.Refund.Status != delivery.RefundSucceeded ||
		refundedWalletFunds.Refund.Source != delivery.SettlementWallet ||
		refundedWalletFunds.Refund.AmountCents != 150 {
		t.Fatalf("refunded wallet funds status=%d result=%#v body=%s", status, refundedWalletFunds, body.Data)
	}

	wechatOrderRequestID, _ := delivery.NewULID()
	wechatOrder := createD2HTTPOrder(t, server, merchant, wechatOrderRequestID)
	wechatPaymentRequestID, _ := delivery.NewULID()
	started, err := runtime.funds.StartOrderPayment(
		ctx, merchantUserID, wechatOrder.DeliveryOrderPublicID,
		wechatOrder.RowVersion, wechatPaymentRequestID, delivery.PaymentWechat,
	)
	if err != nil {
		t.Fatal(err)
	}
	confirmed, _, err := runtime.funds.ConfirmWechatPayment(ctx, delivery.PaymentConfirmation{
		OutTradeNo: started.Attempt.OutTradeNo, WechatTransactionID: wechatTransactionID,
		AmountCents: started.Attempt.AmountCents, RawBody: []byte(`{"d3":"readonly"}`),
		PaidAt: time.Now().UTC(),
	})
	if err != nil || confirmed.Order == nil {
		t.Fatalf("confirm wechat fixture result=%#v err=%v", confirmed, err)
	}
	wechatRefundKey, _ := delivery.NewULID()
	status, body = d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/delivery/merchant/orders/"+wechatOrder.DeliveryOrderPublicID+"/refunds",
		merchant.Token, map[string]any{
			"expectedRowVersion": confirmed.Order.RowVersion,
			"idempotencyKey":     wechatRefundKey,
		})
	if status != http.StatusOK {
		t.Fatalf("wechat pending refund status=%d body=%s message=%s", status, body.Data, body.Message)
	}
	wechatFundsURL := server.URL + "/api/v1/delivery/merchant/orders/" +
		wechatOrder.DeliveryOrderPublicID + "/funds"
	status, body = d2HTTPJSON(t, server.Client(), http.MethodGet, wechatFundsURL, merchant.Token, nil)
	wechatFunds := decodeD3OrderFunds(t, body)
	if status != http.StatusOK || wechatFunds.Settlement == nil ||
		wechatFunds.Settlement.Status != delivery.SettlementRefundPending ||
		wechatFunds.Refund == nil ||
		wechatFunds.Refund.Status != delivery.RefundRetryPending ||
		wechatFunds.Payment == nil ||
		wechatFunds.Payment.Provider != delivery.PaymentWechat ||
		wechatFunds.Payment.Status != delivery.PaymentPaid {
		t.Fatalf("wechat pending funds status=%d result=%#v body=%s", status, wechatFunds, body.Data)
	}

	disabled, err := newDeliveryRuntime(config.Config{
		AppEnv: "dev", EnableDelivery: true, EnableDeliveryPayment: false,
	}, db)
	if err != nil {
		t.Fatal(err)
	}
	disabledRouter := gin.New()
	registerDeliveryRoutes(disabled, disabledRouter.Group("/api/v1"), auth)
	disabledServer := httptest.NewServer(disabledRouter)
	defer disabledServer.Close()
	status, _ = d2HTTPJSON(t, disabledServer.Client(), http.MethodGet,
		disabledServer.URL+"/api/v1/delivery/merchant/orders/"+unpaid.DeliveryOrderPublicID+"/funds",
		merchant.Token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("disabled funds route status=%d", status)
	}
	t.Log("D-3 funds readonly=PASS unpaid_null+wallet_paid+wallet_refunded+wechat_refund_pending+owner_404+auth_401+gate_404+no_mutation")
}

func decodeD3OrderFunds(t *testing.T, body d2Envelope) deliverypayment.OrderFundsView {
	t.Helper()
	var value deliverypayment.OrderFundsView
	decodeD3Data(t, body, &value)
	return value
}

func snapshotD3FundsReadState(
	t *testing.T,
	db *gorm.DB,
	merchantUserID uint64,
	orderPublicID string,
) string {
	t.Helper()
	var order delivery.Order
	if err := db.Where(
		"delivery_order_public_id = ? AND merchant_user_id = ?",
		orderPublicID,
		merchantUserID,
	).First(&order).Error; err != nil {
		t.Fatal(err)
	}
	var settlements []delivery.FeeSettlement
	if err := db.Where(
		"delivery_order_public_id = ? AND merchant_user_id = ?",
		orderPublicID,
		merchantUserID,
	).Order("id").Find(&settlements).Error; err != nil {
		t.Fatal(err)
	}
	var refunds []delivery.Refund
	if err := db.Where(
		"delivery_order_public_id = ? AND merchant_user_id = ?",
		orderPublicID,
		merchantUserID,
	).Order("id").Find(&refunds).Error; err != nil {
		t.Fatal(err)
	}
	var payments []delivery.PaymentAttempt
	if err := db.Where(
		"delivery_order_public_id = ? AND merchant_user_id = ?",
		orderPublicID,
		merchantUserID,
	).Order("id").Find(&payments).Error; err != nil {
		t.Fatal(err)
	}
	var wallets []delivery.MerchantWallet
	if err := db.Where("merchant_user_id = ?", merchantUserID).Order("id").Find(&wallets).Error; err != nil {
		t.Fatal(err)
	}
	var ledger []delivery.WalletLedger
	if err := db.Where("merchant_user_id = ?", merchantUserID).Order("id").Find(&ledger).Error; err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(struct {
		Order       delivery.Order
		Settlements []delivery.FeeSettlement
		Refunds     []delivery.Refund
		Payments    []delivery.PaymentAttempt
		Wallets     []delivery.MerchantWallet
		Ledger      []delivery.WalletLedger
	}{order, settlements, refunds, payments, wallets, ledger})
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func assertD3ReadonlyResidueZero(
	t *testing.T,
	db *gorm.DB,
	merchantUserID, otherUserID uint64,
	wechatTransactionID string,
) {
	t.Helper()
	userIDs := []uint64{merchantUserID, otherUserID}
	for _, value := range []any{
		&delivery.Refund{}, &delivery.FeeSettlement{}, &delivery.PaymentAttempt{},
		&delivery.WalletLedger{}, &delivery.MerchantWallet{}, &delivery.Order{},
	} {
		var count int64
		if err := db.Model(value).Where("merchant_user_id IN ?", userIDs).Count(&count).Error; err != nil {
			t.Errorf("count residue %T: %v", value, err)
		} else if count != 0 {
			t.Errorf("residue %T count=%d", value, count)
		}
	}
	var notificationCount int64
	if err := db.Model(&delivery.PaymentNotification{}).
		Where("wechat_transaction_id = ?", wechatTransactionID).
		Count(&notificationCount).Error; err != nil {
		t.Errorf("count notification residue: %v", err)
	} else if notificationCount != 0 {
		t.Errorf("notification residue count=%d", notificationCount)
	}
}
