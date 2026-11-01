package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestMigration052CancellationIntentAndGuardedRollback(t *testing.T) {
	forward, err := os.ReadFile("052_delivery_recharge_cancellation.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(forward), "cancel_requested_at DATETIME(3) NULL") || strings.Contains(strings.ToUpper(string(forward)), "UPDATE ") {
		t.Fatal("must add nullable intent without historical backfill")
	}
	rollback, err := os.ReadFile("rollback/052_delivery_recharge_cancellation.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"cancel_requested_at IS NOT NULL", "status IN ('created', 'prepay_issued')", "SIGNAL SQLSTATE '45000'", "DROP COLUMN cancel_requested_at"} {
		if !strings.Contains(string(rollback), token) {
			t.Fatalf("missing rollback guard %q", token)
		}
	}
	if strings.Contains(string(rollback), "DROP TABLE") {
		t.Fatal("rollback must not remove payment facts")
	}
}
