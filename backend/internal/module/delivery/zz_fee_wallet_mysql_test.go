package delivery

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

const (
	d6aMerchantGlobal   = uint64(9_006_001_001)
	d6aMerchantOverride = uint64(9_006_001_002)
	d6aMerchantProof    = uint64(9_006_001_003)
	d6aRiderNormal      = uint64(9_006_002_001)
	d6aRiderProof       = uint64(9_006_002_002)
	d6aOperator         = uint64(9_006_003_001)
)

func TestD6A00FeeWalletMigrationRollbackApplyAgainstRealMySQL84(t *testing.T) {
	db := openD3MySQL(t)
	assertD1MySQL84(t, db)
	cleanupD6A(t, db)
	if hasD6ATables(db) {
		assertD6ARiderMoneyEmpty(t, db)
		applyD1SQLFile(t, db, "../../../migrations/rollback/028_delivery_fee_and_rider_wallet.sql")
		if db.Migrator().HasTable(&RiderWallet{}) {
			t.Fatal("rollback 028 left delivery_rider_wallets behind")
		}
	}
	applyD1SQLFile(t, db, "../../../migrations/028_delivery_fee_and_rider_wallet.sql")
	assertD6ASchema(t, db)
	t.Log("migration_028_apply_rollback_apply=PASS fee_config+rider_wallet+earning+ledger foreign_keys=0 rollback_guard=PASS")
}

