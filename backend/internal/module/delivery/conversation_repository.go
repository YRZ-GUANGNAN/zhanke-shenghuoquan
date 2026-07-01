package delivery

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ConversationRepository interface {
	EnsureConversationForCurrentAssignment(context.Context, string, uint64, time.Time) (Conversation, Order, bool, error)
	FindConversation(context.Context, string) (Conversation, error)
	FindConversationForParticipant(context.Context, string, uint64) (Conversation, Order, error)
	FindCurrentConversationForParticipant(context.Context, string, uint64) (Conversation, Order, error)
	FindCurrentConversationForRider(context.Context, string, uint64) (Conversation, Order, error)
	ListConversationsForParticipant(context.Context, uint64, Page) ([]Conversation, int64, error)
	ClaimNextConversation(context.Context, time.Time, time.Duration) (ConversationLease, bool, error)
	ClaimConversation(context.Context, string, time.Time, time.Duration) (ConversationLease, bool, error)
	CompleteConversationProvision(context.Context, ConversationLease, time.Time) error
	FailConversationProvision(context.Context, ConversationLease, string, time.Time, uint32, time.Time) error
	BeginConversationRevocation(context.Context, uint64, ConversationStatus, uint64, time.Time) (Conversation, error)
	CompleteConversationRevocation(context.Context, uint64, uint64, time.Time) error
	MarkConversationReadOnly(context.Context, uint64, uint64, time.Time) (Conversation, error)
	AllocateConversationImageSlot(context.Context, uint64, uint64, time.Time) (uint16, Conversation, error)
}

var _ ConversationRepository = (*GormRepository)(nil)

func (r *GormRepository) FindConversation(ctx context.Context, conversationID string) (Conversation, error) {
	var value Conversation
	err := r.db.WithContext(ctx).
		Where("conversation_public_id = ?", conversationID).
		First(&value).Error
	return value, normalizeConversationNotFound(err)
}

