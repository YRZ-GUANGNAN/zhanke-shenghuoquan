package migrations

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func TestMigration030AgainstRealMySQL84(t *testing.T) {
	dsn := os.Getenv("UI21B2A_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set UI21B2A_MYSQL_DSN to run migration 030 against real MySQL 8.4")
	}

	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	originalDatabase := cfg.DBName
	if originalDatabase == "" {
		t.Fatal("UI21B2A_MYSQL_DSN must name a database")
	}
	cfg.MultiStatements = false
	cfg.DBName = ""
	server, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	var version string
	if err := server.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.4.") {
		t.Fatalf("expected MySQL 8.4, got %s", version)
	}

	databaseName := fmt.Sprintf("ui21b2a_migration_%d", time.Now().UnixNano())
	temporaryDatabase := true
	if _, err := server.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"); err != nil {
		if os.Getenv("UI21B2A_MYSQL_ALLOW_IN_PLACE") != "1" {
			t.Fatalf("create temporary database: %v (set UI21B2A_MYSQL_ALLOW_IN_PLACE=1 to use the DSN database)", err)
		}
		temporaryDatabase = false
		databaseName = originalDatabase
		t.Logf("temporary database creation unavailable; using dedicated acceptance database %s", databaseName)
	}
	if temporaryDatabase {
		t.Cleanup(func() {
			if _, dropErr := server.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`"); dropErr != nil {
				t.Errorf("drop temporary database: %v", dropErr)
			}
		})
	}

	cfg.DBName = databaseName
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if temporaryDatabase {
		applySQLScript(t, db, "001_init.sql")
	} else {
		assertMigration030SafeForInPlaceRollback(t, db)
	}
	cleanupHistorical := seedHistoricalWriteRows(t, db)
	t.Cleanup(cleanupHistorical)
	before := writeTableCounts(t, db)

	applySQLScript(t, db, "030_community_trade_write_contract.sql")
	applySQLScript(t, db, "030_community_trade_write_contract.sql")
	assertMigration030Schema(t, db)
	assertWriteTableCounts(t, db, before)
	assertRequestPairAndUniqueness(t, db)
	assertWriteTableCounts(t, db, before)

	applySQLScript(t, db, filepath.Join("rollback", "030_community_trade_write_contract.sql"))
	assertMigration030RolledBack(t, db)
	assertWriteTableCounts(t, db, before)

	applySQLScript(t, db, "030_community_trade_write_contract.sql")
	assertMigration030Schema(t, db)
	cleanupHistorical()
	t.Logf("migration_030_apply_apply_rollback_apply=PASS mysql=%s foreign_keys=0", version)
}

func applySQLScript(t *testing.T, db *sql.DB, relativePath string) {
	t.Helper()
	file, err := os.Open(filepath.Clean(relativePath))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	delimiter := ";"
	var statement strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") && strings.TrimSpace(statement.String()) == "" {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(trimmed), "DELIMITER ") {
			if strings.TrimSpace(statement.String()) != "" {
				t.Fatalf("%s changes delimiter with pending SQL", relativePath)
			}
			delimiter = strings.TrimSpace(trimmed[len("DELIMITER "):])
			continue
		}
		statement.WriteString(line)
		statement.WriteByte('\n')
		if !strings.HasSuffix(trimmed, delimiter) {
			continue
		}
		query := strings.TrimSpace(statement.String())
		query = strings.TrimSpace(strings.TrimSuffix(query, delimiter))
		statement.Reset()
		if query == "" {
			continue
		}
		if _, err := db.Exec(query); err != nil {
			t.Fatalf("apply %s: %v\nSQL:\n%s", relativePath, err, query)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(statement.String()) != "" {
		t.Fatalf("%s has unterminated SQL", relativePath)
	}
}

func seedHistoricalWriteRows(t *testing.T, db *sql.DB) func() {
	t.Helper()
	fence := fmt.Sprintf("UI21B2A-M030-%d", time.Now().UnixNano())
	for _, userID := range []int64{900000001, 900000002, 900000003, 900000011, 900000012, 900000013, 900000014} {
		if _, err := db.Exec(`INSERT INTO users(id,open_id,nickname) VALUES (?,?,?)`, userID, fmt.Sprintf("%s-%d", fence, userID), "migration 030"); err != nil {
			t.Fatal(err)
		}
	}
	result, err := db.Exec(`INSERT INTO posts(user_id, title, content) VALUES (900000001, ?, 'before 030')`, fence+"-post")
	if err != nil {
		t.Fatal(err)
	}
	postID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO comments(post_id, user_id, content) VALUES (?, 900000002, ?)`, postID, fence+"-comment"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO products(seller_id, title, price) VALUES (900000003, ?, 150)`, fence+"-product"); err != nil {
		t.Fatal(err)
	}
	return func() {
		_, _ = db.Exec(`DELETE FROM comments WHERE content = ?`, fence+"-comment")
		_, _ = db.Exec(`DELETE FROM products WHERE title = ?`, fence+"-product")
		_, _ = db.Exec(`DELETE FROM posts WHERE title = ?`, fence+"-post")
		_, _ = db.Exec(`DELETE FROM users WHERE open_id LIKE ?`, fence+"-%")
	}
}

func writeTableCounts(t *testing.T, db *sql.DB) map[string]int64 {
	t.Helper()
	counts := make(map[string]int64, 3)
	for _, table := range []string{"posts", "comments", "products"} {
		var count int64
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		counts[table] = count
	}
	return counts
}

func assertWriteTableCounts(t *testing.T, db *sql.DB, want map[string]int64) {
	t.Helper()
	got := writeTableCounts(t, db)
	for table, count := range want {
		if got[table] != count {
			t.Errorf("%s rows=%d want=%d", table, got[table], count)
		}
	}
}

func assertMigration030Schema(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, procedure := range []string{"migrate_ui21b2a_write_contract", "migrate_ui21b2a_outbox_indexes"} {
		assertInformationSchemaCount(t, db, 0, `SELECT COUNT(*) FROM information_schema.routines
			WHERE routine_schema=DATABASE() AND routine_type='PROCEDURE' AND routine_name=?`, procedure)
	}
	for _, table := range []string{"posts", "comments", "products"} {
		for _, column := range []string{"client_request_id", "create_request_hash"} {
			assertInformationSchemaCount(t, db, 1, `SELECT COUNT(*) FROM information_schema.columns
				WHERE table_schema=DATABASE() AND table_name=? AND column_name=?`, table, column)
		}
	}
	for table, index := range map[string]string{
		"posts":             "uq_posts_user_client_request",
		"comments":          "uq_comments_user_client_request",
		"products":          "uq_products_seller_client_request",
		"post_likes":        "idx_post_likes_user_post",
		"post_favorites":    "idx_post_favorites_user_post",
		"product_favorites": "idx_product_favorites_user_product",
	} {
		assertInformationSchemaCount(t, db, 1, `SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics
			WHERE table_schema=DATABASE() AND table_name=? AND index_name=?`, table, index)
	}
	for _, spec := range []struct {
		table   string
		index   string
		columns []string
		unique  bool
	}{
		{table: "posts", index: "uq_posts_user_client_request", columns: []string{"user_id", "client_request_id"}, unique: true},
		{table: "comments", index: "uq_comments_user_client_request", columns: []string{"user_id", "client_request_id"}, unique: true},
		{table: "products", index: "uq_products_seller_client_request", columns: []string{"seller_id", "client_request_id"}, unique: true},
		{table: "post_likes", index: "idx_post_likes_user_post", columns: []string{"user_id", "post_id"}},
		{table: "post_favorites", index: "idx_post_favorites_user_post", columns: []string{"user_id", "post_id"}},
		{table: "product_favorites", index: "idx_product_favorites_user_product", columns: []string{"user_id", "product_id"}},
		{table: "community_write_outbox", index: "idx_community_write_outbox_available", columns: []string{"status", "available_at", "id"}},
		{table: "community_write_outbox", index: "idx_community_write_outbox_lease", columns: []string{"status", "lease_expires_at", "id"}},
		{table: "trade_write_outbox", index: "idx_trade_write_outbox_available", columns: []string{"status", "available_at", "id"}},
		{table: "trade_write_outbox", index: "idx_trade_write_outbox_lease", columns: []string{"status", "lease_expires_at", "id"}},
	} {
		assertIndexShape(t, db, spec.table, spec.index, spec.columns, spec.unique)
	}
	for table, constraint := range map[string]string{
		"posts":    "chk_posts_create_request_pair",
		"comments": "chk_comments_create_request_pair",
		"products": "chk_products_create_request_pair",
	} {
		assertInformationSchemaCount(t, db, 1, `SELECT COUNT(*) FROM information_schema.table_constraints
			WHERE table_schema=DATABASE() AND table_name=? AND constraint_name=? AND constraint_type='CHECK'`, table, constraint)
	}
	for _, table := range []string{"community_write_outbox", "trade_write_outbox"} {
		assertInformationSchemaCount(t, db, 1, `SELECT COUNT(*) FROM information_schema.tables
			WHERE table_schema=DATABASE() AND table_name=?`, table)
		assertInformationSchemaCount(t, db, 0, `SELECT COUNT(*) FROM information_schema.referential_constraints
			WHERE constraint_schema=DATABASE() AND table_name=?`, table)
		assertInformationSchemaCount(t, db, 1, `SELECT COUNT(*) FROM information_schema.table_constraints
			WHERE table_schema=DATABASE() AND table_name=? AND constraint_type='CHECK'`, table)
		if _, err := db.Exec(`INSERT INTO ` + table + `
			(event_public_id,aggregate_type,aggregate_id,owner_user_id,event_type,payload,status,available_at,row_version,created_at,updated_at)
			VALUES ('01J00000000000000000000000','create',1,1,'created',JSON_OBJECT('assetPublicIds',JSON_ARRAY('01J00000000000000000000001'),'risk','low'),'invalid',NOW(3),1,NOW(3),NOW(3))`); err == nil {
			t.Errorf("%s accepted invalid status", table)
		}
	}
}

func assertRequestPairAndUniqueness(t *testing.T, db *sql.DB) {
	t.Helper()
	defer func() {
		_, _ = db.Exec(`DELETE FROM posts WHERE title IN ('UI21B2A-M030-keyed','UI21B2A-M030-other-owner')`)
	}()
	if _, err := db.Exec(`INSERT INTO posts(user_id,title,client_request_id)
		VALUES (900000011,'UI21B2A-M030-invalid-pair','01J00000000000000000000010')`); err == nil {
		t.Error("posts accepted client_request_id without create_request_hash")
	}
	if _, err := db.Exec(`INSERT INTO products(seller_id,title,create_request_hash)
		VALUES (900000012,'UI21B2A-M030-invalid-pair',UNHEX(SHA2('invalid',256)))`); err == nil {
		t.Error("products accepted create_request_hash without client_request_id")
	}
	if _, err := db.Exec(`INSERT INTO posts(user_id,title,client_request_id,create_request_hash)
		VALUES (900000013,'UI21B2A-M030-keyed','01J00000000000000000000011',UNHEX(SHA2('same',256)))`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO posts(user_id,title,client_request_id,create_request_hash)
		VALUES (900000013,'UI21B2A-M030-duplicate','01J00000000000000000000011',UNHEX(SHA2('different',256)))`); err == nil {
		t.Error("posts accepted duplicate owner/client_request_id")
	}
	if _, err := db.Exec(`INSERT INTO posts(user_id,title,client_request_id,create_request_hash)
		VALUES (900000014,'UI21B2A-M030-other-owner','01J00000000000000000000011',UNHEX(SHA2('same',256)))`); err != nil {
		t.Fatalf("posts rejected same key for different owner: %v", err)
	}
}

func assertMigration030SafeForInPlaceRollback(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, table := range []string{"community_write_outbox", "trade_write_outbox"} {
		var exists int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables
			WHERE table_schema=DATABASE() AND table_name=?`, table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if exists == 0 {
			continue
		}
		var count int64
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("refusing in-place rollback: %s rows=%d", table, count)
		}
	}
	for _, table := range []string{"posts", "comments", "products"} {
		var columns int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema=DATABASE() AND table_name=? AND column_name IN ('client_request_id','create_request_hash')`, table).Scan(&columns); err != nil {
			t.Fatal(err)
		}
		if columns == 0 {
			continue
		}
		if columns != 2 {
			t.Fatalf("refusing in-place rollback: %s has partial request columns=%d", table, columns)
		}
		var keyed int64
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table + " WHERE client_request_id IS NOT NULL OR create_request_hash IS NOT NULL").Scan(&keyed); err != nil {
			t.Fatal(err)
		}
		if keyed != 0 {
			t.Fatalf("refusing in-place rollback: %s keyed rows=%d", table, keyed)
		}
	}
}

