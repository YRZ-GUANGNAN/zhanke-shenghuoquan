package featureboard

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository persists suggestions, supports and author notes. All database
// access for the feature board lives here, including the support toggle
// transaction, so handlers and services never touch *gorm.DB.
type Repository interface {
	ListUser(ctx context.Context, sort string, offset, limit int, viewerID uint64) ([]SuggestionRecord, int64, error)
	ListAdmin(ctx context.Context, status string, offset, limit int) ([]SuggestionRecord, int64, error)
	CreateSuggestion(ctx context.Context, value *Suggestion) error
	SetSupport(ctx context.Context, publicID string, userID uint64, supported bool, at time.Time) (Suggestion, error)
	FindSuggestionByPublicID(ctx context.Context, publicID string) (Suggestion, error)
	CreateComment(ctx context.Context, value *Comment) error
	ListComments(ctx context.Context, suggestionID uint64, offset, limit int) ([]Comment, int64, error)
	ListNotes(ctx context.Context, limit int) ([]AuthorNote, error)
	CreateNote(ctx context.Context, value *AuthorNote) error
	DeleteNote(ctx context.Context, publicID string) (bool, error)
	SetStatus(ctx context.Context, publicID string, status SuggestionStatus, at time.Time) (Suggestion, error)
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) Repository { return &GormRepository{db: db} }

func visibleSuggestionStatuses() []SuggestionStatus {
	return []SuggestionStatus{StatusOpen, StatusAccepted}
}

func (r *GormRepository) ListUser(ctx context.Context, sort string, offset, limit int, viewerID uint64) ([]SuggestionRecord, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&Suggestion{}).
		Where("status IN ?", visibleSuggestionStatuses()).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := "support_count DESC, id DESC"
	if sort == "latest" {
		order = "id DESC"
	}
	rows := make([]SuggestionRecord, 0, limit)
	err := r.db.WithContext(ctx).Model(&Suggestion{}).
		Select("feature_suggestions.*, EXISTS(SELECT 1 FROM feature_suggestion_supports s WHERE s.suggestion_id = feature_suggestions.id AND s.user_id = ?) AS supported, (SELECT COUNT(*) FROM feature_suggestion_comments c WHERE c.suggestion_id = feature_suggestions.id) AS comment_count", viewerID).
		Where("status IN ?", visibleSuggestionStatuses()).
		Order(order).Offset(offset).Limit(limit).Find(&rows).Error
	return rows, total, err
}

func (r *GormRepository) ListAdmin(ctx context.Context, status string, offset, limit int) ([]SuggestionRecord, int64, error) {
	base := func() *gorm.DB {
		query := r.db.WithContext(ctx).Model(&Suggestion{})
		if status != "" {
			query = query.Where("status = ?", status)
		}
		return query
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]SuggestionRecord, 0, limit)
	err := base().Select("feature_suggestions.*, (SELECT COUNT(*) FROM feature_suggestion_comments c WHERE c.suggestion_id = feature_suggestions.id) AS comment_count").
		Order("id DESC").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, total, err
}

func (r *GormRepository) CreateSuggestion(ctx context.Context, value *Suggestion) error {
	return r.db.WithContext(ctx).Create(value).Error
}

func (r *GormRepository) SetSupport(ctx context.Context, publicID string, userID uint64, supported bool, at time.Time) (Suggestion, error) {
	var result Suggestion
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var suggestion Suggestion
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("public_id = ?", publicID).First(&suggestion).Error; err != nil {
			return err
		}
		if suggestion.Status == StatusClosed {
			result = suggestion
			return ErrClosed
		}
		if supported {
			insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&Support{
				SuggestionID: suggestion.ID, UserID: userID, CreatedAt: at,
			})
			if insert.Error != nil {
				return insert.Error
			}
			if insert.RowsAffected > 0 {
				if err := tx.Model(&Suggestion{}).Where("id = ?", suggestion.ID).
					Updates(map[string]any{
						"support_count": gorm.Expr("IF(support_count < 4294967295, support_count + 1, support_count)"),
						"updated_at":    at,
					}).Error; err != nil {
					return err
				}
				suggestion.SupportCount++
				suggestion.UpdatedAt = at
			}
		} else {
			removed := tx.Where("suggestion_id = ? AND user_id = ?", suggestion.ID, userID).Delete(&Support{})
			if removed.Error != nil {
				return removed.Error
			}
			if removed.RowsAffected > 0 {
				if err := tx.Model(&Suggestion{}).Where("id = ?", suggestion.ID).
					Updates(map[string]any{
						"support_count": gorm.Expr("IF(support_count > 0, support_count - 1, 0)"),
						"updated_at":    at,
					}).Error; err != nil {
					return err
				}
				if suggestion.SupportCount > 0 {
					suggestion.SupportCount--
				}
				suggestion.UpdatedAt = at
			}
		}
		result = suggestion
		return nil
	})
	if err != nil {
		return Suggestion{}, err
	}
	return result, nil
}

func (r *GormRepository) ListNotes(ctx context.Context, limit int) ([]AuthorNote, error) {
	notes := make([]AuthorNote, 0, limit)
	err := r.db.WithContext(ctx).Model(&AuthorNote{}).Order("id DESC").Limit(limit).Find(&notes).Error
	return notes, err
}

func (r *GormRepository) CreateNote(ctx context.Context, value *AuthorNote) error {
	return r.db.WithContext(ctx).Create(value).Error
}

func (r *GormRepository) DeleteNote(ctx context.Context, publicID string) (bool, error) {
	removed := r.db.WithContext(ctx).Where("public_id = ?", publicID).Delete(&AuthorNote{})
	return removed.RowsAffected > 0, removed.Error
}

func (r *GormRepository) SetStatus(ctx context.Context, publicID string, status SuggestionStatus, at time.Time) (Suggestion, error) {
	updated := r.db.WithContext(ctx).Model(&Suggestion{}).
		Where("public_id = ?", publicID).
		Updates(map[string]any{"status": status, "updated_at": at})
	if updated.Error != nil {
		return Suggestion{}, updated.Error
	}
	var suggestion Suggestion
	if err := r.db.WithContext(ctx).Where("public_id = ?", publicID).First(&suggestion).Error; err != nil {
		return Suggestion{}, err
	}
	return suggestion, nil
}

func (r *GormRepository) FindSuggestionByPublicID(ctx context.Context, publicID string) (Suggestion, error) {
	var suggestion Suggestion
	if err := r.db.WithContext(ctx).Where("public_id = ?", publicID).First(&suggestion).Error; err != nil {
		return Suggestion{}, err
	}
	return suggestion, nil
}

func (r *GormRepository) CreateComment(ctx context.Context, value *Comment) error {
	return r.db.WithContext(ctx).Create(value).Error
}

func (r *GormRepository) ListComments(ctx context.Context, suggestionID uint64, offset, limit int) ([]Comment, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&Comment{}).
		Where("suggestion_id = ?", suggestionID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]Comment, 0, limit)
	err := r.db.WithContext(ctx).Where("suggestion_id = ?", suggestionID).
		Order("id ASC").Offset(offset).Limit(limit).Find(&rows).Error
	return rows, total, err
}
