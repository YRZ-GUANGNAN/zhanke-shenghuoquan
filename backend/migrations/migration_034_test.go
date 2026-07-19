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
		"chk_media_asset_purpose_identity_application_guard",
		"chk_media_reference_type_identity_application_guard",
		"users.deleted_at IS NULL",
		"users.closed_at IS NULL",
		"users.status IN ('active','muted')",
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
	for _, token := range []string{
		"users.deleted_at IS NULL",
		"users.closed_at IS NULL",
		"users.status IN ('active','muted')",
	} {
		if count := strings.Count(source, token); count != 2 {
			t.Fatalf("migration 034 requires %q in both rider and merchant backfills, count=%d", token, count)
		}
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
	assertMigration034Order(t, source, "migration 034 unexpected media purpose check constraint", "CREATE TABLE identity_applications")
	assertMigration034Order(t, source, "migration 034 unexpected media reference type check constraint", "CREATE TABLE identity_applications")
	assertMigration034Order(t, source, "migration 034 media purpose canonical constraint mismatch", "CREATE TABLE identity_applications")
	assertMigration034Order(t, source, "migration 034 media reference type canonical constraint mismatch", "CREATE TABLE identity_applications")
	assertMigration034Order(t, source, "migration 034 media purpose guard constraint mismatch", "CREATE TABLE identity_applications")
	assertMigration034Order(t, source, "migration 034 media reference type guard constraint mismatch", "CREATE TABLE identity_applications")
	assertMigration034Order(t, source, "migration 034 unsupported media purpose", "CREATE TABLE identity_applications")
	assertMigration034Order(t, source, "migration 034 unsupported media reference type", "CREATE TABLE identity_applications")
	assertMigration034Order(t, source, "ADD CONSTRAINT chk_media_asset_purpose_identity_application_guard", "DROP CHECK chk_media_asset_purpose")
	assertMigration034Order(t, source, "ADD CONSTRAINT chk_media_reference_type_identity_application_guard", "DROP CHECK chk_media_reference_type")
	if strings.Contains(source, "AND LEFT(LOWER(") {
		t.Fatal("migration 034 canonical checks must use exact normalized-expression allowlists")
	}
	if !strings.Contains(source, "'reference_typein''post'',''product'',''avatar'',''dish'',''merchant_logo'',''delivery_proof'',''delivery_im'',''feedback''") {
		t.Fatal("migration 034 must allow the exact migration 033 reference-type canonical expression")
	}
}

func TestMigration034MySQLIsIdempotentAndBackfillsOnlyRealQualifiedUsers(t *testing.T) {
	db := newMigration034Database(t)
	createMigration034BaseSchema(t, db)

	mustExec034(t, db, `INSERT INTO users(id,open_id,status,closed_at,deleted_at) VALUES
		(101,'real-active-openid','active',NULL,NULL),
		(102,'real-muted-openid','muted',NULL,NULL),
		(103,'real-banned-openid','banned',NULL,NULL),
		(104,'real-closed-openid','closed',NULL,NULL),
		(105,'real-closed-at-openid','active',NOW(3),NULL),
		(106,'real-deleted-openid','active',NULL,NOW(3)),
		(107,'mock_openid_qualified','active',NULL,NULL),
		(108,'real-resting-rider','active',NULL,NULL)`)
	mustExec034(t, db, `INSERT INTO takeout_riders(rider_user_id,status) VALUES
		(101,'active'),(102,'active'),(103,'active'),(104,'active'),
		(105,'active'),(106,'active'),(107,'active'),(108,'resting')`)
	mustExec034(t, db, `INSERT INTO delivery_orders(merchant_user_id) VALUES
		(101),(101),(102),(103),(104),(105),(106),(107),(108)`)

	applyMigration034(t, db)
	mustExec034(t, db, `INSERT INTO user_identity_grants
		(user_id,role,status,source_application_id,granted_at,row_version,created_at,updated_at)
		VALUES (103,'rider','active',NULL,NOW(3),1,NOW(3),NOW(3))`)
	applyMigration034(t, db)

	assertCount034(t, db, 4, `SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema=DATABASE() AND table_name IN (
			'identity_applications','identity_application_assets',
			'identity_application_events','user_identity_grants'
		)`)
	assertCount034(t, db, 2, `SELECT COUNT(*) FROM user_identity_grants
		WHERE user_id=101 AND role IN ('rider','merchant') AND status='active'`)
	assertCount034(t, db, 2, `SELECT COUNT(*) FROM user_identity_grants
		WHERE user_id=102 AND role IN ('rider','merchant') AND status='active'`)
	assertCount034(t, db, 0, `SELECT COUNT(*) FROM user_identity_grants
		WHERE user_id IN (103,104,105,106,107) AND status='active'`)
	assertCount034(t, db, 1, `SELECT COUNT(*) FROM user_identity_grants
		WHERE user_id=103 AND role='rider' AND status='suspended' AND source_application_id IS NULL`)
	assertCount034(t, db, 1, `SELECT COUNT(*) FROM user_identity_grants
		WHERE user_id=108 AND role='merchant' AND status='active'`)
	assertCount034(t, db, 0, `SELECT COUNT(*) FROM user_identity_grants
		WHERE user_id=108 AND role='rider'`)
	assertCount034(t, db, 5, `SELECT COUNT(*) FROM user_identity_grants WHERE status='active'`)
	assertCount034(t, db, 6, `SELECT COUNT(*) FROM user_identity_grants`)
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

func TestMigration034MySQLRejectsUnexpectedMediaChecksBeforeDDL(t *testing.T) {
	for _, tc := range []struct {
		name       string
		setup      []string
		want       string
		columnName string
	}{
		{
			name: "asset_purpose",
			setup: []string{`ALTER TABLE media_assets ADD CONSTRAINT legacy_media_purpose_enum
				CHECK (purpose IN ('post','product','avatar'))`},
			want:       "migration 034 unexpected media purpose check constraint",
			columnName: "purpose",
		},
		{
			name: "reference_type",
			setup: []string{
				`ALTER TABLE media_asset_references DROP CHECK chk_media_reference_type`,
				`ALTER TABLE media_asset_references ADD CONSTRAINT legacy_media_reference_enum
				CHECK (reference_type IN ('post','product','avatar'))`,
			},
			want:       "migration 034 unexpected media reference type check constraint",
			columnName: "reference_type",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newMigration034Database(t)
			createMigration034BaseSchema(t, db)
			for _, statement := range tc.setup {
				mustExec034(t, db, statement)
			}

			err := executeMigration034(db)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("migration 034 error=%v want substring %q", err, tc.want)
			}
			assertCount034(t, db, 0, `SELECT COUNT(*) FROM information_schema.tables
				WHERE table_schema=DATABASE() AND table_name IN (
					'identity_applications','identity_application_assets',
					'identity_application_events','user_identity_grants'
				)`)
			assertCount034(t, db, 1, `SELECT COUNT(*) FROM information_schema.columns
				WHERE table_schema=DATABASE() AND column_name=?
				  AND character_maximum_length=16`, tc.columnName)
		})
	}
}

