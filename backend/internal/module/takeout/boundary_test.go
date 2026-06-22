package takeout

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestG1BoundaryHasNoHTTPOrCrossDomainImports(t *testing.T) {
	entries, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range entries {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		source := string(body)
		for _, forbidden := range []string{
			"github.com/gin-gonic/gin", "net/http", "internal/module/user", "internal/module/community",
			"internal/module/trade", "internal/module/navigation", "internal/module/im", "internal/module/admin",
			"internal/module/media",
		} {
			if strings.Contains(source, forbidden) {
				t.Errorf("%s contains forbidden dependency %q", path, forbidden)
			}
		}
		if filepath.Base(path) != "repository.go" && strings.Contains(source, "gorm.io/") {
			t.Errorf("%s bypasses the Repository database boundary", path)
		}
	}
}

func TestG1MigrationHasNoCrossDomainForeignKey(t *testing.T) {
	body, err := os.ReadFile("../../../migrations/016_takeout_merchant.sql")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if regexp.MustCompile(`(?i)\bFOREIGN\s+KEY\s*\(`).MatchString(source) ||
		regexp.MustCompile(`(?i)\bREFERENCES\s+(users|media_)`).MatchString(source) {
		t.Fatal("G-1 migration contains a cross-domain foreign key")
	}
}
