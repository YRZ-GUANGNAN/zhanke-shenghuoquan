package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMediaGCBoundary(t *testing.T) {
	for _, name := range []string{"gc_types.go", "gc_state_machine.go", "gc_repository.go", "gc_service.go", "gc_cos.go"} {
		raw, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		content := strings.ToLower(string(raw))
		for _, forbidden := range []string{"github.com/gin-gonic", "internal/module/community", "internal/module/trade", "internal/module/user"} {
			if strings.Contains(content, forbidden) {
				t.Fatalf("%s crosses frozen boundary via %s", name, forbidden)
			}
		}
	}
	raw, err := os.ReadFile("gc_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "ActiveRefCount") || strings.Contains(string(raw), "active_ref_count") {
		t.Fatal("GC deletion decisions must not use active_ref_count")
	}
	cosRaw, err := os.ReadFile("gc_cos.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cosRaw), "ObjectDeleteOptions{VersionId: versionID}") {
		t.Fatal("COS deletion is not pinned to versionId")
	}
}
