package delivery

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type RiderWalletState string

const (
	RiderWalletReady      RiderWalletState = "ready"
	RiderWalletNotCreated RiderWalletState = "not_created"
)

type RiderWalletSnapshot struct {
	WalletState           RiderWalletState
	WalletPublicID        string
	AvailableBalanceCents uint64
	ReservedCents         uint64
	UnverifiedCents       uint64
	UpdatedAt             *time.Time
}

type RiderPayoutQueueReadItem struct {
	PayoutQueuePublicID   string
	EarningPublicID       string
	DeliveryOrderPublicID string
	AmountCents           uint64
	Status                string
	ReasonCode            string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type RiderWalletReader interface {
	ReadRiderWalletSnapshot(context.Context, uint64) (RiderWalletSnapshot, error)
	ListRiderEarningsByRider(context.Context, uint64, Page) ([]RiderEarningRecord, int64, error)
	ListRiderLedgerByRider(context.Context, uint64, Page) ([]RiderLedger, int64, error)
	ListRiderPayoutQueueByRider(context.Context, uint64, Page) ([]RiderPayoutQueueReadItem, int64, error)
}

var _ RiderWalletReader = (*GormRepository)(nil)

func (r *GormRepository) ReadRiderWalletSnapshot(ctx context.Context, riderUserID uint64) (RiderWalletSnapshot, error) {
	if riderUserID == 0 {
		return RiderWalletSnapshot{}, ErrInvalidRecord
	}
	if !r.db.Migrator().HasTable(&RiderPayoutQueue{}) || !r.db.Migrator().HasTable(&RiderPayoutReservation{}) {
		return RiderWalletSnapshot{}, ErrPayoutQueueUnavailable
	}
	db := r.db.WithContext(ctx)
	var wallet RiderWallet
	err := db.Where("rider_user_id = ?", riderUserID).First(&wallet).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return RiderWalletSnapshot{}, err
	}
	var earningCount int64
	if countErr := db.Model(&RiderEarningRecord{}).Where("rider_user_id = ?", riderUserID).Count(&earningCount).Error; countErr != nil {
		return RiderWalletSnapshot{}, countErr
	}
	if errors.Is(err, gorm.ErrRecordNotFound) && earningCount > 0 {
		return RiderWalletSnapshot{}, ErrFundingSourceMismatch
	}
	var summary struct {
		ReservedCents   uint64
		UnverifiedCents uint64
	}
	if queryErr := db.Raw("SELECT COALESCE(SUM(CASE WHEN q.status = 'queued' AND r.status = 'active' THEN r.amount_cents ELSE 0 END), 0) AS reserved_cents, "+
		"COALESCE(SUM(CASE WHEN q.status = 'blocked' THEN q.amount_cents ELSE 0 END), 0) AS unverified_cents "+
		"FROM delivery_rider_payout_queue q LEFT JOIN delivery_rider_payout_reservations r "+
		"ON r.payout_queue_public_id = q.payout_queue_public_id WHERE q.rider_user_id = ?", riderUserID).Scan(&summary).Error; queryErr != nil {
		return RiderWalletSnapshot{}, queryErr
	}
	var unqueued struct{ Amount uint64 }
	if queryErr := db.Raw("SELECT COALESCE(SUM(e.rider_fee_cents), 0) AS amount FROM delivery_rider_earning_records e "+
		"LEFT JOIN delivery_rider_payout_queue q ON q.earning_public_id = e.earning_public_id "+
		"WHERE e.rider_user_id = ? AND e.settlement_status IN ('pending', 'settled') AND q.id IS NULL", riderUserID).Scan(&unqueued).Error; queryErr != nil {
		return RiderWalletSnapshot{}, queryErr
	}
	var ok bool
	summary.UnverifiedCents, ok = checkedAddAmount(summary.UnverifiedCents, unqueued.Amount)
	if !ok {
		return RiderWalletSnapshot{}, ErrAmountOverflow
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return RiderWalletSnapshot{WalletState: RiderWalletNotCreated, UnverifiedCents: summary.UnverifiedCents}, nil
	}
	if summary.ReservedCents > wallet.AvailableBalanceCents {
		return RiderWalletSnapshot{}, ErrFundingSourceMismatch
	}
	updatedAt := wallet.UpdatedAt
	return RiderWalletSnapshot{
		WalletState: RiderWalletReady, WalletPublicID: wallet.WalletPublicID,
		AvailableBalanceCents: wallet.AvailableBalanceCents,
		ReservedCents:         summary.ReservedCents, UnverifiedCents: summary.UnverifiedCents,
		UpdatedAt: &updatedAt,
	}, nil
}

func (r *GormRepository) ListRiderEarningsByRider(ctx context.Context, riderUserID uint64, page Page) ([]RiderEarningRecord, int64, error) {
	if riderUserID == 0 || !page.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	query := r.db.WithContext(ctx).Model(&RiderEarningRecord{}).Where("rider_user_id = ?", riderUserID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	values := make([]RiderEarningRecord, 0)
	err := query.Order("created_at DESC, id DESC").Offset((page.Page - 1) * page.Size).Limit(page.Size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) ListRiderLedgerByRider(ctx context.Context, riderUserID uint64, page Page) ([]RiderLedger, int64, error) {
	if riderUserID == 0 || !page.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	query := r.db.WithContext(ctx).Model(&RiderLedger{}).Where("rider_user_id = ?", riderUserID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	values := make([]RiderLedger, 0)
	err := query.Order("created_at DESC, id DESC").Offset((page.Page - 1) * page.Size).Limit(page.Size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) ListRiderPayoutQueueByRider(ctx context.Context, riderUserID uint64, page Page) ([]RiderPayoutQueueReadItem, int64, error) {
	if riderUserID == 0 || !page.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	if !r.db.Migrator().HasTable(&RiderPayoutQueue{}) {
		return nil, 0, ErrPayoutQueueUnavailable
	}
	query := r.db.WithContext(ctx).Table((RiderPayoutQueue{}).TableName()).Where("rider_user_id = ?", riderUserID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	values := make([]RiderPayoutQueueReadItem, 0)
	err := query.Select("payout_queue_public_id, earning_public_id, delivery_order_public_id, amount_cents, status, reason_code, created_at, updated_at").
		Order("created_at DESC, id DESC").Offset((page.Page - 1) * page.Size).Limit(page.Size).Scan(&values).Error
	return values, total, err
}
