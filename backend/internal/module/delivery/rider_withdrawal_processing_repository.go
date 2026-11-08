package delivery

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func validateWithdrawalAction(kind string, a RiderWithdrawalAction, now time.Time) error {
	if !ValidULID(a.WithdrawPublicID) || !ValidULID(a.IdempotencyKey) || a.OperatorUserID == 0 || a.ExpectedRowVersion == 0 || now.IsZero() ||
		len([]rune(a.Reason)) > 256 || len([]rune(a.PaymentReference)) > 128 {
		return ErrInvalidRecord
	}
	switch kind {
	case "claim":
		if !ValidULID(a.ClaimKey) || a.ConfirmedUnpaid || a.PaymentReference != "" || a.Reason != "" {
			return ErrInvalidRecord
		}
	case "release":
		if !ValidULID(a.ClaimKey) || !a.ConfirmedUnpaid || a.PaymentReference != "" {
			return ErrWithdrawalClaimConflict
		}
		return validateWithdrawalReason(a.Reason)
	case "paid":
		if !ValidULID(a.ClaimKey) {
			return ErrWithdrawalNotClaimed
		}
		if !actualWithdrawalPaymentReference(a.PaymentReference) {
			return ErrWithdrawalPaymentRequired
		}
		if a.ConfirmedUnpaid || a.Reason != "" {
			return ErrInvalidRecord
		}
	case "resolve_paid":
		if !ValidULID(a.ClaimKey) {
			return ErrWithdrawalNotClaimed
		}
		if !actualWithdrawalPaymentReference(a.PaymentReference) {
			return ErrWithdrawalPaymentRequired
		}
		if a.ConfirmedUnpaid || a.Reason != "" {
			return ErrInvalidRecord
		}
	case "resolve_unpaid":
		if !ValidULID(a.ClaimKey) || !a.ConfirmedUnpaid || a.PaymentReference != "" {
			return ErrWithdrawalClaimConflict
		}
		return validateWithdrawalReason(a.Reason)
	case "reject":
		if a.ClaimKey != "" || a.PaymentReference != "" || a.ConfirmedUnpaid {
			return ErrInvalidRecord
		}
		return validateWithdrawalReason(a.Reason)
	case "exception":
		if !ValidULID(a.ClaimKey) || a.ConfirmedUnpaid {
			return ErrWithdrawalClaimConflict
		}
		return validateWithdrawalReason(a.Reason)
	default:
		return ErrInvalidRecord
	}
	return nil
}

func withdrawalActionHash(kind string, a RiderWithdrawalAction) string {
	encoded, _ := json.Marshal(struct {
		Kind   string
		Action RiderWithdrawalAction
	}{kind, a})
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}

// An unresolved payment result is evidence to retain, not a transfer receipt
// that can authorize completion or reserve a receipt shared by every rider.
func actualWithdrawalPaymentReference(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && value != "结果未知"
}

