package delivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
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

type RiderWithdrawalProcessingRepository interface {
	RiderWithdrawalRepository
	FindRiderWithdrawal(context.Context, string) (RiderWithdrawal, error)
	ProcessRiderWithdrawal(context.Context, string, RiderWithdrawalAction, time.Time) (RiderWithdrawal, error)
	ReadRiderWithdrawalClaim(context.Context, string) (RiderWithdrawalClaim, error)
	ReadRiderWithdrawable(context.Context, uint64) (uint64, error)
}

var _ RiderWithdrawalProcessingRepository = (*GormRepository)(nil)

func (r *GormRepository) FindRiderWithdrawal(ctx context.Context, id string) (RiderWithdrawal, error) {
	var result RiderWithdrawal
	err := r.db.WithContext(ctx).Where("withdraw_public_id = ?", id).First(&result).Error
	return result, normalizeNotFound(err)
}

func (r *GormRepository) CreateRiderWithdrawal(ctx context.Context, riderID, amount uint64, key string, now time.Time) (RiderWithdrawal, error) {
	if err := validateRiderWithdrawalInput(riderID, amount, key); err != nil {
		return RiderWithdrawal{}, err
	}
	if now.IsZero() {
		return RiderWithdrawal{}, ErrInvalidRecord
	}
	if err := r.RequireWithdrawalSchema(); err != nil {
		return RiderWithdrawal{}, err
	}
	var result RiderWithdrawal
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wallet, err := lockOrCreateRiderWalletTx(tx, riderID, now)
		if err != nil {
			return err
		}
		var existing RiderWithdrawal
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("idempotency_key = ?", key).First(&existing).Error
		if err == nil {
			if err := withdrawalReplay(existing, riderID, amount); err != nil {
				return err
			}
			result = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		funds, err := readWithdrawalFundsTx(tx, wallet, true, now)
		if err != nil {
			return err
		}
		if amount > funds.available {
			return ErrWalletInsufficient
		}
		id, err := NewULID()
		if err != nil {
			return err
		}
		result = RiderWithdrawal{WithdrawPublicID: id, RiderUserID: riderID, WalletPublicID: wallet.WalletPublicID,
			AmountCents: amount, Currency: "CNY", Status: RiderWithdrawalProcessing, IdempotencyKey: key, RowVersion: 1, CreatedAt: now, UpdatedAt: now}
		ledger, err := newRiderLedger(wallet, RiderLedgerDebit, RiderLedgerPayout, amount, "rider_withdrawal", id, key, now)
		if err != nil {
			return err
		}
		// Lock order is id ascending; FIFO selection happens only after all locks.
		sort.Slice(funds.candidates, func(i, j int) bool {
			a, b := funds.candidates[i].earning, funds.candidates[j].earning
			if a.SettleAt.Equal(b.SettleAt) {
				return a.ID < b.ID
			}
			return a.SettleAt.Before(b.SettleAt)
		})
		remaining := amount
		for _, candidate := range funds.candidates {
			if remaining == 0 {
				break
			}
			take := candidate.amount
			if take > remaining {
				take = remaining
			}
			allocationID, err := NewULID()
			if err != nil {
				return err
			}
			allocation := RiderWithdrawalAllocation{
				AllocationPublicID: allocationID, WithdrawPublicID: id, EarningPublicID: candidate.earning.EarningPublicID,
				DeliveryOrderPublicID: candidate.earning.DeliveryOrderPublicID, RiderUserID: riderID, WalletPublicID: wallet.WalletPublicID,
				AmountCents: take, Currency: "CNY", Status: RiderWithdrawalAllocationInFlight,
				SourcePolicyVersion: candidate.source.Policy, SourceEvidenceHash: candidate.source.EvidenceHash,
				EarningRowVersion: candidate.earning.RowVersion, CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&allocation).Error; err != nil {
				return err
			}
			remaining -= take
		}
		if remaining != 0 {
			return ErrWithdrawalSourceMismatch
		}
		if err := debitRiderWalletTx(tx, &wallet, amount, now); err != nil {
			return err
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
		return withdrawalNotificationTx(tx, r.notificationAppender, result, now)
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return result, err
}

func (r *GormRepository) ApproveRiderWithdrawal(ctx context.Context, id string, version, operator uint64, key, reference string, now time.Time) (RiderWithdrawal, error) {
	return r.ProcessRiderWithdrawal(ctx, "paid", RiderWithdrawalAction{WithdrawPublicID: id, ExpectedRowVersion: version,
		OperatorUserID: operator, IdempotencyKey: key, PaymentReference: reference}, now)
}
func (r *GormRepository) RejectRiderWithdrawal(ctx context.Context, id string, version, operator uint64, key, reason string, now time.Time) (RiderWithdrawal, error) {
	return r.ProcessRiderWithdrawal(ctx, "reject", RiderWithdrawalAction{WithdrawPublicID: id, ExpectedRowVersion: version,
		OperatorUserID: operator, IdempotencyKey: key, Reason: reason}, now)
}
func (r *GormRepository) ListRiderWithdrawalsByRider(ctx context.Context, riderID uint64, page Page) ([]RiderWithdrawal, int64, error) {
	if riderID == 0 || !page.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	return listRiderWithdrawals(r.db.WithContext(ctx).Where("rider_user_id = ?", riderID), page)
}
func (r *GormRepository) ListRiderWithdrawals(ctx context.Context, status string, riderID uint64, page Page) ([]RiderWithdrawal, int64, error) {
	if !page.Valid() || (status != "" && !RiderWithdrawalStatus(status).Valid()) {
		return nil, 0, ErrInvalidRecord
	}
	db := r.db.WithContext(ctx)
	if strings.TrimSpace(status) != "" {
		db = db.Where("status = ?", status)
	}
	if riderID != 0 {
		db = db.Where("rider_user_id = ?", riderID)
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
func normalizeWithdrawalNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrWithdrawalNotFound
	}
	return err
}
func withdrawalNotificationTx(tx *gorm.DB, appender NotificationEventAppender, value RiderWithdrawal, now time.Time) error {
	return appendFinancialNotificationTx(tx, appender, EventRiderWithdrawalChanged, value.WithdrawPublicID, value.RowVersion,
		RiderWithdrawalNotificationPayload{WithdrawPublicID: value.WithdrawPublicID, RiderUserID: value.RiderUserID, AmountCents: value.AmountCents, Status: string(value.Status)}, now)
}

// withdrawalNotificationWithIDTx is used after a withdrawal state transition
// has committed.  The audit event id is a stable idempotency key for the
// notification outbox, so a retry after a transient notification failure can
// safely re-append the same fact without creating a second user notification.
func withdrawalNotificationWithIDTx(tx *gorm.DB, appender NotificationEventAppender, value RiderWithdrawal, eventID string, now time.Time) error {
	if appender == nil {
		return nil
	}
	if tx == nil || !ValidULID(eventID) || !ValidULID(value.WithdrawPublicID) || value.RowVersion == 0 || now.IsZero() {
		return ErrInvalidRecord
	}
	payload, err := json.Marshal(RiderWithdrawalNotificationPayload{
		WithdrawPublicID: value.WithdrawPublicID,
		RiderUserID:      value.RiderUserID,
		AmountCents:      value.AmountCents,
		Status:           string(value.Status),
	})
	if err != nil {
		return err
	}
	return appendMetadataNotificationTx(tx, appender, NotificationEvent{
		EventPublicID:     eventID,
		EventType:         EventRiderWithdrawalChanged,
		AggregatePublicID: value.WithdrawPublicID,
		AggregateVersion:  value.RowVersion,
		Payload:           payload,
		OccurredAt:        now,
	})
}
