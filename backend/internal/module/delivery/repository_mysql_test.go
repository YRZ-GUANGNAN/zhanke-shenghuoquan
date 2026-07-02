package delivery

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
	acceptanceMerchant = uint64(981001)
	fixtureMerchant    = uint64(991001)
	acceptanceRiderOne = uint64(982001)
	acceptanceRiderTwo = uint64(982002)
)

var acceptanceMerchantIDs = []uint64{acceptanceMerchant, fixtureMerchant}

func Test00DeliveryMigrationRollbackApplyAgainstRealMySQL84(t *testing.T) {
	db := openD1MySQL(t)
	assertD1MySQL84(t, db)
	rollbackLaterDeliveryMigrationsForD1(t, db)
	cleanupD1Orders(t, db)
	if db.Migrator().HasTable(&Order{}) {
		var count int64
		if err := db.Model(&Order{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("refusing to roll back non-empty delivery_orders rows=%d", count)
		}
	}
	applyD1SQLFile(t, db, "../../../migrations/rollback/023_delivery_order.sql")
	if db.Migrator().HasTable(&Order{}) {
		t.Fatal("rollback 023 left delivery_orders behind")
	}
	applyD1SQLFile(t, db, "../../../migrations/023_delivery_order.sql")
	assertD1Schema(t, db)
	if _, err := os.Stat("../../../migrations/024_delivery_dispatch.sql"); err == nil {
		applyD1SQLFile(t, db, "../../../migrations/024_delivery_dispatch.sql")
	}
	t.Log("migration_023_apply_rollback_apply=PASS tables=1 foreign_keys=0 checks=status+fee_snapshot")
}

func rollbackLaterDeliveryMigrationsForD1(t *testing.T, db *gorm.DB) {
	t.Helper()
	if db.Migrator().HasTable("delivery_outbox") {
		var count int64
		if err := db.Table("delivery_outbox").Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("refusing to remove non-empty delivery_outbox rows=%d", count)
		}
		if err := db.Exec("DROP TABLE delivery_outbox").Error; err != nil {
			t.Fatal(err)
		}
	}
	if db.Migrator().HasColumn(&Order{}, "merchant_request_id") {
		var count int64
		if err := db.Model(&Order{}).Where("merchant_request_id IS NOT NULL").Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("refusing to remove merchant_request_id with rows=%d", count)
		}
		if err := db.Exec(`ALTER TABLE delivery_orders
			DROP INDEX uq_delivery_order_merchant_request,
			DROP COLUMN merchant_request_id`).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestDeliveryLifecycleIdempotencyAndTimestampsAgainstRealMySQL84(t *testing.T) {
	db := openD1MySQLWithTable(t)
	cleanupD1Orders(t, db)
	t.Cleanup(func() { cleanupD1Orders(t, db) })

	ctx := context.Background()
	at := time.Date(2026, 6, 29, 2, 0, 0, 0, time.UTC)
	service, err := NewService(NewRepository(db), func() time.Time { return at })
	if err != nil {
		t.Fatal(err)
	}
	order := createD1Order(t, ctx, service)
	if order.Status != StatusPendingFeePayment || order.RowVersion != 1 || order.RiderUserID != nil ||
		order.DeliveryFeeCents != 150 || order.RiderFeeCents != 100 || order.PlatformCutCents != 50 {
		t.Fatalf("created order=%#v", order)
	}

	at = at.Add(time.Minute)
	order, err = service.MarkPaymentFailed(ctx, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil || order.Status != StatusPaymentFailed || !sameTime(order.PaymentFailedAt, at) {
		t.Fatalf("payment failed=%#v err=%v", order, err)
	}
	at = at.Add(time.Minute)
	order, err = service.RetryFeePayment(ctx, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil || order.Status != StatusPendingFeePayment {
		t.Fatalf("payment retry=%#v err=%v", order, err)
	}
	at = at.Add(time.Minute)
	readyVersion := order.RowVersion
	order, err = service.MarkFeePaid(ctx, order.DeliveryOrderPublicID, readyVersion)
	if err != nil || order.Status != StatusReadyForClaim || !sameTime(order.ReadyForClaimAt, at) {
		t.Fatalf("fee paid=%#v err=%v", order, err)
	}
	idempotentReady, err := service.MarkFeePaid(ctx, order.DeliveryOrderPublicID, readyVersion)
	if err != nil || idempotentReady.RowVersion != order.RowVersion {
		t.Fatalf("idempotent ready=%#v err=%v", idempotentReady, err)
	}
	if _, err := service.AssignRider(ctx, order.DeliveryOrderPublicID, readyVersion, acceptanceRiderOne); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale rider assignment returned %v", err)
	}

	at = at.Add(time.Minute)
	claimVersion := order.RowVersion
	order, err = service.AssignRider(ctx, order.DeliveryOrderPublicID, claimVersion, acceptanceRiderOne)
	if err != nil || order.Status != StatusClaimed || order.RiderUserID == nil ||
		*order.RiderUserID != acceptanceRiderOne || !sameTime(order.ClaimedAt, at) {
		t.Fatalf("claimed=%#v err=%v", order, err)
	}
	idempotentClaim, err := service.AssignRider(ctx, order.DeliveryOrderPublicID, claimVersion, acceptanceRiderOne)
	if err != nil || idempotentClaim.RowVersion != order.RowVersion {
		t.Fatalf("idempotent claim=%#v err=%v", idempotentClaim, err)
	}
	if _, err := service.AssignRider(ctx, order.DeliveryOrderPublicID, claimVersion, acceptanceRiderTwo); !errors.Is(err, ErrRiderMismatch) {
		t.Fatalf("different rider idempotency returned %v", err)
	}

	at = at.Add(time.Minute)
	order, err = service.StartToPickup(ctx, order.DeliveryOrderPublicID, order.RowVersion, acceptanceRiderOne)
	if err != nil || order.Status != StatusToPickup || !sameTime(order.ToPickupAt, at) {
		t.Fatalf("to pickup=%#v err=%v", order, err)
	}
	at = at.Add(time.Minute)
	order, err = service.ReleaseToPool(ctx, order.DeliveryOrderPublicID, StatusToPickup, order.RowVersion, acceptanceRiderOne)
	if err != nil || order.Status != StatusReadyForClaim || order.RiderUserID != nil ||
		!sameTime(order.ReadyForClaimAt, at) {
		t.Fatalf("release to pool=%#v err=%v", order, err)
	}
	at = at.Add(time.Minute)
	order, err = service.AssignRider(ctx, order.DeliveryOrderPublicID, order.RowVersion, acceptanceRiderTwo)
	if err != nil {
		t.Fatal(err)
	}
	at = at.Add(time.Minute)
	order, err = service.StartToPickup(ctx, order.DeliveryOrderPublicID, order.RowVersion, acceptanceRiderTwo)
	if err != nil {
		t.Fatal(err)
	}
	at = at.Add(time.Minute)
	order, err = service.MarkPickedUp(ctx, order.DeliveryOrderPublicID, order.RowVersion, acceptanceRiderTwo)
	if err != nil || order.Status != StatusPickedUp || !sameTime(order.PickedUpAt, at) {
		t.Fatalf("picked up=%#v err=%v", order, err)
	}
	if _, err := service.Cancel(ctx, order.DeliveryOrderPublicID, StatusPickedUp, order.RowVersion); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("post-pickup cancellation returned %v", err)
	}
	at = at.Add(time.Minute)
	order, err = service.StartDelivering(ctx, order.DeliveryOrderPublicID, order.RowVersion, acceptanceRiderTwo)
	if err != nil || order.Status != StatusDelivering || !sameTime(order.DeliveringAt, at) {
		t.Fatalf("delivering=%#v err=%v", order, err)
	}
	at = at.Add(time.Minute)
	deliverVersion := order.RowVersion
	order, err = service.MarkDelivered(ctx, order.DeliveryOrderPublicID, deliverVersion, acceptanceRiderTwo)
	if err != nil || order.Status != StatusDelivered || !order.Status.Terminal() ||
		!sameTime(order.DeliveredAt, at) {
		t.Fatalf("delivered=%#v err=%v", order, err)
	}
	idempotentDelivered, err := service.MarkDelivered(ctx, order.DeliveryOrderPublicID, deliverVersion, acceptanceRiderTwo)
	if err != nil || idempotentDelivered.RowVersion != order.RowVersion {
		t.Fatalf("idempotent delivered=%#v err=%v", idempotentDelivered, err)
	}
	if _, err := service.Cancel(ctx, order.DeliveryOrderPublicID, StatusDelivered, order.RowVersion); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("terminal order exited via cancel: %v", err)
	}
	loaded, err := service.FindOrder(ctx, order.DeliveryOrderPublicID)
	if err != nil || loaded.Status != StatusDelivered || loaded.RowVersion != order.RowVersion {
		t.Fatalf("loaded=%#v err=%v", loaded, err)
	}

	cancelled := createD1Order(t, ctx, service)
	cancelVersion := cancelled.RowVersion
	at = at.Add(time.Minute)
	cancelled, err = service.Cancel(ctx, cancelled.DeliveryOrderPublicID, StatusPendingFeePayment, cancelVersion)
	if err != nil || cancelled.Status != StatusCancelled || !sameTime(cancelled.CancelledAt, at) {
		t.Fatalf("cancelled=%#v err=%v", cancelled, err)
	}
	idempotentCancelled, err := service.Cancel(ctx, cancelled.DeliveryOrderPublicID, StatusPendingFeePayment, cancelVersion)
	if err != nil || idempotentCancelled.RowVersion != cancelled.RowVersion {
		t.Fatalf("idempotent cancelled=%#v err=%v", idempotentCancelled, err)
	}

	if err := db.Model(&Order{}).Where("id = ?", cancelled.ID).
		Update("delivery_fee_cents", 999).Error; err == nil {
		t.Fatal("fee snapshot CHECK accepted mismatched amounts")
	}
	if err := db.Model(&Order{}).Where("id = ?", cancelled.ID).
		Update("status", "not_a_delivery_status").Error; err == nil {
		t.Fatal("status CHECK accepted invalid status")
	}
	t.Log("delivery_lifecycle=PASS retry+requeue+idempotency+rider_mismatch+timestamps+terminal+checks")
}