// The owning wallet is already locked. The two unique indexes make the
// binding atomic across different wallets without locking unrelated riders'
// reference rows. A receipt can never be reassigned after it is captured.
func bindWithdrawalPaymentReferenceTx(tx *gorm.DB, withdrawalID, reference string, now time.Time) error {
	if !actualWithdrawalPaymentReference(reference) {
		return ErrWithdrawalPaymentRequired
	}
	var existing RiderWithdrawalPaymentReference
	err := tx.Where("withdraw_public_id = ?", withdrawalID).First(&existing).Error
	if err == nil {
		if existing.PaymentReference != reference {
			return ErrWithdrawalPaymentReferenceConflict
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	value := RiderWithdrawalPaymentReference{PaymentReference: reference, WithdrawPublicID: withdrawalID, CreatedAt: now}
	if err := tx.Create(&value).Error; err != nil {
		if isDuplicateKey(err) {
			return ErrWithdrawalPaymentReferenceConflict
		}
		return err
	}
	return nil
}

func (r *GormRepository) ReadRiderWithdrawalClaim(ctx context.Context, id string) (RiderWithdrawalClaim, error) {
	if !ValidULID(id) {
		return RiderWithdrawalClaim{}, ErrInvalidRecord
	}
	var value RiderWithdrawalClaim
	err := r.db.WithContext(ctx).Where("withdraw_public_id = ?", id).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return value, nil
	}
	return value, err
}

func (r *GormRepository) ProcessRiderWithdrawal(ctx context.Context, kind string, a RiderWithdrawalAction, now time.Time) (RiderWithdrawal, error) {
	a.Reason = strings.TrimSpace(a.Reason)
	a.PaymentReference = strings.TrimSpace(a.PaymentReference)
	if err := validateWithdrawalAction(kind, a, now); err != nil {
		return RiderWithdrawal{}, err
	}
	if err := r.RequireWithdrawalSchema(); err != nil {
		return RiderWithdrawal{}, err
	}
	var candidate RiderWithdrawal
	if err := r.db.WithContext(ctx).Where("withdraw_public_id = ?", a.WithdrawPublicID).First(&candidate).Error; err != nil {
		return RiderWithdrawal{}, normalizeWithdrawalNotFound(err)
	}
	requestHash := withdrawalActionHash(kind, a)
	var result RiderWithdrawal
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wallet, err := lockOrCreateRiderWalletTx(tx, candidate.RiderUserID, now)
		if err != nil {
			return err
		}
		// An exception record must remain writable even if the source has changed.
		// It never releases allocations, credits money, or records a successful pay.
		var earnings []RiderEarningRecord
		var allocations []RiderWithdrawalAllocation
		if kind != "exception" && kind != "resolve_paid" && kind != "resolve_unpaid" {
			earnings, allocations, err = withdrawalBookTx(tx, wallet, true)
			if err != nil {
				return err
			}
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("withdraw_public_id = ?", a.WithdrawPublicID).First(&result).Error; err != nil {
			return normalizeWithdrawalNotFound(err)
		}
		if result.RiderUserID != wallet.RiderUserID || result.WalletPublicID != wallet.WalletPublicID {
			return ErrWithdrawalSourceMismatch
		}
		var previous RiderWithdrawalEvent
		err = tx.Where("idempotency_key = ?", a.IdempotencyKey).First(&previous).Error
		if err == nil {
			if previous.RequestHash != requestHash || previous.WithdrawPublicID != result.WithdrawPublicID {
				return ErrIdempotencyConflict
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if result.Status != RiderWithdrawalProcessing {
			return ErrWithdrawalAlreadyProcessed
		}
		if result.RowVersion != a.ExpectedRowVersion {
			return ErrConcurrentUpdate
		}
		var claim RiderWithdrawalClaim
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("withdraw_public_id = ?", result.WithdrawPublicID).First(&claim).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		claimed := claim.ID != 0 && claim.Status != RiderWithdrawalClaimReleased
		switch kind {
		case "claim", "reject":
			if claimed {
				return ErrWithdrawalClaimConflict
			}
		case "release", "paid", "exception", "resolve_paid", "resolve_unpaid":
			if !claimed || claim.OperatorUserID != a.OperatorUserID || claim.ClaimKey != a.ClaimKey {
				return ErrWithdrawalNotClaimed
			}
			validClaim := claim.Status == RiderWithdrawalClaimActive
			if kind == "exception" {
				validClaim = validClaim || claim.Status == RiderWithdrawalClaimException
			}
			if kind == "resolve_paid" || kind == "resolve_unpaid" {
				validClaim = claim.Status == RiderWithdrawalClaimException
			}
			if !validClaim {
				return ErrWithdrawalClaimConflict
			}
		}
		if kind == "claim" || kind == "paid" {
			if err := verifyWithdrawalAllocationsTx(tx, wallet, result, earnings, allocations, now); err != nil {
				return err
			}
		}
		if actualWithdrawalPaymentReference(claim.PaymentReference) && a.PaymentReference != "" && a.PaymentReference != claim.PaymentReference {
			return ErrWithdrawalPaymentReferenceConflict
		}
		if (kind == "paid" || kind == "resolve_paid" || kind == "exception") && actualWithdrawalPaymentReference(a.PaymentReference) {
			if err := bindWithdrawalPaymentReferenceTx(tx, result.WithdrawPublicID, a.PaymentReference, now); err != nil {
				return err
			}
		}
		eventType := RiderWithdrawalEventClaimed
		switch kind {
		case "claim":
			if claim.ID == 0 {
				id, err := NewULID()
				if err != nil {
					return err
				}
				claim = RiderWithdrawalClaim{ClaimPublicID: id, WithdrawPublicID: result.WithdrawPublicID, CreatedAt: now}
			}
			claim.OperatorUserID, claim.ClaimKey, claim.IdempotencyKey = a.OperatorUserID, a.ClaimKey, a.IdempotencyKey
			claim.ExpectedRowVersion, claim.Status, claim.Reason, claim.PaymentReference = a.ExpectedRowVersion, RiderWithdrawalClaimActive, "", ""
		case "release":
			eventType, claim.Status, claim.Reason = RiderWithdrawalEventReleased, RiderWithdrawalClaimReleased, a.Reason
		case "exception":
			eventType, claim.Status, claim.Reason = RiderWithdrawalEventException, RiderWithdrawalClaimException, a.Reason
			// Never erase an already captured payment reference on a later report.
			if a.PaymentReference != "" {
				claim.PaymentReference = a.PaymentReference
			}
		case "resolve_paid":
			result.Status, eventType = RiderWithdrawalPaid, RiderWithdrawalEventPaid
			// An exception may intentionally retain the placeholder "结果未知".
			// Resolution is the point at which an operator supplies the actual
			// receipt; an already captured real receipt is immutable and cannot be
			// replaced by a different value.
			if claim.PaymentReference == "" ||
				(actualWithdrawalPaymentReference(claim.PaymentReference) && claim.PaymentReference != a.PaymentReference) {
				return ErrWithdrawalPaymentRequired
			}
			claim.Status, claim.PaymentReference = RiderWithdrawalClaimCompleted, a.PaymentReference
			if update := tx.Model(&RiderWithdrawalAllocation{}).Where("withdraw_public_id = ? AND status = ?", result.WithdrawPublicID, RiderWithdrawalAllocationInFlight).
				Updates(map[string]any{"status": RiderWithdrawalAllocationPaid, "updated_at": now}); update.Error != nil {
				return update.Error
			} else if update.RowsAffected == 0 {
				return ErrWithdrawalSourceMismatch
			}
			result.ProcessedBy, result.ProcessKey, result.ProcessVersion = &a.OperatorUserID, &a.IdempotencyKey, &a.ExpectedRowVersion
			result.ProcessedAt, result.Reason, result.PaymentReference = &now, a.Reason, a.PaymentReference
		case "resolve_unpaid":
			// Only an unresolved placeholder may be confirmed unpaid.  A real
			// receipt is durable evidence and must never be released or credited
			// back through this path.
			if actualWithdrawalPaymentReference(claim.PaymentReference) {
				return ErrWithdrawalClaimConflict
			}
			result.Status, eventType = RiderWithdrawalRejected, RiderWithdrawalEventRejected
			claim.Status, claim.Reason = RiderWithdrawalClaimReleased, a.Reason
			if _, err := addRiderMoney(wallet.AvailableBalanceCents, result.AmountCents); err != nil {
				return err
			}
			ledger, err := newRiderLedger(wallet, RiderLedgerCredit, RiderLedgerAdjustment, result.AmountCents, "rider_withdrawal", result.WithdrawPublicID, a.IdempotencyKey, now)
			if err != nil {
				return err
			}
			if err := creditRiderWalletTx(tx, &wallet, result.AmountCents, now); err != nil {
				return err
			}
			if err := tx.Create(&ledger).Error; err != nil {
				return err
			}
			update := tx.Model(&RiderWithdrawalAllocation{}).Where("withdraw_public_id = ? AND status = ?", result.WithdrawPublicID, RiderWithdrawalAllocationInFlight).
				Updates(map[string]any{"status": RiderWithdrawalAllocationReleased, "updated_at": now})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected == 0 {
				return ErrWithdrawalSourceMismatch
			}
			result.ProcessedBy, result.ProcessKey, result.ProcessVersion = &a.OperatorUserID, &a.IdempotencyKey, &a.ExpectedRowVersion
			result.ProcessedAt, result.Reason, result.PaymentReference = &now, a.Reason, ""
		case "paid", "reject":
			allocationStatus := RiderWithdrawalAllocationPaid
			result.Status, eventType = RiderWithdrawalPaid, RiderWithdrawalEventPaid
			if kind == "reject" {
				allocationStatus, result.Status, eventType = RiderWithdrawalAllocationReleased, RiderWithdrawalRejected, RiderWithdrawalEventRejected
				if _, err := addRiderMoney(wallet.AvailableBalanceCents, result.AmountCents); err != nil {
					return err
				}
				ledger, err := newRiderLedger(wallet, RiderLedgerCredit, RiderLedgerAdjustment, result.AmountCents, "rider_withdrawal", result.WithdrawPublicID, a.IdempotencyKey, now)
				if err != nil {
					return err
				}
				if err := creditRiderWalletTx(tx, &wallet, result.AmountCents, now); err != nil {
					return err
				}
				if err := tx.Create(&ledger).Error; err != nil {
					return err
				}
			} else {
				claim.Status, claim.PaymentReference = RiderWithdrawalClaimCompleted, a.PaymentReference
			}
			update := tx.Model(&RiderWithdrawalAllocation{}).Where("withdraw_public_id = ? AND status = ?", result.WithdrawPublicID, RiderWithdrawalAllocationInFlight).
				Updates(map[string]any{"status": allocationStatus, "updated_at": now})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected == 0 {
				return ErrWithdrawalSourceMismatch
			}
			result.ProcessedBy, result.ProcessKey, result.ProcessVersion = &a.OperatorUserID, &a.IdempotencyKey, &a.ExpectedRowVersion
			result.ProcessedAt, result.Reason, result.PaymentReference = &now, a.Reason, a.PaymentReference
		}
		if kind != "reject" {
			claim.UpdatedAt = now
			if err := tx.Save(&claim).Error; err != nil {
				if isDuplicateKey(err) {
					return ErrIdempotencyConflict
				}
				return err
			}
		}
		result.RowVersion++
		result.UpdatedAt = now
		update := tx.Model(&RiderWithdrawal{}).Where("id = ? AND row_version = ?", result.ID, a.ExpectedRowVersion).
			Select("status", "reason", "payment_reference", "processed_by", "process_key", "process_version", "processed_at", "row_version", "updated_at").Updates(&result)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		id, err := NewULID()
		if err != nil {
			return err
		}
		event := RiderWithdrawalEvent{EventPublicID: id, WithdrawPublicID: result.WithdrawPublicID, EventType: eventType, OperatorUserID: a.OperatorUserID,
			Reason: a.Reason, PaymentReference: a.PaymentReference, ClaimKey: a.ClaimKey, IdempotencyKey: a.IdempotencyKey, RequestHash: requestHash, ExpectedRowVersion: a.ExpectedRowVersion, CreatedAt: now}
		if err := tx.Create(&event).Error; err != nil {
			if isDuplicateKey(err) {
				return ErrIdempotencyConflict
			}
			return err
		}
		if kind == "paid" || kind == "reject" || kind == "resolve_paid" || kind == "resolve_unpaid" {
			// Persist the financial transition and notification intent together.
			// This appends only to the local outbox, never to an external provider.
			// An append failure rolls back the record, leaving the claim and funds
			// occupied so the same command can safely be retried, not paid again.
			return withdrawalNotificationWithIDTx(tx, r.notificationAppender, result, event.EventPublicID, now)
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return result, err
}

func verifyWithdrawalAllocationsTx(tx *gorm.DB, wallet RiderWallet, withdrawal RiderWithdrawal, earnings []RiderEarningRecord, allocations []RiderWithdrawalAllocation, now time.Time) error {
	byID := map[string]RiderEarningRecord{}
	for _, e := range earnings {
		byID[e.EarningPublicID] = e
	}
	selected := make([]RiderEarningRecord, 0)
	for _, a := range allocations {
		if a.WithdrawPublicID == withdrawal.WithdrawPublicID {
			selected = append(selected, byID[a.EarningPublicID])
		}
	}
	if len(selected) == 0 {
		return ErrWithdrawalSourceMismatch
	}
	if err := lockWithdrawalSourcesTx(tx, selected); err != nil {
		return err
	}
	for _, a := range allocations {
		if a.WithdrawPublicID != withdrawal.WithdrawPublicID {
			continue
		}
		e := byID[a.EarningPublicID]
		if e.SettlementStatus != RiderEarningSettled || e.PaidOfflineAt != nil || e.RowVersion != a.EarningRowVersion || e.SettledAt == nil || e.SettledAt.After(now) || e.SettleAt.IsZero() || e.SettleAt.After(now) {
			return ErrWithdrawalSourceMismatch
		}
		if _, active, err := activeRiderPayoutReservationTx(tx, e.EarningPublicID); err != nil {
			return err
		} else if active {
			return ErrWithdrawalSourceMismatch
		}
		source, err := verifiedWithdrawalEarningTx(tx, wallet, e)
		if err != nil {
			return err
		}
		if source.Policy != a.SourcePolicyVersion || source.EvidenceHash != a.SourceEvidenceHash {
			return ErrWithdrawalSourceMismatch
		}
	}
	return nil
}
