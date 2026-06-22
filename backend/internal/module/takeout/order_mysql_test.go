package takeout

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	orderMerchantOne = "01J00000000000000000000201"
	orderMerchantTwo = "01J00000000000000000000202"
	orderBuyerOne    = uint64(910001)
	orderBuyerTwo    = uint64(910002)
	orderRiderOne    = uint64(920001)
)

var orderAcceptanceMerchants = []string{orderMerchantOne, orderMerchantTwo}
var orderAcceptanceBuyers = []uint64{orderBuyerOne, orderBuyerTwo}

func Test00OrderMigrationRollbackApplyAgainstRealMySQL84(t *testing.T) {
	db := openG3MySQL(t)
	assertG3MySQL84(t, db)
	cleanupG3Order(t, db)
	for _, table := range []any{&OrderItem{}, &RiderEarningRecord{}, &Order{}} {
		if !db.Migrator().HasTable(table) {
			continue
		}
		var count int64
		if err := db.Model(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("refusing to roll back non-empty %T rows=%d", table, count)
		}
	}
	if db.Migrator().HasTable(&FeeConfig{}) {
		var configs []FeeConfig
		if err := db.Find(&configs).Error; err != nil {
			t.Fatal(err)
		}
		if len(configs) != 1 || configs[0].ConfigScope != "global" || configs[0].RiderFeeCents != 100 ||
			configs[0].PlatformCutCents != 50 || configs[0].RowVersion != 1 {
			t.Fatalf("refusing to roll back non-seed fee config: %#v", configs)
		}
	}
	applyOrderSQLFile(t, db, "../../../migrations/rollback/019_takeout_order.sql")
	for _, table := range []any{&RiderEarningRecord{}, &OrderItem{}, &Order{}, &FeeConfig{}} {
		if db.Migrator().HasTable(table) {
			t.Fatalf("rollback 019 left %T behind", table)
		}
	}
	applyOrderSQLFile(t, db, "../../../migrations/019_takeout_order.sql")
	assertG3OrderSchema(t, db)
	t.Log("migration_019_apply_rollback_apply=PASS tables=4 foreign_keys=0 fee_seed=global:100:50")
}

