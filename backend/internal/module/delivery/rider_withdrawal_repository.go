package delivery

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RiderWithdrawalRepository interface {
	CreateRiderWithdrawal(context.Context, uint64, uint64, string, time.Time) (RiderWithdrawal, error)
	ListRiderWithdrawalsByRider(context.Context, uint64, Page) ([]RiderWithdrawal, int64, error)
	ListRiderWithdrawals(context.Context, string, uint64, Page) ([]RiderWithdrawal, int64, error)
	ApproveRiderWithdrawal(context.Context, string, uint64, uint64, string, string, time.Time) (RiderWithdrawal, error)
	RejectRiderWithdrawal(context.Context, string, uint64, uint64, string, string, time.Time) (RiderWithdrawal, error)
}

var _ RiderWithdrawalRepository = (*GormRepository)(nil)

func (r *GormRepository) FindRiderWithdrawal(ctx context.Context, withdrawPublicID string) (RiderWithdrawal, error) {
	var result RiderWithdrawal
	err := r.db.WithContext(ctx).Where("withdraw_public_id = ?", withdrawPublicID).First(&result).Error
	return result, normalizeNotFound(err)
}

func (r *GormRepository) CreateRiderWithdrawal(
	ctx context.Context,
	riderUserID, amountCents uint64,
	idempotencyKey string,
	now time.Time,
) (RiderWithdrawal, error) {
	if err := validateRiderWithdrawalInput(riderUserID, amountCents, idempotencyKey); err != nil {
		return RiderWithdrawal{}, err
	}
	var result RiderWithdrawal
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Wallet first serializes new requests and retries from the same rider.
		wallet, err := lockOrCreateRiderWalletTx(tx, riderUserID, now)
		if err != nil {
			return err
		}
		var existing RiderWithdrawal
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("idempotency_key = ?", idempotencyKey).First(&existing).Error
		if err == nil {
			if err := withdrawalReplay(existing, riderUserID, amountCents); err != nil {
				return err
			}
			result = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// Reservations are not subtracted from the stored wallet balance. Never
		// allow the new request to spend funds held by the existing settlement queue.
		reserved, err := activeRiderPayoutReservationAmountTx(tx, riderUserID)
		if err != nil {
			return err
		}
		if reserved > wallet.AvailableBalanceCents || amountCents > wallet.AvailableBalanceCents-reserved {
			return ErrWalletInsufficient
		}
		withdrawID, err := NewULID()
		if err != nil {
			return err
		}
		ledger, err := newRiderLedger(wallet, RiderLedgerDebit, RiderLedgerPayout, amountCents,
			"rider_withdrawal", withdrawID, idempotencyKey, now)
		if err != nil {
			return err
		}
		if err := debitRiderWalletTx(tx, &wallet, amountCents, now); err != nil {
			return err
		}
		result = RiderWithdrawal{
			WithdrawPublicID: withdrawID,
			RiderUserID:      riderUserID,
			WalletPublicID:   wallet.WalletPublicID,
			AmountCents:      amountCents,
			Currency:         "CNY",
			Status:           RiderWithdrawalProcessing,
			Reason:           "",
			IdempotencyKey:   idempotencyKey,
			RowVersion:       1,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err := tx.Create(&result).Error; err != nil {
			if isDuplicateKey(err) {
				return ErrIdempotencyConflict
			}
			return err
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		return appendFinancialNotificationTx(tx, r.notificationAppender, EventRiderWithdrawalChanged, result.WithdrawPublicID, result.RowVersion,
			RiderWithdrawalNotificationPayload{WithdrawPublicID: result.WithdrawPublicID, RiderUserID: result.RiderUserID,
				AmountCents: result.AmountCents, Status: string(result.Status)}, now)
	})
	return result, err
}

func (r *GormRepository) ListRiderWithdrawalsByRider(ctx context.Context, riderUserID uint64, page Page) ([]RiderWithdrawal, int64, error) {
	if riderUserID == 0 || !page.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	return listRiderWithdrawals(r.db.WithContext(ctx).Where("rider_user_id = ?", riderUserID), page)
}

func (r *GormRepository) ListRiderWithdrawals(ctx context.Context, status string, riderUserID uint64, page Page) ([]RiderWithdrawal, int64, error) {
	if !page.Valid() || (status != "" && !RiderWithdrawalStatus(status).Valid()) {
		return nil, 0, ErrInvalidRecord
	}
	db := r.db.WithContext(ctx)
	if strings.TrimSpace(status) != "" {
		db = db.Where("status = ?", status)
	}
	if riderUserID != 0 {
		db = db.Where("rider_user_id = ?", riderUserID)
	}
	return listRiderWithdrawals(db, page)
}

func listRiderWithdrawals(db *gorm.DB, page Page) ([]RiderWithdrawal, int64, error) {
	var total int64
	if err := db.Model(&RiderWithdrawal{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]RiderWithdrawal, 0)
	err := db.Order("created_at DESC, id DESC").Offset((page.Page - 1) * page.Size).Limit(page.Size).Find(&items).Error
	return items, total, err
}

func (r *GormRepository) ApproveRiderWithdrawal(
	ctx context.Context,
	withdrawPublicID string,
	expectedRowVersion, operatorUserID uint64,
	idempotencyKey, paymentReference string,
	now time.Time,
) (RiderWithdrawal, error) {
	if !ValidULID(withdrawPublicID) || expectedRowVersion == 0 || operatorUserID == 0 || !ValidULID(idempotencyKey) {
		return RiderWithdrawal{}, ErrInvalidRecord
	}
	paymentReference = strings.TrimSpace(paymentReference)
	if paymentReference == "" {
		return RiderWithdrawal{}, ErrWithdrawalPaymentRequired
	}
	if len([]rune(paymentReference)) > 128 {
		return RiderWithdrawal{}, ErrInvalidRecord
	}
	var result RiderWithdrawal
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("withdraw_public_id = ?", withdrawPublicID).First(&result).Error; err != nil {
			return normalizeWithdrawalNotFound(err)
		}
		if replay, err := withdrawalDecision(result, RiderWithdrawalPaid, expectedRowVersion, operatorUserID, idempotencyKey, "", paymentReference); err != nil || replay {
			return err
		}
		processedAt := now
		updates := map[string]any{
			"status": RiderWithdrawalPaid, "processed_by": operatorUserID,
			"process_key": idempotencyKey, "process_version": expectedRowVersion, "payment_reference": paymentReference,
			"processed_at": processedAt, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		}
		if update := tx.Model(&RiderWithdrawal{}).Where("id = ? AND row_version = ?", result.ID, expectedRowVersion).Updates(updates); update.Error != nil {
			if isDuplicateKey(update.Error) {
				return ErrIdempotencyConflict
			}
			return update.Error
		} else if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		result.Status = RiderWithdrawalPaid
		result.ProcessedBy = &operatorUserID
		result.ProcessKey = &idempotencyKey
		result.ProcessVersion = &expectedRowVersion
		result.PaymentReference = paymentReference
		result.ProcessedAt = &processedAt
		result.RowVersion++
		result.UpdatedAt = now
		return appendFinancialNotificationTx(tx, r.notificationAppender, EventRiderWithdrawalChanged, result.WithdrawPublicID, result.RowVersion,
			RiderWithdrawalNotificationPayload{WithdrawPublicID: result.WithdrawPublicID, RiderUserID: result.RiderUserID,
				AmountCents: result.AmountCents, Status: string(result.Status)}, now)
	})
	return result, err
}

