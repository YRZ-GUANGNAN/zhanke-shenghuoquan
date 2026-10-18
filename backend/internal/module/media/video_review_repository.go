package media

import (
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// One durable job per immutable asset. Network calls are bounded by the caller
// and serialized by this row lock, so concurrent completion/status requests do
// not create competing decisions. A crash before saving a submission may create
// an unused cloud job on retry; it can never approve a different asset.
type VideoReviewJob struct {
	AssetPublicID string           `gorm:"size:26;primaryKey"`
	JobID         string           `gorm:"size:128;not null"`
	Decision      ProjectionStatus `gorm:"size:24;not null"`
	FailureCode   string           `gorm:"size:64;not null"`
	NextPollAt    time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (VideoReviewJob) TableName() string { return "media_video_review_jobs" }

type VideoReviewRepository interface {
	WithVideoJob(context.Context, string, time.Time, func(*VideoReviewJob) error) error
}

func (r *GormRepository) WithVideoJob(ctx context.Context, id string, now time.Time, fn func(*VideoReviewJob) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		seed := VideoReviewJob{AssetPublicID: id, Decision: ProjectionPending, NextPollAt: now, CreatedAt: now, UpdatedAt: now}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
			return err
		}
		var job VideoReviewJob
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("asset_public_id = ?", id).First(&job).Error; err != nil {
			return err
		}
		if err := fn(&job); err != nil {
			return err
		}
		job.UpdatedAt = now
		return tx.Save(&job).Error
	})
}