func TestDeliveryEveryLegalTransitionAgainstRealMySQL84(t *testing.T) {
	db := openD1MySQLWithTable(t)
	cleanupD1Orders(t, db)
	t.Cleanup(func() { cleanupD1Orders(t, db) })
	ctx := context.Background()
	repository := NewRepository(db)
	riderID := acceptanceRiderOne
	edges := []struct {
		from Status
		to   Status
	}{
		{StatusPendingFeePayment, StatusReadyForClaim},
		{StatusPendingFeePayment, StatusPaymentFailed},
		{StatusPendingFeePayment, StatusCancelled},
		{StatusPaymentFailed, StatusPendingFeePayment},
		{StatusPaymentFailed, StatusCancelled},
		{StatusReadyForClaim, StatusClaimed},
		{StatusReadyForClaim, StatusCancelled},
		{StatusClaimed, StatusToPickup},
		{StatusClaimed, StatusReadyForClaim},
		{StatusClaimed, StatusCancelled},
		{StatusToPickup, StatusPickedUp},
		{StatusToPickup, StatusReadyForClaim},
		{StatusToPickup, StatusCancelled},
		{StatusPickedUp, StatusDelivering},
		{StatusDelivering, StatusDelivered},
	}
	for index, edge := range edges {
		t.Run(string(edge.from)+"_to_"+string(edge.to), func(t *testing.T) {
			at := time.Date(2026, 6, 29, 3, index, 0, 0, time.UTC)
			order := validNewOrder(t, at)
			publicID, err := NewULID()
			if err != nil {
				t.Fatal(err)
			}
			order.DeliveryOrderPublicID = publicID
			order.MerchantUserID = acceptanceMerchant
			order.Status = edge.from
			if statusRequiresRider(edge.from) {
				order.RiderUserID = uint64Pointer(riderID)
			}
			if err := db.Create(&order).Error; err != nil {
				t.Fatal(err)
			}
			var rider *uint64
			if edge.to == StatusClaimed || statusRequiresRider(edge.from) {
				rider = &riderID
			}
			updated, err := repository.Transition(
				ctx, order.DeliveryOrderPublicID, order.RowVersion,
				edge.from, edge.to, rider, at.Add(time.Second),
			)
			if err != nil {
				t.Fatal(err)
			}
			if updated.Status != edge.to || updated.RowVersion != 2 {
				t.Fatalf("transition result=%#v", updated)
			}
		})
	}
	t.Logf("delivery_legal_transition_matrix=PASS edges=%d", len(edges))
}

