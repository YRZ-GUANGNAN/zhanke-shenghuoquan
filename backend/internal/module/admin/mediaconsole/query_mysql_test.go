package mediaconsole

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	media "campus-circle/backend/internal/module/media"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const module9OwnerID = uint64(99_000_901)

func TestMediaConsoleQueriesAllExistingFactsAgainstMySQL84(t *testing.T) {
	db := openModule9MySQL(t)
	cleanupModule9(t, db)
	t.Cleanup(func() { cleanupModule9(t, db) })
	fixture := createModule9Fixture(t, db)
	repository := NewQueryRepository(db)
	service, _ := NewQueryService(repository)
	ctx := context.Background()

	summary, err := service.Summary(ctx)
	if err != nil || summary.Assets < 1 || summary.ConsumerDeadletters < 1 || summary.GCDeadletters < 1 {
		t.Fatalf("summary=%#v err=%v", summary, err)
	}
	assets, err := service.Assets(ctx, ListFilter{AssetID: fixture.assetID, Status: "final_ready", ReviewStatus: "approved", Purpose: "post"})
	if err != nil || assets.Total != 1 || len(assets.Items) != 1 {
		t.Fatalf("assets=%#v err=%v", assets, err)
	}
	asset := assets.Items[0]
	if asset.ReferenceCount != 1 || asset.BlockingReferences != 1 || asset.ReviewTaskID == nil || *asset.ReviewTaskID != fixture.taskID || asset.TicketStatus == nil || *asset.TicketStatus != "consumed" {
		t.Fatalf("asset projection mismatch: %#v", asset)
	}
	detail, err := service.Asset(ctx, fixture.assetID)
	if err != nil || detail.Ticket == nil || detail.Completion == nil || detail.Projection == nil || detail.Checkpoint == nil ||
		len(detail.References) != 1 || len(detail.Decisions) != 1 || len(detail.Events) != 1 || len(detail.GCJobs) != 2 {
		t.Fatalf("detail=%#v err=%v", detail, err)
	}
	decisions, err := service.Decisions(ctx, ListFilter{TaskID: &fixture.taskID, Status: "approved"})
	if err != nil || decisions.Total != 1 || decisions.Items[0].AssetPublicID != fixture.assetID {
		t.Fatalf("decisions=%#v err=%v", decisions, err)
	}
	events, err := service.Events(ctx, ListFilter{AssetID: fixture.projectionID, Status: "deadletter", Operation: "ReviewProjectionDecision"})
	if err != nil || events.Total != 1 || events.Items[0].LastErrorCode != "injected_deadletter" {
		t.Fatalf("events=%#v err=%v", events, err)
	}
	jobs, err := service.GCJobs(ctx, ListFilter{AssetID: fixture.assetID})
	if err != nil || jobs.Total != 2 {
		t.Fatalf("gc jobs=%#v err=%v", jobs, err)
	}
	deadletters, err := service.Deadletters(ctx, ListFilter{AssetID: fixture.assetID})
	if err != nil || deadletters.Consumer.Total != 1 || deadletters.GC.Total != 1 {
		t.Fatalf("deadletters=%#v err=%v", deadletters, err)
	}

	encoded, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{fixture.stagingKey, fixture.finalKey, fixture.stagingVersion, fixture.finalVersion, "AAAAAAAAAAAAAAAAAAAAAA==", "lease-secret", `"payload"`} {
		if strings.Contains(string(encoded), secret) {
			t.Fatalf("admin detail leaked protected value %q", secret)
		}
	}

	// Read-only proof: all mutable versions and statuses remain byte-for-byte
	// equivalent after exercising every query endpoint.
	var assetAfter media.Asset
	var projectionAfter media.ReviewProjection
	var inboxAfter media.ConsumerInbox
	var gcAfter media.GCJob
	if err := db.Where("asset_public_id = ?", fixture.assetID).First(&assetAfter).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("projection_id = ?", fixture.projectionID).First(&projectionAfter).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("event_id = ?", fixture.eventID).First(&inboxAfter).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("job_id = ?", fixture.gcJobID).First(&gcAfter).Error; err != nil {
		t.Fatal(err)
	}
	if assetAfter.RowVersion != 1 || assetAfter.TechnicalStatus != media.TechnicalFinalReady || projectionAfter.RowVersion != 1 ||
		projectionAfter.Status != media.ProjectionApproved || inboxAfter.RowVersion != 1 || inboxAfter.Status != media.ConsumerDeadletter ||
		gcAfter.RowVersion != 1 || gcAfter.Status != media.GCDeadletter {
		t.Fatalf("query mutated facts asset=%#v projection=%#v inbox=%#v gc=%#v", assetAfter, projectionAfter, inboxAfter, gcAfter)
	}
}