func TestMigration034MySQLRejectsMisbindingCanonicalAndGuardChecksBeforeDDL(t *testing.T) {
	for _, tc := range []struct {
		name       string
		setup      []string
		want       string
		columnName string
	}{
		{
			name: "asset_canonical_on_unrelated_column",
			setup: []string{`ALTER TABLE media_assets ADD CONSTRAINT chk_media_asset_purpose
				CHECK (other_flag IN (0,1))`},
			want:       "migration 034 media purpose canonical constraint mismatch",
			columnName: "purpose",
		},
		{
			name: "reference_canonical_on_unrelated_column",
			setup: []string{
				`ALTER TABLE media_asset_references DROP CHECK chk_media_reference_type`,
				`ALTER TABLE media_asset_references ADD CONSTRAINT chk_media_reference_type
				CHECK (other_flag IN (0,1))`,
			},
			want:       "migration 034 media reference type canonical constraint mismatch",
			columnName: "reference_type",
		},
		{
			name: "asset_canonical_mixes_unrelated_column",
			setup: []string{`ALTER TABLE media_assets ADD CONSTRAINT chk_media_asset_purpose
				CHECK (purpose IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application')
				AND other_flag IN (0,1))`},
			want:       "migration 034 media purpose canonical constraint mismatch",
			columnName: "purpose",
		},
		{
			name: "reference_canonical_mixes_unrelated_column",
			setup: []string{
				`ALTER TABLE media_asset_references DROP CHECK chk_media_reference_type`,
				`ALTER TABLE media_asset_references ADD CONSTRAINT chk_media_reference_type
				CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback')
				AND other_flag IN (0,1))`,
			},
			want:       "migration 034 media reference type canonical constraint mismatch",
			columnName: "reference_type",
		},
		{
			name: "asset_guard_missing_identity_application",
			setup: []string{`ALTER TABLE media_assets
				ADD CONSTRAINT chk_media_asset_purpose_identity_application_guard
				CHECK (purpose IN ('post','product','avatar'))`},
			want:       "migration 034 media purpose guard constraint mismatch",
			columnName: "purpose",
		},
		{
			name: "reference_guard_missing_identity_application",
			setup: []string{`ALTER TABLE media_asset_references
				ADD CONSTRAINT chk_media_reference_type_identity_application_guard
				CHECK (reference_type IN ('post','product','avatar'))`},
			want:       "migration 034 media reference type guard constraint mismatch",
			columnName: "reference_type",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := newMigration034Database(t)
			createMigration034BaseSchema(t, db)
			for _, statement := range tc.setup {
				mustExec034(t, db, statement)
			}

			err := executeMigration034(db)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("migration 034 error=%v want substring %q", err, tc.want)
			}
			assertMigration034BusinessSchemaUnchanged(t, db, tc.columnName)
		})
	}
}

