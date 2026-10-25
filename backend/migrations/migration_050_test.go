package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const migration050File = "050_delivery_rider_payout_queue.sql"

func readMigration050(t *testing.T, parts ...string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestMigration050CreatesQueueReservationsAndEventsWithoutBackfill(t *testing.T) {
	source := readMigration050(t, migration050File)
	for _, token := range []string{
		"CREATE TABLE delivery_rider_payout_queue",
		"payout_queue_public_id CHAR(26) NOT NULL",
		"UNIQUE INDEX uq_delivery_rider_payout_queue_earning",
		"status IN ('blocked', 'queued', 'cancelled')",
		"CREATE TABLE delivery_rider_payout_reservations",
		"UNIQUE INDEX uq_delivery_rider_payout_reservation_earning",
		"status IN ('active', 'released')",
		"CREATE TABLE delivery_rider_payout_events",
		"UNIQUE INDEX uq_delivery_rider_payout_event_idempotency",
		"event_type IN ('blocked', 'queued', 'cancelled', 'source_invalidated')",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 050 missing %q", token)
		}
	}
	upper := strings.ToUpper(source)
	for _, forbidden := range []string{
		"INSERT INTO DELIVERY_RIDER_PAYOUT",
		"UPDATE DELIVERY_RIDER_WALLETS",
		"UPDATE DELIVERY_RIDER_EARNING_RECORDS",
		"FOREIGN KEY",
	} {
		if strings.Contains(upper, forbidden) {
			t.Fatalf("migration 050 must not infer historical payout facts: %q", forbidden)
		}
	}
}

func TestMigration050RollbackRequiresEmptyTablesAndDropsInDependencyOrder(t *testing.T) {
	source := readMigration050(t, filepath.Join("rollback", migration050File))
	for _, token := range []string{
		"SELECT COUNT(*) INTO row_count FROM delivery_rider_payout_events",
		"SELECT COUNT(*) INTO row_count FROM delivery_rider_payout_reservations",
		"SELECT COUNT(*) INTO row_count FROM delivery_rider_payout_queue",
		"SIGNAL SQLSTATE '45000'",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("rollback 050 missing protection %q", token)
		}
	}
	events := strings.Index(source, "DROP TABLE IF EXISTS delivery_rider_payout_events")
	reservations := strings.Index(source, "DROP TABLE IF EXISTS delivery_rider_payout_reservations")
	queue := strings.Index(source, "DROP TABLE IF EXISTS delivery_rider_payout_queue")
	if events < 0 || reservations < 0 || queue < 0 || events >= reservations || reservations >= queue {
		t.Fatal("rollback 050 must drop events, reservations, then queue")
	}
}
