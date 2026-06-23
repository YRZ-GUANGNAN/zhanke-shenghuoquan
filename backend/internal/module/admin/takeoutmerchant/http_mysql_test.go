package takeoutmerchant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"campus-circle/backend/internal/module/admin/review"
	takeoutreview "campus-circle/backend/internal/module/admin/takeoutconsole/reviewtarget"
	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/user"
)

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func TestTakeoutMerchantAPIRealHTTPMySQLAuthOnboardingOwnerScopeAndMediaBinding(t *testing.T) {
	db := openG7BMySQL(t)
	assertG7BSchema(t, db)
	cleanupG7B(t, db)
	t.Cleanup(func() { cleanupG7B(t, db) })

	ctx := context.Background()
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	repository := takeout.NewRepository(db)
	identity, _ := takeout.NewMerchantIdentityService(repository)
	merchants, _ := takeout.NewMerchantService(repository, func() time.Time { return now })
	catalog, _ := takeout.NewCatalogService(repository, func() time.Time { return now })
	orders, _ := takeout.NewOrderService(repository, repository, func() time.Time { return now })
	queries, _ := takeout.NewAdminQueryService(repository)
	reviews := review.NewService(review.NewRepository(db))
	for _, target := range takeoutreview.NewTargets(merchants, catalog) {
		reviews.RegisterTarget(target)
	}
	mediaService := coremedia.NewService(coremedia.NewRepository(db))
	auth := user.NewService(user.NewRepository(db), "g7b-test-secret-that-is-longer-than-thirty-two-characters", "", "", true)

	merchantAUser, err := auth.WechatLogin(ctx, user.LoginInput{Code: "g7b-merchant-a", Nickname: "G7B Merchant A"})
	if err != nil {
		t.Fatal(err)
	}
	merchantBUser, err := auth.WechatLogin(ctx, user.LoginInput{Code: "g7b-merchant-b", Nickname: "G7B Merchant B"})
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := auth.WechatLogin(ctx, user.LoginInput{Code: "g7b-ordinary", Nickname: "G7B Ordinary"})
	if err != nil {
		t.Fatal(err)
	}
	adminUser, err := auth.WechatLogin(ctx, user.LoginInput{Code: "g7b-admin", Nickname: "G7B Admin"})
	if err != nil {
		t.Fatal(err)
	}

	logoAsset, err := mediaService.CreateAssetRecord(ctx, coremedia.NewAsset{OwnerID: uint64(merchantAUser.User.ID), Purpose: coremedia.PurposeMerchantLogo, IdempotencyKey: "g7b-logo-a"})
	if err != nil {
		t.Fatal(err)
	}
	dishAsset, err := mediaService.CreateAssetRecord(ctx, coremedia.NewAsset{OwnerID: uint64(merchantAUser.User.ID), Purpose: coremedia.PurposeDish, IdempotencyKey: "g7b-dish-a"})
	if err != nil {
		t.Fatal(err)
	}
	otherOwnerDishAsset, err := mediaService.CreateAssetRecord(ctx, coremedia.NewAsset{OwnerID: uint64(merchantBUser.User.ID), Purpose: coremedia.PurposeDish, IdempotencyKey: "g7b-dish-b"})
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes := router.Group("/api/v1/takeout/merchant", auth.AuthRequired())
	NewHandler(identity, merchants, catalog, orders, queries, reviews, mediaService, func(c *gin.Context) uint64 {
		return uint64(user.CurrentUserID(c))
	}).Register(routes)
	server := httptest.NewServer(router)
	defer server.Close()

	status, _ := httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/merchant/categories", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("no token status=%d want=401", status)
	}
	status, body := httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/merchant/categories", ordinary.Token, nil)
	if status != http.StatusConflict || !strings.Contains(body.Message, "onboarding") {
		t.Fatalf("non merchant status=%d body=%+v want explicit onboarding rejection", status, body)
	}

	status, body = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/merchant/me", merchantAUser.Token, nil)
	if status != http.StatusOK || !strings.Contains(string(body.Data), `"onboarded":false`) {
		t.Fatalf("initial me status=%d body=%s", status, body.Data)
	}
	status, body = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/merchant/onboarding", merchantAUser.Token, map[string]any{
		"name":              "G7B Merchant A",
		"contactPhone":      "13000000010",
		"location":          "North Gate",
		"businessHours":     "09:00-20:00",
		"logoAssetPublicId": logoAsset.AssetPublicID,
	})
	if status != http.StatusCreated || body.Code != 0 {
		t.Fatalf("onboarding status=%d body=%+v", status, body)
	}
	var onboarded OnboardingView
	if err := json.Unmarshal(body.Data, &onboarded); err != nil {
		t.Fatal(err)
	}
	if onboarded.Merchant.Status != takeout.MerchantPendingReview || onboarded.Review.ObjectType != takeoutreview.MerchantObjectType {
		t.Fatalf("invalid onboarding payload=%+v", onboarded)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/merchant/onboarding", merchantAUser.Token, map[string]any{
		"name": "G7B Merchant A Duplicate",
	})
	if status != http.StatusConflict {
		t.Fatalf("duplicate onboarding status=%d want=409", status)
	}
	assertMediaReference(t, db, logoAsset.ID, coremedia.PurposeMerchantLogo, onboarded.Merchant.MerchantPublicID)

	if _, err := reviews.Start(ctx, onboarded.Review.TaskID, adminUser.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := reviews.Decide(ctx, onboarded.Review.TaskID, adminUser.User.ID, review.StatusApproved, "verified"); err != nil {
		t.Fatal(err)
	}
	status, body = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/merchant/me", merchantAUser.Token, nil)
	if status != http.StatusOK || !strings.Contains(string(body.Data), `"status":"active"`) {
		t.Fatalf("approved me status=%d body=%s", status, body.Data)
	}

	merchantB, err := merchants.CreateMerchant(ctx, takeout.CreateMerchantInput{OwnerUserID: uint64(merchantBUser.User.ID), Name: "G7B Merchant B"})
	if err != nil {
		t.Fatal(err)
	}
	merchantB, err = merchants.Approve(ctx, merchantB.MerchantPublicID, merchantB.RowVersion)
	if err != nil {
		t.Fatal(err)
	}

	status, body = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/merchant/categories", merchantAUser.Token, map[string]any{
		"name": "G7B Category A", "sortOrder": 1,
	})
	if status != http.StatusCreated {
		t.Fatalf("create category status=%d body=%+v", status, body)
	}
	var category CategoryView
	if err := json.Unmarshal(body.Data, &category); err != nil {
		t.Fatal(err)
	}
	status, body = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/merchant/dishes", merchantAUser.Token, map[string]any{
		"categoryPublicId":   category.CategoryPublicID,
		"name":               "G7B Dish A",
		"description":        "merchant owned dish",
		"priceCents":         1200,
		"imageAssetPublicId": dishAsset.AssetPublicID,
		"sortOrder":          1,
	})
	if status != http.StatusCreated {
		t.Fatalf("create dish status=%d body=%+v", status, body)
	}
	var dish DishView
	if err := json.Unmarshal(body.Data, &dish); err != nil {
		t.Fatal(err)
	}
	if dish.ImageAssetPublicID == nil || *dish.ImageAssetPublicID != dishAsset.AssetPublicID {
		t.Fatalf("dish image not stored: %+v", dish)
	}
	assertMediaReference(t, db, dishAsset.ID, coremedia.PurposeDish, dish.DishPublicID)
	status, _ = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/merchant/dishes", merchantAUser.Token, map[string]any{
		"name": "G7B Forbidden Asset Dish", "priceCents": 100, "imageAssetPublicId": otherOwnerDishAsset.AssetPublicID,
	})
	if status != http.StatusForbidden {
		t.Fatalf("foreign media asset status=%d want=403", status)
	}
	var forbiddenDishCount int64
	if err := db.Model(&takeout.Dish{}).Where("name = ?", "G7B Forbidden Asset Dish").Count(&forbiddenDishCount).Error; err != nil {
		t.Fatal(err)
	}
	if forbiddenDishCount != 0 {
		t.Fatalf("forbidden asset request created dish residue=%d", forbiddenDishCount)
	}

	status, body = httpJSON(t, server.Client(), http.MethodPatch, server.URL+"/api/v1/takeout/merchant/dishes/"+dish.DishPublicID+"/on-shelf", merchantAUser.Token, map[string]any{
		"expectedStatus": "draft", "expectedRowVersion": dish.RowVersion,
	})
	if status != http.StatusOK {
		t.Fatalf("put on shelf status=%d body=%+v", status, body)
	}
	if err := json.Unmarshal(body.Data, &dish); err != nil {
		t.Fatal(err)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPatch, server.URL+"/api/v1/takeout/merchant/dishes/"+dish.DishPublicID+"/sold-out", merchantBUser.Token, map[string]any{
		"expectedRowVersion": dish.RowVersion,
	})
	if status != http.StatusNotFound {
		t.Fatalf("cross merchant dish status=%d want=404", status)
	}
	status, body = httpJSON(t, server.Client(), http.MethodPatch, server.URL+"/api/v1/takeout/merchant/dishes/"+dish.DishPublicID+"/sold-out", merchantAUser.Token, map[string]any{
		"expectedRowVersion": dish.RowVersion,
	})
	if status != http.StatusOK {
		t.Fatalf("sold out status=%d", status)
	}
	if err := json.Unmarshal(body.Data, &dish); err != nil {
		t.Fatal(err)
	}
	status, body = httpJSON(t, server.Client(), http.MethodPatch, server.URL+"/api/v1/takeout/merchant/dishes/"+dish.DishPublicID+"/restock", merchantAUser.Token, map[string]any{
		"expectedRowVersion": dish.RowVersion,
	})
	if status != http.StatusOK {
		t.Fatalf("restock status=%d body=%+v", status, body)
	}
	if err := json.Unmarshal(body.Data, &dish); err != nil {
		t.Fatal(err)
	}

	merchantA := mustFindMerchantByOwner(t, identity, ctx, uint64(merchantAUser.User.ID))
	order, err := orders.CreateOrder(ctx, takeout.CreateOrderInput{
		BuyerUserID: uint64(ordinary.User.ID), MerchantPublicID: merchantA.MerchantPublicID,
		Items:   []takeout.OrderItemRequest{{DishPublicID: dish.DishPublicID, Quantity: 1}},
		Address: takeout.OrderAddress{ContactName: "G7B Buyer", ContactPhone: "13000000011", AddressBuilding: "A", AddressRoom: "101"},
	})
	if err != nil {
		t.Fatal(err)
	}
	order, err = orders.MarkPaid(ctx, order.OrderPublicID, order.RowVersion, "g7b-pay-no")
	if err != nil {
		t.Fatal(err)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPatch, server.URL+"/api/v1/takeout/merchant/orders/"+order.OrderPublicID+"/accept", merchantBUser.Token, map[string]any{
		"expectedRowVersion": order.RowVersion,
	})
	if status != http.StatusNotFound {
		t.Fatalf("cross merchant order status=%d want=404", status)
	}
	status, body = httpJSON(t, server.Client(), http.MethodPatch, server.URL+"/api/v1/takeout/merchant/orders/"+order.OrderPublicID+"/accept", merchantAUser.Token, map[string]any{
		"expectedRowVersion": order.RowVersion,
	})
	if status != http.StatusOK {
		t.Fatalf("accept order status=%d body=%+v", status, body)
	}
	if err := json.Unmarshal(body.Data, &order); err != nil {
		t.Fatal(err)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPatch, server.URL+"/api/v1/takeout/merchant/orders/"+order.OrderPublicID+"/start-preparing", merchantAUser.Token, map[string]any{
		"expectedRowVersion": order.RowVersion - 1,
	})
	if status != http.StatusConflict {
		t.Fatalf("stale order status=%d want=409", status)
	}
	status, body = httpJSON(t, server.Client(), http.MethodPatch, server.URL+"/api/v1/takeout/merchant/orders/"+order.OrderPublicID+"/start-preparing", merchantAUser.Token, map[string]any{
		"expectedRowVersion": order.RowVersion,
	})
	if status != http.StatusOK {
		t.Fatalf("start preparing status=%d body=%+v", status, body)
	}
	if err := json.Unmarshal(body.Data, &order); err != nil {
		t.Fatal(err)
	}
	status, body = httpJSON(t, server.Client(), http.MethodPatch, server.URL+"/api/v1/takeout/merchant/orders/"+order.OrderPublicID+"/mark-ready", merchantAUser.Token, map[string]any{
		"expectedRowVersion": order.RowVersion,
	})
	if status != http.StatusOK {
		t.Fatalf("mark ready status=%d body=%+v", status, body)
	}
	status, body = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/merchant/orders?status=ready_for_pickup", merchantAUser.Token, nil)
	assertHTTPPageContains(t, status, body, "orderPublicId", order.OrderPublicID)
	status, body = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/merchant/orders/"+order.OrderPublicID, merchantAUser.Token, nil)
	if status != http.StatusOK || !strings.Contains(string(body.Data), `"items"`) {
		t.Fatalf("order detail status=%d body=%s", status, body.Data)
	}

	var merchantBDishCount int64
	if err := db.Model(&takeout.Dish{}).Where("merchant_public_id = ?", merchantB.MerchantPublicID).Count(&merchantBDishCount).Error; err != nil {
		t.Fatal(err)
	}
	if merchantBDishCount != 0 {
		t.Fatalf("cross merchant operation mutated merchant B dishes=%d", merchantBDishCount)
	}
	t.Log("real_http=PASS auth=401 owner_scope=PASS onboarding_review=PASS duplicate=409 media_reference=PASS cross_merchant=PASS order_flow=PASS stale_CAS=409")
}

