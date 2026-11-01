package notification

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestMigration053NotificationSchemaContract(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "053_delivery_notifications.sql"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, token := range []string{
		"CREATE TABLE IF NOT EXISTS delivery_notification_events", "CREATE TABLE IF NOT EXISTS delivery_notification_inbox",
		"CREATE TABLE IF NOT EXISTS delivery_notification_deliveries", "CREATE TABLE IF NOT EXISTS notification_subscriptions", "CREATE TABLE IF NOT EXISTS im_notification_receipts",
		"uq_delivery_notification_event_version", "uq_delivery_notification_delivery_coordinate", "idx_delivery_notification_delivery_claim",
		"idx_delivery_notification_delivery_recipient", "idx_delivery_notification_inbox_recipient", "idx_notification_subscription_available",
		"chk_delivery_notification_delivery_status", "chk_notification_subscription_status", "utf8mb4",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 053 missing %q", token)
		}
	}
	for _, token := range []string{"message_content", "message_body", "raw_body", "private_message", "FOREIGN KEY", "INSERT INTO", "UPDATE delivery_"} {
		if strings.Contains(source, token) {
			t.Fatalf("migration 053 contains forbidden %q", token)
		}
	}
	// Only the authorization table may contain provider identity; display/event
	// tables hold public coordinates and safe summaries only.
	for _, table := range []string{"delivery_notification_events", "delivery_notification_inbox", "delivery_notification_deliveries", "im_notification_receipts"} {
		block := regexp.MustCompile(`(?s)CREATE TABLE IF NOT EXISTS ` + table + ` \(.*?\) ENGINE=`).FindString(source)
		if block == "" || strings.Contains(strings.ToLower(block), "openid") {
			t.Fatalf("unsafe or absent table %s", table)
		}
	}
}
