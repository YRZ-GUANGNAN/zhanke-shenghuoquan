package delivery

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func riderAvailabilitySchema(t *testing.T) *gorm.DB {
	t.Helper()
	db := openIsolatedRiderMoneyMySQL(t)
	for _, name := range []string{"023_delivery_order.sql", "024_delivery_dispatch.sql", "045_delivery_order_source.sql", "055_delivery_rider_availability.sql"} {
		applyD1SQLFile(t, db, "../../../migrations/"+name)
	}
	return db
}

func testRiderEligibility(_ context.Context, tx *gorm.DB, _ uint64) (string, error) {
	if _, ok := tx.Statement.ConnPool.(*sql.Tx); !ok {
		return "", errors.New("eligibility must execute inside the claim transaction")
	}
	return "user-1:rider-1", nil
}

func seedAvailabilityOrder(t *testing.T, db *gorm.DB, riderID *uint64, status Status, now time.Time) Order {
	t.Helper()
	value := Order{DeliveryOrderPublicID: mustRiderMoneyULID(t), MerchantUserID: 996201,
		RiderUserID: riderID, SourcePlatform: SourceLocal, DeliveryFeeCents: 150,
		RiderFeeCents: 100, PlatformCutCents: 50, Status: status, RowVersion: 1,
		CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&value).Error; err != nil {
		t.Fatal(err)
	}
	return value
}

func TestRiderAvailabilityReadOnlyDefaultAndCASAgainstIsolatedMySQL(t *testing.T) {
	db := riderAvailabilitySchema(t)
	store := NewRiderAvailabilityStore(db, nil, testRiderEligibility)
	ctx := context.Background()
	now := time.Date(2026, 11, 4, 8, 0, 0, 0, time.UTC)
	const rider = uint64(996101)
	initial, err := store.ReadRiderAvailability(ctx, rider)
	if err != nil || !initial.Valid() || initial.State != RiderOffDuty || initial.RowVersion != 0 || !initial.UpdatedAt.IsZero() {
		t.Fatalf("initial read: %+v %v", initial, err)
	}
	assertRiderMoneyCount(t, db, "delivery_rider_availability", 0)
	started, err := store.SetRiderDuty(ctx, rider, RiderOnDuty, 0, now)
	if err != nil || started.RowVersion != 1 || started.State != RiderOnDuty {
		t.Fatalf("start: %+v %v", started, err)
	}
	if _, err := store.SetRiderDuty(ctx, rider, RiderOnDuty, 0, now); !errors.Is(err, ErrRiderAvailabilityConflict) {
		t.Fatalf("stale start: %v", err)
	}
	stopped, err := store.SetRiderDuty(ctx, rider, RiderOffDuty, 1, now.Add(time.Second))
	if err != nil || stopped.RowVersion != 2 || stopped.State != RiderOffDuty {
		t.Fatalf("stop: %+v %v", stopped, err)
	}
	if _, err := store.SetRiderDuty(ctx, rider, RiderOnDuty, 1, now); !errors.Is(err, ErrRiderAvailabilityConflict) {
		t.Fatalf("stale resume: %v", err)
	}
	config, err := store.ReadRiderBroadcastConfig(ctx)
	if err != nil || config.MaxInFlightPerRider != 3 || config.RowVersion != 1 {
		t.Fatalf("config: %+v %v", config, err)
	}
	config, err = store.UpdateRiderBroadcastConfig(ctx, 1, 1, now)
	if err != nil || config.MaxInFlightPerRider != 1 || config.RowVersion != 2 {
		t.Fatalf("config update: %+v %v", config, err)
	}
	if _, err := store.UpdateRiderBroadcastConfig(ctx, 2, 1, now); !errors.Is(err, ErrRiderAvailabilityConflict) {
		t.Fatalf("stale config: %v", err)
	}
	for _, invalid := range []uint32{0, 101} {
		if _, err := store.UpdateRiderBroadcastConfig(ctx, invalid, 2, now); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("invalid max %d: %v", invalid, err)
		}
	}
	for _, status := range []Status{StatusClaimed, StatusToPickup, StatusPickedUp, StatusDelivering, StatusDelivered, StatusCancelled} {
		seedAvailabilityOrder(t, db, ptrRiderID(rider), status, now)
	}
	snapshot, err := store.ReadRiderAvailability(ctx, rider)
	if err != nil || !snapshot.Valid() || snapshot.InFlight != 4 || snapshot.MaxInFlight != 1 {
		t.Fatalf("lowered capacity must remain readable: %+v %v", snapshot, err)
	}
	t.Log("initial_get_no_write=PASS duty_and_config_cas=PASS over_capacity_snapshot=PASS")
}