func (r *GormRepository) EnsureConversationForCurrentAssignment(
	ctx context.Context,
	orderID string,
	riderUserID uint64,
	now time.Time,
) (Conversation, Order, bool, error) {
	var result Conversation
	var order Order
	created := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("delivery_order_public_id = ?", orderID).
			First(&order).Error; err != nil {
			return normalizeNotFound(err)
		}
		if order.RiderUserID == nil || *order.RiderUserID != riderUserID ||
			(!orderAllowsConversationSend(order.Status) && !orderConversationReadOnly(order.Status)) {
			return ErrConversationForbidden
		}
		err := tx.Where(
			"delivery_order_public_id = ? AND rider_user_id = ? AND status <> ?",
			orderID, riderUserID, ConversationRevoked,
		).Order("assignment_version DESC").First(&result).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var maxVersion uint64
		if err := tx.Model(&Conversation{}).
			Where("delivery_order_public_id = ?", orderID).
			Select("COALESCE(MAX(assignment_version), 0)").
			Scan(&maxVersion).Error; err != nil {
			return err
		}
		publicID, err := NewULID()
		if err != nil {
			return err
		}
		result = Conversation{
			ConversationPublicID: publicID, DeliveryOrderPublicID: orderID,
			AssignmentVersion: maxVersion + 1, MerchantUserID: order.MerchantUserID,
			RiderUserID: riderUserID, TencentGroupID: "d5_" + publicID,
			Status: ConversationProvisioning, RowVersion: 1,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := result.ValidateNew(); err != nil {
			return err
		}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	return result, order, created, err
}

func (r *GormRepository) FindConversationForParticipant(
	ctx context.Context,
	conversationID string,
	userID uint64,
) (Conversation, Order, error) {
	var conversation Conversation
	if err := r.db.WithContext(ctx).
		Where("conversation_public_id = ?", conversationID).
		First(&conversation).Error; err != nil {
		return Conversation{}, Order{}, normalizeConversationNotFound(err)
	}
	var order Order
	if err := r.db.WithContext(ctx).
		Where("delivery_order_public_id = ?", conversation.DeliveryOrderPublicID).
		First(&order).Error; err != nil {
		return Conversation{}, Order{}, normalizeConversationNotFound(err)
	}
	if userID == order.MerchantUserID {
		return conversation, order, nil
	}
	if order.RiderUserID == nil || *order.RiderUserID != userID ||
		conversation.RiderUserID != userID || conversation.Status == ConversationRevoked {
		return Conversation{}, Order{}, ErrConversationForbidden
	}
	return conversation, order, nil
}

func (r *GormRepository) FindCurrentConversationForParticipant(
	ctx context.Context,
	orderID string,
	userID uint64,
) (Conversation, Order, error) {
	var order Order
	if err := r.db.WithContext(ctx).
		Where("delivery_order_public_id = ?", orderID).
		First(&order).Error; err != nil {
		return Conversation{}, Order{}, normalizeConversationNotFound(err)
	}
	if userID != order.MerchantUserID && (order.RiderUserID == nil || *order.RiderUserID != userID) {
		return Conversation{}, Order{}, ErrConversationForbidden
	}
	if order.RiderUserID == nil {
		return Conversation{}, Order{}, ErrConversationNotFound
	}
	var conversation Conversation
	err := r.db.WithContext(ctx).Where(
		"delivery_order_public_id = ? AND rider_user_id = ? AND status <> ?",
		orderID, *order.RiderUserID, ConversationRevoked,
	).Order("assignment_version DESC").First(&conversation).Error
	if err != nil {
		return Conversation{}, Order{}, normalizeConversationNotFound(err)
	}
	return conversation, order, nil
}

func (r *GormRepository) FindCurrentConversationForRider(
	ctx context.Context,
	orderID string,
	riderUserID uint64,
) (Conversation, Order, error) {
	return r.FindCurrentConversationForParticipant(ctx, orderID, riderUserID)
}

func (r *GormRepository) ListConversationsForParticipant(
	ctx context.Context,
	userID uint64,
	page Page,
) ([]Conversation, int64, error) {
	var values []Conversation
	var total int64
	query := r.db.WithContext(ctx).Table("delivery_im_conversations AS c").
		Joins("JOIN delivery_orders AS o ON o.delivery_order_public_id = c.delivery_order_public_id").
		Where(
			"o.merchant_user_id = ? OR (o.rider_user_id = ? AND c.rider_user_id = ? AND c.status <> ?)",
			userID, userID, userID, ConversationRevoked,
		)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Select("c.*").Order("c.updated_at DESC, c.id DESC").
		Offset((page.Page - 1) * page.Size).Limit(page.Size).Scan(&values).Error
	return values, total, err
}

func (r *GormRepository) ClaimConversation(
	ctx context.Context,
	conversationID string,
	now time.Time,
	leaseTTL time.Duration,
) (ConversationLease, bool, error) {
	return r.claimConversation(ctx, strings.TrimSpace(conversationID), now, leaseTTL)
}

func (r *GormRepository) ClaimNextConversation(
	ctx context.Context,
	now time.Time,
	leaseTTL time.Duration,
) (ConversationLease, bool, error) {
	return r.claimConversation(ctx, "", now, leaseTTL)
}

func (r *GormRepository) claimConversation(
	ctx context.Context,
	conversationID string,
	now time.Time,
	leaseTTL time.Duration,
) (ConversationLease, bool, error) {
	var lease ConversationLease
	found := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(
				"status = ? OR (status = ? AND next_attempt_at IS NOT NULL AND next_attempt_at <= ?)",
				ConversationProvisioning, ConversationFailed, now,
			).
			Where("(lease_expires_at IS NULL OR lease_expires_at <= ?)", now)
		if conversationID != "" {
			query = query.Where("conversation_public_id = ?", conversationID)
		}
		var conversation Conversation
		err := query.Order("id ASC").First(&conversation).Error
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
		update := tx.Model(&Conversation{}).
			Where("id = ? AND row_version = ? AND status = ?", conversation.ID, conversation.RowVersion, conversation.Status).
			Updates(map[string]any{
				"attempt_count": gorm.Expr("attempt_count + 1"),
				"lease_token":   token, "lease_expires_at": expires, "next_attempt_at": nil,
				"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConversationLease
		}
		conversation.AttemptCount++
		conversation.LeaseToken = &token
		conversation.LeaseExpiresAt = &expires
		conversation.NextAttemptAt = nil
		conversation.RowVersion++
		conversation.UpdatedAt = now
		lease = ConversationLease{Conversation: conversation, LeaseToken: token}
		found = true
		return nil
	})
	return lease, found, err
}

