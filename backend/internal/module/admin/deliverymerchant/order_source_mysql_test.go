package deliverymerchant

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/user"
)

type sourceEligibility struct{}

func (sourceEligibility) RequireActive(context.Context, uint64) error { return nil }
func (sourceEligibility) RiderEligible(context.Context, uint64) (user.IdentityEligibilityResult, error) {
	return user.IdentityEligibilityResult{Available: true, Status: "active"}, nil
}
func (sourceEligibility) MerchantEligible(context.Context, uint64) (user.IdentityEligibilityResult, error) {
	return user.IdentityEligibilityResult{Available: true, Status: "active"}, nil
}

type sourceTestFees struct{ policy delivery.FeePolicy }

func (p *sourceTestFees) FeePolicyForMerchant(context.Context, uint64) (delivery.FeePolicy, error) {
	return p.policy, nil
}

func TestOrderSourceTemporaryMySQL(t *testing.T) {
	dsn := os.Getenv("DELIVERY_SOURCE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires isolated delivery-source test engine")
	}
	cfg, err := driver.ParseDSN(dsn)
	if err != nil || cfg.Net != "tcp" || cfg.Addr != "127.0.0.1:33308" || !strings.HasPrefix(cfg.DBName, "campus_delivery_source_test_") {
		t.Fatal("only the isolated delivery-source test engine is permitted")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if db.Migrator().HasColumn("delivery_orders", "source_platform") {
		t.Fatal("expected freshly reconstructed pre-045 schema")
	}
	oldID, _ := delivery.NewULID()
	if err := db.Exec(`INSERT INTO delivery_orders (delivery_order_public_id,merchant_user_id,pickup_contact_name,pickup_contact_phone,pickup_address,recipient_name,recipient_phone,delivery_address,remark,delivery_fee_cents,rider_fee_cents,platform_cut_cents,status,row_version,created_at,updated_at)
		VALUES (?,991077,'old','1','old pickup','old','2','old delivery','old remark',150,100,50,'pending_fee_payment',1,UTC_TIMESTAMP(),UTC_TIMESTAMP())`, oldID).Error; err != nil {
		t.Fatal(err)
	}
	applySourceMigration(t, db)
	applySourceMigration(t, db)
	var legacy delivery.Order
	if err := db.Where("delivery_order_public_id = ?", oldID).First(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if legacy.SourcePlatform != delivery.SourceLocal || legacy.PickupCode != "" || legacy.SourceOrderID != nil || legacy.SourceShopID != nil || legacy.Remark != "old remark" || legacy.DeliveryFeeCents != 150 {
		t.Fatal("migration changed historical business data")
	}
	t.Log("migration_045 full_baseline=PASS repeat=PASS legacy_defaults=PASS")

	ctx := context.Background()
	auth := user.NewService(user.NewRepository(db), "source-test-only-secret-thirty-two-characters", "", "", true, user.WithAppEnv("dev"), user.WithIdentityEligibility(sourceEligibility{}))
	login := func(name string, identity user.Identity) user.LoginResult {
		t.Helper()
		value, err := auth.WechatLogin(ctx, user.LoginInput{Code: "source-test-" + name, Nickname: name})
		if err != nil {
			t.Fatal(err)
		}
		if identity != user.IdentityStudent {
			if _, err := auth.SwitchIdentity(ctx, value.User.ID, identity); err != nil {
				t.Fatal(err)
			}
		}
		return value
	}
	merchant, other, student := login("merchant", user.IdentityMerchant), login("other", user.IdentityMerchant), login("student", user.IdentityStudent)
	repo := delivery.NewRepository(db)
	orders, _ := delivery.NewService(repo, nil)
	fees := &sourceTestFees{policy: delivery.FeePolicy{DeliveryFeeCents: 150, RiderFeeCents: 100, PlatformCutCents: 50}}
	dispatch, err := delivery.NewDispatchServiceWithFeeProvider(repo, orders, sourceEligibility{}, fees, nil)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/delivery/merchant", auth.AuthRequired(), auth.IdentityRequired(user.IdentityMerchant))
	NewHandler(dispatch, func(c *gin.Context) uint64 { return uint64(user.CurrentUserID(c)) }, false).Register(group)
	request := func(method, path, token string, data any) (int, map[string]any, string) {
		body, _ := json.Marshal(data)
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		var response struct {
			Data    map[string]any `json:"data"`
			Message string         `json:"message"`
		}
		_ = json.Unmarshal(recorder.Body.Bytes(), &response)
		return recorder.Code, response.Data, response.Message
	}
	for _, check := range []struct {
		token  string
		status int
	}{{"", 401}, {student.Token, 403}, {merchant.Token, 200}} {
		status, data, _ := request(http.MethodGet, "/delivery/merchant/order-intake", check.token, nil)
		if status != check.status {
			t.Fatalf("intake auth status=%d want=%d", status, check.status)
		}
		if status == 200 && (data["manualAvailable"] != true || data["externalAvailable"] != false) {
			t.Fatal("wrong intake capability")
		}
	}
	makeInput := func(orderID, shopID string) map[string]any {
		key, _ := delivery.NewULID()
		return map[string]any{"clientRequestId": key, "pickupName": "商家", "pickupPhone": "13000000001", "pickupAddress": "校门", "recipientName": "顾客", "recipientPhone": "13000000002", "deliveryAddress": "宿舍", "remark": "测试", "sourcePlatform": "taobao", "pickupCode": "0022", "sourceOrderId": orderID, "sourceShopId": shopID}
	}
	input := makeInput("Order-AbC001", "Shop-A")
	status, created, message := request(http.MethodPost, "/delivery/merchant/orders", merchant.Token, input)
	if status != 201 || created["sourcePlatform"] != "taobao" || created["pickupCode"] != "0022" || created["status"] != "pending_fee_payment" {
		t.Fatalf("create status=%d data=%v message=%s", status, created, message)
	}
	id := created["deliveryOrderPublicId"].(string)
	status, repeated, _ := request(http.MethodPost, "/delivery/merchant/orders", merchant.Token, input)
	if status != 201 || repeated["deliveryOrderPublicId"] != id {
		t.Fatal("same request did not reuse original order")
	}
	fees.policy = delivery.FeePolicy{DeliveryFeeCents: 200, RiderFeeCents: 130, PlatformCutCents: 70}
	status, repeated, _ = request(http.MethodPost, "/delivery/merchant/orders", merchant.Token, input)
	if status != 201 || repeated["deliveryFeeCents"] != float64(150) || repeated["deliveryOrderPublicId"] != id {
		t.Fatal("fee-policy change broke idempotent retry")
	}
	input["pickupCode"] = "22"
	status, _, _ = request(http.MethodPost, "/delivery/merchant/orders", merchant.Token, input)
	if status != 409 {
		t.Fatal("changed source snapshot must conflict")
	}
	status, data, message := request(http.MethodPost, "/delivery/merchant/orders", merchant.Token, makeInput("Order-AbC001", "Shop-A"))
	if status != 409 || data != nil || message != delivery.ErrSourceOrderExists.Error() {
		t.Fatalf("external duplicate status=%d data=%v message=%s", status, data, message)
	}
	status, _, _ = request(http.MethodGet, "/delivery/merchant/orders/"+id, other.Token, nil)
	if status != 404 {
		t.Fatal("cross-merchant order leaked")
	}
	status, _, _ = request(http.MethodPost, "/delivery/merchant/orders", other.Token, makeInput("Order-AbC001", "Shop-A"))
	if status != 201 {
		t.Fatal("source identifier must be merchant-scoped")
	}
	for _, ids := range [][2]string{{"order-AbC001", "Shop-A"}, {"Order-AbC001", "shop-A"}, {"Order-AbC002", "Shop-A"}, {"", ""}, {"", ""}} {
		status, _, msg := request(http.MethodPost, "/delivery/merchant/orders", merchant.Token, makeInput(ids[0], ids[1]))
		if status != 201 {
			t.Fatalf("case-sensitive or nullable identity failed %v status=%d message=%s", ids, status, msg)
		}
	}
	for _, bad := range []map[string]any{{"sourcePlatform": "invalid"}, {"pickupCode": strings.Repeat("a", 17)}, {"sourceShopId": ""}, {"sourcePlatform": "local"}} {
		invalid := makeInput("bad-order", "bad-shop")
		for key, value := range bad {
			invalid[key] = value
		}
		status, _, _ = request(http.MethodPost, "/delivery/merchant/orders", merchant.Token, invalid)
		if status != 400 {
			t.Fatalf("invalid source accepted: %v status=%d", bad, status)
		}
	}
	legacyInput := makeInput("", "")
	delete(legacyInput, "sourcePlatform")
	delete(legacyInput, "pickupCode")
	delete(legacyInput, "sourceOrderId")
	delete(legacyInput, "sourceShopId")
	status, data, _ = request(http.MethodPost, "/delivery/merchant/orders", merchant.Token, legacyInput)
	if status != 201 || data["sourcePlatform"] != "local" {
		t.Fatal("legacy request compatibility failed")
	}
	t.Log("http auth=PASS own_scope=PASS source_roundtrip=PASS duplicate_409=PASS retry_fee_snapshot=PASS")

	const competitors = 8
	results := make(chan int, competitors)
	var wait sync.WaitGroup
	for i := 0; i < competitors; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			code, _, _ := request(http.MethodPost, "/delivery/merchant/orders", merchant.Token, makeInput("Concurrent-001", "Shop-A"))
			results <- code
		}()
	}
	wait.Wait()
	close(results)
	successes, conflicts := 0, 0
	for code := range results {
		if code == 201 {
			successes++
		} else if code == 409 {
			conflicts++
		} else {
			t.Fatalf("unexpected concurrent status=%d", code)
		}
	}
	if successes != 1 || conflicts != competitors-1 {
		t.Fatalf("source race successes=%d conflicts=%d", successes, conflicts)
	}
	var orderCount, outboxCount, settlementCount int64
	for _, err := range []error{
		db.Model(&delivery.Order{}).Where("merchant_user_id = ? AND source_order_id = ?", merchant.User.ID, "Concurrent-001").Count(&orderCount).Error,
		db.Model(&delivery.OutboxEvent{}).Count(&outboxCount).Error,
		db.Model(&delivery.FeeSettlement{}).Count(&settlementCount).Error,
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	if orderCount != 1 || outboxCount != 0 || settlementCount != 0 {
		t.Fatalf("unexpected automatic dispatch/payment rows orders=%d outbox=%d settlements=%d", orderCount, outboxCount, settlementCount)
	}
	t.Log("concurrency one_order=PASS no_auto_dispatch_or_payment=PASS")
	for _, query := range []string{
		"UPDATE delivery_orders SET source_platform='invalid' WHERE delivery_order_public_id=?",
		"UPDATE delivery_orders SET source_shop_id=NULL WHERE delivery_order_public_id=?",
		"UPDATE delivery_orders SET source_platform='local' WHERE delivery_order_public_id=?",
	} {
		if err := db.Exec(query, id).Error; err == nil {
			t.Fatal("source database constraint accepted invalid data")
		}
	}
	applySourceMigration(t, db)
	t.Log("migration_045 populated_repeat=PASS database_constraints=PASS")
}

func applySourceMigration(t *testing.T, db *gorm.DB) {
	t.Helper()
	file, err := os.Open("../../../../migrations/045_delivery_order_source.sql")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	delimiter := ";"
	var statement strings.Builder
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") || trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "DELIMITER ") {
			delimiter = strings.TrimSpace(strings.TrimPrefix(trimmed, "DELIMITER "))
			continue
		}
		statement.WriteString(line)
		statement.WriteByte('\n')
		if !strings.HasSuffix(trimmed, delimiter) {
			continue
		}
		query := strings.TrimSuffix(strings.TrimSpace(statement.String()), delimiter)
		statement.Reset()
		if err := db.Exec(query).Error; err != nil {
			t.Fatal(fmt.Errorf("migration 045: %w", err))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(statement.String()) != "" {
		t.Fatal("migration 045 trailing statement")
	}
}
