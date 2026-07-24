package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const migration035File = "035_admin_password_login.sql"

func TestMigration035DefinesAdminAuthenticationTables(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration035File)
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, token := range []string{
		"CREATE TABLE admin_credentials",
		"CREATE TABLE admin_web_sessions",
		"CREATE TABLE admin_auth_audit_logs",
		"uq_admin_credentials_username",
		"uq_admin_credentials_user_id",
		"token_hash BINARY(32)",
		"csrf_hash BINARY(32)",
		"idle_expires_at DATETIME(3)",
		"absolute_expires_at DATETIME(3)",
		"password_changed_at DATETIME(3)",
		"failed_attempts SMALLINT UNSIGNED",
		"locked_until DATETIME(3)",
		"FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT",
		"event_type IN ('login','logout','password_changed','credential_bootstrap','credential_reset','account_locked','sessions_revoked')",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 035 missing %q", token)
		}
	}
	if _, err := os.Stat(filepath.Join("rollback", migration035File)); !os.IsNotExist(err) {
		t.Fatal("migration 035 must remain forward-only")
	}
}

func TestMigration035IsGuardedForRepeatApplication(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration035File)
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, token := range []string{
		"CREATE PROCEDURE migrate_admin_password_login",
		"GET_LOCK('campus_circle:migration:035'",
		"IF NOT EXISTS",
		"information_schema.tables",
		"information_schema.columns",
		"CALL migrate_admin_password_login()",
		"DROP PROCEDURE migrate_admin_password_login",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 035 repeat-application guard missing %q", token)
		}
	}
	for _, forbidden := range []string{
		"INSERT INTO users",
		"UPDATE users SET role",
		"UPDATE identity_applications",
		"DELETE FROM identity_applications",
	} {
		if strings.Contains(strings.ToUpper(source), strings.ToUpper(forbidden)) {
			t.Fatalf("migration 035 must not mutate business identity state: %q", forbidden)
		}
	}
}
