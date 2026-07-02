package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
)

const (
	dispatchMerchant = uint64(983001)
	dispatchRiderOne = uint64(984001)
	dispatchRiderTwo = uint64(984002)
)

type activeEligibility struct{}

func (activeEligibility) RequireActive(context.Context, uint64) error { return nil }

func Test01DispatchMigrationRollbackApplyAgainstRealMySQL84(t *testing.T) {
	db := openD1MySQL(t)
	assertD1MySQL84(t, db)
	normalizeD2MigrationState(t, db)
	cleanupD2(t, db)
	var outboxCount, orderCount int64
	if err := db.Model(&OutboxEvent{}).Count(&outboxCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&Order{}).Where("merchant_request_id IS NOT NULL").Count(&orderCount).Error; err != nil {
		t.Fatal(err)
	}
	if outboxCount != 0 || orderCount != 0 {
		t.Fatalf("refusing D-2 rollback outbox=%d idempotent_orders=%d", outboxCount, orderCount)
	}
	applyD1SQLFile(t, db, "../../../migrations/rollback/024_delivery_dispatch.sql")
	if db.Migrator().HasTable(&OutboxEvent{}) || db.Migrator().HasColumn(&Order{}, "merchant_request_id") {
		t.Fatal("rollback 024 left schema behind")
	}
	applyD1SQLFile(t, db, "../../../migrations/024_delivery_dispatch.sql")
	assertD2Schema(t, db)
	t.Log("migration_024_apply_rollback_apply=PASS outbox=1 merchant_request_unique=1 foreign_keys=0")
}

