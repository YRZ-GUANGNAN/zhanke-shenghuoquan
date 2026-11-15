package notificationconsole

import (
	"context"
	"errors"
	"strings"
	"time"

	"campus-circle/backend/internal/module/notification"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ChannelStat struct {
	Channel notification.Channel
	Status  notification.DeliveryStatus
	Count   int64
}

type OfficialAccountCounts struct {
	Unknown      int64
	Following    int64
	NotFollowing int64
}

type DeliveryRecord struct {
	DeliveryPublicID  string
	RecipientUserID   uint64
	Channel           notification.Channel
	Purpose           notification.NotificationPurpose
	Status            notification.DeliveryStatus
	AttemptCount      uint32
	LastErrorCode     string
	LastError         string
	NextAttemptAt     *time.Time
	ProcessedAt       *time.Time
	CreatedAt         time.Time
}

type Repository interface {
	ChannelStats(context.Context, time.Time) ([]ChannelStat, error)
	OfficialAccountCounts(context.Context, string) (OfficialAccountCounts, error)
	RecentDeliveries(context.Context, notification.DeliveryStatus, string, int, int) ([]DeliveryRecord, int64, error)
	RetryDelivery(context.Context, string, time.Time) (DeliveryRecord, error)
}

// ErrDeliveryNotRetryable marks delivery records that are still in flight or
// already delivered; only blocked and deadletter records can be re-queued.
var ErrDeliveryNotRetryable = errors.New("delivery is not retryable")

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) ChannelStats(ctx context.Context, since time.Time) ([]ChannelStat, error) {
	if r == nil || r.db == nil || since.IsZero() || !r.db.Migrator().HasTable(&notification.DeliveryItem{}) {
		return []ChannelStat{}, nil
	}
	values := make([]ChannelStat, 0)
	err := r.db.WithContext(ctx).Model(&notification.DeliveryItem{}).
		Select("channel, status, COUNT(*) AS count").
		Where("created_at >= ?", since).
		Group("channel, status").Order("channel, status").Scan(&values).Error
	return values, err
}

func (r *GormRepository) RecentDeliveries(ctx context.Context, status notification.DeliveryStatus, keyword string, limit, offset int) ([]DeliveryRecord, int64, error) {
	if r == nil || r.db == nil || !r.db.Migrator().HasTable(&notification.DeliveryItem{}) {
		return []DeliveryRecord{}, 0, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	query := r.db.WithContext(ctx).Model(&notification.DeliveryItem{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		pattern := "%" + keyword + "%"
		query = query.Where(
			"delivery_public_id LIKE ? OR template_key LIKE ? OR last_error LIKE ? OR CAST(recipient_user_id AS CHAR) LIKE ?",
			pattern, pattern, pattern, pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	records := make([]DeliveryRecord, 0, limit)
	err := query.
		Select("delivery_public_id, recipient_user_id, channel, purpose, status, attempt_count, last_error_code, last_error, next_attempt_at, processed_at, created_at").
		Order("id DESC").Limit(limit).Offset(offset).Scan(&records).Error
	if err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

func (r *GormRepository) RetryDelivery(ctx context.Context, deliveryPublicID string, now time.Time) (DeliveryRecord, error) {
	if r == nil || r.db == nil || deliveryPublicID == "" || !r.db.Migrator().HasTable(&notification.DeliveryItem{}) {
		return DeliveryRecord{}, ErrDeliveryNotRetryable
	}
	var item notification.DeliveryItem
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("delivery_public_id = ?", deliveryPublicID).First(&item).Error; err != nil {
			return err
		}
		if item.Status != notification.DeliveryBlocked && item.Status != notification.DeliveryDeadletter {
			return ErrDeliveryNotRetryable
		}
		return tx.Model(&notification.DeliveryItem{}).Where("id = ?", item.ID).Updates(map[string]any{
			"status":           notification.DeliveryPending,
			"attempt_count":    0,
			"next_attempt_at":  now.UTC(),
			"lease_token":      nil,
			"lease_expires_at": nil,
			// 重投必须刷新有效期，否则 worker 会因过期直接跳过，形成"重投成功但永不发送"。
			"expires_at":    now.UTC().Add(2 * time.Hour),
			"row_version":   gorm.Expr("row_version + 1"),
			"updated_at":    now.UTC(),
		}).Error
	})
	if err != nil {
		return DeliveryRecord{}, err
	}
	return DeliveryRecord{
		DeliveryPublicID: item.DeliveryPublicID, RecipientUserID: item.RecipientUserID,
		Channel: item.Channel, Purpose: item.Purpose, Status: notification.DeliveryPending,
		AttemptCount: 0, LastErrorCode: item.LastErrorCode, LastError: item.LastError,
		NextAttemptAt: &now, ProcessedAt: nil, CreatedAt: item.CreatedAt,
	}, nil
}

func (r *GormRepository) OfficialAccountCounts(ctx context.Context, appID string) (OfficialAccountCounts, error) {
	result := OfficialAccountCounts{}
	if r == nil || r.db == nil || appID == "" || !r.db.Migrator().HasTable(&notification.OfficialAccountBinding{}) {
		return result, nil
	}
	values := []struct {
		FollowState string
		Count       int64
	}{}
	if err := r.db.WithContext(ctx).Model(&notification.OfficialAccountBinding{}).
		Select("follow_state, COUNT(*) AS count").Where("app_id = ?", appID).
		Group("follow_state").Scan(&values).Error; err != nil {
		return result, err
	}
	for _, value := range values {
		switch value.FollowState {
		case "following":
			result.Following = value.Count
		case "not_following":
			result.NotFollowing = value.Count
		default:
			result.Unknown += value.Count
		}
	}
	return result, nil
}
