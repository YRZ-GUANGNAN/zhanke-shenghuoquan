package media

import (
	"context"
	"encoding/json"
	"errors"
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

func Test00MediaConsumerMigrationAndRollbackAgainstMySQL84(t *testing.T) {
	db := openModule7MySQLRaw(t)
	assertMySQL84(t, db)
	for _, model := range []any{&ConsumerInbox{}, &ConsumerCheckpoint{}} {
		if db.Migrator().HasTable(model) {
			var count int64
			if err := db.Model(model).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("refusing rollback against non-empty %T: %d", model, count)
			}
		}
	}
	applyConsumerSQLFile(t, db, "../../../migrations/rollback/013_media_consumer.sql")
	if db.Migrator().HasTable(&ConsumerInbox{}) || db.Migrator().HasTable(&ConsumerCheckpoint{}) {
		t.Fatal("rollback left Module 7 tables behind")
	}
	applyConsumerSQLFile(t, db, "../../../migrations/013_media_consumer.sql")
	if !db.Migrator().HasTable(&ConsumerInbox{}) || !db.Migrator().HasTable(&ConsumerCheckpoint{}) {
		t.Fatal("forward migration did not create Module 7 tables")
	}
	assertConsumerSchemaBoundary(t, db)
}

func TestMediaConsumerLifecycleAndRepeatedScanAgainstMySQL84(t *testing.T) {
	db := openModule7MySQL(t)
	ownerID := uint64(99_000_701)
	cleanupConsumerOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupConsumerOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 21, 21, 0, 0, 0, time.UTC)
	repository, projectionService, projection := prepareConsumerProjection(t, db, ownerID, "lifecycle", now)
	appendConsumerLifecycle(t, repository, projection.ProjectionID, 71_001, now)
	consumer, _ := NewMediaConsumer(repository, projectionService, MediaConsumerConfig{Now: func() time.Time { return now }})

	result, err := consumer.ConsumeBatch(context.Background(), 10)
	if err != nil || result.Succeeded != 3 || result.Claimed != 3 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	loaded, err := repository.FindReviewProjection(context.Background(), projection.ProjectionID)
	if err != nil || loaded.Status != ProjectionApproved || loaded.RowVersion != 4 {
		t.Fatalf("projection=%#v err=%v", loaded, err)
	}
	replayed, err := consumer.ConsumeBatch(context.Background(), 10)
	if err != nil || replayed.Claimed != 0 {
		t.Fatalf("processed outbox was scanned again: %#v err=%v", replayed, err)
	}
	assertConsumerAggregateState(t, db, projection.ProjectionID, 3, ConsumerSucceeded, 3)
}

func TestMediaConsumerCrashRecoveryAndInboxDedupAgainstMySQL84(t *testing.T) {
	db := openModule7MySQL(t)
	ownerID := uint64(99_000_702)
	cleanupConsumerOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupConsumerOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 21, 21, 30, 0, 0, time.UTC)
	repository, projectionService, projection := prepareConsumerProjection(t, db, ownerID, "crash", now)
	event := appendConsumerEvent(t, repository, projection.ProjectionID, 1, ConsumerOperationSubmit,
		ConsumerEventPayload{ProjectionID: projection.ProjectionID, ReviewTaskID: 72_001, ExpectedRowVersion: 1}, now)

	delivery, found, err := repository.ClaimNextConsumerEvent(context.Background(), now, time.Minute)
	if err != nil || !found || delivery.Event.EventID != event.EventID {
		t.Fatalf("claim=%#v found=%v err=%v", delivery, found, err)
	}
	if _, err := projectionService.Submit(context.Background(), projection.ProjectionID, 72_001, 1); err != nil {
		t.Fatal(err)
	}
	// Simulate a process crash after the projection transaction committed but
	// before Inbox/Outbox acknowledgement.
	now = now.Add(2 * time.Minute)
	consumer, _ := NewMediaConsumer(repository, projectionService, MediaConsumerConfig{LeaseTTL: time.Minute, Now: func() time.Time { return now }})
	result, err := consumer.ConsumeBatch(context.Background(), 1)
	if err != nil || result.Succeeded != 1 {
		t.Fatalf("recovery=%#v err=%v", result, err)
	}
	loaded, _ := repository.FindReviewProjection(context.Background(), projection.ProjectionID)
	if loaded.Status != ProjectionSubmitted || loaded.RowVersion != 2 {
		t.Fatalf("duplicate delivery advanced projection twice: %#v", loaded)
	}
	var inbox ConsumerInbox
	if err := db.Where("event_id = ?", event.EventID).First(&inbox).Error; err != nil {
		t.Fatal(err)
	}
	if inbox.Status != ConsumerSucceeded || inbox.AttemptCount != 2 {
		t.Fatalf("inbox recovery=%#v", inbox)
	}
	var count int64
	if err := db.Model(&ConsumerInbox{}).Where("event_id = ?", event.EventID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("inbox dedup count=%d err=%v", count, err)
	}
}

