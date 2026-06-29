package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/admin/deliverynotify"
	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/user"
)

const d2Fence = "d2-delivery-dispatch"

type d2Envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type failingDeliverySender struct{}

func (failingDeliverySender) Enabled() bool { return true }
func (failingDeliverySender) Send(context.Context, delivery.OutboxEvent, delivery.RiderPoolOpenedPayload) error {
	return errors.New("synthetic webhook failure containing no URL")
}

func TestD2DeliveryRealHTTPMySQLAuthConcurrencyOutboxAndGates(t *testing.T) {
	db := openD2ServerMySQL(t)
	assertD2ServerSchema(t, db)
	cleanupD2Server(t, db)
	t.Cleanup(func() { cleanupD2Server(t, db) })

	ctx := context.Background()
	auth := user.NewService(
		user.NewRepository(db), "d2-test-secret-that-is-longer-than-thirty-two-characters",
		"", "", true, user.WithAppEnv("dev"),
	)
	merchant, err := auth.WechatLogin(ctx, user.LoginInput{Code: d2Fence + "-merchant", Nickname: "D2 Merchant"})
	if err != nil {
		t.Fatal(err)
	}
	otherMerchant, err := auth.WechatLogin(ctx, user.LoginInput{Code: d2Fence + "-other", Nickname: "D2 Other"})
	if err != nil {
		t.Fatal(err)
	}
	riderA, err := auth.WechatLogin(ctx, user.LoginInput{Code: d2Fence + "-rider-a", Nickname: "D2 Rider A"})
	if err != nil {
		t.Fatal(err)
	}
	riderB, err := auth.WechatLogin(ctx, user.LoginInput{Code: d2Fence + "-rider-b", Nickname: "D2 Rider B"})
	if err != nil {
		t.Fatal(err)
	}
	unregistered, err := auth.WechatLogin(ctx, user.LoginInput{Code: d2Fence + "-unregistered", Nickname: "D2 Unregistered"})
	if err != nil {
		t.Fatal(err)
	}
	administrator, err := auth.WechatLogin(ctx, user.LoginInput{Code: d2Fence + "-admin", Nickname: "D2 Admin"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&user.User{}).Where("id = ?", administrator.User.ID).
		Update("role", user.RoleAdmin).Error; err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		AppEnv: "dev", EnableDelivery: true, AllowMockDeliveryFeeConfirmation: true,
		WeComRiderWebhookURL: "",
	}
	runtime, err := newDeliveryRuntime(cfg, db)
	if err != nil {
		t.Fatal(err)
	}
	if runtime == nil || !runtime.allowMockConfirm {
		t.Fatal("enabled delivery runtime was not constructed")
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	registerDeliveryRoutes(runtime, api, auth)
	server := httptest.NewServer(router)
	defer server.Close()

	status, _ := d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/admin/delivery/riders", "", map[string]any{
			"riderUserId": riderA.User.ID, "realName": "Rider A", "phone": "13100002001",
		})
	if status != http.StatusUnauthorized {
		t.Fatalf("admin rider open without token status=%d", status)
	}
	status, _ = d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/admin/delivery/riders", merchant.Token, map[string]any{
			"riderUserId": riderA.User.ID, "realName": "Rider A", "phone": "13100002001",
		})
	if status != http.StatusForbidden {
		t.Fatalf("ordinary rider open status=%d", status)
	}
	status, _ = d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/admin/delivery/riders", administrator.Token, map[string]any{
			"riderUserId": uint64(9_999_999_999), "realName": "Missing Rider", "phone": "13100009999",
		})
	if status != http.StatusNotFound {
		t.Fatalf("missing user rider open status=%d", status)
	}
	for _, rider := range []struct {
		id    uint
		name  string
		phone string
	}{
		{riderA.User.ID, "Rider A", "13100002001"},
		{riderB.User.ID, "Rider B", "13100002002"},
	} {
		status, body := d2HTTPJSON(t, server.Client(), http.MethodPost,
			server.URL+"/api/v1/admin/delivery/riders", administrator.Token, map[string]any{
				"riderUserId": rider.id, "realName": rider.name, "phone": rider.phone,
			})
		if status != http.StatusCreated || !strings.Contains(string(body.Data), `"status":"active"`) {
			t.Fatalf("admin rider open status=%d body=%s", status, body.Data)
		}
	}
	status, body := d2HTTPJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/v1/delivery/rider/me", unregistered.Token, nil)
	if status != http.StatusOK || !strings.Contains(string(body.Data), `"registered":false`) ||
		!strings.Contains(string(body.Data), "请联系管理员开通") {
		t.Fatalf("unregistered me status=%d body=%s", status, body.Data)
	}
	status, _ = d2HTTPJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/v1/delivery/rider/orders/claimable", unregistered.Token, nil)
	if status != http.StatusForbidden {
		t.Fatalf("unregistered claim pool status=%d", status)
	}

	requestID := "01J00000000000000000000901"
	privateValues := []string{
		"Merchant Private", "13100002011", "Private Shop Room 101",
		"Recipient Private", "13100002012", "Private Dorm Room 202", "private remark",
	}
	createPayload := map[string]any{
		"clientRequestId": requestID,
		"merchantUserId":  999999,
		"pickupName":      privateValues[0], "pickupPhone": privateValues[1],
		"pickupAddress": privateValues[2], "recipientName": privateValues[3],
		"recipientPhone": privateValues[4], "deliveryAddress": privateValues[5],
		"remark": privateValues[6], "deliveryFeeCents": 1,
	}
	status, body = d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/delivery/merchant/orders", merchant.Token, createPayload)
	if status != http.StatusCreated {
		t.Fatalf("create delivery status=%d body=%s message=%s", status, body.Data, body.Message)
	}
	var created struct {
		DeliveryOrderPublicID string          `json:"deliveryOrderPublicId"`
		MerchantUserID        uint64          `json:"merchantUserId"`
		DeliveryFeeCents      uint64          `json:"deliveryFeeCents"`
		RiderFeeCents         uint64          `json:"riderFeeCents"`
		PlatformCutCents      uint64          `json:"platformCutCents"`
		Status                delivery.Status `json:"status"`
		RowVersion            uint64          `json:"rowVersion"`
	}
	if err := json.Unmarshal(body.Data, &created); err != nil {
		t.Fatal(err)
	}
	if created.MerchantUserID != uint64(merchant.User.ID) || created.DeliveryFeeCents != 150 ||
		created.RiderFeeCents != 100 || created.PlatformCutCents != 50 ||
		created.Status != delivery.StatusPendingFeePayment {
		t.Fatalf("server-owned create fields=%#v", created)
	}
	assertD2AllStageTimesNil(t, body.Data)
	status, repeatedBody := d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/delivery/merchant/orders", merchant.Token, createPayload)
	if status != http.StatusCreated || !strings.Contains(string(repeatedBody.Data), created.DeliveryOrderPublicID) {
		t.Fatalf("idempotent HTTP create status=%d body=%s", status, repeatedBody.Data)
	}
	status, _ = d2HTTPJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/v1/delivery/merchant/orders/"+created.DeliveryOrderPublicID,
		otherMerchant.Token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("merchant owner fence status=%d", status)
	}

	status, body = d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/delivery/merchant/orders/"+created.DeliveryOrderPublicID+"/mock-confirm-fee",
		merchant.Token, map[string]any{"expectedRowVersion": created.RowVersion})
	if status != http.StatusOK {
		t.Fatalf("mock confirm status=%d body=%s message=%s", status, body.Data, body.Message)
	}
	var ready struct {
		Status     delivery.Status `json:"status"`
		RowVersion uint64          `json:"rowVersion"`
	}
	if err := json.Unmarshal(body.Data, &ready); err != nil {
		t.Fatal(err)
	}
	if ready.Status != delivery.StatusReadyForClaim {
		t.Fatalf("mock confirmation state=%#v", ready)
	}
	status, body = d2HTTPJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/v1/delivery/merchant/orders/"+created.DeliveryOrderPublicID,
		merchant.Token, nil)
	if status != http.StatusOK {
		t.Fatalf("merchant ready detail status=%d body=%s", status, body.Data)
	}
	readyTimes := decodeD2StageTimes(t, body.Data)
	if readyTimes.ReadyForClaimAt == nil {
		t.Fatalf("ready stage timestamp not projected: %s", body.Data)
	}
	assertD2LaterStageTimesNil(t, readyTimes)
	var event delivery.OutboxEvent
	if err := db.Where("aggregate_public_id = ?", created.DeliveryOrderPublicID).First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.AggregateVersion != ready.RowVersion {
		t.Fatalf("outbox version=%d ready version=%d", event.AggregateVersion, ready.RowVersion)
	}
	for _, private := range privateValues {
		if strings.Contains(string(event.Payload), private) {
			t.Fatalf("outbox leaked private value %q: %s", private, event.Payload)
		}
	}
	result, err := runtime.notificationConsumer.ConsumeBatch(ctx, 10)
	if err != nil || result.Skipped != 1 {
		t.Fatalf("unconfigured webhook result=%#v err=%v", result, err)
	}
	if err := db.Where("event_public_id = ?", event.EventPublicID).First(&event).Error; err != nil ||
		event.Status != delivery.OutboxSkipped {
		t.Fatalf("skipped outbox=%#v err=%v", event, err)
	}

	status, body = d2HTTPJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/v1/delivery/rider/orders/claimable", riderA.Token, nil)
	if status != http.StatusOK || !strings.Contains(string(body.Data), created.DeliveryOrderPublicID) {
		t.Fatalf("claim pool status=%d body=%s", status, body.Data)
	}
	for _, private := range privateValues {
		if strings.Contains(string(body.Data), private) {
			t.Fatalf("claim pool leaked private value %q: %s", private, body.Data)
		}
	}

	start := make(chan struct{})
	type claimResult struct {
		status int
		body   d2Envelope
		rider  user.LoginResult
	}
	results := make(chan claimResult, 2)
	var wait sync.WaitGroup
	for _, rider := range []user.LoginResult{riderA, riderB} {
		wait.Add(1)
		go func(value user.LoginResult) {
			defer wait.Done()
			<-start
			status, response := d2HTTPJSON(t, server.Client(), http.MethodPost,
				server.URL+"/api/v1/delivery/rider/orders/"+created.DeliveryOrderPublicID+"/claim",
				value.Token, map[string]any{"expectedRowVersion": ready.RowVersion})
			results <- claimResult{status: status, body: response, rider: value}
		}(rider)
	}
	close(start)
	wait.Wait()
	close(results)
	var winner user.LoginResult
	var winnerOrder struct {
		RiderUserID uint64 `json:"riderUserId"`
		RowVersion  uint64 `json:"rowVersion"`
	}
	var successes, conflicts int
	for result := range results {
		switch result.status {
		case http.StatusOK:
			successes++
			winner = result.rider
			if err := json.Unmarshal(result.body.Data, &winnerOrder); err != nil {
				t.Fatal(err)
			}
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("claim status=%d body=%s", result.status, result.body.Data)
		}
	}
	if successes != 1 || conflicts != 1 || winnerOrder.RiderUserID != uint64(winner.User.ID) {
		t.Fatalf("claim successes=%d conflicts=%d winner=%d payload=%#v",
			successes, conflicts, winner.User.ID, winnerOrder)
	}
	loser := riderA
	if winner.User.ID == riderA.User.ID {
		loser = riderB
	}
	status, _ = d2HTTPJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/v1/delivery/rider/orders/"+created.DeliveryOrderPublicID,
		loser.Token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("non-owner rider detail status=%d", status)
	}
	status, _ = d2HTTPJSON(t, server.Client(), http.MethodPatch,
		server.URL+"/api/v1/delivery/rider/orders/"+created.DeliveryOrderPublicID+"/to-pickup",
		winner.Token, map[string]any{"expectedRowVersion": ready.RowVersion})
	if status != http.StatusConflict {
		t.Fatalf("stale rider transition status=%d", status)
	}
	version := winnerOrder.RowVersion
	for _, path := range []string{"to-pickup", "picked-up", "delivering", "delivered"} {
		status, body = d2HTTPJSON(t, server.Client(), http.MethodPatch,
			server.URL+"/api/v1/delivery/rider/orders/"+created.DeliveryOrderPublicID+"/"+path,
			winner.Token, map[string]any{"expectedRowVersion": version})
		if status != http.StatusOK {
			t.Fatalf("%s status=%d body=%s message=%s", path, status, body.Data, body.Message)
		}
		if err := json.Unmarshal(body.Data, &winnerOrder); err != nil {
			t.Fatal(err)
		}
		version = winnerOrder.RowVersion
	}
	for _, endpoint := range []struct {
		name  string
		url   string
		token string
	}{
		{
			name:  "merchant",
			url:   server.URL + "/api/v1/delivery/merchant/orders/" + created.DeliveryOrderPublicID,
			token: merchant.Token,
		},
		{
			name:  "assigned rider",
			url:   server.URL + "/api/v1/delivery/rider/orders/" + created.DeliveryOrderPublicID,
			token: winner.Token,
		},
	} {
		status, body = d2HTTPJSON(t, server.Client(), http.MethodGet, endpoint.url, endpoint.token, nil)
		if status != http.StatusOK {
			t.Fatalf("%s delivered detail status=%d body=%s", endpoint.name, status, body.Data)
		}
		times := decodeD2StageTimes(t, body.Data)
		if times.ReadyForClaimAt == nil || times.ClaimedAt == nil || times.ToPickupAt == nil ||
			times.PickedUpAt == nil || times.DeliveringAt == nil || times.DeliveredAt == nil {
			t.Fatalf("%s missing completed timeline: %s", endpoint.name, body.Data)
		}
		if times.PaymentFailedAt != nil || times.CancelledAt != nil {
			t.Fatalf("%s unexpected terminal timestamp: %s", endpoint.name, body.Data)
		}
	}

	second := createD2HTTPOrder(t, server, merchant, "01J00000000000000000000902")
	status, body = d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/delivery/merchant/orders/"+second.DeliveryOrderPublicID+"/mock-confirm-fee",
		merchant.Token, map[string]any{"expectedRowVersion": second.RowVersion})
	if status != http.StatusOK {
		t.Fatalf("second confirm status=%d", status)
	}
	failingConsumer, err := deliverynotify.NewConsumer(
		delivery.NewRepository(db), failingDeliverySender{},
		deliverynotify.ConsumerConfig{RetryDelay: time.Second, MaxAttempts: 2},
	)
	if err != nil {
		t.Fatal(err)
	}
	failedBatch, err := failingConsumer.ConsumeBatch(ctx, 1)
	if err != nil || failedBatch.Retried != 1 {
		t.Fatalf("failed webhook batch=%#v err=%v", failedBatch, err)
	}
	var stillReady delivery.Order
	if err := db.Where("delivery_order_public_id = ?", second.DeliveryOrderPublicID).First(&stillReady).Error; err != nil ||
		stillReady.Status != delivery.StatusReadyForClaim {
		t.Fatalf("webhook failure rolled back order=%#v err=%v", stillReady, err)
	}

	disabledMock := *runtime
	disabledMock.allowMockConfirm = false
	disabledRouter := gin.New()
	registerDeliveryRoutes(&disabledMock, disabledRouter.Group("/api/v1"), auth)
	disabledServer := httptest.NewServer(disabledRouter)
	defer disabledServer.Close()
	status, _ = d2HTTPJSON(t, disabledServer.Client(), http.MethodPost,
		disabledServer.URL+"/api/v1/delivery/merchant/orders/"+created.DeliveryOrderPublicID+"/mock-confirm-fee",
		merchant.Token, map[string]any{"expectedRowVersion": 1})
	if status != http.StatusNotFound {
		t.Fatalf("disabled mock endpoint status=%d", status)
	}

	disabledRuntime, err := newDeliveryRuntime(config.Config{EnableDelivery: false}, nil)
	if err != nil || disabledRuntime != nil {
		t.Fatalf("disabled runtime=%#v err=%v", disabledRuntime, err)
	}
	disabledAllRouter := gin.New()
	registerDeliveryRoutes(nil, disabledAllRouter.Group("/api/v1"), nil)
	disabledAllServer := httptest.NewServer(disabledAllRouter)
	defer disabledAllServer.Close()
	status, _ = d2HTTPJSON(t, disabledAllServer.Client(), http.MethodGet,
		disabledAllServer.URL+"/api/v1/delivery/rider/me", riderA.Token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("disabled delivery route status=%d", status)
	}

	t.Logf("d2_http=PASS admin_required+owner_fence+fees+idempotency+claim_1v1+delivery+outbox+gates winner=%d",
		winner.User.ID)
}

