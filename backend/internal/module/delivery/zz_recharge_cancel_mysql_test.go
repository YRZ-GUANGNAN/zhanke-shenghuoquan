package delivery

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRechargeCancellationByRequestPreventsLatePrepayAgainstRealMySQL84(t *testing.T) {
	db := openD3MySQL(t)
	cleanupD3(t, db)
	t.Cleanup(func() { cleanupD3(t, db) })
	ctx := context.Background()
	now := time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)
	repo := NewRepository(db)
	funds, _ := NewFundsService(repo, repo, func() time.Time { return now })
	key := mustFundsULID(t)
	missing, err := funds.FindRechargeByRequest(ctx, fundsMerchantOne, key, 123)
	if err != nil || missing != nil {
		t.Fatalf("missing query=%+v err=%v", missing, err)
	}
	assertD3Count(t, db, (&PaymentAttempt{}).TableName(), "merchant_user_id = ?", 0, fundsMerchantOne)
	const workers = 8
	start := make(chan struct{})
	results := make(chan PaymentAttempt, workers)
	failures := make(chan error, workers)
	var wait sync.WaitGroup
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			value, err := funds.RequestRechargeCancellationByRequest(ctx, fundsMerchantOne, key, 123)
			results <- value
			failures <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	var paymentID string
	for value := range results {
		if value.Status != PaymentClosed || value.CancelRequestedAt == nil {
			t.Fatalf("not a closed tombstone=%+v", value)
		}
		if paymentID != "" && paymentID != value.PaymentPublicID {
			t.Fatal("duplicate tombstones")
		}
		paymentID = value.PaymentPublicID
	}
	assertD3Count(t, db, (&PaymentAttempt{}).TableName(), "merchant_user_id = ?", 1, fundsMerchantOne)
	late, err := funds.StartRecharge(ctx, fundsMerchantOne, 123, key, PaymentWechat)
	if err != nil || late.Attempt.PaymentPublicID != paymentID || late.Attempt.Status != PaymentClosed {
		t.Fatalf("late start=%+v err=%v", late, err)
	}
	if _, err := funds.MarkPrepayRequested(ctx, paymentID, late.Attempt.RowVersion, "late-payer"); !errors.Is(err, ErrPaymentNotPending) {
		t.Fatalf("late prepay=%v", err)
	}
	if value, err := funds.FindRechargeByRequest(ctx, fundsMerchantTwo, key, 123); err != nil || value != nil {
		t.Fatalf("other owner read=%+v err=%v", value, err)
	}
	if _, err := funds.RequestRechargeCancellationByRequest(ctx, fundsMerchantOne, key, 124); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("amount mismatch=%v", err)
	}
	activeKey := mustFundsULID(t)
	active, err := funds.StartRecharge(ctx, fundsMerchantOne, 456, activeKey, PaymentWechat)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := funds.MarkPrepayIssued(ctx, active.Attempt.PaymentPublicID, active.Attempt.RowVersion, "active-cancel", "payer")
	if err != nil {
		t.Fatal(err)
	}
	requested, err := funds.RequestRechargeCancellationByRequest(ctx, fundsMerchantOne, activeKey, 456)
	if err != nil || requested.PaymentPublicID != issued.PaymentPublicID || requested.Status != PaymentPrepayIssued || requested.CancelRequestedAt == nil {
		t.Fatalf("active cancellation=%+v err=%v", requested, err)
	}
	t.Log("recharge_cancel_by_request=PASS missing_query_no_write+8_worker_tombstone_dedup+late_start_terminal+owner_fence+amount_fence+existing_intent")
}

