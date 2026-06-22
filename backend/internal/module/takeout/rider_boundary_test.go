package takeout

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestG5RiderBoundaryAndConfigGovernance(t *testing.T) {
	files, err := filepath.Glob("rider_*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		source := strings.ToLower(string(body))
		for _, forbidden := range []string{
			"github.com/gin-gonic/gin", "net/http", "internal/module/media", "internal/module/user",
			"internal/module/community", "internal/module/trade", "internal/module/navigation",
			"internal/module/im", "internal/module/admin", "float32", "float64", "decimal",
		} {
			if strings.Contains(source, strings.ToLower(forbidden)) {
				t.Errorf("%s contains forbidden dependency/type %q", path, forbidden)
			}
		}
	}

	migration, err := os.ReadFile("../../../migrations/020_takeout_rider.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(string(migration), "")
	if regexp.MustCompile(`(?i)\bFOREIGN\s+KEY\b|\bREFERENCES\b`).MatchString(sql) {
		t.Fatal("migration 020 contains a foreign key")
	}
	if regexp.MustCompile(`(?i)\b(FLOAT|DOUBLE|DECIMAL)\b`).MatchString(sql) {
		t.Fatal("migration 020 contains money-like non-integer storage")
	}
	for _, required := range []string{
		"max_in_flight_per_rider INT UNSIGNED NOT NULL",
		"VALUES ('global', 1000, 1, UTC_TIMESTAMP())",
		"idx_takeout_fulfillment_rider_status (rider_user_id, status)",
		"uq_takeout_fulfillment_order (order_public_id)",
	} {
		if !strings.Contains(sql, required) {
			t.Errorf("migration 020 missing %q", required)
		}
	}
	if _, err := os.Stat("../../../migrations/rollback/020_takeout_rider.sql"); err != nil {
		t.Fatal(err)
	}
}
