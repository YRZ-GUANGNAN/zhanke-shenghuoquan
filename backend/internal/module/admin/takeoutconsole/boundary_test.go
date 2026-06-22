package takeoutconsole

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTakeoutConsoleBoundaryAndRouteContract(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var production strings.Builder
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		production.Write(body)
	}
	source := production.String()
	for _, forbidden := range []string{
		"internal/module/admin/review", "internal/module/community", "internal/module/trade",
		"internal/module/media", "gorm.io/", "*gorm.DB",
	} {
		if strings.Contains(source, forbidden) {
			t.Errorf("takeout console contains forbidden dependency %q", forbidden)
		}
	}
	for _, route := range []string{
		`"/merchants"`, `"/categories"`, `"/dishes"`, `"/orders"`, `"/riders"`,
		`"/fee-config"`, `"/rider-config"`, `"/settlement-report"`, `"/withdraw-requests"`,
	} {
		if !strings.Contains(source, route) {
			t.Errorf("takeout console missing route %s", route)
		}
	}
}
