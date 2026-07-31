package trade

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"
	"time"

	"campus-circle/backend/internal/module/admin/review"
)

func TestProductCommentRepositoryCreateReplayMySQL(t *testing.T) {
	db := openTradeWriteContractMySQL(t)
	if !db.Migrator().HasTable(&ProductComment{}) {
		t.Skip("migration 036 is not applied")
	}
	repo := NewRepository(db)
	ctx := context.Background()
	nonce := time.Now().UnixNano()
	userID := uint(880000000 + nonce%10000000)
	product := Product{
		SellerID: userID + 1, Title: fmt.Sprintf("comment-test-%d", nonce), Price: 1,
		Status: StatusSelling, ReviewStatus: review.StatusApproved,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	t.Cleanup(func() {
		db.Where("aggregate_type = ? AND owner_user_id = ?", ProductCommentAggregateType, userID).Delete(&WriteOutbox{})
		db.Unscoped().Where("user_id = ?", userID).Delete(&ProductComment{})
		db.Unscoped().Delete(&product)
	})

	hash := sha256.Sum256([]byte("same"))
	value := ProductComment{
		ProductID: product.ID, UserID: userID, Content: "hello", ReviewStatus: review.StatusSubmitted,
		ClientRequestID: productCommentTestRequestID, RequestHash: hash[:],
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	event := WriteOutbox{EventType: ProductCommentCreatedEventType, Payload: `{}`}
	replayed, err := repo.CreateProductCommentOrReplay(ctx, &value, &event)
	if err != nil || replayed || value.ID == 0 {
		t.Fatalf("create comment: id=%d replayed=%v err=%v", value.ID, replayed, err)
	}
	if event.AggregateType != ProductCommentAggregateType || event.AggregateID != uint64(value.ID) || event.OwnerUserID != uint64(userID) {
		t.Fatalf("unexpected outbox coordinates: %#v", event)
	}

	replay := ProductComment{
		ProductID: product.ID, UserID: userID, Content: "hello", ReviewStatus: review.StatusSubmitted,
		ClientRequestID: productCommentTestRequestID, RequestHash: hash[:],
	}
	replayEvent := WriteOutbox{EventType: ProductCommentCreatedEventType, Payload: `{}`}
	replayed, err = repo.CreateProductCommentOrReplay(ctx, &replay, &replayEvent)
	if err != nil || !replayed || replay.ID != value.ID || replayEvent.ID != event.ID {
		t.Fatalf("replay comment: value=%#v event=%#v replayed=%v err=%v", replay, replayEvent, replayed, err)
	}

	changedHash := sha256.Sum256([]byte("changed"))
	conflict := replay
	conflict.RequestHash = changedHash[:]
	conflict.ID = 0
	if _, err := repo.CreateProductCommentOrReplay(ctx, &conflict, &WriteOutbox{
		EventType: ProductCommentCreatedEventType, Payload: `{}`,
	}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected payload conflict, got %v", err)
	}
}

func TestProductCommentRepositoryReviewNotificationMySQL(t *testing.T) {
	db := openTradeWriteContractMySQL(t)
	if !db.Migrator().HasTable(&ProductComment{}) || !db.Migrator().HasColumn("notifications", "product_comment_id") {
		t.Skip("migration 036 is not applied")
	}
	repo := NewRepository(db)
	nonce := time.Now().UnixNano()
	sellerID := uint(870000000 + nonce%10000000)
	actorID := sellerID + 1
	product := Product{
		SellerID: sellerID, Title: fmt.Sprintf("notification-test-%d", nonce), Price: 1,
		Status: StatusSelling, ReviewStatus: review.StatusApproved,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	comment := ProductComment{
		ProductID: product.ID, UserID: actorID, Content: "hello", ReviewStatus: review.StatusSubmitted,
		ClientRequestID: productCommentTestRequestID, RequestHash: make([]byte, 32),
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := db.Create(&comment).Error; err != nil {
		t.Fatalf("create comment: %v", err)
	}
	t.Cleanup(func() {
		db.Where("product_id = ?", product.ID).Delete(&productCommentNotificationRecord{})
		db.Unscoped().Where("product_id = ?", product.ID).Delete(&ProductComment{})
		db.Unscoped().Delete(&product)
	})

	if err := repo.SetProductCommentReviewStatus(context.Background(), comment.ID, review.StatusApproved); err != nil {
		t.Fatalf("approve comment: %v", err)
	}
	if err := repo.SetProductCommentReviewStatus(context.Background(), comment.ID, review.StatusApproved); err != nil {
		t.Fatalf("replay approval: %v", err)
	}
	var count int64
	if err := db.Model(&productCommentNotificationRecord{}).
		Where("recipient_id = ? AND actor_id = ? AND product_id = ? AND product_comment_id = ?",
			sellerID, actorID, product.ID, comment.ID).Count(&count).Error; err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one idempotent notification, got %d", count)
	}
}

var _ ProductCommentRepository = (*GormRepository)(nil)
