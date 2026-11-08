package delivery

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
)

// Isolated source + income ledger fixture, never a real gateway receipt or live
// transfer. Seeded wallet totals are explicit, separate from production checks.
func seedRiderWithdrawableEarning(t *testing.T, db *gorm.DB, riderID, amount uint64, now time.Time) RiderEarningRecord {
	t.Helper()
	e := seedRiderMoneyEarning(t, db, riderID, amount, now)
	var wallet RiderWallet
	if err := db.Where("rider_user_id = ?", riderID).First(&wallet).Error; err != nil {
		t.Fatal(err)
	}
	wallet.AvailableBalanceCents = 0
	l, err := newRiderLedger(wallet, RiderLedgerCredit, RiderLedgerIncome, amount, "delivery_order", e.DeliveryOrderPublicID, e.DeliveryOrderPublicID, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&l).Error; err != nil {
		t.Fatal(err)
	}
	return e
}

func withdrawalTestAction(t *testing.T, w RiderWithdrawal, kind string) RiderWithdrawalAction {
	t.Helper()
	a := RiderWithdrawalAction{WithdrawPublicID: w.WithdrawPublicID, ExpectedRowVersion: w.RowVersion, OperatorUserID: 99801, IdempotencyKey: mustRiderMoneyULID(t)}
	if kind == "claim" {
		a.ClaimKey = mustRiderMoneyULID(t)
	}
	return a
}

