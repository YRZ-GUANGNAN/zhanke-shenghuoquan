package takeout

import (
	"os"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Test00PaymentMigrationRollbackApplyAgainstRealMySQL84(t *testing.T) {
	db := openG4MySQL(t)
	assertG4MySQL84(t, db)
	hadPaymentTables := false
	for _, table := range []any{&PaymentNotification{}, &PaymentAttempt{}} {
		if !db.Migrator().HasTable(table) {
			continue
		}
		hadPaymentTables = true
		var count int64
		if err := db.Model(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("refusing to roll back non-empty %T rows=%d", table, count)
		}
	}
	if hadPaymentTables {
		applyOrderSQLFile(t, db, "../../../migrations/rollback/022_takeout_payment.sql")
	}
	applyOrderSQLFile(t, db, "../../../migrations/022_takeout_payment.sql")
	assertG4PaymentSchema(t, db)
	applyOrderSQLFile(t, db, "../../../migrations/rollback/022_takeout_payment.sql")
	for _, table := range []any{&PaymentNotification{}, &PaymentAttempt{}} {
		if db.Migrator().HasTable(table) {
			t.Fatalf("rollback 022 left %T behind", table)
		}
	}
	applyOrderSQLFile(t, db, "../../../migrations/022_takeout_payment.sql")
	assertG4PaymentSchema(t, db)
	t.Log("migration_022_apply_rollback_apply=PASS tables=2 foreign_keys=0")
}

func openG4MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("TAKEOUT_G4_MYSQL_DSN"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	}
	if dsn == "" {
		t.Skip("set TAKEOUT_G4_MYSQL_DSN or MYSQL_DSN for G-4 payment real MySQL acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func assertG4MySQL84(t *testing.T, db *gorm.DB) {
	t.Helper()
	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.4.") {
		t.Fatalf("expected MySQL 8.4, got %s", version)
	}
	t.Logf("real_mysql_version=%s", version)
}

func assertG4PaymentSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	expectedColumnCounts := map[string]int64{
		"takeout_payment_attempts":      18,
		"takeout_payment_notifications": 9,
	}
	for table, expected := range expectedColumnCounts {
		var count int64
		if err := db.Raw(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name=?`, table).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != expected {
			t.Errorf("%s columns=%d want=%d", table, count, expected)
		}
	}
	var foreignKeys int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.key_column_usage WHERE table_schema=DATABASE()
		AND table_name IN ('takeout_payment_attempts','takeout_payment_notifications')
		AND referenced_table_name IS NOT NULL`).Scan(&foreignKeys).Error; err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 0 {
		t.Fatalf("payment foreign keys=%d", foreignKeys)
	}
	for table, indexes := range map[string][]string{
		"takeout_payment_attempts": {
			"uq_takeout_payment_public_id", "uq_takeout_payment_out_trade_no",
			"uq_takeout_payment_wechat_transaction_id", "idx_takeout_payment_order",
			"idx_takeout_payment_buyer", "idx_takeout_payment_status",
		},
		"takeout_payment_notifications": {
			"uq_takeout_payment_notification_public_id", "uq_takeout_payment_notification_body_sha256",
			"idx_takeout_payment_notification_out_trade_no", "idx_takeout_payment_notification_status",
		},
	} {
		for _, index := range indexes {
			var count int64
			if err := db.Raw(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=?`, table, index).Scan(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count == 0 {
				t.Errorf("missing index %s.%s", table, index)
			}
		}
	}
}
