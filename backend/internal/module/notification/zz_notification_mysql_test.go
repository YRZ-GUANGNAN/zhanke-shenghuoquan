package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var isolatedNotificationSequence atomic.Uint64

// Never accept a normal application DSN. Both runner coordinates and the
// freshly spawned server's data directory must match before creating a DB.
func openIsolatedNotificationMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("NOTIFICATION_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("run node scripts/test-notification-mysql.cjs for dedicated isolated MySQL 8.4")
	}
	cfg, err := mysqlDriver.ParseDSN(dsn)
	if err != nil || cfg.Net != "tcp" || cfg.Addr != "127.0.0.1:33319" || cfg.User != "root" || cfg.Passwd != "" || !regexp.MustCompile(`^campus_notification_runner_[0-9]+$`).MatchString(cfg.DBName) {
		t.Fatal("only the dedicated notification runner is allowed; shared databases are forbidden")
	}
	expectedDir := os.Getenv("NOTIFICATION_TEST_DATADIR")
	resolvedDir, err := filepath.EvalSymlinks(expectedDir)
	temporaryRoot, rootErr := filepath.EvalSymlinks(os.TempDir())
	if err != nil || rootErr != nil || !strings.EqualFold(filepath.Dir(resolvedDir), temporaryRoot) || !strings.HasPrefix(filepath.Base(resolvedDir), "campus-notification-mysql-") {
		t.Fatal("invalid isolated notification data directory")
	}
	cfg.DBName, cfg.MultiStatements, cfg.ParseTime, cfg.Loc = "", false, true, time.UTC
	cfg.Timeout, cfg.ReadTimeout, cfg.WriteTimeout = 3*time.Second, 15*time.Second, 15*time.Second
	cfg.Params = map[string]string{"time_zone": "'+00:00'"}
	server, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	var actualDir, version string
	if err := server.QueryRow("SELECT @@datadir, VERSION()").Scan(&actualDir, &version); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(filepath.Clean(actualDir), filepath.Clean(resolvedDir)) || !strings.HasPrefix(version, "8.4.") {
		t.Fatal("server identity or MySQL 8.4 version does not match isolated runner")
	}
	databaseName := fmt.Sprintf("notification_isolated_%d_%d", time.Now().UnixNano(), isolatedNotificationSequence.Add(1))
	if _, err := server.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := server.Exec("DROP DATABASE `" + databaseName + "`"); err != nil {
			t.Errorf("isolated notification database cleanup failed: %v", err)
		}
	})
	cfg.DBName = databaseName
	database, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(16)
	t.Cleanup(func() { _ = database.Close() })
	db, err := gorm.Open(gormMySQL.New(gormMySQL.Config{Conn: database, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "053_delivery_notifications.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range strings.Split(string(contents), ";") {
		if strings.TrimSpace(statement) != "" {
			if err := db.Exec(statement).Error; err != nil {
				t.Fatalf("migration 053 failed: %v", err)
			}
		}
	}
	t.Logf("isolated_mysql_version=%s database=%s migration=053", version, databaseName)
	return db
}

func isolatedNotificationID(t *testing.T) string {
	t.Helper()
	id, err := newID()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func isolatedNotificationEvent(t *testing.T) Event {
	now := time.Now().UTC().Truncate(time.Millisecond)
	expires := now.Add(time.Hour)
	id := isolatedNotificationID(t)
	return Event{EventPublicID: id, EventType: "OrderStatusChanged", AggregateType: "delivery_order", AggregatePublicID: id,
		AggregateVersion: 1, PayloadVersion: 1, Payload: json.RawMessage(`{"status":"delivering"}`),
		Status: EventPending, RowVersion: 1, OccurredAt: now, CreatedAt: now, UpdatedAt: now, ExpiresAt: &expires}
}

func isolatedNotificationSubscription(t *testing.T, db *gorm.DB) Subscription {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Millisecond)
	item := Subscription{UserID: 12, AppID: "isolated-app", OpenID: "isolated-openid", TemplateKey: "order", AuthorizationNonce: "isolated:" + authorizationTemplateTag("isolated-app", "order", "isolated-template"),
		Status: "accepted", RemainingQuota: 1, RowVersion: 1, AuthorizedAt: now, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	return item
}

func isolatedNotificationDelivery(t *testing.T, db *gorm.DB) DeliveryItem {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Millisecond)
	expires := now.Add(time.Hour)
	item := DeliveryItem{DeliveryPublicID: isolatedNotificationID(t), EventPublicID: isolatedNotificationID(t), RecipientUserID: 12, Channel: ChannelWechatSubscribe,
		Purpose: PurposeDelivery, TemplateKey: "order", TemplateSnapshot: json.RawMessage(`{"templateId":"isolated-template","page":"/pages/notifications/index","data":{"thing1":{"value":"配送中"}}}`),
		PayloadVersion: 1, Status: DeliveryPending, RowVersion: 1, CreatedAt: now, UpdatedAt: now, ExpiresAt: &expires}
	if err := NewRepository(db).EnsureDeliveryTx(db, item); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("delivery_public_id = ?", item.DeliveryPublicID).First(&item).Error; err != nil {
		t.Fatal(err)
	}
	return item
}

func isolatedNotificationClaim(t *testing.T, repo *GormRepository) DeliveryLease {
	t.Helper()
	lease, found, err := repo.ClaimWechatDelivery(context.Background(), time.Now().UTC(), time.Minute)
	if err != nil || !found {
		t.Fatalf("claim delivery found=%v err=%v", found, err)
	}
	return lease
}

func TestNotificationMigrationConstraintsAgainstIsolatedMySQL(t *testing.T) {
	db := openIsolatedNotificationMySQL(t)
	subscription := isolatedNotificationSubscription(t, db)
	for _, query := range []string{
		"UPDATE notification_subscriptions SET remaining_quota=2 WHERE id=?",
		"UPDATE notification_subscriptions SET reserved_quota=1 WHERE id=?",
		"UPDATE notification_subscriptions SET status='exhausted' WHERE id=?",
	} {
		if err := db.Exec(query, subscription.ID).Error; err == nil {
			t.Fatal("migration 053 CHECK constraint was not enforced")
		}
	}
	event := isolatedNotificationEvent(t)
	if err := NewRepository(db).AppendEventTx(db, event); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE delivery_notification_events SET status='processing' WHERE event_public_id=?", event.EventPublicID).Error; err == nil {
		t.Fatal("processing event without a lease was accepted")
	}
}

