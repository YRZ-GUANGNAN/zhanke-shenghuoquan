package delivery

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

// This test is only reachable through the dedicated localhost/datadir-verified
// rider-money runner. It never consumes a caller database or production dump.
func TestRiderReleaseBackupRestore055056AgainstIsolatedMySQL(t *testing.T) {
	source := openIsolatedRiderMoneyMySQL(t)
	for _, name := range []string{
		"023_delivery_order.sql", "024_delivery_dispatch.sql", "025_delivery_payment_wallet.sql",
		"028_delivery_fee_and_rider_wallet.sql", "045_delivery_order_source.sql",
		"048_delivery_payment_recovery.sql", "049_delivery_funding_sources.sql",
		"050_delivery_rider_payout_queue.sql", "051_delivery_rider_withdrawals.sql",
		"052_delivery_recharge_cancellation.sql", "054_delivery_rider_daily_settlement.sql",
	} {
		applyD1SQLFile(t, source, "../../../migrations/"+name)
	}
	now := time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC)
	seedRiderMoneyWallet(t, source, 988201, 700, now)
	seedRiderWithdrawableEarning(t, source, 988201, 700, now)
	before := riderReleaseTableDigests(t, source, nil)

	var sourceName string
	if err := source.Raw("SELECT DATABASE()").Scan(&sourceName).Error; err != nil {
		t.Fatal(err)
	}
	dump := riderReleaseLocalClient(t, "mysqldump.exe", sourceName, nil,
		"--single-transaction", "--no-tablespaces", "--hex-blob", "--set-gtid-purged=OFF")
	if !bytes.Contains(dump[max(0, len(dump)-4096):], []byte("-- Dump completed on ")) {
		t.Fatal("isolated backup does not contain dump completion evidence")
	}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(dump); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "isolated-rider-backup.sql.gz")
	if err := os.WriteFile(archive, compressed.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	restoredDump, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil || !bytes.Equal(restoredDump, dump) {
		t.Fatal("isolated gzip backup CRC/content verification failed")
	}

	restored := openIsolatedRiderMoneyMySQL(t)
	var restoredName string
	if err := restored.Raw("SELECT DATABASE()").Scan(&restoredName).Error; err != nil {
		t.Fatal(err)
	}
	if sourceName == restoredName {
		t.Fatal("restore must use a separate freshly-created isolated database")
	}
	riderReleaseLocalClient(t, "mysql.exe", restoredName, restoredDump)
	if actual := riderReleaseTableDigests(t, restored, before); !reflect.DeepEqual(actual, before) {
		t.Fatal("restored original table digests differ from isolated backup source")
	}
	for _, name := range []string{"055_delivery_rider_availability.sql", "056_delivery_rider_withdrawal_allocations.sql"} {
		applyD1SQLFile(t, restored, "../../../migrations/"+name)
	}
	if actual := riderReleaseTableDigests(t, restored, before); !reflect.DeepEqual(actual, before) {
		t.Fatal("055/056 rehearsal changed original isolated table rows")
	}
	if actual := riderReleaseTableDigests(t, source, before); !reflect.DeepEqual(actual, before) {
		t.Fatal("isolated source changed during verification-database rehearsal")
	}
	for _, table := range []string{"delivery_rider_availability", "delivery_rider_withdrawal_allocations",
		"delivery_rider_withdrawal_claims", "delivery_rider_withdrawal_events", "delivery_rider_withdrawal_payment_references"} {
		assertRiderMoneyCount(t, restored, table, 0)
	}
	assertRiderMoneyCount(t, restored, "delivery_rider_broadcast_config", 1)
	if err := NewRepository(restored).CheckWithdrawalReadiness(context.Background()); err != nil {
		t.Fatalf("restored migration source-readiness check failed: %v", err)
	}
	t.Logf("isolated_055_056_backup_restore=PASS original_tables=%d backup_sha256=%x original_rows_unchanged=true source_unchanged=true no_production_connection=true", len(before), sha256.Sum256(compressed.Bytes()))
}

func riderReleaseLocalClient(t *testing.T, executable, database string, input []byte, extra ...string) []byte {
	t.Helper()
	if !regexp.MustCompile(`^delivery_rider_money_[0-9]+_[0-9]+$`).MatchString(database) {
		t.Fatal("refusing a non-runner rehearsal database")
	}
	if executable != "mysql.exe" && executable != "mysqldump.exe" {
		t.Fatal("refusing unexpected rehearsal executable")
	}
	program := filepath.Join(`C:\Program Files\MySQL\MySQL Server 8.4\bin`, executable)
	if _, err := os.Stat(program); err != nil {
		t.Fatal("installed MySQL 8.4 client and mysqldump are required for the isolated rehearsal")
	}
	arguments := []string{"--no-defaults", "--no-login-paths", "--protocol=tcp", "--host=127.0.0.1",
		"--port=33318", "--user=root", "--skip-password", "--default-character-set=utf8mb4"}
	if executable == "mysql.exe" {
		arguments = append(arguments, "--connect-timeout=3")
	}
	arguments = append(arguments, extra...)
	arguments = append(arguments, database)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, program, arguments...)
	for _, value := range os.Environ() {
		name := strings.SplitN(value, "=", 2)[0]
		upper := strings.ToUpper(name)
		if !strings.Contains(upper, "MYSQL") && !strings.Contains(upper, "DSN") && !strings.Contains(upper, "DATABASE_URL") {
			command.Env = append(command.Env, value)
		}
	}
	command.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("isolated %s failed; sensitive output withheld", executable)
	}
	return stdout.Bytes()
}

func riderReleaseTableDigests(t *testing.T, db *gorm.DB, originals map[string]string) map[string]string {
	t.Helper()
	var tables []string
	if originals == nil {
		if err := db.Raw("SHOW TABLES").Scan(&tables).Error; err != nil {
			t.Fatal(err)
		}
	} else {
		for name := range originals {
			tables = append(tables, name)
		}
	}
	digests := make(map[string]string, len(tables))
	for _, table := range tables {
		if !regexp.MustCompile(`^[a-z_0-9]+$`).MatchString(table) {
			t.Fatal("unsafe isolated table identifier")
		}
		var columns, primary []string
		if err := db.Raw("SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? ORDER BY ORDINAL_POSITION", table).Scan(&columns).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Raw("SELECT COLUMN_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=? AND INDEX_NAME='PRIMARY' ORDER BY SEQ_IN_INDEX", table).Scan(&primary).Error; err != nil || len(primary) == 0 {
			t.Fatal("stable isolated snapshot requires a primary key")
		}
		parts := make([]string, 0, len(columns))
		for _, name := range columns {
			if !regexp.MustCompile(`^[a-z_0-9]+$`).MatchString(name) {
				t.Fatal("unsafe isolated column identifier")
			}
			parts = append(parts, "CONCAT('"+name+"=',IF(`"+name+"` IS NULL,'N',CONCAT('V',HEX(`"+name+"`))))")
		}
		query := "SELECT CONCAT_WS('|'," + strings.Join(parts, ",") + ") FROM `" + table + "` ORDER BY `" + strings.Join(primary, "`,`") + "`"
		rows, err := db.Raw(query).Rows()
		if err != nil {
			t.Fatal(err)
		}
		hash, count := sha256.New(), 0
		for rows.Next() {
			var value string
			if err := rows.Scan(&value); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			fmt.Fprintln(hash, value)
			count++
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		rows.Close()
		digests[table] = fmt.Sprintf("%d:%x", count, hash.Sum(nil))
	}
	return digests
}
