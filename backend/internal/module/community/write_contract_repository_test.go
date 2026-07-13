package community

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
)

func TestWriteContractHelpers(t *testing.T) {
	key, err := newWriteContractULID(time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), zeroReader{})
	if err != nil {
		t.Fatal(err)
	}
	if !validWriteContractULID(key) || len(key) != 26 {
		t.Fatalf("generated invalid ULID %q", key)
	}
	hash := sha256.Sum256([]byte("post"))
	if !validCreateCoordinates(&key, hash[:]) {
		t.Fatal("valid create coordinates rejected")
	}
	if validCreateCoordinates(nil, hash[:]) || validCreateCoordinates(&key, hash[:31]) {
		t.Fatal("invalid create coordinates accepted")
	}
	if !isWriteContractDuplicate(&mysql.MySQLError{Number: 1062}) {
		t.Fatal("duplicate key was not recognized")
	}
	if isWriteContractDuplicate(errors.New("other")) {
		t.Fatal("non-duplicate error was misclassified")
	}
	if (WriteOutbox{}).TableName() != "community_write_outbox" {
		t.Fatal("unexpected outbox table name")
	}
}

func TestCommunityCreateOrReplayMySQL(t *testing.T) {
	db := openCommunityWriteContractMySQL(t)
	repo := NewRepository(db)
	ctx := context.Background()
	stamp := time.Now().UnixNano()
	owner := user.User{OpenID: fmt.Sprintf("ui21b2a-write-owner-%d", stamp), Nickname: "write owner"}
	commenter := user.User{OpenID: fmt.Sprintf("ui21b2a-write-commenter-%d", stamp), Nickname: "write commenter"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&commenter).Error; err != nil {
		db.Unscoped().Delete(&owner)
		t.Fatal(err)
	}
	ownerID := owner.ID
	commenterID := commenter.ID
	defer func() {
		cleanupCommunityWriteContractRows(t, db, ownerID, commenterID)
		db.Unscoped().Delete(&commenter)
		db.Unscoped().Delete(&owner)
	}()

	key, _ := newWriteContractULID(time.Now().UTC(), zeroReader{})
	hash := sha256.Sum256([]byte("same-post"))
	const workers = 12
	var firsts atomic.Int32
	ids := make(chan uint, workers)
	errs := make(chan error, workers)
	var wait sync.WaitGroup
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			value := Post{
				UserID: ownerID, ClientRequestID: &key, CreateRequestHash: append([]byte(nil), hash[:]...),
				Title: "keyed post", Content: "content", ReviewStatus: review.StatusSubmitted,
				Images: []PostImage{{URL: "https://example.invalid/post.jpg"}},
			}
			event := WriteOutbox{EventType: "community_post_created", Payload: `{}`}
			replayed, err := repo.CreatePostOrReplay(ctx, &value, &event)
			if err == nil {
				ids <- value.ID
				if !replayed {
					firsts.Add(1)
				}
			}
			errs <- err
		}()
	}
	wait.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent create failed: %v", err)
		}
	}
	if firsts.Load() != 1 {
		t.Fatalf("expected one first create, got %d", firsts.Load())
	}
	var postID uint
	for id := range ids {
		if postID == 0 {
			postID = id
		} else if id != postID {
			t.Fatalf("replay returned different post IDs: %d and %d", postID, id)
		}
	}
	var postCount, outboxCount, imageCount int64
	if err := db.Model(&Post{}).Where("user_id = ? AND client_request_id = ?", ownerID, key).Count(&postCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&WriteOutbox{}).Where("owner_user_id = ? AND aggregate_type = ?", ownerID, "post").Count(&outboxCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&PostImage{}).Where("post_id = ?", postID).Count(&imageCount).Error; err != nil {
		t.Fatal(err)
	}
	if postCount != 1 || outboxCount != 1 || imageCount != 1 {
		t.Fatalf("unexpected atomic create counts post=%d outbox=%d images=%d", postCount, outboxCount, imageCount)
	}
	foundPost, found, err := repo.FindPostByRequest(ctx, ownerID, key)
	if err != nil || !found || foundPost.ID != postID || len(foundPost.Images) != 1 {
		t.Fatalf("preflight lookup failed: found=%v post=%+v err=%v", found, foundPost, err)
	}

	conflictHash := sha256.Sum256([]byte("different-post"))
	conflict := Post{UserID: ownerID, ClientRequestID: &key, CreateRequestHash: conflictHash[:], Title: "other"}
	if _, err := repo.CreatePostOrReplay(ctx, &conflict, &WriteOutbox{EventType: "community_post_created", Payload: `{}`}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}

	commentKey, _ := newWriteContractULID(time.Now().UTC().Add(time.Millisecond), oneReader{})
	commentHash := sha256.Sum256([]byte("comment"))
	const commentWorkers = 20
	var commentFirsts atomic.Int32
	commentIDs := make(chan uint, commentWorkers)
	commentErrors := make(chan error, commentWorkers)
	wait = sync.WaitGroup{}
	for index := 0; index < commentWorkers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			value := Comment{
				PostID: postID, UserID: commenterID, ClientRequestID: &commentKey,
				CreateRequestHash: append([]byte(nil), commentHash[:]...), Content: "hello", ReviewStatus: review.StatusSubmitted,
			}
			replayed, createErr := repo.CreateCommentOrReplay(ctx, &value, &WriteOutbox{EventType: "community_comment_created", Payload: `{}`})
			if createErr == nil {
				commentIDs <- value.ID
				if !replayed {
					commentFirsts.Add(1)
				}
			}
			commentErrors <- createErr
		}()
	}
	wait.Wait()
	close(commentIDs)
	close(commentErrors)
	for createErr := range commentErrors {
		if createErr != nil {
			t.Fatalf("concurrent comment create failed: %v", createErr)
		}
	}
	if commentFirsts.Load() != 1 {
		t.Fatalf("expected one first comment create, got %d", commentFirsts.Load())
	}
	var commentID uint
	for id := range commentIDs {
		if commentID == 0 {
			commentID = id
		} else if id != commentID {
			t.Fatalf("comment replay returned different IDs: %d and %d", commentID, id)
		}
	}
	comment := Comment{ID: commentID}
	var commentCount, commentOutboxCount int64
	if err := db.Model(&Comment{}).Where("user_id = ? AND client_request_id = ?", commenterID, commentKey).Count(&commentCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&WriteOutbox{}).Where("owner_user_id = ? AND aggregate_type = ?", commenterID, "comment").Count(&commentOutboxCount).Error; err != nil {
		t.Fatal(err)
	}
	if commentCount != 1 || commentOutboxCount != 1 {
		t.Fatalf("unexpected comment atomic counts comment=%d outbox=%d", commentCount, commentOutboxCount)
	}
	var storedPost Post
	if err := db.First(&storedPost, postID).Error; err != nil {
		t.Fatal(err)
	}
	if storedPost.CommentCount != 0 {
		t.Fatalf("create/replay must not pre-apply approval side effect, count=%d", storedPost.CommentCount)
	}
	if err := repo.SetReviewStatus(ctx, "comment", comment.ID, review.StatusApproved); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetReviewStatus(ctx, "comment", comment.ID, review.StatusApproved); err != nil {
		t.Fatal(err)
	}
	var notifications int64
	if err := db.Model(&Notification{}).Where("post_id = ? AND comment_id = ?", postID, comment.ID).Count(&notifications).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&storedPost, postID).Error; err != nil {
		t.Fatal(err)
	}
	if storedPost.CommentCount != 1 || notifications != 1 {
		t.Fatalf("approval side effect duplicated: count=%d notifications=%d", storedPost.CommentCount, notifications)
	}
}

func openCommunityWriteContractMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("TEST_MYSQL_DSN is not set")
	}
	db, err := gorm.Open(gormmysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	return db
}

func cleanupCommunityWriteContractRows(t *testing.T, db *gorm.DB, ownerIDs ...uint) {
	t.Helper()
	var postIDs []uint
	if err := db.Unscoped().Model(&Post{}).Where("user_id IN ?", ownerIDs).Pluck("id", &postIDs).Error; err != nil {
		t.Errorf("load cleanup post IDs: %v", err)
		return
	}
	if len(postIDs) > 0 {
		db.Where("post_id IN ?", postIDs).Delete(&Notification{})
		db.Where("post_id IN ?", postIDs).Delete(&PostImage{})
		db.Where("post_id IN ?", postIDs).Delete(&Comment{})
	}
	db.Where("owner_user_id IN ?", ownerIDs).Delete(&WriteOutbox{})
	db.Unscoped().Where("user_id IN ?", ownerIDs).Delete(&Comment{})
	db.Unscoped().Where("user_id IN ?", ownerIDs).Delete(&Post{})
}

type zeroReader struct{}

func (zeroReader) Read(value []byte) (int, error) {
	clear(value)
	return len(value), nil
}

type oneReader struct{}

func (oneReader) Read(value []byte) (int, error) {
	for index := range value {
		value[index] = 1
	}
	return len(value), nil
}
