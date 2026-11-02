package migrations

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMigration054GuardsLegacyMoneyAndDocumentsUTCBoundary(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	base := filepath.Dir(file)
	sourceBytes, err := os.ReadFile(filepath.Join(base, "054_delivery_rider_daily_settlement.sql"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	t.Logf("migration_path=%s bytes=%d", filepath.Join(base, "054_delivery_rider_daily_settlement.sql"), len(source))
	for _, required := range []string{
		"rider_daily_settlement_migration_guard",
		"delivery_rider_ledger",
		"l.idempotency_key = e.delivery_order_public_id",
		"settled_at",
		"idx_delivery_rider_earning_due",
		"All DATETIME values are UTC",
		"Asia/Shanghai midnight",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("migration 054 missing %q", required)
		}
	}
	if strings.Contains(source, "available_balance_cents +") {
		t.Fatal("migration 054 must not credit an existing wallet")
	}
	downBytes, err := os.ReadFile(filepath.Join(base, "rollback", "054_delivery_rider_daily_settlement.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down := string(downBytes)
	if !strings.Contains(down, "delivery_rider_earning_records") || !strings.Contains(down, "valid TINYINT") {
		t.Fatal("rollback 054 must refuse to remove persisted earning facts")
	}
}
