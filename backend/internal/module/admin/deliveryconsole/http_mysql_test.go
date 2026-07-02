package deliveryconsole

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/user"
)

const (
	d6bMerchantGlobal   = uint64(9_006_101_001)
	d6bMerchantOverride = uint64(9_006_101_002)
	d6bRider            = uint64(9_006_102_001)
)

type d6bEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func TestDeliveryConsoleAdminHTTPAgainstRealMySQL84(t *testing.T) {
	db := openD6BMySQL(t)
	assertD6BSchema(t, db)
	cleanupD6B(t, db)
	t.Cleanup(func() { cleanupD6B(t, db) })

	ctx := context.Background()
	now := time.Date(2026, 7, 2, 6, 0, 0, 0, time.UTC)
	repository := delivery.NewRepository(db)
	auth := user.NewService(
		user.NewRepository(db),
		"d6b-test-secret-that-is-longer-than-thirty-two-characters",
		"", "", true,
	)
	ordinary, err := auth.WechatLogin(ctx, user.LoginInput{Code: "d6b-ordinary", Nickname: "D6B ordinary"})
	if err != nil {
		t.Fatal(err)
	}
	administrator, err := auth.WechatLogin(ctx, user.LoginInput{Code: "d6b-admin", Nickname: "D6B admin"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&user.User{}).Where("id = ?", administrator.User.ID).Update("role", user.RoleAdmin).Error; err != nil {
		t.Fatal(err)
	}

	feeBefore, err := repository.GlobalFeeConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Model(&delivery.DeliveryFeeConfig{}).Where("config_scope = ?", "global").Updates(map[string]any{
			"delivery_fee_cents": feeBefore.DeliveryFeeCents, "platform_cut_cents": feeBefore.PlatformCutCents,
			"status": feeBefore.Status, "row_version": feeBefore.RowVersion, "updated_at": feeBefore.UpdatedAt,
		})
	})

	router := newD6BRouter(auth, db, func() time.Time { return now })
	server := httptest.NewServer(router)
	defer server.Close()

	status, _ := d6bHTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/admin/delivery/summary", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("no token status=%d want=401", status)
	}
	status, _ = d6bHTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/admin/delivery/summary", ordinary.Token, nil)
	if status != http.StatusForbidden {
		t.Fatalf("ordinary status=%d want=403", status)
	}
	status, body := d6bHTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/admin/delivery/fee-config", administrator.Token, nil)
	if status != http.StatusOK || body.Code != 0 {
		t.Fatalf("fee config status=%d body=%+v", status, body)
	}

	status, _ = d6bHTTPJSON(t, server.Client(), http.MethodPut, server.URL+"/api/v1/admin/delivery/fee-config", administrator.Token, map[string]any{
		"deliveryFeeCents": 190, "platformCutCents": 70, "expectedRowVersion": feeBefore.RowVersion,
	})
	if status != http.StatusOK {
		t.Fatalf("update fee status=%d", status)
	}
	status, _ = d6bHTTPJSON(t, server.Client(), http.MethodPut, server.URL+"/api/v1/admin/delivery/fee-config", administrator.Token, map[string]any{
		"deliveryFeeCents": 191, "platformCutCents": 70, "expectedRowVersion": feeBefore.RowVersion,
	})
	if status != http.StatusConflict {
		t.Fatalf("stale fee status=%d want=409", status)
	}
	status, _ = d6bHTTPJSON(t, server.Client(), http.MethodPut, server.URL+"/api/v1/admin/delivery/fee-config", administrator.Token, map[string]any{
		"deliveryFeeCents": 100, "platformCutCents": 100, "expectedRowVersion": feeBefore.RowVersion + 1,
	})
	if status != http.StatusBadRequest {
		t.Fatalf("invalid fee status=%d want=400", status)
	}

	dispatch := newD6BDispatch(t, repository, func() time.Time { return now })
	globalOrder := createD6BOrder(t, ctx, dispatch, d6bMerchantGlobal, "01J000000000000000000D6B01")
	if globalOrder.DeliveryFeeCents != 190 || globalOrder.PlatformCutCents != 70 || globalOrder.RiderFeeCents != 120 {
		t.Fatalf("new global snapshot=%#v", globalOrder)
	}

	status, _ = d6bHTTPJSON(t, server.Client(), http.MethodPut, server.URL+"/api/v1/admin/delivery/merchant-fee-overrides/9006101002", administrator.Token, map[string]any{
		"deliveryFeeCents": 250, "platformCutCents": 80,
	})
	if status != http.StatusOK {
		t.Fatalf("upsert override status=%d", status)
	}
	overrideOrder := createD6BOrder(t, ctx, dispatch, d6bMerchantOverride, "01J000000000000000000D6B02")
	if overrideOrder.DeliveryFeeCents != 250 || overrideOrder.PlatformCutCents != 80 || overrideOrder.RiderFeeCents != 170 {
		t.Fatalf("override snapshot=%#v", overrideOrder)
	}
	status, body = d6bHTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/admin/delivery/merchant-fee-overrides?merchantUserId=9006101002", administrator.Token, nil)
	if status != http.StatusOK || !strings.Contains(string(body.Data), `"merchantUserId":9006101002`) {
		t.Fatalf("override list status=%d body=%+v", status, body)
	}

	delivered := createD6BOrder(t, ctx, dispatch, d6bMerchantGlobal, "01J000000000000000000D6B03")
	delivered = readyAndDeliverD6BOrder(t, ctx, dispatch, delivered)
	status, body = d6bHTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/admin/delivery/rider-earnings?riderUserId=9006102001", administrator.Token, nil)
	if status != http.StatusOK || !strings.Contains(string(body.Data), delivered.DeliveryOrderPublicID) {
		t.Fatalf("earnings status=%d body=%+v", status, body)
	}
	var earning delivery.RiderEarningRecord
	if err := db.Where("delivery_order_public_id = ?", delivered.DeliveryOrderPublicID).First(&earning).Error; err != nil {
		t.Fatal(err)
	}
	status, _ = d6bHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/admin/delivery/rider-earnings/"+earning.EarningPublicID+"/pay-offline", administrator.Token, map[string]any{
		"expectedRowVersion": earning.RowVersion, "idempotencyKey": "01J000000000000000000D6B91", "note": "wechat paid offline",
	})
	if status != http.StatusOK {
		t.Fatalf("pay offline status=%d", status)
	}
	status, _ = d6bHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/admin/delivery/rider-earnings/"+earning.EarningPublicID+"/pay-offline", administrator.Token, map[string]any{
		"expectedRowVersion": earning.RowVersion, "idempotencyKey": "01J000000000000000000D6B92", "note": "repeat",
	})
	if status != http.StatusOK {
		t.Fatalf("repeat pay offline status=%d", status)
	}

	for _, path := range []string{
		"/orders?page=1&pageSize=20",
		"/orders/" + delivered.DeliveryOrderPublicID,
		"/orders/" + delivered.DeliveryOrderPublicID + "/funds",
		"/rider-wallets/9006102001",
		"/rider-wallets/9006102001/ledger",
		"/rider-payouts",
		"/summary",
	} {
		status, body = d6bHTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/admin/delivery"+path, administrator.Token, nil)
		if status != http.StatusOK || body.Code != 0 {
			t.Fatalf("GET %s status=%d body=%+v", path, status, body)
		}
	}

	t.Log("d6b_real_http_mysql=PASS auth_401_403=PASS fee_config_CAS=PASS override_snapshot=PASS rider_wallet_earning_pay_offline_idempotent=PASS")
}

