package trade

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"time"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/pkg/dbtx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	productCommentNotificationType = "product_commented"
	productReplyNotificationType   = "product_comment_replied"
)

type ProductCommentRepository interface {
	FindProductComment(context.Context, uint) (ProductComment, error)
	FindProductCommentByRequest(context.Context, uint, string) (ProductComment, bool, error)
	CreateProductCommentOrReplay(context.Context, *ProductComment, *WriteOutbox) (bool, error)
	ListProductCommentThreads(context.Context, uint, uint, int, int) ([]ProductComment, int64, error)
	ListProductCommentReplies(context.Context, []uint, uint) ([]ProductComment, error)
	CountRecentProductComments(context.Context, uint, time.Time) (int64, error)
	HasRecentDuplicateProductComment(context.Context, uint, uint, []byte, time.Time) (bool, error)
	SetProductCommentReviewStatus(context.Context, uint, review.Status) error
}

var _ ProductCommentRepository = (*GormRepository)(nil)

func (r *GormRepository) FindProductForCommentHistory(ctx context.Context, id uint) (Product, error) {
	var value Product
	err := r.db.WithContext(ctx).Unscoped().
		Select("id", "seller_id", "status", "review_status", "deleted_at").
		First(&value, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Product{}, ErrNotFound
	}
	return value, err
}

func (r *GormRepository) FindProductComment(ctx context.Context, id uint) (ProductComment, error) {
	var value ProductComment
	err := r.db.WithContext(ctx).First(&value, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ProductComment{}, ErrNotFound
	}
	return value, err
}

func (r *GormRepository) FindProductCommentByRequest(ctx context.Context, userID uint, clientRequestID string) (ProductComment, bool, error) {
	if userID == 0 || !validWriteContractULID(clientRequestID) {
		return ProductComment{}, false, ErrWriteContractInvalid
	}
	var value ProductComment
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND client_request_id = ?", userID, clientRequestID).
		First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ProductComment{}, false, nil
	}
	return value, err == nil, err
}

func (r *GormRepository) CreateProductCommentOrReplay(ctx context.Context, value *ProductComment, event *WriteOutbox) (bool, error) {
	if value == nil || event == nil || value.UserID == 0 || value.ProductID == 0 ||
		!validWriteContractULID(value.ClientRequestID) || len(value.RequestHash) != 32 {
		return false, ErrWriteContractInvalid
	}
	replayed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		create := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(value)
		if create.Error != nil {
			return create.Error
		}
		if create.RowsAffected == 0 {
			var existing ProductComment
			if err := tx.Where("user_id = ? AND client_request_id = ?", value.UserID, value.ClientRequestID).
				First(&existing).Error; err != nil {
				return err
			}
			if !bytes.Equal(existing.RequestHash, value.RequestHash) {
				return ErrIdempotencyConflict
			}
			*value = existing
			if err := loadProductCommentWriteOutbox(tx, uint64(existing.ID), event.EventType, event); err != nil {
				return err
			}
			replayed = true
			return nil
		}
		if value.ParentCommentID != 0 {
			if err := tx.Model(&ProductComment{}).Where("id = ?", value.ParentCommentID).
				UpdateColumn("updated_at", value.CreatedAt).Error; err != nil {
				return err
			}
		}
		return insertProductCommentWriteOutbox(tx, event, uint64(value.ID), uint64(value.UserID))
	})
	return replayed, err
}

func insertProductCommentWriteOutbox(tx *gorm.DB, event *WriteOutbox, aggregateID, ownerID uint64) error {
	now := time.Now().UTC()
	if event.EventPublicID == "" {
		publicID, err := newWriteContractULID(now, rand.Reader)
		if err != nil {
			return err
		}
		event.EventPublicID = publicID
	}
	if event.EventType != ProductCommentCreatedEventType || !json.Valid([]byte(event.Payload)) {
		return ErrWriteContractInvalid
	}
	event.AggregateType = ProductCommentAggregateType
	event.AggregateID = aggregateID
	event.OwnerUserID = ownerID
	event.Status = WriteOutboxPending
	event.AttemptCount = 0
	event.AvailableAt = now
	event.LeaseToken = nil
	event.LeaseExpiresAt = nil
	event.ProcessedAt = nil
	event.FailedAt = nil
	event.LastError = ""
	event.RowVersion = 1
	event.CreatedAt = now
	event.UpdatedAt = now
	return tx.Create(event).Error
}

