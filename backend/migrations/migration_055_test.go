package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestMigration055IsAdditiveAndRollbackPreservesDutyAndConfiguration(t *testing.T) {
	body, err := os.ReadFile("055_delivery_rider_availability.sql")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, required := range []string{"delivery_rider_availability", "delivery_rider_broadcast_config", "UNIQUE INDEX uq_delivery_rider_availability_rider", "CHECK (state IN ('off_duty', 'on_duty'))", "BETWEEN 1 AND 100", "VALUES ('global', 3, 1"} {
		if !strings.Contains(source, required) {
			t.Fatalf("missing migration clause %q", required)
		}
	}
	if strings.Contains(source, "ALTER TABLE delivery_orders") || strings.Contains(source, "UPDATE delivery_rider_wallet") {
		t.Fatal("migration modifies order or money facts")
	}
	body, err = os.ReadFile("rollback/055_delivery_rider_availability.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	source = string(body)
	for _, required := range []string{"row_count > 0", "max_in_flight_per_rider <> 3", "row_version <> 1", "updated_at <> created_at", "SIGNAL SQLSTATE '45000'"} {
		if !strings.Contains(source, required) {
			t.Fatalf("rollback does not protect %q", required)
		}
	}
}
