package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const migration048File = "048_delivery_payment_recovery.sql"

func readMigration048(t *testing.T, parts ...string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestMigration048AddsPaymentRecoveryWithoutBackfillingTrust(t *testing.T) {
	source := readMigration048(t, migration048File)
	for _, token := range []string{
		"ALTER TABLE delivery_payment_attempts",
		"payer_openid VARCHAR(128) NULL",
		"provider_status VARCHAR(32) NOT NULL DEFAULT ''",
		"next_verify_at DATETIME(3) NULL",
		"verify_lease_until DATETIME(3) NULL",
		"verify_attempt_count INT UNSIGNED NOT NULL DEFAULT 0",
		"last_error_code VARCHAR(64) NOT NULL DEFAULT ''",
		"idx_delivery_payment_verify",
		"ALTER TABLE delivery_refunds",
		"out_trade_no VARCHAR(64) NULL",
		"idx_delivery_refund_verify",
		"idx_delivery_refund_out_trade_no",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 048 missing %q", token)
		}
	}
	for _, forbidden := range []string{"UPDATE delivery_payment_attempts", "UPDATE delivery_refunds", "payer_openid VARCHAR(128) NOT NULL"} {
		if strings.Contains(strings.ToUpper(source), strings.ToUpper(forbidden)) {
			t.Fatalf("migration 048 must not infer historical trust: %q", forbidden)
		}
	}
}

func TestMigration048RollbackIsScopedAndSymmetric(t *testing.T) {
	source := readMigration048(t, filepath.Join("rollback", migration048File))
	for _, token := range []string{
		"DROP INDEX idx_delivery_refund_out_trade_no",
		"DROP INDEX idx_delivery_refund_verify",
		"DROP INDEX idx_delivery_payment_verify",
		"DROP COLUMN out_trade_no",
		"DROP COLUMN payer_openid",
		"DROP COLUMN provider_status",
		"DROP COLUMN next_verify_at",
		"DROP COLUMN verify_lease_until",
		"DROP COLUMN verify_attempt_count",
		"DROP COLUMN last_error_code",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("rollback 048 missing %q", token)
		}
	}
	for _, forbidden := range []string{"DROP TABLE", "delivery_orders", "delivery_rider_wallets"} {
		if strings.Contains(strings.ToUpper(source), strings.ToUpper(forbidden)) {
			t.Fatalf("rollback 048 exceeds payment recovery scope: %q", forbidden)
		}
	}
}
