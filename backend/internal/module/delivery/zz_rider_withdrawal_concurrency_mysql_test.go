package delivery

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestRiderWithdrawalMidnightSettlementRaceAgainstIsolatedMySQL(t *testing.T) {
	db := riderMoneySchema(t)
	now := time.Date(2026, 11, 5, 16, 0, 0, 0, time.UTC) // Beijing 00:00
	e := seedPendingRiderEarning(t, db, 8871, 300, now)
	repo, ctx := NewRepository(db), context.Background()
	key := mustRiderMoneyULID(t)
	if _, err := repo.CreateRiderWithdrawal(ctx, 8871, 300, key, now.Add(-time.Hour)); !errors.Is(err, ErrWalletInsufficient) {
		t.Fatalf("early withdrawal=%v", err)
	}
	var settlementErr, withdrawErr error
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, _, settlementErr = repo.SettleRiderEarning(ctx, e.EarningPublicID, now)
	}()
	go func() {
		defer wg.Done()
		<-start
		_, withdrawErr = repo.CreateRiderWithdrawal(ctx, 8871, 300, key, now)
	}()
	close(start)
	wg.Wait()
	if settlementErr != nil {
		t.Fatal(settlementErr)
	}
	if withdrawErr != nil && !errors.Is(withdrawErr, ErrWalletInsufficient) {
		t.Fatal(withdrawErr)
	}
	if _, err := repo.CreateRiderWithdrawal(ctx, 8871, 300, key, now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.SettleRiderEarning(ctx, e.EarningPublicID, now); err != nil {
		t.Fatal(err)
	}
	assertRiderMoneyBalance(t, db, 8871, 0)
	assertRiderMoneyCount(t, db, "delivery_rider_ledger", 2)
	assertRiderMoneyCount(t, db, "delivery_rider_withdrawal_allocations", 1)
}

func TestRiderWithdrawalSharedRechargeRefundRaceAgainstIsolatedMySQL(t *testing.T) {
	db := riderMoneySchema(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// Two riders have different completed orders funded by the same recharge.
	// A third, unclaimed order is refunded concurrently through the real repo.
	seedRiderMoneyWallet(t, db, 8881, 300, now)
	seedRiderMoneyWallet(t, db, 8882, 300, now)
	earnings := []RiderEarningRecord{seedRiderWithdrawableEarning(t, db, 8881, 300, now), seedRiderWithdrawableEarning(t, db, 8882, 300, now)}
	wallet := MerchantWallet{WalletPublicID: mustRiderMoneyULID(t), MerchantUserID: 995201, AvailableBalanceCents: 200, RowVersion: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&wallet).Error; err != nil {
		t.Fatal(err)
	}
	paymentID, txID := mustRiderMoneyULID(t), "isolated-shared-recharge"
	payment := PaymentAttempt{PaymentPublicID: paymentID, MerchantUserID: wallet.MerchantUserID, Purpose: PaymentWalletRecharge, Provider: PaymentWechat, Status: PaymentPaid,
		AmountCents: 1000, Currency: "CNY", ClientRequestID: mustRiderMoneyULID(t), OutTradeNo: paymentID, WechatTransactionID: &txID, ProviderStatus: "SUCCESS", RowVersion: 1, CreatedAt: now, UpdatedAt: now, PaidAt: &now}
	if err := db.Create(&payment).Error; err != nil {
		t.Fatal(err)
	}
	lot := FundingLot{FundingLotPublicID: mustRiderMoneyULID(t), WalletPublicID: wallet.WalletPublicID, MerchantUserID: wallet.MerchantUserID, Source: FundingSourceWechatRecharge,
		VerificationStatus: FundingVerificationVerified, PaymentPublicID: &paymentID, TotalAmountCents: 1000, AllocatedAmountCents: 800, Currency: "CNY", RowVersion: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&lot).Error; err != nil {
		t.Fatal(err)
	}
	for _, e := range earnings {
		var settlement FeeSettlement
		if err := db.Where("delivery_order_public_id = ?", e.DeliveryOrderPublicID).First(&settlement).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&settlement).Updates(map[string]any{"source": SettlementWallet, "payment_public_id": nil}).Error; err != nil {
			t.Fatal(err)
		}
		a, err := newFundingAllocation(settlement, lot, settlement.AmountCents, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&a).Error; err != nil {
			t.Fatal(err)
		}
	}
	order := Order{DeliveryOrderPublicID: mustRiderMoneyULID(t), MerchantUserID: wallet.MerchantUserID, SourcePlatform: SourceLocal, DeliveryFeeCents: 100, RiderFeeCents: 70, PlatformCutCents: 30, Status: StatusReadyForClaim, RowVersion: 1, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	settlement := FeeSettlement{SettlementPublicID: mustRiderMoneyULID(t), DeliveryOrderPublicID: order.DeliveryOrderPublicID, MerchantUserID: wallet.MerchantUserID, Source: SettlementWallet, AmountCents: 100, Status: SettlementPaid, PaidAt: now, RowVersion: 1}
	if err := db.Create(&settlement).Error; err != nil {
		t.Fatal(err)
	}
	a, err := newFundingAllocation(settlement, lot, 100, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	start := make(chan struct{})
	results := make(chan error, 3)
	var wg sync.WaitGroup
	withdrawals := make([]RiderWithdrawal, 2)
	for i, e := range earnings {
		i, e, key := i, e, mustRiderMoneyULID(t)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			w, err := repo.CreateRiderWithdrawal(ctx, e.RiderUserID, 300, key, now)
			withdrawals[i] = w
			results <- err
		}()
	}
	refundKey := mustRiderMoneyULID(t)
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		_, err := repo.RequestRefund(ctx, RefundRequest{MerchantUserID: wallet.MerchantUserID, OrderPublicID: order.DeliveryOrderPublicID, ExpectedRowVersion: 1, IdempotencyKey: refundKey, Now: now})
		results <- err
	}()
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal("shared source concurrency", err)
		}
	}
	// Refunding another order changes the lot's totals but not this withdrawal's
	// source evidence. Both claims must still be valid after the shared mutation.
	for _, w := range withdrawals {
		claim := withdrawalTestAction(t, w, "claim")
		if _, err := repo.ProcessRiderWithdrawal(ctx, "claim", claim, now); err != nil {
			t.Fatal("unrelated refund invalidated source", err)
		}
	}
	assertRiderMoneyBalance(t, db, 8881, 0)
	assertRiderMoneyBalance(t, db, 8882, 0)
	if err := db.Where("id = ?", lot.ID).First(&lot).Error; err != nil {
		t.Fatal(err)
	}
	if lot.RefundedAmountCents != 100 || lot.AllocatedAmountCents != 800 {
		t.Fatalf("lot not conserved: %+v", lot)
	}
}

