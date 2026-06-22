package takeout

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	g6WithdrawRider       = uint64(960001)
	g6WithdrawFaultRider  = uint64(960002)
	g6WithdrawRejectRider = uint64(960003)
)

func Test00WithdrawMigrationRollbackApplyAgainstRealMySQL84(t *testing.T) {
	db := openG6A1MySQL(t)
	assertG6A1MySQL84(t, db)
	cleanupG6A1Withdraw(t, db)
	if db.Migrator().HasTable(&WithdrawRequest{}) {
		var count int64
		if err := db.Model(&WithdrawRequest{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("refusing to roll back non-empty takeout_withdraw_requests rows=%d", count)
		}
		applyG6A1SQLFile(t, db, "../../../migrations/rollback/021_takeout_withdraw.sql")
	}
	if db.Migrator().HasTable(&WithdrawRequest{}) {
		t.Fatal("rollback 021 left takeout_withdraw_requests behind")
	}
	applyG6A1SQLFile(t, db, "../../../migrations/021_takeout_withdraw.sql")
	assertG6A1WithdrawSchema(t, db)
	applyG6A1SQLFile(t, db, "../../../migrations/rollback/021_takeout_withdraw.sql")
	if db.Migrator().HasTable(&WithdrawRequest{}) {
		t.Fatal("second rollback 021 left table behind")
	}
	applyG6A1SQLFile(t, db, "../../../migrations/021_takeout_withdraw.sql")
	assertG6A1WithdrawSchema(t, db)
	t.Log("migration_021_apply_rollback_apply=PASS table=takeout_withdraw_requests foreign_keys=0")
}

func TestWithdrawPaymentCASIdempotencyAndSettlementAgainstRealMySQL84(t *testing.T) {
	db := openG6A1MySQLWithTables(t)
	cleanupG6A1Withdraw(t, db)
	t.Cleanup(func() { cleanupG6A1Withdraw(t, db) })
	now := time.Date(2026, 6, 22, 21, 0, 0, 0, time.UTC)
	repository := NewRepository(db)
	service, _ := NewWithdrawService(repository, func() time.Time { return now })
	request, err := service.CreateWithdrawRequest(context.Background(), g6WithdrawRider, 200)
	if err != nil {
		t.Fatal(err)
	}
	createG6A1Earning(t, db, g6WithdrawRider, "01J00000000000000000000601", "01J00000000000000000000611", 100)
	createG6A1Earning(t, db, g6WithdrawRider, "01J00000000000000000000602", "01J00000000000000000000612", 100)

	if _, err := service.MarkWithdrawPaid(context.Background(), request.WithdrawPublicID, request.RowVersion+1); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale withdraw CAS returned %v", err)
	}
	paid, err := service.MarkWithdrawPaid(context.Background(), request.WithdrawPublicID, request.RowVersion)
	if err != nil || paid.Status != WithdrawPaid || paid.RowVersion != 2 || paid.PaidAt == nil {
		t.Fatalf("paid=%#v err=%v", paid, err)
	}
	var unsettled int64
	if err := db.Model(&RiderEarningRecord{}).Where("rider_user_id = ? AND settlement_status <> ?", g6WithdrawRider, SettlementSettled).Count(&unsettled).Error; err != nil {
		t.Fatal(err)
	}
	if unsettled != 0 {
		t.Fatalf("unsettled earnings=%d", unsettled)
	}
	repeated, err := service.MarkWithdrawPaid(context.Background(), request.WithdrawPublicID, request.RowVersion)
	if err != nil || repeated.Status != WithdrawPaid || repeated.RowVersion != 2 {
		t.Fatalf("repeated paid=%#v err=%v", repeated, err)
	}

	reject, err := service.CreateWithdrawRequest(context.Background(), g6WithdrawRejectRider, 300)
	if err != nil {
		t.Fatal(err)
	}
	reject, err = service.RejectWithdraw(context.Background(), reject.WithdrawPublicID, reject.RowVersion, "manual rejection")
	if err != nil || reject.Status != WithdrawRejected || reject.RowVersion != 2 {
		t.Fatalf("rejected=%#v err=%v", reject, err)
	}
	values, total, err := service.ListWithdrawRequests(context.Background(), WithdrawPaid, 1, 20)
	if err != nil || total != 1 || len(values) != 1 || values[0].WithdrawPublicID != paid.WithdrawPublicID {
		t.Fatalf("paid list=%#v total=%d err=%v", values, total, err)
	}
	t.Log("withdraw_CAS=PASS settlement_atomic=PASS repeated_paid=idempotent")
}

