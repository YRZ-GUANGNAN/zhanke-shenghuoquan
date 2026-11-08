package delivery

import (
	"errors"
	"gorm.io/gorm"
)

// Every state transition has one immutable audit entry with its prior version.
// Refuse orphan/missing history instead of guessing who may have transferred.
func checkWithdrawalAuditTx(tx *gorm.DB, w RiderWithdrawal) error {
	var events []RiderWithdrawalEvent
	if err := tx.Where("withdraw_public_id = ?", w.WithdrawPublicID).Order("id ASC").Find(&events).Error; err != nil {
		return err
	}
	if w.RowVersion == 0 || uint64(len(events)) != w.RowVersion-1 {
		return ErrWithdrawalSourceMismatch
	}
	state := RiderWithdrawalClaimReleased
	key := ""
	var operator uint64
	for i, event := range events {
		if event.ExpectedRowVersion != uint64(i+1) || !ValidULID(event.IdempotencyKey) || event.OperatorUserID == 0 || len(event.RequestHash) != 64 {
			return ErrWithdrawalSourceMismatch
		}
		switch event.EventType {
		case RiderWithdrawalEventClaimed:
			if state != RiderWithdrawalClaimReleased || !ValidULID(event.ClaimKey) {
				return ErrWithdrawalSourceMismatch
			}
			state, key, operator = RiderWithdrawalClaimActive, event.ClaimKey, event.OperatorUserID
		case RiderWithdrawalEventReleased, RiderWithdrawalEventPaid, RiderWithdrawalEventException:
			validTransition := state == RiderWithdrawalClaimActive
			if state == RiderWithdrawalClaimException && (event.EventType == RiderWithdrawalEventException || event.EventType == RiderWithdrawalEventPaid) {
				validTransition = true
			}
			if !validTransition || event.ClaimKey != key || event.OperatorUserID != operator {
				return ErrWithdrawalSourceMismatch
			}
			switch event.EventType {
			case RiderWithdrawalEventReleased:
				state = RiderWithdrawalClaimReleased
			case RiderWithdrawalEventPaid:
				state = RiderWithdrawalClaimCompleted
			case RiderWithdrawalEventException:
				state = RiderWithdrawalClaimException
			}
		case RiderWithdrawalEventRejected:
			if (state != RiderWithdrawalClaimReleased && state != RiderWithdrawalClaimException) || i != len(events)-1 || w.Status != RiderWithdrawalRejected {
				return ErrWithdrawalSourceMismatch
			}
			if state == RiderWithdrawalClaimException && (event.ClaimKey != key || event.OperatorUserID != operator) {
				return ErrWithdrawalSourceMismatch
			}
			state = RiderWithdrawalClaimReleased
		default:
			return ErrWithdrawalSourceMismatch
		}
	}
	var claim RiderWithdrawalClaim
	err := tx.Where("withdraw_public_id = ?", w.WithdrawPublicID).First(&claim).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if key != "" {
		if claim.ID == 0 || claim.Status != state || claim.OperatorUserID != operator || claim.ClaimKey != key {
			return ErrWithdrawalSourceMismatch
		}
	} else if claim.ID != 0 {
		return ErrWithdrawalSourceMismatch
	}
	if w.Status == RiderWithdrawalPaid {
		if len(events) == 0 || state != RiderWithdrawalClaimCompleted || events[len(events)-1].EventType != RiderWithdrawalEventPaid {
			return ErrWithdrawalSourceMismatch
		}
	} else if w.Status == RiderWithdrawalProcessing && state == RiderWithdrawalClaimCompleted {
		return ErrWithdrawalSourceMismatch
	}
	if w.Status != RiderWithdrawalProcessing {
		if len(events) == 0 || w.ProcessKey == nil || w.ProcessVersion == nil || w.ProcessedBy == nil {
			return ErrWithdrawalSourceMismatch
		}
		last := events[len(events)-1]
		if *w.ProcessKey != last.IdempotencyKey || *w.ProcessVersion != last.ExpectedRowVersion || *w.ProcessedBy != last.OperatorUserID || w.PaymentReference != last.PaymentReference || w.Reason != last.Reason {
			return ErrWithdrawalSourceMismatch
		}
	}
	return nil
}
