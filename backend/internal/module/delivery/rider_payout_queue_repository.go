package delivery

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RiderPayoutCandidateCursor struct {
	AfterID uint64
	Limit   int
}

type RiderPayoutSummary struct {
	ReservedCents   uint64
	UnverifiedCents uint64
}

type RiderPayoutQueueRepository interface {
	ListRiderPayoutCandidates(context.Context, RiderPayoutCandidateCursor) ([]RiderEarningRecord, error)
	ReconcileRiderPayoutQueue(context.Context, string, time.Time) (RiderPayoutQueueResult, error)
	CancelRiderPayoutQueue(context.Context, string, uint64, uint64, string, string, time.Time) (RiderPayoutQueueResult, error)
	ReadRiderPayoutSummary(context.Context, uint64) (RiderPayoutSummary, error)
}

var _ RiderPayoutQueueRepository = (*GormRepository)(nil)

func (r *GormRepository) ListRiderPayoutCandidates(ctx context.Context, cursor RiderPayoutCandidateCursor) ([]RiderEarningRecord, error) {
	limit := cursor.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := r.db.WithContext(ctx).Where("settlement_status IN ? AND rider_fee_cents > ?", []RiderEarningStatus{
		RiderEarningPending, RiderEarningSettled,
	}, 0).Order("id ASC").Limit(limit)
	if cursor.AfterID > 0 {
		query = query.Where("id > ?", cursor.AfterID)
	}
	var values []RiderEarningRecord
	if err := query.Find(&values).Error; err != nil {
		return nil, err
	}
	return values, nil
}

func (r *GormRepository) ReadRiderPayoutSummary(ctx context.Context, riderUserID uint64) (RiderPayoutSummary, error) {
	if riderUserID == 0 {
		return RiderPayoutSummary{}, ErrInvalidRecord
	}
	if !r.db.Migrator().HasTable(&RiderPayoutReservation{}) {
		return RiderPayoutSummary{}, ErrPayoutQueueUnavailable
	}
	var result RiderPayoutSummary
	if err := r.db.WithContext(ctx).Model(&RiderPayoutReservation{}).
		Where("rider_user_id = ? AND status = ?", riderUserID, RiderPayoutReservationActive).
		Select("COALESCE(SUM(amount_cents), 0) AS reserved_cents").Scan(&result).Error; err != nil {
		return RiderPayoutSummary{}, err
	}
	// A missing or unverifiable source is deliberately counted from the queue,
	// not inferred from the earning status alone.
	if !r.db.Migrator().HasTable(&RiderPayoutQueue{}) {
		return RiderPayoutSummary{}, ErrPayoutQueueUnavailable
	}
	if err := r.db.WithContext(ctx).Model(&RiderPayoutQueue{}).
		Where("rider_user_id = ? AND status = ? AND amount_cents > 0", riderUserID, RiderPayoutQueueBlocked).
		Select("COALESCE(SUM(amount_cents), 0) AS unverified_cents").Scan(&result).Error; err != nil {
		return RiderPayoutSummary{}, err
	}
	return result, nil
}

