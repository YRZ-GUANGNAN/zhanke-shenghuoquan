package delivery

import (
	"context"
	"errors"
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