func ptrRiderID(value uint64) *uint64 { return &value }

func TestRiderAvailabilityFailClosedAgainstIsolatedMySQL(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	db := openIsolatedRiderMoneyMySQL(t)
	store := NewRiderAvailabilityStore(db, nil, testRiderEligibility)
	if _, err := store.ReadRiderAvailability(ctx, 1); !errors.Is(err, ErrRiderAvailabilityUnavailable) {
		t.Fatalf("missing tables: %v", err)
	}
	for _, name := range []string{"023_delivery_order.sql", "024_delivery_dispatch.sql", "045_delivery_order_source.sql", "055_delivery_rider_availability.sql"} {
		applyD1SQLFile(t, db, "../../../migrations/"+name)
	}
	if _, err := NewRiderAvailabilityStore(db, nil, nil).ReadRiderAvailability(ctx, 1); !errors.Is(err, ErrRiderAvailabilityUnavailable) {
		t.Fatalf("missing callback: %v", err)
	}
	invalidRevision := NewRiderAvailabilityStore(db, nil, func(context.Context, *gorm.DB, uint64) (string, error) { return "", nil })
	if _, err := invalidRevision.SetRiderDuty(ctx, 1, RiderOnDuty, 0, now); !errors.Is(err, ErrRiderAvailabilityUnavailable) {
		t.Fatalf("empty revision: %v", err)
	}
	denied := NewRiderAvailabilityStore(db, nil, func(context.Context, *gorm.DB, uint64) (string, error) { return "", ErrRiderNotRegistered })
	if _, err := denied.SetRiderDuty(ctx, 1, RiderOnDuty, 0, now); !errors.Is(err, ErrRiderNotRegistered) {
		t.Fatalf("denied start: %v", err)
	}
	assertRiderMoneyCount(t, db, "delivery_rider_availability", 0)
	if err := db.Where("config_scope = ?", "global").Delete(&RiderBroadcastConfig{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadRiderAvailability(ctx, 1); !errors.Is(err, ErrRiderAvailabilityUnavailable) {
		t.Fatalf("missing config read: %v", err)
	}
	if _, err := store.SetRiderDuty(ctx, 1, RiderOnDuty, 0, now); !errors.Is(err, ErrRiderAvailabilityUnavailable) {
		t.Fatalf("missing config start: %v", err)
	}
	assertRiderMoneyCount(t, db, "delivery_rider_availability", 0)
}

func TestRiderAvailabilityConcurrentFirstStartAndReadAgainstIsolatedMySQL(t *testing.T) {
	db := riderAvailabilitySchema(t)
	store := NewRiderAvailabilityStore(db, nil, testRiderEligibility)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	now := time.Now().UTC()
	start := make(chan struct{})
	results := make(chan error, 8)
	reads := make(chan error, 8)
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(2)
		go func() {
			defer group.Done()
			<-start
			_, err := store.SetRiderDuty(ctx, 996102, RiderOnDuty, 0, now)
			results <- err
		}()
		go func() { defer group.Done(); <-start; _, err := store.ReadRiderAvailability(ctx, 996102); reads <- err }()
	}
	close(start)
	group.Wait()
	close(results)
	close(reads)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrRiderAvailabilityConflict) {
			t.Fatalf("concurrent first start: %v", err)
		}
	}
	for err := range reads {
		if err != nil {
			t.Fatalf("concurrent read: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("initial start successes=%d, want 1", successes)
	}
	assertRiderMoneyCount(t, db, "delivery_rider_availability", 1)
	// Distinct missing IDs must not hold compatible gaps while serializing config.
	start, results = make(chan struct{}), make(chan error, 8)
	for i := uint64(0); i < 8; i++ {
		id := uint64(996110) + i
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, err := store.SetRiderDuty(ctx, id, RiderOnDuty, 0, now)
			results <- err
		}()
	}
	close(start)
	group.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("distinct initial start: %v", err)
		}
	}
	t.Log("first_start_8_workers=PASS concurrent_missing_get=PASS no_gap_lock_deadlock=true")
}

