package miniappflow

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type mysqlAuthorizer struct{}

func (mysqlAuthorizer) IssueSinglePUT(_ context.Context, request coremedia.AuthorizationRequest) (coremedia.UploadAuthorization, error) {
	return coremedia.UploadAuthorization{
		UploadURL: "https://example.invalid/exact-key",
		RequiredHeaders: coremedia.RequiredUploadHeaders{
			ContentType: request.ContentType, ContentMD5: request.MD5, ContentLength: request.Size, ForbidOverwrite: "true",
		},
		ExpiresAt: time.Now().UTC().Add(time.Minute),
	}, nil
}

func TestModule10TicketAndStatusAgainstMySQL84(t *testing.T) {
	db := openModule10MySQL(t)
	ownerID := uint64(99_001_001)
	cleanupModule10Owner(t, db, ownerID)
	t.Cleanup(func() { cleanupModule10Owner(t, db, ownerID) })

	repository := coremedia.NewRepository(db)
	tickets, err := coremedia.NewUploadTicketService(repository, mysqlAuthorizer{}, coremedia.UploadTicketServiceConfig{Environment: "acceptance"})
	if err != nil {
		t.Fatal(err)
	}
	bundle, created, err := tickets.CreateTicket(context.Background(), coremedia.CreateUploadTicketInput{
		OwnerID: ownerID, Purpose: coremedia.PurposeAvatar, ExpectedSize: 68,
		ExpectedMD5: "1B2M2Y8AsgTpgAmY7PhCfg==", ExpectedContentType: "image/png",
		IdempotencyKey: "module10-mysql-status",
	})
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if _, err := tickets.IssueAuthorization(context.Background(), ownerID, bundle.Ticket.TicketID, bundle.Ticket.RowVersion); err != nil {
		t.Fatal(err)
	}
	statusRepository := NewStatusRepository(db)
	fact, err := statusRepository.FindStatus(context.Background(), ownerID, bundle.Asset.AssetPublicID)
	if err != nil || fact.TicketStatus != coremedia.TicketAuthorizationIssued || fact.TicketID != bundle.Ticket.TicketID {
		t.Fatalf("fact=%#v err=%v", fact, err)
	}
	if _, err := statusRepository.FindStatus(context.Background(), ownerID+1, bundle.Asset.AssetPublicID); !errors.Is(err, coremedia.ErrNotFound) {
		t.Fatalf("cross-owner read was not rejected: %v", err)
	}
	values, total, err := statusRepository.ListStatus(context.Background(), ownerID, 1, 20)
	if err != nil || total != 1 || len(values) != 1 {
		t.Fatalf("values=%d total=%d err=%v", len(values), total, err)
	}
}

func TestModule10LocalMockAutoApprovesProjectionAgainstMySQL84(t *testing.T) {
	db := openModule10MySQL(t)
	ownerID := uint64(99_001_002)
	cleanupModule10Owner(t, db, ownerID)
	t.Cleanup(func() { cleanupModule10Owner(t, db, ownerID) })

	repository := coremedia.NewRepository(db)
	tickets, err := coremedia.NewUploadTicketService(repository, mysqlAuthorizer{}, coremedia.UploadTicketServiceConfig{Environment: "local-mock"})
	if err != nil {
		t.Fatal(err)
	}
	bundle, created, err := tickets.CreateTicket(context.Background(), coremedia.CreateUploadTicketInput{
		OwnerID: ownerID, Purpose: coremedia.PurposeDish, ExpectedSize: 68,
		ExpectedMD5: "1B2M2Y8AsgTpgAmY7PhCfg==", ExpectedContentType: "image/png",
		IdempotencyKey: "module10-local-mock-auto-approve",
	})
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
	if _, err := tickets.IssueAuthorization(context.Background(), ownerID, bundle.Ticket.TicketID, bundle.Ticket.RowVersion); err != nil {
		t.Fatal(err)
	}
	projections, err := coremedia.NewReviewProjectionService(repository, nil)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := coremedia.NewMediaConsumer(repository, projections, coremedia.MediaConsumerConfig{})
	if err != nil {
		t.Fatal(err)
	}
	application, err := NewService(
		tickets, repository, mysqlCompletion{db: db, asset: bundle.Asset}, coremedia.NewService(repository),
		projections, repository, consumer, NewStatusRepository(db), nil, WithLocalMockAutoApproval(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	status, err := application.CompleteUpload(context.Background(), ownerID, bundle.Ticket.TicketID)
	if err != nil || status.State != StateApproved || !status.CanUse || status.ReviewStatus != string(coremedia.ProjectionApproved) {
		t.Fatalf("status=%#v err=%v", status, err)
	}

	var projection coremedia.ReviewProjection
	if err := db.Where("asset_id = ?", bundle.Asset.ID).First(&projection).Error; err != nil {
		t.Fatal(err)
	}
	if projection.Status != coremedia.ProjectionApproved || projection.AppliedDecisionVersion != 1 {
		t.Fatalf("projection=%#v", projection)
	}
	var decisions int64
	if err := db.Model(&coremedia.ReviewDecision{}).Where("projection_id = ?", projection.ProjectionID).Count(&decisions).Error; err != nil || decisions != 1 {
		t.Fatalf("decisions=%d err=%v", decisions, err)
	}
}

type mysqlCompletion struct {
	db    *gorm.DB
	asset coremedia.Asset
}

func (c mysqlCompletion) CompleteUpload(ctx context.Context, ownerID uint64, ticketID string) (coremedia.Asset, error) {
	finalKey := "final/local-mock/" + c.asset.AssetPublicID + ".webp"
	finalVersion := "local-mock-version-" + c.asset.AssetPublicID
	if err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&coremedia.Asset{}).Where("id = ? AND owner_id = ?", c.asset.ID, ownerID).Updates(map[string]any{
			"storage_stage": coremedia.StorageStageFinal, "technical_status": coremedia.TechnicalFinalReady,
			"final_key": finalKey, "final_version_id": finalVersion, "row_version": gorm.Expr("row_version + 1"),
			"updated_at": time.Now().UTC(),
		}).Error; err != nil {
			return err
		}
		return tx.Model(&coremedia.UploadTicket{}).Where("ticket_id = ?", ticketID).Updates(map[string]any{
			"status": coremedia.TicketConsumed, "row_version": gorm.Expr("row_version + 1"), "updated_at": time.Now().UTC(),
		}).Error
	}); err != nil {
		return coremedia.Asset{}, err
	}
	c.asset.StorageStage = coremedia.StorageStageFinal
	c.asset.TechnicalStatus = coremedia.TechnicalFinalReady
	c.asset.FinalKey = &finalKey
	c.asset.FinalVersionID = &finalVersion
	return c.asset, nil
}

