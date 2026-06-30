package delivery

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
)

const (
	fundsMerchantOne = uint64(985001)
	fundsMerchantTwo = uint64(985002)
	fundsRider       = uint64(985101)
)

func Test02FundsMigrationRollbackApplyAgainstRealMySQL84(t *testing.T) {
	db := openD2MySQL(t)
	assertD1MySQL84(t, db)
	ensureD3Schema(t, db)
	cleanupD3(t, db)

	applyD1SQLFile(t, db, "../../../migrations/rollback/025_delivery_payment_wallet.sql")
	for _, table := range d3Tables() {
		if db.Migrator().HasTable(table) {
			t.Fatalf("rollback 025 left %s", table)
		}
	}
	applyD1SQLFile(t, db, "../../../migrations/025_delivery_payment_wallet.sql")
	assertD3Schema(t, db)
	t.Log("migration_025_apply_rollback_apply=PASS tables=6 foreign_keys=0 money_checks=PASS")
}

func TestFundsWalletPaymentAtomicityIdempotencyAndRefundAgainstRealMySQL84(t *testing.T) {
	db := openD3MySQL(t)
	cleanupD3(t, db)
	t.Cleanup(func() { cleanupD3(t, db) })
	ctx := context.Background()
	now := time.Date(2026, 6, 30, 8, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	base, _ := NewService(repository, func() time.Time { return now })
	dispatch, _ := NewDispatchService(
		repository, base, activeEligibility{},
		FeePolicy{DeliveryFeeCents: 150, RiderFeeCents: 100, PlatformCutCents: 50},
		func() time.Time { return now },
	)
	funds, _ := NewFundsService(repository, repository, func() time.Time { return now })

	rechargeRequest := mustFundsULID(t)
	recharge, err := funds.StartRecharge(ctx, fundsMerchantOne, 500, rechargeRequest, PaymentMock)
	if err != nil {
		t.Fatal(err)
	}
	if recharge.Wallet == nil || recharge.Wallet.AvailableBalanceCents != 500 ||
		recharge.Ledger == nil || recharge.Ledger.EntryType != LedgerRecharge {
		t.Fatalf("mock recharge=%#v", recharge)
	}
	repeatedRecharge, err := funds.StartRecharge(ctx, fundsMerchantOne, 500, rechargeRequest, PaymentMock)
	if err != nil || repeatedRecharge.Wallet == nil || repeatedRecharge.Wallet.AvailableBalanceCents != 500 {
		t.Fatalf("idempotent recharge=%#v err=%v", repeatedRecharge, err)
	}
	assertD3Count(t, db, (&PaymentAttempt{}).TableName(), "merchant_user_id = ? AND purpose = ?", 1, fundsMerchantOne, PaymentWalletRecharge)
	assertD3Count(t, db, (&WalletLedger{}).TableName(), "merchant_user_id = ? AND entry_type = ?", 1, fundsMerchantOne, LedgerRecharge)

	order := createFundsOrder(t, ctx, dispatch, fundsMerchantOne)
	payKey := mustFundsULID(t)
	paid, err := funds.PayOrderWithWallet(ctx, fundsMerchantOne, order.DeliveryOrderPublicID, order.RowVersion, payKey)
	if err != nil {
		t.Fatal(err)
	}
	if paid.Order == nil || paid.Order.Status != StatusReadyForClaim || paid.Wallet == nil ||
		paid.Wallet.AvailableBalanceCents != 350 || paid.Ledger == nil ||
		paid.Settlement == nil || paid.Settlement.Source != SettlementWallet ||
		paid.Outbox == nil {
		t.Fatalf("wallet payment=%#v", paid)
	}
	repeatedPay, err := funds.PayOrderWithWallet(
		ctx, fundsMerchantOne, order.DeliveryOrderPublicID, order.RowVersion, payKey,
	)
	if err != nil || repeatedPay.Settlement == nil {
		t.Fatalf("idempotent wallet payment=%#v err=%v", repeatedPay, err)
	}
	assertD3Count(t, db, (&FeeSettlement{}).TableName(), "delivery_order_public_id = ?", 1, order.DeliveryOrderPublicID)
	assertD3Count(t, db, (&WalletLedger{}).TableName(), "merchant_user_id = ? AND entry_type = ?", 1, fundsMerchantOne, LedgerOrderFee)

	poorOrder := createFundsOrder(t, ctx, dispatch, fundsMerchantTwo)
	if _, err := funds.PayOrderWithWallet(
		ctx, fundsMerchantTwo, poorOrder.DeliveryOrderPublicID, poorOrder.RowVersion, mustFundsULID(t),
	); !errors.Is(err, ErrWalletInsufficient) {
		t.Fatalf("insufficient wallet returned %v", err)
	}
	poorLoaded, err := repository.Find(ctx, poorOrder.DeliveryOrderPublicID)
	if err != nil || poorLoaded.Status != StatusPendingFeePayment {
		t.Fatalf("insufficient rollback order=%#v err=%v", poorLoaded, err)
	}
	assertD3Count(t, db, (&FeeSettlement{}).TableName(), "delivery_order_public_id = ?", 0, poorOrder.DeliveryOrderPublicID)
	if _, err := funds.PayOrderWithWallet(
		ctx, fundsMerchantTwo, order.DeliveryOrderPublicID, order.RowVersion, mustFundsULID(t),
	); !errors.Is(err, ErrForbiddenOrder) {
		t.Fatalf("cross-merchant payment returned %v", err)
	}

	refundKey := mustFundsULID(t)
	refunded, err := funds.RequestMerchantRefund(
		ctx, fundsMerchantOne, order.DeliveryOrderPublicID, paid.Order.RowVersion, refundKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	if refunded.Refund.Status != RefundSucceeded || refunded.Settlement.Status != SettlementRefunded ||
		refunded.Wallet == nil || refunded.Wallet.AvailableBalanceCents != 500 ||
		refunded.Ledger == nil || refunded.Ledger.EntryType != LedgerRefund ||
		refunded.Order.Status != StatusCancelled {
		t.Fatalf("wallet refund=%#v", refunded)
	}
	repeatedRefund, err := funds.RequestMerchantRefund(
		ctx, fundsMerchantOne, order.DeliveryOrderPublicID, paid.Order.RowVersion, refundKey,
	)
	if err != nil || repeatedRefund.Refund.RefundPublicID != refunded.Refund.RefundPublicID {
		t.Fatalf("idempotent refund=%#v err=%v", repeatedRefund, err)
	}
	assertD3Count(t, db, (&Refund{}).TableName(), "delivery_order_public_id = ?", 1, order.DeliveryOrderPublicID)
	assertD3Count(t, db, (&WalletLedger{}).TableName(), "merchant_user_id = ? AND entry_type = ?", 1, fundsMerchantOne, LedgerRefund)
	t.Log("wallet_funds=PASS recharge+atomic_debit+insufficient_rollback+owner_fence+idempotent_refund")
}

func TestFundsSingleSettlementCompetitionAndWechatCallbackAgainstRealMySQL84(t *testing.T) {
	db := openD3MySQL(t)
	cleanupD3(t, db)
	t.Cleanup(func() { cleanupD3(t, db) })
	ctx := context.Background()
	now := time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	base, _ := NewService(repository, func() time.Time { return now })
	dispatch, _ := NewDispatchService(
		repository, base, activeEligibility{},
		FeePolicy{DeliveryFeeCents: 150, RiderFeeCents: 100, PlatformCutCents: 50},
		func() time.Time { return now },
	)
	funds, _ := NewFundsService(repository, repository, func() time.Time { return now })
	if _, err := funds.StartRecharge(ctx, fundsMerchantOne, 300, mustFundsULID(t), PaymentMock); err != nil {
		t.Fatal(err)
	}
	order := createFundsOrder(t, ctx, dispatch, fundsMerchantOne)

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		_, err := funds.PayOrderWithWallet(
			ctx, fundsMerchantOne, order.DeliveryOrderPublicID, order.RowVersion, mustFundsULID(t),
		)
		results <- err
	}()
	go func() {
		defer wait.Done()
		<-start
		_, err := funds.StartOrderPayment(
			ctx, fundsMerchantOne, order.DeliveryOrderPublicID, order.RowVersion,
			mustFundsULID(t), PaymentMock,
		)
		results <- err
	}()
	close(start)
	wait.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, ErrAlreadySettled) && !errors.Is(err, ErrPaymentNotPending) &&
			!errors.Is(err, ErrConcurrentUpdate) {
			t.Fatalf("unexpected payment competition error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("payment competition successes=%d want=1", successes)
	}
	assertD3Count(t, db, (&FeeSettlement{}).TableName(), "delivery_order_public_id = ?", 1, order.DeliveryOrderPublicID)

	wechatOrder := createFundsOrder(t, ctx, dispatch, fundsMerchantOne)
	started, err := funds.StartOrderPayment(
		ctx, fundsMerchantOne, wechatOrder.DeliveryOrderPublicID, wechatOrder.RowVersion,
		mustFundsULID(t), PaymentWechat,
	)
	if err != nil {
		t.Fatal(err)
	}
	confirmation := PaymentConfirmation{
		OutTradeNo: started.Attempt.OutTradeNo, WechatTransactionID: "wx-d3-transaction-1",
		AmountCents: 150, RawBody: []byte(`{"fixture":"d3-payment"}`), PaidAt: now,
	}
	confirmed, duplicate, err := funds.ConfirmWechatPayment(ctx, confirmation)
	if err != nil || duplicate || confirmed.Order == nil ||
		confirmed.Order.Status != StatusReadyForClaim || confirmed.Settlement == nil {
		t.Fatalf("wechat confirmation=%#v duplicate=%v err=%v", confirmed, duplicate, err)
	}
	_, duplicate, err = funds.ConfirmWechatPayment(ctx, confirmation)
	if err != nil || !duplicate {
		t.Fatalf("duplicate callback duplicate=%v err=%v", duplicate, err)
	}
	assertD3Count(t, db, (&PaymentNotification{}).TableName(), "out_trade_no = ?", 1, started.Attempt.OutTradeNo)
	assertD3Count(t, db, (&FeeSettlement{}).TableName(), "delivery_order_public_id = ?", 1, wechatOrder.DeliveryOrderPublicID)

	wrongOrder := createFundsOrder(t, ctx, dispatch, fundsMerchantOne)
	wrong, err := funds.StartOrderPayment(
		ctx, fundsMerchantOne, wrongOrder.DeliveryOrderPublicID, wrongOrder.RowVersion,
		mustFundsULID(t), PaymentWechat,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := funds.ConfirmWechatPayment(ctx, PaymentConfirmation{
		OutTradeNo: wrong.Attempt.OutTradeNo, WechatTransactionID: "wx-d3-wrong",
		AmountCents: 151, RawBody: []byte(`{"fixture":"wrong-amount"}`), PaidAt: now,
	}); !errors.Is(err, ErrPaymentAmount) {
		t.Fatalf("wrong callback amount returned %v", err)
	}
	assertD3Count(t, db, (&PaymentNotification{}).TableName(), "out_trade_no = ?", 0, wrong.Attempt.OutTradeNo)
	t.Log("single_settlement=PASS wallet_vs_mock; wechat_callback=PASS amount+dedup+ready_outbox")
}

func TestFundsRefundRulesAndWechatRetryAgainstRealMySQL84(t *testing.T) {
	db := openD3MySQL(t)
	cleanupD3(t, db)
	t.Cleanup(func() { cleanupD3(t, db) })
	ctx := context.Background()
	now := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	base, _ := NewService(repository, func() time.Time { return now })
	dispatch, _ := NewDispatchService(
		repository, base, activeEligibility{},
		FeePolicy{DeliveryFeeCents: 150, RiderFeeCents: 100, PlatformCutCents: 50},
		func() time.Time { return now },
	)
	funds, _ := NewFundsService(repository, repository, func() time.Time { return now })

	order := createFundsOrder(t, ctx, dispatch, fundsMerchantOne)
	paid, err := funds.StartOrderPayment(
		ctx, fundsMerchantOne, order.DeliveryOrderPublicID, order.RowVersion,
		mustFundsULID(t), PaymentMock,
	)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := dispatch.ClaimOrder(ctx, fundsRider, order.DeliveryOrderPublicID, paid.Order.RowVersion)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := funds.RequestMerchantRefund(
		ctx, fundsMerchantOne, order.DeliveryOrderPublicID, claimed.RowVersion, mustFundsULID(t),
	); !errors.Is(err, ErrRefundNotAllowed) {
		t.Fatalf("merchant claimed refund returned %v", err)
	}
	adminRefund, err := funds.RequestAdminRefund(
		ctx, order.DeliveryOrderPublicID, claimed.RowVersion, mustFundsULID(t),
	)
	if err != nil || adminRefund.Refund.Status != RefundSucceeded ||
		adminRefund.Order.Status != StatusCancelled {
		t.Fatalf("admin claimed refund=%#v err=%v", adminRefund, err)
	}

	wechatOrder := createFundsOrder(t, ctx, dispatch, fundsMerchantOne)
	started, err := funds.StartOrderPayment(
		ctx, fundsMerchantOne, wechatOrder.DeliveryOrderPublicID, wechatOrder.RowVersion,
		mustFundsULID(t), PaymentWechat,
	)
	if err != nil {
		t.Fatal(err)
	}
	confirmed, _, err := funds.ConfirmWechatPayment(ctx, PaymentConfirmation{
		OutTradeNo: started.Attempt.OutTradeNo, WechatTransactionID: "wx-d3-refund",
		AmountCents: 150, RawBody: []byte(`{"fixture":"refund"}`), PaidAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	requested, err := funds.RequestMerchantRefund(
		ctx, fundsMerchantOne, wechatOrder.DeliveryOrderPublicID,
		confirmed.Order.RowVersion, mustFundsULID(t),
	)
	if err != nil || requested.Refund.Status != RefundPending ||
		requested.Settlement.Status != SettlementRefundPending {
		t.Fatalf("wechat refund request=%#v err=%v", requested, err)
	}
	retry, err := funds.CompleteRefund(
		ctx, requested.Refund.OutRefundNo, "wx-refund-1", false, "provider timeout",
	)
	if err != nil || retry.Refund.Status != RefundRetryPending ||
		retry.Settlement.Status != SettlementRefundPending {
		t.Fatalf("wechat refund retry=%#v err=%v", retry, err)
	}
	succeeded, err := funds.CompleteRefund(
		ctx, requested.Refund.OutRefundNo, "wx-refund-1", true, "",
	)
	if err != nil || succeeded.Refund.Status != RefundSucceeded ||
		succeeded.Settlement.Status != SettlementRefunded {
		t.Fatalf("wechat refund success=%#v err=%v", succeeded, err)
	}
	repeated, err := funds.CompleteRefund(
		ctx, requested.Refund.OutRefundNo, "wx-refund-1", true, "",
	)
	if err != nil || repeated.Refund.Status != RefundSucceeded {
		t.Fatalf("idempotent refund callback=%#v err=%v", repeated, err)
	}
	assertD3Count(t, db, (&Refund{}).TableName(), "delivery_order_public_id = ?", 1, wechatOrder.DeliveryOrderPublicID)
	t.Log("refund_rules=PASS merchant_ready/admin_claimed/original_source/retry/idempotent_callback")
}

func TestZZFundsResidueAgainstRealMySQL84(t *testing.T) {
	db := openD3MySQL(t)
	cleanupD3(t, db)
	for _, table := range d3Tables() {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("D-3 residue %s=%d", table, count)
		}
	}
	var orders, events int64
	if err := db.Model(&Order{}).Where("merchant_user_id IN ?", []uint64{fundsMerchantOne, fundsMerchantTwo}).Count(&orders).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&OutboxEvent{}).Where("aggregate_public_id LIKE ?", "D3%").Count(&events).Error; err != nil {
		t.Fatal(err)
	}
	if orders != 0 || events != 0 {
		t.Fatalf("D-3 residue orders=%d events=%d", orders, events)
	}
	t.Log("D-3 residue tables=0 orders=0 scoped_outbox=0")
}

func openD3MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	db := openD2MySQL(t)
	ensureD3Schema(t, db)
	return db
}

func ensureD3Schema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if db.Migrator().HasTable(&MerchantWallet{}) {
		return
	}
	applyD1SQLFile(t, db, "../../../migrations/025_delivery_payment_wallet.sql")
}