type module9Fixture struct {
	assetID, projectionID, eventID, gcJobID            string
	stagingKey, finalKey, stagingVersion, finalVersion string
	taskID                                             uint64
}

func createModule9Fixture(t *testing.T, db *gorm.DB) module9Fixture {
	t.Helper()
	now := time.Date(2026, 6, 21, 16, 0, 0, 0, time.UTC)
	assetID, _ := media.NewULID()
	projectionID, _ := media.NewULID()
	ticketID, _ := media.NewULID()
	decisionEventID, _ := media.NewULID()
	operationID, _ := media.NewULID()
	gcJobID, _ := media.NewULID()
	auxJobID, _ := media.NewULID()
	stagingKey, finalKey := "staging/module9/"+assetID+".webp", "final/module9/"+assetID+".webp"
	stagingVersion, finalVersion := "module9-staging-version", "module9-final-version"
	trustedHash := strings.Repeat("a", 64)
	asset := media.Asset{
		AssetPublicID: assetID, OwnerID: module9OwnerID, Purpose: media.PurposePost,
		StorageStage: media.StorageStageFinal, ReviewStatus: media.ReviewPending, TechnicalStatus: media.TechnicalFinalReady,
		TrustedSHA256: &trustedHash, VerifiedMIME: "image/webp", ByteSize: 2048, Width: 128, Height: 96,
		StagingKey: &stagingKey, StagingVersionID: &stagingVersion, FinalKey: &finalKey, FinalVersionID: &finalVersion,
		ProjectedDecision: media.ReviewPending, IdempotencyKey: "module9-" + assetID, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&asset).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&media.AssetReference{
		AssetID: asset.ID, ReferenceType: media.PurposePost, ReferenceID: "module9-post", Slot: 0,
		Status: media.ReferenceActive, ParentVisibility: media.ParentVisible, AggregateVersion: 1,
		OperationID: operationID, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	ticket := media.UploadTicket{
		TicketID: ticketID, AssetID: asset.ID, OwnerID: module9OwnerID, ExpectedSize: 2048,
		ExpectedMD5: "AAAAAAAAAAAAAAAAAAAAAA==", ExpectedContentType: "image/webp", Status: media.TicketConsumed,
		CredentialIssueCount: 1, ExpiresAt: now.Add(time.Hour), RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&ticket).Error; err != nil {
		t.Fatal(err)
	}
	completedAt := now
	if err := db.Create(&media.UploadCompletion{
		TicketID: ticketID, AssetID: asset.ID, Phase: media.CompletionFinalReady,
		StagingVersionID: &stagingVersion, FinalKey: &finalKey, FinalVersionID: &finalVersion,
		ProcessingDeadlineAt: now.Add(time.Hour), ValidationAttempts: 1, PromoteAttempts: 1,
		CompletedAt: &completedAt, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	taskID := uint64(99_009_001)
	projection := media.ReviewProjection{
		ProjectionID: projectionID, AssetID: asset.ID, ReviewTaskID: &taskID, Status: media.ProjectionApproved,
		AppliedDecisionVersion: 1, DecidedAt: &completedAt, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&projection).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&media.ReviewDecision{
		ProjectionID: projectionID, ReviewTaskID: taskID, DecisionVersion: 1, Decision: media.ProjectionApproved,
		PayloadHash: trustedHash, ResultCode: "ok", ReceivedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	outbox := media.OutboxEvent{
		EventID: decisionEventID, AggregateID: projectionID, AggregateVersion: 1,
		Operation: media.ConsumerOperationDecision, Payload: media.JSONDocument(`{"payload":"not-exposed"}`),
		ProcessedAt: &completedAt, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&outbox).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&media.ConsumerInbox{
		EventID: decisionEventID, AggregateID: projectionID, AggregateVersion: 1,
		Operation: media.ConsumerOperationDecision, PayloadHash: trustedHash, Status: media.ConsumerDeadletter,
		AttemptCount: 3, LastErrorCode: "injected_deadletter", ProcessedAt: &completedAt,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&media.ConsumerCheckpoint{
		AggregateID: projectionID, LastEventID: decisionEventID, LastAggregateVersion: 1,
		LastOperation: media.ConsumerOperationDecision, LastPayloadHash: trustedHash,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	lease := "lease-secret"
	leaseExpiry := now.Add(time.Hour)
	if err := db.Create(&media.GCJob{
		JobID: gcJobID, AssetID: asset.ID, ObjectStage: media.GCStageFinal,
		ObjectKey: finalKey, ObjectVersionID: finalVersion, Status: media.GCDeadletter,
		LeaseToken: &lease, LeaseExpiresAt: &leaseExpiry, AttemptCount: 3, LastErrorCode: "retention_guard",
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&media.GCAuxiliaryJob{
		JobID: auxJobID, AssetID: asset.ID, Status: media.GCSucceeded, AttemptCount: 1,
		PurgedAt: &completedAt, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return module9Fixture{
		assetID: assetID, projectionID: projectionID, eventID: decisionEventID, gcJobID: gcJobID,
		stagingKey: stagingKey, finalKey: finalKey, stagingVersion: stagingVersion, finalVersion: finalVersion, taskID: taskID,
	}
}

func openModule9MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	_ = godotenv.Load("../../../../.env")
	dsn := os.Getenv("MEDIA_MODULE9_MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("set MEDIA_MODULE9_MYSQL_DSN or MYSQL_DSN for Module 9 MySQL 8.4 acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.4.") {
		t.Fatalf("Module 9 requires MySQL 8.4, got %s", version)
	}
	for _, table := range []string{"media_assets", "media_upload_tickets", "media_upload_completions", "media_review_projections", "media_review_decisions", "media_outbox", "media_event_inbox", "media_consumer_checkpoints", "media_gc_jobs", "media_gc_auxiliary_jobs"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required frozen Media table missing: %s", table)
		}
	}
	t.Logf("real database version: %s", version)
	return db
}

func cleanupModule9(t *testing.T, db *gorm.DB) {
	t.Helper()
	var assetIDs []uint64
	if err := db.Table("media_assets").Where("owner_id = ?", module9OwnerID).Pluck("id", &assetIDs).Error; err != nil {
		t.Errorf("find Module 9 assets: %v", err)
		return
	}
	if len(assetIDs) == 0 {
		return
	}
	var projectionIDs []string
	_ = db.Table("media_review_projections").Where("asset_id IN ?", assetIDs).Pluck("projection_id", &projectionIDs).Error
	if len(projectionIDs) > 0 {
		var eventIDs []string
		_ = db.Table("media_outbox").Where("aggregate_id IN ?", projectionIDs).Pluck("event_id", &eventIDs).Error
		if len(eventIDs) > 0 {
			_ = db.Where("event_id IN ?", eventIDs).Delete(&media.ConsumerInbox{}).Error
		}
		_ = db.Where("aggregate_id IN ?", projectionIDs).Delete(&media.ConsumerCheckpoint{}).Error
		_ = db.Where("aggregate_id IN ?", projectionIDs).Delete(&media.OutboxEvent{}).Error
		_ = db.Where("projection_id IN ?", projectionIDs).Delete(&media.ReviewDecision{}).Error
	}
	_ = db.Where("asset_id IN ?", assetIDs).Delete(&media.GCJob{}).Error
	_ = db.Where("asset_id IN ?", assetIDs).Delete(&media.GCAuxiliaryJob{}).Error
	_ = db.Where("asset_id IN ?", assetIDs).Delete(&media.AssetReference{}).Error
	_ = db.Where("asset_id IN ?", assetIDs).Delete(&media.ReviewProjection{}).Error
	_ = db.Where("asset_id IN ?", assetIDs).Delete(&media.UploadCompletion{}).Error
	_ = db.Where("owner_id = ?", module9OwnerID).Delete(&media.QuotaReservation{}).Error
	_ = db.Where("owner_id = ?", module9OwnerID).Delete(&media.UploadTicket{}).Error
	if err := db.Where("id IN ?", assetIDs).Delete(&media.Asset{}).Error; err != nil {
		t.Errorf("cleanup Module 9 assets: %v", err)
	}
}

func TestZZMediaConsoleDatabaseResidueAgainstMySQL84(t *testing.T) {
	db := openModule9MySQL(t)
	cleanupModule9(t, db)
	var count int64
	if err := db.Table("media_assets").Where("owner_id = ?", module9OwnerID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("Module 9 database residue: %d assets", count)
	}
}
