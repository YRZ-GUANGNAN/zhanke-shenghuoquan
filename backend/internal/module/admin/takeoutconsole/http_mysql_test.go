package takeoutconsole

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/user"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const g6HTTPRiderID = uint64(960010)

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func TestTakeoutAdminRealHTTPAuthQueriesCASAndWithdrawAgainstMySQL84(t *testing.T) {
	db := openG6A1HTTPMySQL(t)
	assertG6A1HTTPSchema(t, db)
	cleanupG6A1HTTP(t, db)
	t.Cleanup(func() { cleanupG6A1HTTP(t, db) })

	ctx := context.Background()
	now := time.Date(2026, 6, 22, 22, 0, 0, 0, time.UTC)
	repository := takeout.NewRepository(db)
	merchants, _ := takeout.NewMerchantService(repository, func() time.Time { return now })
	catalog, _ := takeout.NewCatalogService(repository, func() time.Time { return now })
	orders, _ := takeout.NewOrderService(repository, repository, func() time.Time { return now })
	orderOps, _ := takeout.NewRiderOrderOps(repository, func() time.Time { return now })
	riders, _ := takeout.NewRiderService(repository, orderOps, func() time.Time { return now })
	queries, _ := takeout.NewAdminQueryService(repository)
	withdraws, _ := takeout.NewWithdrawService(repository, func() time.Time { return now })

	auth := user.NewService(user.NewRepository(db), "g6a1-test-secret-that-is-longer-than-thirty-two-characters", "", "", true)
	ordinary, err := auth.WechatLogin(ctx, user.LoginInput{Code: "g6a1-ordinary", Nickname: "G6 ordinary"})
	if err != nil {
		t.Fatal(err)
	}
	administrator, err := auth.WechatLogin(ctx, user.LoginInput{Code: "g6a1-admin", Nickname: "G6 admin"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&user.User{}).Where("id = ?", administrator.User.ID).Update("role", user.RoleAdmin).Error; err != nil {
		t.Fatal(err)
	}

	merchant, err := merchants.CreateMerchant(ctx, takeout.CreateMerchantInput{OwnerUserID: uint64(administrator.User.ID), Name: "G6 HTTP Merchant"})
	if err != nil {
		t.Fatal(err)
	}
	merchant, err = merchants.Approve(ctx, merchant.MerchantPublicID, merchant.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	category, err := catalog.CreateCategory(ctx, takeout.CreateCategoryInput{MerchantPublicID: merchant.MerchantPublicID, Name: "HTTP category"})
	if err != nil {
		t.Fatal(err)
	}
	dish, err := catalog.CreateDish(ctx, takeout.CreateDishInput{MerchantPublicID: merchant.MerchantPublicID, CategoryPublicID: &category.CategoryPublicID, Name: "HTTP dish", PriceCents: 1000})
	if err != nil {
		t.Fatal(err)
	}
	dish, err = catalog.PutOnShelf(ctx, dish.DishPublicID, merchant.MerchantPublicID, dish.Status, dish.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	order, err := orders.CreateOrder(ctx, takeout.CreateOrderInput{
		BuyerUserID: uint64(ordinary.User.ID), MerchantPublicID: merchant.MerchantPublicID,
		Items:   []takeout.OrderItemRequest{{DishPublicID: dish.DishPublicID, Quantity: 1}},
		Address: takeout.OrderAddress{ContactName: "G6A1 Buyer", ContactPhone: "13000000000", AddressBuilding: "A", AddressRoom: "101"},
	})
	if err != nil {
		t.Fatal(err)
	}
	rider, err := riders.RegisterRider(ctx, takeout.RegisterRiderInput{RiderUserID: g6HTTPRiderID, RealName: "G6 HTTP Rider", Phone: "13000000001"})
	if err != nil {
		t.Fatal(err)
	}
	earningID, _ := takeout.NewULID()
	earningOrderID, _ := takeout.NewULID()
	earning := takeout.RiderEarningRecord{
		EarningPublicID: earningID, OrderPublicID: earningOrderID, RiderUserID: g6HTTPRiderID,
		RiderFeeCents: 100, PlatformCutCents: 50, SettlementStatus: takeout.SettlementPending,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&earning).Error; err != nil {
		t.Fatal(err)
	}
	withdraw, err := withdraws.CreateWithdrawRequest(ctx, g6HTTPRiderID, 100)
	if err != nil {
		t.Fatal(err)
	}
	rejectedWithdraw, err := withdraws.CreateWithdrawRequest(ctx, g6HTTPRiderID, 50)
	if err != nil {
		t.Fatal(err)
	}

	feeBefore, err := orders.GetActiveFeeConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	riderConfigBefore, err := riders.GetRiderConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Model(&takeout.FeeConfig{}).Where("config_scope = ?", "global").Updates(map[string]any{
			"rider_fee_cents": feeBefore.RiderFeeCents, "platform_cut_cents": feeBefore.PlatformCutCents,
			"row_version": feeBefore.RowVersion, "updated_at": feeBefore.UpdatedAt,
		})
		db.Model(&takeout.RiderConfig{}).Where("config_scope = ?", "global").Updates(map[string]any{
			"max_in_flight_per_rider": riderConfigBefore.MaxInFlightPerRider,
			"row_version":             riderConfigBefore.RowVersion, "updated_at": riderConfigBefore.UpdatedAt,
		})
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes := router.Group("/api/v1/admin/takeout", auth.AuthRequired(), user.AdminRequired())
	NewHandler(merchants, catalog, orders, riders, queries, withdraws).Register(routes)
	server := httptest.NewServer(router)
	defer server.Close()

	status, _ := httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/admin/takeout/merchants", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("no token status=%d want=401", status)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/admin/takeout/merchants", ordinary.Token, nil)
	if status != http.StatusForbidden {
		t.Fatalf("ordinary status=%d want=403", status)
	}
	status, body := httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/admin/takeout/merchants?page=1&pageSize=20", administrator.Token, nil)
	assertHTTPPageContains(t, status, body, "merchantPublicId", merchant.MerchantPublicID)

	for _, check := range []struct{ path, field, expected string }{
		{"/categories?merchantId=" + merchant.MerchantPublicID, "categoryPublicId", category.CategoryPublicID},
		{"/dishes?merchantId=" + merchant.MerchantPublicID + "&status=on_shelf", "dishPublicId", dish.DishPublicID},
		{"/orders?status=pending_payment", "orderPublicId", order.OrderPublicID},
		{"/riders?status=active", "riderUserId", strconv.FormatUint(g6HTTPRiderID, 10)},
		{"/settlement-report", "riderUserId", strconv.FormatUint(g6HTTPRiderID, 10)},
		{"/withdraw-requests?status=pending", "withdrawPublicId", withdraw.WithdrawPublicID},
	} {
		status, body = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/admin/takeout"+check.path, administrator.Token, nil)
		assertHTTPPageContains(t, status, body, check.field, check.expected)
	}
	for _, path := range []string{
		"/merchants/" + merchant.MerchantPublicID,
		"/categories/" + category.CategoryPublicID,
		"/dishes/" + dish.DishPublicID,
		"/orders/" + order.OrderPublicID,
		"/withdraw-requests/" + withdraw.WithdrawPublicID,
	} {
		status, body = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/admin/takeout"+path, administrator.Token, nil)
		if status != http.StatusOK || body.Code != 0 {
			t.Fatalf("GET %s status=%d body=%+v", path, status, body)
		}
	}
	for _, path := range []string{"/fee-config", "/rider-config"} {
		status, body = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/admin/takeout"+path, administrator.Token, nil)
		if status != http.StatusOK || body.Code != 0 {
			t.Fatalf("GET %s status=%d body=%+v", path, status, body)
		}
	}

	status, _ = httpJSON(t, server.Client(), http.MethodPatch, server.URL+"/api/v1/admin/takeout/merchants/"+merchant.MerchantPublicID+"/suspend", administrator.Token, map[string]any{
		"expectedStatus": merchant.Status, "expectedRowVersion": merchant.RowVersion,
	})
	if status != http.StatusOK {
		t.Fatalf("suspend merchant status=%d", status)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPatch, server.URL+"/api/v1/admin/takeout/merchants/"+merchant.MerchantPublicID+"/unsuspend", administrator.Token, map[string]any{"expectedRowVersion": merchant.RowVersion + 1})
	if status != http.StatusOK {
		t.Fatalf("unsuspend merchant status=%d", status)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPatch, fmt.Sprintf("%s/api/v1/admin/takeout/riders/%d/status", server.URL, rider.RiderUserID), administrator.Token, map[string]any{
		"status": takeout.RiderResting, "expectedStatus": rider.Status, "expectedRowVersion": rider.RowVersion,
	})
	if status != http.StatusOK {
		t.Fatalf("rider status update=%d", status)
	}

	status, _ = httpJSON(t, server.Client(), http.MethodPut, server.URL+"/api/v1/admin/takeout/fee-config", administrator.Token, map[string]any{
		"riderFeeCents": feeBefore.RiderFeeCents + 1, "platformCutCents": feeBefore.PlatformCutCents, "expectedRowVersion": feeBefore.RowVersion,
	})
	if status != http.StatusOK {
		t.Fatalf("fee config update=%d", status)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPut, server.URL+"/api/v1/admin/takeout/fee-config", administrator.Token, map[string]any{
		"riderFeeCents": feeBefore.RiderFeeCents + 2, "platformCutCents": feeBefore.PlatformCutCents, "expectedRowVersion": feeBefore.RowVersion,
	})
	if status != http.StatusConflict {
		t.Fatalf("stale fee config status=%d want=409", status)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPut, server.URL+"/api/v1/admin/takeout/rider-config", administrator.Token, map[string]any{
		"maxInFlightPerRider": riderConfigBefore.MaxInFlightPerRider + 1, "expectedRowVersion": riderConfigBefore.RowVersion,
	})
	if status != http.StatusOK {
		t.Fatalf("rider config update=%d", status)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPut, server.URL+"/api/v1/admin/takeout/rider-config", administrator.Token, map[string]any{
		"maxInFlightPerRider": riderConfigBefore.MaxInFlightPerRider + 2, "expectedRowVersion": riderConfigBefore.RowVersion,
	})
	if status != http.StatusConflict {
		t.Fatalf("stale rider config status=%d want=409", status)
	}

	status, _ = httpJSON(t, server.Client(), http.MethodPatch, server.URL+"/api/v1/admin/takeout/withdraw-requests/"+withdraw.WithdrawPublicID+"/pay", administrator.Token, map[string]any{"expectedRowVersion": withdraw.RowVersion})
	if status != http.StatusOK {
		t.Fatalf("pay withdraw status=%d", status)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPatch, server.URL+"/api/v1/admin/takeout/withdraw-requests/"+withdraw.WithdrawPublicID+"/pay", administrator.Token, map[string]any{"expectedRowVersion": withdraw.RowVersion})
	if status != http.StatusOK {
		t.Fatalf("repeat pay withdraw status=%d", status)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPatch, server.URL+"/api/v1/admin/takeout/withdraw-requests/"+rejectedWithdraw.WithdrawPublicID+"/reject", administrator.Token, map[string]any{
		"expectedRowVersion": rejectedWithdraw.RowVersion, "reason": "manual verification failed",
	})
	if status != http.StatusOK {
		t.Fatalf("reject withdraw status=%d", status)
	}
	var loadedEarning takeout.RiderEarningRecord
	if err := db.Where("id = ?", earning.ID).First(&loadedEarning).Error; err != nil {
		t.Fatal(err)
	}
	if loadedEarning.SettlementStatus != takeout.SettlementSettled {
		t.Fatalf("earning status=%s", loadedEarning.SettlementStatus)
	}
	t.Log("real_http=PASS auth=401/403/200 paginated_lists=PASS config_CAS=PASS withdraw_payment_rejection=PASS repeat_paid=idempotent")
}

func TestZZTakeoutAdminHTTPResidueAgainstMySQL84(t *testing.T) {
	db := openG6A1HTTPMySQL(t)
	cleanupG6A1HTTP(t, db)
	checks := []struct {
		name  string
		model any
		query string
		args  []any
	}{
		{"withdrawals", &takeout.WithdrawRequest{}, "rider_user_id = ?", []any{g6HTTPRiderID}},
		{"earnings", &takeout.RiderEarningRecord{}, "rider_user_id = ?", []any{g6HTTPRiderID}},
		{"riders", &takeout.Rider{}, "rider_user_id = ?", []any{g6HTTPRiderID}},
		{"merchants", &takeout.Merchant{}, "name = ?", []any{"G6 HTTP Merchant"}},
		{"categories", &takeout.DishCategory{}, "name = ?", []any{"HTTP category"}},
		{"dishes", &takeout.Dish{}, "name = ?", []any{"HTTP dish"}},
		{"orders", &takeout.Order{}, "contact_name = ?", []any{"G6A1 Buyer"}},
	}
	for _, check := range checks {
		var count int64
		if err := db.Model(check.model).Where(check.query, check.args...).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("%s residue=%d", check.name, count)
		}
	}
	var users int64
	if err := db.Unscoped().Model(&user.User{}).Where("open_id IN ?", []string{"mock_openid_g6a1-ordinary", "mock_openid_g6a1-admin"}).Count(&users).Error; err != nil {
		t.Fatal(err)
	}
	if users != 0 {
		t.Errorf("users residue=%d", users)
	}
	var fee takeout.FeeConfig
	if err := db.Where("config_scope = ?", "global").First(&fee).Error; err != nil {
		t.Fatal(err)
	}
	if fee.RiderFeeCents != 100 || fee.PlatformCutCents != 50 || fee.RowVersion != 1 {
		t.Errorf("fee config residue=%#v", fee)
	}
	var riderConfig takeout.RiderConfig
	if err := db.Where("config_scope = ?", "global").First(&riderConfig).Error; err != nil {
		t.Fatal(err)
	}
	if riderConfig.MaxInFlightPerRider != 1000 || riderConfig.RowVersion != 1 {
		t.Errorf("rider config residue=%#v", riderConfig)
	}
	t.Log("http_residue users=0 merchants=0 categories=0 dishes=0 orders=0 riders=0 withdrawals=0 earnings=0 configs=baseline")
}

func httpJSON(t *testing.T, client *http.Client, method, url, token string, payload any) (int, apiEnvelope) {
	t.Helper()
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	request, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	result, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Body.Close()
	var envelope apiEnvelope
	if err := json.NewDecoder(result.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	return result.StatusCode, envelope
}

func assertHTTPPage(t *testing.T, status int, body apiEnvelope, minimumTotal int64) {
	t.Helper()
	if status != http.StatusOK || body.Code != 0 {
		t.Fatalf("page status=%d body=%+v", status, body)
	}
	var page struct {
		Items    []json.RawMessage `json:"items"`
		Page     int               `json:"page"`
		PageSize int               `json:"pageSize"`
		Total    int64             `json:"total"`
	}
	if err := json.Unmarshal(body.Data, &page); err != nil {
		t.Fatal(err)
	}
	if page.Page != 1 || page.PageSize != 20 || page.Total < minimumTotal || len(page.Items) == 0 {
		t.Fatalf("invalid page=%+v", page)
	}
}

func assertHTTPPageContains(t *testing.T, status int, body apiEnvelope, field, expected string) {
	t.Helper()
	assertHTTPPage(t, status, body, 1)
	var page struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body.Data, &page); err != nil {
		t.Fatal(err)
	}
	for _, item := range page.Items {
		if fmt.Sprint(item[field]) == expected {
			return
		}
	}
	t.Fatalf("page does not contain %s=%s: %s", field, expected, body.Data)
}

func openG6A1HTTPMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TAKEOUT_G6A1_MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("set TAKEOUT_G6A1_MYSQL_DSN or MYSQL_DSN for G-6a-1 HTTP acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertG6A1HTTPSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range []any{
		&user.User{}, &takeout.Merchant{}, &takeout.DishCategory{}, &takeout.Dish{}, &takeout.Order{},
		&takeout.OrderItem{}, &takeout.Rider{}, &takeout.RiderEarningRecord{}, &takeout.FeeConfig{},
		&takeout.RiderConfig{}, &takeout.WithdrawRequest{},
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required migration missing for %T", table)
		}
	}
}

