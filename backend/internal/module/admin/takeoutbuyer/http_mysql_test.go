package takeoutbuyer

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

	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/user"
)

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func TestTakeoutBuyerAPIRealHTTPMySQLPublicVisibilityOrderAndOwnership(t *testing.T) {
	db := openG7AMySQL(t)
	assertG7ASchema(t, db)
	cleanupG7A(t, db)
	t.Cleanup(func() { cleanupG7A(t, db) })

	ctx := context.Background()
	now := time.Date(2026, 6, 24, 18, 0, 0, 0, time.UTC)
	repository := takeout.NewRepository(db)
	merchants, _ := takeout.NewMerchantService(repository, func() time.Time { return now })
	catalog, _ := takeout.NewCatalogService(repository, func() time.Time { return now })
	orders, _ := takeout.NewOrderService(repository, repository, func() time.Time { return now })
	queries, _ := takeout.NewBuyerQueryService(repository)
	auth := user.NewService(user.NewRepository(db), "g7a-test-secret-that-is-longer-than-thirty-two-characters", "", "", true)

	buyerA, err := auth.WechatLogin(ctx, user.LoginInput{Code: "g7a-buyer-a", Nickname: "G7A Buyer A"})
	if err != nil {
		t.Fatal(err)
	}
	buyerB, err := auth.WechatLogin(ctx, user.LoginInput{Code: "g7a-buyer-b", Nickname: "G7A Buyer B"})
	if err != nil {
		t.Fatal(err)
	}
	activeMerchant, onShelfDish, offShelfDish := seedG7AMerchantCatalog(t, ctx, merchants, catalog, uint64(buyerA.User.ID), "G7A Active Merchant", takeout.MerchantActive)
	restingMerchant, restingDish, _ := seedG7AMerchantCatalog(t, ctx, merchants, catalog, uint64(buyerB.User.ID), "G7A Resting Merchant", takeout.MerchantResting)
	imageAssetID, err := takeout.NewULID()
	if err != nil {
		t.Fatal(err)
	}
	withImageDish, err := catalog.CreateDish(ctx, takeout.CreateDishInput{
		MerchantPublicID: activeMerchant.MerchantPublicID, Name: "G7A Dish With Image", PriceCents: 1200, ImageAssetPublicID: &imageAssetID,
	})
	if err != nil {
		t.Fatal(err)
	}
	withImageDish, err = catalog.PutOnShelf(ctx, withImageDish.DishPublicID, activeMerchant.MerchantPublicID, withImageDish.Status, withImageDish.RowVersion)
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes := router.Group("/api/v1/takeout/buyer", auth.AuthRequired(), user.UnmutedRequired())
	NewHandler(queries, orders, func(c *gin.Context) uint64 { return uint64(user.CurrentUserID(c)) }).Register(routes)
	server := httptest.NewServer(router)
	defer server.Close()

	status, _ := httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/buyer/merchants", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("no token status=%d want=401", status)
	}
	status, body := httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/buyer/merchants?page=1&pageSize=20", buyerA.Token, nil)
	assertHTTPPageContains(t, status, body, "merchantPublicId", activeMerchant.MerchantPublicID)
	assertHTTPPageNotContains(t, body, restingMerchant.MerchantPublicID)

	status, _ = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/buyer/merchants/"+restingMerchant.MerchantPublicID, buyerA.Token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("resting merchant detail status=%d want=404", status)
	}
	status, body = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/buyer/merchants/"+activeMerchant.MerchantPublicID+"/dishes", buyerA.Token, nil)
	assertHTTPPageContains(t, status, body, "dishPublicId", onShelfDish.DishPublicID)
	assertHTTPPageContains(t, status, body, "dishPublicId", withImageDish.DishPublicID)
	assertHTTPPageContains(t, status, body, "imageContentUrl", "/api/v1/miniapp/media/display/dish/"+withImageDish.DishPublicID+"/0/content")
	assertHTTPPageNotContains(t, body, offShelfDish.DishPublicID)
	status, _ = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/buyer/merchants/"+restingMerchant.MerchantPublicID+"/dishes", buyerA.Token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("resting merchant dishes status=%d want=404", status)
	}

	fee, err := orders.GetActiveFeeConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	expectedTotal := onShelfDish.PriceCents*2 + fee.RiderFeeCents + fee.PlatformCutCents
	status, body = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/buyer/orders", buyerA.Token, map[string]any{
		"merchantPublicId":  activeMerchant.MerchantPublicID,
		"items":             []map[string]any{{"dishPublicId": onShelfDish.DishPublicID, "quantity": 2}},
		"address":           map[string]any{"contactName": "G7A Buyer A", "contactPhone": "13000000000", "addressBuilding": "A", "addressRoom": "101"},
		"remark":            "no onion",
		"totalAmountCents":  1,
		"itemsAmountCents":  1,
		"deliveryFeeCents":  1,
		"untrustedBuyerId":  uint64(buyerB.User.ID),
		"untrustedPaidFlag": true,
	})
	if status != http.StatusCreated {
		t.Fatalf("create order status=%d body=%+v", status, body)
	}
	var created OrderDetailView
	if err := json.Unmarshal(body.Data, &created); err != nil {
		t.Fatal(err)
	}
	if created.Order.Status != takeout.OrderPendingPayment || created.Order.BuyerUserID != uint64(buyerA.User.ID) ||
		created.Order.TotalAmountCents != expectedTotal || len(created.Items) != 1 || created.Items[0].Quantity != 2 {
		t.Fatalf("order did not use server-side facts: %+v expectedTotal=%d", created, expectedTotal)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/buyer/orders", buyerA.Token, map[string]any{
		"merchantPublicId": activeMerchant.MerchantPublicID,
		"items":            []map[string]any{{"dishPublicId": offShelfDish.DishPublicID, "quantity": 1}},
		"address":          map[string]any{"contactName": "G7A Buyer A"},
	})
	if status != http.StatusConflict {
		t.Fatalf("off-shelf order status=%d want=409", status)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/buyer/orders", buyerA.Token, map[string]any{
		"merchantPublicId": restingMerchant.MerchantPublicID,
		"items":            []map[string]any{{"dishPublicId": restingDish.DishPublicID, "quantity": 1}},
		"address":          map[string]any{"contactName": "G7A Buyer A"},
	})
	if status != http.StatusNotFound {
		t.Fatalf("resting merchant order status=%d want=404", status)
	}

	status, body = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/buyer/orders?status=pending_payment", buyerA.Token, nil)
	assertHTTPPageContains(t, status, body, "orderPublicId", created.Order.OrderPublicID)
	status, body = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/buyer/orders/"+created.Order.OrderPublicID, buyerA.Token, nil)
	if status != http.StatusOK || !strings.Contains(string(body.Data), created.Items[0].DishNameSnapshot) {
		t.Fatalf("buyer order detail status=%d body=%s", status, body.Data)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/buyer/orders/"+created.Order.OrderPublicID, buyerB.Token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("cross-buyer order detail status=%d want=404", status)
	}
	for _, path := range []string{"/orders/" + created.Order.OrderPublicID + "/pay", "/orders/" + created.Order.OrderPublicID + "/mark-paid"} {
		status, _ = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/buyer"+path, buyerA.Token, map[string]any{})
		if status != http.StatusNotFound {
			t.Fatalf("unexpected payment route %s status=%d want=404", path, status)
		}
	}
	t.Log("real_http=PASS auth=401 buyer_visibility=active/on_shelf only image_display_path=PASS server_amount_recompute=PASS buyer_order_scope=PASS no_payment_routes=PASS")
}

