package community

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestMySQLTargetStateConcurrencyDoesNotDrift(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("dedicated MySQL integration DSN is not configured")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().UnixNano()
	owner := user.User{OpenID: fmt.Sprintf("ui21b2a-owner-%d", stamp), Nickname: "owner"}
	viewer := user.User{OpenID: fmt.Sprintf("ui21b2a-viewer-%d", stamp), Nickname: "viewer"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&viewer).Error; err != nil {
		db.Unscoped().Delete(&owner)
		t.Fatal(err)
	}
	ownerID := owner.ID
	viewerID := viewer.ID
	post := Post{
		UserID: ownerID, Title: fmt.Sprintf("target-state-%d", stamp), Content: "target state concurrency",
		ReviewStatus: review.StatusApproved,
	}
	if err := db.Create(&post).Error; err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.Where("post_id = ?", post.ID).Delete(&PostLike{})
		db.Where("post_id = ?", post.ID).Delete(&PostFavorite{})
		db.Where("post_id = ? OR (recipient_id = ? AND actor_id = ?)", post.ID, ownerID, viewerID).Delete(&Notification{})
		db.Where("follower_id = ? AND followee_id = ?", viewerID, ownerID).Delete(&UserFollow{})
		db.Unscoped().Delete(&Post{}, post.ID)
		db.Unscoped().Delete(&viewer)
		db.Unscoped().Delete(&owner)
	}()

	repo := NewRepository(db)
	runConcurrent := func(name string, operation func() error) {
		t.Helper()
		var wg sync.WaitGroup
		errorsCh := make(chan error, 20)
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				errorsCh <- operation()
			}()
		}
		wg.Wait()
		close(errorsCh)
		for operationErr := range errorsCh {
			if operationErr != nil {
				t.Fatalf("%s failed: %v", name, operationErr)
			}
		}
	}

	runConcurrent("set like", func() error {
		_, operationErr := repo.SetLike(context.Background(), post.ID, viewerID, true)
		return operationErr
	})
	runConcurrent("set favorite", func() error {
		_, operationErr := repo.SetFavorite(context.Background(), post.ID, viewerID, true)
		return operationErr
	})
	runConcurrent("set follow", func() error {
		_, operationErr := repo.SetFollow(context.Background(), viewerID, ownerID, true)
		return operationErr
	})

	assertCommunityTargetState(t, db, post.ID, viewerID, ownerID, 1, 1)
	var notificationRows int64
	if err := db.Model(&Notification{}).
		Where("recipient_id = ? AND actor_id = ? AND (post_id = ? OR type = ?)", ownerID, viewerID, post.ID, NotificationFollow).
		Count(&notificationRows).Error; err != nil {
		t.Fatal(err)
	}
	if notificationRows != 2 {
		t.Fatalf("target-state retries duplicated notifications: rows=%d", notificationRows)
	}

	runConcurrent("unset like", func() error {
		_, operationErr := repo.SetLike(context.Background(), post.ID, viewerID, false)
		return operationErr
	})
	runConcurrent("unset favorite", func() error {
		_, operationErr := repo.SetFavorite(context.Background(), post.ID, viewerID, false)
		return operationErr
	})
	runConcurrent("unset follow", func() error {
		_, operationErr := repo.SetFollow(context.Background(), viewerID, ownerID, false)
		return operationErr
	})

	assertCommunityTargetState(t, db, post.ID, viewerID, ownerID, 0, 0)
}

func assertCommunityTargetState(t *testing.T, db *gorm.DB, postID, viewerID, ownerID uint, expectedRelationCount, expectedAggregate int64) {
	t.Helper()
	var likeRows, favoriteRows, followRows int64
	if err := db.Model(&PostLike{}).Where("post_id = ? AND user_id = ?", postID, viewerID).Count(&likeRows).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&PostFavorite{}).Where("post_id = ? AND user_id = ?", postID, viewerID).Count(&favoriteRows).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&UserFollow{}).Where("follower_id = ? AND followee_id = ?", viewerID, ownerID).Count(&followRows).Error; err != nil {
		t.Fatal(err)
	}
	var stored Post
	if err := db.First(&stored, postID).Error; err != nil {
		t.Fatal(err)
	}
	if likeRows != expectedRelationCount || favoriteRows != expectedRelationCount || followRows != expectedRelationCount ||
		stored.LikedCount != expectedAggregate || stored.FavoriteCount != expectedAggregate {
		t.Fatalf("target state drift: likes=%d favorites=%d follows=%d liked_count=%d favorite_count=%d",
			likeRows, favoriteRows, followRows, stored.LikedCount, stored.FavoriteCount)
	}
}