func TestMediaConsumerOutOfOrderEventuallyConvergesAgainstMySQL84(t *testing.T) {
	db := openModule7MySQL(t)
	ownerID := uint64(99_000_703)
	cleanupConsumerOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupConsumerOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 21, 22, 0, 0, 0, time.UTC)
	repository, projectionService, projection := prepareConsumerProjection(t, db, ownerID, "order", now)
	appendConsumerEvent(t, repository, projection.ProjectionID, 2, ConsumerOperationReviewing,
		ConsumerEventPayload{ProjectionID: projection.ProjectionID, ExpectedRowVersion: 2}, now)
	consumer, _ := NewMediaConsumer(repository, projectionService, MediaConsumerConfig{RetryDelay: time.Minute, MaxAttempts: 4, Now: func() time.Time { return now }})
	first, err := consumer.ConsumeBatch(context.Background(), 1)
	if err != nil || first.Retried != 1 {
		t.Fatalf("out-of-order=%#v err=%v", first, err)
	}
	loaded, _ := repository.FindReviewProjection(context.Background(), projection.ProjectionID)
	if loaded.Status != ProjectionPending {
		t.Fatalf("out-of-order event changed projection: %#v", loaded)
	}
	appendConsumerEvent(t, repository, projection.ProjectionID, 1, ConsumerOperationSubmit,
		ConsumerEventPayload{ProjectionID: projection.ProjectionID, ReviewTaskID: 73_001, ExpectedRowVersion: 1}, now.Add(time.Second))
	second, err := consumer.ConsumeBatch(context.Background(), 1)
	if err != nil || second.Succeeded != 1 {
		t.Fatalf("missing predecessor=%#v err=%v", second, err)
	}
	now = now.Add(2 * time.Minute)
	third, err := consumer.ConsumeBatch(context.Background(), 1)
	if err != nil || third.Succeeded != 1 {
		t.Fatalf("ordered retry=%#v err=%v", third, err)
	}
	loaded, _ = repository.FindReviewProjection(context.Background(), projection.ProjectionID)
	if loaded.Status != ProjectionReviewing || loaded.RowVersion != 3 {
		t.Fatalf("ordered convergence failed: %#v", loaded)
	}
}

func TestMediaConsumerStaleCASAndDeadletterAgainstMySQL84(t *testing.T) {
	db := openModule7MySQL(t)
	ownerID := uint64(99_000_704)
	cleanupConsumerOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupConsumerOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 21, 22, 30, 0, 0, time.UTC)
	repository, projectionService, projection := prepareConsumerProjection(t, db, ownerID, "stale", now)
	event := appendConsumerEvent(t, repository, projection.ProjectionID, 1, ConsumerOperationSubmit,
		ConsumerEventPayload{ProjectionID: projection.ProjectionID, ReviewTaskID: 74_001, ExpectedRowVersion: 99}, now)
	consumer, _ := NewMediaConsumer(repository, projectionService, MediaConsumerConfig{MaxAttempts: 1, Now: func() time.Time { return now }})
	result, err := consumer.ConsumeBatch(context.Background(), 1)
	if err != nil || result.Deadletters != 1 {
		t.Fatalf("stale CAS result=%#v err=%v", result, err)
	}
	loaded, _ := repository.FindReviewProjection(context.Background(), projection.ProjectionID)
	if loaded.Status != ProjectionPending || loaded.RowVersion != 1 {
		t.Fatalf("stale CAS changed projection: %#v", loaded)
	}
	var inbox ConsumerInbox
	if err := db.Where("event_id = ?", event.EventID).First(&inbox).Error; err != nil {
		t.Fatal(err)
	}
	if inbox.Status != ConsumerDeadletter || inbox.LastErrorCode != "stale_row_version" {
		t.Fatalf("deadletter=%#v", inbox)
	}
}

