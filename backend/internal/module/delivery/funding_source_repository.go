package delivery

import (
	"sort"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func createRechargeFundingLotTx(
	tx *gorm.DB,
	wallet MerchantWallet,
	attempt PaymentAttempt,
	now time.Time,
) (FundingLot, error) {
	if attempt.Purpose != PaymentWalletRecharge || attempt.Status != PaymentPaid ||
		attempt.MerchantUserID != wallet.MerchantUserID || attempt.AmountCents == 0 ||
		attempt.Currency != "CNY" {
		return FundingLot{}, ErrFundingSourceMismatch
	}
	source := FundingSourceMockRecharge
	verification := FundingVerificationUnverified
	switch attempt.Provider {
	case PaymentWechat:
		if attempt.WechatTransactionID == nil || *attempt.WechatTransactionID == "" {
			return FundingLot{}, ErrFundingSourceMismatch
		}
		source = FundingSourceWechatRecharge
		verification = FundingVerificationVerified
	case PaymentMock:
		if attempt.MockTraceID == nil || *attempt.MockTraceID == "" {
			return FundingLot{}, ErrFundingSourceMismatch
		}
	default:
		return FundingLot{}, ErrFundingSourceMismatch
	}
	publicID, err := NewULID()
	if err != nil {
		return FundingLot{}, err
	}
	paymentPublicID := attempt.PaymentPublicID
	lot := FundingLot{
		FundingLotPublicID: publicID,
		WalletPublicID:     wallet.WalletPublicID,
		MerchantUserID:     wallet.MerchantUserID,
		Source:             source,
		VerificationStatus: verification,
		PaymentPublicID:    &paymentPublicID,
		TotalAmountCents:   attempt.AmountCents,
		Currency:           "CNY",
		RowVersion:         1,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := validateFundingLot(lot); err != nil {
		return FundingLot{}, err
	}
	if err := tx.Create(&lot).Error; err != nil {
		return FundingLot{}, err
	}
	return lot, nil
}

func allocateWalletFundingTx(
	tx *gorm.DB,
	wallet MerchantWallet,
	settlement FeeSettlement,
	now time.Time,
) ([]FundingAllocation, error) {
	if settlement.Source != SettlementWallet || settlement.MerchantUserID != wallet.MerchantUserID ||
		settlement.AmountCents == 0 || wallet.AvailableBalanceCents < settlement.AmountCents {
		return nil, ErrFundingSourceMismatch
	}
	lots, available, err := lockWalletFundingLotsTx(tx, wallet)
	if err != nil {
		return nil, err
	}
	lots, available, err = ensureWalletFundingCoverageTx(tx, wallet, lots, available, now)
	if err != nil {
		return nil, err
	}
	if available < settlement.AmountCents {
		return nil, ErrFundingSourceMismatch
	}

	remaining := settlement.AmountCents
	allocations := make([]FundingAllocation, 0, len(lots))
	for index := range lots {
		if remaining == 0 {
			break
		}
		lot := &lots[index]
		lotAvailable, err := fundingLotAvailableCents(*lot)
		if err != nil {
			return nil, err
		}
		if lotAvailable == 0 {
			continue
		}
		amount := lotAvailable
		if amount > remaining {
			amount = remaining
		}
		allocation, err := newFundingAllocation(settlement, *lot, amount, now)
		if err != nil {
			return nil, err
		}
		update := tx.Model(&FundingLot{}).
			Where("id = ? AND row_version = ?", lot.ID, lot.RowVersion).
			Updates(map[string]any{
				"allocated_amount_cents": gorm.Expr("allocated_amount_cents + ?", amount),
				"row_version":            gorm.Expr("row_version + 1"),
				"updated_at":             now,
			})
		if update.Error != nil {
			return nil, update.Error
		}
		if update.RowsAffected != 1 {
			return nil, ErrConcurrentUpdate
		}
		if err := tx.Create(&allocation).Error; err != nil {
			return nil, err
		}
		lot.AllocatedAmountCents += amount
		lot.RowVersion++
		lot.UpdatedAt = now
		allocations = append(allocations, allocation)
		remaining -= amount
	}
	if remaining != 0 {
		return nil, ErrFundingSourceMismatch
	}
	return allocations, nil
}

func restoreWalletFundingTx(
	tx *gorm.DB,
	wallet MerchantWallet,
	settlement FeeSettlement,
	now time.Time,
) error {
	if settlement.Source != SettlementWallet || settlement.MerchantUserID != wallet.MerchantUserID ||
		settlement.AmountCents == 0 {
		return ErrFundingSourceMismatch
	}
	var allocations []FundingAllocation
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("settlement_public_id = ?", settlement.SettlementPublicID).
		Order("id ASC").Find(&allocations).Error; err != nil {
		return err
	}
	if len(allocations) == 0 {
		createdAt := settlement.PaidAt
		if createdAt.IsZero() || createdAt.After(now) {
			createdAt = now
		}
		lot, err := createHistoricalFundingLotTx(
			tx, wallet, settlement.AmountCents, settlement.AmountCents,
			settlement.AmountCents, createdAt, now,
		)
		if err != nil {
			return err
		}
		allocation, err := newFundingAllocation(settlement, lot, settlement.AmountCents, createdAt)
		if err != nil {
			return err
		}
		allocation.RefundedAmountCents = allocation.AmountCents
		allocation.RefundedAt = timePointer(now)
		if err := validateFundingAllocation(allocation); err != nil {
			return err
		}
		return tx.Create(&allocation).Error
	}

	lotIDs := make([]string, 0, len(allocations))
	var allocatedTotal uint64
	var releaseTotal uint64
	for _, allocation := range allocations {
		if allocation.MerchantUserID != wallet.MerchantUserID ||
			allocation.RefundedAmountCents > allocation.AmountCents {
			return ErrFundingSourceMismatch
		}
		var err error
		allocatedTotal, err = addFundingAmount(allocatedTotal, allocation.AmountCents)
		if err != nil {
			return err
		}
		releaseTotal, err = addFundingAmount(
			releaseTotal, allocation.AmountCents-allocation.RefundedAmountCents,
		)
		if err != nil {
			return err
		}
		lotIDs = append(lotIDs, allocation.FundingLotPublicID)
	}
	if allocatedTotal != settlement.AmountCents || releaseTotal != settlement.AmountCents {
		return ErrFundingSourceMismatch
	}
	var lots []FundingLot
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("funding_lot_public_id IN ?", lotIDs).
		Order("id ASC").Find(&lots).Error; err != nil {
		return err
	}
	if len(lots) != len(lotIDs) {
		return ErrFundingSourceMismatch
	}
	lotsByPublicID := make(map[string]*FundingLot, len(lots))
	for index := range lots {
		lotsByPublicID[lots[index].FundingLotPublicID] = &lots[index]
	}
	for index := range allocations {
		allocation := &allocations[index]
		lot := lotsByPublicID[allocation.FundingLotPublicID]
		if lot == nil || lot.MerchantUserID != wallet.MerchantUserID ||
			lot.WalletPublicID != wallet.WalletPublicID {
			return ErrFundingSourceMismatch
		}
		delta := allocation.AmountCents - allocation.RefundedAmountCents
		if ^uint64(0)-lot.RefundedAmountCents < delta ||
			lot.RefundedAmountCents+delta > lot.AllocatedAmountCents {
			return ErrFundingSourceMismatch
		}
		lotUpdate := tx.Model(&FundingLot{}).
			Where("id = ? AND row_version = ?", lot.ID, lot.RowVersion).
			Updates(map[string]any{
				"refunded_amount_cents": gorm.Expr("refunded_amount_cents + ?", delta),
				"row_version":           gorm.Expr("row_version + 1"),
				"updated_at":            now,
			})
		if lotUpdate.Error != nil {
			return lotUpdate.Error
		}
		if lotUpdate.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		allocationUpdate := tx.Model(&FundingAllocation{}).
			Where("id = ? AND row_version = ?", allocation.ID, allocation.RowVersion).
			Updates(map[string]any{
				"refunded_amount_cents": allocation.AmountCents,
				"refunded_at":           now,
				"row_version":           gorm.Expr("row_version + 1"),
			})
		if allocationUpdate.Error != nil {
			return allocationUpdate.Error
		}
		if allocationUpdate.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		lot.RefundedAmountCents += delta
		lot.RowVersion++
		lot.UpdatedAt = now
	}
	return nil
}

func lockWalletFundingLotsTx(
	tx *gorm.DB,
	wallet MerchantWallet,
) ([]FundingLot, uint64, error) {
	var lots []FundingLot
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("wallet_public_id = ?", wallet.WalletPublicID).
		Order("created_at ASC, id ASC").Find(&lots).Error; err != nil {
		return nil, 0, err
	}
	var availableTotal uint64
	for _, lot := range lots {
		if lot.MerchantUserID != wallet.MerchantUserID {
			return nil, 0, ErrFundingSourceMismatch
		}
		available, err := fundingLotAvailableCents(lot)
		if err != nil {
			return nil, 0, err
		}
		availableTotal, err = addFundingAmount(availableTotal, available)
		if err != nil {
			return nil, 0, err
		}
	}
	return lots, availableTotal, nil
}

func ensureWalletFundingCoverageTx(
	tx *gorm.DB,
	wallet MerchantWallet,
	lots []FundingLot,
	available uint64,
	now time.Time,
) ([]FundingLot, uint64, error) {
	if available > wallet.AvailableBalanceCents {
		return nil, 0, ErrFundingSourceMismatch
	}
	if available == wallet.AvailableBalanceCents {
		return lots, available, nil
	}
	gap := wallet.AvailableBalanceCents - available
	createdAt := wallet.CreatedAt
	if createdAt.IsZero() || createdAt.After(now) {
		createdAt = now
	}
	historical, err := createHistoricalFundingLotTx(
		tx, wallet, gap, 0, 0, createdAt, now,
	)
	if err != nil {
		return nil, 0, err
	}
	lots = append(lots, historical)
	sort.SliceStable(lots, func(left, right int) bool {
		if lots[left].CreatedAt.Equal(lots[right].CreatedAt) {
			return lots[left].ID < lots[right].ID
		}
		return lots[left].CreatedAt.Before(lots[right].CreatedAt)
	})
	return lots, wallet.AvailableBalanceCents, nil
}

func createHistoricalFundingLotTx(
	tx *gorm.DB,
	wallet MerchantWallet,
	totalAmountCents uint64,
	allocatedAmountCents uint64,
	refundedAmountCents uint64,
	createdAt time.Time,
	updatedAt time.Time,
) (FundingLot, error) {
	publicID, err := NewULID()
	if err != nil {
		return FundingLot{}, err
	}
	lot := FundingLot{
		FundingLotPublicID:   publicID,
		WalletPublicID:       wallet.WalletPublicID,
		MerchantUserID:       wallet.MerchantUserID,
		Source:               FundingSourceHistoricalUnknown,
		VerificationStatus:   FundingVerificationUnverified,
		TotalAmountCents:     totalAmountCents,
		AllocatedAmountCents: allocatedAmountCents,
		RefundedAmountCents:  refundedAmountCents,
		Currency:             "CNY",
		RowVersion:           1,
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
	}
	if err := validateFundingLot(lot); err != nil {
		return FundingLot{}, err
	}
	if err := tx.Create(&lot).Error; err != nil {
		return FundingLot{}, err
	}
	return lot, nil
}

func newFundingAllocation(
	settlement FeeSettlement,
	lot FundingLot,
	amountCents uint64,
	createdAt time.Time,
) (FundingAllocation, error) {
	publicID, err := NewULID()
	if err != nil {
		return FundingAllocation{}, err
	}
	allocation := FundingAllocation{
		FundingAllocationPublicID: publicID,
		SettlementPublicID:        settlement.SettlementPublicID,
		FundingLotPublicID:        lot.FundingLotPublicID,
		MerchantUserID:            settlement.MerchantUserID,
		AmountCents:               amountCents,
		RowVersion:                1,
		CreatedAt:                 createdAt,
	}
	if lot.MerchantUserID != settlement.MerchantUserID {
		return FundingAllocation{}, ErrFundingSourceMismatch
	}
	if err := validateFundingAllocation(allocation); err != nil {
		return FundingAllocation{}, err
	}
	return allocation, nil
}

func addFundingAmount(left, right uint64) (uint64, error) {
	if ^uint64(0)-left < right {
		return 0, ErrAmountOverflow
	}
	return left + right, nil
}