func TestRiderWithdrawalSourceBoundPartialAndManualProcessingAgainstIsolatedMySQL(t *testing.T) {
	db := riderMoneySchema(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	seedRiderMoneyWallet(t, db, 8801, 1000, now)
	e1 := seedRiderWithdrawableEarning(t, db, 8801, 600, now.Add(-time.Hour))
	seedRiderWithdrawableEarning(t, db, 8801, 400, now)
	repo, ctx := NewRepository(db), context.Background()
	w, err := repo.CreateRiderWithdrawal(ctx, 8801, 750, mustRiderMoneyULID(t), now)
	if err != nil {
		t.Fatal(err)
	}
	assertRiderMoneyCount(t, db, "delivery_rider_withdrawal_allocations", 2)
	if amount, err := repo.ReadRiderWithdrawable(ctx, 8801); err != nil || amount != 250 {
		t.Fatalf("available=%d %v", amount, err)
	}
	if _, err := repo.MarkRiderEarningPaidOffline(ctx, e1.EarningPublicID, 1, 99801, mustRiderMoneyULID(t), "test", now); !errors.Is(err, ErrRiderPayoutReserved) {
		t.Fatalf("offline double payout: %v", err)
	}
	queued, err := repo.ReconcileRiderPayoutQueue(ctx, e1.EarningPublicID, now)
	if err != nil || queued.Queue.ReasonCode != "withdrawal_allocated" {
		t.Fatalf("queue double reserve: %+v %v", queued, err)
	}
	a := withdrawalTestAction(t, w, "claim")
	w, err = repo.ProcessRiderWithdrawal(ctx, "claim", a, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.RejectRiderWithdrawal(ctx, w.WithdrawPublicID, w.RowVersion, 99802, mustRiderMoneyULID(t), "reject", now); !errors.Is(err, ErrWithdrawalClaimConflict) {
		t.Fatalf("claimed rejection: %v", err)
	}
	paid := withdrawalTestAction(t, w, "paid")
	paid.ClaimKey, paid.PaymentReference = a.ClaimKey, "isolated-receipt"
	foreign := paid
	foreign.OperatorUserID++
	if _, err := repo.ProcessRiderWithdrawal(ctx, "paid", foreign, now); !errors.Is(err, ErrWithdrawalNotClaimed) {
		t.Fatalf("cross-admin: %v", err)
	}
	w, err = repo.ProcessRiderWithdrawal(ctx, "paid", paid, now)
	if err != nil || w.Status != RiderWithdrawalPaid {
		t.Fatalf("paid: %+v %v", w, err)
	}
	if _, err := repo.ProcessRiderWithdrawal(ctx, "paid", paid, now); err != nil {
		t.Fatal(err)
	}
	changed := paid
	changed.PaymentReference = "different"
	if _, err := repo.ProcessRiderWithdrawal(ctx, "paid", changed, now); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed replay: %v", err)
	}
	assertRiderMoneyBalance(t, db, 8801, 250)
	assertRiderMoneyCount(t, db, "delivery_rider_ledger", 3)
	if err := repo.CheckWithdrawalReadiness(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateRiderWithdrawal(ctx, 8801, 251, mustRiderMoneyULID(t), now); !errors.Is(err, ErrWalletInsufficient) {
		t.Fatal(err)
	}
}

func TestRiderWithdrawalClaimReleaseAndExceptionAgainstIsolatedMySQL(t *testing.T) {
	db := riderMoneySchema(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	seedRiderMoneyWallet(t, db, 8802, 900, now)
	e := seedRiderWithdrawableEarning(t, db, 8802, 900, now)
	repo, ctx := NewRepository(db), context.Background()
	w, err := repo.CreateRiderWithdrawal(ctx, 8802, 500, mustRiderMoneyULID(t), now)
	if err != nil {
		t.Fatal(err)
	}
	claim := withdrawalTestAction(t, w, "claim")
	w, err = repo.ProcessRiderWithdrawal(ctx, "claim", claim, now)
	if err != nil {
		t.Fatal(err)
	}
	release := withdrawalTestAction(t, w, "release")
	release.ClaimKey, release.Reason, release.ConfirmedUnpaid = claim.ClaimKey, "未进行转账", true
	w, err = repo.ProcessRiderWithdrawal(ctx, "release", release, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ProcessRiderWithdrawal(ctx, "release", release, now); err != nil {
		t.Fatal(err)
	}
	claim2 := withdrawalTestAction(t, w, "claim")
	w, err = repo.ProcessRiderWithdrawal(ctx, "claim", claim2, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&FeeSettlement{}).Where("delivery_order_public_id = ?", e.DeliveryOrderPublicID).UpdateColumn("row_version", gorm.Expr("row_version + 1")).Error; err != nil {
		t.Fatal(err)
	}
	paid := withdrawalTestAction(t, w, "paid")
	paid.ClaimKey, paid.PaymentReference = claim2.ClaimKey, "transferred-but-source-changed"
	if _, err := repo.ProcessRiderWithdrawal(ctx, "paid", paid, now); !errors.Is(err, ErrWithdrawalSourceMismatch) {
		t.Fatalf("source drift: %v", err)
	}
	exception := withdrawalTestAction(t, w, "exception")
	exception.ClaimKey, exception.PaymentReference, exception.Reason = claim2.ClaimKey, paid.PaymentReference, "已操作转账，待人工对账"
	w, err = repo.ProcessRiderWithdrawal(ctx, "exception", exception, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ProcessRiderWithdrawal(ctx, "exception", exception, now); err != nil {
		t.Fatal(err)
	}
	c, err := repo.ReadRiderWithdrawalClaim(ctx, w.WithdrawPublicID)
	if err != nil || c.Status != RiderWithdrawalClaimException || c.PaymentReference != paid.PaymentReference {
		t.Fatalf("evidence lost: %+v %v", c, err)
	}
	release.ExpectedRowVersion, release.IdempotencyKey, release.ClaimKey = w.RowVersion, mustRiderMoneyULID(t), claim2.ClaimKey
	if _, err := repo.ProcessRiderWithdrawal(ctx, "release", release, now); !errors.Is(err, ErrWithdrawalClaimConflict) {
		t.Fatalf("exception released: %v", err)
	}
	if _, err := repo.RejectRiderWithdrawal(ctx, w.WithdrawPublicID, w.RowVersion, 99801, mustRiderMoneyULID(t), "reject", now); !errors.Is(err, ErrWithdrawalClaimConflict) {
		t.Fatal(err)
	}
	assertRiderMoneyBalance(t, db, 8802, 400)
	assertRiderMoneyCount(t, db, "delivery_rider_withdrawal_events", 4)
}

func TestRiderWithdrawalExceptionResolutionAgainstIsolatedMySQL(t *testing.T) {
	t.Run("resolve_paid_does_not_transfer_again", func(t *testing.T) {
		db := riderMoneySchema(t)
		now := time.Now().UTC().Truncate(time.Millisecond)
		seedRiderMoneyWallet(t, db, 8861, 700, now)
		e := seedRiderWithdrawableEarning(t, db, 8861, 700, now)
		repo, ctx := NewRepository(db), context.Background()
		w, err := repo.CreateRiderWithdrawal(ctx, 8861, 500, mustRiderMoneyULID(t), now)
		if err != nil {
			t.Fatal(err)
		}
		claim := withdrawalTestAction(t, w, "claim")
		w, err = repo.ProcessRiderWithdrawal(ctx, "claim", claim, now)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Model(&FeeSettlement{}).Where("delivery_order_public_id = ?", e.DeliveryOrderPublicID).UpdateColumn("row_version", gorm.Expr("row_version + 1")).Error; err != nil {
			t.Fatal(err)
		}
		exception := withdrawalTestAction(t, w, "exception")
		exception.ClaimKey, exception.PaymentReference, exception.Reason = claim.ClaimKey, "offline-original-receipt", "已转账，来源待复核"
		w, err = repo.ProcessRiderWithdrawal(ctx, "exception", exception, now)
		if err != nil {
			t.Fatal(err)
		}
		resolve := withdrawalTestAction(t, w, "resolve_paid")
		resolve.ClaimKey, resolve.PaymentReference = claim.ClaimKey, "offline-original-receipt"
		w, err = repo.ProcessRiderWithdrawal(ctx, "resolve_paid", resolve, now)
		if err != nil || w.Status != RiderWithdrawalPaid {
			t.Fatalf("resolve paid: %+v %v", w, err)
		}
		if _, err := repo.ProcessRiderWithdrawal(ctx, "resolve_paid", resolve, now); err != nil {
			t.Fatal("same resolution replay must be idempotent", err)
		}
		assertRiderMoneyBalance(t, db, 8861, 200)
		assertRiderMoneyCount(t, db, "delivery_rider_ledger", 2)
		assertRiderMoneyCount(t, db, "delivery_rider_withdrawal_events", 3)
		var allocation RiderWithdrawalAllocation
		if err := db.Where("withdraw_public_id = ?", w.WithdrawPublicID).First(&allocation).Error; err != nil || allocation.Status != RiderWithdrawalAllocationPaid {
			t.Fatalf("allocation not paid: %+v %v", allocation, err)
		}
	})

	t.Run("resolve_unpaid_releases_and_refunds_once", func(t *testing.T) {
		db := riderMoneySchema(t)
		now := time.Now().UTC().Truncate(time.Millisecond)
		seedRiderMoneyWallet(t, db, 8862, 700, now)
		seedRiderWithdrawableEarning(t, db, 8862, 700, now)
		repo, ctx := NewRepository(db), context.Background()
		w, err := repo.CreateRiderWithdrawal(ctx, 8862, 400, mustRiderMoneyULID(t), now)
		if err != nil {
			t.Fatal(err)
		}
		claim := withdrawalTestAction(t, w, "claim")
		w, err = repo.ProcessRiderWithdrawal(ctx, "claim", claim, now)
		if err != nil {
			t.Fatal(err)
		}
		exception := withdrawalTestAction(t, w, "exception")
		exception.ClaimKey, exception.PaymentReference, exception.Reason = claim.ClaimKey, "结果未知", "付款未执行，待确认"
		w, err = repo.ProcessRiderWithdrawal(ctx, "exception", exception, now)
		if err != nil {
			t.Fatal(err)
		}
		resolve := withdrawalTestAction(t, w, "resolve_unpaid")
		resolve.ClaimKey, resolve.ConfirmedUnpaid, resolve.Reason = claim.ClaimKey, true, "确认未执行转账，退回余额"
		w, err = repo.ProcessRiderWithdrawal(ctx, "resolve_unpaid", resolve, now)
		if err != nil || w.Status != RiderWithdrawalRejected {
			t.Fatalf("resolve unpaid: %+v %v", w, err)
		}
		if _, err := repo.ProcessRiderWithdrawal(ctx, "resolve_unpaid", resolve, now); err != nil {
			t.Fatal("same unpaid resolution replay must be idempotent", err)
		}
		assertRiderMoneyBalance(t, db, 8862, 700)
		assertRiderMoneyCount(t, db, "delivery_rider_ledger", 3)
		assertRiderMoneyCount(t, db, "delivery_rider_withdrawal_events", 3)
		var allocation RiderWithdrawalAllocation
		if err := db.Where("withdraw_public_id = ?", w.WithdrawPublicID).First(&allocation).Error; err != nil || allocation.Status != RiderWithdrawalAllocationReleased {
			t.Fatalf("allocation not released: %+v %v", allocation, err)
		}
		claimValue, err := repo.ReadRiderWithdrawalClaim(ctx, w.WithdrawPublicID)
		if err != nil || claimValue.Status != RiderWithdrawalClaimReleased {
			t.Fatalf("claim not released: %+v %v", claimValue, err)
		}
	})
}

func TestRiderWithdrawalPaymentReferenceIsGloballyBoundAgainstIsolatedMySQL(t *testing.T) {
	db := riderMoneySchema(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	repo, ctx := NewRepository(db), context.Background()

	seedRiderMoneyWallet(t, db, 8871, 900, now)
	seedRiderWithdrawableEarning(t, db, 8871, 900, now)
	w1, err := repo.CreateRiderWithdrawal(ctx, 8871, 400, mustRiderMoneyULID(t), now)
	if err != nil {
		t.Fatal(err)
	}
	claim1 := withdrawalTestAction(t, w1, "claim")
	w1, err = repo.ProcessRiderWithdrawal(ctx, "claim", claim1, now)
	if err != nil {
		t.Fatal(err)
	}
	exception := withdrawalTestAction(t, w1, "exception")
	exception.ClaimKey, exception.PaymentReference, exception.Reason = claim1.ClaimKey, "结果未知", "转账结果待核实"
	w1, err = repo.ProcessRiderWithdrawal(ctx, "exception", exception, now)
	if err != nil {
		t.Fatal(err)
	}
	resolve := withdrawalTestAction(t, w1, "resolve_paid")
	resolve.ClaimKey, resolve.PaymentReference = claim1.ClaimKey, "manual-receipt-8871"
	w1, err = repo.ProcessRiderWithdrawal(ctx, "resolve_paid", resolve, now)
	if err != nil || w1.Status != RiderWithdrawalPaid {
		t.Fatalf("placeholder resolution: %+v %v", w1, err)
	}
	var binding RiderWithdrawalPaymentReference
	if err := db.Where("withdraw_public_id = ?", w1.WithdrawPublicID).First(&binding).Error; err != nil || binding.PaymentReference != resolve.PaymentReference {
		t.Fatalf("payment binding missing: %+v %v", binding, err)
	}
	if err := repo.CheckWithdrawalReadiness(ctx); err != nil {
		t.Fatal(err)
	}

	seedRiderMoneyWallet(t, db, 8872, 900, now)
	seedRiderWithdrawableEarning(t, db, 8872, 900, now)
	w2, err := repo.CreateRiderWithdrawal(ctx, 8872, 400, mustRiderMoneyULID(t), now)
	if err != nil {
		t.Fatal(err)
	}
	claim2 := withdrawalTestAction(t, w2, "claim")
	w2, err = repo.ProcessRiderWithdrawal(ctx, "claim", claim2, now)
	if err != nil {
		t.Fatal(err)
	}
	paid2 := withdrawalTestAction(t, w2, "paid")
	paid2.ClaimKey, paid2.PaymentReference = claim2.ClaimKey, resolve.PaymentReference
	if _, err := repo.ProcessRiderWithdrawal(ctx, "paid", paid2, now); !errors.Is(err, ErrWithdrawalPaymentReferenceConflict) {
		t.Fatalf("cross-withdrawal payment reference reuse: %v", err)
	}
}

func TestRiderWithdrawalRejectClaimRaceAgainstIsolatedMySQL(t *testing.T) {
	db := riderMoneySchema(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	repo, ctx := NewRepository(db), context.Background()
	for i := uint64(0); i < 5; i++ {
		rider := uint64(8810) + i
		seedRiderMoneyWallet(t, db, rider, 700, now)
		seedRiderWithdrawableEarning(t, db, rider, 700, now)
		w, err := repo.CreateRiderWithdrawal(ctx, rider, 700, mustRiderMoneyULID(t), now)
		if err != nil {
			t.Fatal(err)
		}
		claim := withdrawalTestAction(t, w, "claim")
		reject := withdrawalTestAction(t, w, "reject")
		reject.Reason = "not paid"
		start := make(chan struct{})
		var group sync.WaitGroup
		var claimErr, rejectErr error
		group.Add(2)
		go func() {
			defer group.Done()
			<-start
			_, claimErr = repo.ProcessRiderWithdrawal(ctx, "claim", claim, now)
		}()
		go func() {
			defer group.Done()
			<-start
			_, rejectErr = repo.ProcessRiderWithdrawal(ctx, "reject", reject, now)
		}()
		close(start)
		group.Wait()
		if (claimErr == nil) == (rejectErr == nil) {
			t.Fatalf("must have one winner: %v %v", claimErr, rejectErr)
		}
		if claimErr == nil {
			assertRiderMoneyBalance(t, db, rider, 0)
		} else {
			assertRiderMoneyBalance(t, db, rider, 700)
		}
	}
}

func TestRiderWithdrawalNotificationRollbackAndReplayAgainstIsolatedMySQL(t *testing.T) {
	db := riderMoneySchema(t)
	if err := db.Exec("CREATE TABLE withdrawal_test_notifications (id BIGINT PRIMARY KEY AUTO_INCREMENT, event_id VARCHAR(26) UNIQUE)").Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	seedRiderMoneyWallet(t, db, 8840, 700, now)
	seedRiderWithdrawableEarning(t, db, 8840, 700, now)
	fail := true
	want := errors.New("injected notification failure")
	appender := func(tx *gorm.DB, event NotificationEvent) error {
		if strings.Contains(string(event.Payload), "private-receipt") {
			t.Fatal("private receipt leaked")
		}
		if err := tx.Exec("INSERT INTO withdrawal_test_notifications (event_id) VALUES (?) ON DUPLICATE KEY UPDATE event_id = VALUES(event_id)", event.EventPublicID).Error; err != nil {
			return err
		}
		if fail {
			return want
		}
		return nil
	}
	repo, ctx := NewRepository(db, WithNotificationEvents(appender)), context.Background()
	key := mustRiderMoneyULID(t)
	if _, err := repo.CreateRiderWithdrawal(ctx, 8840, 500, key, now); !errors.Is(err, want) {
		t.Fatal(err)
	}
	assertRiderMoneyBalance(t, db, 8840, 700)
	assertRiderMoneyCount(t, db, "delivery_rider_withdrawal_allocations", 0)
	assertRiderMoneyCount(t, db, "delivery_rider_withdrawals", 0)
	assertRiderMoneyCount(t, db, "delivery_rider_ledger", 1)
	assertRiderMoneyCount(t, db, "withdrawal_test_notifications", 0)
	fail = false
	w, err := repo.CreateRiderWithdrawal(ctx, 8840, 500, key, now)
	if err != nil {
		t.Fatal(err)
	}
	assertRiderMoneyCount(t, db, "withdrawal_test_notifications", 1)
	if _, err := repo.CreateRiderWithdrawal(ctx, 8840, 500, key, now); err != nil {
		t.Fatal(err)
	}
	claim := withdrawalTestAction(t, w, "claim")
	w, err = repo.ProcessRiderWithdrawal(ctx, "claim", claim, now)
	if err != nil {
		t.Fatal(err)
	}
	paid := withdrawalTestAction(t, w, "paid")
	paid.ClaimKey, paid.PaymentReference = claim.ClaimKey, "private-receipt"
	fail = true
	if _, err := repo.ProcessRiderWithdrawal(ctx, "paid", paid, now); !errors.Is(err, want) {
		t.Fatal(err)
	}
	c, err := repo.ReadRiderWithdrawalClaim(ctx, w.WithdrawPublicID)
	if err != nil || c.Status != RiderWithdrawalClaimActive {
		t.Fatalf("paid state was not rolled back: %+v %v", c, err)
	}
	var persisted RiderWithdrawal
	if err := db.Where("withdraw_public_id = ?", w.WithdrawPublicID).First(&persisted).Error; err != nil || persisted.Status != RiderWithdrawalProcessing {
		t.Fatalf("paid withdrawal was not rolled back: %+v %v", persisted, err)
	}
	// The successful creation above already owns one notification; the failed
	// paid transition must not add a second row because its whole transaction
	// (including the appender write) was rolled back.
	assertRiderMoneyCount(t, db, "withdrawal_test_notifications", 1)
	fail = false
	if _, err := repo.ProcessRiderWithdrawal(ctx, "paid", paid, now); err != nil {
		t.Fatal(err)
	}
	assertRiderMoneyCount(t, db, "withdrawal_test_notifications", 2)
	if _, err := repo.ProcessRiderWithdrawal(ctx, "paid", paid, now); err != nil {
		t.Fatal(err)
	}
	assertRiderMoneyCount(t, db, "withdrawal_test_notifications", 2)
	assertRiderMoneyBalance(t, db, 8840, 200)
}

func TestRiderWithdrawalUnfundedAndHistoricalReadinessAgainstIsolatedMySQL(t *testing.T) {
	db := riderMoneySchema(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	seedRiderMoneyWallet(t, db, 8850, 1000, now)
	repo, ctx := NewRepository(db), context.Background()
	if _, err := repo.CreateRiderWithdrawal(ctx, 8850, 1, mustRiderMoneyULID(t), now); !errors.Is(err, ErrWalletInsufficient) {
		t.Fatal(err)
	}
	seedRiderMoneyEarning(t, db, 8850, 1000, now)
	if _, err := repo.CreateRiderWithdrawal(ctx, 8850, 1, mustRiderMoneyULID(t), now); !errors.Is(err, ErrWalletInsufficient) {
		t.Fatal(err)
	}
	if amount, err := repo.ReadRiderWithdrawable(ctx, 8850); err != nil || amount != 0 {
		t.Fatalf("unfunded=%d %v", amount, err)
	}
	var wallet RiderWallet
	if err := db.Where("rider_user_id = ?", 8850).First(&wallet).Error; err != nil {
		t.Fatal(err)
	}
	w := RiderWithdrawal{WithdrawPublicID: mustRiderMoneyULID(t), RiderUserID: 8850, WalletPublicID: wallet.WalletPublicID, AmountCents: 10, Currency: "CNY", Status: RiderWithdrawalProcessing, RowVersion: 1, IdempotencyKey: mustRiderMoneyULID(t), CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&w).Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.CheckWithdrawalReadiness(ctx); !errors.Is(err, ErrWithdrawalSourceMismatch) {
		t.Fatalf("historical unallocated accepted: %v", err)
	}
}
