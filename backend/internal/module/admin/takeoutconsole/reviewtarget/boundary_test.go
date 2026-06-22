package reviewtarget

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewTargetBoundary(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(body)
		for _, forbidden := range []string{"github.com/gin-gonic/gin", "net/http", "gorm.io/", "*gorm.DB"} {
			if strings.Contains(source, forbidden) {
				t.Errorf("%s contains forbidden dependency %q", file, forbidden)
			}
		}
	}
}