func TestRiderAvailabilityConcurrentClaimsRespectCapacityAgainstIsolatedMySQL(t *testing.T) {
	db := riderAvailabilitySchema(t)
	store := NewRiderAvailabilityStore(db, nil, testRiderEligibility)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	now := time.Now().UTC()
	const rider = uint64(996103)
	if _, err := store.SetRiderDuty(ctx, rider, RiderOnDuty, 0, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateRiderBroadcastConfig(ctx, 1, 1, now); err != nil {
		t.Fatal(err)
	}
	orders := make([]Order, 8)
	for i := range orders {
		orders[i] = seedAvailabilityOrder(t, db, nil, StatusReadyForClaim, now)
	}
	type outcome struct {
		order Order
		err   error
	}
	start := make(chan struct{})
	results := make(chan outcome, len(orders))
	var group sync.WaitGroup
	for _, order := range orders {
		order := order
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			value, err := store.ClaimOrderWithAvailability(ctx, rider, order.DeliveryOrderPublicID, 1, now)
			results <- outcome{value, err}
		}()
	}
	close(start)
	group.Wait()
	close(results)
	successes, capacity := 0, 0
	var claimed Order
	for result := range results {
		switch {
		case result.err == nil:
			successes++
			claimed = result.order
		case errors.Is(result.err, ErrRiderAtCapacity):
			capacity++
		default:
			t.Fatalf("unexpected concurrent claim: %v", result.err)
		}
	}
	if successes != 1 || capacity != 7 {
		t.Fatalf("success=%d capacity=%d, want 1/7", successes, capacity)
	}
	if replay, err := store.ClaimOrderWithAvailability(ctx, rider, claimed.DeliveryOrderPublicID, 1, now); err != nil || replay.RowVersion != 2 {
		t.Fatalf("claim replay: %+v %v", replay, err)
	}
	snapshot, err := store.ReadRiderAvailability(ctx, rider)
	if err != nil || snapshot.InFlight != 1 {
		t.Fatalf("capacity after claims: %+v %v", snapshot, err)
	}
	if candidates, err := store.ListOnDutyRiderAvailability(ctx, now); err != nil || len(candidates) != 0 {
		t.Fatalf("full rider candidate: %+v %v", candidates, err)
	}
	if err := db.Model(&Order{}).Where("id = ?", claimed.ID).Update("status", StatusDelivered).Error; err != nil {
		t.Fatal(err)
	}
	if candidates, err := store.ListOnDutyRiderAvailability(ctx, now); err != nil || len(candidates) != 1 {
		t.Fatalf("released capacity candidate: %+v %v", candidates, err)
	}
	t.Log("claim_8_workers_max_1=PASS claim_replay_once=PASS capacity_release=PASS")
}

func TestRiderAvailabilitySameOrderRaceAndStopAgainstIsolatedMySQL(t *testing.T) {
	db := riderAvailabilitySchema(t)
	store := NewRiderAvailabilityStore(db, nil, testRiderEligibility)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	now := time.Now().UTC()
	for _, rider := range []uint64{996104, 996105} {
		if _, err := store.SetRiderDuty(ctx, rider, RiderOnDuty, 0, now); err != nil {
			t.Fatal(err)
		}
	}
	order := seedAvailabilityOrder(t, db, nil, StatusReadyForClaim, now)
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup
	for _, rider := range []uint64{996104, 996105} {
		rider := rider
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, err := store.ClaimOrderWithAvailability(ctx, rider, order.DeliveryOrderPublicID, 1, now)
			results <- err
		}()
	}
	close(start)
	group.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrConcurrentUpdate) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("same order success=%d conflict=%d", successes, conflicts)
	}
	if _, err := store.SetRiderDuty(ctx, 996104, RiderOffDuty, 1, now); err != nil {
		t.Fatal(err)
	}
	unclaimed := seedAvailabilityOrder(t, db, nil, StatusReadyForClaim, now)
	if _, err := store.ClaimOrderWithAvailability(ctx, 996104, unclaimed.DeliveryOrderPublicID, 1, now); !errors.Is(err, ErrRiderOffDuty) {
		t.Fatalf("stopped rider claim: %v", err)
	}
	if _, err := store.ClaimOrderWithAvailability(ctx, 996106, unclaimed.DeliveryOrderPublicID, 1, now); !errors.Is(err, ErrRiderOffDuty) {
		t.Fatalf("never-started rider claim: %v", err)
	}
	var count int64
	if err := db.Model(&RiderAvailability{}).Where("rider_user_id = ?", 996106).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("claim created duty row: %d %v", count, err)
	}
}