func TestOrderLifecycleSnapshotsFeesAndSettlementAgainstRealMySQL84(t *testing.T) {
	db := openG3MySQLWithTables(t)
	cleanupG3Order(t, db)
	t.Cleanup(func() { cleanupG3Order(t, db) })
	ctx := context.Background()
	now := time.Date(2026, 6, 22, 15, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	catalog, _ := NewCatalogService(repository, func() time.Time { return now })
	orders, _ := NewOrderService(repository, repository, func() time.Time { return now })

	dishOne := createG3Dish(t, ctx, catalog, orderMerchantOne, "Rice", 1200, true)
	dishTwo := createG3Dish(t, ctx, catalog, orderMerchantOne, "Soup", 500, true)
	draftDish := createG3Dish(t, ctx, catalog, orderMerchantOne, "Draft", 300, false)
	otherMerchantDish := createG3Dish(t, ctx, catalog, orderMerchantTwo, "Other", 700, true)

	mainOrder := createG3Order(t, ctx, orders, orderBuyerOne, orderMerchantOne, []OrderItemRequest{
		{DishPublicID: dishOne.DishPublicID, Quantity: 2},
		{DishPublicID: dishTwo.DishPublicID, Quantity: 3},
	})
	if mainOrder.ItemsAmountCents != 3900 || mainOrder.DeliveryFeeCents != 150 || mainOrder.TotalAmountCents != 4050 ||
		mainOrder.Status != OrderPendingPayment || mainOrder.RowVersion != 1 {
		t.Fatalf("server-calculated order mismatch: %#v", mainOrder)
	}
	items, err := orders.ListOrderItems(ctx, mainOrder.OrderPublicID)
	if err != nil || len(items) != 2 || items[0].DishNameSnapshot != "Rice" ||
		items[0].UnitPriceCentsSnapshot != 1200 || items[0].LineAmountCents != 2400 ||
		items[1].DishNameSnapshot != "Soup" || items[1].UnitPriceCentsSnapshot != 500 || items[1].LineAmountCents != 1500 {
		t.Fatalf("order item snapshots=%#v err=%v", items, err)
	}
	_, err = catalog.UpdateDish(ctx, UpdateDishInput{
		DishPublicID: dishOne.DishPublicID, MerchantPublicID: orderMerchantOne,
		ExpectedStatus: DishOnShelf, ExpectedRowVersion: dishOne.RowVersion,
		Name: "Rice Updated", Description: "new price", PriceCents: 9999,
	})
	if err != nil {
		t.Fatal(err)
	}
	immutableItems, err := orders.ListOrderItems(ctx, mainOrder.OrderPublicID)
	if err != nil || immutableItems[0].DishNameSnapshot != "Rice" || immutableItems[0].UnitPriceCentsSnapshot != 1200 {
		t.Fatalf("snapshots changed with catalog: %#v err=%v", immutableItems, err)
	}

	if _, err := orders.CreateOrder(ctx, CreateOrderInput{
		BuyerUserID: orderBuyerOne, MerchantPublicID: orderMerchantOne,
		Items: []OrderItemRequest{{DishPublicID: draftDish.DishPublicID, Quantity: 1}},
	}); !errors.Is(err, ErrDishUnavailable) {
		t.Fatalf("draft dish returned %v", err)
	}
	if _, err := orders.CreateOrder(ctx, CreateOrderInput{
		BuyerUserID: orderBuyerOne, MerchantPublicID: orderMerchantOne,
		Items: []OrderItemRequest{{DishPublicID: otherMerchantDish.DishPublicID, Quantity: 1}},
	}); !errors.Is(err, ErrDishUnavailable) {
		t.Fatalf("wrong merchant dish returned %v", err)
	}
	missingDish := "01J00000000000000000000299"
	if _, err := orders.CreateOrder(ctx, CreateOrderInput{
		BuyerUserID: orderBuyerOne, MerchantPublicID: orderMerchantOne,
		Items: []OrderItemRequest{{DishPublicID: missingDish, Quantity: 1}},
	}); !errors.Is(err, ErrDishUnavailable) {
		t.Fatalf("missing dish returned %v", err)
	}

	config, err := orders.GetActiveFeeConfig(ctx)
	if err != nil || config.RiderFeeCents != 100 || config.PlatformCutCents != 50 || config.RowVersion != 1 {
		t.Fatalf("seed fee config=%#v err=%v", config, err)
	}
	config, err = orders.UpdateFeeConfig(ctx, 120, 60, config.RowVersion)
	if err != nil || config.RowVersion != 2 || config.RiderFeeCents != 120 || config.PlatformCutCents != 60 {
		t.Fatalf("fee update=%#v err=%v", config, err)
	}
	if _, err := orders.UpdateFeeConfig(ctx, 130, 70, 1); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale fee update returned %v", err)
	}

	mainOrder, err = orders.MarkPaid(ctx, mainOrder.OrderPublicID, mainOrder.RowVersion, "PAY-G3-001")
	if err != nil || mainOrder.Status != OrderPaid || mainOrder.PayNo == nil || mainOrder.PaidAt == nil {
		t.Fatalf("mark paid=%#v err=%v", mainOrder, err)
	}
	idempotentPaid, err := orders.MarkPaid(ctx, mainOrder.OrderPublicID, 1, "PAY-G3-001")
	if err != nil || idempotentPaid.RowVersion != mainOrder.RowVersion {
		t.Fatalf("idempotent paid=%#v err=%v", idempotentPaid, err)
	}
	if _, err := orders.MarkPaid(ctx, mainOrder.OrderPublicID, 1, "PAY-DIFFERENT"); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("different pay number returned %v", err)
	}
	if _, err := orders.Accept(ctx, mainOrder.OrderPublicID, 1); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale accept returned %v", err)
	}
	mainOrder, err = orders.Accept(ctx, mainOrder.OrderPublicID, mainOrder.RowVersion)
	if err != nil || mainOrder.Status != OrderAccepted || mainOrder.AcceptedAt == nil {
		t.Fatalf("accept=%#v err=%v", mainOrder, err)
	}
	mainOrder, err = orders.StartPreparing(ctx, mainOrder.OrderPublicID, mainOrder.RowVersion)
	if err != nil || mainOrder.Status != OrderPreparing || mainOrder.PreparingAt == nil {
		t.Fatalf("preparing=%#v err=%v", mainOrder, err)
	}
	mainOrder, err = orders.MarkReady(ctx, mainOrder.OrderPublicID, mainOrder.RowVersion)
	if err != nil || mainOrder.Status != OrderReadyForPickup || mainOrder.ReadyAt == nil {
		t.Fatalf("ready=%#v err=%v", mainOrder, err)
	}
	mainOrder, err = orders.StartDelivering(ctx, mainOrder.OrderPublicID, mainOrder.RowVersion, orderRiderOne)
	if err != nil || mainOrder.Status != OrderDelivering || mainOrder.DeliveringAt == nil ||
		mainOrder.RiderUserID == nil || *mainOrder.RiderUserID != orderRiderOne {
		t.Fatalf("delivering=%#v err=%v", mainOrder, err)
	}
	preCompleteVersion := mainOrder.RowVersion
	mainOrder, err = orders.CompleteDelivery(ctx, mainOrder.OrderPublicID, preCompleteVersion)
	if err != nil || mainOrder.Status != OrderCompleted || mainOrder.CompletedAt == nil || !mainOrder.Status.Terminal() {
		t.Fatalf("complete=%#v err=%v", mainOrder, err)
	}
	idempotentComplete, err := orders.CompleteDelivery(ctx, mainOrder.OrderPublicID, preCompleteVersion)
	if err != nil || idempotentComplete.RowVersion != mainOrder.RowVersion {
		t.Fatalf("idempotent complete=%#v err=%v", idempotentComplete, err)
	}
	var earningCount int64
	if err := db.Model(&RiderEarningRecord{}).Where("order_public_id = ?", mainOrder.OrderPublicID).Count(&earningCount).Error; err != nil || earningCount != 1 {
		t.Fatalf("earning count=%d err=%v", earningCount, err)
	}
	earning, err := orders.FindEarningByOrder(ctx, mainOrder.OrderPublicID)
	if err != nil || earning.RiderUserID != orderRiderOne || earning.RiderFeeCents != 120 ||
		earning.PlatformCutCents != 60 || earning.SettlementStatus != SettlementPending || earning.RowVersion != 1 {
		t.Fatalf("earning snapshot=%#v err=%v", earning, err)
	}
	earning, err = orders.MarkSettled(ctx, earning.EarningPublicID, earning.RowVersion)
	if err != nil || earning.SettlementStatus != SettlementSettled || earning.SettledAt == nil || earning.RowVersion != 2 {
		t.Fatalf("settle=%#v err=%v", earning, err)
	}
	idempotentEarning, err := orders.MarkSettled(ctx, earning.EarningPublicID, 1)
	if err != nil || idempotentEarning.RowVersion != 2 {
		t.Fatalf("idempotent settle=%#v err=%v", idempotentEarning, err)
	}

	cancelled := createG3Order(t, ctx, orders, orderBuyerOne, orderMerchantOne, []OrderItemRequest{{DishPublicID: dishTwo.DishPublicID, Quantity: 1}})
	cancelled, err = orders.Cancel(ctx, cancelled.OrderPublicID, cancelled.RowVersion)
	if err != nil || cancelled.Status != OrderCancelled || cancelled.CancelledAt == nil {
		t.Fatalf("cancel=%#v err=%v", cancelled, err)
	}
	if _, err := orders.MarkPaid(ctx, cancelled.OrderPublicID, cancelled.RowVersion, "TOO-LATE"); err == nil {
		t.Fatal("terminal cancelled order accepted a transition")
	}

	refundedPaid := createG3Order(t, ctx, orders, orderBuyerOne, orderMerchantOne, []OrderItemRequest{{DishPublicID: dishTwo.DishPublicID, Quantity: 1}})
	refundedPaid, _ = orders.MarkPaid(ctx, refundedPaid.OrderPublicID, refundedPaid.RowVersion, "PAY-G3-002")
	refundedPaid, err = orders.Refund(ctx, refundedPaid.OrderPublicID, OrderPaid, refundedPaid.RowVersion)
	if err != nil || refundedPaid.Status != OrderRefunded || refundedPaid.RefundedAt == nil {
		t.Fatalf("refund paid=%#v err=%v", refundedPaid, err)
	}
	refundedAccepted := createG3Order(t, ctx, orders, orderBuyerOne, orderMerchantOne, []OrderItemRequest{{DishPublicID: dishTwo.DishPublicID, Quantity: 1}})
	refundedAccepted, _ = orders.MarkPaid(ctx, refundedAccepted.OrderPublicID, refundedAccepted.RowVersion, "PAY-G3-003")
	refundedAccepted, _ = orders.Accept(ctx, refundedAccepted.OrderPublicID, refundedAccepted.RowVersion)
	refundedAccepted, err = orders.Refund(ctx, refundedAccepted.OrderPublicID, OrderAccepted, refundedAccepted.RowVersion)
	if err != nil || refundedAccepted.Status != OrderRefunded {
		t.Fatalf("refund accepted=%#v err=%v", refundedAccepted, err)
	}
	loaded, err := orders.FindOrder(ctx, mainOrder.OrderPublicID)
	if err != nil || loaded.ID != mainOrder.ID || loaded.Status != OrderCompleted {
		t.Fatalf("find order=%#v err=%v", loaded, err)
	}
	t.Log("multi_dish_snapshot=PASS server_amount_recalculation=PASS lifecycle_all_paths=PASS stale_CAS=REJECTED terminal_fail_closed=PASS")
	t.Log("complete_earning_same_tx=PASS repeated_complete_earning_count=1 fee_snapshot=120:60 settlement=PASS")
}

