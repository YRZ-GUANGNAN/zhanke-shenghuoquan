// simulate-order 用于在生产环境生成一条备注为「测试」的配送订单并模拟
// 支付成功，触发真实的订单状态变更事件，使企微值班群收到真实格式的通知。
// 仅用于通知链路联调；订单与事件可随后清理。
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/notification"
)

func dsn() string {
	value := strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	if value == "" {
		fmt.Fprintln(os.Stderr, "MYSQL_DSN is required")
		os.Exit(1)
	}
	return value
}

func main() {
	action := "create"
	if len(os.Args) > 1 {
		action = os.Args[1]
	}
	db, err := gorm.Open(mysql.Open(dsn()), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	switch action {
	case "cleanup":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: simulate-order cleanup <orderPublicId>")
			os.Exit(1)
		}
		id := strings.TrimSpace(os.Args[2])
		if !delivery.ValidULID(id) {
			fmt.Fprintln(os.Stderr, "invalid order id")
			os.Exit(1)
		}
		res := db.Exec(`DELETE FROM delivery_orders WHERE delivery_order_public_id = ?`, id)
		fmt.Printf("cleanup: deleted=%d order=%s\n", res.RowsAffected, id)
	case "create":
		notifications := notification.NewRepository(db)
		repository := delivery.NewRepository(db, delivery.WithNotificationEvents(
			func(tx *gorm.DB, fact delivery.NotificationEvent) error {
				return notifications.AppendEventTx(tx, deliveryFact(fact))
			},
		))
		service, err := delivery.NewService(repository, time.Now)
		if err != nil {
			panic(err)
		}
		ctx := context.Background()
		order, err := service.CreateOrder(ctx, delivery.CreateOrderInput{
			MerchantUserID: 1,
			Pickup:   delivery.Contact{Name: "通知测试（商家）", Phone: "13000000000", Address: "测试取货点"},
			Recipient: delivery.Contact{Name: "通知测试（收件）", Phone: "13000000001", Address: "测试送达点"},
			Remark:          "测试",
			DeliveryFeeCents: 100,
			RiderFeeCents:    80,
			PlatformCutCents: 20,
		})
		if err != nil {
			panic(err)
		}
		fmt.Printf("created order=%s status=%s rowVersion=%d\n", order.DeliveryOrderPublicID, order.Status, order.RowVersion)
		// 模拟配送费支付成功：触发 order.status.changed 事件（商家群 + 骑手群）。
		paid, err := service.MarkFeePaid(ctx, order.DeliveryOrderPublicID, order.RowVersion)
		if err != nil {
			panic(err)
		}
		fmt.Printf("transitioned order=%s status=%s rowVersion=%d\n", paid.DeliveryOrderPublicID, paid.Status, paid.RowVersion)
		fmt.Println("event appended; notification workers will deliver to WeCom groups within seconds")
	case "external":
		// 三条外部平台订单（京东/美团/淘宝），备注均为「测试」。
		notifications := notification.NewRepository(db)
		repository := delivery.NewRepository(db, delivery.WithNotificationEvents(
			func(tx *gorm.DB, fact delivery.NotificationEvent) error {
				return notifications.AppendEventTx(tx, deliveryFact(fact))
			},
		))
		service, err := delivery.NewService(repository, time.Now)
		if err != nil {
			panic(err)
		}
		ctx := context.Background()
		platforms := []struct {
			platform delivery.SourcePlatform
			label    string
			shop     string
		}{
			{delivery.SourceJD, "京东", "test-shop-jd"},
			{delivery.SourceMeituan, "美团", "test-shop-meituan"},
			{delivery.SourceTaobao, "淘宝", "test-shop-taobao"},
		}
		stamp := time.Now().Unix()
		for _, item := range platforms {
			shopID := fmt.Sprintf("%s-%d", item.shop, stamp)
			orderID := fmt.Sprintf("test-order-%s-%d", item.platform, stamp)
			order, err := service.CreateOrder(ctx, delivery.CreateOrderInput{
				MerchantUserID: 1,
				Pickup:   delivery.Contact{Name: fmt.Sprintf("%s测试（取件）", item.label), Phone: "13000000000", Address: fmt.Sprintf("%s测试取货点", item.label)},
				Recipient: delivery.Contact{Name: fmt.Sprintf("%s测试（收件）", item.label), Phone: "13000000001", Address: fmt.Sprintf("%s测试送达点", item.label)},
				Remark:          fmt.Sprintf("测试-%s", item.label),
				DeliveryFeeCents: 100,
				RiderFeeCents:    80,
				PlatformCutCents: 20,
			})
			if err != nil {
				panic(err)
			}
			// 标记外部来源（直接 SQL 更新，模拟外部平台接入后的订单快照）。
			if err := db.Exec(`UPDATE delivery_orders SET source_platform = ?, source_shop_id = ?, source_order_id = ? WHERE id = ?`,
				string(item.platform), shopID, orderID, order.ID).Error; err != nil {
				panic(err)
			}
			paid, err := service.MarkFeePaid(ctx, order.DeliveryOrderPublicID, order.RowVersion)
			if err != nil {
				panic(err)
			}
			fmt.Printf("created %s order=%s status=%s\n", item.label, paid.DeliveryOrderPublicID, paid.Status)
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown action:", action)
		os.Exit(1)
	}
}

// deliveryFact 与生产组合根中的 deliveryNotificationFact 保持一致。
func deliveryFact(fact delivery.NotificationEvent) notification.Event {
	ttl := 24 * time.Hour
	if fact.EventType == delivery.EventRiderPoolOpened {
		ttl = 10 * time.Minute
	}
	aggregateType := "delivery_order"
	expires := fact.OccurredAt.Add(ttl)
	return notification.Event{
		EventPublicID: fact.EventPublicID, EventType: fact.EventType,
		AggregateType: aggregateType, AggregatePublicID: fact.AggregatePublicID,
		AggregateVersion: fact.AggregateVersion, PayloadVersion: 1,
		Payload: append([]byte(nil), fact.Payload...), Status: notification.EventPending,
		RowVersion: 1, OccurredAt: fact.OccurredAt, ExpiresAt: &expires,
		CreatedAt: fact.OccurredAt, UpdatedAt: fact.OccurredAt,
	}
}