func TestDeliveryConcurrentCASOnlyOneTransitionWinsAgainstRealMySQL84(t *testing.T) {
	db := openD1MySQLWithTable(t)
	cleanupD1Orders(t, db)
	t.Cleanup(func() { cleanupD1Orders(t, db) })
	ctx := context.Background()
	at := time.Date(2026, 6, 29, 4, 0, 0, 0, time.UTC)
	service, _ := NewService(NewRepository(db), func() time.Time { return at })
	order := createD1Order(t, ctx, service)

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		_, err := service.MarkFeePaid(ctx, order.DeliveryOrderPublicID, order.RowVersion)
		results <- err
	}()
	go func() {
		defer wait.Done()
		<-start
		_, err := service.MarkPaymentFailed(ctx, order.DeliveryOrderPublicID, order.RowVersion)
		results <- err
	}()
	close(start)
	wait.Wait()
	close(results)

	var successes, conflicts int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrConcurrentUpdate):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent results successes=%d conflicts=%d", successes, conflicts)
	}
	var count int64
	if err := db.Model(&Order{}).Where("delivery_order_public_id = ?", order.DeliveryOrderPublicID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("order count=%d", count)
	}
	t.Log("delivery_cas_competition=PASS winners=1 conflicts=1")
}

func TestZZDeliveryAcceptanceResidueAgainstRealMySQL84(t *testing.T) {
	db := openD1MySQLWithTable(t)
	cleanupD1Orders(t, db)
	var count int64
	if err := db.Model(&Order{}).Where("merchant_user_id IN ?", acceptanceMerchantIDs).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("delivery acceptance residue=%d", count)
	}
	t.Log("delivery_residue acceptance_orders=0")
}

