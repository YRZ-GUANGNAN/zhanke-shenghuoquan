package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const module5HashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const module5HashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func Test00ReviewProjectionMigrationAndRollbackAgainstMySQL84(t *testing.T) {
	db := openModule5MySQLRaw(t)
	assertMySQL84(t, db)
	if db.Migrator().HasTable(&ReviewProjection{}) {
		var projectionCount, decisionCount int64
		if err := db.Model(&ReviewProjection{}).Count(&projectionCount).Error; err != nil {
			t.Fatal(err)
		}
		if db.Migrator().HasTable(&ReviewDecision{}) {
			if err := db.Model(&ReviewDecision{}).Count(&decisionCount).Error; err != nil {
				t.Fatal(err)
			}
		}
		if projectionCount != 0 || decisionCount != 0 {
			t.Fatalf("refusing rollback against non-empty Module 5 tables: projections=%d decisions=%d", projectionCount, decisionCount)
		}
	}
	applyModule5SQLFile(t, db, "../../../migrations/rollback/012_media_review_projection.sql")
	if db.Migrator().HasTable(&ReviewProjection{}) || db.Migrator().HasTable(&ReviewDecision{}) {
		t.Fatal("rollback left Module 5 tables behind")
	}
	applyModule5SQLFile(t, db, "../../../migrations/012_media_review_projection.sql")
	if !db.Migrator().HasTable(&ReviewProjection{}) || !db.Migrator().HasTable(&ReviewDecision{}) {
		t.Fatal("forward migration did not create both Module 5 tables")
	}
	assertModule5SchemaContract(t, db)
}

func TestReviewProjectionLifecycleAndRepositoryAgainstMySQL84(t *testing.T) {
	db := openModule5MySQL(t)
	ownerID := uint64(99_000_501)
	cleanupReviewOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupReviewOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 21, 16, 30, 0, 0, time.UTC)
	repository := NewRepository(db)
	service, _ := NewReviewProjectionService(repository, func() time.Time { return now })
	asset := createModule5FinalAsset(t, repository, ownerID, "lifecycle", now)
	originalFinalKey, originalFinalVersion := *asset.FinalKey, *asset.FinalVersionID

	projection, created, err := service.EnsureProjection(context.Background(), asset.AssetPublicID)
	if err != nil || !created || projection.Status != ProjectionPending || projection.RowVersion != 1 {
		t.Fatalf("ensure projection created=%v projection=%#v err=%v", created, projection, err)
	}
	replayed, replayCreated, err := service.EnsureProjection(context.Background(), asset.AssetPublicID)
	if err != nil || replayCreated || replayed.ProjectionID != projection.ProjectionID {
		t.Fatalf("ensure projection was not idempotent created=%v projection=%#v err=%v", replayCreated, replayed, err)
	}

	projection, err = service.Submit(context.Background(), projection.ProjectionID, 51_001, projection.RowVersion)
	if err != nil || projection.Status != ProjectionSubmitted || projection.RowVersion != 2 || projection.ReviewTaskID == nil {
		t.Fatalf("submit failed: %#v err=%v", projection, err)
	}
	projection, err = service.StartReview(context.Background(), projection.ProjectionID, projection.RowVersion)
	if err != nil || projection.Status != ProjectionReviewing || projection.RowVersion != 3 {
		t.Fatalf("reviewing failed: %#v err=%v", projection, err)
	}
	decision := ApplyReviewDecisionInput{
		ProjectionID: projection.ProjectionID, ReviewTaskID: 51_001, DecisionVersion: 7,
		ExpectedStatus: ProjectionReviewing, ExpectedRowVersion: projection.RowVersion,
		Decision: ProjectionApproved, PayloadHash: module5HashA,
	}
	projection, err = service.RecordDecision(context.Background(), decision)
	if err != nil || projection.Status != ProjectionApproved || projection.AppliedDecisionVersion != 7 || projection.RowVersion != 4 {
		t.Fatalf("decision failed: %#v err=%v", projection, err)
	}
	duplicate, err := service.RecordDecision(context.Background(), decision)
	if err != nil || duplicate.ProjectionID != projection.ProjectionID || duplicate.RowVersion != projection.RowVersion {
		t.Fatalf("duplicate decision was not idempotent: %#v err=%v", duplicate, err)
	}

	var loaded Asset
	if err := db.First(&loaded, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if loaded.ReviewStatus != ReviewPending || loaded.RowVersion != asset.RowVersion || loaded.TechnicalStatus != TechnicalFinalReady ||
		loaded.StorageStage != StorageStageFinal || loaded.FinalKey == nil || *loaded.FinalKey != originalFinalKey ||
		loaded.FinalVersionID == nil || *loaded.FinalVersionID != originalFinalVersion {
		t.Fatalf("review projection mutated historical asset facts: %#v", loaded)
	}
	var decisions int64
	if err := db.Model(&ReviewDecision{}).Where("projection_id = ?", projection.ProjectionID).Count(&decisions).Error; err != nil {
		t.Fatal(err)
	}
	if decisions != 1 {
		t.Fatalf("immutable decision count drifted: %d", decisions)
	}
}

