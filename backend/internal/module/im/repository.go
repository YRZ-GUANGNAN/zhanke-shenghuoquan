package im

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	EnsureMapping(context.Context, *UserMapping) error
	FindMapping(context.Context, uint) (UserMapping, error)
	EnsureConversation(context.Context, *Conversation) error
	ListConversations(context.Context, uint, int, int) ([]Conversation, int64, error)
}
type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }
func (r *GormRepository) EnsureMapping(ctx context.Context, value *UserMapping) error {
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"im_user_id", "updated_at"}),
	}).Create(value).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Where("user_id = ?", value.UserID).First(value).Error
}
func (r *GormRepository) FindMapping(ctx context.Context, userID uint) (UserMapping, error) {
	var value UserMapping
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&value).Error
	return value, err
}
func (r *GormRepository) EnsureConversation(ctx context.Context, value *Conversation) error {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "buyer_id"}, {Name: "seller_id"}, {Name: "reference_type"}, {Name: "reference_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"conversation_id", "updated_at"}),
	}).Create(value)
	if result.Error != nil {
		return result.Error
	}
	return r.db.WithContext(ctx).Where("buyer_id = ? AND seller_id = ? AND reference_type = ? AND reference_id = ?", value.BuyerID, value.SellerID, value.ReferenceType, value.ReferenceID).First(value).Error
}
func (r *GormRepository) ListConversations(ctx context.Context, userID uint, page, size int) ([]Conversation, int64, error) {
	var values []Conversation
	var total int64
	query := r.db.WithContext(ctx).Model(&Conversation{}).Where("buyer_id = ? OR seller_id = ?", userID, userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("updated_at DESC").Offset((page - 1) * size).Limit(size).Find(&values).Error
	return values, total, err
}
