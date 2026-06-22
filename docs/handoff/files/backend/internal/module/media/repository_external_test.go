package media

import (
	"context"
	"os"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestRepositoryAgainstMigratedMySQL(t *testing.T) {
	dsn := os.Getenv("MEDIA_MODULE2_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set MEDIA_MODULE2_MYSQL_DSN to run the Module 2 database acceptance test")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(db)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)

	assetID, _ := NewULID()
	operationID, _ := NewULID()
	ticketID, _ := NewULID()
	reservationID, _ := NewULID()
	eventID, _ := NewULID()
	asset := Asset{
		AssetPublicID: assetID, OwnerID: 99_000_001, Purpose: PurposePost,
		StorageStage: StorageStageStaging, ReviewStatus: ReviewPending,
		TechnicalStatus: TechnicalPendingUpload, ProjectedDecision: ReviewPending,
		IdempotencyKey: "module2-external-" + assetID, RowVersion: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateAsset(ctx, &asset); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM media_outbox WHERE event_id = ?", eventID)
		db.Exec("DELETE FROM media_quota_reservations WHERE reservation_id = ?", reservationID)
		db.Exec("DELETE FROM media_upload_tickets WHERE ticket_id = ?", ticketID)
		db.Exec("DELETE FROM media_asset_references WHERE asset_id = ?", asset.ID)
		db.Exec("DELETE FROM media_assets WHERE id = ?", asset.ID)
	})

	if err := repository.CASAssetTechnicalStatus(ctx, asset.ID, 1, TechnicalPendingUpload, TechnicalUploaded); err != nil {
		t.Fatal(err)
	}
	if err := repository.CASAssetTechnicalStatus(ctx, asset.ID, 1, TechnicalPendingUpload, TechnicalUploaded); err != ErrConcurrentUpdate {
		t.Fatalf("stale CAS returned %v", err)
	}

	reference := AssetReference{
		AssetID: asset.ID, ReferenceType: PurposePost, ReferenceID: "acceptance-post-1", Slot: 0,
		Status: ReferencePending, ParentVisibility: ParentHidden, AggregateVersion: 1,
		OperationID: operationID, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateReference(ctx, &reference); err != nil {
		t.Fatal(err)
	}
	if err := repository.CASReferenceStatus(ctx, reference.ID, 1, ReferencePending, ReferenceActive); err != nil {
		t.Fatal(err)
	}
	count, err := repository.RebuildActiveRefCount(ctx, asset.ID)
	if err != nil || count != 1 {
		t.Fatalf("reference cache rebuild count=%d err=%v", count, err)
	}

	ticket := UploadTicket{
		TicketID: ticketID, AssetID: asset.ID, OwnerID: asset.OwnerID, ExpectedSize: 1024,
		ExpectedMD5: "1B2M2Y8AsgTpgAmY7PhCfg==", ExpectedContentType: "image/webp",
		Status: TicketCreated, ExpiresAt: now.Add(30 * time.Minute), RowVersion: 1,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateUploadTicket(ctx, &ticket); err != nil {
		t.Fatal(err)
	}
	if err := repository.CASTicketStatus(ctx, ticketID, 1, TicketCreated, TicketAuthorizationIssued); err != nil {
		t.Fatal(err)
	}

	reservation := QuotaReservation{
		ReservationID: reservationID, OwnerID: asset.OwnerID, BytesReserved: 1024, CountReserved: 1,
		Status: ReservationActive, TicketID: ticketID, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.CreateQuotaReservation(ctx, &reservation); err != nil {
		t.Fatal(err)
	}
	if err := repository.CASQuotaReservationStatus(ctx, reservationID, 1, ReservationActive, ReservationCommitted); err != nil {
		t.Fatal(err)
	}

	event := OutboxEvent{
		EventID: eventID, AggregateID: assetID, AggregateVersion: 1, Operation: "AssetRecorded",
		Payload: JSONDocument(`{"assetId":"` + assetID + `"}`), RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := repository.AppendOutbox(ctx, &event); err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkOutboxProcessed(ctx, eventID, 1, now); err != nil {
		t.Fatal(err)
	}
}
