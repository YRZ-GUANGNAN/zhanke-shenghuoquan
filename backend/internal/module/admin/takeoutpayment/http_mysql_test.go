package takeoutpayment

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

	"campus-circle/backend/internal/module/admin/takeoutbuyer"
	"campus-circle/backend/internal/module/admin/takeoutmerchant"
	"campus-circle/backend/internal/module/admin/takeoutrider"
	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/user"
)

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func TestTakeoutPaymentMockRealHTTPMySQLPaidAndDownstream(t *testing.T) {
	db := openG4MySQL(t)
	assertG4Schema(t, db)
	cleanupG4(t, db)
	t.Cleanup(func() { cleanupG4(t, db) })

	ctx := context.Background()
	now := time.Date(2026, 6, 25, 14, 0, 0, 0, time.UTC)
	repository := takeout.NewRepository(db)
	merchants, _ := takeout.NewMerchantService(repository, func() time.Time { return now })
	identity, _ := takeout.NewMerchantIdentityService(repository)
	catalog, _ := takeout.NewCatalogService(repository, func() time.Time { return now })
	orders, _ := takeout.NewOrderService(repository, repository, func() time.Time { return now })
	payments, _ := takeout.NewPaymentService(repository, orders, func() time.Time { return now })
	queries, _ := takeout.NewAdminQueryService(repository)
	buyerQueries, _ := takeout.NewBuyerQueryService(repository)
	riderOrderOps, _ := takeout.NewRiderOrderOps(repository, func() time.Time { return now })
	riders, _ := takeout.NewRiderService(repository, riderOrderOps, func() time.Time { return now })
	withdraws, _ := takeout.NewWithdrawService(repository, func() time.Time { return now })
	riderTransport, err := takeoutrider.NewService(riders, orders, withdraws, takeoutrider.NewRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	auth := user.NewService(user.NewRepository(db), "g4-test-secret-that-is-longer-than-thirty-two-characters", "", "", true)

	buyer, _ := auth.WechatLogin(ctx, user.LoginInput{Code: "g4-buyer", Nickname: "G4 Buyer"})
	merchantUser, _ := auth.WechatLogin(ctx, user.LoginInput{Code: "g4-merchant", Nickname: "G4 Merchant"})
	riderUser, _ := auth.WechatLogin(ctx, user.LoginInput{Code: "g4-rider", Nickname: "G4 Rider"})
	merchant, dish := seedG4MerchantDish(t, ctx, merchants, catalog, uint64(merchantUser.User.ID))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	buyerRoutes := router.Group("/api/v1/takeout/buyer", auth.AuthRequired(), user.UnmutedRequired())
	takeoutbuyer.NewHandler(buyerQueries, orders, func(c *gin.Context) uint64 { return uint64(user.CurrentUserID(c)) }).Register(buyerRoutes)
	NewHandler(payments, MockProvider{}, func(c *gin.Context) uint64 {
		return uint64(user.CurrentUserID(c))
	}, func(c *gin.Context) string {
		return user.CurrentUser(c).OpenID
	}).RegisterBuyer(buyerRoutes)
	takeoutmerchant.NewHandler(identity, merchants, catalog, orders, queries, nil, nil, func(c *gin.Context) uint64 {
		return uint64(user.CurrentUserID(c))
	}).Register(router.Group("/api/v1/takeout/merchant", auth.AuthRequired()))
	takeoutrider.NewHandler(riderTransport, func(c *gin.Context) uint64 {
		return uint64(user.CurrentUserID(c))
	}).Register(router.Group("/api/v1/takeout/rider", auth.AuthRequired()))
	server := httptest.NewServer(router)
	defer server.Close()

	status, body := httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/buyer/orders", buyer.Token, map[string]any{
		"merchantPublicId": merchant.MerchantPublicID,
		"items":            []map[string]any{{"dishPublicId": dish.DishPublicID, "quantity": 2}},
		"address":          map[string]any{"contactName": "G4 Buyer", "contactPhone": "13000000000", "addressBuilding": "G4 Building", "addressRoom": "404"},
		"remark":           "G4 payment acceptance",
		"amountCents":      1,
	})
	if status != http.StatusCreated {
		t.Fatalf("create order status=%d body=%+v", status, body)
	}
	var created struct {
		Order struct {
			OrderPublicID    string              `json:"orderPublicId"`
			Status           takeout.OrderStatus `json:"status"`
			RowVersion       uint64              `json:"rowVersion"`
			TotalAmountCents uint64              `json:"totalAmountCents"`
		} `json:"order"`
	}
	if err := json.Unmarshal(body.Data, &created); err != nil {
		t.Fatal(err)
	}
	if created.Order.Status != takeout.OrderPendingPayment || created.Order.TotalAmountCents == 1 {
		t.Fatalf("order did not start pending with server amount: %+v", created.Order)
	}

	status, body = httpJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/takeout/buyer/orders/"+created.Order.OrderPublicID+"/pay", buyer.Token, nil)
	if status != http.StatusOK {
		t.Fatalf("mock pay status=%d body=%+v", status, body)
	}
	var paid PaymentResponse
	if err := json.Unmarshal(body.Data, &paid); err != nil {
		t.Fatal(err)
	}
	if paid.Mode != "mock" || paid.Order.Status != takeout.OrderPaid || paid.AmountCents != created.Order.TotalAmountCents {
		t.Fatalf("mock payment response=%+v", paid)
	}
	var attemptCount int64
	if err := db.Model(&takeout.PaymentAttempt{}).Where("order_public_id = ? AND provider = ? AND status = ?",
		created.Order.OrderPublicID, takeout.PaymentProviderMock, takeout.PaymentPaid).Count(&attemptCount).Error; err != nil || attemptCount != 1 {
		t.Fatalf("payment attempt count=%d err=%v", attemptCount, err)
	}

	currentOrder := paid.Order
	for _, step := range []struct {
		path string
		want takeout.OrderStatus
	}{
		{"/accept", takeout.OrderAccepted},
		{"/start-preparing", takeout.OrderPreparing},
		{"/mark-ready", takeout.OrderReadyForPickup},
	} {
		status, body = httpJSON(t, server.Client(), http.MethodPatch,
			server.URL+"/api/v1/takeout/merchant/orders/"+created.Order.OrderPublicID+step.path,
			merchantUser.Token, map[string]any{"expectedRowVersion": currentOrder.RowVersion})
		if status != http.StatusOK {
			t.Fatalf("merchant %s status=%d body=%+v", step.path, status, body)
		}
		if err := json.Unmarshal(body.Data, &currentOrder); err != nil {
			t.Fatal(err)
		}
		if currentOrder.Status != step.want {
			t.Fatalf("merchant %s status=%s want=%s", step.path, currentOrder.Status, step.want)
		}
	}

	status, _ = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/rider/register", riderUser.Token,
		map[string]any{"realName": "G4 Rider", "phone": "13100000004"})
	if status != http.StatusCreated {
		t.Fatalf("rider register status=%d", status)
	}
	status, body = httpJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/takeout/rider/orders/"+created.Order.OrderPublicID+"/claim",
		riderUser.Token, map[string]any{"expectedRowVersion": currentOrder.RowVersion})
	if status != http.StatusOK {
		t.Fatalf("rider claim status=%d body=%+v", status, body)
	}
	var fulfillment struct {
		Fulfillment struct {
			FulfillmentPublicID string                    `json:"fulfillmentPublicId"`
			Status              takeout.FulfillmentStatus `json:"status"`
			RowVersion          uint64                    `json:"rowVersion"`
		} `json:"fulfillment"`
		Order PaymentOrderView `json:"order"`
	}
	if err := json.Unmarshal(body.Data, &fulfillment); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		path string
		want takeout.FulfillmentStatus
	}{
		{"/to-store", takeout.FulfillmentToStore},
		{"/picked", takeout.FulfillmentPicked},
		{"/to-user", takeout.FulfillmentToUser},
		{"/complete", takeout.FulfillmentDelivered},
	} {
		status, body = httpJSON(t, server.Client(), http.MethodPatch,
			server.URL+"/api/v1/takeout/rider/fulfillments/"+fulfillment.Fulfillment.FulfillmentPublicID+step.path,
			riderUser.Token, map[string]any{"expectedRowVersion": fulfillment.Fulfillment.RowVersion})
		if status != http.StatusOK {
			t.Fatalf("rider %s status=%d body=%+v", step.path, status, body)
		}
		if err := json.Unmarshal(body.Data, &fulfillment); err != nil {
			t.Fatal(err)
		}
		if fulfillment.Fulfillment.Status != step.want {
			t.Fatalf("rider %s status=%s want=%s", step.path, fulfillment.Fulfillment.Status, step.want)
		}
	}
	t.Log("real_http_mysql=PASS mock_payment_pending_to_paid=PASS server_amount_used=PASS downstream_merchant_rider_chain=PASS")
}