func TestReviewProjectionIllegalTransitionsAndAssetInvariantAgainstMySQL84(t *testing.T) {
	db := openModule5MySQL(t)
	ownerID := uint64(99_000_502)
	cleanupReviewOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupReviewOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 21, 17, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	service, _ := NewReviewProjectionService(repository, func() time.Time { return now })
	asset := createModule5FinalAsset(t, repository, ownerID, "illegal", now)
	projection, _, err := service.EnsureProjection(context.Background(), asset.AssetPublicID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartReview(context.Background(), projection.ProjectionID, projection.RowVersion); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("pending -> reviewing returned %v", err)
	}
	if err := db.Model(&Asset{}).Where("id = ?", asset.ID).Update("technical_status", TechnicalDeletePending).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.Submit(context.Background(), projection.ProjectionID, 52_001, projection.RowVersion); !errors.Is(err, ErrReviewInvariant) {
		t.Fatalf("non-final technical asset was not fail-closed: %v", err)
	}
	loaded, err := repository.FindReviewProjection(context.Background(), projection.ProjectionID)
	if err != nil || loaded.Status != ProjectionPending || loaded.RowVersion != 1 {
		t.Fatalf("failed transaction changed projection: %#v err=%v", loaded, err)
	}
}

func TestReviewProjectionConcurrentIdempotencyAndCASAgainstMySQL84(t *testing.T) {
	db := openModule5MySQL(t)
	ownerID := uint64(99_000_503)
	cleanupReviewOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupReviewOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 21, 17, 30, 0, 0, time.UTC)
	repository := NewRepository(db)
	service, _ := NewReviewProjectionService(repository, func() time.Time { return now })
	asset := createModule5FinalAsset(t, repository, ownerID, "concurrency", now)
	projection, _, err := service.EnsureProjection(context.Background(), asset.AssetPublicID)
	if err != nil {
		t.Fatal(err)
	}

	var success, unexpected atomic.Int32
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := service.Submit(context.Background(), projection.ProjectionID, 53_001, projection.RowVersion)
			switch {
			case err == nil:
				success.Add(1)
			default:
				unexpected.Add(1)
			}
		}()
	}
	close(start)
	wait.Wait()
	if success.Load() != 2 || unexpected.Load() != 0 {
		t.Fatalf("concurrent idempotency success=%d unexpected=%d", success.Load(), unexpected.Load())
	}
	loaded, err := repository.FindReviewProjection(context.Background(), projection.ProjectionID)
	if err != nil || loaded.Status != ProjectionSubmitted || loaded.RowVersion != 2 {
		t.Fatalf("idempotent concurrency advanced more than once: %#v err=%v", loaded, err)
	}

	staleAsset := createModule5FinalAsset(t, repository, ownerID, "stale-cas", now)
	staleProjection, _, err := service.EnsureProjection(context.Background(), staleAsset.AssetPublicID)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ReviewProjection{}).Where("projection_id = ?", staleProjection.ProjectionID).
		Update("row_version", gorm.Expr("row_version + 1")).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.Submit(context.Background(), staleProjection.ProjectionID, 53_002, staleProjection.RowVersion); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale row_version was not rejected: %v", err)
	}
}

