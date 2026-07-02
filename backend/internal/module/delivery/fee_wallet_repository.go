package delivery

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FeeConfigRepository interface {
	FeePolicyForMerchant(context.Context, uint64) (FeePolicy, error)
	GlobalFeeConfig(context.Context) (DeliveryFeeConfig, error)
	UpdateGlobalFeeConfig(context.Context, uint64, uint64, uint64, time.Time) (DeliveryFeeConfig, error)
	UpsertMerchantFeeOverride(context.Context, uint64, uint64, uint64, *uint64, time.Time) (MerchantFeeOverride, error)
}

type RiderWalletRepository interface {
	RiderWallet(context.Context, uint64, time.Time) (RiderWallet, error)
	FindRiderEarningByOrder(context.Context, string) (RiderEarningRecord, error)
	FindRiderEarning(context.Context, string) (RiderEarningRecord, error)
	MarkRiderEarningPaidOffline(context.Context, string, uint64, uint64, string, string, time.Time) (RiderPayoutResult, error)
}

var _ FeeConfigRepository = (*GormRepository)(nil)
var _ RiderWalletRepository = (*GormRepository)(nil)

type RiderPayoutResult struct {
	Wallet  RiderWallet
	Earning RiderEarningRecord
	Ledger  RiderLedger
	Payout  RiderPayoutRecord
}

func (r *GormRepository) FeePolicyForMerchant(ctx context.Context, merchantUserID uint64) (FeePolicy, error) {
	if merchantUserID == 0 {
		return FeePolicy{}, ErrInvalidRecord
	}
	var override MerchantFeeOverride
	err := r.db.WithContext(ctx).
		Where("merchant_user_id = ? AND status = ?", merchantUserID, FeeConfigActive).
		First(&override).Error
	if err == nil {
		return feePolicyFromAmounts(override.DeliveryFeeCents, override.PlatformCutCents)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return FeePolicy{}, err
	}
	config, err := r.GlobalFeeConfig(ctx)
	if err != nil {
		return FeePolicy{}, err
	}
	return feePolicyFromAmounts(config.DeliveryFeeCents, config.PlatformCutCents)
}

func (r *GormRepository) GlobalFeeConfig(ctx context.Context) (DeliveryFeeConfig, error) {
	var config DeliveryFeeConfig
	err := r.db.WithContext(ctx).
		Where("config_scope = ? AND status = ?", "global", FeeConfigActive).
		First(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DeliveryFeeConfig{}, ErrFeeConfigNotFound
	}
	return config, err
}

func (r *GormRepository) UpdateGlobalFeeConfig(
	ctx context.Context,
	deliveryFeeCents uint64,
	platformCutCents uint64,
	expectedRowVersion uint64,
	now time.Time,
) (DeliveryFeeConfig, error) {
	if expectedRowVersion == 0 || validateFeeConfigAmount(deliveryFeeCents, platformCutCents) != nil {
		return DeliveryFeeConfig{}, ErrInvalidRecord
	}
	var result DeliveryFeeConfig
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("config_scope = ?", "global").First(&result).Error; err != nil {
			return normalizeNotFound(err)
		}
		update := tx.Model(&DeliveryFeeConfig{}).
			Where("id = ? AND row_version = ?", result.ID, expectedRowVersion).
			Updates(map[string]any{
				"delivery_fee_cents": deliveryFeeCents,
				"platform_cut_cents": platformCutCents,
				"status":             FeeConfigActive,
				"row_version":        gorm.Expr("row_version + 1"),
				"updated_at":         now,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		result.DeliveryFeeCents = deliveryFeeCents
		result.PlatformCutCents = platformCutCents
		result.Status = FeeConfigActive
		result.RowVersion++
		result.UpdatedAt = now
		return nil
	})
	return result, err
}

