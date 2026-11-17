package migrations

import (
	"os"
	"strings"
	"testing"
)

const migration065File = "065_feature_board.sql"
const migration065RollbackFile = "rollback/065_feature_board.sql"

func TestMigration065DefinesFeatureBoardTables(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration065File)
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, token := range []string{
		"CREATE TABLE IF NOT EXISTS feature_suggestions",
		"CREATE TABLE IF NOT EXISTS feature_suggestion_supports",
		"CREATE TABLE IF NOT EXISTS author_notes",
		"UNIQUE KEY uq_feature_suggestion_public_id (public_id)",
		"UNIQUE KEY uq_feature_suggestion_support (suggestion_id, user_id)",
		"CHECK (status IN ('open','accepted','closed'))",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 065 is missing %q", token)
		}
	}
}

func TestMigration065ShipsRollback(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration065RollbackFile)
	if err != nil {
		t.Fatalf("migration 065 must ship a rollback script: %v", err)
	}
	source := string(sourceBytes)
	for _, token := range []string{
		"DROP TABLE IF EXISTS feature_suggestion_supports",
		"DROP TABLE IF EXISTS feature_suggestions",
		"DROP TABLE IF EXISTS author_notes",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("rollback 065 is missing %q", token)
		}
	}
}
