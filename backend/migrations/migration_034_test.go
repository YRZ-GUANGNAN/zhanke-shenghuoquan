package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

const migration034File = "034_identity_application_review.sql"

var migration034DatabaseSequence uint64

func TestMigration034DefinesIdentityApplicationsAndGrants(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration034File)
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, token := range []string{
		"CREATE TABLE identity_applications",
		"CREATE TABLE identity_application_assets",
		"CREATE TABLE identity_application_events",
		"CREATE TABLE user_identity_grants",
		"UNIQUE KEY uq_identity_application_user_role",
		"UNIQUE KEY uq_identity_grant_user_role",
		"CHECK (role IN ('rider','merchant'))",
		"CHECK (status IN ('pending','approved','rejected'))",
		"SELECT tr.rider_user_id,'rider'",
		"SELECT DISTINCT o.merchant_user_id,'merchant'",
		"open_id NOT LIKE 'mock_openid\\_%'",
		"'identity_application'",
		"MODIFY COLUMN purpose VARCHAR(24)",
		"MODIFY COLUMN reference_type VARCHAR(24)",
		"chk_media_reference_type_identity_application_guard",
		"DROP CHECK chk_media_reference_type",
		"ON DUPLICATE KEY UPDATE user_id=VALUES(user_id)",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 034 missing %q", token)
		}
	}
	if _, err := os.Stat(filepath.Join("rollback", migration034File)); !os.IsNotExist(err) {
		t.Fatal("migration 034 must remain forward-only")
	}
}

func TestMigration034IsGuardedForRepeatApplication(t *testing.T) {
	sourceBytes, err := os.ReadFile(migration034File)
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, token := range []string{
		"CREATE PROCEDURE migrate_identity_application_review",
		"IF NOT EXISTS",
		"information_schema.tables",
		"information_schema.columns",
		"information_schema.table_constraints",
		"CALL migrate_identity_application_review()",
		"DROP PROCEDURE migrate_identity_application_review",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 034 repeat-application guard missing %q", token)
		}
	}
	guard := strings.Index(source, "chk_media_reference_type_identity_application_guard")
	drop := strings.Index(source, "DROP CHECK chk_media_reference_type")
	if guard < 0 || drop < 0 || guard > drop {
		t.Fatal("identity-application media guard must be installed before replacing the canonical constraint")
	}
}

func TestMigration034MySQLIsIdempotentAndBackfillsOnlyRealQualifiedUsers(t *testing.T) {
	db := newMigration034Database(t)
	createMigration034BaseSchema(t, db)

	mustExec034(t, db, `INSERT INTO users(id,open_id) VALUES
		(101,'real-rider-openid'),
		(102,'mock_openid_rider'),
		(103,'real-resting-rider'),
		(201,'real-merchant-openid'),
		(202,'mock_openid_merchant')`)
	mustExec034(t, db, `INSERT INTO takeout_riders(rider_user_id,status) VALUES
		(101,'active'),(102,'active'),(103,'resting')`)
	mustExec034(t, db, `INSERT INTO delivery_orders(merchant_user_id) VALUES (201),(201),(202)`)

	applyMigration034(t, db)
	applyMigration034(t, db)

	assertCount034(t, db, 4, `SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema=DATABASE() AND table_name IN (
			'identity_applications','identity_application_assets',
			'identity_application_events','user_identity_grants'
		)`)
	assertCount034(t, db, 1, `SELECT COUNT(*) FROM user_identity_grants
		WHERE user_id=101 AND role='rider' AND status='active'`)
	assertCount034(t, db, 1, `SELECT COUNT(*) FROM user_identity_grants
		WHERE user_id=201 AND role='merchant' AND status='active'`)
	assertCount034(t, db, 0, `SELECT COUNT(*) FROM user_identity_grants
		WHERE user_id IN (102,103,202)`)
	assertCount034(t, db, 2, `SELECT COUNT(*) FROM user_identity_grants`)
	assertCount034(t, db, 2, `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema=DATABASE()
		  AND ((table_name='media_assets' AND column_name='purpose')
		    OR (table_name='media_asset_references' AND column_name='reference_type'))
		  AND data_type='varchar' AND character_maximum_length=24 AND is_nullable='NO'`)
	assertCount034(t, db, 0, `SELECT COUNT(*) FROM information_schema.table_constraints
		WHERE constraint_schema=DATABASE()
		  AND constraint_name IN (
		    'chk_media_asset_purpose_identity_application_guard',
		    'chk_media_reference_type_identity_application_guard'
		  )`)

	mustExec034(t, db, `INSERT INTO media_assets(purpose) VALUES ('identity_application')`)
	mustExec034(t, db, `INSERT INTO media_asset_references(reference_type) VALUES ('identity_application')`)
}

func newMigration034Database(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run migration 034 against an independent temporary MySQL database")
	}
	cfg, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MultiStatements = false
	cfg.DBName = ""
	server, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Ping(); err != nil {
		server.Close()
		t.Fatalf("connect MySQL for migration 034: %v", err)
	}
	var version string
	if err := server.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		server.Close()
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.") {
		server.Close()
		t.Skipf("migration 034 requires MySQL 8, got %s", version)
	}

	sequence := atomic.AddUint64(&migration034DatabaseSequence, 1)
	databaseName := fmt.Sprintf("identity_review_m034_%d_%d", time.Now().UnixNano(), sequence)
	if _, err := server.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		server.Close()
		t.Skipf("cannot create independent migration 034 database; shared campus_circle fallback is forbidden: %v", err)
	}
	cfg.DBName = databaseName
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		_, _ = server.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
		server.Close()
		t.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		_, _ = server.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`")
		server.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close migration 034 database: %v", err)
		}
		if _, err := server.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`"); err != nil {
			t.Errorf("drop independent migration 034 database: %v", err)
		}
		if err := server.Close(); err != nil {
			t.Errorf("close migration 034 server connection: %v", err)
		}
	})
	return db
}

func createMigration034BaseSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, statement := range []string{
		`CREATE TABLE users (
			id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
			open_id VARCHAR(96) NOT NULL UNIQUE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE media_assets (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			purpose VARCHAR(16) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE media_asset_references (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			reference_type VARCHAR(16) NOT NULL,
			CONSTRAINT chk_media_reference_type
			CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback'))
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE takeout_riders (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			rider_user_id BIGINT UNSIGNED NOT NULL,
			status VARCHAR(24) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE delivery_orders (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			merchant_user_id BIGINT UNSIGNED NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	} {
		mustExec034(t, db, statement)
	}
}

func applyMigration034(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := applyMigration031Script(db, migration034File); err != nil {
		t.Fatalf("apply migration 034: %v", err)
	}
}

func mustExec034(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec migration 034 test SQL: %v\nSQL:\n%s", err, query)
	}
}

func assertCount034(t *testing.T, db *sql.DB, want int64, query string, args ...any) {
	t.Helper()
	var got int64
	if err := db.QueryRow(query, args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("query count=%d want=%d query=%s args=%v", got, want, query, args)
	}
}
