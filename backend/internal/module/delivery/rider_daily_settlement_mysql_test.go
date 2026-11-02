package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
)

func riderDailySchema(t *testing.T) *gorm.DB {
	t.Helper()
	db := openIsolatedRiderMoneyMySQL(t)
	for _, name := range []string{
		"023_delivery_order.sql", "024_delivery_dispatch.sql", "025_delivery_payment_wallet.sql",
		"028_delivery_fee_and_rider_wallet.sql", "045_delivery_order_source.sql",
		"048_delivery_payment_recovery.sql", "049_delivery_funding_sources.sql",
		"050_delivery_rider_payout_queue.sql", "051_delivery_rider_withdrawals.sql",
		"052_delivery_recharge_cancellation.sql", "054_delivery_rider_daily_settlement.sql",
	} {
		applyD1SQLFile(t, db, "../../../migrations/"+name)
	}
	return db
}

func seedPendingRiderEarning(t *testing.T, db *gorm.DB, riderID, amount uint64, now time.Time) RiderEarningRecord {
	t.Helper()
	wallet := RiderWallet{WalletPublicID: mustRiderMoneyULID(t), RiderUserID: riderID, AvailableBalanceCents: 0, RowVersion: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&wallet).Error; err != nil {
		t.Fatal(err)
	}
	orderID := mustRiderMoneyULID(t)
	order := Order{DeliveryOrderPublicID: orderID, MerchantUserID: 995211, RiderUserID: &riderID, SourcePlatform: SourceLocal,
		DeliveryFeeCents: amount + 50, RiderFeeCents: amount, PlatformCutCents: 50, Status: StatusDelivered, RowVersion: 1, CreatedAt: now, UpdatedAt: now, DeliveredAt: &now}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	paymentID := mustRiderMoneyULID(t)
	txn := "isolated-settlement-" + paymentID
	attempt := PaymentAttempt{PaymentPublicID: paymentID, MerchantUserID: order.MerchantUserID, Purpose: PaymentOrderFee, DeliveryOrderPublicID: &orderID,
		Provider: PaymentWechat, Status: PaymentPaid, ProviderStatus: "SUCCESS", AmountCents: order.DeliveryFeeCents, Currency: "CNY", ClientRequestID: mustRiderMoneyULID(t), OutTradeNo: paymentID,
		WechatTransactionID: &txn, PaidAt: &now, RowVersion: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	settlement := FeeSettlement{SettlementPublicID: mustRiderMoneyULID(t), DeliveryOrderPublicID: orderID, MerchantUserID: order.MerchantUserID, Source: SettlementWechat,
		AmountCents: order.DeliveryFeeCents, PaymentPublicID: &paymentID, Status: SettlementPaid, PaidAt: now, RowVersion: 1}
	if err := db.Create(&settlement).Error; err != nil {
		t.Fatal(err)
	}
	earning := RiderEarningRecord{EarningPublicID: mustRiderMoneyULID(t), DeliveryOrderPublicID: orderID, RiderUserID: riderID,
		DeliveryFeeCents: order.DeliveryFeeCents, PlatformCutCents: 50, RiderFeeCents: amount, SettlementStatus: RiderEarningPending,
		SettleAt: now.Add(-time.Minute), RowVersion: 1, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	if err := db.Create(&earning).Error; err != nil {
		t.Fatal(err)
	}
	return earning
}

func TestRiderDailySettlementIdempotentAgainstIsolatedMySQL(t *testing.T) {
	db := riderDailySchema(t)
	now := time.Date(2026, 11, 2, 16, 0, 1, 0, time.UTC)
	earning := seedPendingRiderEarning(t, db, 995211, 180, now)
	repository := NewRepository(db)
	first, ledger, err := repository.SettleRiderEarning(context.Background(), earning.EarningPublicID, now)
	if err != nil || first.SettlementStatus != RiderEarningSettled || ledger.AmountCents != 180 {
		t.Fatalf("first settlement=%#v ledger=%#v err=%v", first, ledger, err)
	}
	second, replayLedger, err := repository.SettleRiderEarning(context.Background(), earning.EarningPublicID, now.Add(time.Second))
	if err != nil || second.RowVersion != first.RowVersion || replayLedger.LedgerPublicID != ledger.LedgerPublicID {
		t.Fatalf("replay=%#v ledger=%#v err=%v", second, replayLedger, err)
	}
	var wallet RiderWallet
	if err := db.Where("rider_user_id = ?", earning.RiderUserID).First(&wallet).Error; err != nil {
		t.Fatal(err)
	}
	if wallet.AvailableBalanceCents != 180 {
		t.Fatalf("wallet=%d want=180", wallet.AvailableBalanceCents)
	}
	var count int64
	if err := db.Model(&RiderLedger{}).Where("idempotency_key = ?", earning.DeliveryOrderPublicID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("ledger count=%d err=%v", count, err)
	}
	t.Log("rider_daily_settlement_idempotency=PASS single_credit=true single_ledger=true")
}

func TestRiderDailySettlementRejectsUnfundedAndNotDueAgainstIsolatedMySQL(t *testing.T) {
	db := riderDailySchema(t)
	now := time.Date(2026, 11, 2, 16, 0, 1, 0, time.UTC)
	earning := seedPendingRiderEarning(t, db, 995212, 180, now)
	if err := db.Model(&RiderEarningRecord{}).Where("earning_public_id = ?", earning.EarningPublicID).Update("settle_at", now.Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	if _, _, err := repository.SettleRiderEarning(context.Background(), earning.EarningPublicID, now); !errors.Is(err, ErrRiderEarningNotDue) {
		t.Fatalf("not due err=%v", err)
	}
	if err := db.Where("delivery_order_public_id = ?", earning.DeliveryOrderPublicID).Delete(&FeeSettlement{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&RiderEarningRecord{}).Where("earning_public_id = ?", earning.EarningPublicID).Update("settle_at", now.Add(-time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.SettleRiderEarning(context.Background(), earning.EarningPublicID, now); !errors.Is(err, ErrPayoutQueueSourceBlocked) {
		t.Fatalf("unfunded err=%v", err)
	}
	t.Log("rider_daily_settlement_guards=PASS not_due=true funding_required=true")
}

func TestRiderDailyMigrationRejectsLegacyUnmatchedPendingAgainstIsolatedMySQL(t *testing.T) {
	db := openIsolatedRiderMoneyMySQL(t)
	for _, name := range []string{"023_delivery_order.sql", "024_delivery_dispatch.sql", "025_delivery_payment_wallet.sql", "028_delivery_fee_and_rider_wallet.sql", "045_delivery_order_source.sql"} {
		applyD1SQLFile(t, db, "../../../migrations/"+name)
	}
	now := time.Now().UTC()
	id := mustRiderMoneyULID(t)
	orderID := mustRiderMoneyULID(t)
	riderID := uint64(995213)
	if err := db.Exec("INSERT INTO delivery_rider_earning_records (earning_public_id, delivery_order_public_id, rider_user_id, delivery_fee_cents, platform_cut_cents, rider_fee_cents, settlement_status, row_version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", id, orderID, riderID, 100, 20, 80, RiderEarningPending, 1, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := applyD6ASQLFileExpectError(t, db, "../../../migrations/054_delivery_rider_daily_settlement.sql"); err == nil {
		t.Fatal("migration accepted unmatched legacy pending earning")
	}
	if db.Migrator().HasColumn(&RiderEarningRecord{}, "settle_at") {
		t.Fatal("failed migration changed schema")
	}
	t.Log("migration_054_legacy_guard=PASS unmatched_pending_refused=true")
}

// A real InnoDB event table keeps the notification assertion transactional.
// These isolated facts are not external payment or message-provider responses.
func riderDailyEventAppender(t *testing.T, db *gorm.DB, afterInsertError error) NotificationEventAppender {
	t.Helper()
	if err := db.Exec(`CREATE TABLE rider_daily_test_events (
		event_public_id CHAR(26) NOT NULL PRIMARY KEY,
		aggregate_public_id CHAR(26) NOT NULL,
		aggregate_version BIGINT UNSIGNED NOT NULL,
		payload JSON NOT NULL,
		UNIQUE INDEX uq_daily_event_fact (aggregate_public_id, aggregate_version)
	) ENGINE=InnoDB`).Error; err != nil {
		t.Fatal(err)
	}
	return func(tx *gorm.DB, event NotificationEvent) error {
		var fact RiderIncomeNotificationPayload
		if event.EventType != EventRiderIncomeRecorded || json.Unmarshal(event.Payload, &fact) != nil ||
			fact.SettlementStatus != string(RiderEarningSettled) || fact.EarningPublicID != event.AggregatePublicID {
			return errors.New("unexpected settlement notification fact")
		}
		if err := tx.Exec("INSERT INTO rider_daily_test_events (event_public_id, aggregate_public_id, aggregate_version, payload) VALUES (?, ?, ?, ?)",
			event.EventPublicID, event.AggregatePublicID, event.AggregateVersion, string(event.Payload)).Error; err != nil {
			return err
		}
		return afterInsertError
	}
}

func assertRiderDailyState(t *testing.T, db *gorm.DB, earning RiderEarningRecord, status RiderEarningStatus, balance uint64, ledgerCount int64) RiderEarningRecord {
	t.Helper()
	var loaded RiderEarningRecord
	if err := db.Where("earning_public_id = ?", earning.EarningPublicID).First(&loaded).Error; err != nil {
		t.Fatal(err)
	}
	if loaded.SettlementStatus != status {
		t.Fatalf("earning status=%s want=%s", loaded.SettlementStatus, status)
	}
	assertRiderMoneyBalance(t, db, earning.RiderUserID, balance)
	var count int64
	if err := db.Model(&RiderLedger{}).Where("reference_public_id = ? AND entry_type = ?", earning.DeliveryOrderPublicID, RiderLedgerIncome).Count(&count).Error; err != nil || count != ledgerCount {
		t.Fatalf("income ledger count=%d want=%d err=%v", count, ledgerCount, err)
	}
	return loaded
}

func TestRiderDailyEightConcurrentWorkersCreditAndNotifyOnceAgainstIsolatedMySQL(t *testing.T) {
	db := riderDailySchema(t)
	now := time.Date(2026, 11, 2, 16, 0, 1, 0, time.UTC)
	earning := seedPendingRiderEarning(t, db, 995221, 180, now)
	repository := NewRepository(db, WithNotificationEvents(riderDailyEventAppender(t, db, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	type result struct {
		earning RiderEarningRecord
		ledger  RiderLedger
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, 8)
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			value, ledger, err := repository.SettleRiderEarning(ctx, earning.EarningPublicID, now)
			results <- result{value, ledger, err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	var ledgerID string
	for got := range results {
		if got.err != nil || got.earning.SettlementStatus != RiderEarningSettled || got.earning.RowVersion != 2 || got.ledger.AmountCents != 180 {
			t.Fatalf("concurrent settlement result=%+v", got)
		}
		if ledgerID != "" && ledgerID != got.ledger.LedgerPublicID {
			t.Fatal("concurrent replay returned distinct income ledgers")
		}
		ledgerID = got.ledger.LedgerPublicID
	}
	loaded := assertRiderDailyState(t, db, earning, RiderEarningSettled, 180, 1)
	if loaded.SettledAt == nil || !loaded.SettledAt.Equal(now) || loaded.RowVersion != 2 {
		t.Fatalf("settlement metadata changed more than once: %+v", loaded)
	}
	assertRiderMoneyCount(t, db, "rider_daily_test_events", 1)
	var event struct {
		AggregatePublicID string
		AggregateVersion  uint64
		Payload           string
	}
	if err := db.Table("rider_daily_test_events").First(&event).Error; err != nil {
		t.Fatal(err)
	}
	var fact RiderIncomeNotificationPayload
	if json.Unmarshal([]byte(event.Payload), &fact) != nil || event.AggregatePublicID != earning.EarningPublicID || event.AggregateVersion != 2 ||
		fact.RiderUserID != earning.RiderUserID || fact.AmountCents != 180 || fact.SettlementStatus != string(RiderEarningSettled) {
		t.Fatalf("persisted notification disagrees with money: %+v fact=%+v", event, fact)
	}
	t.Log("daily_8_workers=PASS balance_180=true one_income_ledger=true one_durable_notification=true")
}

func TestRiderDailyNotificationFailureRollsBackAllMoneyAgainstIsolatedMySQL(t *testing.T) {
	db := riderDailySchema(t)
	now := time.Date(2026, 11, 2, 16, 0, 1, 0, time.UTC)
	earning := seedPendingRiderEarning(t, db, 995222, 180, now)
	wantErr := errors.New("isolated notification append failure")
	repository := NewRepository(db, WithNotificationEvents(riderDailyEventAppender(t, db, wantErr)))
	if _, _, err := repository.SettleRiderEarning(context.Background(), earning.EarningPublicID, now); !errors.Is(err, wantErr) {
		t.Fatalf("notification failure=%v want=%v", err, wantErr)
	}
	loaded := assertRiderDailyState(t, db, earning, RiderEarningPending, 0, 0)
	if loaded.RowVersion != earning.RowVersion || loaded.SettledAt != nil || !loaded.SettleAt.Equal(earning.SettleAt) {
		t.Fatalf("earning partially committed after notification failure: %+v", loaded)
	}
	var wallet RiderWallet
	if err := db.Where("rider_user_id = ?", earning.RiderUserID).First(&wallet).Error; err != nil {
		t.Fatal(err)
	}
	if wallet.RowVersion != 1 {
		t.Fatalf("wallet CAS committed despite rollback: %+v", wallet)
	}
	assertRiderMoneyCount(t, db, "rider_daily_test_events", 0)
	t.Log("daily_notification_rollback=PASS wallet=true earning=true ledger=true notification=true")
}

func TestRiderDailyMigrationPreservesPreviouslyCreditedPendingAgainstIsolatedMySQL(t *testing.T) {
	db := openIsolatedRiderMoneyMySQL(t)
	for _, name := range []string{"023_delivery_order.sql", "024_delivery_dispatch.sql", "025_delivery_payment_wallet.sql", "028_delivery_fee_and_rider_wallet.sql", "045_delivery_order_source.sql"} {
		applyD1SQLFile(t, db, "../../../migrations/"+name)
	}
	now := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	riderID := uint64(995223)
	wallet := seedRiderMoneyWallet(t, db, riderID, 180, now)
	earningID, orderID := mustRiderMoneyULID(t), mustRiderMoneyULID(t)
	if err := db.Exec("INSERT INTO delivery_rider_earning_records (earning_public_id, delivery_order_public_id, rider_user_id, delivery_fee_cents, platform_cut_cents, rider_fee_cents, settlement_status, row_version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		earningID, orderID, riderID, 230, 50, 180, RiderEarningPending, 1, now, now).Error; err != nil {
		t.Fatal(err)
	}
	legacyLedger := RiderLedger{LedgerPublicID: mustRiderMoneyULID(t), WalletPublicID: wallet.WalletPublicID, RiderUserID: riderID,
		Direction: RiderLedgerCredit, EntryType: RiderLedgerIncome, AmountCents: 180, BalanceBeforeCents: 0, BalanceAfterCents: 180,
		ReferenceType: "delivery_order", ReferencePublicID: orderID, IdempotencyKey: orderID, CreatedAt: now}
	if err := db.Create(&legacyLedger).Error; err != nil {
		t.Fatal(err)
	}
	applyD1SQLFile(t, db, "../../../migrations/054_delivery_rider_daily_settlement.sql")
	repository := NewRepository(db, WithNotificationEvents(riderDailyEventAppender(t, db, nil)))
	loaded := assertRiderDailyState(t, db, RiderEarningRecord{EarningPublicID: earningID, DeliveryOrderPublicID: orderID, RiderUserID: riderID}, RiderEarningSettled, 180, 1)
	if loaded.RowVersion != 2 || loaded.SettledAt == nil || !loaded.SettledAt.Equal(now) || !loaded.SettleAt.Equal(now) {
		t.Fatalf("migration metadata=%+v", loaded)
	}
	count, err := NewRiderDailySettlementService(repository, func() time.Time { return now.Add(48 * time.Hour) }).ReconcileBatch(context.Background())
	if err != nil || count != 0 {
		t.Fatalf("worker reselected legacy income: count=%d err=%v", count, err)
	}
	replayed, ledger, err := repository.SettleRiderEarning(context.Background(), earningID, now.Add(48*time.Hour))
	if err != nil || ledger.LedgerPublicID != legacyLedger.LedgerPublicID || replayed.RowVersion != 2 {
		t.Fatalf("legacy direct replay=%+v ledger=%+v err=%v", replayed, ledger, err)
	}
	assertRiderDailyState(t, db, loaded, RiderEarningSettled, 180, 1)
	assertRiderMoneyCount(t, db, "rider_daily_test_events", 0)
	var afterWallet RiderWallet
	if err := db.Where("rider_user_id = ?", riderID).First(&afterWallet).Error; err != nil {
		t.Fatal(err)
	}
	if afterWallet.RowVersion != wallet.RowVersion {
		t.Fatal("legacy migration/replay changed wallet row_version")
	}
	t.Log("migration_054_credited_pending=PASS preserved_180=true worker_credit_zero=true direct_replay_same_ledger=true")
}

func TestRiderDailyMockAndRefundSourcesCannotCreditAgainstIsolatedMySQL(t *testing.T) {
	db := riderDailySchema(t)
	now := time.Date(2026, 11, 2, 16, 0, 1, 0, time.UTC)
	repository := NewRepository(db)
	for index, kind := range []string{"mock", "refund_pending", "refund_retry", "refund_succeeded", "refunded_settlement"} {
		t.Run(kind, func(t *testing.T) {
			earning := seedPendingRiderEarning(t, db, uint64(995230+index), 180, now)
			var settlement FeeSettlement
			if err := db.Where("delivery_order_public_id = ?", earning.DeliveryOrderPublicID).First(&settlement).Error; err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "mock":
				if err := db.Model(&FeeSettlement{}).Where("id = ?", settlement.ID).Updates(map[string]any{"source": SettlementMock, "payment_public_id": nil}).Error; err != nil {
					t.Fatal(err)
				}
			case "refunded_settlement":
				if err := db.Model(&FeeSettlement{}).Where("id = ?", settlement.ID).Updates(map[string]any{"status": SettlementRefunded, "refunded_at": now}).Error; err != nil {
					t.Fatal(err)
				}
			default:
				status := map[string]RefundStatus{"refund_pending": RefundPending, "refund_retry": RefundRetryPending, "refund_succeeded": RefundSucceeded}[kind]
				refundID := mustRiderMoneyULID(t)
				refund := Refund{RefundPublicID: refundID, SettlementPublicID: settlement.SettlementPublicID, DeliveryOrderPublicID: earning.DeliveryOrderPublicID,
					MerchantUserID: settlement.MerchantUserID, Source: SettlementWechat, AmountCents: settlement.AmountCents, Status: status,
					IdempotencyKey: mustRiderMoneyULID(t), OutRefundNo: refundID, RowVersion: 1, CreatedAt: now, UpdatedAt: now}
				if err := db.Create(&refund).Error; err != nil {
					t.Fatal(err)
				}
			}
			if _, _, err := repository.SettleRiderEarning(context.Background(), earning.EarningPublicID, now); !isRiderPayoutSourceBlockError(err) {
				t.Fatalf("%s source allowed settlement or unexpected error=%v", kind, err)
			}
			loaded := assertRiderDailyState(t, db, earning, RiderEarningPending, 0, 0)
			if loaded.RowVersion != 1 || loaded.SettledAt != nil {
				t.Fatalf("blocked source updated earning: %+v", loaded)
			}
		})
	}
	t.Log("daily_unverified_sources=PASS mock=true refund_pending=true refund_retry=true refund_succeeded=true refunded_settlement=true")
}

func TestRiderDailyDeliveredRecordsPendingWithoutCreditingAgainstIsolatedMySQL(t *testing.T) {
	db := riderDailySchema(t)
	now := time.Date(2026, 11, 2, 15, 59, 59, 0, time.UTC)
	riderID := uint64(995241)
	order := Order{DeliveryOrderPublicID: mustRiderMoneyULID(t), MerchantUserID: 995211, RiderUserID: &riderID, SourcePlatform: SourceLocal,
		DeliveryFeeCents: 230, RiderFeeCents: 180, PlatformCutCents: 50, Status: StatusDelivering, RowVersion: 7, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute)}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	delivered, err := repository.Transition(context.Background(), order.DeliveryOrderPublicID, order.RowVersion, StatusDelivering, StatusDelivered, &riderID, now)
	if err != nil || delivered.Status != StatusDelivered {
		t.Fatalf("deliver transition=%+v err=%v", delivered, err)
	}
	earning, err := repository.FindRiderEarningByOrder(context.Background(), order.DeliveryOrderPublicID)
	if err != nil {
		t.Fatal(err)
	}
	assertRiderDailyState(t, db, earning, RiderEarningPending, 0, 0)
	wantSettlement := time.Date(2026, 11, 2, 16, 0, 0, 0, time.UTC)
	if !earning.SettleAt.Equal(wantSettlement) || earning.SettledAt != nil {
		t.Fatalf("pending schedule=%+v", earning)
	}
	if _, err := repository.Transition(context.Background(), order.DeliveryOrderPublicID, order.RowVersion, StatusDelivering, StatusDelivered, &riderID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertRiderMoneyCount(t, db, "delivery_rider_earning_records", 1)
	snapshot, err := repository.ReadRiderWalletSnapshot(context.Background(), riderID)
	if err != nil || snapshot.WalletState != RiderWalletReady || snapshot.AvailableBalanceCents != 0 || snapshot.PendingSettlementCents != 180 ||
		snapshot.UnverifiedCents != 0 || snapshot.ReservedCents != 0 || snapshot.NextSettlementAt == nil || !snapshot.NextSettlementAt.Equal(wantSettlement) {
		t.Fatalf("post-delivery wallet snapshot=%+v err=%v", snapshot, err)
	}
	if _, err := repository.MarkRiderEarningPaidOffline(context.Background(), earning.EarningPublicID, earning.RowVersion, 995901, mustRiderMoneyULID(t), "isolated pending guard", now); !errors.Is(err, ErrEarningSettled) {
		t.Fatalf("pending income allowed offline payout: %v", err)
	}
	t.Log("daily_delivered_pending=PASS no_credit=true no_income_ledger=true snapshot_available_zero=true expected_income_180=true replay_once=true")
}

func TestRiderDailyDeliveryReplayUsesOriginalDeliveryDayAgainstIsolatedMySQL(t *testing.T) {
	db := riderDailySchema(t)
	deliveredAt := time.Date(2026, 11, 2, 15, 59, 59, 0, time.UTC)
	retryAt := deliveredAt.Add(48 * time.Hour)
	riderID := uint64(995242)
	order := Order{DeliveryOrderPublicID: mustRiderMoneyULID(t), MerchantUserID: 995211, RiderUserID: &riderID, SourcePlatform: SourceLocal,
		DeliveryFeeCents: 230, RiderFeeCents: 180, PlatformCutCents: 50, Status: StatusDelivered, DeliveredAt: &deliveredAt,
		RowVersion: 8, CreatedAt: deliveredAt.Add(-time.Hour), UpdatedAt: deliveredAt}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	if _, err := repository.Transition(context.Background(), order.DeliveryOrderPublicID, 7, StatusDelivering, StatusDelivered, &riderID, retryAt); err != nil {
		t.Fatal(err)
	}
	earning, err := repository.FindRiderEarningByOrder(context.Background(), order.DeliveryOrderPublicID)
	if err != nil {
		t.Fatal(err)
	}
	if !earning.CreatedAt.Equal(deliveredAt) || !earning.SettleAt.Equal(time.Date(2026, 11, 2, 16, 0, 0, 0, time.UTC)) {
		t.Fatalf("late delivery replay changed business day: %+v", earning)
	}
	assertRiderDailyState(t, db, earning, RiderEarningPending, 0, 0)
	t.Log("daily_late_delivery_replay=PASS original_income_day=true original_settlement_day=true no_immediate_credit=true")
}