func createD1Order(t *testing.T, ctx context.Context, service *Service) Order {
	t.Helper()
	order, err := service.CreateOrder(ctx, CreateOrderInput{
		MerchantUserID: acceptanceMerchant,
		Pickup: Contact{
			Name: "D1 Merchant", Phone: "13800001001", Address: "Campus Gate 1",
		},
		Recipient: Contact{
			Name: "D1 Recipient", Phone: "13800001002", Address: "Dormitory 2-201",
		},
		Remark:           "D-1 acceptance",
		DeliveryFeeCents: 150, RiderFeeCents: 100, PlatformCutCents: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	return order
}

func openD1MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DELIVERY_D1_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set DELIVERY_D1_MYSQL_DSN to run D-1 real MySQL acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func openD1MySQLWithTable(t *testing.T) *gorm.DB {
	t.Helper()
	db := openD1MySQL(t)
	if !db.Migrator().HasTable(&Order{}) {
		t.Fatal("required migration 023 is missing")
	}
	return db
}

func assertD1MySQL84(t *testing.T, db *gorm.DB) {
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

func applyD1SQLFile(t *testing.T, db *gorm.DB, relativePath string) {
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

func assertD1Schema(t *testing.T, db *gorm.DB) {
	t.Helper()
	var columns int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema=DATABASE() AND table_name='delivery_orders'`).Scan(&columns).Error; err != nil {
		t.Fatal(err)
	}
	if columns != 26 {
		t.Fatalf("delivery_orders columns=%d want=26", columns)
	}
	for column, expected := range map[string]string{
		"delivery_order_public_id": "char(26)",
		"merchant_user_id":         "bigint unsigned",
		"rider_user_id":            "bigint unsigned",
		"delivery_fee_cents":       "bigint unsigned",
		"rider_fee_cents":          "bigint unsigned",
		"platform_cut_cents":       "bigint unsigned",
		"status":                   "varchar(24)",
		"row_version":              "bigint unsigned",
	} {
		var actual string
		if err := db.Raw(`SELECT column_type FROM information_schema.columns
			WHERE table_schema=DATABASE() AND table_name='delivery_orders' AND column_name=?`,
			column).Scan(&actual).Error; err != nil {
			t.Fatal(err)
		}
		if actual != expected {
			t.Errorf("delivery_orders.%s type=%s want=%s", column, actual, expected)
		}
	}
	for _, index := range []string{
		"uq_delivery_order_public_id", "idx_delivery_order_merchant_created",
		"idx_delivery_order_status_created", "idx_delivery_order_rider_status",
	} {
		var count int64
		if err := db.Raw(`SELECT COUNT(*) FROM information_schema.statistics
			WHERE table_schema=DATABASE() AND table_name='delivery_orders' AND index_name=?`,
			index).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			t.Errorf("missing index delivery_orders.%s", index)
		}
	}
	var foreignKeys int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.key_column_usage
		WHERE table_schema=DATABASE() AND table_name='delivery_orders'
		AND referenced_table_name IS NOT NULL`).Scan(&foreignKeys).Error; err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 0 {
		t.Fatalf("delivery foreign keys=%d", foreignKeys)
	}
	checks := make(map[string]string)
	rows, err := db.Raw(`SELECT tc.constraint_name, cc.check_clause
		FROM information_schema.table_constraints tc
		JOIN information_schema.check_constraints cc
		  ON cc.constraint_schema=tc.constraint_schema AND cc.constraint_name=tc.constraint_name
		WHERE tc.table_schema=DATABASE() AND tc.table_name='delivery_orders'
		  AND tc.constraint_type='CHECK'`).Rows()
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, clause string
		if err := rows.Scan(&name, &clause); err != nil {
			t.Fatal(err)
		}
		checks[name] = clause
	}
	statusCheck := checks["chk_delivery_order_status"]
	for _, status := range []Status{
		StatusPendingFeePayment, StatusPaymentFailed, StatusReadyForClaim, StatusClaimed,
		StatusToPickup, StatusPickedUp, StatusDelivering, StatusDelivered, StatusCancelled,
	} {
		if !strings.Contains(statusCheck, string(status)) {
			t.Errorf("status CHECK missing %s: %s", status, statusCheck)
		}
	}
	feeCheck := checks["chk_delivery_order_fee_snapshot"]
	for _, field := range []string{"delivery_fee_cents", "rider_fee_cents", "platform_cut_cents"} {
		if !strings.Contains(strings.ToLower(feeCheck), field) {
			t.Errorf("fee CHECK missing %s: %s", field, feeCheck)
		}
	}
}

func cleanupD1Orders(t *testing.T, db *gorm.DB) {
	t.Helper()
	if !db.Migrator().HasTable(&Order{}) {
		return
	}
	var orderIDs []string
	if err := db.Model(&Order{}).
		Where("merchant_user_id IN ?", acceptanceMerchantIDs).
		Pluck("delivery_order_public_id", &orderIDs).Error; err != nil {
		t.Errorf("list D-1 delivery orders: %v", err)
	}
	cleanupD6RiderMoney(t, db, []uint64{acceptanceRiderOne, acceptanceRiderTwo}, orderIDs)
	if err := db.Where("merchant_user_id IN ?", acceptanceMerchantIDs).Delete(&Order{}).Error; err != nil {
		t.Errorf("cleanup delivery orders: %v", err)
	}
}

func sameTime(value *time.Time, expected time.Time) bool {
	return value != nil && value.Equal(expected)
}

func statusRequiresRider(status Status) bool {
	switch status {
	case StatusClaimed, StatusToPickup, StatusPickedUp, StatusDelivering, StatusDelivered:
		return true
	default:
		return false
	}
}
