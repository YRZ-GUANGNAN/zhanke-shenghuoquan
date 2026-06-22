package miniappflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModule10Boundary(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Clean(entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		source := string(raw)
		for _, forbidden := range []string{
			"internal/module/community", "internal/module/trade", "internal/module/user\"",
			"internal/pkg/media", "cos-go-sdk", "tencentcloud-sdk", "CopyObject", "DeleteObject",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains forbidden dependency %q", entry.Name(), forbidden)
			}
		}
	}
	if _, err := os.Stat("../../../../migrations/015_media_miniapp.sql"); !os.IsNotExist(err) {
		t.Fatal("Module 10 must not add a migration")
	}
	mainSource, err := os.ReadFile("../../../../cmd/server/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mainSource), `api.Group("/miniapp/media", userService.AuthRequired(), user.UnmutedRequired())`) {
		t.Fatal("Module 10 routes are not protected by frozen authentication and muted-user middleware")
	}
}
