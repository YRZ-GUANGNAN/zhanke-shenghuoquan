package migrations

import (
	"os"
	"strings"
	"testing"
)

const migration063File = "063_student_verification.sql"
const migration063RollbackFile = "rollback/063_student_verification.sql"

func TestMigration063DefinesStudentVerificationTables(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration063File)
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, token := range []string{
		"CREATE TABLE IF NOT EXISTS student_verifications",
		"CREATE TABLE IF NOT EXISTS student_verification_assets",
		"CREATE TABLE IF NOT EXISTS student_verification_events",
		"UNIQUE KEY uq_student_verification_public_id",
		"UNIQUE KEY uq_student_verification_user",
		"CHECK (status IN ('pending','approved','rejected','cancelled'))",
		"CHECK (event_type IN ('submitted','resubmitted','approved','rejected','cancel_requested','cancel_approved'))",
		"'profile_cover'",
		"'student_verification'",
		"chk_media_asset_purpose",
		"chk_media_reference_type",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 063 is missing %q", token)
		}
	}
}

func TestMigration063ShipsRollback(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration063RollbackFile)
	if err != nil {
		t.Fatalf("migration 063 must ship a rollback script: %v", err)
	}
	source := string(sourceBytes)
	for _, token := range []string{
		"DROP TABLE IF EXISTS student_verification_events",
		"DROP TABLE IF EXISTS student_verification_assets",
		"DROP TABLE IF EXISTS student_verifications",
		"chk_media_asset_purpose",
		"chk_media_reference_type",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 063 rollback is missing %q", token)
		}
	}
	if strings.Contains(source, "'student_verification'") {
		t.Fatal("migration 063 rollback must restore the previous media purpose allowlist")
	}
}