func (r *GormRepository) UpsertMerchantFeeOverride(
	ctx context.Context,
	merchantUserID uint64,
	deliveryFeeCents uint64,
	platformCutCents uint64,
	expectedRowVersion *uint64,
	now time.Time,
) (MerchantFeeOverride, error) {
	if merchantUserID == 0 || validateFeeConfigAmount(deliveryFeeCents, platformCutCents) != nil {
		return MerchantFeeOverride{}, ErrInvalidRecord
	}
	var result MerchantFeeOverride
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("merchant_user_id = ?", merchantUserID).First(&result).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if expectedRowVersion != nil && *expectedRowVersion != 0 {
				return ErrConcurrentUpdate
			}
			result = MerchantFeeOverride{
				MerchantUserID: merchantUserID, DeliveryFeeCents: deliveryFeeCents,
				PlatformCutCents: platformCutCents, Status: FeeConfigActive,
				RowVersion: 1, CreatedAt: now, UpdatedAt: now,
			}
			return tx.Create(&result).Error
		}
		if err != nil {
			return err
		}
		if expectedRowVersion == nil || *expectedRowVersion == 0 {
			return ErrConcurrentUpdate
		}
		update := tx.Model(&MerchantFeeOverride{}).
			Where("id = ? AND row_version = ?", result.ID, *expectedRowVersion).
			Updates(map[string]any{
				"delivery_fee_cents": deliveryFeeCents, "platform_cut_cents": platformCutCents,
				"status": FeeConfigActive, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		result.DeliveryFeeCents = deliveryFeeCents
		result.PlatformCutCents = platformCutCents
		result.Status = FeeConfigActive
		result.RowVersion++
		result.UpdatedAt = now
		return nil
	})
	return result, err
}

func (r *GormRepository) RiderWallet(ctx context.Context, riderUserID uint64, now time.Time) (RiderWallet, error) {
	var result RiderWallet
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = lockOrCreateRiderWalletTx(tx, riderUserID, now)
		return err
	})
	return result, err
}

func (r *GormRepository) FindRiderEarningByOrder(ctx context.Context, orderPublicID string) (RiderEarningRecord, error) {
	var result RiderEarningRecord
	err := r.db.WithContext(ctx).
		Where("delivery_order_public_id = ?", orderPublicID).
		First(&result).Error
	return result, normalizeNotFound(err)
}

func (r *GormRepository) FindRiderEarning(ctx context.Context, earningPublicID string) (RiderEarningRecord, error) {
	var result RiderEarningRecord
	err := r.db.WithContext(ctx).
		Where("earning_public_id = ?", earningPublicID).
		First(&result).Error
	return result, normalizeNotFound(err)
}

func (r *GormRepository) MarkRiderEarningPaidOffline(
	ctx context.Context,
	earningPublicID string,
	expectedRowVersion uint64,
	operatorUserID uint64,
	idempotencyKey string,
	note string,
	now time.Time,
) (RiderPayoutResult, error) {
	if !ValidULID(earningPublicID) || expectedRowVersion == 0 || operatorUserID == 0 || !ValidULID(idempotencyKey) {
		return RiderPayoutResult{}, ErrInvalidRecord
	}
	var result RiderPayoutResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var earning RiderEarningRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("earning_public_id = ?", earningPublicID).First(&earning).Error; err != nil {
			return normalizeNotFound(err)
		}
		if earning.SettlementStatus == RiderEarningPaidOffline {
			var payout RiderPayoutRecord
			if err := tx.Where("earning_public_id = ?", earning.EarningPublicID).First(&payout).Error; err != nil {
				return normalizeNotFound(err)
			}
			var ledger RiderLedger
			if err := tx.Where("idempotency_key = ?", payout.IdempotencyKey).First(&ledger).Error; err != nil {
				return normalizeNotFound(err)
			}
			wallet, err := lockOrCreateRiderWalletTx(tx, earning.RiderUserID, now)
			result = RiderPayoutResult{Wallet: wallet, Earning: earning, Ledger: ledger, Payout: payout}
			return err
		}
		if earning.SettlementStatus != RiderEarningPending && earning.SettlementStatus != RiderEarningSettled {
			return ErrEarningSettled
		}
		if earning.RowVersion != expectedRowVersion {
			return ErrConcurrentUpdate
		}
		wallet, err := lockOrCreateRiderWalletTx(tx, earning.RiderUserID, now)
		if err != nil {
			return err
		}
		payoutID, err := NewULID()
		if err != nil {
			return err
		}
		ledger, err := newRiderLedger(
			wallet, RiderLedgerDebit, RiderLedgerPayout, earning.RiderFeeCents,
			"rider_payout", payoutID, idempotencyKey, now,
		)
		if err != nil {
			return err
		}
		payout := RiderPayoutRecord{
			PayoutPublicID: payoutID, EarningPublicID: earning.EarningPublicID,
			DeliveryOrderPublicID: earning.DeliveryOrderPublicID, RiderUserID: earning.RiderUserID,
			OperatorUserID: operatorUserID, AmountCents: earning.RiderFeeCents,
			IdempotencyKey: idempotencyKey, Note: truncateRiderPayoutNote(note),
			PaidOfflineAt: now, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
		}
		if err := debitRiderWalletTx(tx, &wallet, earning.RiderFeeCents, now); err != nil {
			return err
		}
		if err := tx.Create(&payout).Error; err != nil {
			return err
		}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		update := tx.Model(&RiderEarningRecord{}).
			Where("id = ? AND row_version = ? AND settlement_status IN ?", earning.ID, expectedRowVersion,
				[]RiderEarningStatus{RiderEarningPending, RiderEarningSettled}).
			Updates(map[string]any{
				"settlement_status": RiderEarningPaidOffline,
				"paid_offline_at":   now,
				"row_version":       gorm.Expr("row_version + 1"),
				"updated_at":        now,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		earning.SettlementStatus = RiderEarningPaidOffline
		earning.PaidOfflineAt = timePointer(now)
		earning.RowVersion++
		earning.UpdatedAt = now
		result = RiderPayoutResult{Wallet: wallet, Earning: earning, Ledger: ledger, Payout: payout}
		return nil
	})
	return result, err
}

