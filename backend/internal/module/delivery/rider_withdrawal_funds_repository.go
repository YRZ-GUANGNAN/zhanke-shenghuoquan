package delivery

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type withdrawalCandidate struct {
	earning RiderEarningRecord
	amount  uint64
	source  RiderPayoutSourceSnapshot
}
type withdrawalFunds struct {
	available  uint64
	candidates []withdrawalCandidate
}

func moneyRead(db *gorm.DB, lock bool) *gorm.DB {
	if lock {
		return db.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	return db
}

// The caller owns the wallet lock for commands. Queries use a read-only,
// repeatable-read snapshot and never create money facts.
func readWithdrawalFundsTx(tx *gorm.DB, wallet RiderWallet, lock bool, now time.Time) (withdrawalFunds, error) {
	var result withdrawalFunds
	earnings, allocations, err := withdrawalBookTx(tx, wallet, lock)
	if err != nil {
		return result, err
	}
	var reservations []RiderPayoutReservation
	if err := moneyRead(tx, lock).Where("rider_user_id = ? AND status = ?", wallet.RiderUserID, RiderPayoutReservationActive).
		Order("id ASC").Find(&reservations).Error; err != nil {
		return result, err
	}
	reservedByEarning := map[string]bool{}
	var reserved uint64
	for _, reservation := range reservations {
		var queue RiderPayoutQueue
		if err := moneyRead(tx, lock).Where("payout_queue_public_id = ?", reservation.PayoutQueuePublicID).First(&queue).Error; err != nil {
			return result, err
		}
		if queue.Status != RiderPayoutQueueQueued || queue.RiderUserID != wallet.RiderUserID || queue.EarningPublicID != reservation.EarningPublicID ||
			queue.AmountCents != reservation.AmountCents || reservation.AmountCents == 0 || reservation.WalletPublicID != wallet.WalletPublicID || reservedByEarning[reservation.EarningPublicID] {
			return result, ErrWithdrawalSourceMismatch
		}
		found := false
		for _, e := range earnings {
			if e.EarningPublicID == reservation.EarningPublicID && e.RiderFeeCents == reservation.AmountCents && e.DeliveryOrderPublicID == queue.DeliveryOrderPublicID {
				found = true
				break
			}
		}
		if !found {
			return result, ErrWithdrawalSourceMismatch
		}
		reservedByEarning[reservation.EarningPublicID] = true
		reserved, err = addRiderMoney(reserved, reservation.AmountCents)
		if err != nil {
			return result, err
		}
	}
	if wallet.AvailableBalanceCents > MaxRiderMoneyAmount || reserved > wallet.AvailableBalanceCents {
		return result, ErrWithdrawalSourceMismatch
	}
	used := map[string]uint64{}
	for _, allocation := range allocations {
		if allocation.Status == RiderWithdrawalAllocationReleased {
			continue
		}
		used[allocation.EarningPublicID], err = addRiderMoney(used[allocation.EarningPublicID], allocation.AmountCents)
		if err != nil {
			return result, err
		}
		if reservedByEarning[allocation.EarningPublicID] {
			return result, ErrWithdrawalSourceMismatch
		}
	}
	payable := make([]RiderEarningRecord, 0)
	for _, earning := range earnings {
		if used[earning.EarningPublicID] > earning.RiderFeeCents {
			return result, ErrWithdrawalSourceMismatch
		}
		if earning.SettlementStatus != RiderEarningSettled || earning.PaidOfflineAt != nil ||
			earning.SettledAt == nil || earning.SettledAt.IsZero() || earning.SettleAt.IsZero() || earning.SettleAt.After(now) ||
			earning.SettledAt.After(now) || reservedByEarning[earning.EarningPublicID] {
			continue
		}
		if used[earning.EarningPublicID] == earning.RiderFeeCents {
			continue
		}
		payable = append(payable, earning)
	}
	if lock {
		if err := lockWithdrawalSourcesTx(tx, payable); err != nil {
			return result, err
		}
	}
	var verified uint64
	for _, earning := range payable {
		source, sourceErr := verifiedWithdrawalEarningTx(tx, wallet, earning)
		if sourceErr != nil {
			if isRiderPayoutSourceBlockError(sourceErr) || errors.Is(sourceErr, ErrWithdrawalSourceMismatch) {
				continue
			}
			return result, sourceErr
		}
		amount := earning.RiderFeeCents - used[earning.EarningPublicID]
		verified, err = addRiderMoney(verified, amount)
		if err != nil {
			return result, err
		}
		result.candidates = append(result.candidates, withdrawalCandidate{earning, amount, source})
	}
	result.available = wallet.AvailableBalanceCents - reserved
	if result.available > verified {
		result.available = verified
	}
	return result, nil
}

func withdrawalBookTx(tx *gorm.DB, wallet RiderWallet, lock bool) ([]RiderEarningRecord, []RiderWithdrawalAllocation, error) {
	var earnings []RiderEarningRecord
	if err := moneyRead(tx, lock).Where("rider_user_id = ?", wallet.RiderUserID).Order("id ASC").Find(&earnings).Error; err != nil {
		return nil, nil, err
	}
	var allocations []RiderWithdrawalAllocation
	if err := moneyRead(tx, lock).Where("rider_user_id = ?", wallet.RiderUserID).Order("id ASC").Find(&allocations).Error; err != nil {
		return nil, nil, err
	}
	var withdrawals []RiderWithdrawal
	if err := moneyRead(tx, lock).Where("rider_user_id = ?", wallet.RiderUserID).Order("id ASC").Find(&withdrawals).Error; err != nil {
		return nil, nil, err
	}
	byEarning := map[string]RiderEarningRecord{}
	byWithdrawal := map[string]RiderWithdrawal{}
	withdrawalIDs := make([]string, 0, len(withdrawals))
	for _, earning := range earnings {
		byEarning[earning.EarningPublicID] = earning
	}
	for _, withdrawal := range withdrawals {
		byWithdrawal[withdrawal.WithdrawPublicID] = withdrawal
		withdrawalIDs = append(withdrawalIDs, withdrawal.WithdrawPublicID)
	}
	var paymentReferences []RiderWithdrawalPaymentReference
	if len(withdrawalIDs) != 0 {
		// These bindings are immutable and written under the same wallet lock.
		// Never lock all riders' receipts while holding just this rider's wallet.
		if err := tx.Where("withdraw_public_id IN ?", withdrawalIDs).Order("id ASC").Find(&paymentReferences).Error; err != nil {
			return nil, nil, err
		}
	}
	byPaymentWithdrawal := make(map[string]RiderWithdrawalPaymentReference, len(paymentReferences))
	byPaymentValue := make(map[string]RiderWithdrawalPaymentReference, len(paymentReferences))
	for _, reference := range paymentReferences {
		if !actualWithdrawalPaymentReference(reference.PaymentReference) || reference.WithdrawPublicID == "" {
			return nil, nil, ErrWithdrawalSourceMismatch
		}
		if _, exists := byPaymentWithdrawal[reference.WithdrawPublicID]; exists {
			return nil, nil, ErrWithdrawalSourceMismatch
		}
		if _, exists := byPaymentValue[reference.PaymentReference]; exists {
			return nil, nil, ErrWithdrawalSourceMismatch
		}
		byPaymentWithdrawal[reference.WithdrawPublicID] = reference
		byPaymentValue[reference.PaymentReference] = reference
	}
	totals := map[string]uint64{}
	used := map[string]uint64{}
	for _, a := range allocations {
		e, exists := byEarning[a.EarningPublicID]
		w, found := byWithdrawal[a.WithdrawPublicID]
		if !exists || !found || a.AmountCents == 0 || a.Currency != "CNY" || a.WalletPublicID != wallet.WalletPublicID ||
			a.DeliveryOrderPublicID != e.DeliveryOrderPublicID || e.RiderUserID != w.RiderUserID || len(a.SourceEvidenceHash) != 64 || a.SourcePolicyVersion == "" ||
			(w.Status == RiderWithdrawalProcessing && a.Status != RiderWithdrawalAllocationInFlight) ||
			(w.Status == RiderWithdrawalPaid && a.Status != RiderWithdrawalAllocationPaid) ||
			(w.Status == RiderWithdrawalRejected && a.Status != RiderWithdrawalAllocationReleased) || !a.Status.Valid() {
			return nil, nil, ErrWithdrawalSourceMismatch
		}
		var err error
		totals[a.WithdrawPublicID], err = addRiderMoney(totals[a.WithdrawPublicID], a.AmountCents)
		if err != nil {
			return nil, nil, err
		}
		if a.Status != RiderWithdrawalAllocationReleased {
			used[a.EarningPublicID], err = addRiderMoney(used[a.EarningPublicID], a.AmountCents)
			if err != nil {
				return nil, nil, err
			}
			if used[a.EarningPublicID] > e.RiderFeeCents || e.SettlementStatus != RiderEarningSettled || e.PaidOfflineAt != nil {
				return nil, nil, ErrWithdrawalSourceMismatch
			}
		}
	}
	for _, w := range withdrawals {
		if w.AmountCents == 0 || w.AmountCents != totals[w.WithdrawPublicID] || w.WalletPublicID != wallet.WalletPublicID ||
			w.Currency != "CNY" || !w.Status.Valid() {
			return nil, nil, ErrWithdrawalSourceMismatch
		}
		var ledgers []RiderLedger
		// Read every ledger row with this public reference, then validate the
		// type explicitly.  Filtering by type in SQL would hide a forged or
		// cross-domain row that reuses the withdrawal id.
		if err := tx.Where("reference_public_id = ?", w.WithdrawPublicID).Find(&ledgers).Error; err != nil {
			return nil, nil, err
		}
		debits, credits := 0, 0
		for _, l := range ledgers {
			if l.ReferenceType != "rider_withdrawal" || l.ReferencePublicID != w.WithdrawPublicID ||
				l.RiderUserID != wallet.RiderUserID || l.WalletPublicID != wallet.WalletPublicID || l.AmountCents != w.AmountCents {
				return nil, nil, ErrWithdrawalSourceMismatch
			}
			switch {
			case l.Direction == RiderLedgerDebit && l.EntryType == RiderLedgerPayout && l.IdempotencyKey == w.IdempotencyKey &&
				l.BalanceBeforeCents >= l.AmountCents && l.BalanceBeforeCents-l.AmountCents == l.BalanceAfterCents:
				debits++
			case l.Direction == RiderLedgerCredit && l.EntryType == RiderLedgerAdjustment && w.ProcessKey != nil && l.IdempotencyKey == *w.ProcessKey &&
				l.BalanceAfterCents >= l.AmountCents && l.BalanceAfterCents-l.AmountCents == l.BalanceBeforeCents:
				credits++
			default:
				return nil, nil, ErrWithdrawalSourceMismatch
			}
		}
		if debits != 1 || (w.Status == RiderWithdrawalRejected && credits != 1) || (w.Status != RiderWithdrawalRejected && credits != 0) {
			return nil, nil, ErrWithdrawalSourceMismatch
		}
		payment, hasPayment := byPaymentWithdrawal[w.WithdrawPublicID]
		var claim RiderWithdrawalClaim
		claimErr := tx.Where("withdraw_public_id = ?", w.WithdrawPublicID).First(&claim).Error
		if claimErr != nil && !errors.Is(claimErr, gorm.ErrRecordNotFound) {
			return nil, nil, claimErr
		}
		switch {
		case w.Status == RiderWithdrawalPaid:
			if !hasPayment || payment.PaymentReference != w.PaymentReference || !actualWithdrawalPaymentReference(w.PaymentReference) {
				return nil, nil, ErrWithdrawalSourceMismatch
			}
		case w.Status == RiderWithdrawalRejected:
			if hasPayment || w.PaymentReference != "" {
				return nil, nil, ErrWithdrawalSourceMismatch
			}
		case claimErr == nil && actualWithdrawalPaymentReference(claim.PaymentReference):
			if !hasPayment || payment.PaymentReference != claim.PaymentReference || w.PaymentReference != "" {
				return nil, nil, ErrWithdrawalSourceMismatch
			}
		default:
			if hasPayment || w.PaymentReference != "" {
				return nil, nil, ErrWithdrawalSourceMismatch
			}
		}
		if err := checkWithdrawalAuditTx(tx, w); err != nil {
			return nil, nil, err
		}
	}
	return earnings, allocations, nil
}

func (r *GormRepository) RequireWithdrawalSchema() error {
	if r == nil || r.db == nil {
		return ErrWithdrawalAllocationUnavailable
	}
	for _, table := range []any{&RiderWithdrawalAllocation{}, &RiderWithdrawalClaim{}, &RiderWithdrawalEvent{}, &RiderWithdrawal{}, &RiderWithdrawalPaymentReference{}, &RiderPayoutReservation{}, &RiderPayoutQueue{}} {
		if !r.db.Migrator().HasTable(table) {
			return ErrWithdrawalAllocationUnavailable
		}
		statement := &gorm.Statement{DB: r.db}
		if err := statement.Parse(table); err != nil {
			return ErrWithdrawalAllocationUnavailable
		}
		columns, err := r.db.Migrator().ColumnTypes(table)
		if err != nil {
			return ErrWithdrawalAllocationUnavailable
		}
		present := map[string]bool{}
		for _, column := range columns {
			present[column.Name()] = true
		}
		for _, name := range statement.Schema.DBNames {
			if !present[name] {
				return ErrWithdrawalAllocationUnavailable
			}
		}
	}
	// GORM's HasIndex only tells us that an index with this name exists.  A
	// deployment with a downgraded/non-unique index would therefore pass the
	// old gate and could double-claim funds.  Read MySQL's catalog and require
	// exact uniqueness and column order for every money-critical key.
	if strings.ToLower(r.db.Dialector.Name()) != "mysql" {
		return ErrWithdrawalAllocationUnavailable
	}
	type catalogIndexColumn struct {
		TableName string `gorm:"column:table_name"`
		IndexName string `gorm:"column:index_name"`
		NonUnique int    `gorm:"column:non_unique"`
		Seq       int    `gorm:"column:seq_in_index"`
		Column    string `gorm:"column:column_name"`
	}
	tables := []string{
		"delivery_rider_withdrawals",
		"delivery_rider_withdrawal_allocations",
		"delivery_rider_withdrawal_events",
		"delivery_rider_withdrawal_claims",
		"delivery_rider_withdrawal_payment_references",
		"delivery_rider_payout_queue",
		"delivery_rider_payout_reservations",
		"delivery_rider_payout_events",
	}
	var catalog []catalogIndexColumn
	if err := r.db.Raw(`
		SELECT TABLE_NAME AS table_name, INDEX_NAME AS index_name,
		       NON_UNIQUE AS non_unique, SEQ_IN_INDEX AS seq_in_index,
		       COLUMN_NAME AS column_name
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME IN ?
		ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`, tables).Scan(&catalog).Error; err != nil {
		return ErrWithdrawalAllocationUnavailable
	}
	type expectedIndex struct {
		table   string
		name    string
		columns []string
	}
	expected := []expectedIndex{
		{"delivery_rider_withdrawals", "uq_delivery_rider_withdrawal_public_id", []string{"withdraw_public_id"}},
		{"delivery_rider_withdrawals", "uq_delivery_rider_withdrawal_idempotency", []string{"idempotency_key"}},
		{"delivery_rider_withdrawals", "uq_delivery_rider_withdrawal_process_key", []string{"process_key"}},
		{"delivery_rider_withdrawal_allocations", "uq_delivery_rider_withdrawal_allocation_public_id", []string{"allocation_public_id"}},
		{"delivery_rider_withdrawal_allocations", "uq_delivery_rider_withdrawal_allocation_pair", []string{"withdraw_public_id", "earning_public_id"}},
		{"delivery_rider_withdrawal_events", "uq_delivery_rider_withdrawal_event_public_id", []string{"event_public_id"}},
		{"delivery_rider_withdrawal_events", "uq_delivery_rider_withdrawal_event_idempotency", []string{"idempotency_key"}},
		{"delivery_rider_withdrawal_claims", "uq_delivery_rider_withdrawal_claim_public_id", []string{"claim_public_id"}},
		{"delivery_rider_withdrawal_claims", "uq_delivery_rider_withdrawal_claim_withdraw", []string{"withdraw_public_id"}},
		{"delivery_rider_withdrawal_claims", "uq_delivery_rider_withdrawal_claim_key", []string{"claim_key"}},
		{"delivery_rider_withdrawal_claims", "uq_delivery_rider_withdrawal_claim_idempotency", []string{"idempotency_key"}},
		{"delivery_rider_withdrawal_payment_references", "uq_delivery_rider_withdrawal_payment_reference", []string{"payment_reference"}},
		{"delivery_rider_withdrawal_payment_references", "uq_delivery_rider_withdrawal_payment_withdraw", []string{"withdraw_public_id"}},
		{"delivery_rider_payout_queue", "uq_delivery_rider_payout_queue_public_id", []string{"payout_queue_public_id"}},
		{"delivery_rider_payout_queue", "uq_delivery_rider_payout_queue_earning", []string{"earning_public_id"}},
		{"delivery_rider_payout_reservations", "uq_delivery_rider_payout_reservation_public_id", []string{"reservation_public_id"}},
		{"delivery_rider_payout_reservations", "uq_delivery_rider_payout_reservation_queue", []string{"payout_queue_public_id"}},
		{"delivery_rider_payout_reservations", "uq_delivery_rider_payout_reservation_earning", []string{"earning_public_id"}},
		{"delivery_rider_payout_events", "uq_delivery_rider_payout_event_public_id", []string{"event_public_id"}},
		{"delivery_rider_payout_events", "uq_delivery_rider_payout_event_idempotency", []string{"idempotency_key"}},
	}
	byIndex := make(map[string][]catalogIndexColumn)
	for _, row := range catalog {
		key := strings.ToLower(row.TableName) + "\x00" + strings.ToLower(row.IndexName)
		byIndex[key] = append(byIndex[key], row)
	}
	for _, want := range expected {
		rows := byIndex[strings.ToLower(want.table)+"\x00"+strings.ToLower(want.name)]
		if len(rows) != len(want.columns) {
			return ErrWithdrawalAllocationUnavailable
		}
		for i, row := range rows {
			if row.NonUnique != 0 || row.Seq != i+1 || !strings.EqualFold(row.Column, want.columns[i]) {
				return ErrWithdrawalAllocationUnavailable
			}
		}
	}
	// This non-unique index is needed by the source reconciliation query, but
	// uniqueness is intentionally not required for it.
	if !r.db.Migrator().HasIndex(&RiderWithdrawalAllocation{}, "idx_delivery_rider_withdrawal_allocation_earning_status") {
		return ErrWithdrawalAllocationUnavailable
	}
	return nil
}
func (r *GormRepository) ReadRiderWithdrawable(ctx context.Context, riderID uint64) (uint64, error) {
	if riderID == 0 {
		return 0, ErrInvalidRecord
	}
	if err := r.RequireWithdrawalSchema(); err != nil {
		return 0, err
	}
	var amount uint64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var wallet RiderWallet
		if err := tx.Where("rider_user_id = ?", riderID).First(&wallet).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				var count int64
				if err := tx.Model(&RiderEarningRecord{}).Where("rider_user_id = ?", riderID).Count(&count).Error; err != nil {
					return err
				}
				if count != 0 {
					return ErrWithdrawalSourceMismatch
				}
				return nil
			}
			return err
		}
		funds, err := readWithdrawalFundsTx(tx, wallet, false, time.Now().UTC())
		amount = funds.available
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return amount, err
}
func (r *GormRepository) CheckWithdrawalReadiness(ctx context.Context) error {
	if err := r.RequireWithdrawalSchema(); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var orphan int64
		if err := tx.Raw("SELECT COUNT(*) FROM delivery_rider_withdrawals w LEFT JOIN delivery_rider_wallets b ON b.rider_user_id=w.rider_user_id WHERE b.id IS NULL OR b.wallet_public_id<>w.wallet_public_id").Scan(&orphan).Error; err != nil {
			return err
		}
		if orphan != 0 {
			return ErrWithdrawalSourceMismatch
		}
		for _, query := range []string{
			"SELECT COUNT(*) FROM delivery_rider_withdrawal_claims c LEFT JOIN delivery_rider_withdrawals w ON w.withdraw_public_id=c.withdraw_public_id WHERE w.id IS NULL",
			"SELECT COUNT(*) FROM delivery_rider_withdrawal_events e LEFT JOIN delivery_rider_withdrawals w ON w.withdraw_public_id=e.withdraw_public_id WHERE w.id IS NULL",
			"SELECT COUNT(*) FROM delivery_rider_earning_records e LEFT JOIN delivery_rider_wallets w ON w.rider_user_id=e.rider_user_id WHERE w.id IS NULL",
		} {
			if err := tx.Raw(query).Scan(&orphan).Error; err != nil {
				return err
			}
			if orphan != 0 {
				return ErrWithdrawalSourceMismatch
			}
		}
		if err := tx.Raw("SELECT COUNT(*) FROM delivery_rider_withdrawal_allocations a LEFT JOIN delivery_rider_withdrawals w ON w.withdraw_public_id=a.withdraw_public_id WHERE w.id IS NULL OR w.rider_user_id<>a.rider_user_id").Scan(&orphan).Error; err != nil {
			return err
		}
		if orphan != 0 {
			return ErrWithdrawalSourceMismatch
		}
		if err := tx.Raw("SELECT COUNT(*) FROM delivery_rider_withdrawal_payment_references p LEFT JOIN delivery_rider_withdrawals w ON w.withdraw_public_id=p.withdraw_public_id WHERE w.id IS NULL OR w.status = 'rejected' OR (w.status = 'paid' AND w.payment_reference <> p.payment_reference)").Scan(&orphan).Error; err != nil {
			return err
		}
		if orphan != 0 {
			return ErrWithdrawalSourceMismatch
		}
		var wallets []RiderWallet
		if err := tx.Order("id ASC").Find(&wallets).Error; err != nil {
			return err
		}
		for _, wallet := range wallets {
			if _, err := readWithdrawalFundsTx(tx, wallet, false, time.Now().UTC()); err != nil {
				return err
			}
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
}

// Old per-order payouts and internal queue creation must share the persisted
// earning exclusion even when their UI is disabled.
func hasWithdrawalAllocationTx(tx *gorm.DB, earningID string) (bool, error) {
	if !tx.Migrator().HasTable(&RiderWithdrawalAllocation{}) {
		return false, nil
	}
	var rows []RiderWithdrawalAllocation
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("earning_public_id = ? AND status IN ?", earningID,
		[]RiderWithdrawalAllocationStatus{RiderWithdrawalAllocationInFlight, RiderWithdrawalAllocationPaid}).Find(&rows).Error; err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}