type d2CreatedOrder struct {
	DeliveryOrderPublicID string `json:"deliveryOrderPublicId"`
	RowVersion            uint64 `json:"rowVersion"`
}

type d2StageTimes struct {
	ReadyForClaimAt *time.Time `json:"readyForClaimAt"`
	ClaimedAt       *time.Time `json:"claimedAt"`
	ToPickupAt      *time.Time `json:"toPickupAt"`
	PickedUpAt      *time.Time `json:"pickedUpAt"`
	DeliveringAt    *time.Time `json:"deliveringAt"`
	DeliveredAt     *time.Time `json:"deliveredAt"`
	PaymentFailedAt *time.Time `json:"paymentFailedAt"`
	CancelledAt     *time.Time `json:"cancelledAt"`
}

func decodeD2StageTimes(t *testing.T, raw json.RawMessage) d2StageTimes {
	t.Helper()
	var value d2StageTimes
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func assertD2AllStageTimesNil(t *testing.T, raw json.RawMessage) {
	t.Helper()
	value := decodeD2StageTimes(t, raw)
	if value.ReadyForClaimAt != nil || value.ClaimedAt != nil || value.ToPickupAt != nil ||
		value.PickedUpAt != nil || value.DeliveringAt != nil || value.DeliveredAt != nil ||
		value.PaymentFailedAt != nil || value.CancelledAt != nil {
		t.Fatalf("new order has unexpected stage timestamp: %s", raw)
	}
	for _, field := range []string{
		`"readyForClaimAt":null`, `"claimedAt":null`, `"toPickupAt":null`, `"pickedUpAt":null`,
		`"deliveringAt":null`, `"deliveredAt":null`, `"paymentFailedAt":null`, `"cancelledAt":null`,
	} {
		if !strings.Contains(string(raw), field) {
			t.Fatalf("missing nullable stage field %s in %s", field, raw)
		}
	}
}

func assertD2LaterStageTimesNil(t *testing.T, value d2StageTimes) {
	t.Helper()
	if value.ClaimedAt != nil || value.ToPickupAt != nil || value.PickedUpAt != nil ||
		value.DeliveringAt != nil || value.DeliveredAt != nil ||
		value.PaymentFailedAt != nil || value.CancelledAt != nil {
		t.Fatalf("ready order has unexpected later stage timestamp: %#v", value)
	}
}

func createD2HTTPOrder(t *testing.T, server *httptest.Server, merchant user.LoginResult, requestID string) d2CreatedOrder {
	t.Helper()
	status, body := d2HTTPJSON(t, server.Client(), http.MethodPost,
		server.URL+"/api/v1/delivery/merchant/orders", merchant.Token, map[string]any{
			"clientRequestId": requestID,
			"pickupName":      "M", "pickupPhone": "1", "pickupAddress": "A",
			"recipientName": "R", "recipientPhone": "2", "deliveryAddress": "B",
		})
	if status != http.StatusCreated {
		t.Fatalf("create helper status=%d body=%s", status, body.Data)
	}
	var result d2CreatedOrder
	if err := json.Unmarshal(body.Data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func openD2ServerMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DELIVERY_D2_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set DELIVERY_D2_MYSQL_DSN to run D-2 real HTTP/MySQL acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertD2ServerSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range []string{"users", "takeout_riders", "delivery_orders", "delivery_outbox"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing D-2 required table %s", table)
		}
	}
	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.4.") {
		t.Fatalf("expected MySQL 8.4, got %s", version)
	}
	t.Logf("real_mysql_version=%s", version)
}