func Test05RechargeCancellationMigrationRollbackApplyAgainstRealMySQL84(t *testing.T) {
	db := openD3MySQL(t)
	cleanupD3(t, db)
	t.Cleanup(func() { cleanupD3(t, db) })
	ctx := context.Background()
	now := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	repo := NewRepository(db)
	funds, _ := NewFundsService(repo, repo, func() time.Time { return now })
	start, err := funds.StartRecharge(ctx, fundsMerchantOne, 123, mustFundsULID(t), PaymentWechat)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := funds.MarkPrepayIssued(ctx, start.Attempt.PaymentPublicID, start.Attempt.RowVersion, "cancel-migration", "cancel-payer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := funds.RequestRechargeCancellation(ctx, fundsMerchantOne, issued.PaymentPublicID); err != nil {
		t.Fatal(err)
	}
	if err := executeRiderMoneySQL(db, "../../../migrations/rollback/052_delivery_recharge_cancellation.sql"); err == nil || !strings.Contains(err.Error(), "pending recharge cancellation exists") {
		t.Fatalf("rollback guard=%v", err)
	}
	if !db.Migrator().HasColumn(&PaymentAttempt{}, "cancel_requested_at") {
		t.Fatal("rollback dropped pending intent")
	}
	cleanupD3(t, db)
	applyD1SQLFile(t, db, "../../../migrations/rollback/052_delivery_recharge_cancellation.sql")
	if db.Migrator().HasColumn(&PaymentAttempt{}, "cancel_requested_at") {
		t.Fatal("rollback left intent column")
	}
	applyD1SQLFile(t, db, "../../../migrations/052_delivery_recharge_cancellation.sql")
	if !db.Migrator().HasColumn(&PaymentAttempt{}, "cancel_requested_at") {
		t.Fatal("reapply missing intent column")
	}
	t.Log("migration_052=PASS nullable_intent+pending_rollback_guard+rollback+reapply")
}

func TestRechargeCancellationOwnershipLeaseAndPaidRaceAgainstRealMySQL84(t *testing.T) {
	db := openD3MySQL(t)
	cleanupD3(t, db)
	t.Cleanup(func() { cleanupD3(t, db) })
	ctx := context.Background()
	now := time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)
	repo := NewRepository(db)
	funds, _ := NewFundsService(repo, repo, func() time.Time { return now })
	start, err := funds.StartRecharge(ctx, fundsMerchantOne, 123, mustFundsULID(t), PaymentWechat)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := funds.MarkPrepayIssued(ctx, start.Attempt.PaymentPublicID, start.Attempt.RowVersion, "cancel-prepay", "cancel-payer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := funds.RequestRechargeCancellation(ctx, fundsMerchantTwo, issued.PaymentPublicID); !errors.Is(err, ErrPaymentForbidden) {
		t.Fatalf("cross-owner cancellation: %v", err)
	}
	cancelled, err := funds.RequestRechargeCancellation(ctx, fundsMerchantOne, issued.PaymentPublicID)
	if err != nil || cancelled.CancelRequestedAt == nil || cancelled.Status != PaymentPrepayIssued || cancelled.NextVerifyAt == nil || cancelled.NextVerifyAt.After(now) {
		t.Fatalf("cancel=%+v err=%v", cancelled, err)
	}
	lease, found, err := funds.ClaimPaymentVerification(ctx, issued.PaymentPublicID, now, 30*time.Second)
	if err != nil || !found || lease.Attempt.CancelRequestedAt == nil {
		t.Fatalf("immediate claim=%+v found=%v err=%v", lease, found, err)
	}
	before := lease.Attempt.RowVersion
	again, err := funds.RequestRechargeCancellation(ctx, fundsMerchantOne, issued.PaymentPublicID)
	if err != nil || again.RowVersion != before || again.VerifyLeaseUntil == nil || !again.VerifyLeaseUntil.Equal(*lease.Attempt.VerifyLeaseUntil) {
		t.Fatalf("active lease changed=%+v err=%v", again, err)
	}
	if _, found, err := funds.ClaimPaymentVerification(ctx, issued.PaymentPublicID, now, 30*time.Second); err != nil || found {
		t.Fatalf("stole active lease=%v err=%v", found, err)
	}
	if _, err := funds.MarkPrepayRequested(ctx, issued.PaymentPublicID, again.RowVersion, "cancel-payer"); !errors.Is(err, ErrPaymentNotPending) {
		t.Fatalf("reopened requested prepay: %v", err)
	}
	if _, err := funds.MarkPrepayIssued(ctx, issued.PaymentPublicID, again.RowVersion, "cancel-prepay", "cancel-payer"); !errors.Is(err, ErrPaymentNotPending) {
		t.Fatalf("reopened issued prepay: %v", err)
	}
	confirmation := PaymentConfirmation{OutTradeNo: issued.OutTradeNo, WechatTransactionID: "cancel-paid-race", PayerOpenID: "cancel-payer", AmountCents: 123, ProviderStatus: "SUCCESS", RawBody: []byte(`{"fixture":"paid-before-close"}`), PaidAt: now}
	if _, _, err := funds.ConfirmWechatPayment(ctx, confirmation); err != nil {
		t.Fatal(err)
	}
	if _, _, err := funds.ConfirmWechatPayment(ctx, confirmation); err != nil {
		t.Fatal(err)
	}
	if err := funds.UpdatePaymentVerification(ctx, lease, PaymentVerificationUpdate{Status: PaymentClosed, ProviderStatus: "CLOSED", UpdatedAt: now}); !errors.Is(err, ErrVerificationLease) {
		t.Fatalf("stale close overwrote callback: %v", err)
	}
	paid, err := funds.RequestRechargeCancellation(ctx, fundsMerchantOne, issued.PaymentPublicID)
	if err != nil || paid.Status != PaymentPaid {
		t.Fatalf("paid cancellation=%+v err=%v", paid, err)
	}
	wallet, err := funds.Wallet(ctx, fundsMerchantOne)
	if err != nil || wallet.AvailableBalanceCents != 123 {
		t.Fatalf("wallet=%+v err=%v", wallet, err)
	}
	assertD3Count(t, db, (&WalletLedger{}).TableName(), "merchant_user_id = ?", 1, fundsMerchantOne)
	t.Log("recharge_cancel=PASS ownership+immediate_claim+lease_exclusion+no_reopen+paid_CAS_wins+credit_once")
}

