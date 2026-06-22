package takeout

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestG3OrderBoundaryAndIntegerMoney(t *testing.T) {
	files, err := filepath.Glob("order_*.go")
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

	migration, err := os.ReadFile("../../../migrations/019_takeout_order.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(string(migration), "")
	if regexp.MustCompile(`(?i)\bFOREIGN\s+KEY\b|\bREFERENCES\b`).MatchString(sql) {
		t.Fatal("migration 019 contains a foreign key")
	}
	for _, column := range []string{
		"items_amount_cents", "delivery_fee_cents", "total_amount_cents",
		"unit_price_cents_snapshot", "line_amount_cents", "rider_fee_cents", "platform_cut_cents",
	} {
		pattern := `(?i)` + regexp.QuoteMeta(column) + `\s+BIGINT\s+UNSIGNED\s+NOT\s+NULL`
		if !regexp.MustCompile(pattern).MatchString(sql) {
			t.Errorf("%s is not BIGINT UNSIGNED NOT NULL", column)
		}
	}
	if regexp.MustCompile(`(?i)\b(FLOAT|DOUBLE|DECIMAL)\b`).MatchString(sql) {
		t.Fatal("migration 019 contains non-integer money storage")
	}
	if _, err := os.Stat("../../../migrations/rollback/019_takeout_order.sql"); err != nil {
		t.Fatal(err)
	}
}
