package media

import (
	"os"
	"strings"
	"testing"
)

func TestMediaConsumerBoundaryIsolation(t *testing.T) {
	productionFiles := []string{"consumer_types.go", "consumer_state_machine.go", "consumer_repository.go", "consumer_service.go"}
	var source strings.Builder
	for _, file := range productionFiles {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		source.Write(raw)
	}
	text := source.String()
	for _, forbidden := range []string{
		"net/http", "gin-gonic", "tencentyun", "tencentcloud", "CopyObject", "DeleteObject", "PutObject",
		"internal/module/community", "internal/module/trade", "internal/module/user", "MediaGateway", "GC",
		"media_assets", "media_review_projections",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("Module 7 contains forbidden capability %q", forbidden)
		}
	}
	service, err := os.ReadFile("consumer_service.go")
	if err != nil {
		t.Fatal(err)
	}
	serviceText := string(service)
	if !strings.Contains(serviceText, "ReviewProjectionCommands") || !strings.Contains(serviceText, "*ReviewProjectionService") {
		t.Fatal("Consumer is not wired exclusively through Module 5 service boundary")
	}
	for _, forbidden := range []string{"*gorm.DB", ".WithContext(", ".Model(", ".Exec("} {
		if strings.Contains(serviceText, forbidden) {
			t.Fatalf("Consumer service bypasses Repository with %q", forbidden)
		}
	}
}

func TestMediaConsumerRepositoryCASGuardsArePresent(t *testing.T) {
	raw, err := os.ReadFile("consumer_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, required := range []string{
		"event_id = ? AND status = ? AND row_version = ?",
		"aggregate_id = ? AND row_version = ? AND active_event_id = ? AND lease_token = ?",
		"event_id = ? AND row_version = ? AND processed_at IS NULL",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("required expected status/version CAS guard missing: %s", required)
		}
	}
}
