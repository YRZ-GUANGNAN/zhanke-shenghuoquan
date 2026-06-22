package takeout

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestG6A1WithdrawBoundaryAndIntegerMoney(t *testing.T) {
	migration, err := os.ReadFile("../../../migrations/021_takeout_withdraw.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := regexp.MustCompile(`(?m)--.*$`).ReplaceAllString(string(migration), "")
	if regexp.MustCompile(`(?i)\bFOREIGN\s+KEY\b|\bREFERENCES\b`).MatchString(sql) {
		t.Fatal("migration 021 contains a foreign key")
	}
	if !regexp.MustCompile(`(?i)amount_cents\s+BIGINT\s+UNSIGNED\s+NOT\s+NULL`).MatchString(sql) {
		t.Fatal("amount_cents is not BIGINT UNSIGNED NOT NULL")
	}
	if regexp.MustCompile(`(?i)\b(FLOAT|DOUBLE|DECIMAL)\b`).MatchString(sql) {
		t.Fatal("migration 021 contains non-integer money storage")
	}
	for _, required := range []string{"pending", "paid", "rejected", "row_version", "uq_takeout_withdraw_public_id"} {
		if !strings.Contains(sql, required) {
			t.Errorf("migration 021 missing %q", required)
		}
	}
	if _, err := os.Stat("../../../migrations/rollback/021_takeout_withdraw.sql"); err != nil {
		t.Fatal(err)
	}
}