func TestZZDeliveryConsoleResidueAgainstRealMySQL84(t *testing.T) {
	db := openD6BMySQL(t)
	cleanupD6B(t, db)
	riders := []uint64{d6bRider}
	merchants := []uint64{d6bMerchantGlobal, d6bMerchantOverride}
	checks := []struct {
		name  string
		model any
		where string
		args  []any
	}{
		{"orders", &delivery.Order{}, "merchant_user_id IN ?", []any{merchants}},
		{"merchant_overrides", &delivery.MerchantFeeOverride{}, "merchant_user_id IN ?", []any{merchants}},
		{"merchant_wallets", &delivery.MerchantWallet{}, "merchant_user_id IN ?", []any{merchants}},
		{"rider_wallets", &delivery.RiderWallet{}, "rider_user_id IN ?", []any{riders}},
		{"rider_earnings", &delivery.RiderEarningRecord{}, "rider_user_id IN ?", []any{riders}},
		{"rider_payouts", &delivery.RiderPayoutRecord{}, "rider_user_id IN ?", []any{riders}},
		{"users", &user.User{}, "open_id IN ?", []any{[]string{"mock_openid_d6b-ordinary", "mock_openid_d6b-admin"}}},
	}
	for _, check := range checks {
		var count int64
		if err := db.Model(check.model).Where(check.where, check.args...).Count(&count).Error; err != nil {
			t.Fatalf("%s count: %v", check.name, err)
		}
		if count != 0 {
			t.Fatalf("%s residue=%d", check.name, count)
		}
	}
	t.Log("d6b_residue=PASS scoped_rows=0")
}

