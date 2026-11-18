package notificationconsole

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// WeComSettings is the administrator-managed WeCom webhook configuration.
// Saved=false means no administrator ever saved settings, so env config wins.
type WeComSettings struct {
	MerchantWebhook string
	RiderWebhook    string
	RowVersion      uint64
	Saved           bool
	UpdatedAt       time.Time
}

var ErrSettingsConflict = errors.New("notification settings changed")

type WeComSettingsStore interface {
	LoadWeComSettings(context.Context) (WeComSettings, error)
	SaveWeComSettings(context.Context, string, string, uint64, uint64, time.Time) (WeComSettings, error)
}

type GormWeComSettingsStore struct{ db *gorm.DB }

func NewWeComSettingsStore(db *gorm.DB) *GormWeComSettingsStore {
	return &GormWeComSettingsStore{db: db}
}

var _ WeComSettingsStore = (*GormWeComSettingsStore)(nil)

func (s *GormWeComSettingsStore) LoadWeComSettings(ctx context.Context) (WeComSettings, error) {
	result := WeComSettings{}
	if s == nil || s.db == nil || !s.db.Migrator().HasTable("notification_channel_settings") {
		return result, nil
	}
	var row struct {
		MerchantWebhook  string
		RiderWebhook     string
		RowVersion       uint64
		UpdatedBy        *uint64
		UpdatedByAdminID *uint64
		UpdatedAt        time.Time
	}
	if err := s.db.WithContext(ctx).Raw(`SELECT wecom_merchant_webhook AS merchant_webhook, wecom_rider_webhook AS rider_webhook, row_version, updated_by, updated_by_admin_id, updated_at FROM notification_channel_settings WHERE id = 1`).Scan(&row).Error; err != nil {
		return WeComSettings{}, err
	}
	return WeComSettings{
		MerchantWebhook: row.MerchantWebhook,
		RiderWebhook:    row.RiderWebhook,
		RowVersion:      row.RowVersion,
		Saved:           row.UpdatedBy != nil || row.UpdatedByAdminID != nil,
		UpdatedAt:       row.UpdatedAt,
	}, nil
}

func (s *GormWeComSettingsStore) SaveWeComSettings(ctx context.Context, merchant, rider string, expectedRowVersion, actor uint64, now time.Time) (WeComSettings, error) {
	if s == nil || s.db == nil {
		return WeComSettings{}, ErrSettingsConflict
	}
	var result WeComSettings
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row struct {
			RowVersion uint64
		}
		if err := tx.Table("notification_channel_settings").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = 1").Take(&row).Error; err != nil {
			return err
		}
		if row.RowVersion != expectedRowVersion {
			return ErrSettingsConflict
		}
		if err := tx.Table("notification_channel_settings").
			Where("id = 1 AND row_version = ?", expectedRowVersion).
			Updates(map[string]any{
				"wecom_merchant_webhook": merchant,
				"wecom_rider_webhook":    rider,
				"row_version":            gorm.Expr("row_version + 1"),
				"updated_by_admin_id":    actor,
				"updated_at":             now.UTC(),
			}).Error; err != nil {
			return err
		}
		result = WeComSettings{
			MerchantWebhook: merchant, RiderWebhook: rider,
			RowVersion: expectedRowVersion + 1, Saved: true, UpdatedAt: now.UTC(),
		}
		return nil
	})
	if err != nil {
		return WeComSettings{}, err
	}
	return result, nil
}
