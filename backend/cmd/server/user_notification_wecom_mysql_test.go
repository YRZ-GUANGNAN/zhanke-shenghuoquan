package main

import (
	"encoding/json"
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/notification"
)

// 企微群卡片的发送闸门依赖真实 SQL（状态/版本/骑手复核）。普通单测都传
// db=nil 绕过了这段代码，历史上曾因此让一个坏列名（deleted_at）静默吞掉
// 全部订单群卡片；这里用按迁移建表的临时 MySQL 库钉住真实路径。
// 与 identity 的 MySQL 验收共用 TEST_MYSQL_DSN 临时库脚手架。
func TestDeliveryWecomFormatterChecksLiveOrderState(t *testing.T) {
	db := openUI21AIdentityMySQL(t)
	executeUI21AIdentitySQLFile(t, db, "../../migrations/023_delivery_order.sql")

	previous, next := uint64(2), uint64(3)
	now := time.Now().UTC()
	if err := db.Exec(
		`INSERT INTO delivery_orders (delivery_order_public_id, merchant_user_id, rider_user_id, status,
			delivery_fee_cents, rider_fee_cents, platform_cut_cents, pickup_address, delivery_address,
			row_version, created_at, updated_at)
		 VALUES (?, 1, ?, ?, 150, 100, 50, '东门菜鸟驿站', '12栋305', 6, ?, ?)`,
		notificationTestOrderID, next, string(delivery.StatusDelivering), now, now).Error; err != nil {
		t.Fatal(err)
	}
	format := deliveryWecomGroupFormatter(db, func() bool { return true }, func() bool { return true }, "")

	statusPayload, _ := json.Marshal(delivery.OrderNotificationPayload{
		OrderPublicID: notificationTestOrderID, MerchantUserID: 1,
		RiderUserID: &next, From: delivery.StatusPickedUp, To: delivery.StatusDelivering,
	})
	statusEvent := notification.Event{EventType: delivery.EventOrderStatusChanged, AggregateType: "delivery_order",
		AggregatePublicID: notificationTestOrderID, AggregateVersion: 6, PayloadVersion: 1, Payload: statusPayload}
	if groups, err := format(statusEvent); err != nil || len(groups) != 2 {
		t.Fatalf("current order must send group cards: groups=%d err=%v", len(groups), err)
	}

	// 订单已取消（状态/版本都与事件不符）→ 状态卡过期，跳过。
	if err := db.Exec(`UPDATE delivery_orders SET status = ?, row_version = 7 WHERE delivery_order_public_id = ?`,
		string(delivery.StatusCancelled), notificationTestOrderID).Error; err != nil {
		t.Fatal(err)
	}
	if groups, err := format(statusEvent); err != nil || len(groups) != 0 {
		t.Fatalf("obsolete status event must be skipped: groups=%d err=%v", len(groups), err)
	}

	// 改派事件：订单推进到 to_pickup、版本更高，但骑手未变 → 卡片仍应发出。
	if err := db.Exec(`UPDATE delivery_orders SET status = ?, row_version = 7 WHERE delivery_order_public_id = ?`,
		string(delivery.StatusToPickup), notificationTestOrderID).Error; err != nil {
		t.Fatal(err)
	}
	reassignPayload, _ := json.Marshal(delivery.OrderNotificationPayload{
		OrderPublicID: notificationTestOrderID, MerchantUserID: 1,
		RiderUserID: &next, PreviousRiderUserID: &previous,
		From: delivery.StatusClaimed, To: delivery.StatusClaimed,
	})
	reassignEvent := notification.Event{EventType: delivery.EventOrderReassigned, AggregateType: "delivery_order",
		AggregatePublicID: notificationTestOrderID, AggregateVersion: 5, PayloadVersion: 1, Payload: reassignPayload}
	if groups, err := format(reassignEvent); err != nil || len(groups) != 2 {
		t.Fatalf("reassign with matching rider must send cards: groups=%d err=%v", len(groups), err)
	}

	// 又被改派（当前骑手已变）→ 旧改派卡跳过。
	if err := db.Exec(`UPDATE delivery_orders SET rider_user_id = 4, row_version = 8 WHERE delivery_order_public_id = ?`,
		notificationTestOrderID).Error; err != nil {
		t.Fatal(err)
	}
	if groups, err := format(reassignEvent); err != nil || len(groups) != 0 {
		t.Fatalf("superseded reassign must be skipped: groups=%d err=%v", len(groups), err)
	}

	// 订单已终结 → 改派卡跳过。
	if err := db.Exec(`UPDATE delivery_orders SET rider_user_id = ?, status = ?, row_version = 9 WHERE delivery_order_public_id = ?`,
		next, string(delivery.StatusCancelled), notificationTestOrderID).Error; err != nil {
		t.Fatal(err)
	}
	if groups, err := format(reassignEvent); err != nil || len(groups) != 0 {
		t.Fatalf("terminal-order reassign must be skipped: groups=%d err=%v", len(groups), err)
	}

	// 订单行不存在 → 跳过而非报错。
	if err := db.Exec(`DELETE FROM delivery_orders WHERE delivery_order_public_id = ?`, notificationTestOrderID).Error; err != nil {
		t.Fatal(err)
	}
	if groups, err := format(statusEvent); err != nil || len(groups) != 0 {
		t.Fatalf("missing order row must be skipped: groups=%d err=%v", len(groups), err)
	}

	// 查询本身失败（表缺失）→ 必须把错误传出去，让事件进入重试而不是静默丢失。
	if err := db.Exec(`DROP TABLE delivery_orders`).Error; err != nil {
		t.Fatal(err)
	}
	if groups, err := format(statusEvent); err == nil || len(groups) != 0 {
		t.Fatalf("state lookup failure must surface as an error: groups=%d err=%v", len(groups), err)
	}

	// 两个值班群都未启用时卡片本就不会产出：闸门不执行，也不把错误抛给
	// 站内信/订阅等非群聊通道。
	disabled := deliveryWecomGroupFormatter(db, func() bool { return false }, func() bool { return false }, "")
	if groups, err := disabled(statusEvent); err != nil || len(groups) != 0 {
		t.Fatalf("disabled groups must skip before the gate query: groups=%d err=%v", len(groups), err)
	}
}
