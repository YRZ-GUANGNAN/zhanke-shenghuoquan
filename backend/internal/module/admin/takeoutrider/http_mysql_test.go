package takeoutrider

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
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

func TestTakeoutRiderAPIRealHTTPMySQLConcurrencyOwnerWalletAndResidue(t *testing.T) {
	db := openG7CMySQL(t)
	assertG7CSchema(t, db)
	cleanupG7C(t, db)
	t.Cleanup(func() { cleanupG7C(t, db) })

	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	t.Logf("real_mysql_version=%s", version)

	ctx := context.Background()
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	repository := takeout.NewRepository(db)
	merchants, _ := takeout.NewMerchantService(repository, func() time.Time { return now })
	catalog, _ := takeout.NewCatalogService(repository, func() time.Time { return now })
	orders, _ := takeout.NewOrderService(repository, repository, func() time.Time { return now })
	riderOrderOps, _ := takeout.NewRiderOrderOps(repository, func() time.Time { return now })
	riders, _ := takeout.NewRiderService(repository, riderOrderOps, func() time.Time { return now })
	withdraws, _ := takeout.NewWithdrawService(repository, func() time.Time { return now })
	transport, err := NewService(riders, orders, withdraws, NewRepository(db))
	if err != nil {
		t.Fatal(err)
	}
	auth := user.NewService(user.NewRepository(db), "g7c-test-secret-that-is-longer-than-thirty-two-characters", "", "", true)

	riderA, err := auth.WechatLogin(ctx, user.LoginInput{Code: "g7c-rider-a", Nickname: "G7C Rider A"})
	if err != nil {
		t.Fatal(err)
	}
	riderB, err := auth.WechatLogin(ctx, user.LoginInput{Code: "g7c-rider-b", Nickname: "G7C Rider B"})
	if err != nil {
		t.Fatal(err)
	}
	buyer, err := auth.WechatLogin(ctx, user.LoginInput{Code: "g7c-buyer", Nickname: "G7C Buyer"})
	if err != nil {
		t.Fatal(err)
	}
	merchantUser, err := auth.WechatLogin(ctx, user.LoginInput{Code: "g7c-merchant", Nickname: "G7C Merchant"})
	if err != nil {
		t.Fatal(err)
	}

	originalConfig, err := riders.GetRiderConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	updatedConfig, err := riders.UpdateRiderConfig(ctx, 1, originalConfig.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		latest, findErr := riders.GetRiderConfig(context.Background())
		if findErr == nil && latest.MaxInFlightPerRider != originalConfig.MaxInFlightPerRider {
			_, _ = riders.UpdateRiderConfig(context.Background(), originalConfig.MaxInFlightPerRider, latest.RowVersion)
		}
	})
	if updatedConfig.MaxInFlightPerRider != 1 {
		t.Fatalf("failed to set in-flight limit: %+v", updatedConfig)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	routes := router.Group("/api/v1/takeout/rider", auth.AuthRequired())
	NewHandler(transport, func(c *gin.Context) uint64 { return uint64(user.CurrentUserID(c)) }).Register(routes)
	server := httptest.NewServer(router)
	defer server.Close()

	status, _ := httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/rider/me", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("no token status=%d want=401", status)
	}
	status, body := httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/rider/me", riderA.Token, nil)
	if status != http.StatusOK || !strings.Contains(string(body.Data), `"registered":false`) {
		t.Fatalf("initial me status=%d body=%s", status, body.Data)
	}
	status, body = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/rider/register", riderA.Token, map[string]any{
		"realName": "G7C Rider A", "phone": "13100000001",
	})
	if status != http.StatusCreated || !strings.Contains(string(body.Data), `"status":"active"`) {
		t.Fatalf("register A status=%d body=%s", status, body.Data)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/rider/register", riderA.Token, map[string]any{
		"realName": "G7C Rider A Duplicate", "phone": "13100000001",
	})
	if status != http.StatusCreated {
		t.Fatalf("duplicate register status=%d want idempotent 201", status)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/rider/register", riderB.Token, map[string]any{
		"realName": "G7C Rider B", "phone": "13100000002",
	})
	if status != http.StatusCreated {
		t.Fatalf("register B status=%d", status)
	}

	merchant, dish := seedG7CMerchantDish(t, ctx, merchants, catalog, uint64(merchantUser.User.ID))
	firstOrder := seedG7CReadyOrder(t, ctx, orders, uint64(buyer.User.ID), merchant.MerchantPublicID, dish.DishPublicID, "G7C Buyer One")

	status, body = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/rider/claimable-orders", riderA.Token, nil)
	if status != http.StatusOK || !strings.Contains(string(body.Data), firstOrder.OrderPublicID) ||
		!strings.Contains(string(body.Data), `"merchantName":"G7C Merchant"`) {
		t.Fatalf("claimable list status=%d body=%s", status, body.Data)
	}

	type claimResult struct {
		status int
		body   apiEnvelope
		token  string
	}
	claims := make(chan claimResult, 2)
	var wg sync.WaitGroup
	for _, token := range []string{riderA.Token, riderB.Token} {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			status, body := httpJSON(t, server.Client(), http.MethodPost,
				server.URL+"/api/v1/takeout/rider/orders/"+firstOrder.OrderPublicID+"/claim",
				token, map[string]any{"expectedRowVersion": firstOrder.RowVersion})
			claims <- claimResult{status: status, body: body, token: token}
		}(token)
	}
	wg.Wait()
	close(claims)
	var success claimResult
	successCount, conflictCount := 0, 0
	for result := range claims {
		switch result.status {
		case http.StatusOK:
			success = result
			successCount++
		case http.StatusConflict:
			conflictCount++
		default:
			t.Fatalf("unexpected concurrent claim status=%d body=%+v", result.status, result.body)
		}
	}
	if successCount != 1 || conflictCount != 1 {
		t.Fatalf("concurrent claim success=%d conflict=%d want 1/1", successCount, conflictCount)
	}
	var claimed FulfillmentDetailView
	if err := json.Unmarshal(success.body.Data, &claimed); err != nil {
		t.Fatal(err)
	}
	var fulfillmentCount int64
	if err := db.Model(&takeout.Fulfillment{}).Where("order_public_id = ?", firstOrder.OrderPublicID).Count(&fulfillmentCount).Error; err != nil {
		t.Fatal(err)
	}
	if fulfillmentCount != 1 || claimed.Order.Status != takeout.OrderDelivering || claimed.Fulfillment.Status != takeout.FulfillmentAssigned {
		t.Fatalf("claim did not assign exactly once: count=%d claimed=%+v", fulfillmentCount, claimed)
	}
	winnerToken := success.token
	loserToken := riderA.Token
	if winnerToken == riderA.Token {
		loserToken = riderB.Token
	}

	secondOrder := seedG7CReadyOrder(t, ctx, orders, uint64(buyer.User.ID), merchant.MerchantPublicID, dish.DishPublicID, "G7C Buyer Two")
	status, _ = httpJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/takeout/rider/orders/"+secondOrder.OrderPublicID+"/claim",
		winnerToken, map[string]any{"expectedRowVersion": secondOrder.RowVersion})
	if status != http.StatusConflict {
		t.Fatalf("in-flight limit claim status=%d want=409", status)
	}

	status, _ = httpJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/v1/takeout/rider/fulfillments/"+claimed.Fulfillment.FulfillmentPublicID,
		loserToken, nil)
	if status != http.StatusNotFound {
		t.Fatalf("cross-rider fulfillment detail status=%d want=404", status)
	}
	status, body = httpJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/v1/takeout/rider/fulfillments",
		loserToken, nil)
	if status != http.StatusOK || strings.Contains(string(body.Data), claimed.Fulfillment.FulfillmentPublicID) {
		t.Fatalf("cross-rider fulfillment list leaked status=%d body=%s", status, body.Data)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPatch,
		server.URL+"/api/v1/takeout/rider/fulfillments/"+claimed.Fulfillment.FulfillmentPublicID+"/to-store",
		loserToken, map[string]any{"expectedRowVersion": claimed.Fulfillment.RowVersion})
	if status != http.StatusNotFound {
		t.Fatalf("cross-rider transition status=%d want=404", status)
	}

	current := claimed.Fulfillment
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
			server.URL+"/api/v1/takeout/rider/fulfillments/"+current.FulfillmentPublicID+step.path,
			winnerToken, map[string]any{"expectedRowVersion": current.RowVersion})
		if status != http.StatusOK {
			t.Fatalf("transition %s status=%d body=%+v", step.path, status, body)
		}
		var detail FulfillmentDetailView
		if err := json.Unmarshal(body.Data, &detail); err != nil {
			t.Fatal(err)
		}
		if detail.Fulfillment.Status != step.want {
			t.Fatalf("transition %s status=%s want=%s", step.path, detail.Fulfillment.Status, step.want)
		}
		current = detail.Fulfillment
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPatch,
		server.URL+"/api/v1/takeout/rider/fulfillments/"+current.FulfillmentPublicID+"/complete",
		winnerToken, map[string]any{"expectedRowVersion": current.RowVersion})
	if status != http.StatusOK {
		t.Fatalf("repeat complete status=%d want idempotent 200", status)
	}
	var earningCount int64
	if err := db.Model(&takeout.RiderEarningRecord{}).Where("order_public_id = ?", firstOrder.OrderPublicID).Count(&earningCount).Error; err != nil {
		t.Fatal(err)
	}
	if earningCount != 1 {
		t.Fatalf("earning_count=%d want=1", earningCount)
	}
	earning, err := orders.FindEarningByOrder(ctx, firstOrder.OrderPublicID)
	if err != nil {
		t.Fatal(err)
	}
	status, body = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/rider/wallet", winnerToken, nil)
	if status != http.StatusOK || !strings.Contains(string(body.Data), `"availableWithdrawCents":`) ||
		!strings.Contains(string(body.Data), earning.EarningPublicID) {
		t.Fatalf("wallet status=%d body=%s", status, body.Data)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/rider/withdrawals", winnerToken,
		map[string]any{"amountCents": earning.RiderFeeCents + 1})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("over-balance withdrawal status=%d want=422", status)
	}
	status, _ = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/rider/withdrawals", winnerToken,
		map[string]any{"amountCents": uint64(0)})
	if status != http.StatusBadRequest {
		t.Fatalf("zero withdrawal status=%d want=400", status)
	}
	status, body = httpJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/takeout/rider/withdrawals", winnerToken,
		map[string]any{"amountCents": earning.RiderFeeCents})
	if status != http.StatusCreated || !strings.Contains(string(body.Data), `"status":"pending"`) {
		t.Fatalf("valid withdrawal status=%d body=%s", status, body.Data)
	}
	var withdraw WithdrawView
	if err := json.Unmarshal(body.Data, &withdraw); err != nil {
		t.Fatal(err)
	}
	status, body = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/rider/withdrawals", winnerToken, nil)
	if status != http.StatusOK || !strings.Contains(string(body.Data), withdraw.WithdrawPublicID) {
		t.Fatalf("winner withdrawals status=%d body=%s", status, body.Data)
	}
	status, body = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/rider/withdrawals", loserToken, nil)
	if status != http.StatusOK || strings.Contains(string(body.Data), withdraw.WithdrawPublicID) {
		t.Fatalf("withdrawal leaked to loser status=%d body=%s", status, body.Data)
	}
	status, body = httpJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/takeout/rider/wallet", winnerToken, nil)
	if status != http.StatusOK || !strings.Contains(string(body.Data), `"availableWithdrawCents":0`) {
		t.Fatalf("wallet after pending withdraw status=%d body=%s", status, body.Data)
	}
	t.Log("real_http=PASS rider_register=PASS concurrent_claim_200_409=PASS in_flight_limit=PASS fulfillment_owner_scope=PASS delivery_completion_earning_idempotent=PASS wallet_balance_guard=PASS withdrawals_owner_scope=PASS")
}

