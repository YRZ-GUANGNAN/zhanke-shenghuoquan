package deliverypayment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeliveryPaymentTransportDoesNotImportFrozenDomains(t *testing.T) {
	forbidden := []string{
		`internal/module/takeout`, `internal/module/community`, `internal/module/trade`,
		`internal/module/media`, `internal/module/im`,
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range forbidden {
			if strings.Contains(string(raw), value) {
				t.Fatalf("%s imports forbidden domain %s", file, value)
			}
		}
	}
}
