package deliveryconsole

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/user"
)

const (
	cmdMerchant   = uint64(9_006_101_101)
	cmdRiderOne   = uint64(9_006_102_101)
	cmdRiderTwo   = uint64(9_006_102_102)
	cmdRequestID1 = "01J000000000000000000C0001"
	cmdRequestID2 = "01J000000000000000000C0002"
	cmdRequestID3 = "01J000000000000000000C0003"
)

func TestDeliveryAdminCommandsAgainstRealMySQL84(t *testing.T) {
	db := openD6BMySQL(t)
	assertD6BSchema(t, db)
	if !db.Migrator().HasTable(&delivery.AdminOrderAction{}) {
		t.Skip("migration 060 is not applied to this database")
	}
	var hasCancelReason int64
	db.Raw(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'delivery_orders' AND column_name = 'cancel_reason'`).Scan(&hasCancelReason)
	if hasCancelReason == 0 {
		t.Skip("migration 060 is not applied to this database")
	}
	cleanup := func() {
		db.Where("merchant_user_id = ?", cmdMerchant).Delete(&delivery.Order{})
		db.Where("rider_user_id IN ?", []uint64{cmdRiderOne, cmdRiderTwo}).Delete(&takeout.Rider{})
		db.Unscoped().Where("open_id IN ?", []string{"mock_openid_cmd-admin"}).Delete(&user.User{})
	}
	cleanup()
	t.Cleanup(cleanup)

	ctx := context.Background()
	now := time.Date(2026, 7, 3, 6, 0, 0, 0, time.UTC)
	repository := delivery.NewRepository(db)
	auth := user.NewService(
		user.NewRepository(db),
		"d6b-test-secret-that-is-longer-than-thirty-two-characters",
		"", "", true,
	)
	administrator, err := auth.WechatLogin(ctx, user.LoginInput{Code: "cmd-admin", Nickname: "CMD admin"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&user.User{}).Where("id = ?", administrator.User.ID).Update("role", user.RoleAdmin).Error; err != nil {
		t.Fatal(err)
	}
	for _, rider := range []uint64{cmdRiderOne, cmdRiderTwo} {
		value := takeout.Rider{
			RiderUserID: rider, Status: takeout.RiderActive,
			RealName: "命令测试骑手", Phone: "13000000000",
			RowVersion: 1, CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(&value).Error; err != nil {
			t.Fatal(err)
		}
	}

	router := newD6BRouter(auth, db, func() time.Time { return now })
	server := httptest.NewServer(router)
	defer server.Close()

	status, body := d6bHTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/admin/delivery/riders", administrator.Token, nil)
	if status != http.StatusOK || body.Code != 0 || !strings.Contains(string(body.Data), `"riderUserId":9006102101`) {
		t.Fatalf("rider list status=%d body=%+v", status, body)
	}
	status, body = d6bHTTPJSON(t, server.Client(), http.MethodGet, server.URL+"/api/v1/admin/delivery/riders/9006102101/orders", administrator.Token, nil)
	if status != http.StatusOK || body.Code != 0 {
		t.Fatalf("rider orders status=%d body=%+v", status, body)
	}

	dispatch := newD6BDispatch(t, repository, func() time.Time { return now })

	// Reassign: claim by rider one, then hand over to rider two.
	claimed := createD6BOrder(t, ctx, dispatch, cmdMerchant, cmdRequestID1)
	claimed, _, err = dispatch.ConfirmMockFee(ctx, claimed.MerchantUserID, claimed.DeliveryOrderPublicID, claimed.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err = dispatch.ClaimOrder(ctx, cmdRiderOne, claimed.DeliveryOrderPublicID, claimed.RowVersion)
	if err != nil {
		t.Fatal(err)
	}

	status, _ = d6bHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/admin/delivery/orders/"+claimed.DeliveryOrderPublicID+"/reassign", administrator.Token, map[string]any{
		"riderUserId": cmdRiderTwo, "rowVersion": claimed.RowVersion - 1, "reason": "stale attempt",
	})
	if status != http.StatusConflict {
		t.Fatalf("stale reassign status=%d want=409", status)
	}
	status, body = d6bHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/admin/delivery/orders/"+claimed.DeliveryOrderPublicID+"/reassign", administrator.Token, map[string]any{
		"riderUserId": cmdRiderTwo, "rowVersion": claimed.RowVersion, "reason": "原骑手车辆故障",
	})
	if status != http.StatusOK || body.Code != 0 || !strings.Contains(string(body.Data), `"riderUserId":9006102102`) {
		t.Fatalf("reassign status=%d body=%+v", status, body)
	}
	var action delivery.AdminOrderAction
	if err := db.Where("delivery_order_public_id = ?", claimed.DeliveryOrderPublicID).First(&action).Error; err != nil {
		t.Fatal(err)
	}
	if action.Action != delivery.AdminActionReassign || action.ActorUserID != uint64(administrator.User.ID) ||
		action.FromRiderUserID == nil || *action.FromRiderUserID != cmdRiderOne ||
		action.ToRiderUserID == nil || *action.ToRiderUserID != cmdRiderTwo || action.Reason != "原骑手车辆故障" {
		t.Fatalf("audit row=%#v", action)
	}

	// Hand the order back to rider one so the suspended-rider case can target
	// rider two from an active flow.
	var latest delivery.Order
	if err := db.Where("delivery_order_public_id = ?", claimed.DeliveryOrderPublicID).First(&latest).Error; err != nil {
		t.Fatal(err)
	}
	status, _ = d6bHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/admin/delivery/orders/"+claimed.DeliveryOrderPublicID+"/reassign", administrator.Token, map[string]any{
		"riderUserId": cmdRiderOne, "rowVersion": latest.RowVersion, "reason": "原骑手返回接单",
	})
	if status != http.StatusOK {
		t.Fatalf("reassign back status=%d", status)
	}

	// Suspended riders cannot receive reassignments.
	status, body = d6bHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/admin/delivery/riders/9006102102/status", administrator.Token, map[string]any{
		"status": "suspended", "rowVersion": 1, "reason": "多次配送超时",
	})
	if status != http.StatusOK || body.Code != 0 || !strings.Contains(string(body.Data), `"status":"suspended"`) {
		t.Fatalf("suspend rider status=%d body=%+v", status, body)
	}
	status, _ = d6bHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/admin/delivery/riders/9006102102/status", administrator.Token, map[string]any{
		"status": "active", "rowVersion": 1, "reason": "stale attempt",
	})
	if status != http.StatusConflict {
		t.Fatalf("stale rider status=%d want=409", status)
	}
	if err := db.Where("delivery_order_public_id = ?", claimed.DeliveryOrderPublicID).First(&latest).Error; err != nil {
		t.Fatal(err)
	}
	status, body = d6bHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/admin/delivery/orders/"+claimed.DeliveryOrderPublicID+"/reassign", administrator.Token, map[string]any{
		"riderUserId": cmdRiderTwo, "rowVersion": latest.RowVersion, "reason": "改派给封禁骑手",
	})
	if status != http.StatusConflict || !strings.Contains(body.Message, "not active") {
		t.Fatalf("reassign to suspended rider status=%d body=%+v", status, body)
	}
	// Restore rider two for cleanup symmetry.
	status, _ = d6bHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/admin/delivery/riders/9006102102/status", administrator.Token, map[string]any{
		"status": "resting", "rowVersion": 2, "reason": "恢复检查",
	})
	if status != http.StatusOK {
		t.Fatalf("restore rider status=%d", status)
	}

	// Cancel: ready_for_claim order keeps its reason.
	cancellable := createD6BOrder(t, ctx, dispatch, cmdMerchant, cmdRequestID2)
	cancellable, _, err = dispatch.ConfirmMockFee(ctx, cancellable.MerchantUserID, cancellable.DeliveryOrderPublicID, cancellable.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	status, body = d6bHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/admin/delivery/orders/"+cancellable.DeliveryOrderPublicID+"/cancel", administrator.Token, map[string]any{
		"rowVersion": cancellable.RowVersion, "reason": "用户要求取消",
	})
	if status != http.StatusOK || body.Code != 0 || !strings.Contains(string(body.Data), `"status":"cancelled"`) {
		t.Fatalf("cancel status=%d body=%+v", status, body)
	}
	var cancelled delivery.Order
	if err := db.Where("delivery_order_public_id = ?", cancellable.DeliveryOrderPublicID).First(&cancelled).Error; err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != delivery.StatusCancelled || cancelled.CancelReason != "用户要求取消" || cancelled.CancelledAt == nil {
		t.Fatalf("cancelled order=%#v", cancelled)
	}
	status, _ = d6bHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/admin/delivery/orders/"+cancellable.DeliveryOrderPublicID+"/cancel", administrator.Token, map[string]any{
		"rowVersion": cancellable.RowVersion, "reason": "重复取消",
	})
	if status != http.StatusConflict {
		t.Fatalf("repeat cancel status=%d want=409", status)
	}

	// Picked-up orders cannot be cancelled directly.
	pickedUp := createD6BOrder(t, ctx, dispatch, cmdMerchant, cmdRequestID3)
	pickedUp, _, err = dispatch.ConfirmMockFee(ctx, pickedUp.MerchantUserID, pickedUp.DeliveryOrderPublicID, pickedUp.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	pickedUp, err = dispatch.ClaimOrder(ctx, cmdRiderOne, pickedUp.DeliveryOrderPublicID, pickedUp.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	pickedUp, err = dispatch.StartToPickup(ctx, cmdRiderOne, pickedUp.DeliveryOrderPublicID, pickedUp.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	pickedUp, err = dispatch.MarkPickedUp(ctx, cmdRiderOne, pickedUp.DeliveryOrderPublicID, pickedUp.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	status, _ = d6bHTTPJSON(t, server.Client(), http.MethodPost, server.URL+"/api/v1/admin/delivery/orders/"+pickedUp.DeliveryOrderPublicID+"/cancel", administrator.Token, map[string]any{
		"rowVersion": pickedUp.RowVersion, "reason": "已取货仍取消",
	})
	if status != http.StatusConflict {
		t.Fatalf("cancel picked up status=%d want=409", status)
	}

	t.Log("admin_commands_mysql=PASS reassign_CAS=PASS rider_status_CAS=PASS cancel_reason=PASS picked_up_guard=PASS")
}
