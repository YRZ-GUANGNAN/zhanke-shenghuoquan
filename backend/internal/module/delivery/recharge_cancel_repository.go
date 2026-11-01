package delivery

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RequestRechargeCancellation records intent before any provider operation. It
// brings verification forward without clearing or taking over a live lease.
func (r *GormRepository) RequestRechargeCancellation(ctx context.Context, publicID string, merchantUserID uint64, now time.Time) (PaymentAttempt, error) {
	return r.requestRechargeCancellation(ctx, publicID, merchantUserID, 0, now)
}

func (r *GormRepository) FindRechargeByRequestOwned(ctx context.Context, merchantUserID uint64, clientRequestID string, amountCents uint64) (*PaymentAttempt, error) {
	var value PaymentAttempt
	err := r.db.WithContext(ctx).Where("merchant_user_id = ? AND purpose = ? AND client_request_id = ?", merchantUserID, PaymentWalletRecharge, clientRequestID).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if value.Provider != PaymentWechat || value.AmountCents != amountCents {
		return nil, ErrIdempotencyConflict
	}
	return &value, nil
}

// An absent request gets an already-closed tombstone under the original unique
// request key. A delayed create can only recover this terminal record, never
// issue a prepay; if create won, the existing request is cancelled normally.
func (r *GormRepository) RequestRechargeCancellationByRequest(ctx context.Context, candidate *PaymentAttempt, now time.Time) (PaymentAttempt, error) {
	if candidate.Purpose != PaymentWalletRecharge || candidate.Provider != PaymentWechat {
		return PaymentAttempt{}, ErrInvalidRecord
	}
	candidate.Status = PaymentClosed
	candidate.CancelRequestedAt, candidate.ClosedAt = timePointer(now), timePointer(now)
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(candidate).Error; err != nil {
		return PaymentAttempt{}, err
	}
	value, err := r.FindRechargeByRequestOwned(ctx, candidate.MerchantUserID, candidate.ClientRequestID, candidate.AmountCents)
	if err != nil {
		return PaymentAttempt{}, err
	}
	if value == nil {
		return PaymentAttempt{}, ErrNotFound
	}
	return r.requestRechargeCancellation(ctx, value.PaymentPublicID, candidate.MerchantUserID, candidate.AmountCents, now)
}

func (r *GormRepository) requestRechargeCancellation(ctx context.Context, publicID string, merchantUserID, expectedAmount uint64, now time.Time) (PaymentAttempt, error) {
	var result PaymentAttempt
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("payment_public_id = ? AND merchant_user_id = ?", publicID, merchantUserID).First(&result).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPaymentForbidden
			}
			return err
		}
		if result.Purpose != PaymentWalletRecharge || result.Provider != PaymentWechat {
			return ErrPaymentForbidden
		}
		if expectedAmount != 0 && result.AmountCents != expectedAmount {
			return ErrIdempotencyConflict
		}
		switch result.Status {
		case PaymentPaid, PaymentClosed, PaymentFailed:
			return nil
		case PaymentCreated, PaymentPrepayIssued:
		default:
			return ErrPaymentNotPending
		}
		activeLease := result.VerifyLeaseUntil != nil && result.VerifyLeaseUntil.After(now)
		if result.CancelRequestedAt != nil && activeLease {
			return nil
		}
		updates := map[string]any{
			"next_verify_at": now, "updated_at": now, "row_version": gorm.Expr("row_version + 1"),
		}
		if result.CancelRequestedAt == nil {
			updates["cancel_requested_at"] = now
			result.CancelRequestedAt = timePointer(now)
		}
		// MarkPrepayRequested takes this same row lock before making any network
		// request. Only this untouched state proves no WeChat order can exist.
		unissued := result.Status == PaymentCreated && result.PayerOpenID == nil && result.PrepayID == nil && result.ProviderStatus == ""
		if unissued {
			updates["status"], updates["closed_at"] = PaymentClosed, now
			updates["next_verify_at"] = nil
			result.Status, result.ClosedAt, result.NextVerifyAt = PaymentClosed, timePointer(now), nil
		} else {
			result.NextVerifyAt = timePointer(now)
		}
		update := tx.Model(&PaymentAttempt{}).
			Where("id = ? AND row_version = ? AND status IN ?", result.ID, result.RowVersion, []PaymentStatus{PaymentCreated, PaymentPrepayIssued}).Updates(updates)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		result.RowVersion++
		result.UpdatedAt = now
		return nil
	})
	return result, err
}