func TestNotificationEventLeaseCompetitionAgainstIsolatedMySQL(t *testing.T) {
	db := openIsolatedNotificationMySQL(t)
	event := isolatedNotificationEvent(t)
	if err := NewRepository(db).AppendEventTx(db, event); err != nil {
		t.Fatal(err)
	}
	repo := NewEventRepository(db)
	type result struct {
		lease EventLease
		found bool
		err   error
	}
	results := make(chan result, 8)
	start := make(chan struct{})
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			lease, found, err := repo.ClaimNextEvent(context.Background(), time.Now().UTC(), time.Minute)
			results <- result{lease, found, err}
		}()
	}
	close(start)
	group.Wait()
	close(results)
	claims := 0
	var original EventLease
	for got := range results {
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.found {
			claims++
			original = got.lease
		}
	}
	if claims != 1 {
		t.Fatalf("concurrent event claims=%d, want 1", claims)
	}
	if err := db.Model(&Event{}).Where("id=?", original.Event.ID).Update("lease_expires_at", time.Now().UTC().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	replacement, found, err := repo.ClaimNextEvent(context.Background(), time.Now().UTC(), time.Minute)
	if err != nil || !found || replacement.LeaseToken == original.LeaseToken {
		t.Fatalf("reclaim found=%v err=%v", found, err)
	}
	if err := repo.ExpandAndComplete(context.Background(), original, nil, nil, EventSkipped, time.Now().UTC()); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale lease accepted: %v", err)
	}
	if err := repo.ExpandAndComplete(context.Background(), replacement, nil, nil, EventSkipped, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
}

