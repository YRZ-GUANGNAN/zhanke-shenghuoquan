package delivery

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ProofRepository interface {
	CreateProofAndTransition(context.Context, *Proof, uint64, Status, Status, time.Time) (Proof, Order, error)
	FindProof(context.Context, string, ProofType, uint16) (Proof, error)
	ListProofs(context.Context, string) ([]Proof, error)
	FindAssignedRiderOrder(context.Context, string, uint64) (Order, error)
	FindParticipantOrder(context.Context, string, uint64) (Order, error)
	ClaimProofBinding(context.Context, string, time.Time, time.Duration) (ProofBindingLease, bool, error)
	ClaimNextProofBinding(context.Context, time.Time, time.Duration) (ProofBindingLease, bool, error)
	CompleteProofBinding(context.Context, ProofBindingLease, time.Time) error
	FailProofBinding(context.Context, ProofBindingLease, time.Time, uint32, time.Time) (ProofStatus, error)
}

var _ ProofRepository = (*GormRepository)(nil)

func (r *GormRepository) CreateProofAndTransition(
	ctx context.Context,
	candidate *Proof,
	expectedRowVersion uint64,
	from, to Status,
	now time.Time,
) (Proof, Order, error) {
	if candidate == nil || candidate.ValidateNew() != nil || expectedRowVersion == 0 ||
		!proofTransitionMatches(candidate.ProofType, from, to) {
		return Proof{}, Order{}, ErrInvalidProof
	}
	var proof Proof
	var order Order
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("delivery_order_public_id = ?", candidate.DeliveryOrderPublicID).
			First(&order).Error; err != nil {
			return normalizeNotFound(err)
		}
		if order.RiderUserID == nil || *order.RiderUserID != candidate.UploaderUserID {
			return ErrForbiddenOrder
		}

		existing, found, err := findProofByCoordinateTx(
			tx, candidate.DeliveryOrderPublicID, candidate.ProofType, candidate.Slot,
		)
		if err != nil {
			return err
		}
		if found {
			if existing.AssetPublicID != candidate.AssetPublicID ||
				existing.UploaderUserID != candidate.UploaderUserID {
				return ErrProofConflict
			}
			if order.Status != to {
				return ErrConcurrentUpdate
			}
			proof = existing
			return nil
		}
		if order.Status == to {
			return ErrProofRequired
		}
		if order.Status != from || order.RowVersion != expectedRowVersion {
			return ErrConcurrentUpdate
		}
		updates, err := buildTransitionUpdates(order, from, to, &candidate.UploaderUserID, now)
		if err != nil {
			return err
		}
		if err := tx.Create(candidate).Error; err != nil {
			if isDuplicateKey(err) {
				return ErrProofConflict
			}
			return err
		}
		updates["status"] = to
		updates["updated_at"] = now
		updates["row_version"] = gorm.Expr("row_version + 1")
		result := tx.Model(&Order{}).
			Where("id = ? AND status = ? AND row_version = ?", order.ID, from, expectedRowVersion).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		proof = *candidate
		applyTransition(&order, to, &candidate.UploaderUserID, now)
		order.RowVersion++
		order.UpdatedAt = now
		return nil
	})
	return proof, order, err
}

func proofTransitionMatches(proofType ProofType, from, to Status) bool {
	return (proofType == ProofPickup && from == StatusToPickup && to == StatusPickedUp) ||
		(proofType == ProofDelivery && from == StatusDelivering && to == StatusDelivered)
}

func findProofByCoordinateTx(tx *gorm.DB, orderID string, proofType ProofType, slot uint16) (Proof, bool, error) {
	var value Proof
	err := tx.Where("delivery_order_public_id = ? AND proof_type = ? AND slot = ?", orderID, proofType, slot).
		First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Proof{}, false, nil
	}
	return value, err == nil, err
}

func (r *GormRepository) FindProof(ctx context.Context, orderID string, proofType ProofType, slot uint16) (Proof, error) {
	var value Proof
	err := r.db.WithContext(ctx).
		Where("delivery_order_public_id = ? AND proof_type = ? AND slot = ?", orderID, proofType, slot).
		First(&value).Error
	return value, normalizeNotFound(err)
}

func (r *GormRepository) ListProofs(ctx context.Context, orderID string) ([]Proof, error) {
	var values []Proof
	err := r.db.WithContext(ctx).
		Where("delivery_order_public_id = ?", orderID).
		Order("proof_type ASC, slot ASC").
		Find(&values).Error
	return values, err
}