func cleanupG6A1HTTP(t *testing.T, db *gorm.DB) {
	t.Helper()
	db.Where("rider_user_id = ?", g6HTTPRiderID).Delete(&takeout.WithdrawRequest{})
	db.Where("rider_user_id = ?", g6HTTPRiderID).Delete(&takeout.RiderEarningRecord{})
	db.Where("rider_user_id = ?", g6HTTPRiderID).Delete(&takeout.Rider{})
	var merchants []takeout.Merchant
	db.Where("name = ?", "G6 HTTP Merchant").Find(&merchants)
	for _, merchant := range merchants {
		db.Where("order_public_id IN (?)", db.Model(&takeout.Order{}).Select("order_public_id").Where("merchant_public_id = ?", merchant.MerchantPublicID)).Delete(&takeout.OrderItem{})
		db.Where("merchant_public_id = ?", merchant.MerchantPublicID).Delete(&takeout.Order{})
		db.Where("merchant_public_id = ?", merchant.MerchantPublicID).Delete(&takeout.Dish{})
		db.Where("merchant_public_id = ?", merchant.MerchantPublicID).Delete(&takeout.DishCategory{})
		db.Where("id = ?", merchant.ID).Delete(&takeout.Merchant{})
	}
	db.Unscoped().Where("open_id IN ?", []string{"mock_openid_g6a1-ordinary", "mock_openid_g6a1-admin"}).Delete(&user.User{})
}
