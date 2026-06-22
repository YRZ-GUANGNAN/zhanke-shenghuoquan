package media

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestUploadCompletionMigrationAndRollbackAgainstMySQL84(t *testing.T) {
	db := openModule4MySQLRaw(t)
	if db.Migrator().HasTable(&UploadCompletion{}) {
		var count int64
		if err := db.Model(&UploadCompletion{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("refusing rollback acceptance against non-empty media_upload_completions: %d rows", count)
		}
	}
	up, err := os.ReadFile("../../../migrations/011_media_upload_completion.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../../migrations/rollback/011_media_upload_completion.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(down)).Error; err != nil {
		t.Fatalf("rollback migration: %v", err)
	}
	if db.Migrator().HasTable(&UploadCompletion{}) {
		t.Fatal("rollback left media_upload_completions behind")
	}
	if err := db.Exec(string(up)).Error; err != nil {
		t.Fatalf("forward migration: %v", err)
	}
	if !db.Migrator().HasTable(&UploadCompletion{}) {
		t.Fatal("forward migration did not create media_upload_completions")
	}
}

func TestUploadCompletionLifecycleAgainstMySQL84(t *testing.T) {
	db := openModule4MySQL(t)
	ownerID := uint64(99_000_401)
	cleanupCompletionOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupCompletionOwner(t, db, ownerID) })

	payload := unitPNG(t)
	now := time.Date(2026, 6, 21, 13, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	bundle := prepareUploadedBundle(t, repository, ownerID, "module4-mysql-success", payload, now)
	storage := completionStoreForBundle(bundle, payload, now)
	service, err := NewCompletionService(repository, storage, CompletionServiceConfig{Environment: "acceptance", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.CompleteUpload(context.Background(), ownerID, bundle.Ticket.TicketID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CompleteUpload(context.Background(), ownerID, bundle.Ticket.TicketID)
	if err != nil {
		t.Fatal(err)
	}
	if first.TechnicalStatus != TechnicalFinalReady || first.StorageStage != StorageStageFinal || first.FinalVersionID == nil ||
		second.FinalVersionID == nil || *first.FinalVersionID != *second.FinalVersionID || storage.promoteCalls != 1 {
		t.Fatalf("MySQL completion is not idempotent: first=%#v second=%#v copies=%d", first, second, storage.promoteCalls)
	}
	loaded, err := repository.FindUploadBundle(context.Background(), bundle.Ticket.TicketID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Ticket.Status != TicketConsumed || loaded.Reservation.Status != ReservationCommitted || loaded.Asset.TechnicalStatus != TechnicalFinalReady {
		t.Fatalf("atomic finalize failed: asset=%s ticket=%s reservation=%s", loaded.Asset.TechnicalStatus, loaded.Ticket.Status, loaded.Reservation.Status)
	}
}

func TestUploadCompletionValidationFailureReleasesReservationAgainstMySQL84(t *testing.T) {
	db := openModule4MySQL(t)
	ownerID := uint64(99_000_402)
	cleanupCompletionOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupCompletionOwner(t, db, ownerID) })

	payload := unitPNG(t)
	now := time.Date(2026, 6, 21, 13, 30, 0, 0, time.UTC)
	repository := NewRepository(db)
	bundle := prepareUploadedBundle(t, repository, ownerID, "module4-mysql-invalid", payload, now)
	storage := completionStoreForBundle(bundle, payload, now)
	storage.staging.ContentType = "image/jpeg"
	service, _ := NewCompletionService(repository, storage, CompletionServiceConfig{Environment: "acceptance", Now: func() time.Time { return now }})
	if _, err := service.CompleteUpload(context.Background(), ownerID, bundle.Ticket.TicketID); !errors.Is(err, ErrCompletionValidation) {
		t.Fatalf("expected fail-closed validation, got %v", err)
	}
	loaded, err := repository.FindUploadBundle(context.Background(), bundle.Ticket.TicketID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	work, err := repository.FindCompletion(context.Background(), ownerID, bundle.Ticket.TicketID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Asset.TechnicalStatus != TechnicalDeletePending || loaded.Ticket.Status != TicketCancelled ||
		loaded.Reservation.Status != ReservationReleased || work.Completion.Phase != CompletionValidationFailed {
		t.Fatalf("failure transaction drifted: asset=%s ticket=%s reservation=%s completion=%s", loaded.Asset.TechnicalStatus, loaded.Ticket.Status, loaded.Reservation.Status, work.Completion.Phase)
	}
}

func TestUploadCompletionCASConflictAgainstMySQL84(t *testing.T) {
	db := openModule4MySQL(t)
	ownerID := uint64(99_000_403)
	cleanupCompletionOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupCompletionOwner(t, db, ownerID) })

	payload := unitPNG(t)
	now := time.Date(2026, 6, 21, 14, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	bundle := prepareUploadedBundle(t, repository, ownerID, "module4-mysql-cas", payload, now)
	work, err := repository.ClaimCompletion(context.Background(), ownerID, bundle.Ticket.TicketID, now, time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&UploadCompletion{}).Where("ticket_id = ?", bundle.Ticket.TicketID).Update("row_version", gorm.Expr("row_version + 1")).Error; err != nil {
		t.Fatal(err)
	}
	storage := completionStoreForBundle(bundle, payload, now)
	if _, err := repository.PinCompletionSource(context.Background(), work, storage.staging, now); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("expected stale row_version CAS conflict, got %v", err)
	}
}

func openModule4MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	db := openModule4MySQLRaw(t)
	if !db.Migrator().HasTable(&UploadCompletion{}) {
		t.Fatal("migration 011_media_upload_completion.sql has not been applied")
	}
	return db
}

func openModule4MySQLRaw(t *testing.T) *gorm.DB {
	t.Helper()
	_ = godotenv.Load("../../../.env")
	dsn := os.Getenv("MEDIA_MODULE4_MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("set MEDIA_MODULE4_MYSQL_DSN or MYSQL_DSN to run Module 4 database acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func prepareUploadedBundle(t *testing.T, repository *GormRepository, ownerID uint64, idempotencyKey string, payload []byte, now time.Time) UploadBundle {
	t.Helper()
	digest := md5.Sum(payload)
	uploads, err := NewUploadTicketService(repository, &uploadAuthorizerStub{}, UploadTicketServiceConfig{Environment: "acceptance", Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	input := CreateUploadTicketInput{
		OwnerID: ownerID, Purpose: PurposePost, ExpectedSize: uint64(len(payload)),
		ExpectedMD5: base64.StdEncoding.EncodeToString(digest[:]), ExpectedContentType: "image/png", IdempotencyKey: idempotencyKey,
	}
	bundle, created, err := uploads.CreateTicket(context.Background(), input)
	if err != nil || !created {
		t.Fatalf("create upload bundle created=%v err=%v", created, err)
	}
	if _, err := uploads.IssueAuthorization(context.Background(), ownerID, bundle.Ticket.TicketID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := uploads.MarkUploaded(context.Background(), ownerID, bundle.Ticket.TicketID, 2); err != nil {
		t.Fatal(err)
	}
	if err := NewService(repository).TransitionTechnical(context.Background(), bundle.Asset.ID, 1, TechnicalPendingUpload, TechnicalUploaded); err != nil {
		t.Fatal(err)
	}
	loaded, err := repository.FindUploadBundle(context.Background(), bundle.Ticket.TicketID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

func completionStoreForBundle(bundle UploadBundle, payload []byte, now time.Time) *completionStoreStub {
	return &completionStoreStub{
		stagingBytes: payload,
		staging: CompletionObject{
			Key: *bundle.Asset.StagingKey, VersionID: "mysql-staging-version", ETag: hexMD5(payload),
			ContentType: bundle.Ticket.ExpectedContentType, Size: uint64(len(payload)), LastModified: now,
		},
	}
}

func cleanupCompletionOwner(t *testing.T, db *gorm.DB, ownerID uint64) {
	t.Helper()
	var assetIDs []uint64
	if err := db.Model(&Asset{}).Where("owner_id = ?", ownerID).Pluck("id", &assetIDs).Error; err != nil {
		t.Errorf("find completion assets: %v", err)
		return
	}
	if len(assetIDs) > 0 {
		if err := db.Where("asset_id IN ?", assetIDs).Delete(&UploadCompletion{}).Error; err != nil {
			t.Errorf("cleanup completions: %v", err)
		}
	}
	if err := db.Where("owner_id = ?", ownerID).Delete(&QuotaReservation{}).Error; err != nil {
		t.Errorf("cleanup reservations: %v", err)
	}
	if err := db.Where("owner_id = ?", ownerID).Delete(&UploadTicket{}).Error; err != nil {
		t.Errorf("cleanup tickets: %v", err)
	}
	if err := db.Where("owner_id = ?", ownerID).Delete(&Asset{}).Error; err != nil {
		t.Errorf("cleanup assets: %v", err)
	}
}

func TestZZUploadCompletionDatabaseResidueAgainstMySQL84(t *testing.T) {
	db := openModule4MySQL(t)
	owners := []uint64{99_000_401, 99_000_402, 99_000_403, 99_000_404}
	for _, model := range []any{&Asset{}, &UploadTicket{}, &QuotaReservation{}} {
		var count int64
		if err := db.Model(model).Where("owner_id IN ?", owners).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("Module 4 database residue in %T: %d", model, count)
		}
	}
	var completionCount int64
	if err := db.Model(&UploadCompletion{}).
		Where("asset_id IN (?)", db.Model(&Asset{}).Select("id").Where("owner_id IN ?", owners)).
		Count(&completionCount).Error; err != nil {
		t.Fatal(err)
	}
	if completionCount != 0 {
		t.Fatalf("Module 4 completion residue: %d", completionCount)
	}
}