func TestDispatchIdempotencyOutboxClaimAndLifecycleAgainstRealMySQL84(t *testing.T) {
	db := openD2MySQL(t)
	cleanupD2(t, db)
	t.Cleanup(func() { cleanupD2(t, db) })
	ctx := context.Background()
	now := time.Date(2026, 6, 30, 2, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	base, _ := NewService(repository, func() time.Time { return now })
	dispatch, _ := NewDispatchService(
		repository, base, activeEligibility{},
		FeePolicy{DeliveryFeeCents: 150, RiderFeeCents: 100, PlatformCutCents: 50},
		func() time.Time { return now },
	)
	requestID := "01J00000000000000000000811"
	input := CreateDispatchInput{
		MerchantUserID: dispatchMerchant, MerchantRequestID: requestID,
		Pickup:    Contact{Name: "Merchant Secret", Phone: "13800008111", Address: "Shop Room 101"},
		Recipient: Contact{Name: "Recipient Secret", Phone: "13800008112", Address: "Dorm Room 202"},
		Remark:    "private remark",
	}
	order, err := dispatch.CreateOrder(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := dispatch.CreateOrder(ctx, input)
	if err != nil || repeated.DeliveryOrderPublicID != order.DeliveryOrderPublicID {
		t.Fatalf("idempotent create=%#v err=%v", repeated, err)
	}
	input.Recipient.Address = "different"
	if _, err := dispatch.CreateOrder(ctx, input); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflicting idempotency returned %v", err)
	}
	var count int64
	if err := db.Model(&Order{}).Where("merchant_user_id = ? AND merchant_request_id = ?",
		dispatchMerchant, requestID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("idempotent order count=%d err=%v", count, err)
	}
	if order.Status != StatusPendingFeePayment || order.DeliveryFeeCents != 150 ||
		order.RiderFeeCents != 100 || order.PlatformCutCents != 50 {
		t.Fatalf("server fee snapshot/order state=%#v", order)
	}

	now = now.Add(time.Minute)
	ready, event, err := dispatch.ConfirmMockFee(ctx, dispatchMerchant, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil || ready.Status != StatusReadyForClaim || event.AggregateVersion != ready.RowVersion ||
		event.Status != OutboxPending {
		t.Fatalf("ready=%#v event=%#v err=%v", ready, event, err)
	}
	repeatedReady, repeatedEvent, err := dispatch.ConfirmMockFee(
		ctx, dispatchMerchant, order.DeliveryOrderPublicID, order.RowVersion,
	)
	if err != nil || repeatedReady.RowVersion != ready.RowVersion ||
		repeatedEvent.EventPublicID != event.EventPublicID {
		t.Fatalf("idempotent ready=%#v event=%#v err=%v", repeatedReady, repeatedEvent, err)
	}
	if err := db.Model(&OutboxEvent{}).Where("aggregate_public_id = ?", order.DeliveryOrderPublicID).
		Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("outbox count=%d err=%v", count, err)
	}
	var payload RiderPoolOpenedPayload
	if err := jsonUnmarshal(event.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	for _, leaked := range []string{input.Pickup.Name, input.Pickup.Phone, input.Pickup.Address,
		input.Recipient.Name, input.Recipient.Phone, input.Recipient.Address, input.Remark} {
		if containsJSON(event.Payload, leaked) {
			t.Fatalf("outbox payload leaked private value %q: %s", leaked, event.Payload)
		}
	}

	start := make(chan struct{})
	type result struct {
		order Order
		err   error
		rider uint64
	}
	results := make(chan result, 2)
	var wait sync.WaitGroup
	for _, riderID := range []uint64{dispatchRiderOne, dispatchRiderTwo} {
		wait.Add(1)
		go func(id uint64) {
			defer wait.Done()
			<-start
			value, claimErr := dispatch.ClaimOrder(ctx, id, ready.DeliveryOrderPublicID, ready.RowVersion)
			results <- result{order: value, err: claimErr, rider: id}
		}(riderID)
	}
	close(start)
	wait.Wait()
	close(results)
	var winner Order
	var winnerID uint64
	var successes, conflicts int
	for item := range results {
		if item.err == nil {
			successes++
			winner = item.order
			winnerID = item.rider
		} else if errors.Is(item.err, ErrConcurrentUpdate) || errors.Is(item.err, ErrRiderMismatch) {
			conflicts++
		} else {
			t.Fatalf("unexpected claim error: %v", item.err)
		}
	}
	if successes != 1 || conflicts != 1 || winner.RiderUserID == nil || *winner.RiderUserID != winnerID {
		t.Fatalf("claim successes=%d conflicts=%d winner=%#v id=%d", successes, conflicts, winner, winnerID)
	}
	loserID := dispatchRiderOne
	if winnerID == loserID {
		loserID = dispatchRiderTwo
	}
	if _, err := dispatch.RiderOrder(ctx, loserID, winner.DeliveryOrderPublicID); !errors.Is(err, ErrForbiddenOrder) {
		t.Fatalf("non-owner rider detail returned %v", err)
	}
	if _, err := dispatch.StartToPickup(ctx, winnerID, winner.DeliveryOrderPublicID, ready.RowVersion); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale fulfillment returned %v", err)
	}
	now = now.Add(time.Minute)
	winner, err = dispatch.StartToPickup(ctx, winnerID, winner.DeliveryOrderPublicID, winner.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	winner, err = dispatch.MarkPickedUp(ctx, winnerID, winner.DeliveryOrderPublicID, winner.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	winner, err = dispatch.StartDelivering(ctx, winnerID, winner.DeliveryOrderPublicID, winner.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	winner, err = dispatch.MarkDelivered(ctx, winnerID, winner.DeliveryOrderPublicID, winner.RowVersion)
	if err != nil || winner.Status != StatusDelivered {
		t.Fatalf("delivered=%#v err=%v", winner, err)
	}
	t.Logf("dispatch_claim_competition=PASS winner=%d conflict=1 lifecycle=delivered", winnerID)
}

func TestDispatchOutboxLeaseRetryDeadletterAndSkipAgainstRealMySQL84(t *testing.T) {
	db := openD2MySQL(t)
	cleanupD2(t, db)
	t.Cleanup(func() { cleanupD2(t, db) })
	ctx := context.Background()
	now := time.Date(2026, 6, 30, 4, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	base, _ := NewService(repository, func() time.Time { return now })
	dispatch, _ := NewDispatchService(
		repository, base, activeEligibility{},
		FeePolicy{DeliveryFeeCents: 150, RiderFeeCents: 100, PlatformCutCents: 50},
		func() time.Time { return now },
	)
	order, err := dispatch.CreateOrder(ctx, CreateDispatchInput{
		MerchantUserID: dispatchMerchant, MerchantRequestID: "01J00000000000000000000821",
		Pickup:    Contact{Name: "M", Phone: "1", Address: "A"},
		Recipient: Contact{Name: "R", Phone: "2", Address: "B"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, event, err := dispatch.ConfirmMockFee(ctx, dispatchMerchant, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	item, found, err := repository.ClaimNextOutbox(ctx, now, time.Minute)
	if err != nil || !found || item.Event.EventPublicID != event.EventPublicID ||
		item.Event.Status != OutboxProcessing || item.Event.AttemptCount != 1 {
		t.Fatalf("claim=%#v found=%v err=%v", item, found, err)
	}
	if _, err := repository.FailOutbox(ctx, item, "webhook_failed", "safe", now.Add(time.Second), 2, now); err != nil {
		t.Fatal(err)
	}
	item, found, err = repository.ClaimNextOutbox(ctx, now.Add(2*time.Second), time.Minute)
	if err != nil || !found || item.Event.AttemptCount != 2 {
		t.Fatalf("retry claim=%#v found=%v err=%v", item, found, err)
	}
	status, err := repository.FailOutbox(ctx, item, "webhook_failed", "safe", now.Add(time.Minute), 2, now)
	if err != nil || status != OutboxDeadletter {
		t.Fatalf("deadletter status=%s err=%v", status, err)
	}

	second, err := dispatch.CreateOrder(ctx, CreateDispatchInput{
		MerchantUserID: dispatchMerchant, MerchantRequestID: "01J00000000000000000000822",
		Pickup:    Contact{Name: "M", Phone: "1", Address: "A"},
		Recipient: Contact{Name: "R", Phone: "2", Address: "B"},
	})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(3 * time.Minute)
	_, _, err = dispatch.ConfirmMockFee(ctx, dispatchMerchant, second.DeliveryOrderPublicID, second.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	skipped, found, err := repository.ClaimNextOutbox(ctx, now, time.Minute)
	if err != nil || !found {
		t.Fatalf("skip claim found=%v err=%v", found, err)
	}
	if err := repository.CompleteOutbox(ctx, skipped, OutboxSkipped, now); err != nil {
		t.Fatal(err)
	}
	var loaded OutboxEvent
	if err := db.Where("id = ?", skipped.Event.ID).First(&loaded).Error; err != nil ||
		loaded.Status != OutboxSkipped || loaded.ProcessedAt == nil {
		t.Fatalf("skipped event=%#v err=%v", loaded, err)
	}

	third, err := dispatch.CreateOrder(ctx, CreateDispatchInput{
		MerchantUserID: dispatchMerchant, MerchantRequestID: "01J00000000000000000000823",
		Pickup:    Contact{Name: "M", Phone: "1", Address: "A"},
		Recipient: Contact{Name: "R", Phone: "2", Address: "B"},
	})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	_, _, err = dispatch.ConfirmMockFee(ctx, dispatchMerchant, third.DeliveryOrderPublicID, third.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	expiredLease, found, err := repository.ClaimNextOutbox(ctx, now, time.Minute)
	if err != nil || !found {
		t.Fatalf("initial crash lease found=%v err=%v", found, err)
	}
	recoveredLease, found, err := repository.ClaimNextOutbox(ctx, now.Add(2*time.Minute), time.Minute)
	if err != nil || !found || recoveredLease.Event.ID != expiredLease.Event.ID ||
		recoveredLease.LeaseToken == expiredLease.LeaseToken {
		t.Fatalf("recovered lease=%#v initial=%#v found=%v err=%v",
			recoveredLease, expiredLease, found, err)
	}
	if err := repository.CompleteOutbox(ctx, expiredLease, OutboxSucceeded, now.Add(2*time.Minute)); !errors.Is(err, ErrOutboxLease) {
		t.Fatalf("stale lease completion returned %v", err)
	}
	if err := repository.CompleteOutbox(ctx, recoveredLease, OutboxSucceeded, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	t.Log("dispatch_outbox=PASS lease_recovery+stale_CAS+retry+deadletter+skipped")
}

func TestZZDispatchResidueAgainstRealMySQL84(t *testing.T) {
	db := openD2MySQL(t)
	cleanupD2(t, db)
	var orders, events int64
	if err := db.Model(&Order{}).Where("merchant_user_id = ?", dispatchMerchant).Count(&orders).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&OutboxEvent{}).Count(&events).Error; err != nil {
		t.Fatal(err)
	}
	if orders != 0 || events != 0 {
		t.Fatalf("dispatch residue orders=%d events=%d", orders, events)
	}
	t.Log("dispatch_residue orders=0 outbox=0")
}

func openD2MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	db := openD1MySQL(t)
	if !db.Migrator().HasTable(&OutboxEvent{}) || !db.Migrator().HasColumn(&Order{}, "merchant_request_id") {
		t.Fatal("required migration 024 is missing")
	}
	return db
}

func normalizeD2MigrationState(t *testing.T, db *gorm.DB) {
	t.Helper()
	hasOutbox := db.Migrator().HasTable(&OutboxEvent{})
	hasColumn := db.Migrator().HasColumn(&Order{}, "merchant_request_id")
	if hasOutbox && !hasColumn {
		if err := db.Exec("DROP TABLE delivery_outbox").Error; err != nil {
			t.Fatal(err)
		}
		hasOutbox = false
	}
	if !hasOutbox && hasColumn {
		if err := db.Exec("ALTER TABLE delivery_orders DROP INDEX uq_delivery_order_merchant_request, DROP COLUMN merchant_request_id").Error; err != nil {
			t.Fatal(err)
		}
		hasColumn = false
	}
	if !hasOutbox && !hasColumn {
		applyD1SQLFile(t, db, "../../../migrations/024_delivery_dispatch.sql")
	}
}

func assertD2Schema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if !db.Migrator().HasTable(&OutboxEvent{}) || !db.Migrator().HasColumn(&Order{}, "merchant_request_id") {
		t.Fatal("D-2 schema missing table or idempotency column")
	}
	for table, index := range map[string]string{
		"delivery_orders": "uq_delivery_order_merchant_request",
		"delivery_outbox": "uq_delivery_outbox_event_public_id",
	} {
		var count int64
		if err := db.Raw(`SELECT COUNT(*) FROM information_schema.statistics
			WHERE table_schema=DATABASE() AND table_name=? AND index_name=?`, table, index).
			Scan(&count).Error; err != nil || count == 0 {
			t.Fatalf("missing index %s.%s count=%d err=%v", table, index, count, err)
		}
	}
	var aggregateUnique, claimIndex int64
	if err := db.Raw(`SELECT COUNT(DISTINCT column_name) FROM information_schema.statistics
		WHERE table_schema=DATABASE() AND table_name='delivery_outbox'
		AND index_name='uq_delivery_outbox_aggregate_event'`).Scan(&aggregateUnique).Error; err != nil || aggregateUnique != 3 {
		t.Fatalf("outbox aggregate unique columns=%d err=%v", aggregateUnique, err)
	}
	if err := db.Raw(`SELECT COUNT(DISTINCT column_name) FROM information_schema.statistics
		WHERE table_schema=DATABASE() AND table_name='delivery_outbox'
		AND index_name='idx_delivery_outbox_claim'`).Scan(&claimIndex).Error; err != nil || claimIndex != 4 {
		t.Fatalf("outbox claim index columns=%d err=%v", claimIndex, err)
	}
	var foreignKeys int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.key_column_usage
		WHERE table_schema=DATABASE() AND table_name IN ('delivery_orders','delivery_outbox')
		AND referenced_table_name IS NOT NULL`).Scan(&foreignKeys).Error; err != nil || foreignKeys != 0 {
		t.Fatalf("D-2 foreign keys=%d err=%v", foreignKeys, err)
	}
	var statusCheck string
	if err := db.Raw(`SELECT cc.check_clause FROM information_schema.table_constraints tc
		JOIN information_schema.check_constraints cc
		  ON cc.constraint_schema=tc.constraint_schema AND cc.constraint_name=tc.constraint_name
		WHERE tc.table_schema=DATABASE() AND tc.table_name='delivery_outbox'
		  AND tc.constraint_name='chk_delivery_outbox_status'`).Scan(&statusCheck).Error; err != nil {
		t.Fatal(err)
	}
	for _, status := range []OutboxStatus{
		OutboxPending, OutboxProcessing, OutboxRetryWait,
		OutboxSucceeded, OutboxSkipped, OutboxDeadletter,
	} {
		if !stringsContains(statusCheck, string(status)) {
			t.Errorf("outbox CHECK missing %s: %s", status, statusCheck)
		}
	}
}

func cleanupD2(t *testing.T, db *gorm.DB) {
	t.Helper()
	var orderIDs []string
	if db.Migrator().HasTable(&Order{}) && db.Migrator().HasColumn(&Order{}, "merchant_request_id") {
		if err := db.Model(&Order{}).Where("merchant_user_id = ?", dispatchMerchant).
			Pluck("delivery_order_public_id", &orderIDs).Error; err != nil {
			t.Errorf("list D-2 orders: %v", err)
		}
	}
	cleanupD6RiderMoney(t, db, []uint64{dispatchRiderOne, dispatchRiderTwo}, orderIDs)
	if db.Migrator().HasTable(&OutboxEvent{}) {
		if len(orderIDs) > 0 {
			if err := db.Where("aggregate_public_id IN ?", orderIDs).Delete(&OutboxEvent{}).Error; err != nil {
				t.Errorf("cleanup D-2 outbox: %v", err)
			}
		}
	}
	if db.Migrator().HasTable(&Order{}) {
		if err := db.Where("merchant_user_id = ?", dispatchMerchant).Delete(&Order{}).Error; err != nil {
			t.Errorf("cleanup D-2 orders: %v", err)
		}
	}
}

func jsonUnmarshal(raw []byte, target any) error {
	return json.Unmarshal(raw, target)
}

func containsJSON(raw []byte, value string) bool {
	return strings.Contains(string(raw), value)
}

func stringsContains(value, part string) bool { return strings.Contains(value, part) }
