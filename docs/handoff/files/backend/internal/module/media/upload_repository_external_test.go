package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestUploadTicketLifecycleAgainstMySQL84(t *testing.T) {
	dsn := os.Getenv("MEDIA_MODULE3_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set MEDIA_MODULE3_MYSQL_DSN to run Module 3 database acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ownerID := uint64(99_000_101)
	cleanupUploadOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupUploadOwner(t, db, ownerID) })

	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	authorizer := &uploadAuthorizerStub{}
	service, err := NewUploadTicketService(NewRepository(db), authorizer, UploadTicketServiceConfig{
		Environment: "acceptance", Now: func() time.Time { return now }, Quota: DefaultQuotaPolicy(),
	})
	if err != nil {
		t.Fatal(err)
	}
	input := validCreateUploadInput()
	input.OwnerID, input.IdempotencyKey = ownerID, "module3-lifecycle"
	bundle, created, err := service.CreateTicket(context.Background(), input)
	if err != nil || !created {
		t.Fatalf("create ticket created=%v err=%v", created, err)
	}
	replayed, replayCreated, err := service.CreateTicket(context.Background(), input)
	if err != nil || replayCreated || replayed.Ticket.TicketID != bundle.Ticket.TicketID || replayed.Asset.AssetPublicID != bundle.Asset.AssetPublicID {
		t.Fatalf("idempotent replay failed created=%v err=%v", replayCreated, err)
	}
	conflict := input
	conflict.ExpectedSize++
	if _, _, err := service.CreateTicket(context.Background(), conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}

	authorization, err := service.IssueAuthorization(context.Background(), ownerID, bundle.Ticket.TicketID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if authorization.RequiredHeaders.ContentMD5 != input.ExpectedMD5 || authorizer.request.ObjectKey != *bundle.Asset.StagingKey {
		t.Fatal("authorization does not bind ticket declaration")
	}
	if _, err := service.IssueAuthorization(context.Background(), ownerID, bundle.Ticket.TicketID, 2); !errors.Is(err, ErrAuthorizationAlreadyIssued) {
		t.Fatalf("second authorization was not rejected: %v", err)
	}
	if _, err := service.MarkUploaded(context.Background(), ownerID+1, bundle.Ticket.TicketID, 2); !errors.Is(err, ErrTicketForbidden) {
		t.Fatalf("owner boundary failed: %v", err)
	}
	uploaded, err := service.MarkUploaded(context.Background(), ownerID, bundle.Ticket.TicketID, 2)
	if err != nil || uploaded.Ticket.Status != TicketUploaded || uploaded.Ticket.RowVersion != 3 {
		t.Fatalf("uploaded transition failed: %#v err=%v", uploaded.Ticket, err)
	}
	if _, err := service.MarkUploaded(context.Background(), ownerID, bundle.Ticket.TicketID, 2); err != nil {
		t.Fatalf("uploaded idempotent replay failed: %v", err)
	}
	consumed, err := service.Consume(context.Background(), ownerID, bundle.Ticket.TicketID, 3)
	if err != nil || consumed.Ticket.Status != TicketConsumed || consumed.Reservation.Status != ReservationCommitted {
		t.Fatalf("consume transaction failed ticket=%s reservation=%s err=%v", consumed.Ticket.Status, consumed.Reservation.Status, err)
	}
	if _, err := service.Consume(context.Background(), ownerID, bundle.Ticket.TicketID, 4); err != nil {
		t.Fatalf("consume idempotent replay failed: %v", err)
	}
}

