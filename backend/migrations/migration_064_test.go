package migrations

import (
	"os"
	"strings"
	"testing"
)

const migration064File = "064_campus_maker_category.sql"
const migration064RollbackFile = "rollback/064_campus_maker_category.sql"

func TestMigration064SeedsCampusMakerCategory(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration064File)
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, token := range []string{
		"INSERT INTO navigation_categories (name, system_key, sort, enabled, created_at, updated_at)",
		"'众创'",
		"'campus_maker'",
		"WHERE NOT EXISTS (SELECT 1 FROM navigation_categories WHERE system_key = 'campus_maker')",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 064 is missing %q", token)
		}
	}
}

func TestMigration064ShipsRollback(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration064RollbackFile)
	if err != nil {
		t.Fatalf("migration 064 must ship a rollback script: %v", err)
	}
	source := string(sourceBytes)
	for _, token := range []string{
		"DELETE FROM navigation_items WHERE category_id IN (SELECT id FROM navigation_categories WHERE system_key = 'campus_maker')",
		"DELETE FROM navigation_categories WHERE system_key = 'campus_maker'",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("rollback 064 is missing %q", token)
		}
	}
}