func seedG4MerchantDish(t *testing.T, ctx context.Context, merchants *takeout.MerchantService, catalog *takeout.CatalogService, ownerID uint64) (takeout.Merchant, takeout.Dish) {
	t.Helper()
	merchant, err := merchants.CreateMerchant(ctx, takeout.CreateMerchantInput{
		OwnerUserID: ownerID, Name: "G4 Merchant", ContactPhone: "13000000004",
		Location: "G4 Gate", BusinessHours: "09:00-20:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	merchant, err = merchants.Approve(ctx, merchant.MerchantPublicID, merchant.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	category, err := catalog.CreateCategory(ctx, takeout.CreateCategoryInput{MerchantPublicID: merchant.MerchantPublicID, Name: "G4 Category"})
	if err != nil {
		t.Fatal(err)
	}
	dish, err := catalog.CreateDish(ctx, takeout.CreateDishInput{
		MerchantPublicID: merchant.MerchantPublicID, CategoryPublicID: &category.CategoryPublicID,
		Name: "G4 Dish", PriceCents: 1200,
	})
	if err != nil {
		t.Fatal(err)
	}
	dish, err = catalog.PutOnShelf(ctx, dish.DishPublicID, merchant.MerchantPublicID, dish.Status, dish.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	return merchant, dish
}

func httpJSON(t *testing.T, client *http.Client, method, url, token string, payload any) (int, apiEnvelope) {
	t.Helper()
	var body *bytes.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(raw)
	} else {
		body = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var envelope apiEnvelope
	_ = json.NewDecoder(resp.Body).Decode(&envelope)
	return resp.StatusCode, envelope
}

func openG4MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TAKEOUT_G4_MYSQL_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	}
	if dsn == "" {
		t.Skip("set TAKEOUT_G4_MYSQL_DSN or MYSQL_DSN for G-4 payment real HTTP MySQL acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertG4Schema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range []any{
		&user.User{}, &takeout.Merchant{}, &takeout.DishCategory{}, &takeout.Dish{},
		&takeout.Order{}, &takeout.OrderItem{}, &takeout.FeeConfig{},
		&takeout.PaymentAttempt{}, &takeout.PaymentNotification{},
		&takeout.Rider{}, &takeout.Fulfillment{}, &takeout.RiderConfig{}, &takeout.RiderEarningRecord{},
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required migration missing for %T", table)
		}
	}
}

func cleanupG4(t *testing.T, db *gorm.DB) {
	t.Helper()
	var merchantPublicIDs []string
	db.Model(&takeout.Merchant{}).Where("name LIKE ?", "G4%").Pluck("merchant_public_id", &merchantPublicIDs)
	var orderPublicIDs []string
	orderQuery := db.Model(&takeout.Order{}).Where("contact_name LIKE ? OR remark LIKE ?", "G4%", "G4%")
	if len(merchantPublicIDs) > 0 {
		orderQuery = orderQuery.Or("merchant_public_id IN ?", merchantPublicIDs)
	}
	orderQuery.Pluck("order_public_id", &orderPublicIDs)
	if len(orderPublicIDs) > 0 {
		db.Where("out_trade_no IN (?)", db.Model(&takeout.PaymentAttempt{}).Select("out_trade_no").Where("order_public_id IN ?", orderPublicIDs)).Delete(&takeout.PaymentNotification{})
		db.Where("order_public_id IN ?", orderPublicIDs).Delete(&takeout.PaymentAttempt{})
		db.Where("order_public_id IN ?", orderPublicIDs).Delete(&takeout.RiderEarningRecord{})
		db.Where("order_public_id IN ?", orderPublicIDs).Delete(&takeout.Fulfillment{})
		db.Where("order_public_id IN ?", orderPublicIDs).Delete(&takeout.OrderItem{})
		db.Where("order_public_id IN ?", orderPublicIDs).Delete(&takeout.Order{})
	}
	if len(merchantPublicIDs) > 0 {
		db.Where("merchant_public_id IN ?", merchantPublicIDs).Delete(&takeout.Dish{})
		db.Where("merchant_public_id IN ?", merchantPublicIDs).Delete(&takeout.DishCategory{})
	}
	db.Where("name LIKE ?", "G4%").Delete(&takeout.Merchant{})
	var userIDs []uint
	db.Model(&user.User{}).Where("open_id LIKE ?", "mock_openid_g4-%").Pluck("id", &userIDs)
	if len(userIDs) > 0 {
		db.Where("rider_user_id IN ?", userIDs).Delete(&takeout.Rider{})
	}
	db.Unscoped().Where("open_id LIKE ?", "mock_openid_g4-%").Delete(&user.User{})
}

func TestZZTakeoutPaymentResidueAgainstMySQL84(t *testing.T) {
	db := openG4MySQL(t)
	cleanupG4(t, db)
	checks := []struct {
		name  string
		model any
		query string
		args  []any
	}{
		{"users", &user.User{}, "open_id LIKE ?", []any{"mock_openid_g4-%"}},
		{"merchants", &takeout.Merchant{}, "name LIKE ?", []any{"G4%"}},
		{"orders", &takeout.Order{}, "contact_name LIKE ? OR remark LIKE ?", []any{"G4%", "G4%"}},
		{"payments", &takeout.PaymentAttempt{}, "out_trade_no LIKE ?", []any{"G4%"}},
	}
	for _, check := range checks {
		var count int64
		if err := db.Unscoped().Model(check.model).Where(check.query, check.args...).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s residue=%d", check.name, count)
		}
	}
}