func (r *GormRepository) ReconcileRiderPayoutQueue(ctx context.Context, earningPublicID string, now time.Time) (RiderPayoutQueueResult, error) {
	if !ValidULID(earningPublicID) || now.IsZero() {
		return RiderPayoutQueueResult{}, ErrInvalidRecord
	}
	if !r.queueTablesAvailable() {
		return RiderPayoutQueueResult{}, ErrPayoutQueueUnavailable
	}
	// Read the rider without locking first. The transaction below then uses a
	// single lock order shared with offline payouts and cancellation.
	var candidate RiderEarningRecord
	if err := r.db.WithContext(ctx).Where("earning_public_id = ?", earningPublicID).First(&candidate).Error; err != nil {
		return RiderPayoutQueueResult{}, normalizeNotFound(err)
	}
	var result RiderPayoutQueueResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		wallet, err := lockRiderWalletForQueueTx(tx, candidate.RiderUserID, now)
		if err != nil {
			return err
		}
		var earning RiderEarningRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", candidate.ID).First(&earning).Error; err != nil {
			return normalizeNotFound(err)
		}
		var existing RiderPayoutQueue
		queueErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("earning_public_id = ?", earning.EarningPublicID).First(&existing).Error
		if queueErr == nil {
			if existing.AmountCents != earning.RiderFeeCents || existing.RiderUserID != earning.RiderUserID || existing.DeliveryOrderPublicID != earning.DeliveryOrderPublicID {
				return ErrIdempotencyConflict
			}
			var reservation RiderPayoutReservation
			resErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("payout_queue_public_id = ?", existing.PayoutQueuePublicID).First(&reservation).Error
			if errors.Is(resErr, gorm.ErrRecordNotFound) {
				resErr = nil
			}
			if resErr != nil {
				return resErr
			}
			if existing.Status == RiderPayoutQueueCancelled {
				if reservation.ID != 0 && reservation.Status == RiderPayoutReservationActive {
					return ErrPayoutQueueUnavailable
				}
				result = RiderPayoutQueueResult{Queue: existing}
				return nil
			}
			value, err := reconcileExistingRiderPayoutQueueTx(tx, wallet, earning, existing, reservation, now)
			if err != nil {
				return err
			}
			result = value
			return nil
		}
		if !errors.Is(queueErr, gorm.ErrRecordNotFound) {
			return queueErr
		}
		queueID, err := NewULID()
		if err != nil {
			return err
		}
		source, verifyErr := verifyRiderEarningFundingTx(tx, earning)
		if verifyErr != nil && !isRiderPayoutSourceBlockError(verifyErr) {
			return verifyErr
		}
		canQueue := verifyErr == nil && source.State == RiderPayoutSourceVerified && earning.SettlementStatus != RiderEarningPaidOffline
		balanceBlocked := false
		if canQueue {
			reserved, reserveErr := activeRiderPayoutReservationAmountTx(tx, wallet.RiderUserID)
			if reserveErr != nil {
				return reserveErr
			}
			if reserved > wallet.AvailableBalanceCents || earning.RiderFeeCents > wallet.AvailableBalanceCents-reserved {
				canQueue = false
				balanceBlocked = true
			}
		}
		queue := RiderPayoutQueue{
			PayoutQueuePublicID: queueID, EarningPublicID: earning.EarningPublicID,
			DeliveryOrderPublicID: earning.DeliveryOrderPublicID, RiderUserID: earning.RiderUserID,
			AmountCents: earning.RiderFeeCents, Currency: "CNY", RowVersion: 1,
			SourcePolicyVersion: "rider-payout-source-v1", SourceEvidenceHash: source.EvidenceHash,
			CreatedAt: now, UpdatedAt: now,
		}
		if canQueue {
			queue.Status = RiderPayoutQueueQueued
			queue.ReasonCode = ""
		} else {
			queue.Status = RiderPayoutQueueBlocked
			if balanceBlocked {
				queue.ReasonCode = "balance_insufficient"
			} else if verifyErr != nil {
				queue.ReasonCode = reasonCodeForPayoutError(verifyErr)
			} else {
				queue.ReasonCode = "earning_not_payable"
			}
		}
		if err := validateRiderPayoutQueue(queue); err != nil {
			return err
		}
		if err := tx.Create(&queue).Error; err != nil {
			if isDuplicateKey(err) {
				return ErrIdempotencyConflict
			}
			return err
		}
		event, err := newRiderPayoutQueueEvent(queue, RiderPayoutQueueEventType(queue.Status), RiderPayoutQueueActorSystem, nil, queue.ReasonCode, "", nil, nil, now)
		if err != nil {
			return err
		}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		result.Queue = queue
		if queue.Status == RiderPayoutQueueQueued {
			reservationID, err := NewULID()
			if err != nil {
				return err
			}
			reservation := RiderPayoutReservation{
				ReservationPublicID: reservationID, PayoutQueuePublicID: queue.PayoutQueuePublicID,
				EarningPublicID: earning.EarningPublicID, WalletPublicID: wallet.WalletPublicID,
				RiderUserID: earning.RiderUserID, AmountCents: earning.RiderFeeCents,
				Status: RiderPayoutReservationActive, RowVersion: 1, ReservedAt: now,
			}
			if err := validateRiderPayoutReservation(reservation); err != nil {
				return err
			}
			if err := tx.Create(&reservation).Error; err != nil {
				return err
			}
			result.Reservation = &reservation
		}
		return nil
	})
	return result, err
}