func settleDeliveredOrderTx(tx *gorm.DB, order Order, now time.Time) (RiderEarningRecord, RiderWallet, RiderLedger, error) {
	if order.Status != StatusDelivered || order.RiderUserID == nil || *order.RiderUserID == 0 {
		return RiderEarningRecord{}, RiderWallet{}, RiderLedger{}, ErrInvalidRecord
	}
	if err := validateRiderEarningAmount(order.DeliveryFeeCents, order.PlatformCutCents, order.RiderFeeCents); err != nil {
		return RiderEarningRecord{}, RiderWallet{}, RiderLedger{}, err
	}
	var existing RiderEarningRecord
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("delivery_order_public_id = ?", order.DeliveryOrderPublicID).First(&existing).Error
	if err == nil {
		if existing.RiderUserID != *order.RiderUserID ||
			existing.DeliveryFeeCents != order.DeliveryFeeCents ||
			existing.PlatformCutCents != order.PlatformCutCents ||
			existing.RiderFeeCents != order.RiderFeeCents {
			return RiderEarningRecord{}, RiderWallet{}, RiderLedger{}, ErrIdempotencyConflict
		}
		wallet, walletErr := lockOrCreateRiderWalletTx(tx, existing.RiderUserID, now)
		var ledger RiderLedger
		if ledgerErr := tx.Where("idempotency_key = ?", order.DeliveryOrderPublicID).First(&ledger).Error; ledgerErr != nil {
			return existing, wallet, ledger, normalizeNotFound(ledgerErr)
		}
		return existing, wallet, ledger, walletErr
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return RiderEarningRecord{}, RiderWallet{}, RiderLedger{}, err
	}
	wallet, err := lockOrCreateRiderWalletTx(tx, *order.RiderUserID, now)
	if err != nil {
		return RiderEarningRecord{}, RiderWallet{}, RiderLedger{}, err
	}
	earningID, err := NewULID()
	if err != nil {
		return RiderEarningRecord{}, RiderWallet{}, RiderLedger{}, err
	}
	earning := RiderEarningRecord{
		EarningPublicID: earningID, DeliveryOrderPublicID: order.DeliveryOrderPublicID,
		RiderUserID: *order.RiderUserID, DeliveryFeeCents: order.DeliveryFeeCents,
		PlatformCutCents: order.PlatformCutCents, RiderFeeCents: order.RiderFeeCents,
		SettlementStatus: RiderEarningPending, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	ledger, err := newRiderLedger(
		wallet, RiderLedgerCredit, RiderLedgerIncome, order.RiderFeeCents,
		"delivery_order", order.DeliveryOrderPublicID, order.DeliveryOrderPublicID, now,
	)
	if err != nil {
		return RiderEarningRecord{}, RiderWallet{}, RiderLedger{}, err
	}
	if err := creditRiderWalletTx(tx, &wallet, order.RiderFeeCents, now); err != nil {
		return RiderEarningRecord{}, RiderWallet{}, RiderLedger{}, err
	}
	if err := tx.Create(&earning).Error; err != nil {
		if isDuplicateKey(err) {
			return RiderEarningRecord{}, RiderWallet{}, RiderLedger{}, ErrIdempotencyConflict
		}
		return RiderEarningRecord{}, RiderWallet{}, RiderLedger{}, err
	}
	if err := tx.Create(&ledger).Error; err != nil {
		return RiderEarningRecord{}, RiderWallet{}, RiderLedger{}, err
	}
	return earning, wallet, ledger, nil
}

func lockOrCreateRiderWalletTx(tx *gorm.DB, riderUserID uint64, now time.Time) (RiderWallet, error) {
	if riderUserID == 0 {
		return RiderWallet{}, ErrInvalidRecord
	}
	var wallet RiderWallet
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("rider_user_id = ?", riderUserID).First(&wallet).Error
	if err == nil {
		return wallet, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return RiderWallet{}, err
	}
	publicID, err := NewULID()
	if err != nil {
		return RiderWallet{}, err
	}
	wallet = RiderWallet{
		WalletPublicID: publicID, RiderUserID: riderUserID,
		AvailableBalanceCents: 0, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&wallet).Error; err != nil {
		return RiderWallet{}, err
	}
	if wallet.ID == 0 {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("rider_user_id = ?", riderUserID).First(&wallet).Error; err != nil {
			return RiderWallet{}, err
		}
	}
	return wallet, nil
}

func creditRiderWalletTx(tx *gorm.DB, wallet *RiderWallet, amount uint64, now time.Time) error {
	if amount == 0 || ^uint64(0)-wallet.AvailableBalanceCents < amount {
		return ErrAmountOverflow
	}
	update := tx.Model(&RiderWallet{}).Where("id = ? AND row_version = ?", wallet.ID, wallet.RowVersion).
		Updates(map[string]any{
			"available_balance_cents": gorm.Expr("available_balance_cents + ?", amount),
			"row_version":             gorm.Expr("row_version + 1"), "updated_at": now,
		})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return ErrConcurrentUpdate
	}
	wallet.AvailableBalanceCents += amount
	wallet.RowVersion++
	wallet.UpdatedAt = now
	return nil
}