func TestReviewProjectionDecisionConflictAndFailedPathAgainstMySQL84(t *testing.T) {
	db := openModule5MySQL(t)
	ownerID := uint64(99_000_504)
	cleanupReviewOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupReviewOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 21, 18, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	service, _ := NewReviewProjectionService(repository, func() time.Time { return now })
	asset := createModule5FinalAsset(t, repository, ownerID, "failed-decision", now)
	projection, _, _ := service.EnsureProjection(context.Background(), asset.AssetPublicID)
	projection, _ = service.Submit(context.Background(), projection.ProjectionID, 54_001, projection.RowVersion)
	decision := ApplyReviewDecisionInput{
		ProjectionID: projection.ProjectionID, ReviewTaskID: 54_001, DecisionVersion: 1,
		ExpectedStatus: ProjectionSubmitted, ExpectedRowVersion: projection.RowVersion,
		Decision: ProjectionReviewFailed, PayloadHash: module5HashA, ResultCode: "wechat_timeout",
	}
	projection, err := service.RecordDecision(context.Background(), decision)
	if err != nil || projection.Status != ProjectionReviewFailed {
		t.Fatalf("review_failed path failed: %#v err=%v", projection, err)
	}
	conflicting := decision
	conflicting.PayloadHash = module5HashB
	if _, err := service.RecordDecision(context.Background(), conflicting); !errors.Is(err, ErrDecisionConflict) {
		t.Fatalf("same decision version conflict returned %v", err)
	}
	var count int64
	if err := db.Model(&ReviewDecision{}).Where("projection_id = ?", projection.ProjectionID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("conflicting retry persisted another fact: %d", count)
	}
}

func openModule5MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	db := openModule5MySQLRaw(t)
	if !db.Migrator().HasTable(&ReviewProjection{}) || !db.Migrator().HasTable(&ReviewDecision{}) {
		applyModule5SQLFile(t, db, "../../../migrations/012_media_review_projection.sql")
	}
	return db
}

func openModule5MySQLRaw(t *testing.T) *gorm.DB {
	t.Helper()
	_ = godotenv.Load("../../../.env")
	dsn := os.Getenv("MEDIA_MODULE5_MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("set MEDIA_MODULE5_MYSQL_DSN or MYSQL_DSN to run Module 5 MySQL 8.4 acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertMySQL84(t *testing.T, db *gorm.DB) {
	t.Helper()
	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.4.") {
		t.Fatalf("Module 5 requires MySQL 8.4, got %s", version)
	}
	t.Logf("real database version: %s", version)
}

func assertModule5SchemaContract(t *testing.T, db *gorm.DB) {
	t.Helper()
	requiredIndexes := map[string]bool{
		"uq_media_review_projection_asset":            false,
		"uq_media_review_projection_task":             false,
		"idx_media_review_projection_status":          false,
		"uq_media_review_decision_task_version":       false,
		"uq_media_review_decision_projection_version": false,
	}
	var indexes []string
	if err := db.Raw(`SELECT DISTINCT index_name FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name IN ('media_review_projections','media_review_decisions')`).Scan(&indexes).Error; err != nil {
		t.Fatal(err)
	}
	for _, name := range indexes {
		if _, ok := requiredIndexes[name]; ok {
			requiredIndexes[name] = true
		}
	}
	for name, found := range requiredIndexes {
		if !found {
			t.Fatalf("required Module 5 index missing: %s", name)
		}
	}
	var foreignKeys []string
	if err := db.Raw(`SELECT DISTINCT referenced_table_name FROM information_schema.key_column_usage
		WHERE table_schema = DATABASE() AND table_name IN ('media_review_projections','media_review_decisions')
		AND referenced_table_name IS NOT NULL`).Scan(&foreignKeys).Error; err != nil {
		t.Fatal(err)
	}
	for _, referencedTable := range foreignKeys {
		if referencedTable != "media_assets" && referencedTable != "media_review_projections" {
			t.Fatalf("cross-domain foreign key found: %s", referencedTable)
		}
	}
	if len(foreignKeys) != 2 {
		t.Fatalf("unexpected Module 5 foreign key count: %d", len(foreignKeys))
	}
	t.Logf("schema contract: %d required indexes, %d media-only foreign keys", len(requiredIndexes), len(foreignKeys))
}

func applyModule5SQLFile(t *testing.T, db *gorm.DB, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range strings.Split(string(raw), ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("execute %s: %v", path, err)
		}
	}
}

