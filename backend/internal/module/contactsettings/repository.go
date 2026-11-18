package contactsettings

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	Ensure(ctx context.Context, at time.Time) error
	Get(ctx context.Context) (Settings, error)
	Update(ctx context.Context, input UpdateInput, at time.Time) (Settings, error)
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &GormRepository{db: db} }

// Ensure creates the single configuration row when a deployment missed the
// migration, so the public read never dead-ends.
func (r *GormRepository) Ensure(ctx context.Context, at time.Time) error {
	row := Row{ID: 1, MakerWechat: DefaultMakerWechat, MakerIntro: DefaultMakerIntro, RowVersion: 1, UpdatedAt: at}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

func (r *GormRepository) Get(ctx context.Context) (Settings, error) {
	var row Row
	if err := r.db.WithContext(ctx).Where("id = 1").First(&row).Error; err != nil {
		return Settings{}, err
	}
	return row.Settings(), nil
}

func (r *GormRepository) Update(ctx context.Context, input UpdateInput, at time.Time) (Settings, error) {
	updated := r.db.WithContext(ctx).Model(&Row{}).
		Where("id = 1 AND row_version = ?", input.ExpectedRowVersion).
		Updates(map[string]any{
			"maker_wechat": input.MakerWechat,
			"maker_intro":  input.MakerIntro,
			"row_version":  gorm.Expr("row_version + 1"),
			"updated_at":   at,
		})
	if updated.Error != nil {
		return Settings{}, updated.Error
	}
	if updated.RowsAffected == 0 {
		var exists Row
		if err := r.db.WithContext(ctx).Where("id = 1").First(&exists).Error; err != nil {
			return Settings{}, err
		}
		return Settings{}, ErrConflict
	}
	return r.Get(ctx)
}
