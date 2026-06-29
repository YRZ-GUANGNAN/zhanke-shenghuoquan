package delivery

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestD2DeliveryBoundaryAndMigration(t *testing.T) {
	for _, file := range []string{"dispatch_types.go", "dispatch_repository.go", "dispatch_service.go"} {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		source := strings.ToLower(string(body))
		for _, forbidden := range []string{
			"github.com/gin-gonic/gin", "net/http", "internal/module/takeout",
			"internal/module/user", "internal/module/admin", "internal/module/media",
			"internal/module/community", "internal/module/trade", "internal/module/im",
		} {
			if strings.Contains(source, forbidden) {
				t.Errorf("%s contains forbidden dependency %q", file, forbidden)
			}
		}
	}
	migration, err := os.ReadFile("../../../migrations/024_delivery_dispatch.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(string(migration), "")
	if regexp.MustCompile(`(?i)\bFOREIGN\s+KEY\b|\bREFERENCES\b`).MatchString(sql) {
		t.Fatal("migration 024 contains a foreign key")
	}
	for _, expected := range []string{
		"merchant_request_id", "delivery_outbox", "uq_delivery_order_merchant_request",
		"uq_delivery_outbox_aggregate_event", "idx_delivery_outbox_claim",
	} {
		if !strings.Contains(sql, expected) {
			t.Errorf("migration 024 missing %s", expected)
		}
	}
}