func TestRiderAvailabilityEligibilityRevisionRequiresExplicitRestartAgainstIsolatedMySQL(t *testing.T) {
	db := riderAvailabilitySchema(t)
	// This test-only identity table models a composition-root adapter without
	// importing an identity domain into delivery.
	if err := db.Exec("CREATE TABLE availability_test_identity (id BIGINT UNSIGNED PRIMARY KEY, revision BIGINT UNSIGNED NOT NULL, active BOOLEAN NOT NULL)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO availability_test_identity VALUES (996107, 1, TRUE)").Error; err != nil {
		t.Fatal(err)
	}
	eligible := func(ctx context.Context, tx *gorm.DB, id uint64) (string, error) {
		if _, err := testRiderEligibility(ctx, tx, id); err != nil {
			return "", err
		}
		var row struct {
			ID, Revision uint64
			Active       bool
		}
		if err := tx.Table("availability_test_identity").Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ?", id).Take(&row).Error; err != nil {
			return "", err
		}
		if !row.Active {
			return "", ErrRiderUnavailable
		}
		return fmt.Sprintf("identity-%d", row.Revision), nil
	}
	store := NewRiderAvailabilityStore(db, nil, eligible)
	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := store.SetRiderDuty(ctx, 996107, RiderOnDuty, 0, now); err != nil {
		t.Fatal(err)
	}
	// Revoke and restore without any intervening availability read.
	if err := db.Exec("UPDATE availability_test_identity SET revision = 2, active = FALSE WHERE id = 996107").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE availability_test_identity SET revision = 3, active = TRUE WHERE id = 996107").Error; err != nil {
		t.Fatal(err)
	}
	order := seedAvailabilityOrder(t, db, nil, StatusReadyForClaim, now)
	if _, err := store.ClaimOrderWithAvailability(ctx, 996107, order.DeliveryOrderPublicID, 1, now); !errors.Is(err, ErrRiderAvailabilityConflict) {
		t.Fatalf("restored identity auto-resumed claim: %v", err)
	}
	snapshot, err := store.ReadRiderAvailability(ctx, 996107)
	if err != nil || snapshot.State != RiderOffDuty || snapshot.RowVersion != 2 {
		t.Fatalf("persistent revision invalidation: %+v %v", snapshot, err)
	}
	if _, err := store.SetRiderDuty(ctx, 996107, RiderOnDuty, 1, now); !errors.Is(err, ErrRiderAvailabilityConflict) {
		t.Fatalf("old duty version restarted: %v", err)
	}
	if _, err := store.SetRiderDuty(ctx, 996107, RiderOnDuty, 2, now); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE availability_test_identity SET revision = 4, active = FALSE WHERE id = 996107").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadRiderAvailability(ctx, 996107); !errors.Is(err, ErrRiderUnavailable) {
		t.Fatalf("disabled identity read: %v", err)
	}
	var duty RiderAvailability
	if err := db.Where("rider_user_id = ?", 996107).First(&duty).Error; err != nil || duty.State != RiderOffDuty || duty.RowVersion != 4 {
		t.Fatalf("revocation not persisted: %+v %v", duty, err)
	}
	if err := db.Exec("UPDATE availability_test_identity SET revision = 5, active = TRUE WHERE id = 996107").Error; err != nil {
		t.Fatal(err)
	}
	if candidates, err := store.ListOnDutyRiderAvailability(ctx, now); err != nil || len(candidates) != 0 {
		t.Fatalf("revoked candidate restored implicitly: %+v %v", candidates, err)
	}
	if _, err := store.ClaimOrderWithAvailability(ctx, 996107, order.DeliveryOrderPublicID, 1, now); !errors.Is(err, ErrRiderOffDuty) {
		t.Fatalf("restored identity implicitly on duty: %v", err)
	}
	t.Log("revoke_restore_without_read=PASS transaction_eligibility=PASS persistent_off_duty=PASS")
}