func TestD6AFeePolicyGlobalOverrideAndSnapshotAgainstRealMySQL84(t *testing.T) {
	db := openD6AMySQL(t)
	cleanupD6A(t, db)
	t.Cleanup(func() { cleanupD6A(t, db) })
	ctx := context.Background()
	now := time.Date(2026, 7, 2, 1, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	resetD6AGlobalConfig(t, db, 180, 60, now)
	dispatch := newD6ADispatch(t, repository, func() time.Time { return now })

	globalOrder := createD6AOrder(t, ctx, dispatch, d6aMerchantGlobal, "01J0000000000000000000A001")
	if globalOrder.DeliveryFeeCents != 180 || globalOrder.PlatformCutCents != 60 || globalOrder.RiderFeeCents != 120 {
		t.Fatalf("global fee snapshot=%#v", globalOrder)
	}

	_, err := repository.UpsertMerchantFeeOverride(ctx, d6aMerchantOverride, 250, 80, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	overrideOrder := createD6AOrder(t, ctx, dispatch, d6aMerchantOverride, "01J0000000000000000000A002")
	if overrideOrder.DeliveryFeeCents != 250 || overrideOrder.PlatformCutCents != 80 || overrideOrder.RiderFeeCents != 170 {
		t.Fatalf("override fee snapshot=%#v", overrideOrder)
	}

	config, err := repository.GlobalFeeConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.UpdateGlobalFeeConfig(ctx, 210, 70, config.RowVersion, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	loadedOld, err := repository.Find(ctx, globalOrder.DeliveryOrderPublicID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedOld.DeliveryFeeCents != 180 || loadedOld.PlatformCutCents != 60 || loadedOld.RiderFeeCents != 120 {
		t.Fatalf("old order snapshot drifted=%#v", loadedOld)
	}
	newGlobal := createD6AOrder(t, ctx, dispatch, d6aMerchantGlobal, "01J0000000000000000000A003")
	if newGlobal.DeliveryFeeCents != 210 || newGlobal.PlatformCutCents != 70 || newGlobal.RiderFeeCents != 140 {
		t.Fatalf("new global fee snapshot=%#v", newGlobal)
	}
	t.Log("d6a_fee_policy=PASS global+merchant_override+snapshot_immutability")
}

func TestD6ARiderEarningSettlementNormalProofIdempotencyAndPayoutAgainstRealMySQL84(t *testing.T) {
	db := openD6AMySQL(t)
	if !db.Migrator().HasTable(&Proof{}) {
		t.Fatal("required D-4 proof migration 026 is missing")
	}
	cleanupD6A(t, db)
	t.Cleanup(func() { cleanupD6A(t, db) })
	ctx := context.Background()
	now := time.Date(2026, 7, 2, 2, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	resetD6AGlobalConfig(t, db, 180, 60, now)
	dispatch := newD6ADispatch(t, repository, func() time.Time { return now })

	normal := createD6AOrder(t, ctx, dispatch, d6aMerchantGlobal, "01J0000000000000000000A011")
	normal = readyAndDeliverD6AOrder(t, ctx, dispatch, normal, d6aRiderNormal, &now)
	assertD6AEarningAndWallet(t, db, normal, d6aRiderNormal, 120)
	repeated, err := dispatch.MarkDelivered(ctx, d6aRiderNormal, normal.DeliveryOrderPublicID, normal.RowVersion-1)
	if err != nil || repeated.RowVersion != normal.RowVersion {
		t.Fatalf("idempotent delivered=%#v err=%v", repeated, err)
	}
	assertD6AEarningAndWallet(t, db, normal, d6aRiderNormal, 120)

	proofOrder := createD6AOrder(t, ctx, dispatch, d6aMerchantProof, "01J0000000000000000000A012")
	proofOrder = readyClaimAndStartProofDelivery(t, ctx, dispatch, proofOrder, d6aRiderProof, &now)
	proofService, err := NewProofService(repository, &proofMediaStub{}, ProofServiceConfig{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	proofOrder, _, err = proofService.AdvanceWithProof(ctx, ProofTransitionInput{
		OrderPublicID:      proofOrder.DeliveryOrderPublicID,
		RiderUserID:        d6aRiderProof,
		ExpectedRowVersion: proofOrder.RowVersion,
		ProofType:          ProofDelivery,
		AssetPublicID:      "01J0000000000000000000A101",
	})
	if err != nil || proofOrder.Status != StatusDelivered {
		t.Fatalf("proof delivered order=%#v err=%v", proofOrder, err)
	}
	assertD6AEarningAndWallet(t, db, proofOrder, d6aRiderProof, 120)

	earning, err := repository.FindRiderEarningByOrder(ctx, normal.DeliveryOrderPublicID)
	if err != nil {
		t.Fatal(err)
	}
	payout, err := repository.MarkRiderEarningPaidOffline(
		ctx, earning.EarningPublicID, earning.RowVersion, d6aOperator,
		"01J0000000000000000000A021", "wechat paid offline", now.Add(time.Minute),
	)
	if err != nil || payout.Earning.SettlementStatus != RiderEarningPaidOffline ||
		payout.Wallet.AvailableBalanceCents != 0 || payout.Ledger.Direction != RiderLedgerDebit {
		t.Fatalf("payout=%#v err=%v", payout, err)
	}
	repeatedPayout, err := repository.MarkRiderEarningPaidOffline(
		ctx, earning.EarningPublicID, earning.RowVersion, d6aOperator,
		"01J0000000000000000000A022", "repeated", now.Add(2*time.Minute),
	)
	if err != nil || repeatedPayout.Payout.PayoutPublicID != payout.Payout.PayoutPublicID {
		t.Fatalf("idempotent payout=%#v err=%v", repeatedPayout, err)
	}
	t.Log("d6a_rider_settlement=PASS normal_delivered+proof_delivered+idempotent+wallet_CAS+ledger+payout")
}

func openD6AMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	db := openD3MySQL(t)
	if !hasD6ATables(db) {
		applyD1SQLFile(t, db, "../../../migrations/028_delivery_fee_and_rider_wallet.sql")
	}
	assertD6ASchema(t, db)
	return db
}

func hasD6ATables(db *gorm.DB) bool {
	return db.Migrator().HasTable(&DeliveryFeeConfig{}) &&
		db.Migrator().HasTable(&RiderWallet{}) &&
		db.Migrator().HasTable(&RiderEarningRecord{})
}

func newD6ADispatch(t *testing.T, repository *GormRepository, now func() time.Time) *DispatchService {
	t.Helper()
	base, err := NewService(repository, now)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := NewDispatchServiceWithFeeProvider(repository, base, activeEligibility{}, repository, now)
	if err != nil {
		t.Fatal(err)
	}
	return dispatch
}

func createD6AOrder(t *testing.T, ctx context.Context, dispatch *DispatchService, merchant uint64, requestID string) Order {
	t.Helper()
	order, err := dispatch.CreateOrder(ctx, CreateDispatchInput{
		MerchantUserID: merchant, MerchantRequestID: requestID,
		Pickup:    Contact{Name: "D6A merchant", Phone: "13800006001", Address: "Pickup"},
		Recipient: Contact{Name: "D6A recipient", Phone: "13800006002", Address: "Delivery"},
		Remark:    "D-6a acceptance",
	})
	if err != nil {
		t.Fatal(err)
	}
	return order
}

func readyAndDeliverD6AOrder(
	t *testing.T,
	ctx context.Context,
	dispatch *DispatchService,
	order Order,
	riderID uint64,
	now *time.Time,
) Order {
	t.Helper()
	var err error
	*now = now.Add(time.Minute)
	order, _, err = dispatch.ConfirmMockFee(ctx, order.MerchantUserID, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Minute)
	order, err = dispatch.ClaimOrder(ctx, riderID, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Minute)
	order, err = dispatch.StartToPickup(ctx, riderID, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Minute)
	order, err = dispatch.MarkPickedUp(ctx, riderID, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Minute)
	order, err = dispatch.StartDelivering(ctx, riderID, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Minute)
	order, err = dispatch.MarkDelivered(ctx, riderID, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil || order.Status != StatusDelivered {
		t.Fatalf("delivered=%#v err=%v", order, err)
	}
	return order
}

func readyClaimAndStartProofDelivery(
	t *testing.T,
	ctx context.Context,
	dispatch *DispatchService,
	order Order,
	riderID uint64,
	now *time.Time,
) Order {
	t.Helper()
	var err error
	*now = now.Add(time.Minute)
	order, _, err = dispatch.ConfirmMockFee(ctx, order.MerchantUserID, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Minute)
	order, err = dispatch.ClaimOrder(ctx, riderID, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Minute)
	order, err = dispatch.StartToPickup(ctx, riderID, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	proofService, err := NewProofService(dispatch.repository.(*GormRepository), &proofMediaStub{}, ProofServiceConfig{Now: func() time.Time { return *now }})
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Minute)
	order, _, err = proofService.AdvanceWithProof(ctx, ProofTransitionInput{
		OrderPublicID:      order.DeliveryOrderPublicID,
		RiderUserID:        riderID,
		ExpectedRowVersion: order.RowVersion,
		ProofType:          ProofPickup,
		AssetPublicID:      "01J0000000000000000000A100",
	})
	if err != nil || order.Status != StatusPickedUp {
		t.Fatalf("proof pickup=%#v err=%v", order, err)
	}
	*now = now.Add(time.Minute)
	order, err = dispatch.StartDelivering(ctx, riderID, order.DeliveryOrderPublicID, order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	return order
}

func assertD6AEarningAndWallet(t *testing.T, db *gorm.DB, order Order, riderID uint64, expectedBalance uint64) {
	t.Helper()
	var earningCount int64
	if err := db.Model(&RiderEarningRecord{}).
		Where("delivery_order_public_id = ?", order.DeliveryOrderPublicID).
		Count(&earningCount).Error; err != nil || earningCount != 1 {
		t.Fatalf("earning count=%d err=%v", earningCount, err)
	}
	var earning RiderEarningRecord
	if err := db.Where("delivery_order_public_id = ?", order.DeliveryOrderPublicID).First(&earning).Error; err != nil {
		t.Fatal(err)
	}
	if earning.RiderUserID != riderID || earning.RiderFeeCents != order.RiderFeeCents ||
		earning.PlatformCutCents != order.PlatformCutCents ||
		earning.DeliveryFeeCents != order.DeliveryFeeCents ||
		earning.SettlementStatus != RiderEarningPending {
		t.Fatalf("earning snapshot=%#v order=%#v", earning, order)
	}
	var wallet RiderWallet
	if err := db.Where("rider_user_id = ?", riderID).First(&wallet).Error; err != nil {
		t.Fatal(err)
	}
	if wallet.AvailableBalanceCents != expectedBalance {
		t.Fatalf("wallet balance=%d want=%d wallet=%#v", wallet.AvailableBalanceCents, expectedBalance, wallet)
	}
	var ledgerCount int64
	if err := db.Model(&RiderLedger{}).
		Where("rider_user_id = ? AND reference_public_id = ? AND entry_type = ?",
			riderID, order.DeliveryOrderPublicID, RiderLedgerIncome).
		Count(&ledgerCount).Error; err != nil || ledgerCount != 1 {
		t.Fatalf("ledger count=%d err=%v", ledgerCount, err)
	}
}

func resetD6AGlobalConfig(t *testing.T, db *gorm.DB, deliveryFee, platformCut uint64, now time.Time) {
	t.Helper()
	if err := db.Exec(`
		INSERT INTO delivery_fee_configs
		  (config_scope, delivery_fee_cents, platform_cut_cents, status, row_version, created_at, updated_at)
		VALUES ('global', ?, ?, 'active', 1, ?, ?)
		ON DUPLICATE KEY UPDATE
		  delivery_fee_cents=VALUES(delivery_fee_cents),
		  platform_cut_cents=VALUES(platform_cut_cents),
		  status='active',
		  row_version=row_version+1,
		  updated_at=VALUES(updated_at)
	`, deliveryFee, platformCut, now, now).Error; err != nil {
		t.Fatal(err)
	}
}

func cleanupD6A(t *testing.T, db *gorm.DB) {
	t.Helper()
	orderIDs := d6AOrderIDs(t, db)
	cleanupD6RiderMoney(t, db, []uint64{d6aRiderNormal, d6aRiderProof}, orderIDs)
	if db.Migrator().HasTable(&Proof{}) && len(orderIDs) > 0 {
		if err := db.Where("delivery_order_public_id IN ?", orderIDs).Delete(&Proof{}).Error; err != nil {
			t.Errorf("cleanup D-6a proofs: %v", err)
		}
	}
	if db.Migrator().HasTable(&OutboxEvent{}) && len(orderIDs) > 0 {
		if err := db.Where("aggregate_public_id IN ?", orderIDs).Delete(&OutboxEvent{}).Error; err != nil {
			t.Errorf("cleanup D-6a outbox: %v", err)
		}
	}
	if db.Migrator().HasTable(&Order{}) {
		if err := db.Where("merchant_user_id IN ?", []uint64{d6aMerchantGlobal, d6aMerchantOverride, d6aMerchantProof}).Delete(&Order{}).Error; err != nil {
			t.Errorf("cleanup D-6a orders: %v", err)
		}
	}
	if db.Migrator().HasTable(&MerchantFeeOverride{}) {
		if err := db.Where("merchant_user_id IN ?", []uint64{d6aMerchantGlobal, d6aMerchantOverride, d6aMerchantProof}).Delete(&MerchantFeeOverride{}).Error; err != nil {
			t.Errorf("cleanup D-6a fee overrides: %v", err)
		}
	}
}

func d6AOrderIDs(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	if !db.Migrator().HasTable(&Order{}) {
		return nil
	}
	var orderIDs []string
	if err := db.Model(&Order{}).
		Where("merchant_user_id IN ?", []uint64{d6aMerchantGlobal, d6aMerchantOverride, d6aMerchantProof}).
		Pluck("delivery_order_public_id", &orderIDs).Error; err != nil {
		t.Errorf("list D-6a orders: %v", err)
	}
	return orderIDs
}

func cleanupD6RiderMoney(t *testing.T, db *gorm.DB, riderIDs []uint64, orderIDs []string) {
	t.Helper()
	if len(riderIDs) == 0 && len(orderIDs) == 0 {
		return
	}
	if db.Migrator().HasTable(&RiderLedger{}) {
		query := db.Where("rider_user_id IN ?", riderIDs)
		if len(orderIDs) > 0 {
			query = query.Or("reference_public_id IN ?", orderIDs)
		}
		if err := query.Delete(&RiderLedger{}).Error; err != nil {
			t.Errorf("cleanup D-6a rider ledger: %v", err)
		}
	}
	if db.Migrator().HasTable(&RiderPayoutRecord{}) {
		query := db.Where("rider_user_id IN ?", riderIDs)
		if len(orderIDs) > 0 {
			query = query.Or("delivery_order_public_id IN ?", orderIDs)
		}
		if err := query.Delete(&RiderPayoutRecord{}).Error; err != nil {
			t.Errorf("cleanup D-6a rider payouts: %v", err)
		}
	}
	if db.Migrator().HasTable(&RiderEarningRecord{}) {
		query := db.Where("rider_user_id IN ?", riderIDs)
		if len(orderIDs) > 0 {
			query = query.Or("delivery_order_public_id IN ?", orderIDs)
		}
		if err := query.Delete(&RiderEarningRecord{}).Error; err != nil {
			t.Errorf("cleanup D-6a rider earnings: %v", err)
		}
	}
	if db.Migrator().HasTable(&RiderWallet{}) {
		if err := db.Where("rider_user_id IN ?", riderIDs).Delete(&RiderWallet{}).Error; err != nil {
			t.Errorf("cleanup D-6a rider wallets: %v", err)
		}
	}
}

func assertD6ARiderMoneyEmpty(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range []string{
		"delivery_rider_ledger",
		"delivery_rider_payout_records",
		"delivery_rider_earning_records",
		"delivery_rider_wallets",
	} {
		if !db.Migrator().HasTable(table) {
			continue
		}
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("refusing D-6a rollback with %s rows=%d", table, count)
		}
	}
}

func assertD6ASchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range []any{
		&DeliveryFeeConfig{}, &MerchantFeeOverride{}, &RiderWallet{},
		&RiderEarningRecord{}, &RiderPayoutRecord{}, &RiderLedger{},
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing D-6a table %T", table)
		}
	}
	for table, index := range map[string]string{
		"delivery_fee_configs":            "uq_delivery_fee_config_scope",
		"delivery_merchant_fee_overrides": "uq_delivery_merchant_fee_override",
		"delivery_rider_wallets":          "uq_delivery_rider_wallet_rider",
		"delivery_rider_earning_records":  "uq_delivery_rider_earning_order",
		"delivery_rider_payout_records":   "uq_delivery_rider_payout_earning",
		"delivery_rider_ledger":           "uq_delivery_rider_ledger_idempotency",
	} {
		var count int64
		if err := db.Raw(`SELECT COUNT(*) FROM information_schema.statistics
			WHERE table_schema=DATABASE() AND table_name=? AND index_name=?`, table, index).
			Scan(&count).Error; err != nil || count == 0 {
			t.Fatalf("missing D-6a index %s.%s count=%d err=%v", table, index, count, err)
		}
	}
	var foreignKeys int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.key_column_usage
		WHERE table_schema=DATABASE() AND table_name IN ?
		AND referenced_table_name IS NOT NULL`, []string{
		"delivery_fee_configs", "delivery_merchant_fee_overrides", "delivery_rider_wallets",
		"delivery_rider_earning_records", "delivery_rider_payout_records", "delivery_rider_ledger",
	}).Scan(&foreignKeys).Error; err != nil || foreignKeys != 0 {
		t.Fatalf("D-6a foreign keys=%d err=%v", foreignKeys, err)
	}
	var globalCount int64
	if err := db.Model(&DeliveryFeeConfig{}).Where("config_scope = ? AND status = ?", "global", FeeConfigActive).
		Count(&globalCount).Error; err != nil || globalCount != 1 {
		t.Fatalf("global fee config count=%d err=%v", globalCount, err)
	}
}

func TestD6ARollbackGuardRejectsMoneyFactsAgainstRealMySQL84(t *testing.T) {
	db := openD6AMySQL(t)
	cleanupD6A(t, db)
	t.Cleanup(func() { cleanupD6A(t, db) })
	ctx := context.Background()
	now := time.Date(2026, 7, 2, 3, 0, 0, 0, time.UTC)
	resetD6AGlobalConfig(t, db, 180, 60, now)
	repository := NewRepository(db)
	dispatch := newD6ADispatch(t, repository, func() time.Time { return now })
	order := createD6AOrder(t, ctx, dispatch, d6aMerchantGlobal, "01J0000000000000000000A031")
	_ = readyAndDeliverD6AOrder(t, ctx, dispatch, order, d6aRiderNormal, &now)
	fileErr := applyD6ASQLFileExpectError(t, db, "../../../migrations/rollback/028_delivery_fee_and_rider_wallet.sql")
	if fileErr == nil {
		t.Fatal("rollback 028 unexpectedly succeeded with rider money facts")
	}
	if !db.Migrator().HasTable(&RiderWallet{}) {
		t.Fatal("rollback guard dropped D-6a tables despite money facts")
	}
	t.Log("migration_028_rollback_guard=PASS money_facts_fail_closed")
}

func applyD6ASQLFileExpectError(t *testing.T, db *gorm.DB, relativePath string) error {
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
				return err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