func (r *GormRepository) CompleteConversationProvision(
	ctx context.Context,
	lease ConversationLease,
	now time.Time,
) error {
	if !CanTransitionConversation(lease.Conversation.Status, ConversationActive) {
		return ErrInvalidTransition
	}
	update := r.db.WithContext(ctx).Model(&Conversation{}).
		Where("id = ? AND status = ? AND row_version = ? AND lease_token = ?",
			lease.Conversation.ID, lease.Conversation.Status, lease.Conversation.RowVersion, lease.LeaseToken).
		Updates(map[string]any{
			"status": ConversationActive, "lease_token": nil, "lease_expires_at": nil,
			"next_attempt_at": nil, "last_error": "", "activated_at": now,
			"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return ErrConversationLease
	}
	return nil
}

func (r *GormRepository) FailConversationProvision(
	ctx context.Context,
	lease ConversationLease,
	message string,
	retryAt time.Time,
	maxAttempts uint32,
	now time.Time,
) error {
	status := ConversationFailed
	if !CanTransitionConversation(lease.Conversation.Status, status) {
		return ErrInvalidTransition
	}
	if len(message) > 512 {
		message = message[:512]
	}
	nextAttempt := &retryAt
	if lease.Conversation.AttemptCount >= maxAttempts {
		nextAttempt = nil
	}
	update := r.db.WithContext(ctx).Model(&Conversation{}).
		Where("id = ? AND status = ? AND row_version = ? AND lease_token = ?",
			lease.Conversation.ID, lease.Conversation.Status, lease.Conversation.RowVersion, lease.LeaseToken).
		Updates(map[string]any{
			"status": status, "lease_token": nil, "lease_expires_at": nil,
			"next_attempt_at": nextAttempt, "last_error": message,
			"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return ErrConversationLease
	}
	return nil
}

func (r *GormRepository) BeginConversationRevocation(
	ctx context.Context,
	id uint64,
	from ConversationStatus,
	version uint64,
	now time.Time,
) (Conversation, error) {
	if from == ConversationRevoking {
		var value Conversation
		err := r.db.WithContext(ctx).First(&value, id).Error
		return value, normalizeConversationNotFound(err)
	}
	if !CanTransitionConversation(from, ConversationRevoking) {
		return Conversation{}, ErrInvalidTransition
	}
	update := r.db.WithContext(ctx).Model(&Conversation{}).
		Where("id = ? AND status = ? AND row_version = ?", id, from, version).
		Updates(map[string]any{
			"status":      ConversationRevoking,
			"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		})
	if update.Error != nil {
		return Conversation{}, update.Error
	}
	if update.RowsAffected != 1 {
		return Conversation{}, ErrConversationConcurrent
	}
	var value Conversation
	err := r.db.WithContext(ctx).First(&value, id).Error
	return value, normalizeConversationNotFound(err)
}

func (r *GormRepository) CompleteConversationRevocation(
	ctx context.Context,
	id, version uint64,
	now time.Time,
) error {
	if !CanTransitionConversation(ConversationRevoking, ConversationRevoked) {
		return ErrInvalidTransition
	}
	update := r.db.WithContext(ctx).Model(&Conversation{}).
		Where("id = ? AND status = ? AND row_version = ?", id, ConversationRevoking, version).
		Updates(map[string]any{
			"status": ConversationRevoked, "revoked_at": now,
			"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		})
	if update.Error != nil {
		return update.Error
	}
	if update.RowsAffected != 1 {
		return ErrConversationConcurrent
	}
	return nil
}

func (r *GormRepository) MarkConversationReadOnly(
	ctx context.Context,
	id, version uint64,
	now time.Time,
) (Conversation, error) {
	if !CanTransitionConversation(ConversationActive, ConversationReadOnly) {
		return Conversation{}, ErrInvalidTransition
	}
	update := r.db.WithContext(ctx).Model(&Conversation{}).
		Where("id = ? AND status = ? AND row_version = ?", id, ConversationActive, version).
		Updates(map[string]any{
			"status": ConversationReadOnly, "read_only_at": now,
			"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		})
	if update.Error != nil {
		return Conversation{}, update.Error
	}
	if update.RowsAffected != 1 {
		return Conversation{}, ErrConversationConcurrent
	}
	var value Conversation
	err := r.db.WithContext(ctx).First(&value, id).Error
	return value, normalizeConversationNotFound(err)
}

func (r *GormRepository) AllocateConversationImageSlot(
	ctx context.Context,
	id, version uint64,
	now time.Time,
) (uint16, Conversation, error) {
	var value Conversation
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&value, id).Error; err != nil {
			return normalizeConversationNotFound(err)
		}
		if value.Status != ConversationActive || value.RowVersion != version || value.NextImageSlot == ^uint16(0) {
			return ErrConversationConcurrent
		}
		update := tx.Model(&Conversation{}).
			Where("id = ? AND status = ? AND row_version = ?", id, ConversationActive, version).
			Updates(map[string]any{
				"next_image_slot": gorm.Expr("next_image_slot + 1"),
				"row_version":     gorm.Expr("row_version + 1"), "updated_at": now,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrConversationConcurrent
		}
		value.RowVersion++
		value.NextImageSlot++
		value.UpdatedAt = now
		return nil
	})
	if err != nil {
		return 0, Conversation{}, err
	}
	return value.NextImageSlot - 1, value, nil
}

func normalizeConversationNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrConversationNotFound
	}
	return err
}