func reconcileExistingRiderPayoutQueueTx(
	tx *gorm.DB,
	wallet RiderWallet,
	earning RiderEarningRecord,
	queue RiderPayoutQueue,
	reservation RiderPayoutReservation,
	now time.Time,
) (RiderPayoutQueueResult, error) {
	if queue.Status == RiderPayoutQueueQueued {
		if reservation.ID == 0 || reservation.Status != RiderPayoutReservationActive ||
			reservation.AmountCents != queue.AmountCents || reservation.RiderUserID != queue.RiderUserID ||
			reservation.WalletPublicID != wallet.WalletPublicID {
			return RiderPayoutQueueResult{}, ErrPayoutQueueUnavailable
		}
	} else if queue.Status == RiderPayoutQueueBlocked {
		if reservation.ID != 0 && reservation.Status == RiderPayoutReservationActive {
			return RiderPayoutQueueResult{}, ErrPayoutQueueUnavailable
		}
	} else {
		return RiderPayoutQueueResult{}, ErrPayoutQueueNotCancellable
	}

	source, verifyErr := verifyRiderEarningFundingTx(tx, earning)
	if verifyErr != nil && !isRiderPayoutSourceBlockError(verifyErr) {
		return RiderPayoutQueueResult{}, verifyErr
	}
	canQueue := verifyErr == nil && source.State == RiderPayoutSourceVerified &&
		(earning.SettlementStatus == RiderEarningPending || earning.SettlementStatus == RiderEarningSettled)
	reasonCode := ""
	if !canQueue {
		if verifyErr != nil {
			reasonCode = reasonCodeForPayoutError(verifyErr)
		} else {
			reasonCode = "earning_not_payable"
		}
	}
	reserved, err := activeRiderPayoutReservationAmountTx(tx, wallet.RiderUserID)
	if err != nil {
		return RiderPayoutQueueResult{}, err
	}
	if canQueue {
		if reserved > wallet.AvailableBalanceCents {
			canQueue = false
			reasonCode = "balance_insufficient"
		} else if reservation.Status != RiderPayoutReservationActive && queue.AmountCents > wallet.AvailableBalanceCents-reserved {
			canQueue = false
			reasonCode = "balance_insufficient"
		}
	}

	if !canQueue {
		if reservation.ID != 0 && reservation.Status == RiderPayoutReservationActive {
			if err := releaseRiderPayoutReservationTx(tx, &reservation, now); err != nil {
				return RiderPayoutQueueResult{}, err
			}
		}
		changed := queue.Status != RiderPayoutQueueBlocked || queue.ReasonCode != reasonCode ||
			queue.SourceEvidenceHash != source.EvidenceHash || queue.SourcePolicyVersion != source.Policy
		if changed {
			priorStatus := queue.Status
			if err := updateRiderPayoutQueueStateTx(tx, &queue, RiderPayoutQueueBlocked, reasonCode, source, now); err != nil {
				return RiderPayoutQueueResult{}, err
			}
			eventType := RiderPayoutQueueEventBlocked
			if priorStatus == RiderPayoutQueueQueued {
				eventType = RiderPayoutQueueEventSourceInvalidated
			}
			event, err := newRiderPayoutQueueEvent(queue, eventType, RiderPayoutQueueActorSystem, nil, reasonCode, "", nil, nil, now)
			if err != nil {
				return RiderPayoutQueueResult{}, err
			}
			if err := tx.Create(&event).Error; err != nil {
				return RiderPayoutQueueResult{}, err
			}
		}
		value := RiderPayoutQueueResult{Queue: queue}
		if reservation.ID != 0 {
			value.Reservation = &reservation
		}
		return value, nil
	}

	if reservation.ID == 0 {
		reservationID, err := NewULID()
		if err != nil {
			return RiderPayoutQueueResult{}, err
		}
		reservation = RiderPayoutReservation{
			ReservationPublicID: reservationID, PayoutQueuePublicID: queue.PayoutQueuePublicID,
			EarningPublicID: earning.EarningPublicID, WalletPublicID: wallet.WalletPublicID,
			RiderUserID: earning.RiderUserID, AmountCents: earning.RiderFeeCents,
			Status: RiderPayoutReservationActive, RowVersion: 1, ReservedAt: now,
		}
		if err := validateRiderPayoutReservation(reservation); err != nil {
			return RiderPayoutQueueResult{}, err
		}
		if err := tx.Create(&reservation).Error; err != nil {
			return RiderPayoutQueueResult{}, err
		}
	} else if reservation.Status == RiderPayoutReservationReleased {
		update := tx.Model(&RiderPayoutReservation{}).
			Where("id = ? AND row_version = ? AND status = ?", reservation.ID, reservation.RowVersion, RiderPayoutReservationReleased).
			Updates(map[string]any{
				"status": RiderPayoutReservationActive, "reserved_at": now, "released_at": nil,
				"row_version": gorm.Expr("row_version + 1"),
			})
		if update.Error != nil {
			return RiderPayoutQueueResult{}, update.Error
		}
		if update.RowsAffected != 1 {
			return RiderPayoutQueueResult{}, ErrConcurrentUpdate
		}
		reservation.Status = RiderPayoutReservationActive
		reservation.ReservedAt = now
		reservation.ReleasedAt = nil
		reservation.RowVersion++
	}
	changed := queue.Status != RiderPayoutQueueQueued || queue.ReasonCode != "" ||
		queue.SourceEvidenceHash != source.EvidenceHash || queue.SourcePolicyVersion != source.Policy
	if changed {
		if err := updateRiderPayoutQueueStateTx(tx, &queue, RiderPayoutQueueQueued, "", source, now); err != nil {
			return RiderPayoutQueueResult{}, err
		}
		event, err := newRiderPayoutQueueEvent(queue, RiderPayoutQueueEventQueued, RiderPayoutQueueActorSystem, nil, "", "", nil, nil, now)
		if err != nil {
			return RiderPayoutQueueResult{}, err
		}
		if err := tx.Create(&event).Error; err != nil {
			return RiderPayoutQueueResult{}, err
		}
	}
	return RiderPayoutQueueResult{Queue: queue, Reservation: &reservation}, nil
}

