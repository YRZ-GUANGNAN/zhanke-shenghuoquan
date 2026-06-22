package media

import (
	"os"
	"strings"
	"testing"
)

func TestMediaGatewayProductionBoundaryIsReadOnly(t *testing.T) {
	files := []string{"gateway_types.go", "gateway_repository.go", "gateway_service.go"}
	var source strings.Builder
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		source.Write(raw)
	}
	text := source.String()
	for _, forbidden := range []string{
		".Create(", ".Save(", ".Update(", ".Updates(", ".Delete(", ".Exec(", ".Transaction(",
		"clause.Locking", "UploadTicket", "QuotaReservation", "UploadCompletion", "OutboxEvent",
		"tencentyun", "tencentcloud", "net/http", "gin-gonic", "CopyObject", "DeleteObject", "PutObject",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("MediaGateway boundary contains forbidden capability %q", forbidden)
		}
	}
	if !strings.Contains(text, "MediaGatewayReadRepository") || !strings.Contains(text, "Scan(") {
		t.Fatal("MediaGateway does not use its read-only Repository boundary")
	}
}

func TestMediaGatewaySnapshotHasNoStorageIdentityFields(t *testing.T) {
	raw, err := os.ReadFile("gateway_types.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, forbidden := range []string{"StagingKey", "StagingVersion", "FinalKey", "FinalVersion", "SecretID", "SecretKey", "TicketID"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("Gateway read model exposes forbidden field %q", forbidden)
		}
	}
}