func TestOrderCompetingCASAgainstRealMySQL84(t *testing.T) {
	db := openG3MySQLWithTables(t)
	cleanupG3Order(t, db)
	t.Cleanup(func() { cleanupG3Order(t, db) })
	ctx := context.Background()
	repository := NewRepository(db)
	catalog, _ := NewCatalogService(repository, time.Now)
	orders, _ := NewOrderService(repository, repository, time.Now)
	dish := createG3Dish(t, ctx, catalog, orderMerchantOne, "CAS Dish", 800, true)
	order := createG3Order(t, ctx, orders, orderBuyerTwo, orderMerchantOne, []OrderItemRequest{{DishPublicID: dish.DishPublicID, Quantity: 1}})
	order, err := orders.MarkPaid(ctx, order.OrderPublicID, order.RowVersion, "PAY-G3-CAS")
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		_, transitionErr := orders.Accept(ctx, order.OrderPublicID, order.RowVersion)
		results <- transitionErr
	}()
	go func() {
		defer wait.Done()
		<-start
		_, transitionErr := orders.Refund(ctx, order.OrderPublicID, OrderPaid, order.RowVersion)
		results <- transitionErr
	}()
	close(start)
	wait.Wait()
	close(results)

	var success, conflict int
	for result := range results {
		switch {
		case result == nil:
			success++
		case errors.Is(result, ErrConcurrentUpdate):
			conflict++
		default:
			t.Fatalf("unexpected competing error %v", result)
		}
	}
	loaded, err := orders.FindOrder(ctx, order.OrderPublicID)
	if err != nil || success != 1 || conflict != 1 || loaded.RowVersion != order.RowVersion+1 ||
		(loaded.Status != OrderAccepted && loaded.Status != OrderRefunded) {
		t.Fatalf("competing result success=%d conflict=%d order=%#v err=%v", success, conflict, loaded, err)
	}
	t.Logf("competing_order_CAS=PASS success=%d conflict=%d final_status=%s", success, conflict, loaded.Status)
}