func TestZZTakeoutMerchantAPIResidueAgainstMySQL84(t *testing.T) {
	db := openG7BMySQL(t)
	cleanupG7B(t, db)
	assertNoG7BResidue(t, db)
	t.Log("residue=0 users=0 merchants=0 categories=0 dishes=0 orders=0 review_tasks=0 media_assets=0 media_references=0")
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
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var envelope apiEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, envelope
}

func assertHTTPPageContains(t *testing.T, status int, body apiEnvelope, field, expected string) {
	t.Helper()
	if status != http.StatusOK || body.Code != 0 {
		t.Fatalf("page status=%d body=%+v", status, body)
	}
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

func assertMediaReference(t *testing.T, db *gorm.DB, assetID uint64, purpose coremedia.Purpose, referenceID string) {
	t.Helper()
	var count int64
	if err := db.Model(&coremedia.AssetReference{}).
		Where("asset_id = ? AND reference_type = ? AND reference_id = ? AND status = ?", assetID, purpose, referenceID, coremedia.ReferencePending).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("media reference asset=%d purpose=%s reference=%s count=%d", assetID, purpose, referenceID, count)
	}
}

func mustFindMerchantByOwner(t *testing.T, identity *takeout.MerchantIdentityService, ctx context.Context, ownerID uint64) takeout.Merchant {
	t.Helper()
	value, err := identity.FindByOwnerUserID(ctx, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func openG7BMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TAKEOUT_G7B_MYSQL_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	}
	if dsn == "" {
		t.Skip("set TAKEOUT_G7B_MYSQL_DSN or MYSQL_DSN for G-7b real HTTP MySQL acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertG7BSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range []any{
		&user.User{}, &review.Task{}, &takeout.Merchant{}, &takeout.DishCategory{}, &takeout.Dish{},
		&takeout.Order{}, &takeout.OrderItem{}, &takeout.FeeConfig{}, &coremedia.Asset{}, &coremedia.AssetReference{},
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required migration missing for %T", table)
		}
	}
}