func debitRiderWalletTx(tx *gorm.DB, wallet *RiderWallet, amount uint64, now time.Time) error {
	if amount == 0 || wallet.AvailableBalanceCents < amount {
		return ErrWalletInsufficient
	}
	update := tx.Model(&RiderWallet{}).
		Where("id = ? AND row_version = ? AND available_balance_cents >= ?", wallet.ID, wallet.RowVersion, amount).
		Updates(map[string]any{
			"available_balance_cents": gorm.Expr("available_balance_cents - ?", amount),
			"row_version":             gorm.Expr("row_version + 1"), "updated_at": now,
		})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return ErrConcurrentUpdate
	}
	wallet.AvailableBalanceCents -= amount
	wallet.RowVersion++
	wallet.UpdatedAt = now
	return nil
}

func newRiderLedger(
	wallet RiderWallet,
	direction RiderLedgerDirection,
	entryType RiderLedgerEntryType,
	amount uint64,
	referenceType, referenceID, idempotencyKey string,
	now time.Time,
) (RiderLedger, error) {
	publicID, err := NewULID()
	if err != nil {
		return RiderLedger{}, err
	}
	if !ValidULID(referenceID) || !ValidULID(idempotencyKey) || amount == 0 {
		return RiderLedger{}, ErrInvalidRecord
	}
	after := wallet.AvailableBalanceCents
	if direction == RiderLedgerCredit {
		if ^uint64(0)-after < amount {
			return RiderLedger{}, ErrAmountOverflow
		}
		after += amount
	} else {
		if after < amount {
			return RiderLedger{}, ErrWalletInsufficient
		}
		after -= amount
	}
	return RiderLedger{
		LedgerPublicID: publicID, WalletPublicID: wallet.WalletPublicID, RiderUserID: wallet.RiderUserID,
		Direction: direction, EntryType: entryType, AmountCents: amount,
		BalanceBeforeCents: wallet.AvailableBalanceCents, BalanceAfterCents: after,
		ReferenceType: referenceType, ReferencePublicID: referenceID,
		IdempotencyKey: idempotencyKey, CreatedAt: now,
	}, nil
}
