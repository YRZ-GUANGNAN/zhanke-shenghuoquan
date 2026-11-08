package migrations

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMigration056BindsWithdrawalsToEarningsAndGuardsRollback(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	base := filepath.Dir(file)

	upBytes, err := os.ReadFile(filepath.Join(base, "056_delivery_rider_withdrawal_allocations.sql"))
	if err != nil {
		t.Fatal(err)
	}
	up := string(upBytes)
	for _, required := range []string{
		"CREATE TABLE delivery_rider_withdrawal_allocations",
		"CREATE TABLE delivery_rider_withdrawal_events",
		"CREATE TABLE delivery_rider_withdrawal_claims",
		"UNIQUE INDEX uq_delivery_rider_withdrawal_allocation_pair",
		"UNIQUE INDEX uq_delivery_rider_withdrawal_event_idempotency",
		"UNIQUE INDEX uq_delivery_rider_withdrawal_claim_withdraw",
		"amount_cents > 0 AND amount_cents <= 9007199254740991",
		"status IN ('in_flight', 'paid', 'released')",
		"event_type IN ('claimed', 'released', 'paid', 'rejected', 'exception')",
		"status IN ('active', 'released', 'completed', 'exception')",
		"source_evidence_hash CHAR(64) NOT NULL",
		"operator_user_id > 0 AND expected_row_version > 0",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("migration 056 missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"UPDATE delivery_rider_wallet",
		"DELETE FROM delivery_rider_earning_records",
		"ALTER TABLE delivery_rider_earning_records",
	} {
		if strings.Contains(up, forbidden) {
			t.Fatalf("migration 056 must not mutate existing money facts with %q", forbidden)
		}
	}

	downBytes, err := os.ReadFile(filepath.Join(base, "rollback", "056_delivery_rider_withdrawal_allocations.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	down := string(downBytes)
	for _, table := range []string{
		"delivery_rider_withdrawal_allocations",
		"delivery_rider_withdrawal_events",
		"delivery_rider_withdrawal_claims",
	} {
		if !strings.Contains(down, "SELECT COUNT(*) INTO row_count FROM "+table) {
			t.Fatalf("rollback 056 does not preserve nonempty %s", table)
		}
		if !strings.Contains(down, "DROP TABLE IF EXISTS "+table) {
			t.Fatalf("rollback 056 does not remove empty %s", table)
		}
	}
	if strings.Count(down, "SIGNAL SQLSTATE '45000'") < 3 {
		t.Fatal("rollback 056 must refuse to delete every durable withdrawal fact class")
	}
}
