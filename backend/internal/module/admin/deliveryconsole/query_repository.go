package deliveryconsole

import (
	"context"
	"errors"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInvalidQuery = errors.New("invalid delivery admin query")

type ListFilter struct {
	Page           int
	PageSize       int
	Status         string
	MerchantUserID uint64
	RiderUserID    uint64
	From           *time.Time
	To             *time.Time
}

type QueryRepository interface {
	Summary(context.Context, time.Time) (SummaryView, error)
	FundingSources(context.Context, time.Time) (FundingSourceSummaryView, error)
	ListOrders(context.Context, ListFilter) ([]delivery.Order, int64, error)
	Order(context.Context, string) (delivery.Order, error)
	OrderFunds(context.Context, string) (OrderFundsSnapshot, error)
	ListMerchantFeeOverrides(context.Context, ListFilter) ([]delivery.MerchantFeeOverride, int64, error)
	MerchantFeeOverride(context.Context, uint64) (delivery.MerchantFeeOverride, error)
	DisableMerchantFeeOverride(context.Context, uint64, uint64, time.Time) (delivery.MerchantFeeOverride, error)
	ListMerchantWallets(context.Context, ListFilter) ([]delivery.MerchantWallet, int64, error)
	MerchantWallet(context.Context, uint64) (delivery.MerchantWallet, error)
	ListMerchantLedger(context.Context, uint64, ListFilter) ([]delivery.WalletLedger, int64, error)
	ListRiderWallets(context.Context, ListFilter) ([]delivery.RiderWallet, int64, error)
	RiderWallet(context.Context, uint64) (delivery.RiderWallet, error)
	ListRiderLedger(context.Context, uint64, ListFilter) ([]delivery.RiderLedger, int64, error)
	ListRiderEarnings(context.Context, ListFilter) ([]delivery.RiderEarningRecord, int64, error)
	RiderEarning(context.Context, string) (delivery.RiderEarningRecord, error)
	ListRiderPayouts(context.Context, ListFilter) ([]delivery.RiderPayoutRecord, int64, error)
	ListRefunds(context.Context, ListFilter) ([]delivery.Refund, int64, error)
	Refund(context.Context, string) (delivery.Refund, error)
}

type payoutQueueQueries interface {
	ListRiderPayoutQueues(context.Context, ListFilter) ([]RiderPayoutQueueSnapshot, int64, error)
	RiderPayoutQueue(context.Context, string) (RiderPayoutQueueSnapshot, error)
	ListRiderPayoutQueueEvents(context.Context, string, ListFilter) ([]delivery.RiderPayoutQueueEvent, int64, error)
}

type RiderPayoutQueueSnapshot struct {
	Queue       delivery.RiderPayoutQueue
	Reservation *delivery.RiderPayoutReservation
}

type GormQueryRepository struct{ db *gorm.DB }

func NewQueryRepository(db *gorm.DB) *GormQueryRepository { return &GormQueryRepository{db: db} }

func (r *GormQueryRepository) ListRiderPayoutQueues(ctx context.Context, filter ListFilter) ([]RiderPayoutQueueSnapshot, int64, error) {
	if !r.db.Migrator().HasTable(&delivery.RiderPayoutQueue{}) || !r.db.Migrator().HasTable(&delivery.RiderPayoutReservation{}) {
		return nil, 0, delivery.ErrPayoutQueueUnavailable
	}
	if filter.Status != "" && !delivery.RiderPayoutQueueStatus(filter.Status).Valid() {
		return nil, 0, ErrInvalidQuery
	}
	var result []RiderPayoutQueueSnapshot
	var total int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&delivery.RiderPayoutQueue{})
		if filter.Status != "" {
			query = query.Where("status = ?", filter.Status)
		}
		if filter.RiderUserID != 0 {
			query = query.Where("rider_user_id = ?", filter.RiderUserID)
		}
		if filter.From != nil {
			query = query.Where("created_at >= ?", filter.From)
		}
		if filter.To != nil {
			query = query.Where("created_at < ?", filter.To)
		}
		values, count, err := listModels[delivery.RiderPayoutQueue](query.Order("created_at DESC, id DESC"), filter)
		if err != nil {
			return err
		}
		total = count
		result = make([]RiderPayoutQueueSnapshot, 0, len(values))
		for _, value := range values {
			snapshot, err := readQueueReservation(tx, value)
			if err != nil {
				return err
			}
			result = append(result, snapshot)
		}
		return nil
	})
	return result, total, err
}