func TestMediaConsumerInboxCASRejectsStaleDeliveryAgainstMySQL84(t *testing.T) {
	db := openModule7MySQL(t)
	ownerID := uint64(99_000_705)
	cleanupConsumerOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupConsumerOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 21, 23, 0, 0, 0, time.UTC)
	repository, _, projection := prepareConsumerProjection(t, db, ownerID, "inbox-cas", now)
	appendConsumerEvent(t, repository, projection.ProjectionID, 1, ConsumerOperationSubmit,
		ConsumerEventPayload{ProjectionID: projection.ProjectionID, ReviewTaskID: 75_001, ExpectedRowVersion: 1}, now)
	delivery, found, err := repository.ClaimNextConsumerEvent(context.Background(), now, time.Minute)
	if err != nil || !found {
		t.Fatalf("claim found=%v err=%v", found, err)
	}
	if err := db.Model(&ConsumerInbox{}).Where("event_id = ?", delivery.Event.EventID).Update("row_version", gorm.Expr("row_version + 1")).Error; err != nil {
		t.Fatal(err)
	}
	if err := repository.CompleteConsumerEvent(context.Background(), delivery, now); !errors.Is(err, ErrConsumerLease) {
		t.Fatalf("stale inbox CAS returned %v", err)
	}
}

func TestMediaConsumerAggregateLeaseSerializesWorkersAgainstMySQL84(t *testing.T) {
	db := openModule7MySQL(t)
	ownerID := uint64(99_000_707)
	cleanupConsumerOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupConsumerOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 21, 23, 15, 0, 0, time.UTC)
	repository, _, projection := prepareConsumerProjection(t, db, ownerID, "aggregate-lease", now)
	appendConsumerEvent(t, repository, projection.ProjectionID, 1, ConsumerOperationSubmit,
		ConsumerEventPayload{ProjectionID: projection.ProjectionID, ReviewTaskID: 77_001, ExpectedRowVersion: 1}, now)
	appendConsumerEvent(t, repository, projection.ProjectionID, 2, ConsumerOperationReviewing,
		ConsumerEventPayload{ProjectionID: projection.ProjectionID, ExpectedRowVersion: 2}, now.Add(time.Millisecond))

	start := make(chan struct{})
	var wait sync.WaitGroup
	var claimed, unexpected atomic.Int32
	for index := 0; index < 2; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, found, err := repository.ClaimNextConsumerEvent(context.Background(), now, time.Minute)
			if err != nil {
				unexpected.Add(1)
				return
			}
			if found {
				claimed.Add(1)
			}
		}()
	}
	close(start)
	wait.Wait()
	if claimed.Load() != 1 || unexpected.Load() != 0 {
		t.Fatalf("aggregate lease claimed=%d unexpected=%d", claimed.Load(), unexpected.Load())
	}
	var processing int64
	if err := db.Model(&ConsumerInbox{}).Where("aggregate_id = ? AND status = ?", projection.ProjectionID, ConsumerProcessing).Count(&processing).Error; err != nil {
		t.Fatal(err)
	}
	if processing != 1 {
		t.Fatalf("parallel aggregate processing rows=%d", processing)
	}
}

func TestMediaConsumerMalformedEventDeadlettersAgainstMySQL84(t *testing.T) {
	db := openModule7MySQL(t)
	ownerID := uint64(99_000_706)
	cleanupConsumerOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupConsumerOwner(t, db, ownerID) })
	now := time.Date(2026, 6, 21, 23, 30, 0, 0, time.UTC)
	repository, projectionService, projection := prepareConsumerProjection(t, db, ownerID, "malformed", now)
	eventID, _ := NewULID()
	event := OutboxEvent{
		EventID: eventID, AggregateID: projection.ProjectionID, AggregateVersion: 1,
		Operation: ConsumerOperationSubmit, Payload: JSONDocument(`{"unexpected":true}`),
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.AppendOutbox(context.Background(), &event); err != nil {
		t.Fatal(err)
	}
	consumer, _ := NewMediaConsumer(repository, projectionService, MediaConsumerConfig{Now: func() time.Time { return now }})
	result, err := consumer.ConsumeBatch(context.Background(), 1)
	if err != nil || result.Deadletters != 1 {
		t.Fatalf("malformed=%#v err=%v", result, err)
	}
	var inbox ConsumerInbox
	if err := db.Where("event_id = ?", eventID).First(&inbox).Error; err != nil {
		t.Fatal(err)
	}
	if inbox.Status != ConsumerDeadletter || inbox.LastErrorCode != "invalid_payload" {
		t.Fatalf("malformed inbox=%#v", inbox)
	}
}