func TestRiderAvailabilityNotificationFailureRollsBackClaimAgainstIsolatedMySQL(t *testing.T) {
	db := riderAvailabilitySchema(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := db.Exec("CREATE TABLE availability_test_notifications (id CHAR(26) PRIMARY KEY)").Error; err != nil {
		t.Fatal(err)
	}
	failure := errors.New("isolated outbox append failure")
	appender := func(tx *gorm.DB, event NotificationEvent) error {
		if err := tx.Exec("INSERT INTO availability_test_notifications VALUES (?)", event.EventPublicID).Error; err != nil {
			return err
		}
		return failure
	}
	store := NewRiderAvailabilityStore(db, appender, testRiderEligibility)
	if _, err := store.SetRiderDuty(ctx, 996108, RiderOnDuty, 0, now); err != nil {
		t.Fatal(err)
	}
	order := seedAvailabilityOrder(t, db, nil, StatusReadyForClaim, now)
	if _, err := store.ClaimOrderWithAvailability(ctx, 996108, order.DeliveryOrderPublicID, 1, now); !errors.Is(err, failure) {
		t.Fatalf("append failure: %v", err)
	}
	var current Order
	if err := db.First(&current, order.ID).Error; err != nil || current.Status != StatusReadyForClaim || current.RowVersion != 1 || current.RiderUserID != nil {
		t.Fatalf("partial claim committed: %+v %v", current, err)
	}
	assertRiderMoneyCount(t, db, "availability_test_notifications", 0)
	if snapshot, err := store.ReadRiderAvailability(ctx, 996108); err != nil || snapshot.InFlight != 0 {
		t.Fatalf("rolled-back capacity: %+v %v", snapshot, err)
	}
}

func TestRiderAvailabilityMigrationRollbackGuardsAgainstIsolatedMySQL(t *testing.T) {
	const up = "../../../migrations/055_delivery_rider_availability.sql"
	const down = "../../../migrations/rollback/055_delivery_rider_availability.down.sql"
	db := riderAvailabilitySchema(t)
	if err := executeRiderMoneySQL(db, down); err != nil {
		t.Fatalf("empty rollback: %v", err)
	}
	applyD1SQLFile(t, db, up)
	store := NewRiderAvailabilityStore(db, nil, testRiderEligibility)
	if _, err := store.UpdateRiderBroadcastConfig(context.Background(), 2, 1, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	err := executeRiderMoneySQL(db, down)
	var mysqlError *mysqlDriver.MySQLError
	if !errors.As(err, &mysqlError) || mysqlError.Number != 1644 {
		t.Fatalf("changed config rollback accepted: %v", err)
	}
	if !db.Migrator().HasTable(&RiderAvailability{}) || !db.Migrator().HasTable(&RiderBroadcastConfig{}) {
		t.Fatal("guard partially dropped tables")
	}
	dbWithDuty := riderAvailabilitySchema(t)
	store = NewRiderAvailabilityStore(dbWithDuty, nil, testRiderEligibility)
	if _, err := store.SetRiderDuty(context.Background(), 996109, RiderOffDuty, 0, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	err = executeRiderMoneySQL(dbWithDuty, down)
	if !errors.As(err, &mysqlError) || mysqlError.Number != 1644 {
		t.Fatalf("duty fact rollback accepted: %v", err)
	}
	assertRiderMoneyCount(t, dbWithDuty, "delivery_rider_availability", 1)
	t.Log("empty_rollback=PASS changed_config_guard=PASS duty_fact_guard=PASS")
}
