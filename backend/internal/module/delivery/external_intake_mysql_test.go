package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	drivermysql "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type intakeTestBinding struct {
	binding ExternalShopBinding
	err     error
}

func (r *intakeTestBinding) ResolveExternalBinding(_ context.Context, key string) (ExternalShopBinding, error) {
	if key != "binding-A" {
		return ExternalShopBinding{}, ErrExternalBinding
	}
	return r.binding, r.err
}

type intakeTestFees struct {
	err          error
	beforeReturn func()
}

func (r *intakeTestFees) FeePolicyForMerchant(context.Context, uint64) (FeePolicy, error) {
	if r.beforeReturn != nil {
		r.beforeReturn()
	}
	return FeePolicy{DeliveryFeeCents: 150, RiderFeeCents: 100, PlatformCutCents: 50}, r.err
}

// Test-only boundary: this is never registered with production routes.
type intakeTestAdapter struct{}

func (intakeTestAdapter) Platform() SourcePlatform { return SourceTaobao }
func (intakeTestAdapter) VerifyAndNormalize(_ context.Context, input ExternalOrderEnvelope, _ ExternalShopBinding) (ExternalOrderEvent, error) {
	var event ExternalOrderEvent
	if len(input.Headers["Test-Verified"]) != 1 || input.Headers["Test-Verified"][0] != "yes" {
		return event, ErrExternalAdapter
	}
	err := json.Unmarshal(input.Payload, &event)
	return event, err
}