func TestUploadTicketCancelAndExpiryReleaseQuotaAgainstMySQL84(t *testing.T) {
	dsn := os.Getenv("MEDIA_MODULE3_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set MEDIA_MODULE3_MYSQL_DSN to run Module 3 database acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ownerID := uint64(99_000_102)
	cleanupUploadOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupUploadOwner(t, db, ownerID) })

	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	service, _ := NewUploadTicketService(NewRepository(db), &uploadAuthorizerStub{}, UploadTicketServiceConfig{
		Environment: "acceptance", TicketTTL: time.Minute, Now: func() time.Time { return now },
	})
	input := validCreateUploadInput()
	input.OwnerID, input.IdempotencyKey = ownerID, "module3-cancel"
	cancelBundle, _, err := service.CreateTicket(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := service.Cancel(context.Background(), ownerID, cancelBundle.Ticket.TicketID, 1)
	if err != nil || cancelled.Ticket.Status != TicketCancelled || cancelled.Reservation.Status != ReservationReleased {
		t.Fatalf("cancel failed ticket=%s reservation=%s err=%v", cancelled.Ticket.Status, cancelled.Reservation.Status, err)
	}

	input.IdempotencyKey = "module3-expire"
	expireBundle, _, err := service.CreateTicket(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	count, err := service.Expire(context.Background(), 100)
	if err != nil || count != 1 {
		t.Fatalf("expiry worker count=%d err=%v", count, err)
	}
	loaded, err := NewRepository(db).FindUploadBundle(context.Background(), expireBundle.Ticket.TicketID, ownerID)
	if err != nil || loaded.Ticket.Status != TicketExpired || loaded.Reservation.Status != ReservationReleased {
		t.Fatalf("expiry did not release quota: %#v err=%v", loaded, err)
	}
	input.IdempotencyKey = "module3-expire-retry"
	retryBundle, created, err := service.CreateTicket(context.Background(), input)
	if err != nil || !created || retryBundle.Ticket.TicketID == expireBundle.Ticket.TicketID ||
		retryBundle.Asset.AssetPublicID == expireBundle.Asset.AssetPublicID || *retryBundle.Asset.StagingKey == *expireBundle.Asset.StagingKey {
		t.Fatalf("expired retry did not create a new asset/ticket/key: created=%v err=%v", created, err)
	}
}

func TestAtomicQuotaReservationUnderConcurrencyAgainstMySQL84(t *testing.T) {
	dsn := os.Getenv("MEDIA_MODULE3_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set MEDIA_MODULE3_MYSQL_DSN to run Module 3 database acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ownerID := uint64(99_000_103)
	cleanupUploadOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupUploadOwner(t, db, ownerID) })
	policy := DefaultQuotaPolicy()
	policy.ActiveCount = 5
	service, _ := NewUploadTicketService(NewRepository(db), &uploadAuthorizerStub{}, UploadTicketServiceConfig{
		Environment: "acceptance", Quota: policy,
	})

	var success, quotaRejected, unexpected atomic.Int32
	var wait sync.WaitGroup
	start := make(chan struct{})
	for index := 0; index < 20; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			input := validCreateUploadInput()
			input.OwnerID = ownerID
			input.IdempotencyKey = fmt.Sprintf("module3-concurrent-%02d", index)
			_, created, err := service.CreateTicket(context.Background(), input)
			switch {
			case err == nil && created:
				success.Add(1)
			case errors.Is(err, ErrQuotaExceeded):
				quotaRejected.Add(1)
			default:
				unexpected.Add(1)
			}
		}(index)
	}
	close(start)
	wait.Wait()
	if success.Load() != 5 || quotaRejected.Load() != 15 || unexpected.Load() != 0 {
		t.Fatalf("success=%d quotaRejected=%d unexpected=%d", success.Load(), quotaRejected.Load(), unexpected.Load())
	}
	var active int64
	if err := db.Model(&QuotaReservation{}).Where("owner_id = ? AND status = ?", ownerID, ReservationActive).Count(&active).Error; err != nil {
		t.Fatal(err)
	}
	if active != 5 {
		t.Fatalf("quota drift: active=%d", active)
	}
}

func TestAuthorizationFailureFailClosedAgainstMySQL84(t *testing.T) {
	dsn := os.Getenv("MEDIA_MODULE3_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set MEDIA_MODULE3_MYSQL_DSN to run Module 3 database acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	ownerID := uint64(99_000_104)
	cleanupUploadOwner(t, db, ownerID)
	t.Cleanup(func() { cleanupUploadOwner(t, db, ownerID) })
	service, err := NewUploadTicketService(NewRepository(db), &uploadAuthorizerStub{err: errors.New("forced STS failure")}, UploadTicketServiceConfig{Environment: "acceptance"})
	if err != nil {
		t.Fatal(err)
	}
	input := validCreateUploadInput()
	input.OwnerID, input.IdempotencyKey = ownerID, "module3-fail-closed"
	bundle, _, err := service.CreateTicket(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.IssueAuthorization(context.Background(), ownerID, bundle.Ticket.TicketID, 1); err == nil {
		t.Fatal("forced authorization failure unexpectedly succeeded")
	}
	loaded, err := NewRepository(db).FindUploadBundle(context.Background(), bundle.Ticket.TicketID, ownerID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Ticket.Status != TicketCancelled || loaded.Ticket.CredentialIssueCount != 1 || loaded.Reservation.Status != ReservationReleased {
		t.Fatalf("authorization failure was not fail-closed: ticket=%s issues=%d reservation=%s", loaded.Ticket.Status, loaded.Ticket.CredentialIssueCount, loaded.Reservation.Status)
	}
}

func cleanupUploadOwner(t *testing.T, db *gorm.DB, ownerID uint64) {
	t.Helper()
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