func releaseRiderPayoutReservationTx(tx *gorm.DB, reservation *RiderPayoutReservation, now time.Time) error {
	update := tx.Model(&RiderPayoutReservation{}).
		Where("id = ? AND row_version = ? AND status = ?", reservation.ID, reservation.RowVersion, RiderPayoutReservationActive).
		Updates(map[string]any{
			"status": RiderPayoutReservationReleased, "released_at": now,
			"row_version": gorm.Expr("row_version + 1"),
		})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return ErrConcurrentUpdate
	}
	reservation.Status = RiderPayoutReservationReleased
	reservation.ReleasedAt = timePointer(now)
	reservation.RowVersion++
	return nil
}

func updateRiderPayoutQueueStateTx(tx *gorm.DB, queue *RiderPayoutQueue, status RiderPayoutQueueStatus, reasonCode string, source RiderPayoutSourceSnapshot, now time.Time) error {
	update := tx.Model(&RiderPayoutQueue{}).Where("id = ? AND row_version = ?", queue.ID, queue.RowVersion).Updates(map[string]any{
		"status": status, "reason_code": reasonCode, "source_policy_version": source.Policy,
		"source_evidence_hash": source.EvidenceHash, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
	})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return ErrConcurrentUpdate
	}
	queue.Status = status
	queue.ReasonCode = reasonCode
	queue.SourcePolicyVersion = source.Policy
	queue.SourceEvidenceHash = source.EvidenceHash
	queue.RowVersion++
	queue.UpdatedAt = now
	return nil
}

