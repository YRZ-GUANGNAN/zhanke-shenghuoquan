package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const migration049File = "049_delivery_funding_sources.sql"

func readMigration049(t *testing.T, parts ...string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestMigration049AddsFundingLotsAndAllocationsWithoutHistoricalBackfill(t *testing.T) {
	source := readMigration049(t, migration049File)
	for _, token := range []string{
		"CREATE TABLE delivery_funding_lots",
		"funding_lot_public_id CHAR(26) NOT NULL",
		"source VARCHAR(32) NOT NULL",
		"verification_status VARCHAR(24) NOT NULL",
		"payment_public_id CHAR(26) NULL",
		"total_amount_cents BIGINT UNSIGNED NOT NULL",
		"allocated_amount_cents BIGINT UNSIGNED NOT NULL DEFAULT 0",
		"refunded_amount_cents BIGINT UNSIGNED NOT NULL DEFAULT 0",
		"uq_delivery_funding_lot_payment",
		"idx_delivery_funding_lot_wallet_fifo",
		"chk_delivery_funding_lot_source",
		"chk_delivery_funding_lot_amounts",
		"CREATE TABLE delivery_funding_allocations",
		"settlement_public_id CHAR(26) NOT NULL",
		"funding_lot_public_id CHAR(26) NOT NULL",
		"uq_delivery_funding_allocation_settlement_lot",
		"chk_delivery_funding_allocation_amounts",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 049 missing %q", token)
		}
	}
	upper := strings.ToUpper(source)
	for _, forbidden := range []string{
		"INSERT INTO DELIVERY_FUNDING_LOTS",
		"UPDATE DELIVERY_MERCHANT_WALLETS",
		"UPDATE DELIVERY_PAYMENT_ATTEMPTS",
		"FOREIGN KEY",
	} {
		if strings.Contains(upper, forbidden) {
			t.Fatalf("migration 049 must not infer or couple historical funding: %q", forbidden)
		}
	}
}

func TestMigration049RollbackOnlyDropsFundingTablesInDependencyOrder(t *testing.T) {
	source := readMigration049(t, filepath.Join("rollback", migration049File))
	allocations := strings.Index(source, "DROP TABLE IF EXISTS delivery_funding_allocations")
	lots := strings.Index(source, "DROP TABLE IF EXISTS delivery_funding_lots")
	if allocations < 0 || lots < 0 || allocations >= lots {
		t.Fatal("rollback 049 must drop allocations before lots")
	}
	for _, forbidden := range []string{
		"delivery_merchant_wallets",
		"delivery_wallet_ledger",
		"delivery_payment_attempts",
		"delivery_fee_settlements",
		"delivery_refunds",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("rollback 049 exceeds funding-source scope: %q", forbidden)
		}
	}
}