func (r *GormRepository) FindParticipantOrder(ctx context.Context, orderID string, userID uint64) (Order, error) {
	var value Order
	err := r.db.WithContext(ctx).
		Where("delivery_order_public_id = ? AND (merchant_user_id = ? OR rider_user_id = ?)", orderID, userID, userID).
		First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Order{}, ErrForbiddenOrder
	}
	return value, err
}

func (r *GormRepository) FindAssignedRiderOrder(ctx context.Context, orderID string, riderUserID uint64) (Order, error) {
	var value Order
	err := r.db.WithContext(ctx).
		Where("delivery_order_public_id = ? AND rider_user_id = ?", orderID, riderUserID).
		First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Order{}, ErrForbiddenOrder
	}
	return value, err
}

func (r *GormRepository) ClaimProofBinding(
	ctx context.Context,
	proofPublicID string,
	now time.Time,
	leaseTTL time.Duration,
) (ProofBindingLease, bool, error) {
	return r.claimProofBinding(ctx, proofPublicID, now, leaseTTL)
}

func (r *GormRepository) ClaimNextProofBinding(
	ctx context.Context,
	now time.Time,
	leaseTTL time.Duration,
) (ProofBindingLease, bool, error) {
	return r.claimProofBinding(ctx, "", now, leaseTTL)
}

func (r *GormRepository) claimProofBinding(
	ctx context.Context,
	proofPublicID string,
	now time.Time,
	leaseTTL time.Duration,
) (ProofBindingLease, bool, error) {
	var claimed ProofBindingLease
	found := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ?", ProofBinding).
			Where("(next_attempt_at IS NULL OR next_attempt_at <= ?)", now).
			Where("(lease_expires_at IS NULL OR lease_expires_at <= ?)", now)
		if proofPublicID != "" {
			query = query.Where("proof_public_id = ?", proofPublicID)
		}
		var proof Proof
		err := query.Order("id ASC").First(&proof).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		token, err := NewULID()
		if err != nil {
			return err
		}
		expires := now.Add(leaseTTL)
		result := tx.Model(&Proof{}).
			Where("id = ? AND status = ? AND row_version = ?", proof.ID, ProofBinding, proof.RowVersion).
			Updates(map[string]any{
				"attempt_count": gorm.Expr("attempt_count + 1"),
				"lease_token":   token, "lease_expires_at": expires, "next_attempt_at": nil,
				"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrProofLease
		}
		proof.AttemptCount++
		proof.LeaseToken = &token
		proof.LeaseExpiresAt = &expires
		proof.NextAttemptAt = nil
		proof.RowVersion++
		proof.UpdatedAt = now
		claimed = ProofBindingLease{Proof: proof, LeaseToken: token}
		found = true
		return nil
	})
	return claimed, found, err
}

func (r *GormRepository) CompleteProofBinding(ctx context.Context, lease ProofBindingLease, now time.Time) error {
	if !CanTransitionProof(ProofBinding, ProofActive) {
		return ErrInvalidTransition
	}
	result := r.db.WithContext(ctx).Model(&Proof{}).
		Where("id = ? AND status = ? AND row_version = ? AND lease_token = ?",
			lease.Proof.ID, ProofBinding, lease.Proof.RowVersion, lease.LeaseToken).
		Updates(map[string]any{
			"status": ProofActive, "lease_token": nil, "lease_expires_at": nil,
			"next_attempt_at": nil, "activated_at": now,
			"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrProofLease
	}
	return nil
}

func (r *GormRepository) FailProofBinding(
	ctx context.Context,
	lease ProofBindingLease,
	retryAt time.Time,
	maxAttempts uint32,
	now time.Time,
) (ProofStatus, error) {
	status := ProofBinding
	if lease.Proof.AttemptCount >= maxAttempts {
		status = ProofDeadletter
	}
	if !CanTransitionProof(ProofBinding, status) {
		return "", ErrInvalidTransition
	}
	updates := map[string]any{
		"status": status, "lease_token": nil, "lease_expires_at": nil,
		"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
	}
	if status == ProofBinding {
		updates["next_attempt_at"] = retryAt
	} else {
		updates["next_attempt_at"] = nil
	}
	result := r.db.WithContext(ctx).Model(&Proof{}).
		Where("id = ? AND status = ? AND row_version = ? AND lease_token = ?",
			lease.Proof.ID, ProofBinding, lease.Proof.RowVersion, lease.LeaseToken).
		Updates(updates)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected != 1 {
		return "", ErrProofLease
	}
	return status, nil
}
