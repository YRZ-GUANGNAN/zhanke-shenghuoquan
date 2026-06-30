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
	"campus-circle/backend/internal/module/delivery"
	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/module/media/miniappflow"
	"campus-circle/backend/internal/module/user"
)

const d4HTTPFence = "d4-delivery-proof-http"

type d4HTTPProofPort struct{}

func (d4HTTPProofPort) ValidateOwnedApprovedAsset(context.Context, uint64, string) error {
	return nil
}
func (d4HTTPProofPort) PublishProofReference(context.Context, string, string, uint16) error {
	return nil
}
func (d4HTTPProofPort) ResolveProofURL(context.Context, string, uint16, time.Duration) (string, error) {
	return "https://cos.example/private-proof?q-signature=redacted", nil
}

type d4GenericDisplay struct{}

func (d4GenericDisplay) ContentURL(context.Context, coremedia.Purpose, string, uint16) (string, error) {
	return "https://must-not-be-reached.example", nil
}

func TestD4PrivateProofParticipantFenceAgainstRealHTTPMySQL84(t *testing.T) {
	db := openD2ServerMySQL(t)
	assertD2ServerSchema(t, db)
	cleanupD4HTTP(t, db)
	t.Cleanup(func() { cleanupD4HTTP(t, db) })

	ctx := context.Background()
	auth := user.NewService(
		user.NewRepository(db), "d4-http-secret-that-is-longer-than-thirty-two-characters",
		"", "", true, user.WithAppEnv("dev"),
	)
	merchant, err := auth.WechatLogin(ctx, user.LoginInput{Code: d4HTTPFence + "-merchant", Nickname: "D4 Merchant"})
	if err != nil {
		t.Fatal(err)
	}
	rider, err := auth.WechatLogin(ctx, user.LoginInput{Code: d4HTTPFence + "-rider", Nickname: "D4 Rider"})
	if err != nil {
		t.Fatal(err)
	}
	outsider, err := auth.WechatLogin(ctx, user.LoginInput{Code: d4HTTPFence + "-outsider", Nickname: "D4 Outsider"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	riderID := uint64(rider.User.ID)
	order := delivery.Order{
		DeliveryOrderPublicID: "01J00000000000000000000701",
		MerchantUserID:        uint64(merchant.User.ID), RiderUserID: &riderID,
		PickupContactName: "Pickup", PickupContactPhone: "13100007001", PickupAddress: "Private pickup",
		RecipientName: "Recipient", RecipientPhone: "13100007002", DeliveryAddress: "Private delivery",
		Remark: "", DeliveryFeeCents: 150, RiderFeeCents: 100, PlatformCutCents: 50,
		Status: delivery.StatusPickedUp, ReadyForClaimAt: &now, ClaimedAt: &now,
		ToPickupAt: &now, PickedUpAt: &now, RowVersion: 4, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	activated := now
	proof := delivery.Proof{
		ProofPublicID:         "01J00000000000000000000702",
		DeliveryOrderPublicID: order.DeliveryOrderPublicID,
		ProofType:             delivery.ProofPickup, Slot: 0,
		AssetPublicID: "01J00000000000000000000703", UploaderUserID: riderID,
		Status: delivery.ProofActive, OperationID: "01J00000000000000000000704",
		RowVersion: 2, AttemptCount: 1, CreatedAt: now, UpdatedAt: now, ActivatedAt: &activated,
	}
	if err := db.Create(&proof).Error; err != nil {
		t.Fatal(err)
	}

	runtime, err := newDeliveryRuntime(config.Config{
		AppEnv: "dev", EnableDelivery: true, WeComRiderWebhookURL: "",
	}, db, d4HTTPProofPort{})
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	registerDeliveryRoutes(runtime, api, auth)
	miniappflow.NewHandler(nil, func(c *gin.Context) uint64 {
		return uint64(user.CurrentUserID(c))
	}, d4GenericDisplay{}).Register(api.Group("/miniapp/media", auth.AuthRequired(), user.UnmutedRequired()))
	server := httptest.NewServer(router)
	defer server.Close()

	for _, participant := range []user.LoginResult{merchant, rider} {
		status, body := d2HTTPJSON(t, server.Client(), http.MethodGet,
			server.URL+"/api/v1/delivery/orders/"+order.DeliveryOrderPublicID+"/proofs",
			participant.Token, nil)
		if status != http.StatusOK || !strings.Contains(string(body.Data), `"proofType":"pickup"`) ||
			strings.Contains(strings.ToLower(string(body.Data)), "asset") ||
			strings.Contains(strings.ToLower(string(body.Data)), "url") {
			t.Fatalf("participant list status=%d body=%s", status, body.Data)
		}
		request, err := http.NewRequest(http.MethodGet,
			server.URL+"/api/v1/delivery/orders/"+order.DeliveryOrderPublicID+"/proofs/pickup/0/resolve", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+participant.Token)
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		var envelope d2Envelope
		_ = json.NewDecoder(response.Body).Decode(&envelope)
		response.Body.Close()
		if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "no-store" ||
			!strings.Contains(string(envelope.Data), `"expiresInSeconds":60`) ||
			strings.Contains(strings.ToLower(string(envelope.Data)), "finalkey") ||
			strings.Contains(strings.ToLower(string(envelope.Data)), "versionid") {
			t.Fatalf("participant resolve status=%d cache=%q body=%s",
				response.StatusCode, response.Header.Get("Cache-Control"), envelope.Data)
		}
	}
	status, _ := d2HTTPJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/v1/delivery/orders/"+order.DeliveryOrderPublicID+"/proofs/pickup/0/resolve",
		outsider.Token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("nonparticipant resolve status=%d want=404", status)
	}
	status, _ = d2HTTPJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/v1/miniapp/media/resolve/delivery_proof/"+proof.ProofPublicID+"/0",
		outsider.Token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("generic delivery_proof resolve status=%d want=404", status)
	}
	if err := db.Model(&delivery.Order{}).Where("id = ?", order.ID).Update("rider_user_id", outsider.User.ID).Error; err != nil {
		t.Fatal(err)
	}
	status, _ = d2HTTPJSON(t, server.Client(), http.MethodGet,
		server.URL+"/api/v1/delivery/orders/"+order.DeliveryOrderPublicID+"/proofs/pickup/0/resolve",
		rider.Token, nil)
	if status != http.StatusNotFound {
		t.Fatalf("released rider resolve status=%d want=404", status)
	}
	t.Log("participant_auth=PASS merchant=200 assigned_rider=200 outsider=404 released_rider=404 generic_resolve=404 no_store=PASS ttl=60")
}

func cleanupD4HTTP(t *testing.T, db *gorm.DB) {
	t.Helper()
	var userIDs []uint
	if err := db.Unscoped().Model(&user.User{}).
		Where("open_id LIKE ?", "mock_openid_"+d4HTTPFence+"%").Pluck("id", &userIDs).Error; err != nil {
		t.Errorf("list D-4 HTTP users: %v", err)
		return
	}
	var orderIDs []string
	if len(userIDs) > 0 {
		if err := db.Model(&delivery.Order{}).Where("merchant_user_id IN ?", userIDs).
			Pluck("delivery_order_public_id", &orderIDs).Error; err != nil {
			t.Errorf("list D-4 HTTP orders: %v", err)
		}
	}
	if len(orderIDs) > 0 {
		db.Where("delivery_order_public_id IN ?", orderIDs).Delete(&delivery.Proof{})
		db.Where("delivery_order_public_id IN ?", orderIDs).Delete(&delivery.Order{})
	}
	if len(userIDs) > 0 {
		db.Unscoped().Where("id IN ?", userIDs).Delete(&user.User{})
	}
}
