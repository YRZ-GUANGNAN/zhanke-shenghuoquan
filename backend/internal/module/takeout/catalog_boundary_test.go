package takeout

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestG2CatalogBoundaryAndIntegerMoney(t *testing.T) {
	files, err := filepath.Glob("catalog_*.go")
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
		source := string(body)
		for _, forbidden := range []string{
			"github.com/gin-gonic/gin", "net/http", "internal/module/media", "internal/module/user",
			"internal/module/community", "internal/module/trade", "internal/module/navigation",
			"internal/module/im", "internal/module/admin", "float32", "float64", "decimal",
		} {
			if strings.Contains(strings.ToLower(source), strings.ToLower(forbidden)) {
				t.Errorf("%s contains forbidden dependency/type %q", path, forbidden)
			}
		}
	}

	migration, err := os.ReadFile("../../../migrations/017_takeout_catalog.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(string(migration), "")
	if regexp.MustCompile(`(?i)\bFOREIGN\s+KEY\b|\bREFERENCES\b`).MatchString(sql) {
		t.Fatal("migration 017 contains a foreign key")
	}
	if !regexp.MustCompile(`(?i)price_cents\s+BIGINT\s+UNSIGNED\s+NOT\s+NULL`).MatchString(sql) {
		t.Fatal("price_cents is not BIGINT UNSIGNED NOT NULL")
	}
	if regexp.MustCompile(`(?i)\b(FLOAT|DOUBLE|DECIMAL)\b`).MatchString(sql) {
		t.Fatal("migration 017 contains non-integer money storage")
	}
	if _, err := os.Stat("../../../migrations/rollback/017_takeout_catalog.sql"); err != nil {
		t.Fatal(err)
	}
}