func (r *GormRepository) CancelRiderPayoutQueue(ctx context.Context, queuePublicID string, expectedRowVersion, operatorUserID uint64, idempotencyKey, reason string, now time.Time) (RiderPayoutQueueResult, error) {
	if !ValidULID(queuePublicID) || expectedRowVersion == 0 || operatorUserID == 0 || !ValidULID(idempotencyKey) || strings.TrimSpace(reason) == "" || now.IsZero() {
		return RiderPayoutQueueResult{}, ErrInvalidRecord
	}
	if !r.queueTablesAvailable() {
		return RiderPayoutQueueResult{}, ErrPayoutQueueUnavailable
	}
	var candidateQueue RiderPayoutQueue
	if err := r.db.WithContext(ctx).Where("payout_queue_public_id = ?", queuePublicID).First(&candidateQueue).Error; err != nil {
		return RiderPayoutQueueResult{}, normalizeNotFound(err)
	}
	var result RiderPayoutQueueResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// All payout writes use wallet -> earning -> queue/reservation. The
		// initial queue read above is intentionally unlocked only to obtain the
		// rider and earning keys for this ordered transaction.
		wallet, err := lockRiderWalletForQueueTx(tx, candidateQueue.RiderUserID, now)
		if err != nil {
			return err
		}
		var earning RiderEarningRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("earning_public_id = ?", candidateQueue.EarningPublicID).First(&earning).Error; err != nil {
			return normalizeNotFound(err)
		}
		var queue RiderPayoutQueue
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("payout_queue_public_id = ?", queuePublicID).First(&queue).Error; err != nil {
			return normalizeNotFound(err)
		}
		var prior RiderPayoutQueueEvent
		if err := tx.Where("idempotency_key = ?", idempotencyKey).First(&prior).Error; err == nil {
			if prior.PayoutQueuePublicID != queuePublicID || prior.Reason != strings.TrimSpace(reason) || prior.OperatorUserID == nil || *prior.OperatorUserID != operatorUserID || prior.ExpectedRowVersion == nil || *prior.ExpectedRowVersion != expectedRowVersion {
				return ErrIdempotencyConflict
			}
			var reservation RiderPayoutReservation
			_ = tx.Where("payout_queue_public_id = ?", queuePublicID).First(&reservation).Error
			result = RiderPayoutQueueResult{Queue: queue}
			if reservation.ID != 0 {
				result.Reservation = &reservation
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var reservation RiderPayoutReservation
		resErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("payout_queue_public_id = ?", queuePublicID).First(&reservation).Error
		if errors.Is(resErr, gorm.ErrRecordNotFound) {
			resErr = nil
		}
		if resErr != nil {
			return resErr
		}
		if queue.RowVersion != expectedRowVersion {
			return ErrConcurrentUpdate
		}
		if queue.Status == RiderPayoutQueueCancelled {
			return ErrPayoutQueueNotCancellable
		}
		if queue.Status != RiderPayoutQueueQueued && queue.Status != RiderPayoutQueueBlocked {
			return ErrPayoutQueueNotCancellable
		}
		if queue.Status == RiderPayoutQueueQueued {
			if reservation.ID == 0 || reservation.Status != RiderPayoutReservationActive || reservation.AmountCents != queue.AmountCents || reservation.WalletPublicID != wallet.WalletPublicID {
				return ErrPayoutQueueUnavailable
			}
			if err := releaseRiderPayoutReservationTx(tx, &reservation, now); err != nil {
				return err
			}
		}
		update := tx.Model(&RiderPayoutQueue{}).Where("id = ? AND row_version = ?", queue.ID, queue.RowVersion).Updates(map[string]any{
			"status": RiderPayoutQueueCancelled, "cancelled_at": now, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		queue.Status = RiderPayoutQueueCancelled
		queue.CancelledAt = timePointer(now)
		queue.RowVersion++
		queue.UpdatedAt = now
		key := idempotencyKey
		event, err := newRiderPayoutQueueEvent(queue, RiderPayoutQueueEventCancelled, RiderPayoutQueueActorAdmin, &operatorUserID, "", strings.TrimSpace(reason), &key, &expectedRowVersion, now)
		if err != nil {
			return err
		}
		if err := tx.Create(&event).Error; err != nil {
			if isDuplicateKey(err) {
				return ErrIdempotencyConflict
			}
			return err
		}
		result.Queue = queue
		if reservation.ID != 0 {
			result.Reservation = &reservation
		}
		_ = earning
		return nil
	})
	return result, err
}

func (r *GormRepository) queueTablesAvailable() bool {
	return r.db.Migrator().HasTable(&RiderPayoutQueue{}) && r.db.Migrator().HasTable(&RiderPayoutReservation{}) && r.db.Migrator().HasTable(&RiderPayoutQueueEvent{})
}

func activeRiderPayoutReservationTx(tx *gorm.DB, earningPublicID string) (RiderPayoutReservation, bool, error) {
	queueExists := tx.Migrator().HasTable(&RiderPayoutQueue{})
	reservationExists := tx.Migrator().HasTable(&RiderPayoutReservation{})
	if !queueExists && !reservationExists {
		// Older isolated test schemas may not have migration 050 yet. The
		// production path enables the guard as soon as the migration exists.
		return RiderPayoutReservation{}, false, nil
	}
	if !queueExists || !reservationExists {
		return RiderPayoutReservation{}, false, ErrPayoutQueueUnavailable
	}
	var queue RiderPayoutQueue
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("earning_public_id = ?", earningPublicID).First(&queue).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return RiderPayoutReservation{}, false, nil
	}
	if err != nil {
		return RiderPayoutReservation{}, false, err
	}
	var reservation RiderPayoutReservation
	err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("payout_queue_public_id = ?", queue.PayoutQueuePublicID).First(&reservation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if queue.Status == RiderPayoutQueueQueued {
			return RiderPayoutReservation{}, false, ErrPayoutQueueUnavailable
		}
		return RiderPayoutReservation{}, false, nil
	}
	if err != nil {
		return RiderPayoutReservation{}, false, err
	}
	if queue.Status == RiderPayoutQueueQueued && reservation.Status == RiderPayoutReservationActive {
		return reservation, true, nil
	}
	if queue.Status == RiderPayoutQueueQueued {
		return RiderPayoutReservation{}, false, ErrPayoutQueueUnavailable
	}
	return reservation, false, nil
}

func activeRiderPayoutReservationAmountTx(tx *gorm.DB, riderUserID uint64) (uint64, error) {
	if riderUserID == 0 {
		return 0, ErrInvalidRecord
	}
	var row struct {
		Amount uint64 `gorm:"column:amount"`
	}
	if err := tx.Model(&RiderPayoutReservation{}).
		Where("rider_user_id = ? AND status = ?", riderUserID, RiderPayoutReservationActive).
		Select("COALESCE(SUM(amount_cents), 0) AS amount").Scan(&row).Error; err != nil {
		return 0, err
	}
	return row.Amount, nil
}

func lockRiderWalletForQueueTx(tx *gorm.DB, riderUserID uint64, now time.Time) (RiderWallet, error) {
	if riderUserID == 0 {
		return RiderWallet{}, ErrInvalidRecord
	}
	var wallet RiderWallet
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("rider_user_id = ?", riderUserID).First(&wallet).Error; err != nil {
		return RiderWallet{}, normalizeNotFound(err)
	}
	_ = now
	return wallet, nil
}

func newRiderPayoutQueueEvent(queue RiderPayoutQueue, eventType RiderPayoutQueueEventType, actorType RiderPayoutQueueActorType, operatorUserID *uint64, reasonCode, reason string, idempotencyKey *string, expectedRowVersion *uint64, now time.Time) (RiderPayoutQueueEvent, error) {
	id, err := NewULID()
	if err != nil {
		return RiderPayoutQueueEvent{}, err
	}
	event := RiderPayoutQueueEvent{EventPublicID: id, PayoutQueuePublicID: queue.PayoutQueuePublicID, EventType: eventType, ActorType: actorType, OperatorUserID: operatorUserID, ReasonCode: reasonCode, Reason: reason, IdempotencyKey: idempotencyKey, ExpectedRowVersion: expectedRowVersion, CreatedAt: now}
	if err := validateRiderPayoutQueueEvent(event); err != nil {
		return RiderPayoutQueueEvent{}, err
	}
	return event, nil
}

func reasonCodeForPayoutError(err error) string {
	switch {
	case errors.Is(err, ErrPayoutQueueSourceBlocked):
		return "source_blocked"
	case errors.Is(err, ErrFundingSourceMismatch):
		return "source_mismatch"
	case errors.Is(err, ErrNotFound):
		return "source_missing"
	default:
		return "source_unavailable"
	}
}

func isRiderPayoutSourceBlockError(err error) bool {
	return errors.Is(err, ErrPayoutQueueSourceBlocked) || errors.Is(err, ErrFundingSourceMismatch) || errors.Is(err, ErrNotFound)
}

func verifyRiderEarningFundingTx(tx *gorm.DB, earning RiderEarningRecord) (RiderPayoutSourceSnapshot, error) {
	snapshot := RiderPayoutSourceSnapshot{State: RiderPayoutSourceBlocked, Policy: "rider-payout-source-v1"}
	var order Order
	if err := tx.Where("delivery_order_public_id = ?", earning.DeliveryOrderPublicID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return snapshot, ErrPayoutQueueSourceBlocked
		}
		return snapshot, err
	}
	if order.Status != StatusDelivered || order.MerchantUserID == 0 || order.RiderUserID == nil || *order.RiderUserID != earning.RiderUserID || order.DeliveryFeeCents != earning.DeliveryFeeCents || order.PlatformCutCents != earning.PlatformCutCents || order.RiderFeeCents != earning.RiderFeeCents {
		return snapshot, ErrFundingSourceMismatch
	}
	var settlement FeeSettlement
	if err := tx.Where("delivery_order_public_id = ?", earning.DeliveryOrderPublicID).First(&settlement).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return snapshot, ErrPayoutQueueSourceBlocked
		}
		return snapshot, err
	}
	if settlement.Status != SettlementPaid || settlement.AmountCents != earning.DeliveryFeeCents || settlement.MerchantUserID == 0 {
		return snapshot, ErrFundingSourceMismatch
	}
	if settlement.RefundedAt != nil || settlement.Status == SettlementRefunded || settlement.Status == SettlementRefundPending {
		return snapshot, ErrPayoutQueueSourceBlocked
	}
	var refunds []Refund
	if err := tx.Where("settlement_public_id = ?", settlement.SettlementPublicID).Order("id ASC").Find(&refunds).Error; err != nil {
		return snapshot, err
	}
	for _, refund := range refunds {
		switch refund.Status {
		case RefundPending, RefundRetryPending, RefundSucceeded:
			return snapshot, ErrPayoutQueueSourceBlocked
		}
	}
	parts := []string{settlement.SettlementPublicID, fmt.Sprint(settlement.MerchantUserID), fmt.Sprint(settlement.AmountCents), string(settlement.Source)}
	switch settlement.Source {
	case SettlementWechat:
		if settlement.PaymentPublicID == nil || !ValidULID(*settlement.PaymentPublicID) {
			return snapshot, ErrFundingSourceMismatch
		}
		var attempt PaymentAttempt
		if err := tx.Where("payment_public_id = ?", *settlement.PaymentPublicID).First(&attempt).Error; err != nil {
			return snapshot, normalizeNotFound(err)
		}
		if attempt.Purpose != PaymentOrderFee || attempt.Provider != PaymentWechat || attempt.Status != PaymentPaid || attempt.ProviderStatus != "SUCCESS" || attempt.PaidAt == nil || attempt.PaidAt.IsZero() || attempt.AmountCents != settlement.AmountCents || attempt.Currency != "CNY" || attempt.MerchantUserID != settlement.MerchantUserID || attempt.WechatTransactionID == nil || strings.TrimSpace(*attempt.WechatTransactionID) == "" {
			return snapshot, ErrFundingSourceMismatch
		}
		if attempt.DeliveryOrderPublicID == nil || *attempt.DeliveryOrderPublicID != earning.DeliveryOrderPublicID {
			return snapshot, ErrFundingSourceMismatch
		}
		parts = append(parts, attempt.PaymentPublicID, *attempt.WechatTransactionID)
	case SettlementWallet:
		var allocations []FundingAllocation
		if err := tx.Where("settlement_public_id = ?", settlement.SettlementPublicID).Order("id ASC").Find(&allocations).Error; err != nil {
			return snapshot, err
		}
		if len(allocations) == 0 {
			return snapshot, ErrPayoutQueueSourceBlocked
		}
		var total uint64
		for _, allocation := range allocations {
			if allocation.MerchantUserID != settlement.MerchantUserID || allocation.RefundedAmountCents != 0 || allocation.AmountCents == 0 || allocation.AmountCents > ^uint64(0)-total {
				return snapshot, ErrFundingSourceMismatch
			}
			total += allocation.AmountCents
			var lot FundingLot
			if err := tx.Where("funding_lot_public_id = ?", allocation.FundingLotPublicID).First(&lot).Error; err != nil {
				return snapshot, normalizeNotFound(err)
			}
			if lot.Source != FundingSourceWechatRecharge || lot.VerificationStatus != FundingVerificationVerified || lot.Currency != "CNY" || lot.MerchantUserID != settlement.MerchantUserID || lot.PaymentPublicID == nil {
				return snapshot, ErrFundingSourceMismatch
			}
			var recharge PaymentAttempt
			if err := tx.Where("payment_public_id = ?", *lot.PaymentPublicID).First(&recharge).Error; err != nil {
				return snapshot, normalizeNotFound(err)
			}
			if recharge.Purpose != PaymentWalletRecharge || recharge.Provider != PaymentWechat || recharge.Status != PaymentPaid || recharge.ProviderStatus != "SUCCESS" || recharge.PaidAt == nil || recharge.PaidAt.IsZero() || recharge.Currency != "CNY" || recharge.MerchantUserID != settlement.MerchantUserID || recharge.WechatTransactionID == nil || strings.TrimSpace(*recharge.WechatTransactionID) == "" {
				return snapshot, ErrFundingSourceMismatch
			}
			parts = append(parts, allocation.FundingAllocationPublicID, lot.FundingLotPublicID, recharge.PaymentPublicID, *recharge.WechatTransactionID)
		}
		if total != settlement.AmountCents {
			return snapshot, ErrFundingSourceMismatch
		}
	default:
		return snapshot, ErrPayoutQueueSourceBlocked
	}
	hash := sha256.Sum256([]byte(strings.Join(parts, "|")))
	snapshot.State = RiderPayoutSourceVerified
	snapshot.EvidenceHash = hex.EncodeToString(hash[:])
	return snapshot, nil
}
