package takeoutrider

import (
	"context"

	"gorm.io/gorm"

	"campus-circle/backend/internal/module/takeout"
)

type WalletFacts struct {
	PendingSettlementCents uint64
	PendingWithdrawCents   uint64
	AvailableCents         uint64
	SettledCents           uint64
}

type Repository interface {
	FindFulfillment(context.Context, string) (takeout.Fulfillment, error)
	ListFulfillmentsByRider(context.Context, uint64, takeout.FulfillmentStatus, int, int) ([]takeout.Fulfillment, int64, error)
	MerchantNames(context.Context, []string) (map[string]string, error)
	ListEarningsByRider(context.Context, uint64, int, int) ([]takeout.RiderEarningRecord, int64, error)
	WalletFacts(context.Context, uint64) (WalletFacts, error)
	ListWithdrawalsByRider(context.Context, uint64, takeout.WithdrawStatus, int, int) ([]takeout.WithdrawRequest, int64, error)
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) FindFulfillment(ctx context.Context, publicID string) (takeout.Fulfillment, error) {
	var value takeout.Fulfillment
	err := r.db.WithContext(ctx).Where("fulfillment_public_id = ?", publicID).First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) ListFulfillmentsByRider(
	ctx context.Context,
	riderUserID uint64,
	status takeout.FulfillmentStatus,
	page, size int,
) ([]takeout.Fulfillment, int64, error) {
	query := r.db.WithContext(ctx).Model(&takeout.Fulfillment{}).Where("rider_user_id = ?", riderUserID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []takeout.Fulfillment
	err := query.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) MerchantNames(ctx context.Context, publicIDs []string) (map[string]string, error) {
	result := map[string]string{}
	if len(publicIDs) == 0 {
		return result, nil
	}
	var values []takeout.Merchant
	if err := r.db.WithContext(ctx).Where("merchant_public_id IN ?", publicIDs).Find(&values).Error; err != nil {
		return nil, err
	}
	for _, value := range values {
		result[value.MerchantPublicID] = value.Name
	}
	return result, nil
}

func (r *GormRepository) ListEarningsByRider(ctx context.Context, riderUserID uint64, page, size int) ([]takeout.RiderEarningRecord, int64, error) {
	query := r.db.WithContext(ctx).Model(&takeout.RiderEarningRecord{}).Where("rider_user_id = ?", riderUserID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []takeout.RiderEarningRecord
	err := query.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) WalletFacts(ctx context.Context, riderUserID uint64) (WalletFacts, error) {
	var pendingSettlement uint64
	if err := r.db.WithContext(ctx).Model(&takeout.RiderEarningRecord{}).
		Where("rider_user_id = ? AND settlement_status = ?", riderUserID, takeout.SettlementPending).
		Select("COALESCE(SUM(rider_fee_cents), 0)").Scan(&pendingSettlement).Error; err != nil {
		return WalletFacts{}, err
	}
	var settled uint64
	if err := r.db.WithContext(ctx).Model(&takeout.RiderEarningRecord{}).
		Where("rider_user_id = ? AND settlement_status = ?", riderUserID, takeout.SettlementSettled).
		Select("COALESCE(SUM(rider_fee_cents), 0)").Scan(&settled).Error; err != nil {
		return WalletFacts{}, err
	}
	var pendingWithdraw uint64
	if err := r.db.WithContext(ctx).Model(&takeout.WithdrawRequest{}).
		Where("rider_user_id = ? AND status = ?", riderUserID, takeout.WithdrawPending).
		Select("COALESCE(SUM(amount_cents), 0)").Scan(&pendingWithdraw).Error; err != nil {
		return WalletFacts{}, err
	}
	available := uint64(0)
	if pendingSettlement > pendingWithdraw {
		available = pendingSettlement - pendingWithdraw
	}
	return WalletFacts{
		PendingSettlementCents: pendingSettlement, PendingWithdrawCents: pendingWithdraw,
		AvailableCents: available, SettledCents: settled,
	}, nil
}

func (r *GormRepository) ListWithdrawalsByRider(
	ctx context.Context,
	riderUserID uint64,
	status takeout.WithdrawStatus,
	page, size int,
) ([]takeout.WithdrawRequest, int64, error) {
	query := r.db.WithContext(ctx).Model(&takeout.WithdrawRequest{}).Where("rider_user_id = ?", riderUserID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []takeout.WithdrawRequest
	err := query.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}

func normalizeNotFound(err error) error {
	if err == nil {
		return nil
	}
	if err == gorm.ErrRecordNotFound {
		return takeout.ErrNotFound
	}
	return err
}

var _ Repository = (*GormRepository)(nil)