func readQueueReservation(db *gorm.DB, value delivery.RiderPayoutQueue) (RiderPayoutQueueSnapshot, error) {
	result := RiderPayoutQueueSnapshot{Queue: value}
	var reservation delivery.RiderPayoutReservation
	err := db.Where("payout_queue_public_id = ?", value.PayoutQueuePublicID).First(&reservation).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return result, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if value.Status == delivery.RiderPayoutQueueQueued || value.Status == delivery.RiderPayoutQueueCancelled {
			return result, delivery.ErrFundingSourceMismatch
		}
		return result, nil
	}
	if value.Status == delivery.RiderPayoutQueueBlocked {
		return result, delivery.ErrFundingSourceMismatch
	}
	if reservation.EarningPublicID != value.EarningPublicID || reservation.RiderUserID != value.RiderUserID || reservation.AmountCents != value.AmountCents ||
		(value.Status == delivery.RiderPayoutQueueQueued) != (reservation.Status == delivery.RiderPayoutReservationActive) {
		return result, delivery.ErrFundingSourceMismatch
	}
	result.Reservation = &reservation
	return result, nil
}

func (r *GormQueryRepository) RiderPayoutQueue(ctx context.Context, id string) (RiderPayoutQueueSnapshot, error) {
	if !r.db.Migrator().HasTable(&delivery.RiderPayoutQueue{}) || !r.db.Migrator().HasTable(&delivery.RiderPayoutReservation{}) {
		return RiderPayoutQueueSnapshot{}, delivery.ErrPayoutQueueUnavailable
	}
	var result RiderPayoutQueueSnapshot
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var queue delivery.RiderPayoutQueue
		if err := tx.Where("payout_queue_public_id = ?", id).First(&queue).Error; err != nil {
			return normalizeNotFound(err)
		}
		var err error
		result, err = readQueueReservation(tx, queue)
		return err
	})
	return result, err
}

func (r *GormQueryRepository) ListRiderPayoutQueueEvents(ctx context.Context, id string, filter ListFilter) ([]delivery.RiderPayoutQueueEvent, int64, error) {
	if !r.db.Migrator().HasTable(&delivery.RiderPayoutQueueEvent{}) {
		return nil, 0, delivery.ErrPayoutQueueUnavailable
	}
	var queue delivery.RiderPayoutQueue
	if err := r.db.WithContext(ctx).Where("payout_queue_public_id = ?", id).First(&queue).Error; err != nil {
		return nil, 0, normalizeNotFound(err)
	}
	return listModels[delivery.RiderPayoutQueueEvent](r.db.WithContext(ctx).Model(&delivery.RiderPayoutQueueEvent{}).Where("payout_queue_public_id = ?", id), filter)
}

type OrderFundsSnapshot struct {
	Order      delivery.Order
	Settlement *delivery.FeeSettlement
	Refund     *delivery.Refund
	Payment    *delivery.PaymentAttempt
}

