package delivery

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Do not lock orders here: delivery completion already holds order -> rider
// wallet. Refund callbacks hold refund -> settlement, so never lock refunds
// either. READ COMMITTED re-reads them after the settlement lock is acquired.
// Recharge callbacks lock payment -> merchant wallet -> lot. All payments
// (including wallet recharge sources) must therefore precede all lots.
func lockWithdrawalSourcesTx(tx *gorm.DB, earnings []RiderEarningRecord) error {
	orderIDs := make([]string, 0, len(earnings))
	for _, e := range earnings {
		orderIDs = append(orderIDs, e.DeliveryOrderPublicID)
	}
	if len(orderIDs) == 0 {
		return nil
	}
	sort.Strings(orderIDs)
	var settlements []FeeSettlement
	for _, id := range orderIDs {
		var rows []FeeSettlement
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("delivery_order_public_id = ?", id).Find(&rows).Error; err != nil {
			return err
		}
		settlements = append(settlements, rows...)
	}
	settlementIDs := make([]string, 0, len(settlements))
	paymentIDs := map[string]bool{}
	for _, s := range settlements {
		settlementIDs = append(settlementIDs, s.SettlementPublicID)
		if s.PaymentPublicID != nil {
			paymentIDs[*s.PaymentPublicID] = true
		}
	}
	if len(settlementIDs) == 0 {
		return nil
	}
	var allocations []FundingAllocation
	if err := tx.Where("settlement_public_id IN ?", settlementIDs).Order("id ASC").Find(&allocations).Error; err != nil {
		return err
	}
	lotIDs := map[string]bool{}
	for _, a := range allocations {
		lotIDs[a.FundingLotPublicID] = true
	}
	// Source identities are immutable in normal money paths. Read them first
	// without a lock solely to discover the payment lock set.
	for _, id := range sortedWithdrawalKeys(lotIDs) {
		var rows []FundingLot
		if err := tx.Where("funding_lot_public_id = ?", id).Find(&rows).Error; err != nil {
			return err
		}
		for _, lot := range rows {
			if lot.PaymentPublicID != nil {
				paymentIDs[*lot.PaymentPublicID] = true
			}
		}
	}
	for _, id := range sortedWithdrawalKeys(paymentIDs) {
		var rows []PaymentAttempt
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("payment_public_id = ?", id).Find(&rows).Error; err != nil {
			return err
		}
	}
	var current []FundingAllocation
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("settlement_public_id IN ?", settlementIDs).Order("id ASC").Find(&current).Error; err != nil {
		return err
	}
	if len(current) != len(allocations) {
		return ErrWithdrawalSourceMismatch
	}
	for i, a := range current {
		if a.ID != allocations[i].ID || a.FundingLotPublicID != allocations[i].FundingLotPublicID {
			return ErrWithdrawalSourceMismatch
		}
	}
	if len(lotIDs) > 0 {
		var rows []FundingLot
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("funding_lot_public_id IN ?", sortedWithdrawalKeys(lotIDs)).Order("id ASC").Find(&rows).Error; err != nil {
			return err
		}
		for _, lot := range rows {
			if lot.PaymentPublicID != nil && !paymentIDs[*lot.PaymentPublicID] {
				return ErrWithdrawalSourceMismatch
			}
		}
	}
	return nil
}

func sortedWithdrawalKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func verifiedWithdrawalEarningTx(tx *gorm.DB, wallet RiderWallet, earning RiderEarningRecord) (RiderPayoutSourceSnapshot, error) {
	source, err := verifyRiderEarningFundingTx(tx, earning)
	if err != nil {
		return source, err
	}
	if earning.RiderUserID != wallet.RiderUserID || earning.RiderFeeCents == 0 || earning.RiderFeeCents > MaxRiderMoneyAmount {
		return source, ErrWithdrawalSourceMismatch
	}
	var ledgers []RiderLedger
	if err := tx.Where("idempotency_key = ?", earning.DeliveryOrderPublicID).Find(&ledgers).Error; err != nil {
		return source, err
	}
	if len(ledgers) != 1 {
		return source, ErrWithdrawalSourceMismatch
	}
	l := ledgers[0]
	after, err := addRiderMoney(l.BalanceBeforeCents, l.AmountCents)
	if err != nil || after != l.BalanceAfterCents || l.RiderUserID != wallet.RiderUserID || l.WalletPublicID != wallet.WalletPublicID ||
		l.Direction != RiderLedgerCredit || l.EntryType != RiderLedgerIncome || l.AmountCents != earning.RiderFeeCents ||
		l.ReferenceType != "delivery_order" || l.ReferencePublicID != earning.DeliveryOrderPublicID {
		return source, ErrWithdrawalSourceMismatch
	}
	var settlement FeeSettlement
	if err := tx.Where("delivery_order_public_id = ?", earning.DeliveryOrderPublicID).First(&settlement).Error; err != nil {
		return source, err
	}
	// Hash only the source facts for this order. Do not include a shared lot's
	// running allocation totals/version: unrelated deliveries legitimately change
	// them and must not invalidate an already allocated withdrawal.
	parts := []any{source.EvidenceHash, settlement.SettlementPublicID, settlement.RowVersion, l.LedgerPublicID,
		l.AmountCents, earning.EarningPublicID, earning.RowVersion}
	if settlement.Source == SettlementWallet {
		var allocations []FundingAllocation
		if err := tx.Where("settlement_public_id = ?", settlement.SettlementPublicID).Order("id ASC").Find(&allocations).Error; err != nil {
			return source, err
		}
		for _, a := range allocations {
			var lot FundingLot
			if err := tx.Where("funding_lot_public_id = ?", a.FundingLotPublicID).First(&lot).Error; err != nil {
				return source, err
			}
			var recharge PaymentAttempt
			if lot.PaymentPublicID == nil {
				return source, ErrWithdrawalSourceMismatch
			}
			if err := tx.Where("payment_public_id = ?", *lot.PaymentPublicID).First(&recharge).Error; err != nil {
				return source, err
			}
			if err := validateFundingAllocation(a); err != nil {
				return source, ErrWithdrawalSourceMismatch
			}
			if err := validateFundingLot(lot); err != nil {
				return source, ErrWithdrawalSourceMismatch
			}
			if recharge.AmountCents != lot.TotalAmountCents || lot.TotalAmountCents > MaxRiderMoneyAmount {
				return source, ErrWithdrawalSourceMismatch
			}
			var merchantWallet MerchantWallet
			if err := tx.Where("wallet_public_id = ?", lot.WalletPublicID).First(&merchantWallet).Error; err != nil {
				return source, normalizeNotFound(err)
			}
			if merchantWallet.MerchantUserID != settlement.MerchantUserID || lot.AllocatedAmountCents-lot.RefundedAmountCents < a.AmountCents {
				return source, ErrWithdrawalSourceMismatch
			}
			parts = append(parts, a.FundingAllocationPublicID, a.RowVersion, a.AmountCents, lot.FundingLotPublicID, lot.WalletPublicID, lot.TotalAmountCents)
		}
	}
	encoded, err := json.Marshal(parts)
	if err != nil {
		return source, err
	}
	hash := sha256.Sum256(encoded)
	source.Policy = "rider-withdrawal-source-v1"
	source.EvidenceHash = hex.EncodeToString(hash[:])
	return source, nil
}
