package community

import (
	"context"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type motionMySQLProvider struct{ submissions int }

func (p *motionMySQLProvider) SubmitVideo(context.Context, coremedia.Asset) (string, error) {
	p.submissions++
	return "mysql-video-job", nil
}
func (p *motionMySQLProvider) QueryVideo(context.Context, string) (coremedia.VideoReviewResult, error) {
	return coremedia.VideoReviewResult{}, nil
}

func TestPostMotionTemporaryMySQL(t *testing.T) {
	dsn := os.Getenv("POST_MOTION_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires an isolated post-motion database")
	}
	config, err := mysqldriver.ParseDSN(dsn)
	if err != nil || config.Addr != "127.0.0.1:33308" || !regexp.MustCompile(`^campus_motion_test_[0-9]+$`).MatchString(config.DBName) {
		t.Fatal("refusing non-isolated database")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	if err = db.AutoMigrate(&Post{}, &PostImage{}, &WriteOutbox{}, &coremedia.Asset{}, &coremedia.UploadCompletion{}); err != nil {
		t.Fatal(err)
	}
	// Start from the previous schema and apply the actual additive migration,
	// twice, with a historical image/text record already present.
	for _, target := range []struct {
		model any
		field string
	}{{&Post{}, "Motion"}, {&coremedia.Asset{}, "DurationMS"}, {&coremedia.UploadCompletion{}, "VerifiedDurationMS"}} {
		if err := db.Migrator().DropColumn(target.model, target.field); err != nil {
			t.Fatal(err)
		}
	}
	if err = db.Exec("INSERT INTO posts (user_id,title,content,topic,review_status) VALUES (1,'legacy','unchanged','campus','approved')").Error; err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../../migrations/047_post_motion_media.sql")
	if err != nil {
		t.Fatal(err)
	}
	script := regexp.MustCompile(`(?m)^DELIMITER[^\r\n]*`).ReplaceAllString(string(raw), "")
	for repeat := 0; repeat < 2; repeat++ {
		for _, statement := range strings.Split(script, "$$") {
			if strings.TrimSpace(statement) == "" {
				continue
			}
			if err = db.Exec(statement).Error; err != nil {
				t.Fatalf("migration pass %d: %v", repeat, err)
			}
		}
	}
	repo := NewRepository(db)
	legacy, err := repo.FindPost(context.Background(), 1, true)
	if err != nil || legacy.Content != "unchanged" || legacy.Motion != nil {
		t.Fatalf("migration changed legacy post: %+v %v", legacy, err)
	}
	requestID := "01J00000000000000000000009"
	value := Post{UserID: 7, ClientRequestID: &requestID, CreateRequestHash: make([]byte, 32), Title: "video", Content: "motion body", Topic: "campus", ReviewStatus: "submitted",
		Motion: &PostMotion{Kind: "video", URL: "/clip", PosterURL: "/poster", Width: 720, Height: 1280, DurationMS: 6000}, Images: []PostImage{{URL: "/poster"}}}
	event := WriteOutbox{EventPublicID: requestID, AggregateType: "post", OwnerUserID: 7, EventType: "post_created", Payload: `{}`, Status: WriteOutboxPending, AvailableAt: time.Now(), RowVersion: 1}
	if _, err = repo.CreatePostOrReplay(context.Background(), &value, &event); err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.FindPost(context.Background(), value.ID, false)
	if err != nil || loaded.Motion == nil || *loaded.Motion != *value.Motion || len(loaded.Images) != 1 {
		t.Fatalf("motion JSON roundtrip: %+v %v", loaded, err)
	}
	if _, found, err := repo.FindPostByRequest(context.Background(), 7, requestID); err != nil || !found {
		t.Fatalf("keyed lookup: %v", err)
	}
	provider := &motionMySQLProvider{}
	videoReview, _ := coremedia.NewVideoReviewService(coremedia.NewRepository(db), provider)
	asset := coremedia.Asset{AssetPublicID: requestID, Purpose: coremedia.PurposePost, VerifiedMIME: "video/mp4", TechnicalStatus: coremedia.TechnicalFinalReady}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := videoReview.ReviewVideo(context.Background(), asset); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if provider.submissions != 1 {
		t.Fatalf("concurrent requests submitted %d jobs", provider.submissions)
	}
	var job coremedia.VideoReviewJob
	if err = db.First(&job, "asset_public_id = ?", requestID).Error; err != nil || job.JobID != "mysql-video-job" || job.Decision != coremedia.ProjectionPending {
		t.Fatalf("durable job: %+v %v", job, err)
	}
}