func assertD3Schema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range d3Tables() {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("missing D-3 table %s", table)
		}
	}
	for table, index := range map[string]string{
		"delivery_merchant_wallets":      "uq_delivery_wallet_merchant",
		"delivery_wallet_ledger":         "uq_delivery_ledger_idempotency",
		"delivery_payment_attempts":      "uq_delivery_payment_client_request",
		"delivery_payment_notifications": "uq_delivery_notification_transaction",
		"delivery_fee_settlements":       "uq_delivery_settlement_order",
		"delivery_refunds":               "uq_delivery_refund_settlement",
	} {
		var count int64
		if err := db.Raw(`SELECT COUNT(*) FROM information_schema.statistics
			WHERE table_schema=DATABASE() AND table_name=? AND index_name=?`, table, index).
			Scan(&count).Error; err != nil || count == 0 {
			t.Fatalf("missing D-3 index %s.%s count=%d err=%v", table, index, count, err)
		}
	}
	var foreignKeys int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.key_column_usage
		WHERE table_schema=DATABASE() AND table_name IN ?
		AND referenced_table_name IS NOT NULL`, d3Tables()).Scan(&foreignKeys).Error; err != nil || foreignKeys != 0 {
		t.Fatalf("D-3 foreign keys=%d err=%v", foreignKeys, err)
	}
	for table, check := range map[string]string{
		"delivery_wallet_ledger":    "chk_delivery_ledger_balance",
		"delivery_payment_attempts": "chk_delivery_payment_reference",
		"delivery_fee_settlements":  "chk_delivery_settlement_status",
		"delivery_refunds":          "chk_delivery_refund_status",
	} {
		var count int64
		if err := db.Raw(`SELECT COUNT(*) FROM information_schema.table_constraints
			WHERE table_schema=DATABASE() AND table_name=? AND constraint_name=? AND constraint_type='CHECK'`,
			table, check).Scan(&count).Error; err != nil || count != 1 {
			t.Fatalf("missing D-3 check %s.%s count=%d err=%v", table, check, count, err)
		}
	}
}

func createFundsOrder(t *testing.T, ctx context.Context, service *DispatchService, merchant uint64) Order {
	t.Helper()
	order, err := service.CreateOrder(ctx, CreateDispatchInput{
		MerchantUserID: merchant, MerchantRequestID: mustFundsULID(t),
		Pickup:    Contact{Name: "D3 merchant", Phone: "13800008501", Address: "Campus shop"},
		Recipient: Contact{Name: "D3 recipient", Phone: "13800008502", Address: "Dormitory 3"},
		Remark:    "D-3 acceptance",
	})
	if err != nil {
		t.Fatal(err)
	}
	return order
}

func mustFundsULID(t *testing.T) string {
	t.Helper()
	value, err := NewULID()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func assertD3Count(t *testing.T, db *gorm.DB, table, where string, expected int64, args ...any) {
	t.Helper()
	var count int64
	if err := db.Table(table).Where(where, args...).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != expected {
		t.Fatalf("%s count=%d want=%d where=%s args=%v", table, count, expected, where, args)
	}
}

func cleanupD3(t *testing.T, db *gorm.DB) {
	t.Helper()
	if !db.Migrator().HasTable(&MerchantWallet{}) {
		return
	}
	merchants := []uint64{fundsMerchantOne, fundsMerchantTwo}
	var orderIDs []string
	if db.Migrator().HasTable(&Order{}) {
		if err := db.Model(&Order{}).Where("merchant_user_id IN ?", merchants).
			Pluck("delivery_order_public_id", &orderIDs).Error; err != nil {
			t.Errorf("list D-3 orders: %v", err)
		}
	}
	if len(orderIDs) > 0 && db.Migrator().HasTable(&OutboxEvent{}) {
		if err := db.Where("aggregate_public_id IN ?", orderIDs).Delete(&OutboxEvent{}).Error; err != nil {
			t.Errorf("cleanup D-3 outbox: %v", err)
		}
	}
	if err := db.Where("out_trade_no LIKE ?", "D3%").Delete(&PaymentNotification{}).Error; err != nil {
		t.Errorf("cleanup D-3 notifications: %v", err)
	}
	for _, value := range []any{
		&Refund{}, &FeeSettlement{}, &PaymentAttempt{}, &WalletLedger{}, &MerchantWallet{},
	} {
		if err := db.Where("merchant_user_id IN ?", merchants).Delete(value).Error; err != nil {
			t.Errorf("cleanup D-3 %T: %v", value, err)
		}
	}
	if len(orderIDs) > 0 {
		if err := db.Where("delivery_order_public_id IN ?", orderIDs).Delete(&Order{}).Error; err != nil {
			t.Errorf("cleanup D-3 orders: %v", err)
		}
	}
}

func d3Tables() []string {
	return []string{
		"delivery_merchant_wallets", "delivery_wallet_ledger", "delivery_payment_attempts",
		"delivery_payment_notifications", "delivery_fee_settlements", "delivery_refunds",
	}
}

func ExampleFundsService() {
	fmt.Println("delivery funds use integer cents and server-side fee snapshots")
	// Output: delivery funds use integer cents and server-side fee snapshots
}
