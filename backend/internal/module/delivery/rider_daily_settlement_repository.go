package delivery

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RiderDailySettlementRepository contains only internal T+1 settlement
// operations. It never calls a payment provider.
type RiderDailySettlementRepository interface {
	ListDueRiderEarnings(context.Context, time.Time, uint64, uint64) ([]RiderEarningRecord, error)
	SettleRiderEarning(context.Context, string, time.Time) (RiderEarningRecord, RiderLedger, error)
}

var _ RiderDailySettlementRepository = (*GormRepository)(nil)

func (r *GormRepository) ListDueRiderEarnings(ctx context.Context, now time.Time, afterID, limit uint64) ([]RiderEarningRecord, error) {
	if now.IsZero() {
		return nil, ErrInvalidRecord
	}
	if limit == 0 || limit > 500 {
		limit = 100
	}
	var values []RiderEarningRecord
	err := r.db.WithContext(ctx).Where("settlement_status = ? AND settle_at <= ? AND id > ? AND rider_fee_cents > 0", RiderEarningPending, now.UTC(), afterID).
		Order("id ASC").Limit(int(limit)).Find(&values).Error
	return values, err
}

func (r *GormRepository) SettleRiderEarning(ctx context.Context, earningPublicID string, now time.Time) (RiderEarningRecord, RiderLedger, error) {
	if !ValidULID(earningPublicID) || now.IsZero() {
		return RiderEarningRecord{}, RiderLedger{}, ErrInvalidRecord
	}
	now = now.UTC()
	var result RiderEarningRecord
	var ledger RiderLedger
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var candidate RiderEarningRecord
		if err := tx.Where("earning_public_id = ?", earningPublicID).First(&candidate).Error; err != nil {
			return normalizeNotFound(err)
		}
		wallet, err := lockOrCreateRiderWalletTx(tx, candidate.RiderUserID, now)
		if err != nil {
			return err
		}
		var earning RiderEarningRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("earning_public_id = ?", earningPublicID).First(&earning).Error; err != nil {
			return normalizeNotFound(err)
		}
		if earning.RiderUserID != candidate.RiderUserID {
			return ErrFundingSourceMismatch
		}
		if earning.SettlementStatus == RiderEarningSettled || earning.SettlementStatus == RiderEarningPaidOffline {
			result = earning
			if err := tx.Where("idempotency_key = ?", earning.DeliveryOrderPublicID).First(&ledger).Error; err != nil {
				return normalizeNotFound(err)
			}
			if ledger.WalletPublicID != wallet.WalletPublicID || ledger.RiderUserID != earning.RiderUserID ||
				ledger.AmountCents != earning.RiderFeeCents || ledger.Direction != RiderLedgerCredit ||
				ledger.EntryType != RiderLedgerIncome || ledger.ReferenceType != "delivery_order" ||
				ledger.ReferencePublicID != earning.DeliveryOrderPublicID {
				return ErrFundingSourceMismatch
			}
			return nil
		}
		if earning.SettlementStatus != RiderEarningPending || earning.SettleAt.IsZero() || earning.SettleAt.After(now) {
			return ErrRiderEarningNotDue
		}
		// Reuse verified funding facts: mock fees, missing payment evidence and
		// refunds must never become withdrawable income merely because time passed.
		if _, err := verifyRiderEarningFundingTx(tx, earning); err != nil {
			return err
		}
		newLedger, err := newRiderLedger(wallet, RiderLedgerCredit, RiderLedgerIncome, earning.RiderFeeCents,
			"delivery_order", earning.DeliveryOrderPublicID, earning.DeliveryOrderPublicID, now)
		if err != nil {
			return err
		}
		if err := creditRiderWalletTx(tx, &wallet, earning.RiderFeeCents, now); err != nil {
			return err
		}
		if err := tx.Create(&newLedger).Error; err != nil {
			if isDuplicateKey(err) {
				return ErrIdempotencyConflict
			}
			return err
		}
		settledAt := now
		update := tx.Model(&RiderEarningRecord{}).Where("id = ? AND row_version = ? AND settlement_status = ? AND settle_at <= ?",
			earning.ID, earning.RowVersion, RiderEarningPending, now).Updates(map[string]any{
			"settlement_status": RiderEarningSettled,
			"settled_at":        settledAt,
			"row_version":       gorm.Expr("row_version + 1"),
			"updated_at":        now,
		})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		earning.SettlementStatus = RiderEarningSettled
		earning.SettledAt = &settledAt
		earning.RowVersion++
		earning.UpdatedAt = now
		if err := appendFinancialNotificationTx(tx, r.notificationAppender, EventRiderIncomeRecorded, earning.EarningPublicID, earning.RowVersion,
			RiderIncomeNotificationPayload{EarningPublicID: earning.EarningPublicID, OrderPublicID: earning.DeliveryOrderPublicID,
				RiderUserID: earning.RiderUserID, AmountCents: earning.RiderFeeCents, SettlementStatus: string(earning.SettlementStatus)}, now); err != nil {
			return err
		}
		result, ledger = earning, newLedger
		return nil
	})
	return result, ledger, err
}

var ErrRiderEarningNotDue = errors.New("rider earning is not due for settlement")