func TestNotificationExpansionIdempotencyAndRollbackAgainstIsolatedMySQL(t *testing.T) {
	db := openIsolatedNotificationMySQL(t)
	repo, events := NewRepository(db), NewEventRepository(db)
	event := isolatedNotificationEvent(t)
	if err := repo.AppendEventTx(db, event); err != nil {
		t.Fatal(err)
	}
	lease, found, err := events.ClaimNextEvent(context.Background(), time.Now().UTC(), time.Minute)
	if err != nil || !found {
		t.Fatal("event not claimable", err)
	}
	recipient := Recipient{UserID: 12, Purpose: PurposeDelivery, Title: "配送中", Summary: "查看订单", AggregateType: event.AggregateType, AggregatePublicID: event.AggregatePublicID, AggregateVersion: event.AggregateVersion, TargetPath: "/pages/notifications/index"}
	inbox, deliveries, err := NewExpander(events, nil).build(event, []Recipient{recipient, recipient}, time.Now().UTC())
	if err != nil || len(inbox) != 1 || len(deliveries) != 1 {
		t.Fatal("build deduplication failed", err)
	}
	bad := append([]DeliveryItem(nil), deliveries...)
	bad[0].EventPublicID = isolatedNotificationID(t)
	if err := events.ExpandAndComplete(context.Background(), lease, inbox, bad, EventExpanded, time.Now().UTC()); !errors.Is(err, ErrInvalidRecord) {
		t.Fatal("invalid expansion not rejected", err)
	}
	var count int64
	if err := db.Model(&InboxItem{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("rolled back expansion retained inbox: %d err=%v", count, err)
	}
	if err := events.ExpandAndComplete(context.Background(), lease, inbox, deliveries, EventExpanded, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if ok, err := repo.MarkInboxRead(context.Background(), 99, inbox[0].InboxPublicID, time.Now().UTC()); err != nil || ok {
		t.Fatal("wrong user marked inbox read", err)
	}
	if ok, err := repo.MarkInboxRead(context.Background(), 12, inbox[0].InboxPublicID, time.Now().UTC()); err != nil || !ok {
		t.Fatal("owner could not mark read", err)
	}
	for i := 0; i < 3; i++ {
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := repo.AppendEventTx(tx, event); err != nil {
				return err
			}
			if err := repo.EnsureInboxTx(tx, inbox[0]); err != nil {
				return err
			}
			return repo.EnsureDeliveryTx(tx, deliveries[0])
		}); err != nil {
			t.Fatal(err)
		}
	}
	items, unread, err := repo.ListInbox(context.Background(), 12, "", 20)
	if err != nil || len(items) != 1 || unread != 0 || items[0].Status != DeliveryRead {
		t.Fatalf("replay reset read state or duplicated inbox: items=%d unread=%d err=%v", len(items), unread, err)
	}
	if err := db.Model(&DeliveryItem{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("duplicate delivery count=%d err=%v", count, err)
	}
}

func TestNotificationQuotaConcurrentReserveReleaseConsumeAgainstIsolatedMySQL(t *testing.T) {
	db := openIsolatedNotificationMySQL(t)
	repo := NewRepository(db)
	subscription := isolatedNotificationSubscription(t, db)
	leases := make([]DeliveryLease, 8)
	for i := range leases {
		isolatedNotificationDelivery(t, db)
		leases[i] = isolatedNotificationClaim(t, repo)
	}
	type result struct {
		lease DeliveryLease
		err   error
	}
	results := make(chan result, len(leases))
	start := make(chan struct{})
	var group sync.WaitGroup
	for _, lease := range leases {
		group.Add(1)
		go func(lease DeliveryLease) {
			defer group.Done()
			<-start
			reserved, err := repo.ReserveForDelivery(context.Background(), lease, "isolated-app", "isolated-openid", "isolated-template", time.Now().UTC())
			results <- result{reserved, err}
		}(lease)
	}
	close(start)
	group.Wait()
	close(results)
	reservedCount := 0
	var winner DeliveryLease
	for result := range results {
		if result.err == nil {
			reservedCount++
			winner = result.lease
		} else if !errors.Is(result.err, ErrSubscriptionEmpty) {
			t.Fatalf("unexpected reserve competition error: %v", result.err)
		}
	}
	if reservedCount != 1 {
		t.Fatalf("reserved quota %d times", reservedCount)
	}
	var current Subscription
	if err := db.First(&current, subscription.ID).Error; err != nil || current.RemainingQuota != 0 || current.ReservedQuota != 1 {
		t.Fatal("quota not exclusively occupied", err)
	}
	next := time.Now().UTC().Add(-time.Millisecond)
	if err := repo.FinishSubscribe(context.Background(), winner, SubscribeOutcome{Status: DeliveryRetryWait, Code: "provider_not_sent", NextAttemptAt: &next, QuotaAction: "release"}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := repo.FinishSubscribe(context.Background(), winner, SubscribeOutcome{Status: DeliveryRetryWait, Code: "provider_not_sent", NextAttemptAt: &next, QuotaAction: "release"}, time.Now().UTC()); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatal("stale finish released twice", err)
	}
	if err := db.First(&current, subscription.ID).Error; err != nil || current.RemainingQuota != 1 || current.ReservedQuota != 0 {
		t.Fatal("quota release not conserved", err)
	}
	retried := isolatedNotificationClaim(t, repo)
	reserved, err := repo.ReserveForDelivery(context.Background(), retried, "isolated-app", "isolated-openid", "isolated-template", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.FinishSubscribe(context.Background(), reserved, SubscribeOutcome{Status: DeliveryAccepted, QuotaAction: "consume"}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&current, subscription.ID).Error; err != nil || current.Status != "exhausted" || current.RemainingQuota != 0 || current.ReservedQuota != 0 {
		t.Fatal("accepted send did not consume quota exactly once", err)
	}
	var accepted DeliveryItem
	if err := db.First(&accepted, reserved.Item.ID).Error; err != nil || accepted.Status != DeliveryAccepted || accepted.LeaseToken != nil || accepted.LeaseExpiresAt != nil || accepted.ProcessedAt == nil {
		t.Fatal("accepted send did not persist its terminal state and clear lease", err)
	}
}

func TestNotificationExpiredExternalLeaseUnknownAgainstIsolatedMySQL(t *testing.T) {
	db := openIsolatedNotificationMySQL(t)
	repo := NewRepository(db)
	subscription := isolatedNotificationSubscription(t, db)
	item := isolatedNotificationDelivery(t, db)
	lease := isolatedNotificationClaim(t, repo)
	reserved, err := repo.ReserveForDelivery(context.Background(), lease, "isolated-app", "isolated-openid", "isolated-template", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&DeliveryItem{}).Where("id=?", item.ID).Update("lease_expires_at", time.Now().UTC().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, found, err := repo.ClaimWechatDelivery(context.Background(), time.Now().UTC(), time.Minute); err != nil || found {
			t.Fatalf("unknown external request was reclaimed found=%v err=%v", found, err)
		}
	}
	var current DeliveryItem
	if err := db.First(&current, item.ID).Error; err != nil || current.Status != DeliveryUnknown || current.AttemptCount != 1 || current.LastErrorCode != "lease_expired_unknown" {
		t.Fatal("expired lease not terminal unknown", err)
	}
	var quota Subscription
	if err := db.First(&quota, subscription.ID).Error; err != nil || quota.Status != "exhausted" || quota.RemainingQuota != 0 || quota.ReservedQuota != 0 {
		t.Fatal("unknown send restored quota", err)
	}
	if err := repo.FinishSubscribe(context.Background(), reserved, SubscribeOutcome{Status: DeliveryBlocked, QuotaAction: "release"}, time.Now().UTC()); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatal("stale worker released unknown consent", err)
	}
}

func TestNotificationFinishAtomicRollbackAgainstIsolatedMySQL(t *testing.T) {
	db := openIsolatedNotificationMySQL(t)
	repo := NewRepository(db)
	subscription := isolatedNotificationSubscription(t, db)
	item := isolatedNotificationDelivery(t, db)
	lease := isolatedNotificationClaim(t, repo)
	reserved, err := repo.ReserveForDelivery(context.Background(), lease, "isolated-app", "isolated-openid", "isolated-template", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	// A real server-side write failure after quota update must roll back both.
	if err := db.Exec("CREATE TRIGGER notification_test_finish_failure BEFORE UPDATE ON delivery_notification_deliveries FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='isolated injected failure'").Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.FinishSubscribe(context.Background(), reserved, SubscribeOutcome{Status: DeliveryAccepted, QuotaAction: "consume"}, time.Now().UTC()); err == nil {
		t.Fatal("injected database failure did not fire")
	}
	var quota Subscription
	var current DeliveryItem
	if err := db.First(&quota, subscription.ID).Error; err != nil || quota.Status != "accepted" || quota.ReservedQuota != 1 || quota.RemainingQuota != 0 {
		t.Fatal("failed finish changed quota", err)
	}
	if err := db.First(&current, item.ID).Error; err != nil || current.Status != DeliveryProcessing || current.RowVersion != reserved.Item.RowVersion {
		t.Fatal("failed finish changed delivery", err)
	}
	if err := db.Exec("DROP TRIGGER notification_test_finish_failure").Error; err != nil {
		t.Fatal(err)
	}
	if err := repo.FinishSubscribe(context.Background(), reserved, SubscribeOutcome{Status: DeliveryUnknown, QuotaAction: "consume"}, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	var completed DeliveryItem
	if err := db.First(&completed, item.ID).Error; err != nil || completed.Status != DeliveryUnknown || completed.LeaseToken != nil || completed.LeaseExpiresAt != nil || completed.ProcessedAt == nil {
		t.Fatal("explicit unknown finish did not persist terminal state and clear lease", err)
	}
	var consumed Subscription
	if err := db.First(&consumed, subscription.ID).Error; err != nil || consumed.Status != "exhausted" || consumed.RemainingQuota != 0 || consumed.ReservedQuota != 0 || consumed.ReservationNonce != nil || consumed.ReservedAt != nil {
		t.Fatal("explicit unknown finish did not consume and clear reserved quota", err)
	}
	// Test beyond the original lease deadline: a still-processing row would
	// otherwise look unclaimable merely because its current lease is live.
	afterLease := reserved.Item.LeaseExpiresAt.Add(time.Second)
	if _, found, err := repo.ClaimWechatDelivery(context.Background(), afterLease, time.Minute); err != nil || found {
		t.Fatal("committed unknown was retried", err)
	}
	var afterClaim DeliveryItem
	if err := db.First(&afterClaim, item.ID).Error; err != nil || afterClaim.Status != DeliveryUnknown || afterClaim.RowVersion != completed.RowVersion || afterClaim.AttemptCount != completed.AttemptCount || afterClaim.LeaseToken != nil || afterClaim.LeaseExpiresAt != nil {
		t.Fatal("claim after former lease expiry changed terminal unknown delivery", err)
	}
	var quotaAfterClaim Subscription
	if err := db.First(&quotaAfterClaim, subscription.ID).Error; err != nil || quotaAfterClaim.Status != "exhausted" || quotaAfterClaim.RemainingQuota != 0 || quotaAfterClaim.ReservedQuota != 0 || quotaAfterClaim.RowVersion != consumed.RowVersion {
		t.Fatal("claim after former lease expiry changed consumed unknown quota", err)
	}
}