func (r *GormRepository) RejectRiderWithdrawal(
	ctx context.Context,
	withdrawPublicID string,
	expectedRowVersion, operatorUserID uint64,
	idempotencyKey, reason string,
	now time.Time,
) (RiderWithdrawal, error) {
	if !ValidULID(withdrawPublicID) || expectedRowVersion == 0 || operatorUserID == 0 || !ValidULID(idempotencyKey) {
		return RiderWithdrawal{}, ErrInvalidRecord
	}
	if err := validateWithdrawalReason(reason); err != nil {
		return RiderWithdrawal{}, err
	}
	reason = strings.TrimSpace(reason)
	var candidate RiderWithdrawal
	if err := r.db.WithContext(ctx).Where("withdraw_public_id = ?", withdrawPublicID).First(&candidate).Error; err != nil {
		return RiderWithdrawal{}, normalizeWithdrawalNotFound(err)
	}
	var result RiderWithdrawal
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Keep the shared wallet -> withdrawal lock order used by income settlement
		// so rejection cannot deadlock against a concurrent withdrawal request.
		wallet, err := lockOrCreateRiderWalletTx(tx, candidate.RiderUserID, now)
		if err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("withdraw_public_id = ?", withdrawPublicID).First(&result).Error; err != nil {
			return normalizeWithdrawalNotFound(err)
		}
		if result.RiderUserID != candidate.RiderUserID {
			return ErrConcurrentUpdate
		}
		if replay, err := withdrawalDecision(result, RiderWithdrawalRejected, expectedRowVersion, operatorUserID, idempotencyKey, reason, ""); err != nil || replay {
			return err
		}
		ledger, err := newRiderLedger(wallet, RiderLedgerCredit, RiderLedgerAdjustment, result.AmountCents,
			"rider_withdrawal", result.WithdrawPublicID, idempotencyKey, now)
		if err != nil {
			return err
		}
		if err := creditRiderWalletTx(tx, &wallet, result.AmountCents, now); err != nil {
			return err
		}
		if err := tx.Create(&ledger).Error; err != nil {
			if isDuplicateKey(err) {
				return ErrIdempotencyConflict
			}
			return err
		}
		processedAt := now
		if update := tx.Model(&RiderWithdrawal{}).Where("id = ? AND row_version = ?", result.ID, expectedRowVersion).Updates(map[string]any{
			"status": RiderWithdrawalRejected, "reason": strings.TrimSpace(reason), "processed_by": operatorUserID,
			"process_key": idempotencyKey, "process_version": expectedRowVersion,
			"processed_at": processedAt, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		}); update.Error != nil {
			if isDuplicateKey(update.Error) {
				return ErrIdempotencyConflict
			}
			return update.Error
		} else if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		result.Status = RiderWithdrawalRejected
		result.Reason = strings.TrimSpace(reason)
		result.ProcessedBy = &operatorUserID
		result.ProcessKey = &idempotencyKey
		result.ProcessVersion = &expectedRowVersion
		result.ProcessedAt = &processedAt
		result.RowVersion++
		result.UpdatedAt = now
		return appendFinancialNotificationTx(tx, r.notificationAppender, EventRiderWithdrawalChanged, result.WithdrawPublicID, result.RowVersion,
			RiderWithdrawalNotificationPayload{WithdrawPublicID: result.WithdrawPublicID, RiderUserID: result.RiderUserID,
				AmountCents: result.AmountCents, Status: string(result.Status)}, now)
	})
	return result, err
}

func normalizeWithdrawalNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrWithdrawalNotFound
	}
	return err
}