func (r *GormQueryRepository) FundingSources(ctx context.Context, now time.Time) (FundingSourceSummaryView, error) {
	result := FundingSourceSummaryView{}
	db := r.db.WithContext(ctx)
	if err := db.Model(&delivery.MerchantWallet{}).
		Select("COALESCE(SUM(available_balance_cents),0)").Scan(&result.WalletBalanceCents).Error; err != nil {
		return FundingSourceSummaryView{}, err
	}
	if !db.Migrator().HasTable(&delivery.FundingLot{}) ||
		!db.Migrator().HasTable(&delivery.FundingAllocation{}) {
		return result, nil
	}
	result.FundingTablesAvailable = true
	if err := db.Model(&delivery.FundingLot{}).Count(&result.FundingLots).Error; err != nil {
		return FundingSourceSummaryView{}, err
	}
	if err := db.Model(&delivery.FundingAllocation{}).Count(&result.FundingAllocations).Error; err != nil {
		return FundingSourceSummaryView{}, err
	}
	var lots []delivery.FundingLot
	dataError := false
	if err := db.Select("source, verification_status, total_amount_cents, allocated_amount_cents, refunded_amount_cents").
		Find(&lots).Error; err != nil {
		return FundingSourceSummaryView{}, err
	}
	for _, lot := range lots {
		available, err := fundingLotAvailableForAdmin(lot)
		if err != nil {
			dataError = true
			continue
		}
		result.KnownAvailableCents += available
		if lot.VerificationStatus == delivery.FundingVerificationVerified {
			result.VerifiedAvailableCents += available
		} else {
			result.UnverifiedAvailableCents += available
		}
	}
	if dataError {
		result.CoverageDirection = "data_error"
	} else if result.WalletBalanceCents > result.KnownAvailableCents {
		result.CoverageGapCents = result.WalletBalanceCents - result.KnownAvailableCents
		result.CoverageDirection = "untracked_wallet_balance"
	} else if result.KnownAvailableCents > result.WalletBalanceCents {
		result.CoverageGapCents = result.KnownAvailableCents - result.WalletBalanceCents
		result.CoverageDirection = "source_exceeds_wallet"
	} else if result.CoverageDirection == "" {
		result.CoverageDirection = "balanced"
	}

	if db.Migrator().HasColumn(&delivery.PaymentAttempt{}, "next_verify_at") {
		if err := db.Model(&delivery.PaymentAttempt{}).
			Where("status IN ? AND (next_verify_at IS NULL OR next_verify_at <= ?)",
				[]delivery.PaymentStatus{delivery.PaymentCreated, delivery.PaymentPrepayIssued}, now.UTC()).
			Count(&result.PaymentVerificationPending).Error; err != nil {
			return FundingSourceSummaryView{}, err
		}
		if err := db.Model(&delivery.PaymentAttempt{}).Where("last_error_code <> ''").
			Count(&result.PaymentVerificationErrorCount).Error; err != nil {
			return FundingSourceSummaryView{}, err
		}
	}
	if db.Migrator().HasColumn(&delivery.Refund{}, "next_verify_at") {
		if err := db.Model(&delivery.Refund{}).
			Where("status IN ? AND (next_verify_at IS NULL OR next_verify_at <= ?)",
				[]delivery.RefundStatus{delivery.RefundPending, delivery.RefundRetryPending}, now.UTC()).
			Count(&result.RefundVerificationPending).Error; err != nil {
			return FundingSourceSummaryView{}, err
		}
		if err := db.Model(&delivery.Refund{}).Where("last_error_code <> ''").
			Count(&result.RefundVerificationErrorCount).Error; err != nil {
			return FundingSourceSummaryView{}, err
		}
	}
	return result, nil
}

func fundingLotAvailableForAdmin(lot delivery.FundingLot) (uint64, error) {
	if lot.RefundedAmountCents > lot.AllocatedAmountCents {
		return 0, delivery.ErrFundingSourceMismatch
	}
	net := lot.AllocatedAmountCents - lot.RefundedAmountCents
	if net > lot.TotalAmountCents {
		return 0, delivery.ErrFundingSourceMismatch
	}
	return lot.TotalAmountCents - net, nil
}

