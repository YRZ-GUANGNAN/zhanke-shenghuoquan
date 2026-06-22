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
	riderMerchantOne = "01J00000000000000000000401"
	riderBuyerOne    = uint64(940001)
	riderOne         = uint64(930001)
	riderTwo         = uint64(930002)
	riderThree       = uint64(930003)
	riderFour        = uint64(930004)
	riderFive        = uint64(930005)
	riderSix         = uint64(930006)
)

var riderAcceptanceUsers = []uint64{riderOne, riderTwo, riderThree, riderFour, riderFive, riderSix}

func Test00RiderMigrationRollbackApplyAgainstRealMySQL84(t *testing.T) {
	db := openG5MySQL(t)
	assertG5MySQL84(t, db)
	cleanupG5(t, db)
	orderCheckBefore := readG5OrderStatusConstraint(t, db)
	for _, table := range []any{&Fulfillment{}, &Rider{}} {
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
	if db.Migrator().HasTable(&RiderConfig{}) {
		var configs []RiderConfig
		if err := db.Find(&configs).Error; err != nil {
			t.Fatal(err)
		}
		if len(configs) != 1 || configs[0].ConfigScope != "global" ||
			configs[0].MaxInFlightPerRider != 1000 || configs[0].RowVersion != 1 {
			t.Fatalf("refusing to roll back non-seed rider config: %#v", configs)
		}
	}
	applyRiderSQLFile(t, db, "../../../migrations/rollback/020_takeout_rider.sql")
	for _, table := range []any{&Fulfillment{}, &Rider{}, &RiderConfig{}} {
		if db.Migrator().HasTable(table) {
			t.Fatalf("rollback 020 left %T behind", table)
		}
	}
	applyRiderSQLFile(t, db, "../../../migrations/020_takeout_rider.sql")
	assertG5RiderSchema(t, db)
	if after := readG5OrderStatusConstraint(t, db); after != orderCheckBefore {
		t.Fatalf("migration 020 changed frozen G-3 order CHECK before=%s after=%s", orderCheckBefore, after)
	}
	t.Log("migration_020_apply_rollback_apply=PASS tables=3 foreign_keys=0 rider_config=global:1000 g3_order_unchanged=PASS")
}

func TestRiderLifecycleClaimLimitsAndDeliveryAgainstRealMySQL84(t *testing.T) {
	db := openG5MySQLWithTables(t)
	cleanupG5(t, db)
	t.Cleanup(func() { cleanupG5(t, db) })
	ctx := context.Background()
	now := time.Date(2026, 6, 22, 18, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	catalog, _ := NewCatalogService(repository, func() time.Time { return now })
	orders, _ := NewOrderService(repository, repository, func() time.Time { return now })
	orderOps, _ := NewRiderOrderOps(repository, func() time.Time { return now })
	riders, _ := NewRiderService(repository, orderOps, func() time.Time { return now })
	dish := createG5Dish(t, ctx, catalog, "Rider Dish", 1000)

	registered := registerG5Rider(t, ctx, riders, riderOne)
	registered, err := riders.SetResting(ctx, riderOne, registered.RowVersion)
	if err != nil || registered.Status != RiderResting {
		t.Fatalf("set resting=%#v err=%v", registered, err)
	}
	if _, err := riders.SetActive(ctx, riderOne, 1); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale rider CAS returned %v", err)
	}
	registered, err = riders.SetActive(ctx, riderOne, registered.RowVersion)
	if err != nil || registered.Status != RiderActive {
		t.Fatalf("set active=%#v err=%v", registered, err)
	}
	registered, err = riders.Suspend(ctx, riderOne, registered.RowVersion, RiderActive)
	if err != nil || registered.Status != RiderSuspended {
		t.Fatalf("suspend=%#v err=%v", registered, err)
	}
	registered, err = riders.Unsuspend(ctx, riderOne, registered.RowVersion)
	if err != nil || registered.Status != RiderActive {
		t.Fatalf("unsuspend=%#v err=%v", registered, err)
	}
	loadedRider, err := riders.FindRider(ctx, riderOne)
	if err != nil || loadedRider.ID != registered.ID || loadedRider.Status != RiderActive {
		t.Fatalf("find rider=%#v err=%v", loadedRider, err)
	}

	config, err := riders.GetRiderConfig(ctx)
	if err != nil || config.MaxInFlightPerRider != 1000 || config.RowVersion != 1 {
		t.Fatalf("seed rider config=%#v err=%v", config, err)
	}
	config, err = riders.UpdateRiderConfig(ctx, 1, config.RowVersion)
	if err != nil || config.MaxInFlightPerRider != 1 || config.RowVersion != 2 {
		t.Fatalf("config update=%#v err=%v", config, err)
	}
	if _, err := riders.UpdateRiderConfig(ctx, 2, 1); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale config CAS returned %v", err)
	}

	mainOrder := createG5ReadyOrder(t, ctx, orders, dish)
	claimable, err := riders.ListClaimableOrders(ctx)
	if err != nil || !containsG5Order(claimable, mainOrder.OrderPublicID) {
		t.Fatalf("claimable pool missing order=%s values=%#v err=%v", mainOrder.OrderPublicID, claimable, err)
	}
	fulfillment, err := riders.ClaimOrder(ctx, riderOne, mainOrder.OrderPublicID, mainOrder.RowVersion)
	if err != nil || fulfillment.Status != FulfillmentAssigned || fulfillment.RowVersion != 1 {
		t.Fatalf("claim=%#v err=%v", fulfillment, err)
	}
	claimedOrder, err := orders.FindOrder(ctx, mainOrder.OrderPublicID)
	if err != nil || claimedOrder.Status != OrderDelivering || claimedOrder.RiderUserID == nil || *claimedOrder.RiderUserID != riderOne {
		t.Fatalf("claimed order=%#v err=%v", claimedOrder, err)
	}
	idempotentClaim, err := riders.ClaimOrder(ctx, riderOne, mainOrder.OrderPublicID, mainOrder.RowVersion)
	if err != nil || idempotentClaim.ID != fulfillment.ID || idempotentClaim.RowVersion != fulfillment.RowVersion {
		t.Fatalf("idempotent claim=%#v err=%v", idempotentClaim, err)
	}
	var mainFulfillmentCount int64
	if err := db.Model(&Fulfillment{}).Where("order_public_id = ?", mainOrder.OrderPublicID).Count(&mainFulfillmentCount).Error; err != nil || mainFulfillmentCount != 1 {
		t.Fatalf("main fulfillment count=%d err=%v", mainFulfillmentCount, err)
	}
	claimable, err = riders.ListClaimableOrders(ctx)
	if err != nil || containsG5Order(claimable, mainOrder.OrderPublicID) {
		t.Fatalf("claimed order still visible in pool values=%#v err=%v", claimable, err)
	}

	fulfillment, err = riders.StartToStore(ctx, fulfillment.FulfillmentPublicID, fulfillment.RowVersion)
	if err != nil || fulfillment.Status != FulfillmentToStore || fulfillment.ToStoreAt == nil {
		t.Fatalf("to store=%#v err=%v", fulfillment, err)
	}
	idempotentStep, err := riders.StartToStore(ctx, fulfillment.FulfillmentPublicID, 1)
	if err != nil || idempotentStep.RowVersion != fulfillment.RowVersion {
		t.Fatalf("idempotent to-store=%#v err=%v", idempotentStep, err)
	}
	if _, err := riders.MarkPicked(ctx, fulfillment.FulfillmentPublicID, 1); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale fulfillment CAS returned %v", err)
	}
	fulfillment, err = riders.MarkPicked(ctx, fulfillment.FulfillmentPublicID, fulfillment.RowVersion)
	if err != nil || fulfillment.Status != FulfillmentPicked || fulfillment.PickedAt == nil {
		t.Fatalf("picked=%#v err=%v", fulfillment, err)
	}
	fulfillment, err = riders.StartToUser(ctx, fulfillment.FulfillmentPublicID, fulfillment.RowVersion)
	if err != nil || fulfillment.Status != FulfillmentToUser || fulfillment.ToUserAt == nil {
		t.Fatalf("to user=%#v err=%v", fulfillment, err)
	}
	preCompleteVersion := fulfillment.RowVersion
	fulfillment, err = riders.CompleteFulfillment(ctx, fulfillment.FulfillmentPublicID, preCompleteVersion)
	if err != nil || fulfillment.Status != FulfillmentDelivered || fulfillment.DeliveredAt == nil || !fulfillment.Status.Terminal() {
		t.Fatalf("delivered=%#v err=%v", fulfillment, err)
	}
	completedOrder, err := orders.FindOrder(ctx, mainOrder.OrderPublicID)
	if err != nil || completedOrder.Status != OrderCompleted {
		t.Fatalf("completed order=%#v err=%v", completedOrder, err)
	}
	var earningCount int64
	if err := db.Model(&RiderEarningRecord{}).Where("order_public_id = ?", mainOrder.OrderPublicID).Count(&earningCount).Error; err != nil || earningCount != 1 {
		t.Fatalf("earning count=%d err=%v", earningCount, err)
	}
	idempotentDelivered, err := riders.CompleteFulfillment(ctx, fulfillment.FulfillmentPublicID, preCompleteVersion)
	if err != nil || idempotentDelivered.RowVersion != fulfillment.RowVersion {
		t.Fatalf("idempotent delivered=%#v err=%v", idempotentDelivered, err)
	}
	if err := db.Model(&RiderEarningRecord{}).Where("order_public_id = ?", mainOrder.OrderPublicID).Count(&earningCount).Error; err != nil || earningCount != 1 {
		t.Fatalf("earning count after repeat=%d err=%v", earningCount, err)
	}
	loadedFulfillment, err := riders.FindFulfillmentByOrder(ctx, mainOrder.OrderPublicID)
	if err != nil || loadedFulfillment.ID != fulfillment.ID || loadedFulfillment.Status != FulfillmentDelivered {
		t.Fatalf("find fulfillment=%#v err=%v", loadedFulfillment, err)
	}
	if _, err := riders.StartToStore(ctx, fulfillment.FulfillmentPublicID, fulfillment.RowVersion); err == nil {
		t.Fatal("terminal fulfillment accepted transition")
	}

	limitRider := registerG5Rider(t, ctx, riders, riderTwo)
	_ = limitRider
	limitOrderOne := createG5ReadyOrder(t, ctx, orders, dish)
	limitOrderTwo := createG5ReadyOrder(t, ctx, orders, dish)
	if _, err := riders.ClaimOrder(ctx, riderTwo, limitOrderOne.OrderPublicID, limitOrderOne.RowVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := riders.ClaimOrder(ctx, riderTwo, limitOrderTwo.OrderPublicID, limitOrderTwo.RowVersion); !errors.Is(err, ErrInFlightLimit) {
		t.Fatalf("second in-flight claim returned %v", err)
	}
	config, err = riders.UpdateRiderConfig(ctx, 1000, config.RowVersion)
	if err != nil || config.MaxInFlightPerRider != 1000 {
		t.Fatalf("raise limit=%#v err=%v", config, err)
	}
	if _, err := riders.ClaimOrder(ctx, riderTwo, limitOrderTwo.OrderPublicID, limitOrderTwo.RowVersion); err != nil {
		t.Fatalf("claim after raising limit: %v", err)
	}

	t.Log("rider_lifecycle=PASS fulfillment_linear=PASS stale_CAS=REJECTED idempotency=PASS")
	t.Log("claim_order_same_tx=PASS ready_pool=PASS configurable_limit=1_reject_then_1000_accept delivery_crosspoint=PASS earning_count=1")
}

