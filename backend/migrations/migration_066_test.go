package migrations

import (
	"os"
	"strings"
	"testing"
)

const migration066File = "066_contact_settings.sql"
const migration066RollbackFile = "rollback/066_contact_settings.sql"

func TestMigration066DefinesContactSettings(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration066File)
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, token := range []string{
		"CREATE TABLE IF NOT EXISTS contact_settings",
		"maker_wechat VARCHAR(64) NOT NULL DEFAULT ''",
		"maker_intro VARCHAR(500) NOT NULL DEFAULT ''",
		"CHECK (id = 1)",
		"WHERE NOT EXISTS (SELECT 1 FROM contact_settings WHERE id = 1)",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 066 is missing %q", token)
		}
	}
}

func TestMigration066ShipsRollback(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration066RollbackFile)
	if err != nil {
		t.Fatalf("migration 066 must ship a rollback script: %v", err)
	}
	if !strings.Contains(string(sourceBytes), "DROP TABLE IF EXISTS contact_settings") {
		t.Fatalf("rollback 066 is missing the drop statement")
	}
}
