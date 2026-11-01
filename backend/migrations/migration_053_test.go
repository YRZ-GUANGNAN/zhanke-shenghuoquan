package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestMigration053RepeatableAndGuardedRollback(t *testing.T) {
	raw, err := os.ReadFile("053_delivery_notifications.sql")
	if err != nil { t.Fatal(err) }
	source := string(raw)
	if strings.Count(source, "CREATE TABLE IF NOT EXISTS ") != 5 { t.Fatal("migration 053 must create five repeatable tables") }
	for _, token := range []string{"uq_delivery_notification_event_version", "uq_delivery_notification_delivery_coordinate", "idx_delivery_notification_delivery_claim", "CHECK (status IN", "DEFAULT CHARSET=utf8mb4"} {
		if !strings.Contains(source, token) { t.Fatalf("migration 053 missing %q", token) }
	}
	raw, err = os.ReadFile("rollback/053_delivery_notifications.sql")
	if err != nil { t.Fatal(err) }
	rollback := string(raw)
	previous := -1
	for _, table := range []string{"im_notification_receipts", "notification_subscriptions", "delivery_notification_deliveries", "delivery_notification_inbox", "delivery_notification_events"} {
		if !strings.Contains(rollback, "SELECT COUNT(*) INTO row_count FROM "+table) { t.Fatalf("rollback must preserve nonempty %s", table) }
		position := strings.Index(rollback, "DROP TABLE IF EXISTS "+table)
		if position <= previous { t.Fatalf("rollback order invalid for %s", table) }
		previous = position
	}
	if !strings.Contains(rollback, "SIGNAL SQLSTATE '45000'") { t.Fatal("rollback must refuse to delete durable facts") }
}