func seedG7AMerchantCatalog(
	t *testing.T,
	ctx context.Context,
	merchants *takeout.MerchantService,
	catalog *takeout.CatalogService,
	ownerID uint64,
	name string,
	finalStatus takeout.MerchantStatus,
) (takeout.Merchant, takeout.Dish, takeout.Dish) {
	t.Helper()
	merchant, err := merchants.CreateMerchant(ctx, takeout.CreateMerchantInput{OwnerUserID: ownerID, Name: name, ContactPhone: "13000000000", Location: "North Gate", BusinessHours: "09:00-20:00"})
	if err != nil {
		t.Fatal(err)
	}
	merchant, err = merchants.Approve(ctx, merchant.MerchantPublicID, merchant.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	category, err := catalog.CreateCategory(ctx, takeout.CreateCategoryInput{MerchantPublicID: merchant.MerchantPublicID, Name: name + " Category"})
	if err != nil {
		t.Fatal(err)
	}
	onShelf, err := catalog.CreateDish(ctx, takeout.CreateDishInput{MerchantPublicID: merchant.MerchantPublicID, CategoryPublicID: &category.CategoryPublicID, Name: name + " On Shelf", PriceCents: 1000})
	if err != nil {
		t.Fatal(err)
	}
	onShelf, err = catalog.PutOnShelf(ctx, onShelf.DishPublicID, merchant.MerchantPublicID, onShelf.Status, onShelf.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	offShelf, err := catalog.CreateDish(ctx, takeout.CreateDishInput{MerchantPublicID: merchant.MerchantPublicID, CategoryPublicID: &category.CategoryPublicID, Name: name + " Off Shelf", PriceCents: 1100})
	if err != nil {
		t.Fatal(err)
	}
	offShelf, err = catalog.PutOnShelf(ctx, offShelf.DishPublicID, merchant.MerchantPublicID, offShelf.Status, offShelf.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	offShelf, err = catalog.TakeOffShelf(ctx, offShelf.DishPublicID, merchant.MerchantPublicID, offShelf.Status, offShelf.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	if finalStatus == takeout.MerchantResting {
		merchant, err = merchants.Rest(ctx, merchant.MerchantPublicID, ownerID, merchant.RowVersion)
		if err != nil {
			t.Fatal(err)
		}
	}
	return merchant, onShelf, offShelf
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

func assertHTTPPageContains(t *testing.T, status int, body apiEnvelope, field, expected string) {
	t.Helper()
	if status != http.StatusOK || body.Code != 0 {
		t.Fatalf("status=%d body=%+v", status, body)
	}
	if !strings.Contains(string(body.Data), `"`+field+`"`) || !strings.Contains(string(body.Data), expected) {
		t.Fatalf("page missing %s=%s body=%s", field, expected, body.Data)
	}
}

func assertHTTPPageNotContains(t *testing.T, body apiEnvelope, unexpected string) {
	t.Helper()
	if strings.Contains(string(body.Data), unexpected) {
		t.Fatalf("page unexpectedly contains %s body=%s", unexpected, body.Data)
	}
}

func openG7AMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TAKEOUT_G7A_MYSQL_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	}
	if dsn == "" {
		t.Skip("set TAKEOUT_G7A_MYSQL_DSN or MYSQL_DSN for G-7a buyer real HTTP MySQL acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertG7ASchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range []any{
		&user.User{}, &takeout.Merchant{}, &takeout.DishCategory{}, &takeout.Dish{},
		&takeout.Order{}, &takeout.OrderItem{}, &takeout.FeeConfig{},
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required migration missing for %T", table)
		}
	}
}

func cleanupG7A(t *testing.T, db *gorm.DB) {
	t.Helper()
	var merchantPublicIDs []string
	db.Model(&takeout.Merchant{}).Where("name LIKE ?", "G7A%").Pluck("merchant_public_id", &merchantPublicIDs)
	if len(merchantPublicIDs) > 0 {
		db.Where("order_public_id IN (?)", db.Model(&takeout.Order{}).Select("order_public_id").Where("merchant_public_id IN ?", merchantPublicIDs)).Delete(&takeout.OrderItem{})
		db.Where("merchant_public_id IN ?", merchantPublicIDs).Delete(&takeout.Order{})
		db.Where("merchant_public_id IN ?", merchantPublicIDs).Delete(&takeout.Dish{})
		db.Where("merchant_public_id IN ?", merchantPublicIDs).Delete(&takeout.DishCategory{})
	}
	db.Where("name LIKE ?", "G7A%").Delete(&takeout.Merchant{})
	db.Unscoped().Where("open_id IN ?", []string{"mock_openid_g7a-buyer-a", "mock_openid_g7a-buyer-b"}).Delete(&user.User{})
}

func TestZZTakeoutBuyerHTTPResidueAgainstMySQL84(t *testing.T) {
	db := openG7AMySQL(t)
	cleanupG7A(t, db)
	checks := []struct {
		name  string
		model any
		query string
		args  []any
	}{
		{"users", &user.User{}, "open_id IN ?", []any{[]string{"mock_openid_g7a-buyer-a", "mock_openid_g7a-buyer-b"}}},
		{"merchants", &takeout.Merchant{}, "name LIKE ?", []any{"G7A%"}},
		{"categories", &takeout.DishCategory{}, "name LIKE ?", []any{"G7A%"}},
		{"dishes", &takeout.Dish{}, "name LIKE ?", []any{"G7A%"}},
		{"orders", &takeout.Order{}, "contact_name LIKE ?", []any{"G7A%"}},
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