func seedG7CMerchantDish(
	t *testing.T,
	ctx context.Context,
	merchants *takeout.MerchantService,
	catalog *takeout.CatalogService,
	ownerID uint64,
) (takeout.Merchant, takeout.Dish) {
	t.Helper()
	merchant, err := merchants.CreateMerchant(ctx, takeout.CreateMerchantInput{
		OwnerUserID: ownerID, Name: "G7C Merchant", ContactPhone: "13200000000",
		Location: "G7C Gate", BusinessHours: "09:00-21:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	merchant, err = merchants.Approve(ctx, merchant.MerchantPublicID, merchant.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	category, err := catalog.CreateCategory(ctx, takeout.CreateCategoryInput{MerchantPublicID: merchant.MerchantPublicID, Name: "G7C Category"})
	if err != nil {
		t.Fatal(err)
	}
	dish, err := catalog.CreateDish(ctx, takeout.CreateDishInput{
		MerchantPublicID: merchant.MerchantPublicID, CategoryPublicID: &category.CategoryPublicID,
		Name: "G7C Dish", PriceCents: 1500,
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

func seedG7CReadyOrder(
	t *testing.T,
	ctx context.Context,
	orders *takeout.OrderService,
	buyerUserID uint64,
	merchantPublicID string,
	dishPublicID string,
	contactName string,
) takeout.Order {
	t.Helper()
	order, err := orders.CreateOrder(ctx, takeout.CreateOrderInput{
		BuyerUserID: buyerUserID, MerchantPublicID: merchantPublicID,
		Items: []takeout.OrderItemRequest{{DishPublicID: dishPublicID, Quantity: 1}},
		Address: takeout.OrderAddress{
			ContactName: contactName, ContactPhone: "13300000000",
			AddressBuilding: "G7C Building", AddressRoom: "G7C Room",
		},
		Remark: "G7C ready order",
	})
	if err != nil {
		t.Fatal(err)
	}
	order, err = orders.MarkPaid(ctx, order.OrderPublicID, order.RowVersion, "G7C-PAY-"+order.OrderPublicID)
	if err != nil {
		t.Fatal(err)
	}
	order, err = orders.Accept(ctx, order.OrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	order, err = orders.StartPreparing(ctx, order.OrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	order, err = orders.MarkReady(ctx, order.OrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	return order
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

func openG7CMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TAKEOUT_G7C_MYSQL_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	}
	if dsn == "" {
		t.Skip("set TAKEOUT_G7C_MYSQL_DSN or MYSQL_DSN for G-7c rider real HTTP MySQL acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertG7CSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range []any{
		&user.User{}, &takeout.Merchant{}, &takeout.DishCategory{}, &takeout.Dish{},
		&takeout.Order{}, &takeout.OrderItem{}, &takeout.FeeConfig{},
		&takeout.Rider{}, &takeout.Fulfillment{}, &takeout.RiderConfig{},
		&takeout.RiderEarningRecord{}, &takeout.WithdrawRequest{},
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required migration missing for %T", table)
		}
	}
}

func cleanupG7C(t *testing.T, db *gorm.DB) {
	t.Helper()
	var merchantPublicIDs []string
	db.Model(&takeout.Merchant{}).Where("name LIKE ?", "G7C%").Pluck("merchant_public_id", &merchantPublicIDs)
	var orderPublicIDs []string
	orderQuery := db.Model(&takeout.Order{}).Where("contact_name LIKE ? OR remark LIKE ?", "G7C%", "G7C%")
	if len(merchantPublicIDs) > 0 {
		orderQuery = orderQuery.Or("merchant_public_id IN ?", merchantPublicIDs)
	}
	orderQuery.Pluck("order_public_id", &orderPublicIDs)
	if len(orderPublicIDs) > 0 {
		db.Where("order_public_id IN ?", orderPublicIDs).Delete(&takeout.RiderEarningRecord{})
		db.Where("order_public_id IN ?", orderPublicIDs).Delete(&takeout.Fulfillment{})
		db.Where("order_public_id IN ?", orderPublicIDs).Delete(&takeout.OrderItem{})
		db.Where("order_public_id IN ?", orderPublicIDs).Delete(&takeout.Order{})
	}
	if len(merchantPublicIDs) > 0 {
		db.Where("merchant_public_id IN ?", merchantPublicIDs).Delete(&takeout.Dish{})
		db.Where("merchant_public_id IN ?", merchantPublicIDs).Delete(&takeout.DishCategory{})
	}
	db.Where("name LIKE ?", "G7C%").Delete(&takeout.Merchant{})
	var userIDs []uint
	db.Model(&user.User{}).Where("open_id LIKE ?", "mock_openid_g7c-%").Pluck("id", &userIDs)
	if len(userIDs) > 0 {
		db.Where("rider_user_id IN ?", userIDs).Delete(&takeout.WithdrawRequest{})
		db.Where("rider_user_id IN ?", userIDs).Delete(&takeout.RiderEarningRecord{})
		db.Where("rider_user_id IN ?", userIDs).Delete(&takeout.Fulfillment{})
		db.Where("rider_user_id IN ?", userIDs).Delete(&takeout.Rider{})
	}
	db.Unscoped().Where("open_id LIKE ?", "mock_openid_g7c-%").Delete(&user.User{})
}

func TestZZTakeoutRiderHTTPResidueAgainstMySQL84(t *testing.T) {
	db := openG7CMySQL(t)
	cleanupG7C(t, db)
	checks := []struct {
		name  string
		model any
		query string
		args  []any
	}{
		{"users", &user.User{}, "open_id LIKE ?", []any{"mock_openid_g7c-%"}},
		{"riders", &takeout.Rider{}, "real_name LIKE ?", []any{"G7C%"}},
		{"merchants", &takeout.Merchant{}, "name LIKE ?", []any{"G7C%"}},
		{"categories", &takeout.DishCategory{}, "name LIKE ?", []any{"G7C%"}},
		{"dishes", &takeout.Dish{}, "name LIKE ?", []any{"G7C%"}},
		{"orders", &takeout.Order{}, "contact_name LIKE ? OR remark LIKE ?", []any{"G7C%", "G7C%"}},
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
