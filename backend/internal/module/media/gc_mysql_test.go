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

const module8OwnerBase = uint64(99_000_800)

type gcMemoryStore struct {
	mu      sync.Mutex
	present map[string]bool
	calls   []string
	err     error
}

func newGCMemoryStore() *gcMemoryStore { return &gcMemoryStore{present: map[string]bool{}} }

func (s *gcMemoryStore) add(key, version string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.present[key+"@"+version] = true
}

func (s *gcMemoryStore) DeleteVersion(_ context.Context, key, version string) (GCDeleteResult, error) {
	if key == "" || version == "" {
		return GCDeleteResult{}, ErrGCFixedIdentity
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	identity := key + "@" + version
	s.calls = append(s.calls, identity)
	if s.err != nil {
		return GCDeleteResult{}, s.err
	}
	if !s.present[identity] {
		return GCDeleteResult{AlreadyMissing: true, ProviderRequestID: "memory-missing"}, nil
	}
	delete(s.present, identity)
	return GCDeleteResult{ProviderRequestID: "memory-deleted"}, nil
}

func (s *gcMemoryStore) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func TestAGCMigrationRollbackAndSchemaAgainstMySQL84(t *testing.T) {
	db := openModule8MySQLRaw(t)
	assertModule8MySQL84(t, db)
	applyModule8SQLFile(t, db, "../../../migrations/rollback/014_media_gc.sql")
	if db.Migrator().HasTable(&GCJob{}) || db.Migrator().HasTable(&GCAuxiliaryJob{}) {
		t.Fatal("rollback did not remove Module 8 GC tables")
	}
	applyModule8SQLFile(t, db, "../../../migrations/014_media_gc.sql")
	assertModule8SchemaBoundary(t, db)
}

func TestGCReferenceFactsOverrideCacheAgainstMySQL84(t *testing.T) {
	db := openModule8MySQL(t)
	cleanupGCOwners(t, db)
	t.Cleanup(func() { cleanupGCOwners(t, db) })
	now := time.Date(2026, 6, 21, 8, 0, 0, 0, time.UTC)
	policy := testGCPolicy()
	repository := NewRepository(db)
	store := newGCMemoryStore()
	service, _ := NewMediaGCService(repository, store, policy, func() time.Time { return now })

	blocked := createFinalGCCandidate(t, db, module8OwnerBase+1, ProjectionApproved, 0, now)
	operationID, _ := NewULID()
	reference := AssetReference{
		AssetID: blocked.ID, ReferenceType: PurposePost, ReferenceID: "module8-active-reference", Slot: 0,
		Status: ReferenceActive, ParentVisibility: ParentVisible, AggregateVersion: 1,
		OperationID: operationID, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&reference).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Plan(context.Background(), blocked.AssetPublicID, GCStageFinal); !errors.Is(err, ErrGCReferenceExists) {
		t.Fatalf("active reference fact did not block GC: %v", err)
	}

	allowed := createFinalGCCandidate(t, db, module8OwnerBase+2, ProjectionApproved, 999, now)
	store.add(*allowed.FinalKey, *allowed.FinalVersionID)
	job, created, err := service.Plan(context.Background(), allowed.AssetPublicID, GCStageFinal)
	if err != nil || !created {
		t.Fatalf("cache-independent plan created=%v err=%v", created, err)
	}
	result, err := service.RunBatch(context.Background(), 1)
	if err != nil || result.Succeeded != 1 {
		t.Fatalf("GC result=%#v err=%v", result, err)
	}
	var loaded Asset
	if err := db.First(&loaded, allowed.ID).Error; err != nil {
		t.Fatal(err)
	}
	if loaded.TechnicalStatus != TechnicalDeleted || loaded.DeletedAt == nil {
		t.Fatalf("asset not deleted: %#v", loaded)
	}
	var loadedJob GCJob
	if err := db.Where("job_id = ?", job.JobID).First(&loadedJob).Error; err != nil {
		t.Fatal(err)
	}
	if loadedJob.Status != GCSucceeded || loadedJob.ProviderRequestID != "memory-deleted" {
		t.Fatalf("job not converged: %#v", loadedJob)
	}
}

func TestGCRevalidatesReferencesImmediatelyBeforeDeleteAgainstMySQL84(t *testing.T) {
	db := openModule8MySQL(t)
	cleanupGCOwners(t, db)
	t.Cleanup(func() { cleanupGCOwners(t, db) })
	now := time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	store := newGCMemoryStore()
	service, _ := NewMediaGCService(repository, store, testGCPolicy(), func() time.Time { return now })
	asset := createFinalGCCandidate(t, db, module8OwnerBase+3, ProjectionRejected, 0, now)
	store.add(*asset.FinalKey, *asset.FinalVersionID)
	if _, _, err := service.Plan(context.Background(), asset.AssetPublicID, GCStageFinal); err != nil {
		t.Fatal(err)
	}
	operationID, _ := NewULID()
	if err := db.Create(&AssetReference{
		AssetID: asset.ID, ReferenceType: PurposeProduct, ReferenceID: "module8-late-reference", Slot: 0,
		Status: ReferencePending, ParentVisibility: ParentHidden, AggregateVersion: 1,
		OperationID: operationID, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	result, err := service.RunBatch(context.Background(), 1)
	if err != nil || result.Retried != 1 || store.callCount() != 0 {
		t.Fatalf("fail-closed result=%#v calls=%d err=%v", result, store.callCount(), err)
	}
}

func TestGCFailsClosedOnReviewAndRetentionAgainstMySQL84(t *testing.T) {
	db := openModule8MySQL(t)
	cleanupGCOwners(t, db)
	t.Cleanup(func() { cleanupGCOwners(t, db) })
	now := time.Date(2026, 6, 21, 9, 30, 0, 0, time.UTC)
	repository := NewRepository(db)
	service, _ := NewMediaGCService(repository, newGCMemoryStore(), testGCPolicy(), func() time.Time { return now })

	nonterminal := createFinalGCCandidate(t, db, module8OwnerBase+7, ProjectionApproved, 0, now)
	if err := db.Model(&ReviewProjection{}).Where("asset_id = ?", nonterminal.ID).
		Updates(map[string]any{"status": ProjectionReviewing, "decided_at": nil, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Plan(context.Background(), nonterminal.AssetPublicID, GCStageFinal); !errors.Is(err, ErrGCNotEligible) {
		t.Fatalf("nonterminal review did not fail closed: %v", err)
	}

	retained := createFinalGCCandidate(t, db, module8OwnerBase+8, ProjectionApproved, 0, now)
	if err := db.Model(&ReviewProjection{}).Where("asset_id = ?", retained.ID).
		Updates(map[string]any{"decided_at": now, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Plan(context.Background(), retained.AssetPublicID, GCStageFinal); !errors.Is(err, ErrGCNotEligible) {
		t.Fatalf("unexpired retention did not fail closed: %v", err)
	}
}

func TestGCLeaseCASConcurrencyAgainstMySQL84(t *testing.T) {
	db := openModule8MySQL(t)
	cleanupGCOwners(t, db)
	t.Cleanup(func() { cleanupGCOwners(t, db) })
	now := time.Date(2026, 6, 21, 10, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	asset := createFinalGCCandidate(t, db, module8OwnerBase+4, ProjectionApproved, 0, now)
	if _, _, err := repository.PlanGCJob(context.Background(), asset.AssetPublicID, GCStageFinal, now, testGCPolicy()); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var wait sync.WaitGroup
	var claimed, failures atomic.Int32
	var winning GCDelivery
	var winningMu sync.Mutex
	for i := 0; i < 2; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			delivery, found, err := repository.ClaimNextGCJob(context.Background(), now, time.Minute)
			if err != nil {
				failures.Add(1)
				return
			}
			if found {
				claimed.Add(1)
				winningMu.Lock()
				winning = delivery
				winningMu.Unlock()
			}
		}()
	}
	close(start)
	wait.Wait()
	if claimed.Load() != 1 || failures.Load() != 0 {
		t.Fatalf("claimed=%d failures=%d", claimed.Load(), failures.Load())
	}
	if err := db.Model(&GCJob{}).Where("job_id = ?", winning.Job.JobID).Update("row_version", gorm.Expr("row_version + 1")).Error; err != nil {
		t.Fatal(err)
	}
	storeCalls := 0
	_, err := repository.ExecuteGCDeletion(context.Background(), winning, now, testGCPolicy(), func(context.Context, string, string) (GCDeleteResult, error) {
		storeCalls++
		return GCDeleteResult{}, nil
	})
	if !errors.Is(err, ErrGCLease) || storeCalls != 0 {
		t.Fatalf("stale CAS err=%v storeCalls=%d", err, storeCalls)
	}
}

func TestGCCrashRecoveryTreatsMissingFixedVersionAsSuccessAgainstMySQL84(t *testing.T) {
	db := openModule8MySQL(t)
	cleanupGCOwners(t, db)
	t.Cleanup(func() { cleanupGCOwners(t, db) })
	now := time.Date(2026, 6, 21, 11, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	asset := createFinalGCCandidate(t, db, module8OwnerBase+5, ProjectionRejected, 0, now)
	if _, _, err := repository.PlanGCJob(context.Background(), asset.AssetPublicID, GCStageFinal, now, testGCPolicy()); err != nil {
		t.Fatal(err)
	}
	if _, found, err := repository.ClaimNextGCJob(context.Background(), now, time.Minute); err != nil || !found {
		t.Fatalf("initial claim found=%v err=%v", found, err)
	}
	// Simulate: COS accepted the exact-version delete, then the process crashed
	// before the database acknowledgement. The durable job remains processing.
	now = now.Add(2 * time.Minute)
	store := newGCMemoryStore() // the exact object version is already absent
	service, _ := NewMediaGCService(repository, store, testGCPolicy(), func() time.Time { return now })
	result, err := service.RunBatch(context.Background(), 1)
	if err != nil || result.Succeeded != 1 || store.callCount() != 1 {
		t.Fatalf("recovery result=%#v calls=%d err=%v", result, store.callCount(), err)
	}
}

func TestGCRetryExhaustionDeadlettersAgainstMySQL84(t *testing.T) {
	db := openModule8MySQL(t)
	cleanupGCOwners(t, db)
	t.Cleanup(func() { cleanupGCOwners(t, db) })
	now := time.Date(2026, 6, 21, 11, 30, 0, 0, time.UTC)
	repository := NewRepository(db)
	asset := createFinalGCCandidate(t, db, module8OwnerBase+9, ProjectionRejected, 0, now)
	store := newGCMemoryStore()
	store.add(*asset.FinalKey, *asset.FinalVersionID)
	store.err = errors.New("injected COS timeout")
	policy := testGCPolicy()
	policy.MaxAttempts = 2
	service, _ := NewMediaGCService(repository, store, policy, func() time.Time { return now })
	job, _, err := service.Plan(context.Background(), asset.AssetPublicID, GCStageFinal)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.RunBatch(context.Background(), 1)
	if err != nil || first.Retried != 1 {
		t.Fatalf("first retry=%#v err=%v", first, err)
	}
	now = now.Add(policy.RetryDelay)
	second, err := service.RunBatch(context.Background(), 1)
	if err != nil || second.Deadletters != 1 {
		t.Fatalf("deadletter=%#v err=%v", second, err)
	}
	var loaded GCJob
	if err := db.Where("job_id = ?", job.JobID).First(&loaded).Error; err != nil {
		t.Fatal(err)
	}
	if loaded.Status != GCDeadletter || loaded.AttemptCount != 2 || loaded.LastErrorCode != "storage_timeout" {
		t.Fatalf("retry exhaustion job=%#v", loaded)
	}
}

func TestGCStagingCleanupAndAuxiliaryRetentionAgainstMySQL84(t *testing.T) {
	db := openModule8MySQL(t)
	cleanupGCOwners(t, db)
	t.Cleanup(func() { cleanupGCOwners(t, db) })
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	asset := createPromotedStagingCandidate(t, db, module8OwnerBase+6, now)
	store := newGCMemoryStore()
	store.add(*asset.StagingKey, *asset.StagingVersionID)
	service, _ := NewMediaGCService(repository, store, testGCPolicy(), func() time.Time { return now })
	if _, _, err := service.Plan(context.Background(), asset.AssetPublicID, GCStageStaging); err != nil {
		t.Fatal(err)
	}
	result, err := service.RunBatch(context.Background(), 1)
	if err != nil || result.Succeeded != 1 {
		t.Fatalf("staging cleanup=%#v err=%v", result, err)
	}
	var loaded Asset
	if err := db.First(&loaded, asset.ID).Error; err != nil {
		t.Fatal(err)
	}
	if loaded.TechnicalStatus != TechnicalFinalReady || loaded.StorageStage != StorageStageFinal {
		t.Fatalf("staging cleanup mutated final asset: %#v", loaded)
	}

	if err := db.Model(&Asset{}).Where("id = ?", asset.ID).Updates(map[string]any{
		"technical_status": TechnicalDeleted, "deleted_at": now.Add(-48 * time.Hour), "updated_at": now.Add(-48 * time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&UploadTicket{}).Where("asset_id = ?", asset.ID).Update("updated_at", now.Add(-48*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&QuotaReservation{}).Where("owner_id = ?", asset.OwnerID).Update("updated_at", now.Add(-48*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&UploadCompletion{}).Where("asset_id = ?", asset.ID).Update("updated_at", now.Add(-48*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	purged, err := service.PurgeTerminalAuxiliary(context.Background(), 10)
	if err != nil || purged.Reservations != 1 || purged.Completions != 1 || purged.Tickets != 1 {
		t.Fatalf("purged=%#v err=%v", purged, err)
	}
	var auxiliaryJob GCAuxiliaryJob
	if err := db.Where("asset_id = ?", asset.ID).First(&auxiliaryJob).Error; err != nil || auxiliaryJob.Status != GCSucceeded || auxiliaryJob.PurgedAt == nil || auxiliaryJob.RowVersion != 3 {
		t.Fatalf("auxiliary lease/CAS job=%#v err=%v", auxiliaryJob, err)
	}
}

func createFinalGCCandidate(t *testing.T, db *gorm.DB, ownerID uint64, decision ProjectionStatus, cachedRefs uint64, now time.Time) Asset {
	t.Helper()
	publicID, _ := NewULID()
	projectionID, _ := NewULID()
	stagingKey := fmt.Sprintf("staging/module8/%d/%s.webp", ownerID, publicID)
	stagingVersion := "staging-version-" + publicID
	finalKey := fmt.Sprintf("final/module8/%d/%s.webp", ownerID, publicID)
	finalVersion := "final-version-" + publicID
	requested := now.Add(-72 * time.Hour)
	asset := Asset{
		AssetPublicID: publicID, OwnerID: ownerID, Purpose: PurposePost,
		StorageStage: StorageStageFinal, ReviewStatus: ReviewPending, TechnicalStatus: TechnicalDeletePending,
		VerifiedMIME: "image/webp", ByteSize: 128, Width: 16, Height: 16,
		StagingKey: &stagingKey, StagingVersionID: &stagingVersion, FinalKey: &finalKey, FinalVersionID: &finalVersion,
		ProjectedDecision: ReviewPending, ActiveRefCount: cachedRefs, IdempotencyKey: "module8-" + publicID,
		DeleteRequestedAt: &requested, RowVersion: 1, CreatedAt: requested, UpdatedAt: requested,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	decided := requested
	projection := ReviewProjection{
		ProjectionID: projectionID, AssetID: asset.ID, Status: decision, AppliedDecisionVersion: 1,
		DecidedAt: &decided, RowVersion: 1, CreatedAt: requested, UpdatedAt: requested,
	}
	if err := db.Create(&projection).Error; err != nil {
		t.Fatal(err)
	}
	return asset
}

func createPromotedStagingCandidate(t *testing.T, db *gorm.DB, ownerID uint64, now time.Time) Asset {
	t.Helper()
	publicID, _ := NewULID()
	ticketID, _ := NewULID()
	reservationID, _ := NewULID()
	stagingKey := fmt.Sprintf("staging/module8/%d/%s.webp", ownerID, publicID)
	stagingVersion := "staging-version-" + publicID
	finalKey := fmt.Sprintf("final/module8/%d/%s.webp", ownerID, publicID)
	finalVersion := "final-version-" + publicID
	completed := now.Add(-48 * time.Hour)
	asset := Asset{
		AssetPublicID: publicID, OwnerID: ownerID, Purpose: PurposeProduct,
		StorageStage: StorageStageFinal, ReviewStatus: ReviewPending, TechnicalStatus: TechnicalFinalReady,
		VerifiedMIME: "image/webp", ByteSize: 256, Width: 32, Height: 32,
		StagingKey: &stagingKey, StagingVersionID: &stagingVersion, FinalKey: &finalKey, FinalVersionID: &finalVersion,
		ProjectedDecision: ReviewPending, IdempotencyKey: "module8-staging-" + publicID,
		RowVersion: 1, CreatedAt: completed, UpdatedAt: completed,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	ticket := UploadTicket{
		TicketID: ticketID, AssetID: asset.ID, OwnerID: ownerID, ExpectedSize: 256,
		ExpectedMD5: "AAAAAAAAAAAAAAAAAAAAAA==", ExpectedContentType: "image/webp",
		Status: TicketConsumed, CredentialIssueCount: 1, ExpiresAt: completed.Add(time.Hour),
		RowVersion: 1, CreatedAt: completed, UpdatedAt: completed,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&QuotaReservation{
		ReservationID: reservationID, OwnerID: ownerID, BytesReserved: 256, CountReserved: 1,
		Status: ReservationCommitted, TicketID: ticketID, RowVersion: 1, CreatedAt: completed, UpdatedAt: completed,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&UploadCompletion{
		TicketID: ticketID, AssetID: asset.ID, Phase: CompletionFinalReady,
		StagingVersionID: &stagingVersion, FinalKey: &finalKey, FinalVersionID: &finalVersion,
		ProcessingDeadlineAt: completed.Add(time.Hour), CompletedAt: &completed,
		RowVersion: 1, CreatedAt: completed, UpdatedAt: completed,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return asset
}

func openModule8MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	db := openModule8MySQLRaw(t)
	if !db.Migrator().HasTable(&GCJob{}) {
		applyModule8SQLFile(t, db, "../../../migrations/014_media_gc.sql")
	}
	return db
}

func openModule8MySQLRaw(t *testing.T) *gorm.DB {
	t.Helper()
	_ = godotenv.Load("../../../.env")
	dsn := os.Getenv("MEDIA_MODULE8_MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("set MEDIA_MODULE8_MYSQL_DSN or MYSQL_DSN for Module 8 MySQL 8.4 acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertModule8MySQL84(t *testing.T, db *gorm.DB) {
	t.Helper()
	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.4.") {
		t.Fatalf("Module 8 requires MySQL 8.4, got %s", version)
	}
	t.Logf("real database version: %s", version)
}

func assertModule8SchemaBoundary(t *testing.T, db *gorm.DB) {
	t.Helper()
	var foreignTables []string
	if err := db.Raw(`SELECT DISTINCT referenced_table_name FROM information_schema.key_column_usage
		WHERE table_schema = DATABASE() AND table_name IN ('media_gc_jobs','media_gc_auxiliary_jobs') AND referenced_table_name IS NOT NULL`).Scan(&foreignTables).Error; err != nil {
		t.Fatal(err)
	}
	if len(foreignTables) != 1 || foreignTables[0] != "media_assets" {
		t.Fatalf("unexpected GC foreign keys: %#v", foreignTables)
	}
	var indexes []string
	if err := db.Raw(`SELECT DISTINCT index_name FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = 'media_gc_jobs'`).Scan(&indexes).Error; err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"uq_media_gc_object_identity", "idx_media_gc_claim", "idx_media_gc_asset_status"} {
		found := false
		for _, index := range indexes {
			found = found || index == required
		}
		if !found {
			t.Fatalf("missing GC index %s", required)
		}
	}
	var auxiliaryIndexes []string
	if err := db.Raw(`SELECT DISTINCT index_name FROM information_schema.statistics
		WHERE table_schema = DATABASE() AND table_name = 'media_gc_auxiliary_jobs'`).Scan(&auxiliaryIndexes).Error; err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"uq_media_gc_aux_asset", "idx_media_gc_aux_claim"} {
		found := false
		for _, index := range auxiliaryIndexes {
			found = found || index == required
		}
		if !found {
			t.Fatalf("missing GC auxiliary index %s", required)
		}
	}
}

func applyModule8SQLFile(t *testing.T, db *gorm.DB, path string) {
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

func cleanupGCOwners(t *testing.T, db *gorm.DB) {
	t.Helper()
	var assetIDs []uint64
	if err := db.Model(&Asset{}).Where("owner_id >= ? AND owner_id < ?", module8OwnerBase, module8OwnerBase+100).Pluck("id", &assetIDs).Error; err != nil {
		t.Errorf("find Module 8 assets: %v", err)
		return
	}
	if len(assetIDs) == 0 {
		return
	}
	var projectionIDs []string
	_ = db.Model(&ReviewProjection{}).Where("asset_id IN ?", assetIDs).Pluck("projection_id", &projectionIDs).Error
	if len(projectionIDs) > 0 {
		_ = db.Where("projection_id IN ?", projectionIDs).Delete(&ReviewDecision{}).Error
	}
	_ = db.Where("asset_id IN ?", assetIDs).Delete(&GCJob{}).Error
	_ = db.Where("asset_id IN ?", assetIDs).Delete(&GCAuxiliaryJob{}).Error
	_ = db.Where("asset_id IN ?", assetIDs).Delete(&AssetReference{}).Error
	_ = db.Where("asset_id IN ?", assetIDs).Delete(&ReviewProjection{}).Error
	_ = db.Where("asset_id IN ?", assetIDs).Delete(&UploadCompletion{}).Error
	_ = db.Where("owner_id >= ? AND owner_id < ?", module8OwnerBase, module8OwnerBase+100).Delete(&QuotaReservation{}).Error
	_ = db.Where("owner_id >= ? AND owner_id < ?", module8OwnerBase, module8OwnerBase+100).Delete(&UploadTicket{}).Error
	if err := db.Where("id IN ?", assetIDs).Delete(&Asset{}).Error; err != nil {
		t.Errorf("cleanup Module 8 assets: %v", err)
	}
}

func TestZZMediaGCDatabaseResidueAgainstMySQL84(t *testing.T) {
	db := openModule8MySQL(t)
	cleanupGCOwners(t, db)
	var count int64
	if err := db.Model(&Asset{}).Where("owner_id >= ? AND owner_id < ?", module8OwnerBase, module8OwnerBase+100).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("Module 8 database residue: %d assets", count)
	}
}