func TestWithdrawSettlementFailureRollsBackRealMySQLTransaction(t *testing.T) {
	db := openG6A1MySQLWithTables(t)
	cleanupG6A1Withdraw(t, db)
	t.Cleanup(func() { cleanupG6A1Withdraw(t, db) })
	now := time.Date(2026, 6, 22, 21, 30, 0, 0, time.UTC)
	repository := NewRepository(db)
	service, _ := NewWithdrawService(repository, func() time.Time { return now })
	request, err := service.CreateWithdrawRequest(context.Background(), g6WithdrawFaultRider, 200)
	if err != nil {
		t.Fatal(err)
	}
	createG6A1Earning(t, db, g6WithdrawFaultRider, "01J00000000000000000000621", "01J00000000000000000000631", 100)
	createG6A1Earning(t, db, g6WithdrawFaultRider, "01J00000000000000000000622", "01J00000000000000000000632", 100)

	updates := 0
	callbackName := "g6a1:fail_second_earning_update"
	err = db.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Table != "takeout_rider_earning_records" {
			return
		}
		updates++
		if updates == 2 {
			tx.AddError(errors.New("injected second settlement failure"))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.MarkWithdrawPaid(context.Background(), request.WithdrawPublicID, request.RowVersion); err == nil || !strings.Contains(err.Error(), "injected") {
		t.Fatalf("expected injected failure, got %v", err)
	}
	_ = db.Callback().Update().Remove(callbackName)

	loaded, err := service.FindWithdrawByID(context.Background(), request.WithdrawPublicID)
	if err != nil || loaded.Status != WithdrawPending || loaded.RowVersion != 1 || loaded.PaidAt != nil {
		t.Fatalf("withdraw rollback=%#v err=%v", loaded, err)
	}
	var settled int64
	if err := db.Model(&RiderEarningRecord{}).Where("rider_user_id = ? AND settlement_status = ?", g6WithdrawFaultRider, SettlementSettled).Count(&settled).Error; err != nil {
		t.Fatal(err)
	}
	if settled != 0 {
		t.Fatalf("partial settlement residue=%d", settled)
	}
	t.Log("withdraw_failure_injection=PASS transaction_rollback=PASS partial_settlement=0")
}

func TestZZWithdrawAcceptanceResidueAgainstRealMySQL84(t *testing.T) {
	db := openG6A1MySQLWithTables(t)
	cleanupG6A1Withdraw(t, db)
	var withdrawals, earnings int64
	if err := db.Model(&WithdrawRequest{}).Where("rider_user_id IN ?", g6A1RiderIDs()).Count(&withdrawals).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&RiderEarningRecord{}).Where("rider_user_id IN ?", g6A1RiderIDs()).Count(&earnings).Error; err != nil {
		t.Fatal(err)
	}
	if withdrawals != 0 || earnings != 0 {
		t.Fatalf("G6a1 residue withdrawals=%d earnings=%d", withdrawals, earnings)
	}
	t.Log("withdraw_residue withdrawals=0 earnings=0")
}

func openG6A1MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TAKEOUT_G6A1_MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("MYSQL_DSN")
	}
	if dsn == "" {
		t.Skip("set TAKEOUT_G6A1_MYSQL_DSN or MYSQL_DSN for G-6a-1 real MySQL acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func openG6A1MySQLWithTables(t *testing.T) *gorm.DB {
	t.Helper()
	db := openG6A1MySQL(t)
	for _, table := range []any{&WithdrawRequest{}, &RiderEarningRecord{}, &FeeConfig{}, &RiderConfig{}} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required migration missing for %T", table)
		}
	}
	return db
}

func assertG6A1MySQL84(t *testing.T, db *gorm.DB) {
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

func applyG6A1SQLFile(t *testing.T, db *gorm.DB, relativePath string) {
	t.Helper()
	file, err := os.Open(filepath.Clean(relativePath))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var statement strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		statement.WriteString(line)
		statement.WriteByte('\n')
		if !strings.HasSuffix(strings.TrimSpace(line), ";") {
			continue
		}
		query := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(statement.String()), ";"))
		statement.Reset()
		if query != "" {
			if err := db.Exec(query).Error; err != nil {
				t.Fatalf("apply %s: %v", relativePath, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func assertG6A1WithdrawSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	var columns int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='takeout_withdraw_requests'`).Scan(&columns).Error; err != nil {
		t.Fatal(err)
	}
	if columns != 10 {
		t.Fatalf("withdraw columns=%d want=10", columns)
	}
	for column, expected := range map[string]string{
		"withdraw_public_id": "char(26)", "rider_user_id": "bigint unsigned", "amount_cents": "bigint unsigned",
		"status": "varchar(24)", "note": "varchar(256)", "row_version": "bigint unsigned",
	} {
		var actual string
		if err := db.Raw(`SELECT column_type FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='takeout_withdraw_requests' AND column_name=?`, column).Scan(&actual).Error; err != nil {
			t.Fatal(err)
		}
		if actual != expected {
			t.Errorf("%s type=%s want=%s", column, actual, expected)
		}
	}
	var foreignKeys int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.key_column_usage WHERE table_schema=DATABASE() AND table_name='takeout_withdraw_requests' AND referenced_table_name IS NOT NULL`).Scan(&foreignKeys).Error; err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 0 {
		t.Fatalf("withdraw foreign keys=%d", foreignKeys)
	}
}

func createG6A1Earning(t *testing.T, db *gorm.DB, riderUserID uint64, earningID, orderID string, amount uint64) {
	t.Helper()
	now := time.Date(2026, 6, 22, 20, 0, 0, 0, time.UTC)
	value := RiderEarningRecord{
		EarningPublicID: earningID, OrderPublicID: orderID, RiderUserID: riderUserID,
		RiderFeeCents: amount, PlatformCutCents: 50, SettlementStatus: SettlementPending,
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&value).Error; err != nil {
		t.Fatal(err)
	}
}

func g6A1RiderIDs() []uint64 {
	return []uint64{g6WithdrawRider, g6WithdrawFaultRider, g6WithdrawRejectRider, 960010}
}

func cleanupG6A1Withdraw(t *testing.T, db *gorm.DB) {
	t.Helper()
	if db.Migrator().HasTable(&WithdrawRequest{}) {
		if err := db.Where("rider_user_id IN ?", g6A1RiderIDs()).Delete(&WithdrawRequest{}).Error; err != nil {
			t.Errorf("cleanup withdrawals: %v", err)
		}
	}
	if db.Migrator().HasTable(&RiderEarningRecord{}) {
		if err := db.Where("rider_user_id IN ?", g6A1RiderIDs()).Delete(&RiderEarningRecord{}).Error; err != nil {
			t.Errorf("cleanup earnings: %v", err)
		}
	}
}
