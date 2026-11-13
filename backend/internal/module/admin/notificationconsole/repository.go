package notificationconsole

import (
	"context"
	"time"

	"campus-circle/backend/internal/module/notification"
	"gorm.io/gorm"
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

type Repository interface {
	ChannelStats(context.Context, time.Time) ([]ChannelStat, error)
	OfficialAccountCounts(context.Context, string) (OfficialAccountCounts, error)
}

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
