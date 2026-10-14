package community

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCommentInteractionsTemporaryMySQL(t *testing.T) {
	dsn := os.Getenv("COMMENT_INTERACTIONS_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires dedicated temporary MySQL")
	}
	config, err := driver.ParseDSN(dsn)
	if err != nil || !strings.HasPrefix(config.DBName, "campus_comments_test_") || config.Addr != "127.0.0.1:33308" {
		t.Fatal("refusing non-temporary comment database")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(12)
	if err = db.AutoMigrate(&Post{}, &PostImage{}, &PostLike{}, &PostFavorite{}, &UserFollow{}, &Notification{}, &WriteOutbox{}); err != nil {
		t.Fatal(err)
	}
	// Start with the previous comments schema and apply the actual migration.
	if err = db.Exec(`CREATE TABLE comments (
	 id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, post_id BIGINT UNSIGNED NOT NULL, user_id BIGINT UNSIGNED NOT NULL,
	 content TEXT, review_status VARCHAR(24) NOT NULL DEFAULT 'submitted', client_request_id CHAR(26) NULL, create_request_hash BINARY(32) NULL,
	 created_at DATETIME(3), updated_at DATETIME(3), UNIQUE KEY idx_comment_request(user_id,client_request_id)
	) ENGINE=InnoDB`).Error; err != nil {
		t.Fatal(err)
	}
	if err = db.Exec("INSERT INTO comments(id,post_id,user_id,content,review_status) VALUES(1,1,2,'existing','approved')").Error; err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../../../migrations/043_community_comment_interactions.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range strings.Split(string(migration), ";") {
		if strings.TrimSpace(statement) != "" {
			if err = db.Exec(statement).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	var legacy Comment
	if err = db.First(&legacy, 1).Error; err != nil || legacy.ParentCommentID != 0 || legacy.LikedCount != 0 {
		t.Fatalf("legacy migration: %+v %v", legacy, err)
	}
	post := Post{ID: 1, UserID: 1, ReviewStatus: review.StatusApproved, CommentCount: 1}
	if err = db.Create(&post).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	service := NewService(repo, profileDirectory{2: {ID: 2, Nickname: "作者"}, 7: {ID: 7, Nickname: "本人"}}, &submissionRecorder{})
	ctx := context.Background()
	reply, err := service.ReplyCommentKeyed(ctx, 1, 1, 7, "真实回复协议测试", communityTestRequestID, "")
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.ReplyCommentKeyed(ctx, 1, 1, 7, "真实回复协议测试", communityTestRequestID, "")
	if err != nil || !again.Replayed || reply.Value.ID != again.Value.ID {
		t.Fatalf("replay: %+v %v", again, err)
	}
	var outboxCount int64
	db.Model(&WriteOutbox{}).Where("aggregate_id = ? AND aggregate_type = 'comment'", reply.Value.ID).Count(&outboxCount)
	if outboxCount != 1 {
		t.Fatalf("outbox count = %d", outboxCount)
	}
	for _, viewer := range []uint{7, 8} {
		page, err := service.CommentReplies(ctx, 1, 1, viewer, 1, 20)
		if err != nil || (viewer == 7 && len(page.Items) != 1) || (viewer == 8 && len(page.Items) != 0) {
			t.Fatalf("private reply visibility: %+v %v", page, err)
		}
	}
	if _, err = repo.SetCommentLiked(ctx, 1, reply.Value.ID, 8, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("liked pending: %v", err)
	}
	assertCount := func(want int64) {
		t.Helper()
		var value Post
		if err := db.First(&value, 1).Error; err != nil || value.CommentCount != want {
			t.Fatalf("public comment count: got %d want %d err %v", value.CommentCount, want, err)
		}
	}
	assertCount(1)
	if err = repo.SetReviewStatus(ctx, "comment", reply.Value.ID, review.StatusApproved); err != nil {
		t.Fatal(err)
	}
	assertCount(2)
	runConcurrent := func(count int, run func(int) error) {
		t.Helper()
		var wg sync.WaitGroup
		failures := make(chan error, count)
		for i := 0; i < count; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				if e := run(i); e != nil {
					failures <- e
				}
			}(i)
		}
		wg.Wait()
		close(failures)
		for e := range failures {
			t.Fatal(e)
		}
	}
	runConcurrent(20, func(int) error { _, e := repo.SetCommentLiked(ctx, 1, reply.Value.ID, 8, true); return e })
	state, err := repo.SetCommentLiked(ctx, 1, reply.Value.ID, 8, true)
	if err != nil || !state.Liked || state.LikedCount != 1 {
		t.Fatalf("same-user PUT: %+v %v", state, err)
	}
	runConcurrent(12, func(i int) error { _, e := repo.SetCommentLiked(ctx, 1, reply.Value.ID, uint(20+i), true); return e })
	state, err = repo.SetCommentLiked(ctx, 1, reply.Value.ID, 8, true)
	if err != nil || state.LikedCount != 13 {
		t.Fatalf("many-user PUT: %+v %v", state, err)
	}
	runConcurrent(20, func(int) error { _, e := repo.SetCommentLiked(ctx, 1, reply.Value.ID, 8, false); return e })
	state, err = repo.SetCommentLiked(ctx, 1, reply.Value.ID, 8, false)
	if err != nil || state.Liked || state.LikedCount != 12 {
		t.Fatalf("same-user DELETE: %+v %v", state, err)
	}
	if _, err = repo.SetCommentLiked(ctx, 99, reply.Value.ID, 8, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-post like: %v", err)
	}
	runConcurrent(20, func(int) error { _, e := repo.SetLikeState(ctx, 1, 8, true); return e })
	postLike, err := repo.SetLikeState(ctx, 1, 8, true)
	viewer, viewerErr := repo.ViewerState(ctx, 1, 8, 1)
	if err != nil || viewerErr != nil || !postLike.Liked || postLike.LikedCount != 1 || !viewer.Liked || viewer.LikedCount != 1 {
		t.Fatalf("post authoritative snapshot: %+v %+v %v %v", postLike, viewer, err, viewerErr)
	}
	for i := 0; i < 22; i++ {
		value := Comment{PostID: 1, UserID: 7, ParentCommentID: 1, ReplyToCommentID: reply.Value.ID, ReplyToUserID: 7, Content: fmt.Sprint(i), ReviewStatus: review.StatusSubmitted}
		if err = db.Create(&value).Error; err != nil {
			t.Fatal(err)
		}
		if err = repo.SetReviewStatus(ctx, "comment", value.ID, review.StatusApproved); err != nil {
			t.Fatal(err)
		}
	}
	first, err := service.CommentReplies(ctx, 1, 1, 8, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CommentReplies(ctx, 1, 1, 8, 2, 20)
	if err != nil || first.Total != 23 || len(first.Items) != 20 || len(second.Items) != 3 || first.Items[19].ID >= second.Items[0].ID {
		t.Fatalf("pagination: %+v %+v %v", first, second, err)
	}
	roots, total, err := repo.ListComments(ctx, 1, 1, 20, true)
	if err != nil || total != 1 || len(roots) != 1 {
		t.Fatal("legacy list exposed replies")
	}
	assertCount(24)
	if err = repo.SetReviewStatus(ctx, "comment", 1, ReviewStatusTakenDown); err != nil {
		t.Fatal(err)
	}
	assertCount(0)
	if _, err = service.CommentReplies(ctx, 1, 1, 7, 1, 20); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removed root exposed children: %v", err)
	}
	if _, err = repo.SetCommentLiked(ctx, 1, reply.Value.ID, 8, true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removed root like: %v", err)
	}
	if err = repo.SetReviewStatus(ctx, "comment", reply.Value.ID, review.StatusRejected); err != nil {
		t.Fatal(err)
	}
	assertCount(0)
	if err = repo.SetReviewStatus(ctx, "comment", 1, review.StatusApproved); err != nil {
		t.Fatal(err)
	}
	assertCount(23)
	if err = repo.AdminTakedownPost(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err = service.CommentThreads(ctx, 1, 7, 1, 20); !errors.Is(err, ErrNotFound) {
		t.Fatalf("removed post exposed threads: %v", err)
	}
	t.Log("PASS: actual migration, legacy roots, moderation/outbox, reply paging, cross-post guards, concurrent PUT/DELETE and authoritative post counts")
}