func TestRiderWithdrawalSchemaRollbackAndAuditGuardsAgainstIsolatedMySQL(t *testing.T) {
	db := riderMoneySchema(t)
	const rollback = "../../../migrations/rollback/056_delivery_rider_withdrawal_allocations.down.sql"
	if err := executeRiderMoneySQL(db, rollback); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	if err := repo.RequireWithdrawalSchema(); !errors.Is(err, ErrWithdrawalAllocationUnavailable) {
		t.Fatal(err)
	}
	applyD1SQLFile(t, db, "../../../migrations/056_delivery_rider_withdrawal_allocations.sql")
	if err := repo.RequireWithdrawalSchema(); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	seedRiderMoneyWallet(t, db, 8891, 300, now)
	seedRiderWithdrawableEarning(t, db, 8891, 300, now)
	w, err := repo.CreateRiderWithdrawal(context.Background(), 8891, 100, mustRiderMoneyULID(t), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := executeRiderMoneySQL(db, rollback); err == nil {
		t.Fatal("rollback erased allocation facts")
	}
	if err := db.Model(&RiderWithdrawalAllocation{}).Where("withdraw_public_id = ?", w.WithdrawPublicID).Update("amount_cents", uint64(0)).Error; err == nil {
		t.Fatal("zero allocation accepted")
	}
	claim := withdrawalTestAction(t, w, "claim")
	w, err = repo.ProcessRiderWithdrawal(context.Background(), "claim", claim, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&RiderWithdrawalClaim{}).Where("withdraw_public_id = ?", w.WithdrawPublicID).Update("operator_user_id", 99999).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.CheckWithdrawalReadiness(context.Background()); !errors.Is(err, ErrWithdrawalSourceMismatch) {
		t.Fatalf("mismatched claim audit accepted: %v", err)
	}
	if err := db.Migrator().DropColumn(&RiderWithdrawalEvent{}, "payment_reference"); err != nil {
		t.Fatal(err)
	}
	if err := repo.RequireWithdrawalSchema(); !errors.Is(err, ErrWithdrawalAllocationUnavailable) {
		t.Fatal("partial migration accepted", err)
	}
}