func createModule5FinalAsset(t *testing.T, repository *GormRepository, ownerID uint64, suffix string, now time.Time) Asset {
	t.Helper()
	publicID, err := NewULID()
	if err != nil {
		t.Fatal(err)
	}
	finalKey := fmt.Sprintf("final/acceptance/%d/%s.webp", ownerID, publicID)
	finalVersion := "module5-fixed-version-" + suffix
	trustedHash := module5HashA
	asset := Asset{
		AssetPublicID: publicID, OwnerID: ownerID, Purpose: PurposePost,
		StorageStage: StorageStageFinal, ReviewStatus: ReviewPending, TechnicalStatus: TechnicalFinalReady,
		TrustedSHA256: &trustedHash, VerifiedMIME: "image/webp", ByteSize: 1024, Width: 64, Height: 64,
		FinalKey: &finalKey, FinalVersionID: &finalVersion, ProjectedDecision: ReviewPending,
		IdempotencyKey: fmt.Sprintf("module5-%s-%s", suffix, publicID), RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateAsset(context.Background(), &asset); err != nil {
		t.Fatal(err)
	}
	return asset
}

func cleanupReviewOwner(t *testing.T, db *gorm.DB, ownerID uint64) {
	t.Helper()
	var assetIDs []uint64
	if err := db.Model(&Asset{}).Where("owner_id = ?", ownerID).Pluck("id", &assetIDs).Error; err != nil {
		t.Errorf("find Module 5 assets: %v", err)
		return
	}
	if len(assetIDs) > 0 && db.Migrator().HasTable(&ReviewProjection{}) {
		var projectionIDs []string
		if err := db.Model(&ReviewProjection{}).Where("asset_id IN ?", assetIDs).Pluck("projection_id", &projectionIDs).Error; err != nil {
			t.Errorf("find Module 5 projections: %v", err)
		}
		if len(projectionIDs) > 0 && db.Migrator().HasTable(&ReviewDecision{}) {
			if err := db.Where("projection_id IN ?", projectionIDs).Delete(&ReviewDecision{}).Error; err != nil {
				t.Errorf("cleanup Module 5 decisions: %v", err)
			}
		}
		if err := db.Where("asset_id IN ?", assetIDs).Delete(&ReviewProjection{}).Error; err != nil {
			t.Errorf("cleanup Module 5 projections: %v", err)
		}
	}
	if err := db.Where("owner_id = ?", ownerID).Delete(&Asset{}).Error; err != nil {
		t.Errorf("cleanup Module 5 assets: %v", err)
	}
}

func TestZZReviewProjectionDatabaseResidueAgainstMySQL84(t *testing.T) {
	db := openModule5MySQL(t)
	owners := []uint64{99_000_501, 99_000_502, 99_000_503, 99_000_504}
	var assetCount int64
	if err := db.Model(&Asset{}).Where("owner_id IN ?", owners).Count(&assetCount).Error; err != nil {
		t.Fatal(err)
	}
	var projectionCount, decisionCount int64
	assetSubquery := db.Model(&Asset{}).Select("id").Where("owner_id IN ?", owners)
	if err := db.Model(&ReviewProjection{}).Where("asset_id IN (?)", assetSubquery).Count(&projectionCount).Error; err != nil {
		t.Fatal(err)
	}
	projectionSubquery := db.Model(&ReviewProjection{}).Select("projection_id").Where("asset_id IN (?)", assetSubquery)
	if err := db.Model(&ReviewDecision{}).Where("projection_id IN (?)", projectionSubquery).Count(&decisionCount).Error; err != nil {
		t.Fatal(err)
	}
	if assetCount != 0 || projectionCount != 0 || decisionCount != 0 {
		t.Fatalf("Module 5 residue assets=%d projections=%d decisions=%d", assetCount, projectionCount, decisionCount)
	}
}