func TestRechargeCancellationUnissuedAndScopeAgainstRealMySQL84(t *testing.T) {
	db := openD3MySQL(t)
	cleanupD3(t, db)
	t.Cleanup(func() { cleanupD3(t, db) })
	ctx := context.Background()
	now := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	repo := NewRepository(db)
	funds, _ := NewFundsService(repo, repo, func() time.Time { return now })
	start, err := funds.StartRecharge(ctx, fundsMerchantOne, 123, mustFundsULID(t), PaymentWechat)
	if err != nil {
		t.Fatal(err)
	}
	closed, err := funds.RequestRechargeCancellation(ctx, fundsMerchantOne, start.Attempt.PaymentPublicID)
	if err != nil || closed.Status != PaymentClosed || closed.CancelRequestedAt == nil || closed.ClosedAt == nil {
		t.Fatalf("unissued=%+v err=%v", closed, err)
	}
	again, err := funds.RequestRechargeCancellation(ctx, fundsMerchantOne, start.Attempt.PaymentPublicID)
	if err != nil || again.RowVersion != closed.RowVersion {
		t.Fatalf("closed replay=%+v err=%v", again, err)
	}
	if _, err := funds.MarkPrepayRequested(ctx, closed.PaymentPublicID, closed.RowVersion, "payer"); !errors.Is(err, ErrPaymentNotPending) {
		t.Fatal(err)
	}
	mock, err := funds.StartRecharge(ctx, fundsMerchantOne, 123, mustFundsULID(t), PaymentMock)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := funds.RequestRechargeCancellation(ctx, fundsMerchantOne, mock.Attempt.PaymentPublicID); !errors.Is(err, ErrPaymentForbidden) {
		t.Fatalf("mock cancellation=%v", err)
	}
	base, _ := NewService(repo, func() time.Time { return now })
	dispatch, _ := NewDispatchService(repo, base, activeEligibility{}, FeePolicy{DeliveryFeeCents: 150, RiderFeeCents: 100, PlatformCutCents: 50}, func() time.Time { return now })
	order := createFundsOrder(t, ctx, dispatch, fundsMerchantOne)
	fee, err := funds.StartOrderPayment(ctx, fundsMerchantOne, order.DeliveryOrderPublicID, order.RowVersion, mustFundsULID(t), PaymentWechat)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := funds.RequestRechargeCancellation(ctx, fundsMerchantOne, fee.Attempt.PaymentPublicID); !errors.Is(err, ErrPaymentForbidden) {
		t.Fatalf("order fee cancellation=%v", err)
	}
	t.Log("recharge_cancel_scope=PASS untouched_unissued_close+closed_idempotency+reject_mock+reject_order_fee")
}
