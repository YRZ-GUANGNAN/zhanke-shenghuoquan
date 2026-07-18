package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestMigration033DefinesUserSettingsFeedbackAndClosure(t *testing.T) {
	content, err := os.ReadFile("033_user_settings_feedback_account_closure.sql")
	if err != nil {
		t.Fatal(err)
	}
	source := string(content)
	for _, required := range []string{
		"ADD COLUMN phone VARCHAR(32) NULL",
		"ADD COLUMN closed_at DATETIME(3) NULL",
		"CREATE TABLE user_settings",
		"CREATE TABLE user_blocks",
		"CREATE TABLE user_feedbacks",
		"CREATE TABLE user_feedback_images",
		"UNIQUE KEY uq_users_phone",
		"UNIQUE KEY uq_user_settings_user",
		"PRIMARY KEY (user_id, blocked_user_id)",
		"UNIQUE KEY uq_user_feedback_public_id",
		"UNIQUE KEY uq_user_feedback_image_slot",
		"'feedback'",
		"DROP CHECK chk_media_reference_type",
	} {
		if !strings.Contains(source, required) {
			t.Errorf("migration 033 missing %q", required)
		}
	}
	guard := strings.Index(source, "chk_media_reference_type_feedback_guard")
	drop := strings.Index(source, "DROP CHECK chk_media_reference_type")
	if guard < 0 || drop < 0 || guard > drop {
		t.Fatal("feedback media guard must be installed before replacing the canonical constraint")
	}
	if _, err := os.Stat("rollback/033_user_settings_feedback_account_closure.sql"); !os.IsNotExist(err) {
		t.Fatal("migration 033 must be forward-only")
	}
}