func TestExternalIntakeTemporaryMySQL(t *testing.T) {
	dsn := os.Getenv("DELIVERY_INTAKE_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("isolated intake MySQL not configured")
	}
	cfg, err := drivermysql.ParseDSN(dsn)
	if err != nil || cfg.Addr != "127.0.0.1:33308" || !strings.HasPrefix(cfg.DBName, "campus_delivery_source_test_") {
		t.Fatal("requires the isolated test runner")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	binding := &intakeTestBinding{binding: ExternalShopBinding{MerchantUserID: 87101, Platform: SourceTaobao, SourceShopID: "Shop-A"}}
	fees := &intakeTestFees{}
	svc, err := NewExternalIntake(db, binding, fees, []ExternalOrderAdapter{intakeTestAdapter{}}, ExternalIntakeConfig{Now: func() time.Time { return now }, MaxAttempts: 2})
	if err != nil {
		t.Fatal(err)
	}
	eventFor := func(eventID, orderID string, kind ExternalOrderEventType) ExternalOrderEvent {
		shop := "Shop-A"
		return ExternalOrderEvent{EventID: eventID, EventType: kind, OccurredAt: now,
			Source: SourceMetadata{SourcePlatform: SourceTaobao, SourceShopID: &shop, SourceOrderID: &orderID, PickupCode: "0022"},
			Pickup: Contact{Name: "商家", Phone: "13000000001", Address: "校门"}, Recipient: Contact{Name: "同学", Phone: "13000000002", Address: "宿舍"}, Remark: "当面交付"}
	}
	receive := func(e ExternalOrderEvent) (ExternalReceipt, error) {
		payload, _ := json.Marshal(e)
		return svc.Receive(ctx, "binding-A", SourceTaobao, ExternalOrderEnvelope{Payload: payload, Headers: map[string][]string{"Test-Verified": {"yes"}}})
	}
	process := func(want string) ExternalReceipt {
		t.Helper()
		receipt, found, err := svc.ProcessNext(ctx)
		if err != nil || !found || receipt.Status != want {
			t.Fatalf("process: %+v found=%v err=%v want=%s", receipt, found, err, want)
		}
		return receipt
	}
	base := eventFor("event-1", "Order-001", ExternalOrderCreated)
	payload, _ := json.Marshal(base)
	if _, err := svc.Receive(ctx, "binding-A", SourceTaobao, ExternalOrderEnvelope{Payload: payload, Headers: map[string][]string{"Test-Verified": {"no"}}}); err == nil {
		t.Fatal("unsigned accepted")
	}
	if _, err := svc.Receive(ctx, "binding-A", SourceJD, ExternalOrderEnvelope{Payload: payload}); !errors.Is(err, ErrExternalAdapter) {
		t.Fatal("missing adapter accepted")
	}
	wrongShop := base
	otherShop := "Other"
	wrongShop.Source.SourceShopID = &otherShop
	if _, err := receive(wrongShop); err == nil {
		t.Fatal("unbound shop accepted")
	}
	bad := base
	bad.Recipient.Address = ""
	if _, err := receive(bad); err == nil {
		t.Fatal("incomplete create accepted")
	}
	var wg sync.WaitGroup
	ids := make(chan uint64, 8)
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r, e := receive(base); ids <- r.ID; failures <- e }()
	}
	wg.Wait()
	close(ids)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	first := uint64(0)
	for id := range ids {
		if first == 0 {
			first = id
		}
		if id != first {
			t.Fatal("concurrent event duplicated")
		}
	}
	conflict := base
	conflict.Remark = "different"
	if _, err := receive(conflict); !errors.Is(err, ErrExternalEventConflict) {
		t.Fatal("event fingerprint conflict missed")
	}
	// Reclaim an abandoned lease; its previous owner must not complete the job.
	old, found, err := svc.claim(ctx)
	if err != nil || !found {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	created := process("created")
	if err := svc.finish(ctx, &old, "created", ""); !errors.Is(err, ErrExternalLease) {
		t.Fatal("stale worker completed")
	}
	var order Order
	if err := db.Where("delivery_order_public_id = ?", created.OrderPublicID).First(&order).Error; err != nil {
		t.Fatal(err)
	}
	if order.Status != StatusPendingFeePayment || order.DeliveryFeeCents != 150 || order.PickupCode != "0022" {
		t.Fatalf("invalid order %+v", order)
	}
	if _, err := svc.Receipt(ctx, 87102, first); !errors.Is(err, ErrNotFound) {
		t.Fatal("receipt crossed merchants")
	}
	duplicate := base
	duplicate.EventID = "event-2"
	if _, err := receive(duplicate); err != nil {
		t.Fatal(err)
	}
	if r := process("reused"); r.OrderPublicID != created.OrderPublicID {
		t.Fatal("source duplicate created a new order")
	}
	conflict.EventID = "event-3"
	if _, err := receive(conflict); err != nil {
		t.Fatal(err)
	}
	process("needs_review")
	// Cancellation arriving before create prevents stale order creation.
	for _, e := range []ExternalOrderEvent{eventFor("cancel-1", "Order-cancel", ExternalOrderCancelled), eventFor("create-late", "Order-cancel", ExternalOrderCreated)} {
		if _, err := receive(e); err != nil {
			t.Fatal(err)
		}
		process("needs_review")
	}
	// Updates never mutate an in-flight order or pay/refund its delivery fee.
	update := base
	update.EventID = "update-1"
	update.EventType = ExternalOrderUpdated
	update.Recipient.Address = "变更地址"
	if _, err := receive(update); err != nil {
		t.Fatal(err)
	}
	process("needs_review")
	var unchanged Order
	db.First(&unchanged, order.ID)
	if unchanged.DeliveryAddress != order.DeliveryAddress || unchanged.Status != order.Status {
		t.Fatal("update mutated delivery")
	}
	if _, err := receive(eventFor("revoke-1", "Order-revoked", ExternalOrderCreated)); err != nil {
		t.Fatal(err)
	}
	binding.err = ErrExternalBinding
	process("needs_review")
	binding.err = nil
	// Database/provider errors back off, exhaust, then explicitly retry without
	// changing the persisted request ID or inserting a duplicate delivery.
	queued, err := receive(eventFor("retry-1", "Order-retry", ExternalOrderCreated))
	if err != nil {
		t.Fatal(err)
	}
	fees.err = errors.New("simulated unavailable")
	r, _, err := svc.ProcessNext(ctx)
	if err == nil || r.Status != "retry_wait" {
		t.Fatalf("retry %+v %v", r, err)
	}
	if _, found, err := svc.ProcessNext(ctx); err != nil || found {
		t.Fatal("retry ran before backoff")
	}
	now = now.Add(time.Minute)
	r, _, err = svc.ProcessNext(ctx)
	if err == nil || r.Status != "deadletter" {
		t.Fatal("failure not exhausted")
	}
	if err := svc.Retry(ctx, 87102, queued.ID); err == nil {
		t.Fatal("foreign merchant retried")
	}
	if err := svc.Retry(ctx, 87101, queued.ID); err != nil {
		t.Fatal(err)
	}
	fees.err = nil
	process("created")
	// Lease expiry between order creation and receipt completion rolls back the
	// whole transaction. Reclaiming it can safely create the order once.
	if _, err := receive(eventFor("atomic-1", "Order-atomic", ExternalOrderCreated)); err != nil {
		t.Fatal(err)
	}
	fees.beforeReturn = func() { now = now.Add(time.Minute) }
	if _, _, err := svc.ProcessNext(ctx); !errors.Is(err, ErrExternalLease) {
		t.Fatalf("expected lease rollback, got %v", err)
	}
	var atomicCount int64
	if err := db.Model(&Order{}).Where("source_order_id = ?", "Order-atomic").Count(&atomicCount).Error; err != nil || atomicCount != 0 {
		t.Fatalf("transaction leaked order: %d %v", atomicCount, err)
	}
	fees.beforeReturn = nil
	process("created")
	// Repeated process crashes eventually stop being reclaimed indefinitely.
	if _, err := receive(eventFor("crash-1", "Order-crash", ExternalOrderCreated)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, found, err := svc.claim(ctx); err != nil || !found {
			t.Fatal("claim", err)
		}
		now = now.Add(time.Minute)
	}
	if r := process("deadletter"); r.Reason != "lease_exhausted" {
		t.Fatal("crash retry bound not enforced")
	}
	// Multiple workers consuming distinct notifications of one source order.
	for i := 0; i < 6; i++ {
		e := eventFor(string(rune('a'+i)), "Order-race", ExternalOrderCreated)
		if _, err := receive(e); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, _ = svc.ProcessNext(ctx) }()
	}
	wg.Wait()
	now = now.Add(time.Minute)
	for i := 0; i < 10; i++ {
		_, found, err := svc.ProcessNext(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			break
		}
	}
	var count int64
	db.Model(&Order{}).Where("source_order_id = ?", "Order-race").Count(&count)
	if count != 1 {
		t.Fatalf("concurrent source orders=%d", count)
	}
	db.Model(&ExternalInbox{}).Where("source_order_id = ? AND status NOT IN ('created','reused')", "Order-race").Count(&count)
	if count != 0 {
		t.Fatal("concurrent events did not settle")
	}
	for _, table := range []string{"delivery_outbox", "delivery_wallet_ledger", "delivery_payment_attempts", "delivery_payment_notifications", "delivery_fee_settlements", "delivery_refunds"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing test baseline: %s", table)
		}
		if err := db.Table(table).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("unexpected financial/push side effect: %s count=%d err=%v", table, count, err)
		}
	}
	db.Model(&Order{}).Where("status <> ?", StatusPendingFeePayment).Count(&count)
	if count != 0 {
		t.Fatal("intake dispatched an unpaid order")
	}
	t.Log("PASS verified binding+signature boundary, 8-way event dedup, conflict, lease recovery, source reuse, reordered cancellation, revoked binding, retry/deadletter, 6-worker source uniqueness, no payment or outbox")
}
