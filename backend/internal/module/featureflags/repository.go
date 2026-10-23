package featureflags

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

type ConfigRow struct {
	ID                uint      `gorm:"primaryKey"`
	Forum             bool      `gorm:"not null"`
	Post              bool      `gorm:"not null"`
	Comment           bool      `gorm:"not null"`
	Secondhand        bool      `gorm:"not null"`
	SecondhandPublish bool      `gorm:"not null"`
	SecondhandChat    bool      `gorm:"not null"`
	Takeout           bool      `gorm:"not null"`
	CampusTools       bool      `gorm:"not null"`
	Delivery          bool      `gorm:"not null"`
	Merchant          bool      `gorm:"not null"`
	Rider             bool      `gorm:"not null"`
	Messages          bool      `gorm:"not null"`
	Following         bool      `gorm:"not null"`
	ExtraFlags        string    `gorm:"type:json;not null"`
	PostingMode       string    `gorm:"size:16;not null"`
	PresentationMode  string    `gorm:"size:24;not null;default:standard"`
	RowVersion        uint64    `gorm:"not null"`
	UpdatedBy         uint      `gorm:"not null"`
	UpdatedAt         time.Time `gorm:"not null"`
	CreatedAt         time.Time `gorm:"not null"`
}

func (ConfigRow) TableName() string { return "feature_policy_configs" }

type AuditRow struct {
	ID         uint      `gorm:"primaryKey"`
	ActorID    uint      `gorm:"not null;index"`
	BeforeJSON string    `gorm:"type:json;not null"`
	AfterJSON  string    `gorm:"type:json;not null"`
	CreatedAt  time.Time `gorm:"not null;index"`
}

func (AuditRow) TableName() string { return "feature_policy_audit_logs" }

type GormStore struct{ db *gorm.DB }

func NewGormStore(db *gorm.DB) *GormStore { return &GormStore{db: db} }

func (s *GormStore) Ensure(ctx context.Context) error {
	m := s.db.WithContext(ctx).Migrator()
	if !m.HasTable(&ConfigRow{}) || !m.HasTable(&AuditRow{}) || !m.HasColumn(&ConfigRow{}, "extra_flags") || !m.HasColumn(&ConfigRow{}, "presentation_mode") {
		return errors.New("feature policy schema missing; apply migrations 038, 039 and 041 before starting the server")
	}
	return nil
}

func (s *GormStore) Load(ctx context.Context) (Policy, bool, error) {
	var row ConfigRow
	err := s.db.WithContext(ctx).First(&row, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Policy{}, false, nil
	}
	if err != nil {
		return Policy{}, false, err
	}
	value := rowPolicy(row)
	if !validPresentationMode(value.PresentationMode) {
		return Policy{}, false, ErrInvalidPresentationMode
	}
	if err := json.Unmarshal([]byte(row.ExtraFlags), &value.Flags); err != nil {
		return Policy{}, false, err
	}
	return value, true, nil
}

func (s *GormStore) CreateIfMissing(ctx context.Context, value Policy) error {
	row := policyRow(value)
	row.ID = 1
	row.ExtraFlags = extraFlagsJSON(value)
	return s.db.WithContext(ctx).Create(&row).Error
}

func (s *GormStore) Save(ctx context.Context, before, after Policy) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ConfigRow{}).Where("id = ? AND row_version = ?", 1, before.RowVersion).Updates(map[string]any{
			"forum": after.Flags[KeyForum], "post": after.Flags[KeyPost], "comment": after.Flags[KeyComment],
			"secondhand": after.Flags[KeySecondhand], "secondhand_publish": after.Flags[KeySecondhandPublish],
			"secondhand_chat": after.Flags[KeySecondhandChat], "takeout": after.Flags[KeyTakeout],
			"campus_tools": after.Flags[KeyCampusTools], "delivery": after.Flags[KeyDelivery],
			"merchant": after.Flags[KeyMerchant], "rider": after.Flags[KeyRider], "messages": after.Flags[KeyMessages],
			"following": after.Flags[KeyFollowing], "posting_mode": after.PostingMode, "row_version": after.RowVersion,
			"updated_by": after.UpdatedBy, "updated_at": after.UpdatedAt,
			"extra_flags":       extraFlagsJSON(after),
			"presentation_mode": after.PresentationMode,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrVersionConflict
		}
		beforeJSON, _ := json.Marshal(before)
		afterJSON, _ := json.Marshal(after)
		return tx.Create(&AuditRow{ActorID: after.UpdatedBy, BeforeJSON: string(beforeJSON), AfterJSON: string(afterJSON), CreatedAt: after.UpdatedAt}).Error
	})
}

func policyRow(value Policy) ConfigRow {
	return ConfigRow{ID: 1, Forum: value.Flags[KeyForum], Post: value.Flags[KeyPost], Comment: value.Flags[KeyComment], Secondhand: value.Flags[KeySecondhand], SecondhandPublish: value.Flags[KeySecondhandPublish], SecondhandChat: value.Flags[KeySecondhandChat], Takeout: value.Flags[KeyTakeout], CampusTools: value.Flags[KeyCampusTools], Delivery: value.Flags[KeyDelivery], Merchant: value.Flags[KeyMerchant], Rider: value.Flags[KeyRider], Messages: value.Flags[KeyMessages], Following: value.Flags[KeyFollowing], PostingMode: value.PostingMode, PresentationMode: value.PresentationMode, RowVersion: value.RowVersion, UpdatedBy: value.UpdatedBy, UpdatedAt: value.UpdatedAt, CreatedAt: time.Now().UTC()}
}

func rowPolicy(row ConfigRow) Policy {
	return Policy{Flags: map[string]bool{KeyForum: row.Forum, KeyPost: row.Post, KeyComment: row.Comment, KeySecondhand: row.Secondhand, KeySecondhandPublish: row.SecondhandPublish, KeySecondhandChat: row.SecondhandChat, KeyTakeout: row.Takeout, KeyCampusTools: row.CampusTools, KeyDelivery: row.Delivery, KeyMerchant: row.Merchant, KeyRider: row.Rider, KeyMessages: row.Messages, KeyFollowing: row.Following}, PostingMode: row.PostingMode, PresentationMode: row.PresentationMode, RowVersion: row.RowVersion, UpdatedBy: row.UpdatedBy, UpdatedAt: row.UpdatedAt, LabEnabled: true}
}

func extraFlagsJSON(value Policy) string {
	flags := make(map[string]bool)
	for _, key := range []string{KeySearch, KeyLikes, KeyFavorites, KeyNotifications, KeyIdentityApplication, KeyPublish, KeySecondhandComments} {
		flags[key] = value.Flags[key]
	}
	data, _ := json.Marshal(flags)
	return string(data)
}