func createG3Dish(t *testing.T, ctx context.Context, service *CatalogService, merchant, name string, price uint64, onShelf bool) Dish {
	t.Helper()
	dish, err := service.CreateDish(ctx, CreateDishInput{MerchantPublicID: merchant, Name: name, PriceCents: price})
	if err != nil {
		t.Fatal(err)
	}
	if onShelf {
		dish, err = service.PutOnShelf(ctx, dish.DishPublicID, merchant, DishDraft, dish.RowVersion)
		if err != nil {
			t.Fatal(err)
		}
	}
	return dish
}

func createG3Order(t *testing.T, ctx context.Context, service *OrderService, buyer uint64, merchant string, items []OrderItemRequest) Order {
	t.Helper()
	order, err := service.CreateOrder(ctx, CreateOrderInput{
		BuyerUserID: buyer, MerchantPublicID: merchant, Items: items,
		Address: OrderAddress{ContactName: "Buyer", ContactPhone: "13800000000", AddressBuilding: "A", AddressRoom: "101"},
		Remark:  "G3 acceptance",
	})
	if err != nil {
		t.Fatal(err)
	}
	return order
}

func openG3MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TAKEOUT_G3_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TAKEOUT_G3_MYSQL_DSN to run G-3 real MySQL acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func openG3MySQLWithTables(t *testing.T) *gorm.DB {
	t.Helper()
	db := openG3MySQL(t)
	for _, table := range []any{&Order{}, &OrderItem{}, &RiderEarningRecord{}, &FeeConfig{}, &Dish{}} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required migration missing for %T", table)
		}
	}
	return db
}

func assertG3MySQL84(t *testing.T, db *gorm.DB) {
	t.Helper()
	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.4.") {
		t.Fatalf("expected MySQL 8.4, got %s", version)
	}
	t.Logf("real_mysql_version=%s", version)
}

