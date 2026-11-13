package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestMigration057OfficialAccountNotificationContract(t *testing.T) {
	up, err := os.ReadFile("057_notification_official_account.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("rollback/057_notification_official_account.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"wechat_official",
		"notification_official_account_bindings",
		"chk_notification_official_follow_state",
		"uq_notification_official_openid",
	} {
		if !strings.Contains(string(up), required) {
			t.Fatalf("migration 057 missing %q", required)
		}
	}
	if !strings.Contains(string(down), "DROP TABLE IF EXISTS notification_official_account_bindings") ||
		strings.Contains(string(down), "'wechat_official'") {
		t.Fatal("migration 057 rollback does not restore the previous channel contract")
	}
}
