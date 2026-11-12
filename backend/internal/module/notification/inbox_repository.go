package notification

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// InboxRepository exposes only the authenticated recipient's visible inbox.
type InboxRepository interface {
	ListInbox(context.Context, uint64, string, int) ([]InboxItem, int64, error)
	MarkInboxRead(context.Context, uint64, string, time.Time) (bool, error)
}

var _ InboxRepository = (*GormRepository)(nil)
var _ SubscriptionAuthorizationRepository = (*GormRepository)(nil)

func (r *GormRepository) ReadSubscriptionAuthorizationState(ctx context.Context, userID uint64, appID, templateKey, templateTag string, now time.Time) (SubscriptionAuthorizationState, error) {
	if r == nil || r.db == nil || userID == 0 || appID == "" || !validTemplateKey(templateKey) || len(templateTag) != 32 || now.IsZero() {
		return SubscriptionAuthorizationState{}, ErrInvalidRecord
	}
	if decoded, err := hex.DecodeString(templateTag); err != nil || len(decoded) != 16 {
		return SubscriptionAuthorizationState{}, ErrInvalidRecord
	}
	totals := struct {
		RemainingQuota uint64 `gorm:"column:remaining_quota"`
		ReservedQuota  uint64 `gorm:"column:reserved_quota"`
	}{}
	err := r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Discard}).Model(&Subscription{}).
		Select("COALESCE(SUM(remaining_quota), 0) AS remaining_quota, COALESCE(SUM(reserved_quota), 0) AS reserved_quota").
		Where("user_id = ? AND app_id = ? AND template_key = ? AND RIGHT(authorization_nonce, 32) = ? AND status IN ? AND (expires_at IS NULL OR expires_at > ?)",
			userID, appID, templateKey, templateTag, []string{"accepted", "exhausted"}, now).
		Scan(&totals).Error
	if err != nil || totals.RemainingQuota > uint64(^uint32(0)) || totals.ReservedQuota > uint64(^uint32(0)) {
		if err != nil {
			return SubscriptionAuthorizationState{}, err
		}
		return SubscriptionAuthorizationState{}, ErrInvalidRecord
	}
	return SubscriptionAuthorizationState{RemainingQuota: uint32(totals.RemainingQuota), ReservedQuota: uint32(totals.ReservedQuota)}, nil
}

func (r *GormRepository) ListInbox(ctx context.Context, userID uint64, before string, limit int) ([]InboxItem, int64, error) {
	if r == nil || r.db == nil || userID == 0 || limit < 1 || limit > 100 || (before != "" && !validID(before)) {
		return nil, 0, ErrInvalidRecord
	}
	items := make([]InboxItem, 0)
	visible := r.db.WithContext(ctx).Model(&InboxItem{}).Where("recipient_user_id = ? AND status IN ?", userID, []DeliveryStatus{DeliveryDelivered, DeliveryRead})
	var unread int64
	if err := visible.Session(&gorm.Session{}).Where("read_at IS NULL").Count(&unread).Error; err != nil {
		return nil, 0, err
	}
	query := visible.Session(&gorm.Session{})
	if before != "" {
		query = query.Where("inbox_public_id < ?", before)
	}
	if err := query.Order("inbox_public_id DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, unread, nil
}

func (r *GormRepository) MarkInboxRead(ctx context.Context, userID uint64, publicID string, now time.Time) (bool, error) {
	if r == nil || r.db == nil || userID == 0 || !validID(publicID) || now.IsZero() {
		return false, ErrInvalidRecord
	}
	result := r.db.WithContext(ctx).Model(&InboxItem{}).
		Where("recipient_user_id = ? AND inbox_public_id = ? AND status = ? AND read_at IS NULL", userID, publicID, DeliveryDelivered).
		Updates(map[string]any{"status": DeliveryRead, "read_at": now, "updated_at": now, "row_version": gorm.Expr("row_version + 1")})
	if result.Error != nil || result.RowsAffected > 0 {
		return result.RowsAffected > 0, result.Error
	}
	var count int64
	err := r.db.WithContext(ctx).Model(&InboxItem{}).Where("recipient_user_id = ? AND inbox_public_id = ? AND status = ? AND read_at IS NOT NULL", userID, publicID, DeliveryRead).Count(&count).Error
	return count > 0, err
}

// A nonce is persisted as an invalid, zero-quota subscription until the user
// reports a result. Existing sender queries cannot reserve these placeholders.
func (r *GormRepository) IssueSubscriptionAuthorization(ctx context.Context, item Subscription, notBefore time.Time) error {
	if r == nil || r.db == nil || !validAuthorizationIdentity(item) || item.Status != "invalid" || item.InvalidationReason != authorizationPending || item.ExpiresAt == nil || !item.ExpiresAt.After(item.CreatedAt) || item.RemainingQuota != 0 || item.ReservedQuota != 0 {
		return ErrInvalidRecord
	}
	// Disable SQL logging for statements carrying OpenID. Serializable range
	// locking also protects the first issuance when no subscription row exists,
	// independent of the connection's default transaction isolation level.
	return r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Discard}).Transaction(func(tx *gorm.DB) error {
		var previous Subscription
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND app_id = ? AND template_key = ?", item.UserID, item.AppID, item.TemplateKey).Order("id DESC").First(&previous).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil && previous.CreatedAt.After(notBefore) {
			return ErrAuthorizationRateLimited
		}
		return tx.Create(&item).Error
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
}