func applyOrderSQLFile(t *testing.T, db *gorm.DB, relativePath string) {
	t.Helper()
	file, err := os.Open(filepath.Clean(relativePath))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var statement strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		statement.WriteString(line)
		statement.WriteByte('\n')
		if !strings.HasSuffix(strings.TrimSpace(line), ";") {
			continue
		}
		query := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(statement.String()), ";"))
		statement.Reset()
		if query != "" {
			if err := db.Exec(query).Error; err != nil {
				t.Fatalf("apply %s: %v", relativePath, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func assertG3OrderSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	expectedColumnCounts := map[string]int64{
		"takeout_orders": 26, "takeout_order_items": 8,
		"takeout_rider_earning_records": 11, "takeout_fee_config": 6,
	}
	for table, expected := range expectedColumnCounts {
		var count int64
		if err := db.Raw(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=?`, table).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != expected {
			t.Errorf("%s columns=%d want=%d", table, count, expected)
		}
	}
	criticalTypes := map[string]map[string]string{
		"takeout_orders": {
			"order_public_id": "char(26)", "buyer_user_id": "bigint unsigned", "merchant_public_id": "char(26)",
			"rider_user_id": "bigint unsigned", "status": "varchar(24)", "items_amount_cents": "bigint unsigned",
			"delivery_fee_cents": "bigint unsigned", "total_amount_cents": "bigint unsigned", "row_version": "bigint unsigned",
		},
		"takeout_order_items": {
			"order_public_id": "char(26)", "dish_public_id": "char(26)", "unit_price_cents_snapshot": "bigint unsigned",
			"quantity": "int unsigned", "line_amount_cents": "bigint unsigned",
		},
		"takeout_rider_earning_records": {
			"earning_public_id": "char(26)", "order_public_id": "char(26)", "rider_user_id": "bigint unsigned",
			"rider_fee_cents": "bigint unsigned", "platform_cut_cents": "bigint unsigned", "settlement_status": "varchar(24)",
			"row_version": "bigint unsigned",
		},
		"takeout_fee_config": {
			"config_scope": "varchar(32)", "rider_fee_cents": "bigint unsigned", "platform_cut_cents": "bigint unsigned",
			"row_version": "bigint unsigned",
		},
	}
	for table, columns := range criticalTypes {
		for column, expected := range columns {
			var actual string
			if err := db.Raw(`SELECT column_type FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=? AND column_name=?`, table, column).Scan(&actual).Error; err != nil {
				t.Fatal(err)
			}
			if actual != expected {
				t.Errorf("%s.%s type=%s want=%s", table, column, actual, expected)
			}
		}
	}
	var foreignKeys int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.key_column_usage WHERE table_schema=DATABASE()
		AND table_name IN ('takeout_orders','takeout_order_items','takeout_rider_earning_records','takeout_fee_config')
		AND referenced_table_name IS NOT NULL`).Scan(&foreignKeys).Error; err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 0 {
		t.Fatalf("order foreign keys=%d", foreignKeys)
	}
	for table, indexes := range map[string][]string{
		"takeout_orders":                {"uq_takeout_order_public_id", "idx_takeout_order_buyer", "idx_takeout_order_merchant", "idx_takeout_order_rider", "idx_takeout_order_status"},
		"takeout_order_items":           {"idx_takeout_order_item_order"},
		"takeout_rider_earning_records": {"uq_takeout_earning_public_id", "uq_takeout_earning_order", "idx_takeout_earning_settlement_status"},
		"takeout_fee_config":            {"uq_takeout_fee_config_scope"},
	} {
		for _, index := range indexes {
			var count int64
			if err := db.Raw(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=?`, table, index).Scan(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count == 0 {
				t.Errorf("missing index %s.%s", table, index)
			}
		}
	}
	var orderCheck, settlementCheck string
	if err := db.Raw(`SELECT cc.check_clause FROM information_schema.table_constraints tc
		JOIN information_schema.check_constraints cc ON cc.constraint_schema=tc.constraint_schema AND cc.constraint_name=tc.constraint_name
		WHERE tc.table_schema=DATABASE() AND tc.table_name='takeout_orders' AND tc.constraint_name='chk_takeout_order_status'`).Scan(&orderCheck).Error; err != nil {
		t.Fatal(err)
	}
	for _, status := range []OrderStatus{OrderPendingPayment, OrderPaid, OrderAccepted, OrderPreparing, OrderReadyForPickup, OrderDelivering, OrderCompleted, OrderCancelled, OrderRefunded} {
		if !strings.Contains(orderCheck, string(status)) {
			t.Errorf("order CHECK missing %s: %s", status, orderCheck)
		}
	}
	if err := db.Raw(`SELECT cc.check_clause FROM information_schema.table_constraints tc
		JOIN information_schema.check_constraints cc ON cc.constraint_schema=tc.constraint_schema AND cc.constraint_name=tc.constraint_name
		WHERE tc.table_schema=DATABASE() AND tc.table_name='takeout_rider_earning_records'
		AND tc.constraint_name='chk_takeout_earning_settlement_status'`).Scan(&settlementCheck).Error; err != nil {
		t.Fatal(err)
	}
	for _, status := range []SettlementStatus{SettlementPending, SettlementSettled} {
		if !strings.Contains(settlementCheck, string(status)) {
			t.Errorf("settlement CHECK missing %s: %s", status, settlementCheck)
		}
	}
	var config FeeConfig
	if err := db.Where("config_scope = ?", "global").First(&config).Error; err != nil {
		t.Fatal(err)
	}
	if config.RiderFeeCents != 100 || config.PlatformCutCents != 50 || config.RowVersion != 1 {
		t.Fatalf("fee seed=%#v", config)
	}
}

func cleanupG3Order(t *testing.T, db *gorm.DB) {
	t.Helper()
	if db.Migrator().HasTable(&Order{}) {
		var orderIDs []string
		if err := db.Model(&Order{}).Where("buyer_user_id IN ? OR merchant_public_id IN ?", orderAcceptanceBuyers, orderAcceptanceMerchants).
			Pluck("order_public_id", &orderIDs).Error; err != nil {
			t.Errorf("list test orders: %v", err)
		}
		if len(orderIDs) > 0 {
			if db.Migrator().HasTable(&RiderEarningRecord{}) {
				if err := db.Where("order_public_id IN ?", orderIDs).Delete(&RiderEarningRecord{}).Error; err != nil {
					t.Errorf("cleanup earnings: %v", err)
				}
			}
			if db.Migrator().HasTable(&OrderItem{}) {
				if err := db.Where("order_public_id IN ?", orderIDs).Delete(&OrderItem{}).Error; err != nil {
					t.Errorf("cleanup items: %v", err)
				}
			}
			if err := db.Where("order_public_id IN ?", orderIDs).Delete(&Order{}).Error; err != nil {
				t.Errorf("cleanup orders: %v", err)
			}
		}
	}
	if db.Migrator().HasTable(&Dish{}) {
		if err := db.Where("merchant_public_id IN ?", orderAcceptanceMerchants).Delete(&Dish{}).Error; err != nil {
			t.Errorf("cleanup dishes: %v", err)
		}
	}
	if db.Migrator().HasTable(&DishCategory{}) {
		if err := db.Where("merchant_public_id IN ?", orderAcceptanceMerchants).Delete(&DishCategory{}).Error; err != nil {
			t.Errorf("cleanup categories: %v", err)
		}
	}
	if db.Migrator().HasTable(&FeeConfig{}) {
		if err := db.Model(&FeeConfig{}).Where("config_scope = ?", "global").Updates(map[string]any{
			"rider_fee_cents": 100, "platform_cut_cents": 50, "row_version": 1, "updated_at": time.Now().UTC(),
		}).Error; err != nil {
			t.Errorf("restore fee config: %v", err)
		}
	}
}

func TestZZOrderAcceptanceResidueAgainstRealMySQL84(t *testing.T) {
	db := openG3MySQLWithTables(t)
	cleanupG3Order(t, db)
	for name, model := range map[string]any{
		"orders": &Order{}, "items": &OrderItem{}, "earnings": &RiderEarningRecord{},
	} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("%s residue=%d", name, count)
		}
	}
	var dishes int64
	if err := db.Model(&Dish{}).Where("merchant_public_id IN ?", orderAcceptanceMerchants).Count(&dishes).Error; err != nil {
		t.Fatal(err)
	}
	if dishes != 0 {
		t.Errorf("catalog test dish residue=%d", dishes)
	}
	var config FeeConfig
	if err := db.Where("config_scope = ?", "global").First(&config).Error; err != nil {
		t.Fatal(err)
	}
	if config.RiderFeeCents != 100 || config.PlatformCutCents != 50 || config.RowVersion != 1 {
		t.Errorf("fee config residue=%#v", config)
	}
	t.Log("order_residue orders=0 items=0 earnings=0 catalog_test_dishes=0 fee_config=global:100:50:rv1")
}