func (r *GormQueryRepository) Summary(ctx context.Context, now time.Time) (SummaryView, error) {
	var result SummaryView
	db := r.db.WithContext(ctx)
	if err := db.Model(&delivery.Order{}).Count(&result.Orders).Error; err != nil {
		return SummaryView{}, err
	}
	counts := []struct {
		target *int64
		where  string
		args   []any
	}{
		{&result.PendingFeePayment, "status = ?", []any{delivery.StatusPendingFeePayment}},
		{&result.ReadyForClaim, "status = ?", []any{delivery.StatusReadyForClaim}},
		{&result.InProgress, "status IN ?", []any{[]delivery.Status{
			delivery.StatusClaimed, delivery.StatusToPickup, delivery.StatusPickedUp, delivery.StatusDelivering,
		}}},
		{&result.Delivered, "status = ?", []any{delivery.StatusDelivered}},
		{&result.Cancelled, "status = ?", []any{delivery.StatusCancelled}},
	}
	for _, count := range counts {
		if err := db.Model(&delivery.Order{}).Where(count.where, count.args...).Count(count.target).Error; err != nil {
			return SummaryView{}, err
		}
	}
	start := shanghaiBusinessDayStart(now)
	if err := db.Model(&delivery.Order{}).Where("created_at >= ?", start).Count(&result.TodayOrders).Error; err != nil {
		return SummaryView{}, err
	}
	if err := db.Model(&delivery.Order{}).Where("delivered_at >= ?", start).Count(&result.TodayDelivered).Error; err != nil {
		return SummaryView{}, err
	}
	if err := db.Model(&delivery.Order{}).
		Where("status = ? AND delivered_at >= ?", delivery.StatusDelivered, start).
		Select("COALESCE(SUM(delivery_fee_cents),0)").
		Scan(&result.TodayDeliveryFeeCents).Error; err != nil {
		return SummaryView{}, err
	}
	if err := db.Model(&delivery.Order{}).
		Where("status = ? AND delivered_at >= ?", delivery.StatusDelivered, start).
		Select("COALESCE(SUM(platform_cut_cents),0)").
		Scan(&result.TodayPlatformFeeCents).Error; err != nil {
		return SummaryView{}, err
	}
	if err := db.Model(&delivery.MerchantWallet{}).
		Select("COALESCE(SUM(available_balance_cents),0)").Scan(&result.MerchantWalletCents).Error; err != nil {
		return SummaryView{}, err
	}
	if err := db.Model(&delivery.RiderWallet{}).
		Select("COALESCE(SUM(available_balance_cents),0)").Scan(&result.RiderWalletCents).Error; err != nil {
		return SummaryView{}, err
	}
	if err := db.Model(&delivery.RiderEarningRecord{}).Where("settlement_status IN ?", []delivery.RiderEarningStatus{
		delivery.RiderEarningPending, delivery.RiderEarningSettled,
	}).Select("COALESCE(SUM(rider_fee_cents),0)").Scan(&result.PendingRiderFeeCents).Error; err != nil {
		return SummaryView{}, err
	}
	if err := db.Model(&delivery.RiderPayoutRecord{}).
		Select("COALESCE(SUM(amount_cents),0)").Scan(&result.PaidOfflineCents).Error; err != nil {
		return SummaryView{}, err
	}
	if err := db.Model(&delivery.Order{}).Where("status = ?", delivery.StatusDelivered).
		Select("COALESCE(SUM(platform_cut_cents),0)").Scan(&result.PlatformFeeIncomeCents).Error; err != nil {
		return SummaryView{}, err
	}
	return result, nil
}

func shanghaiBusinessDayStart(now time.Time) time.Time {
	zone := time.FixedZone("Asia/Shanghai", 8*60*60)
	local := now.In(zone)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, zone).UTC()
}

func (r *GormQueryRepository) ListOrders(ctx context.Context, filter ListFilter) ([]delivery.Order, int64, error) {
	query := r.db.WithContext(ctx).Model(&delivery.Order{})
	if filter.Status != "" {
		status := delivery.Status(filter.Status)
		if !status.Valid() {
			return nil, 0, ErrInvalidQuery
		}
		query = query.Where("status = ?", status)
	}
	if filter.MerchantUserID != 0 {
		query = query.Where("merchant_user_id = ?", filter.MerchantUserID)
	}
	if filter.RiderUserID != 0 {
		query = query.Where("rider_user_id = ?", filter.RiderUserID)
	}
	query = applyTimeFilter(query, "created_at", filter)
	return listModels[delivery.Order](query, filter)
}