func TestRiderConcurrentClaimAgainstRealMySQL84(t *testing.T) {
	db := openG5MySQLWithTables(t)
	cleanupG5(t, db)
	t.Cleanup(func() { cleanupG5(t, db) })
	ctx := context.Background()
	repository := NewRepository(db)
	catalog, _ := NewCatalogService(repository, time.Now)
	orders, _ := NewOrderService(repository, repository, time.Now)
	orderOps, _ := NewRiderOrderOps(repository, time.Now)
	riders, _ := NewRiderService(repository, orderOps, time.Now)
	dish := createG5Dish(t, ctx, catalog, "Concurrent Dish", 900)

	config, err := riders.GetRiderConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	config, err = riders.UpdateRiderConfig(ctx, 1, config.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	registerG5Rider(t, ctx, riders, riderThree)
	sameRiderOrders := []Order{createG5ReadyOrder(t, ctx, orders, dish), createG5ReadyOrder(t, ctx, orders, dish)}
	sameResults := runG5Claims(ctx, riders, []g5Claim{
		{riderThree, sameRiderOrders[0].OrderPublicID, sameRiderOrders[0].RowVersion},
		{riderThree, sameRiderOrders[1].OrderPublicID, sameRiderOrders[1].RowVersion},
	})
	assertG5CompetingErrors(t, sameResults, ErrInFlightLimit)
	var sameRiderInFlight int64
	if err := db.Model(&Fulfillment{}).Where("rider_user_id = ? AND status <> ?", riderThree, FulfillmentDelivered).Count(&sameRiderInFlight).Error; err != nil || sameRiderInFlight != 1 {
		t.Fatalf("same rider in-flight=%d err=%v", sameRiderInFlight, err)
	}

	config, err = riders.UpdateRiderConfig(ctx, 1000, config.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	registerG5Rider(t, ctx, riders, riderFour)
	registerG5Rider(t, ctx, riders, riderFive)
	sharedOrder := createG5ReadyOrder(t, ctx, orders, dish)
	sharedResults := runG5Claims(ctx, riders, []g5Claim{
		{riderFour, sharedOrder.OrderPublicID, sharedOrder.RowVersion},
		{riderFive, sharedOrder.OrderPublicID, sharedOrder.RowVersion},
	})
	assertG5CompetingErrors(t, sharedResults, ErrOrderAlreadyClaimed)
	claimedOrder, err := orders.FindOrder(ctx, sharedOrder.OrderPublicID)
	if err != nil || claimedOrder.Status != OrderDelivering || claimedOrder.RiderUserID == nil ||
		(*claimedOrder.RiderUserID != riderFour && *claimedOrder.RiderUserID != riderFive) {
		t.Fatalf("shared claimed order=%#v err=%v", claimedOrder, err)
	}
	var sharedFulfillments int64
	if err := db.Model(&Fulfillment{}).Where("order_public_id = ?", sharedOrder.OrderPublicID).Count(&sharedFulfillments).Error; err != nil || sharedFulfillments != 1 {
		t.Fatalf("shared fulfillment count=%d err=%v", sharedFulfillments, err)
	}
	t.Log("same_rider_two_orders_lock=PASS success=1 in_flight_limit=1")
	t.Logf("multi_rider_same_order=PASS success=1 already_claimed=1 winning_rider=%d", *claimedOrder.RiderUserID)
}

func TestRiderCrossPointRollbackAgainstRealMySQL84(t *testing.T) {
	db := openG5MySQLWithTables(t)
	cleanupG5(t, db)
	t.Cleanup(func() { cleanupG5(t, db) })
	ctx := context.Background()
	now := time.Now().UTC()
	repository := NewRepository(db)
	catalog, _ := NewCatalogService(repository, func() time.Time { return now })
	orders, _ := NewOrderService(repository, repository, func() time.Time { return now })
	orderOps, _ := NewRiderOrderOps(repository, func() time.Time { return now })
	riders, _ := NewRiderService(repository, orderOps, func() time.Time { return now })
	dish := createG5Dish(t, ctx, catalog, "Rollback Dish", 600)
	readyOrder := createG5ReadyOrder(t, ctx, orders, dish)
	inactiveRider := registerG5Rider(t, ctx, riders, riderSix)
	inactiveRider, err := riders.SetResting(ctx, riderSix, inactiveRider.RowVersion)
	if err != nil || inactiveRider.Status != RiderResting {
		t.Fatalf("prepare inactive rider=%#v err=%v", inactiveRider, err)
	}
	if _, err := riders.ClaimOrder(ctx, riderSix, readyOrder.OrderPublicID, readyOrder.RowVersion); !errors.Is(err, ErrRiderUnavailable) {
		t.Fatalf("inactive rider claim returned %v", err)
	}
	orderBeforeRollbackCase, err := orders.FindOrder(ctx, readyOrder.OrderPublicID)
	if err != nil || orderBeforeRollbackCase.Status != OrderReadyForPickup {
		t.Fatalf("inactive claim changed order=%#v err=%v", orderBeforeRollbackCase, err)
	}
	var claimResidue int64
	if err := db.Model(&Fulfillment{}).Where("order_public_id = ?", readyOrder.OrderPublicID).Count(&claimResidue).Error; err != nil || claimResidue != 0 {
		t.Fatalf("inactive claim fulfillment residue=%d err=%v", claimResidue, err)
	}

	fulfillmentID, _ := NewULID()
	assigned, toStore, picked, toUser := now, now, now, now
	broken := Fulfillment{
		FulfillmentPublicID: fulfillmentID, OrderPublicID: readyOrder.OrderPublicID, RiderUserID: riderSix,
		Status: FulfillmentToUser, AssignedAt: &assigned, ToStoreAt: &toStore, PickedAt: &picked, ToUserAt: &toUser,
		RowVersion: 4, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&broken).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := riders.CompleteFulfillment(ctx, fulfillmentID, broken.RowVersion); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("cross-point failure returned %v", err)
	}
	var after Fulfillment
	if err := db.Where("fulfillment_public_id = ?", fulfillmentID).First(&after).Error; err != nil {
		t.Fatal(err)
	}
	orderAfter, err := orders.FindOrder(ctx, readyOrder.OrderPublicID)
	if err != nil || after.Status != FulfillmentToUser || after.RowVersion != 4 || orderAfter.Status != OrderReadyForPickup {
		t.Fatalf("rollback failed fulfillment=%#v order=%#v err=%v", after, orderAfter, err)
	}
	t.Log("inactive_rider_claim=REJECTED order=ready_for_pickup fulfillment_count=0")
	t.Log("complete_crosspoint_failure_rollback=PASS fulfillment=TO_USER order=ready_for_pickup")
}

type g5Claim struct {
	riderUserID uint64
	orderID     string
	orderRV     uint64
}

func runG5Claims(ctx context.Context, service *RiderService, claims []g5Claim) []error {
	start := make(chan struct{})
	results := make(chan error, len(claims))
	var wait sync.WaitGroup
	for _, claim := range claims {
		claim := claim
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := service.ClaimOrder(ctx, claim.riderUserID, claim.orderID, claim.orderRV)
			results <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	values := make([]error, 0, len(claims))
	for result := range results {
		values = append(values, result)
	}
	return values
}

func assertG5CompetingErrors(t *testing.T, results []error, expectedFailure error) {
	t.Helper()
	var success, failure int
	for _, result := range results {
		switch {
		case result == nil:
			success++
		case errors.Is(result, expectedFailure):
			failure++
		default:
			t.Fatalf("unexpected competing claim result %v", result)
		}
	}
	if success != 1 || failure != 1 {
		t.Fatalf("competing claims success=%d expected_failure=%d", success, failure)
	}
}

func registerG5Rider(t *testing.T, ctx context.Context, service *RiderService, userID uint64) Rider {
	t.Helper()
	value, err := service.RegisterRider(ctx, RegisterRiderInput{RiderUserID: userID, RealName: "Rider", Phone: "13800000000"})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func createG5Dish(t *testing.T, ctx context.Context, service *CatalogService, name string, price uint64) Dish {
	t.Helper()
	dish, err := service.CreateDish(ctx, CreateDishInput{MerchantPublicID: riderMerchantOne, Name: name, PriceCents: price})
	if err != nil {
		t.Fatal(err)
	}
	dish, err = service.PutOnShelf(ctx, dish.DishPublicID, riderMerchantOne, DishDraft, dish.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	return dish
}

func createG5ReadyOrder(t *testing.T, ctx context.Context, service *OrderService, dish Dish) Order {
	t.Helper()
	order, err := service.CreateOrder(ctx, CreateOrderInput{
		BuyerUserID: riderBuyerOne, MerchantPublicID: riderMerchantOne,
		Items:   []OrderItemRequest{{DishPublicID: dish.DishPublicID, Quantity: 1}},
		Address: OrderAddress{ContactName: "Buyer", ContactPhone: "13800000000", AddressBuilding: "A", AddressRoom: "101"},
	})
	if err != nil {
		t.Fatal(err)
	}
	order, err = service.MarkPaid(ctx, order.OrderPublicID, order.RowVersion, "PAY-"+order.OrderPublicID)
	if err != nil {
		t.Fatal(err)
	}
	order, err = service.Accept(ctx, order.OrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	order, err = service.StartPreparing(ctx, order.OrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	order, err = service.MarkReady(ctx, order.OrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	return order
}

func containsG5Order(values []Order, publicID string) bool {
	for _, value := range values {
		if value.OrderPublicID == publicID {
			return true
		}
	}
	return false
}

func openG5MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TAKEOUT_G5_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TAKEOUT_G5_MYSQL_DSN to run G-5 real MySQL acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func openG5MySQLWithTables(t *testing.T) *gorm.DB {
	t.Helper()
	db := openG5MySQL(t)
	for _, table := range []any{&Rider{}, &Fulfillment{}, &RiderConfig{}, &Order{}, &Dish{}} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required migration missing for %T", table)
		}
	}
	return db
}

func assertG5MySQL84(t *testing.T, db *gorm.DB) {
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

func applyRiderSQLFile(t *testing.T, db *gorm.DB, relativePath string) {
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

func assertG5RiderSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for table, expected := range map[string]int64{
		"takeout_riders": 8, "takeout_fulfillments": 13, "takeout_rider_config": 5,
	} {
		var count int64
		if err := db.Raw(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=?`, table).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != expected {
			t.Errorf("%s columns=%d want=%d", table, count, expected)
		}
	}
	criticalTypes := map[string]map[string]string{
		"takeout_riders": {
			"rider_user_id": "bigint unsigned", "status": "varchar(24)", "real_name": "varchar(64)",
			"phone": "varchar(32)", "row_version": "bigint unsigned",
		},
		"takeout_fulfillments": {
			"fulfillment_public_id": "char(26)", "order_public_id": "char(26)",
			"rider_user_id": "bigint unsigned", "status": "varchar(24)", "row_version": "bigint unsigned",
		},
		"takeout_rider_config": {
			"config_scope": "varchar(32)", "max_in_flight_per_rider": "int unsigned", "row_version": "bigint unsigned",
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
		AND table_name IN ('takeout_riders','takeout_fulfillments','takeout_rider_config')
		AND referenced_table_name IS NOT NULL`).Scan(&foreignKeys).Error; err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 0 {
		t.Fatalf("rider foreign keys=%d", foreignKeys)
	}
	for table, indexes := range map[string][]string{
		"takeout_riders":       {"uq_takeout_rider_user", "idx_takeout_rider_status"},
		"takeout_fulfillments": {"uq_takeout_fulfillment_public_id", "uq_takeout_fulfillment_order", "idx_takeout_fulfillment_rider", "idx_takeout_fulfillment_status", "idx_takeout_fulfillment_rider_status"},
		"takeout_rider_config": {"uq_takeout_rider_config_scope"},
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
	var riderCheck, fulfillmentCheck string
	if err := db.Raw(`SELECT cc.check_clause FROM information_schema.table_constraints tc
		JOIN information_schema.check_constraints cc ON cc.constraint_schema=tc.constraint_schema AND cc.constraint_name=tc.constraint_name
		WHERE tc.table_schema=DATABASE() AND tc.table_name='takeout_riders' AND tc.constraint_name='chk_takeout_rider_status'`).Scan(&riderCheck).Error; err != nil {
		t.Fatal(err)
	}
	for _, status := range []RiderStatus{RiderActive, RiderResting, RiderSuspended} {
		if !strings.Contains(riderCheck, string(status)) {
			t.Errorf("rider CHECK missing %s: %s", status, riderCheck)
		}
	}
	if err := db.Raw(`SELECT cc.check_clause FROM information_schema.table_constraints tc
		JOIN information_schema.check_constraints cc ON cc.constraint_schema=tc.constraint_schema AND cc.constraint_name=tc.constraint_name
		WHERE tc.table_schema=DATABASE() AND tc.table_name='takeout_fulfillments' AND tc.constraint_name='chk_takeout_fulfillment_status'`).Scan(&fulfillmentCheck).Error; err != nil {
		t.Fatal(err)
	}
	for _, status := range []FulfillmentStatus{FulfillmentAssigned, FulfillmentToStore, FulfillmentPicked, FulfillmentToUser, FulfillmentDelivered} {
		if !strings.Contains(fulfillmentCheck, string(status)) {
			t.Errorf("fulfillment CHECK missing %s: %s", status, fulfillmentCheck)
		}
	}
	var config RiderConfig
	if err := db.Where("config_scope = ?", "global").First(&config).Error; err != nil {
		t.Fatal(err)
	}
	if config.MaxInFlightPerRider != 1000 || config.RowVersion != 1 {
		t.Fatalf("rider config seed=%#v", config)
	}
}

func readG5OrderStatusConstraint(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var clause string
	if err := db.Raw(`SELECT cc.check_clause FROM information_schema.table_constraints tc
		JOIN information_schema.check_constraints cc ON cc.constraint_schema=tc.constraint_schema AND cc.constraint_name=tc.constraint_name
		WHERE tc.table_schema=DATABASE() AND tc.table_name='takeout_orders' AND tc.constraint_name='chk_takeout_order_status'`).Scan(&clause).Error; err != nil {
		t.Fatal(err)
	}
	if clause == "" {
		t.Fatal("frozen G-3 order status constraint missing")
	}
	return clause
}

func cleanupG5(t *testing.T, db *gorm.DB) {
	t.Helper()
	if db.Migrator().HasTable(&Fulfillment{}) {
		if err := db.Where("rider_user_id IN ?", riderAcceptanceUsers).Delete(&Fulfillment{}).Error; err != nil {
			t.Errorf("cleanup fulfillments: %v", err)
		}
	}
	if db.Migrator().HasTable(&Rider{}) {
		if err := db.Where("rider_user_id IN ?", riderAcceptanceUsers).Delete(&Rider{}).Error; err != nil {
			t.Errorf("cleanup riders: %v", err)
		}
	}
	if db.Migrator().HasTable(&Order{}) {
		var orderIDs []string
		if err := db.Model(&Order{}).Where("buyer_user_id = ? OR merchant_public_id = ?", riderBuyerOne, riderMerchantOne).
			Pluck("order_public_id", &orderIDs).Error; err != nil {
			t.Errorf("list G5 orders: %v", err)
		}
		if len(orderIDs) > 0 {
			if db.Migrator().HasTable(&RiderEarningRecord{}) {
				if err := db.Where("order_public_id IN ?", orderIDs).Delete(&RiderEarningRecord{}).Error; err != nil {
					t.Errorf("cleanup earnings: %v", err)
				}
			}
			if db.Migrator().HasTable(&OrderItem{}) {
				if err := db.Where("order_public_id IN ?", orderIDs).Delete(&OrderItem{}).Error; err != nil {
					t.Errorf("cleanup order items: %v", err)
				}
			}
			if err := db.Where("order_public_id IN ?", orderIDs).Delete(&Order{}).Error; err != nil {
				t.Errorf("cleanup orders: %v", err)
			}
		}
	}
	if db.Migrator().HasTable(&Dish{}) {
		if err := db.Where("merchant_public_id = ?", riderMerchantOne).Delete(&Dish{}).Error; err != nil {
			t.Errorf("cleanup dishes: %v", err)
		}
	}
	if db.Migrator().HasTable(&DishCategory{}) {
		if err := db.Where("merchant_public_id = ?", riderMerchantOne).Delete(&DishCategory{}).Error; err != nil {
			t.Errorf("cleanup categories: %v", err)
		}
	}
	if db.Migrator().HasTable(&RiderConfig{}) {
		if err := db.Model(&RiderConfig{}).Where("config_scope = ?", "global").Updates(map[string]any{
			"max_in_flight_per_rider": 1000, "row_version": 1, "updated_at": time.Now().UTC(),
		}).Error; err != nil {
			t.Errorf("restore rider config: %v", err)
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

func TestZZRiderAcceptanceResidueAgainstRealMySQL84(t *testing.T) {
	db := openG5MySQLWithTables(t)
	cleanupG5(t, db)
	for name, model := range map[string]any{"riders": &Rider{}, "fulfillments": &Fulfillment{}} {
		var count int64
		if err := db.Model(model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("%s residue=%d", name, count)
		}
	}
	for name, model := range map[string]any{"orders": &Order{}, "items": &OrderItem{}, "earnings": &RiderEarningRecord{}} {
		var count int64
		query := db.Model(model)
		if name == "orders" {
			query = query.Where("buyer_user_id = ? OR merchant_public_id = ?", riderBuyerOne, riderMerchantOne)
		}
		if err := query.Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("G5 %s residue=%d", name, count)
		}
	}
	var dishes int64
	if err := db.Model(&Dish{}).Where("merchant_public_id = ?", riderMerchantOne).Count(&dishes).Error; err != nil {
		t.Fatal(err)
	}
	if dishes != 0 {
		t.Errorf("G5 catalog dish residue=%d", dishes)
	}
	var config RiderConfig
	if err := db.Where("config_scope = ?", "global").First(&config).Error; err != nil {
		t.Fatal(err)
	}
	if config.MaxInFlightPerRider != 1000 || config.RowVersion != 1 {
		t.Errorf("rider config residue=%#v", config)
	}
	t.Log("rider_residue riders=0 fulfillments=0 orders=0 items=0 earnings=0 dishes=0 rider_config=global:1000:rv1")
}