func TestMigration034MySQLPreservesUnrelatedMediaChecks(t *testing.T) {
	db := newMigration034Database(t)
	createMigration034BaseSchema(t, db)
	mustExec034(t, db, `ALTER TABLE media_assets ADD CONSTRAINT chk_media_asset_other_flag
		CHECK (other_flag IN (0,1))`)
	mustExec034(t, db, `ALTER TABLE media_asset_references ADD CONSTRAINT chk_media_reference_other_flag
		CHECK (other_flag IN (0,1))`)

	applyMigration034(t, db)

	assertCount034(t, db, 2, `SELECT COUNT(*) FROM information_schema.table_constraints
		WHERE constraint_schema=DATABASE() AND constraint_name IN (
			'chk_media_asset_other_flag','chk_media_reference_other_flag'
		) AND constraint_type='CHECK'`)
}

func TestMigration034MySQLAllowsWhitelistedCanonicalChecks(t *testing.T) {
	db := newMigration034Database(t)
	createMigration034BaseSchema(t, db)
	mustExec034(t, db, `ALTER TABLE media_assets ADD CONSTRAINT chk_media_asset_purpose
		CHECK (purpose IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application'))`)

	// The base reference canonical is migration 033's exact set. The first run
	// upgrades it; the second run proves the migration 034 canonical also passes.
	applyMigration034(t, db)
	applyMigration034(t, db)

	assertCount034(t, db, 2, `SELECT COUNT(*) FROM information_schema.table_constraints
		WHERE constraint_schema=DATABASE() AND constraint_name IN (
			'chk_media_asset_purpose','chk_media_reference_type'
		) AND constraint_type='CHECK'`)
	assertCount034(t, db, 2, `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema=DATABASE()
		  AND ((table_name='media_assets' AND column_name='purpose')
		    OR (table_name='media_asset_references' AND column_name='reference_type'))
		  AND character_maximum_length=24`)
}

func TestMigration034MySQLConvergesFromRecoverablePartialState(t *testing.T) {
	db := newMigration034Database(t)
	createMigration034BaseSchema(t, db)
	applyMigration034(t, db)

	mustExec034(t, db, `DROP TABLE identity_application_assets, identity_application_events, user_identity_grants`)
	mustExec034(t, db, `ALTER TABLE media_assets
		ADD CONSTRAINT chk_media_asset_purpose_identity_application_guard
		CHECK (purpose IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application'))`)
	mustExec034(t, db, `ALTER TABLE media_assets DROP CHECK chk_media_asset_purpose`)
	mustExec034(t, db, `ALTER TABLE media_asset_references
		ADD CONSTRAINT chk_media_reference_type_identity_application_guard
		CHECK (reference_type IN ('post','product','avatar','dish','merchant_logo','delivery_proof','delivery_im','feedback','identity_application'))`)
	mustExec034(t, db, `ALTER TABLE media_asset_references DROP CHECK chk_media_reference_type`)

	applyMigration034(t, db)

	assertCount034(t, db, 4, `SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema=DATABASE() AND table_name IN (
			'identity_applications','identity_application_assets',
			'identity_application_events','user_identity_grants'
		)`)
	assertCount034(t, db, 2, `SELECT COUNT(*) FROM information_schema.table_constraints
		WHERE constraint_schema=DATABASE() AND constraint_name IN (
			'chk_media_asset_purpose','chk_media_reference_type'
		)`)
	assertCount034(t, db, 0, `SELECT COUNT(*) FROM information_schema.table_constraints
		WHERE constraint_schema=DATABASE() AND constraint_name IN (
			'chk_media_asset_purpose_identity_application_guard',
			'chk_media_reference_type_identity_application_guard'
		)`)
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
			open_id VARCHAR(96) NOT NULL UNIQUE,
			status VARCHAR(24) NOT NULL DEFAULT 'active',
			closed_at DATETIME(3) NULL,
			deleted_at DATETIME(3) NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE media_assets (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			purpose VARCHAR(16) NOT NULL,
			other_flag INT NOT NULL DEFAULT 0
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE media_asset_references (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			reference_type VARCHAR(16) NOT NULL,
			other_flag INT NOT NULL DEFAULT 0,
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
	if err := executeMigration034(db); err != nil {
		t.Fatalf("apply migration 034: %v", err)
	}
}

func executeMigration034(db *sql.DB) error {
	return applyMigration031Script(db, migration034File)
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

func assertMigration034Order(t *testing.T, source, beforeToken, afterToken string) {
	t.Helper()
	before := strings.Index(source, beforeToken)
	after := strings.Index(source, afterToken)
	if before < 0 || after < 0 || before >= after {
		t.Fatalf("migration 034 requires %q before %q", beforeToken, afterToken)
	}
}

func assertMigration034BusinessSchemaUnchanged(t *testing.T, db *sql.DB, columnName string) {
	t.Helper()
	assertCount034(t, db, 0, `SELECT COUNT(*) FROM information_schema.tables
		WHERE table_schema=DATABASE() AND table_name IN (
			'identity_applications','identity_application_assets',
			'identity_application_events','user_identity_grants'
		)`)
	assertCount034(t, db, 1, `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema=DATABASE() AND column_name=?
		  AND character_maximum_length=16`, columnName)
}