func openModule7MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	db := openModule7MySQLRaw(t)
	if !db.Migrator().HasTable(&ConsumerInbox{}) || !db.Migrator().HasTable(&ConsumerCheckpoint{}) {
		applyConsumerSQLFile(t, db, "../../../migrations/013_media_consumer.sql")
	}
	return db
}

func openModule7MySQLRaw(t *testing.T) *gorm.DB {
	t.Helper()
	_ = godotenv.Load("../../../.env")
	dsn := os.Getenv("MEDIA_MODULE7_MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("set MEDIA_MODULE7_MYSQL_DSN or MYSQL_DSN to run Module 7 MySQL 8.4 acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func prepareConsumerProjection(t *testing.T, db *gorm.DB, ownerID uint64, suffix string, now time.Time) (*GormRepository, *ReviewProjectionService, ReviewProjection) {
	t.Helper()
	repository := NewRepository(db)
	asset := createModule5FinalAsset(t, repository, ownerID, "module7-"+suffix, now)
	projectionService, _ := NewReviewProjectionService(repository, func() time.Time { return now })
	projection, created, err := projectionService.EnsureProjection(context.Background(), asset.AssetPublicID)
	if err != nil || !created {
		t.Fatalf("prepare projection created=%v err=%v", created, err)
	}
	return repository, projectionService, projection
}

func appendConsumerLifecycle(t *testing.T, repository *GormRepository, projectionID string, taskID uint64, now time.Time) {
	t.Helper()
	appendConsumerEvent(t, repository, projectionID, 1, ConsumerOperationSubmit,
		ConsumerEventPayload{ProjectionID: projectionID, ReviewTaskID: taskID, ExpectedRowVersion: 1}, now)
	appendConsumerEvent(t, repository, projectionID, 2, ConsumerOperationReviewing,
		ConsumerEventPayload{ProjectionID: projectionID, ExpectedRowVersion: 2}, now.Add(time.Millisecond))
	appendConsumerEvent(t, repository, projectionID, 3, ConsumerOperationDecision,
		ConsumerEventPayload{
			ProjectionID: projectionID, ReviewTaskID: taskID, ExpectedRowVersion: 3,
			ExpectedStatus: ProjectionReviewing, DecisionVersion: 1,
			Decision: ProjectionApproved, PayloadHash: module5HashA,
		}, now.Add(2*time.Millisecond))
}

func appendConsumerEvent(t *testing.T, repository *GormRepository, aggregateID string, version uint64, operation string, payload ConsumerEventPayload, createdAt time.Time) OutboxEvent {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	eventID, _ := NewULID()
	event := OutboxEvent{
		EventID: eventID, AggregateID: aggregateID, AggregateVersion: version,
		Operation: operation, Payload: JSONDocument(raw), RowVersion: 1,
		CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	if err := repository.AppendOutbox(context.Background(), &event); err != nil {
		t.Fatal(err)
	}
	return event
}

func assertConsumerAggregateState(t *testing.T, db *gorm.DB, aggregateID string, expectedVersion uint64, expectedStatus ConsumerStatus, expectedInboxCount int64) {
	t.Helper()
	var checkpoint ConsumerCheckpoint
	if err := db.Where("aggregate_id = ?", aggregateID).First(&checkpoint).Error; err != nil {
		t.Fatal(err)
	}
	if checkpoint.LastAggregateVersion != expectedVersion {
		t.Fatalf("checkpoint=%#v", checkpoint)
	}
	var inboxCount int64
	if err := db.Model(&ConsumerInbox{}).Where("aggregate_id = ? AND status = ?", aggregateID, expectedStatus).Count(&inboxCount).Error; err != nil {
		t.Fatal(err)
	}
	if inboxCount != expectedInboxCount {
		t.Fatalf("inbox status count=%d want=%d", inboxCount, expectedInboxCount)
	}
	var pendingOutbox int64
	if err := db.Model(&OutboxEvent{}).Where("aggregate_id = ? AND processed_at IS NULL", aggregateID).Count(&pendingOutbox).Error; err != nil {
		t.Fatal(err)
	}
	if pendingOutbox != 0 {
		t.Fatalf("pending outbox=%d", pendingOutbox)
	}
}

func cleanupConsumerOwner(t *testing.T, db *gorm.DB, ownerID uint64) {
	t.Helper()
	var assetIDs []uint64
	if err := db.Model(&Asset{}).Where("owner_id = ?", ownerID).Pluck("id", &assetIDs).Error; err != nil {
		t.Errorf("find Module 7 assets: %v", err)
		return
	}
	if len(assetIDs) == 0 {
		return
	}
	var aggregateIDs []string
	if err := db.Model(&ReviewProjection{}).Where("asset_id IN ?", assetIDs).Pluck("projection_id", &aggregateIDs).Error; err != nil {
		t.Errorf("find Module 7 projections: %v", err)
	}
	if len(aggregateIDs) > 0 {
		var eventIDs []string
		if err := db.Model(&OutboxEvent{}).Where("aggregate_id IN ?", aggregateIDs).Pluck("event_id", &eventIDs).Error; err != nil {
			t.Errorf("find Module 7 outbox: %v", err)
		}
		if len(eventIDs) > 0 {
			if err := db.Where("event_id IN ?", eventIDs).Delete(&ConsumerInbox{}).Error; err != nil {
				t.Errorf("cleanup Module 7 inbox: %v", err)
			}
		}
		if err := db.Where("aggregate_id IN ?", aggregateIDs).Delete(&ConsumerCheckpoint{}).Error; err != nil {
			t.Errorf("cleanup Module 7 checkpoints: %v", err)
		}
		if err := db.Where("aggregate_id IN ?", aggregateIDs).Delete(&OutboxEvent{}).Error; err != nil {
			t.Errorf("cleanup Module 7 outbox: %v", err)
		}
		if err := db.Where("projection_id IN ?", aggregateIDs).Delete(&ReviewDecision{}).Error; err != nil {
			t.Errorf("cleanup Module 7 decisions: %v", err)
		}
	}
	if err := db.Where("asset_id IN ?", assetIDs).Delete(&ReviewProjection{}).Error; err != nil {
		t.Errorf("cleanup Module 7 projections: %v", err)
	}
	if err := db.Where("id IN ?", assetIDs).Delete(&Asset{}).Error; err != nil {
		t.Errorf("cleanup Module 7 assets: %v", err)
	}
}

func applyConsumerSQLFile(t *testing.T, db *gorm.DB, path string) {
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

func assertConsumerSchemaBoundary(t *testing.T, db *gorm.DB) {
	t.Helper()
	var foreignTables []string
	if err := db.Raw(`SELECT DISTINCT referenced_table_name FROM information_schema.key_column_usage
		WHERE table_schema = DATABASE() AND table_name IN ('media_event_inbox','media_consumer_checkpoints')
		AND referenced_table_name IS NOT NULL`).Scan(&foreignTables).Error; err != nil {
		t.Fatal(err)
	}
	for _, table := range foreignTables {
		if table != "media_outbox" {
			t.Fatalf("cross-domain foreign key found: %s", table)
		}
	}
	if len(foreignTables) != 1 {
		t.Fatalf("unexpected consumer foreign key count: %d", len(foreignTables))
	}
}

func TestZZMediaConsumerDatabaseResidueAgainstMySQL84(t *testing.T) {
	db := openModule7MySQL(t)
	owners := []uint64{99_000_701, 99_000_702, 99_000_703, 99_000_704, 99_000_705, 99_000_706, 99_000_707}
	var assetCount int64
	if err := db.Model(&Asset{}).Where("owner_id IN ?", owners).Count(&assetCount).Error; err != nil {
		t.Fatal(err)
	}
	assetSubquery := db.Model(&Asset{}).Select("id").Where("owner_id IN ?", owners)
	projectionSubquery := db.Model(&ReviewProjection{}).Select("projection_id").Where("asset_id IN (?)", assetSubquery)
	var projectionCount, decisionCount, inboxCount, checkpointCount, outboxCount int64
	if err := db.Model(&ReviewProjection{}).Where("asset_id IN (?)", assetSubquery).Count(&projectionCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ReviewDecision{}).Where("projection_id IN (?)", projectionSubquery).Count(&decisionCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ConsumerInbox{}).Where("aggregate_id IN (?)", projectionSubquery).Count(&inboxCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&ConsumerCheckpoint{}).Where("aggregate_id IN (?)", projectionSubquery).Count(&checkpointCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&OutboxEvent{}).Where("aggregate_id IN (?)", projectionSubquery).Count(&outboxCount).Error; err != nil {
		t.Fatal(err)
	}
	if assetCount+projectionCount+decisionCount+inboxCount+checkpointCount+outboxCount != 0 {
		t.Fatalf("Module 7 residue assets=%d projections=%d decisions=%d inbox=%d checkpoints=%d outbox=%d", assetCount, projectionCount, decisionCount, inboxCount, checkpointCount, outboxCount)
	}
}
