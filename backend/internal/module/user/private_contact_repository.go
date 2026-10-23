package user

import "context"

func (r *GormRepository) HasBlockBetween(ctx context.Context, firstID, secondID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&UserBlock{}).
		Where("(user_id = ? AND blocked_user_id = ?) OR (user_id = ? AND blocked_user_id = ?)", firstID, secondID, secondID, firstID).
		Count(&count).Error
	return count > 0, err
}
