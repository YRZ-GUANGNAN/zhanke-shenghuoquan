package sharesettings

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

type ConfigRow struct {
	ID         uint `gorm:"primaryKey"`
	Cards      `gorm:"embedded"`
	RowVersion uint64
	UpdatedBy  uint
	UpdatedAt  time.Time
}

func (ConfigRow) TableName() string { return "share_settings" }

type AuditRow struct {
	ID         uint `gorm:"primaryKey"`
	ActorID    uint
	BeforeJSON string
	AfterJSON  string
	CreatedAt  time.Time
}

func (AuditRow) TableName() string { return "share_settings_audit_logs" }

type Store interface {
	Load(context.Context) (Settings, error)
	Save(context.Context, uint, UpdateInput) (Settings, error)
}

type GormStore struct{ db *gorm.DB }

func NewGormStore(db *gorm.DB) *GormStore { return &GormStore{db: db} }

func (s *GormStore) Ensure(ctx context.Context) error {
	m := s.db.WithContext(ctx).Migrator()
	if !m.HasTable(&ConfigRow{}) || !m.HasTable(&AuditRow{}) {
		return errors.New("share settings schema missing; apply migration 040 before starting the server")
	}
	_, err := s.Load(ctx)
	return err
}

func (s *GormStore) Load(ctx context.Context) (Settings, error) {
	var row ConfigRow
	if err := s.db.WithContext(ctx).First(&row, 1).Error; err != nil {
		return Settings{}, err
	}
	return Settings{Cards: row.Cards, RowVersion: row.RowVersion, UpdatedAt: row.UpdatedAt}, nil
}

func (s *GormStore) Save(ctx context.Context, actor uint, input UpdateInput) (Settings, error) {
	var saved Settings
	if actor == 0 || input.ExpectedRowVersion == 0 {
		return saved, ErrInvalid
	}
	if err := input.Cards.Validate(); err != nil {
		return saved, err
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		before, err := NewGormStore(tx).Load(ctx)
		if err != nil {
			return err
		}
		if before.RowVersion != input.ExpectedRowVersion {
			return ErrConflict
		}
		saved = Settings{Cards: input.Cards, RowVersion: before.RowVersion + 1, UpdatedAt: time.Now().UTC()}
		result := tx.Model(&ConfigRow{}).Where("id = 1 AND row_version = ?", before.RowVersion).Updates(map[string]any{
			"home_title": saved.HomeTitle, "secondhand_title": saved.SecondhandTitle, "services_title": saved.ServicesTitle,
			"default_image_url": saved.DefaultImageURL, "detail_title_suffix": saved.DetailTitleSuffix, "prefer_content_image": saved.PreferContentImage,
			"row_version": saved.RowVersion, "updated_by": actor, "updated_at": saved.UpdatedAt,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrConflict
		}
		beforeJSON, _ := json.Marshal(before)
		afterJSON, _ := json.Marshal(saved)
		return tx.Create(&AuditRow{ActorID: actor, BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON), CreatedAt: saved.UpdatedAt}).Error
	})
	return saved, err
}