func loadProductCommentWriteOutbox(tx *gorm.DB, aggregateID uint64, eventType string, result *WriteOutbox) error {
	if eventType != ProductCommentCreatedEventType {
		return ErrWriteContractInvalid
	}
	err := tx.Where("aggregate_type = ? AND aggregate_id = ? AND event_type = ?",
		ProductCommentAggregateType, aggregateID, eventType).First(result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrWriteOutboxInvariant
	}
	return err
}

func (r *GormRepository) ListProductCommentThreads(ctx context.Context, productID, viewerID uint, page, pageSize int) ([]ProductComment, int64, error) {
	query := r.db.WithContext(ctx).Model(&ProductComment{}).
		Where("product_id = ? AND parent_comment_id = 0", productID).
		Where("(review_status = ? OR user_id = ?)", review.StatusApproved, viewerID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	values := make([]ProductComment, 0, pageSize)
	err := query.Order("updated_at DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&values).Error
	return values, total, err
}

func (r *GormRepository) ListProductCommentReplies(ctx context.Context, parentIDs []uint, viewerID uint) ([]ProductComment, error) {
	if len(parentIDs) == 0 {
		return []ProductComment{}, nil
	}
	values := make([]ProductComment, 0)
	err := r.db.WithContext(ctx).
		Where("parent_comment_id IN ?", parentIDs).
		Where("(review_status = ? OR user_id = ?)", review.StatusApproved, viewerID).
		Order("created_at ASC, id ASC").Find(&values).Error
	return values, err
}

func (r *GormRepository) CountRecentProductComments(ctx context.Context, userID uint, since time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&ProductComment{}).
		Where("user_id = ? AND created_at >= ?", userID, since).Count(&count).Error
	return count, err
}

func (r *GormRepository) HasRecentDuplicateProductComment(ctx context.Context, userID, productID uint, requestHash []byte, since time.Time) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&ProductComment{}).
		Where("user_id = ? AND product_id = ? AND request_hash = ? AND created_at >= ?",
			userID, productID, requestHash, since).Count(&count).Error
	return count > 0, err
}

type productCommentNotificationRecord struct {
	ID               uint   `gorm:"primaryKey"`
	RecipientID      uint   `gorm:"uniqueIndex:idx_notification_event"`
	ActorID          uint   `gorm:"uniqueIndex:idx_notification_event"`
	Type             string `gorm:"size:32;uniqueIndex:idx_notification_event"`
	PostID           uint   `gorm:"uniqueIndex:idx_notification_event"`
	CommentID        uint   `gorm:"uniqueIndex:idx_notification_event"`
	ProductID        uint   `gorm:"uniqueIndex:idx_notification_event"`
	ProductCommentID uint   `gorm:"uniqueIndex:idx_notification_event"`
	CreatedAt        time.Time
}

func (productCommentNotificationRecord) TableName() string { return "notifications" }

func (r *GormRepository) SetProductCommentReviewStatus(ctx context.Context, id uint, status review.Status) error {
	return dbtx.DB(ctx, r.db).Transaction(func(tx *gorm.DB) error {
		var comment ProductComment
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&comment, id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil || comment.ReviewStatus == status {
			return err
		}
		var product Product
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&product, comment.ProductID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if err := tx.Model(&ProductComment{}).Where("id = ?", id).
			Update("review_status", status).Error; err != nil {
			return err
		}
		if status != review.StatusApproved {
			return nil
		}
		recipientID, notificationType := product.SellerID, productCommentNotificationType
		if comment.ParentCommentID != 0 {
			recipientID, notificationType = comment.ReplyToUserID, productReplyNotificationType
		}
		if recipientID == 0 || recipientID == comment.UserID {
			return nil
		}
		notification := productCommentNotificationRecord{
			RecipientID: recipientID, ActorID: comment.UserID, Type: notificationType,
			ProductID: comment.ProductID, ProductCommentID: comment.ID, CreatedAt: time.Now().UTC(),
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&notification).Error
	})
}