func cleanupD2Server(t *testing.T, db *gorm.DB) {
	t.Helper()
	var userIDs []uint
	if db.Migrator().HasTable(&user.User{}) {
		if err := db.Unscoped().Model(&user.User{}).Where("open_id LIKE ?", "mock_openid_"+d2Fence+"%").
			Pluck("id", &userIDs).Error; err != nil {
			t.Errorf("list D-2 users: %v", err)
		}
	}
	var orderIDs []string
	if len(userIDs) > 0 && db.Migrator().HasTable(&delivery.Order{}) {
		if err := db.Model(&delivery.Order{}).Where("merchant_user_id IN ?", userIDs).
			Pluck("delivery_order_public_id", &orderIDs).Error; err != nil {
			t.Errorf("list D-2 orders: %v", err)
		}
	}
	if len(orderIDs) > 0 && db.Migrator().HasTable(&delivery.OutboxEvent{}) {
		if err := db.Where("aggregate_public_id IN ?", orderIDs).Delete(&delivery.OutboxEvent{}).Error; err != nil {
			t.Errorf("cleanup D-2 outbox: %v", err)
		}
	}
	if len(userIDs) > 0 {
		if db.Migrator().HasTable(&delivery.Order{}) {
			if err := db.Where("merchant_user_id IN ?", userIDs).Delete(&delivery.Order{}).Error; err != nil {
				t.Errorf("cleanup D-2 orders: %v", err)
			}
		}
		if db.Migrator().HasTable(&takeout.Rider{}) {
			if err := db.Where("rider_user_id IN ?", userIDs).Delete(&takeout.Rider{}).Error; err != nil {
				t.Errorf("cleanup D-2 riders: %v", err)
			}
		}
		if err := db.Unscoped().Where("id IN ?", userIDs).Delete(&user.User{}).Error; err != nil {
			t.Errorf("cleanup D-2 users: %v", err)
		}
	}
}

func d2HTTPJSON(
	t *testing.T,
	client *http.Client,
	method, endpoint, token string,
	payload any,
) (int, d2Envelope) {
	t.Helper()
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, endpoint, body)
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
	var envelope d2Envelope
	_ = json.NewDecoder(response.Body).Decode(&envelope)
	return response.StatusCode, envelope
}
