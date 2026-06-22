package mediaconsole

import (
	"os"
	"strings"
	"testing"
)

func TestMediaConsoleReadOnlyBoundary(t *testing.T) {
	repository, err := os.ReadFile("query_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(repository)
	for _, forbidden := range []string{".Create(", ".Save(", ".Update(", ".Updates(", ".Delete(", ".Exec("} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("read repository contains write primitive %s", forbidden)
		}
	}
	handler, err := os.ReadFile("handler.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{".POST(", ".PUT(", ".PATCH(", ".DELETE("} {
		if strings.Contains(string(handler), forbidden) {
			t.Fatalf("handler contains write route %s", forbidden)
		}
	}
	types, err := os.ReadFile("types.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"StorageKey", "VersionID", "ExpectedMD5", "TrustedSHA256", "LeaseToken", "Payload JSON"} {
		if strings.Contains(string(types), secret) {
			t.Fatalf("admin DTO exposes internal media field %s", secret)
		}
	}
}