func assertMigration030RolledBack(t *testing.T, db *sql.DB) {
	t.Helper()
	assertInformationSchemaCount(t, db, 0, `SELECT COUNT(*) FROM information_schema.routines
		WHERE routine_schema=DATABASE() AND routine_type='PROCEDURE'
		AND routine_name IN ('rollback_ui21b2a_write_contract','migrate_ui21b2a_write_contract','migrate_ui21b2a_outbox_indexes')`)
	for _, table := range []string{"community_write_outbox", "trade_write_outbox"} {
		assertInformationSchemaCount(t, db, 0, `SELECT COUNT(*) FROM information_schema.tables
			WHERE table_schema=DATABASE() AND table_name=?`, table)
	}
	for _, table := range []string{"posts", "comments", "products"} {
		for _, column := range []string{"client_request_id", "create_request_hash"} {
			assertInformationSchemaCount(t, db, 0, `SELECT COUNT(*) FROM information_schema.columns
				WHERE table_schema=DATABASE() AND table_name=? AND column_name=?`, table, column)
		}
	}
}

func assertIndexShape(t *testing.T, db *sql.DB, table, index string, wantColumns []string, unique bool) {
	t.Helper()
	rows, err := db.Query(`SELECT column_name, non_unique FROM information_schema.statistics
		WHERE table_schema=DATABASE() AND table_name=? AND index_name=? ORDER BY seq_in_index`, table, index)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var gotColumns []string
	var gotNonUnique int
	for rows.Next() {
		var column string
		if err := rows.Scan(&column, &gotNonUnique); err != nil {
			t.Fatal(err)
		}
		gotColumns = append(gotColumns, column)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(gotColumns, ",") != strings.Join(wantColumns, ",") {
		t.Errorf("%s.%s columns=%v want=%v", table, index, gotColumns, wantColumns)
	}
	wantNonUnique := 1
	if unique {
		wantNonUnique = 0
	}
	if len(gotColumns) > 0 && gotNonUnique != wantNonUnique {
		t.Errorf("%s.%s non_unique=%d want=%d", table, index, gotNonUnique, wantNonUnique)
	}
}

func assertInformationSchemaCount(t *testing.T, db *sql.DB, want int64, query string, args ...any) {
	t.Helper()
	var got int64
	if err := db.QueryRow(query, args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("information_schema count=%d want=%d query=%s args=%v", got, want, query, args)
	}
}
