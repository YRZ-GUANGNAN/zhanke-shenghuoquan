package migrations

import (
	"os"
	"strings"
	"testing"
)

const migration068File = "068_navigation_item_icon.sql"
const migration068RollbackFile = "rollback/068_navigation_item_icon.sql"

func TestMigration068AddsIconPathColumn(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration068File)
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, token := range []string{
		"COLUMN_NAME = 'icon_path'",
		"ADD COLUMN icon_path VARCHAR(255) NOT NULL DEFAULT ''",
		"PREPARE migrate_stmt",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 068 is missing %q", token)
		}
	}
}

func TestMigration068ShipsRollback(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration068RollbackFile)
	if err != nil {
		t.Fatalf("migration 068 must ship a rollback script: %v", err)
	}
	source := string(sourceBytes)
	for _, token := range []string{"DROP COLUMN icon_path", "PREPARE rollback_stmt"} {
		if !strings.Contains(source, token) {
			t.Fatalf("rollback 068 is missing %q", token)
		}
	}
}