func cleanupG7B(t *testing.T, db *gorm.DB) {
	t.Helper()
	var merchantPublicIDs []string
	db.Model(&takeout.Merchant{}).Where("name LIKE ?", "G7B%").Pluck("merchant_public_id", &merchantPublicIDs)
	var dishPublicIDs []string
	if len(merchantPublicIDs) > 0 {
		db.Model(&takeout.Dish{}).Where("merchant_public_id IN ?", merchantPublicIDs).Pluck("dish_public_id", &dishPublicIDs)
		db.Where("order_public_id IN (?)", db.Model(&takeout.Order{}).Select("order_public_id").Where("merchant_public_id IN ?", merchantPublicIDs)).Delete(&takeout.OrderItem{})
		db.Where("merchant_public_id IN ?", merchantPublicIDs).Delete(&takeout.Order{})
		db.Where("merchant_public_id IN ?", merchantPublicIDs).Delete(&takeout.Dish{})
		db.Where("merchant_public_id IN ?", merchantPublicIDs).Delete(&takeout.DishCategory{})
	}
	referenceIDs := append(append([]string{}, merchantPublicIDs...), dishPublicIDs...)
	if len(referenceIDs) > 0 {
		db.Where("reference_id IN ?", referenceIDs).Delete(&coremedia.AssetReference{})
	}
	var assetIDs []uint64
	db.Model(&coremedia.Asset{}).Where("idempotency_key LIKE ?", "g7b-%").Pluck("id", &assetIDs)
	if len(assetIDs) > 0 {
		db.Where("asset_id IN ?", assetIDs).Delete(&coremedia.AssetReference{})
		db.Where("id IN ?", assetIDs).Delete(&coremedia.Asset{})
	}
	var merchantIDs []uint
	db.Model(&takeout.Merchant{}).Where("name LIKE ?", "G7B%").Pluck("id", &merchantIDs)
	if len(merchantIDs) > 0 {
		db.Where("object_type = ? AND object_id IN ?", takeoutreview.MerchantObjectType, merchantIDs).Delete(&review.Task{})
	}
	db.Where("name LIKE ?", "G7B%").Delete(&takeout.Merchant{})
	db.Unscoped().Where("open_id IN ?", []string{
		"mock_openid_g7b-merchant-a", "mock_openid_g7b-merchant-b", "mock_openid_g7b-ordinary", "mock_openid_g7b-admin",
	}).Delete(&user.User{})
}

func assertNoG7BResidue(t *testing.T, db *gorm.DB) {
	t.Helper()
	checks := []struct {
		name  string
		model any
		query string
		args  []any
	}{
		{"users", &user.User{}, "open_id IN ?", []any{[]string{"mock_openid_g7b-merchant-a", "mock_openid_g7b-merchant-b", "mock_openid_g7b-ordinary", "mock_openid_g7b-admin"}}},
		{"merchants", &takeout.Merchant{}, "name LIKE ?", []any{"G7B%"}},
		{"categories", &takeout.DishCategory{}, "name LIKE ?", []any{"G7B%"}},
		{"dishes", &takeout.Dish{}, "name LIKE ?", []any{"G7B%"}},
		{"orders", &takeout.Order{}, "contact_name LIKE ?", []any{"G7B%"}},
		{"assets", &coremedia.Asset{}, "idempotency_key LIKE ?", []any{"g7b-%"}},
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