func (r *GormRepository) ReportSubscriptionAuthorization(ctx context.Context, input AuthorizationReport) (bool, error) {
	if r == nil || r.db == nil || input.UserID == 0 || input.AppID == "" || input.OpenID == "" || input.TemplateKey == "" || !validAuthorizationNonce(input.Nonce) || input.Now.IsZero() || !validAuthorizationStatus(input.Status) {
		return false, ErrInvalidRecord
	}
	duplicate := false
	err := r.db.WithContext(ctx).Session(&gorm.Session{Logger: logger.Discard}).Transaction(func(tx *gorm.DB) error {
		var item Subscription
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND app_id = ? AND template_key = ? AND authorization_nonce = ?", input.UserID, input.AppID, input.TemplateKey, input.Nonce).First(&item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAuthorizationNonce
		}
		if err != nil {
			return err
		}
		if item.OpenID != input.OpenID {
			return ErrAuthorizationNonce
		}
		if item.InvalidationReason != authorizationPending {
			// Consumption can change accepted to exhausted; keep the reported result
			// in invalidation_reason so a retry never restores consumed quota.
			if item.InvalidationReason != authorizationReported+input.Status {
				return ErrAuthorizationConflict
			}
			duplicate = true
			return nil
		}
		if item.ExpiresAt == nil || !input.Now.Before(*item.ExpiresAt) {
			return ErrAuthorizationNonce
		}
		quota := uint32(0)
		if input.Status == "accepted" {
			quota = 1
		}
		result := tx.Model(&Subscription{}).Where("id = ? AND row_version = ? AND invalidation_reason = ?", item.ID, item.RowVersion, authorizationPending).Updates(map[string]any{
			"status": input.Status, "remaining_quota": quota, "authorized_at": input.Now, "expires_at": input.ExpiresAt,
			"invalidation_reason": authorizationReported + input.Status, "row_version": gorm.Expr("row_version + 1"), "updated_at": input.Now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrConcurrentUpdate
		}
		if input.Status == "rejected" || input.Status == "banned" {
			// A fresh explicit rejection supersedes prior available grants for the
			// same account/template. Keep each grant's original reported marker:
			// replaying its old accepted report must never replenish consent.
			// Occupied grants belong to in-flight/accepted/unknown deliveries and
			// are never released or rewritten by an authorization report.
			if err := tx.Model(&Subscription{}).
				Where("user_id = ? AND app_id = ? AND openid = ? AND template_key = ? AND status = ? AND remaining_quota = 1 AND reserved_quota = 0 AND reservation_nonce IS NULL", input.UserID, input.AppID, input.OpenID, input.TemplateKey, "accepted").
				Updates(map[string]any{"status": "revoked", "remaining_quota": 0, "invalidated_at": input.Now, "updated_at": input.Now, "row_version": gorm.Expr("row_version + 1")}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return duplicate, err
}