func openModule10MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	_ = godotenv.Load("../../../../.env")
	dsn := strings.TrimSpace(os.Getenv("MEDIA_MODULE10_MYSQL_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	}
	if dsn == "" {
		t.Skip("set MEDIA_MODULE10_MYSQL_DSN or MYSQL_DSN")
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
		t.Fatalf("Module 10 requires real MySQL 8.4, got %q", version)
	}
	for _, table := range []string{
		"media_assets", "media_upload_tickets", "media_quota_reservations", "media_upload_completions",
		"media_review_projections", "media_outbox", "media_event_inbox", "media_consumer_checkpoints",
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required frozen table %s is missing", table)
		}
	}
	return db
}

func cleanupModule10Owner(t *testing.T, db *gorm.DB, ownerID uint64) {
	t.Helper()
	err := db.Transaction(func(tx *gorm.DB) error {
		statements := []string{
			`DELETE i FROM media_event_inbox i JOIN media_outbox o ON o.event_id = i.event_id JOIN media_review_projections p ON p.projection_id = o.aggregate_id JOIN media_assets a ON a.id = p.asset_id WHERE a.owner_id = ?`,
			`DELETE c FROM media_consumer_checkpoints c JOIN media_review_projections p ON p.projection_id = c.aggregate_id JOIN media_assets a ON a.id = p.asset_id WHERE a.owner_id = ?`,
			`DELETE o FROM media_outbox o JOIN media_review_projections p ON p.projection_id = o.aggregate_id JOIN media_assets a ON a.id = p.asset_id WHERE a.owner_id = ?`,
			`DELETE d FROM media_review_decisions d JOIN media_review_projections p ON p.projection_id = d.projection_id JOIN media_assets a ON a.id = p.asset_id WHERE a.owner_id = ?`,
			`DELETE p FROM media_review_projections p JOIN media_assets a ON a.id = p.asset_id WHERE a.owner_id = ?`,
			`DELETE c FROM media_upload_completions c JOIN media_assets a ON a.id = c.asset_id WHERE a.owner_id = ?`,
			`DELETE r FROM media_asset_references r JOIN media_assets a ON a.id = r.asset_id WHERE a.owner_id = ?`,
			`DELETE FROM media_quota_reservations WHERE owner_id = ?`,
			`DELETE FROM media_upload_tickets WHERE owner_id = ?`,
			`DELETE FROM media_assets WHERE owner_id = ?`,
		}
		for _, statement := range statements {
			if err := tx.Exec(statement, ownerID).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("cleanup Module 10 owner: %v", err)
	}
	var residue int64
	if err := db.Table("media_assets").Where("owner_id = ?", ownerID).Count(&residue).Error; err != nil || residue != 0 {
		t.Fatalf("Module 10 database residue=%d err=%v", residue, err)
	}
}
