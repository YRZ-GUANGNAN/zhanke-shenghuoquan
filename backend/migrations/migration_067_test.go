package migrations

import (
	"os"
	"strings"
	"testing"
)

const migration067File = "067_feature_suggestion_comments.sql"
const migration067RollbackFile = "rollback/067_feature_suggestion_comments.sql"

func TestMigration067DefinesSuggestionComments(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration067File)
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, token := range []string{
		"CREATE TABLE IF NOT EXISTS feature_suggestion_comments",
		"UNIQUE KEY uq_feature_comment_public_id (public_id)",
		"KEY idx_feature_comment_suggestion (suggestion_id, id)",
		"is_maintainer TINYINT(1) NOT NULL DEFAULT 0",
		"content VARCHAR(300) NOT NULL",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 067 is missing %q", token)
		}
	}
}

func TestMigration067ShipsRollback(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration067RollbackFile)
	if err != nil {
		t.Fatalf("migration 067 must ship a rollback script: %v", err)
	}
	if !strings.Contains(string(sourceBytes), "DROP TABLE IF EXISTS feature_suggestion_comments") {
		t.Fatalf("rollback 067 is missing the drop statement")
	}
}