func (r *GormQueryRepository) Order(ctx context.Context, publicID string) (delivery.Order, error) {
	var value delivery.Order
	err := r.db.WithContext(ctx).Where("delivery_order_public_id = ?", publicID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormQueryRepository) OrderFunds(ctx context.Context, publicID string) (OrderFundsSnapshot, error) {
	order, err := r.Order(ctx, publicID)
	if err != nil {
		return OrderFundsSnapshot{}, err
	}
	result := OrderFundsSnapshot{Order: order}
	db := r.db.WithContext(ctx)
	var settlement delivery.FeeSettlement
	if err := db.Where("delivery_order_public_id = ?", publicID).First(&settlement).Error; err == nil {
		result.Settlement = &settlement
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return OrderFundsSnapshot{}, err
	}
	var refund delivery.Refund
	if err := db.Where("delivery_order_public_id = ?", publicID).First(&refund).Error; err == nil {
		result.Refund = &refund
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return OrderFundsSnapshot{}, err
	}
	if result.Settlement == nil || result.Settlement.PaymentPublicID == nil {
		return result, nil
	}
	var payment delivery.PaymentAttempt
	if err := db.Where("payment_public_id = ?", *result.Settlement.PaymentPublicID).First(&payment).Error; err == nil {
		result.Payment = &payment
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return OrderFundsSnapshot{}, err
	}
	return result, nil
}

func (r *GormQueryRepository) ListMerchantFeeOverrides(ctx context.Context, filter ListFilter) ([]delivery.MerchantFeeOverride, int64, error) {
	query := r.db.WithContext(ctx).Model(&delivery.MerchantFeeOverride{})
	if filter.Status != "" {
		status := delivery.FeeConfigStatus(filter.Status)
		if !status.Valid() {
			return nil, 0, ErrInvalidQuery
		}
		query = query.Where("status = ?", status)
	}
	if filter.MerchantUserID != 0 {
		query = query.Where("merchant_user_id = ?", filter.MerchantUserID)
	}
	query = applyTimeFilter(query, "created_at", filter)
	return listModels[delivery.MerchantFeeOverride](query, filter)
}

func (r *GormQueryRepository) MerchantFeeOverride(ctx context.Context, merchantUserID uint64) (delivery.MerchantFeeOverride, error) {
	var value delivery.MerchantFeeOverride
	err := r.db.WithContext(ctx).Where("merchant_user_id = ?", merchantUserID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormQueryRepository) DisableMerchantFeeOverride(
	ctx context.Context,
	merchantUserID uint64,
	expectedRowVersion uint64,
	now time.Time,
) (delivery.MerchantFeeOverride, error) {
	if merchantUserID == 0 || expectedRowVersion == 0 {
		return delivery.MerchantFeeOverride{}, delivery.ErrInvalidRecord
	}
	var result delivery.MerchantFeeOverride
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("merchant_user_id = ?", merchantUserID).First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		if result.Status == delivery.FeeConfigDisabled {
			return nil
		}
		update := tx.Model(&delivery.MerchantFeeOverride{}).
			Where("id = ? AND row_version = ?", result.ID, expectedRowVersion).
			Updates(map[string]any{
				"status": delivery.FeeConfigDisabled, "row_version": gorm.Expr("row_version + 1"),
				"updated_at": now,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return delivery.ErrConcurrentUpdate
		}
		result.Status = delivery.FeeConfigDisabled
		result.RowVersion++
		result.UpdatedAt = now
		return nil
	})
	return result, err
}

func (r *GormQueryRepository) ListMerchantWallets(ctx context.Context, filter ListFilter) ([]delivery.MerchantWallet, int64, error) {
	query := r.db.WithContext(ctx).Model(&delivery.MerchantWallet{})
	if filter.MerchantUserID != 0 {
		query = query.Where("merchant_user_id = ?", filter.MerchantUserID)
	}
	query = applyTimeFilter(query, "created_at", filter)
	return listModels[delivery.MerchantWallet](query, filter)
}

func (r *GormQueryRepository) MerchantWallet(ctx context.Context, merchantUserID uint64) (delivery.MerchantWallet, error) {
	var value delivery.MerchantWallet
	err := r.db.WithContext(ctx).Where("merchant_user_id = ?", merchantUserID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormQueryRepository) ListMerchantLedger(ctx context.Context, merchantUserID uint64, filter ListFilter) ([]delivery.WalletLedger, int64, error) {
	query := r.db.WithContext(ctx).Model(&delivery.WalletLedger{}).Where("merchant_user_id = ?", merchantUserID)
	if filter.Status != "" {
		query = query.Where("entry_type = ?", filter.Status)
	}
	query = applyTimeFilter(query, "created_at", filter)
	return listModels[delivery.WalletLedger](query, filter)
}

func (r *GormQueryRepository) ListRiderWallets(ctx context.Context, filter ListFilter) ([]delivery.RiderWallet, int64, error) {
	query := r.db.WithContext(ctx).Model(&delivery.RiderWallet{})
	if filter.RiderUserID != 0 {
		query = query.Where("rider_user_id = ?", filter.RiderUserID)
	}
	query = applyTimeFilter(query, "created_at", filter)
	return listModels[delivery.RiderWallet](query, filter)
}

func (r *GormQueryRepository) RiderWallet(ctx context.Context, riderUserID uint64) (delivery.RiderWallet, error) {
	var value delivery.RiderWallet
	err := r.db.WithContext(ctx).Where("rider_user_id = ?", riderUserID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormQueryRepository) ListRiderLedger(ctx context.Context, riderUserID uint64, filter ListFilter) ([]delivery.RiderLedger, int64, error) {
	query := r.db.WithContext(ctx).Model(&delivery.RiderLedger{}).Where("rider_user_id = ?", riderUserID)
	if filter.Status != "" {
		query = query.Where("entry_type = ?", filter.Status)
	}
	query = applyTimeFilter(query, "created_at", filter)
	return listModels[delivery.RiderLedger](query, filter)
}

func (r *GormQueryRepository) ListRiderEarnings(ctx context.Context, filter ListFilter) ([]delivery.RiderEarningRecord, int64, error) {
	query := r.db.WithContext(ctx).Model(&delivery.RiderEarningRecord{})
	if filter.Status != "" {
		status := delivery.RiderEarningStatus(filter.Status)
		if !status.Valid() {
			return nil, 0, ErrInvalidQuery
		}
		query = query.Where("settlement_status = ?", status)
	}
	if filter.RiderUserID != 0 {
		query = query.Where("rider_user_id = ?", filter.RiderUserID)
	}
	query = applyTimeFilter(query, "created_at", filter)
	return listModels[delivery.RiderEarningRecord](query, filter)
}

func (r *GormQueryRepository) RiderEarning(ctx context.Context, earningPublicID string) (delivery.RiderEarningRecord, error) {
	var value delivery.RiderEarningRecord
	err := r.db.WithContext(ctx).Where("earning_public_id = ?", earningPublicID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormQueryRepository) ListRiderPayouts(ctx context.Context, filter ListFilter) ([]delivery.RiderPayoutRecord, int64, error) {
	query := r.db.WithContext(ctx).Model(&delivery.RiderPayoutRecord{})
	if filter.RiderUserID != 0 {
		query = query.Where("rider_user_id = ?", filter.RiderUserID)
	}
	query = applyTimeFilter(query, "created_at", filter)
	return listModels[delivery.RiderPayoutRecord](query, filter)
}

func (r *GormQueryRepository) ListRefunds(ctx context.Context, filter ListFilter) ([]delivery.Refund, int64, error) {
	query := r.db.WithContext(ctx).Model(&delivery.Refund{})
	if filter.Status != "" {
		status := delivery.RefundStatus(filter.Status)
		if status != delivery.RefundPending && status != delivery.RefundSucceeded &&
			status != delivery.RefundFailed && status != delivery.RefundRetryPending {
			return nil, 0, ErrInvalidQuery
		}
		query = query.Where("status = ?", status)
	}
	if filter.MerchantUserID != 0 {
		query = query.Where("merchant_user_id = ?", filter.MerchantUserID)
	}
	query = applyTimeFilter(query, "created_at", filter)
	return listModels[delivery.Refund](query, filter)
}

func (r *GormQueryRepository) Refund(ctx context.Context, refundPublicID string) (delivery.Refund, error) {
	var value delivery.Refund
	err := r.db.WithContext(ctx).Where("refund_public_id = ?", refundPublicID).First(&value).Error
	return value, normalizeNotFound(err)
}

func listModels[T any](query *gorm.DB, filter ListFilter) ([]T, int64, error) {
	if filter.Page < 1 || filter.PageSize < 1 || filter.PageSize > 100 {
		return nil, 0, ErrInvalidQuery
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []T
	err := query.Order("id DESC").Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Find(&values).Error
	return values, total, err
}

func applyTimeFilter(query *gorm.DB, column string, filter ListFilter) *gorm.DB {
	if filter.From != nil {
		query = query.Where(column+" >= ?", *filter.From)
	}
	if filter.To != nil {
		query = query.Where(column+" < ?", *filter.To)
	}
	return query
}

func normalizeNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return delivery.ErrNotFound
	}
	return err
}

var _ QueryRepository = (*GormQueryRepository)(nil)