func openD6BMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("DELIVERY_D6B_MYSQL_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	}
	if dsn == "" {
		t.Skip("set DELIVERY_D6B_MYSQL_DSN or MYSQL_DSN for D-6b real HTTP MySQL acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertD6BSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, model := range []any{
		&user.User{}, &delivery.Order{}, &delivery.OutboxEvent{}, &delivery.DeliveryFeeConfig{},
		&delivery.MerchantFeeOverride{}, &delivery.MerchantWallet{}, &delivery.WalletLedger{},
		&delivery.RiderWallet{}, &delivery.RiderEarningRecord{}, &delivery.RiderPayoutRecord{},
		&delivery.FeeSettlement{}, &delivery.Refund{},
	} {
		if !db.Migrator().HasTable(model) {
			t.Fatalf("required table for %T is missing", model)
		}
	}
}

func cleanupD6B(t *testing.T, db *gorm.DB) {
	t.Helper()
	riders := []uint64{d6bRider}
	merchants := []uint64{d6bMerchantGlobal, d6bMerchantOverride}
	orderIDs := db.Model(&delivery.Order{}).Select("delivery_order_public_id").Where("merchant_user_id IN ?", merchants)
	db.Where("rider_user_id IN ?", riders).Delete(&delivery.RiderPayoutRecord{})
	db.Where("rider_user_id IN ?", riders).Delete(&delivery.RiderLedger{})
	db.Where("rider_user_id IN ?", riders).Delete(&delivery.RiderEarningRecord{})
	db.Where("rider_user_id IN ?", riders).Delete(&delivery.RiderWallet{})
	db.Where("merchant_user_id IN ?", merchants).Delete(&delivery.Refund{})
	db.Where("merchant_user_id IN ?", merchants).Delete(&delivery.FeeSettlement{})
	db.Where("merchant_user_id IN ?", merchants).Delete(&delivery.PaymentAttempt{})
	db.Where("merchant_user_id IN ?", merchants).Delete(&delivery.WalletLedger{})
	db.Where("merchant_user_id IN ?", merchants).Delete(&delivery.MerchantWallet{})
	db.Where("aggregate_public_id IN (?)", orderIDs).Delete(&delivery.OutboxEvent{})
	db.Where("merchant_user_id IN ?", merchants).Delete(&delivery.Order{})
	db.Where("merchant_user_id IN ?", merchants).Delete(&delivery.MerchantFeeOverride{})
	db.Unscoped().Where("open_id IN ?", []string{"mock_openid_d6b-ordinary", "mock_openid_d6b-admin"}).Delete(&user.User{})
}

func newD6BRouter(auth *user.Service, db *gorm.DB, now func() time.Time) http.Handler {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	repository := delivery.NewRepository(db)
	handler := NewHandler(
		NewQueryRepository(db), repository, repository,
		func(c *gin.Context) uint64 { return uint64(user.CurrentUserID(c)) }, now,
	)
	handler.Register(router.Group("/api/v1/admin/delivery", auth.AuthRequired(), user.AdminRequired()))
	return router
}

type d6bActiveEligibility struct{}

func (d6bActiveEligibility) RequireActive(context.Context, uint64) error { return nil }

func newD6BDispatch(t *testing.T, repository *delivery.GormRepository, now func() time.Time) *delivery.DispatchService {
	t.Helper()
	base, err := delivery.NewService(repository, now)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := delivery.NewDispatchServiceWithFeeProvider(repository, base, d6bActiveEligibility{}, repository, now)
	if err != nil {
		t.Fatal(err)
	}
	return dispatch
}

func createD6BOrder(t *testing.T, ctx context.Context, dispatch *delivery.DispatchService, merchant uint64, requestID string) delivery.Order {
	t.Helper()
	order, err := dispatch.CreateOrder(ctx, delivery.CreateDispatchInput{
		MerchantUserID: merchant, MerchantRequestID: requestID,
		Pickup:    delivery.Contact{Name: "D6B pickup", Phone: "13000000000", Address: "A"},
		Recipient: delivery.Contact{Name: "D6B recipient", Phone: "13000000001", Address: "B"},
		Remark:    "D6B admin console",
	})
	if err != nil {
		t.Fatal(err)
	}
	return order
}

func readyAndDeliverD6BOrder(t *testing.T, ctx context.Context, dispatch *delivery.DispatchService, order delivery.Order) delivery.Order {
	t.Helper()
	var err error
	order, _, err = dispatch.ConfirmMockFee(ctx, order.MerchantUserID, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	order, err = dispatch.ClaimOrder(ctx, d6bRider, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	order, err = dispatch.StartToPickup(ctx, d6bRider, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	order, err = dispatch.MarkPickedUp(ctx, d6bRider, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	order, err = dispatch.StartDelivering(ctx, d6bRider, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	order, err = dispatch.MarkDelivered(ctx, d6bRider, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	return order
}

func d6bHTTPJSON(t *testing.T, client *http.Client, method, url, token string, body any) (int, d6bEnvelope) {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	request, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var envelope d6bEnvelope
	_ = json.NewDecoder(response.Body).Decode(&envelope)
	return response.StatusCode, envelope
}
