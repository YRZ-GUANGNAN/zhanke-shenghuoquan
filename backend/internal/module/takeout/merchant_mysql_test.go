package takeout

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const g1OwnerBase = uint64(99_100_100)

func Test00MerchantMigrationRollbackApplyAgainstRealMySQL84(t *testing.T) {
	db := openG1MySQL(t)
	assertG1MySQL84(t, db)
	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	t.Logf("real_mysql_version=%s", version)
	if db.Migrator().HasTable(&Merchant{}) {
		var count int64
		if err := db.Model(&Merchant{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("refusing to roll back non-empty takeout_merchants: rows=%d", count)
		}
	}
	applyG1SQLFile(t, db, "../../../migrations/rollback/016_takeout_merchant.sql")
	if db.Migrator().HasTable(&Merchant{}) {
		t.Fatal("rollback left takeout_merchants behind")
	}
	applyG1SQLFile(t, db, "../../../migrations/016_takeout_merchant.sql")
	if !db.Migrator().HasTable(&Merchant{}) {
		t.Fatal("forward migration did not create takeout_merchants")
	}
	assertG1SchemaContract(t, db)
	t.Log("migration_apply_rollback_apply=PASS schema_contract=PASS cross_domain_fk=0")
}

func TestMerchantLifecycleCASIdempotencyAndLookupAgainstRealMySQL84(t *testing.T) {
	db := openG1MySQLWithTable(t)
	cleanupG1Owners(t, db)
	t.Cleanup(func() { cleanupG1Owners(t, db) })
	repository := NewRepository(db)
	now := time.Date(2026, 6, 22, 18, 0, 0, 0, time.UTC)
	service, _ := NewMerchantService(repository, func() time.Time { return now })
	ctx := context.Background()

	merchant, err := service.CreateMerchant(ctx, CreateMerchantInput{
		OwnerUserID: g1OwnerBase, Name: "G1 Acceptance Merchant", ContactPhone: "13800000000",
		Location: "North Campus", BusinessHours: "08:00-20:00",
	})
	if err != nil || merchant.Status != MerchantPendingReview || merchant.RowVersion != 1 {
		t.Fatalf("create failed merchant=%#v err=%v", merchant, err)
	}
	merchant, err = service.Approve(ctx, merchant.MerchantPublicID, 1)
	if err != nil || merchant.Status != MerchantActive || merchant.RowVersion != 2 {
		t.Fatalf("approve failed merchant=%#v err=%v", merchant, err)
	}
	idempotent, err := service.Approve(ctx, merchant.MerchantPublicID, 1)
	if err != nil || idempotent.Status != MerchantActive || idempotent.RowVersion != 2 {
		t.Fatalf("duplicate approve was not idempotent merchant=%#v err=%v", idempotent, err)
	}
	merchant, err = service.Rest(ctx, merchant.MerchantPublicID, g1OwnerBase, merchant.RowVersion)
	if err != nil || merchant.Status != MerchantResting || merchant.RowVersion != 3 {
		t.Fatalf("rest failed merchant=%#v err=%v", merchant, err)
	}
	repeatedRest, err := service.Rest(ctx, merchant.MerchantPublicID, g1OwnerBase, 2)
	if err != nil || repeatedRest.Status != MerchantResting || repeatedRest.RowVersion != 3 {
		t.Fatalf("duplicate rest was not idempotent merchant=%#v err=%v", repeatedRest, err)
	}
	merchant, err = service.OpenForBusiness(ctx, merchant.MerchantPublicID, g1OwnerBase, merchant.RowVersion)
	if err != nil || merchant.Status != MerchantActive || merchant.RowVersion != 4 {
		t.Fatalf("open failed merchant=%#v err=%v", merchant, err)
	}
	merchant, err = service.Suspend(ctx, merchant.MerchantPublicID, MerchantActive, merchant.RowVersion)
	if err != nil || merchant.Status != MerchantSuspended || merchant.RowVersion != 5 {
		t.Fatalf("suspend failed merchant=%#v err=%v", merchant, err)
	}
	merchant, err = service.Unsuspend(ctx, merchant.MerchantPublicID, merchant.RowVersion)
	if err != nil || merchant.Status != MerchantActive || merchant.RowVersion != 6 {
		t.Fatalf("unsuspend failed merchant=%#v err=%v", merchant, err)
	}
	if _, err := service.Rest(ctx, merchant.MerchantPublicID, g1OwnerBase, 5); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale row_version returned %v", err)
	}
	if _, err := service.Rest(ctx, merchant.MerchantPublicID, g1OwnerBase+99, merchant.RowVersion); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong owner returned %v", err)
	}
	loaded, err := service.FindMerchant(ctx, merchant.MerchantPublicID)
	if err != nil || loaded.ID != merchant.ID || loaded.Status != MerchantActive || loaded.RowVersion != 6 {
		t.Fatalf("find failed merchant=%#v err=%v", loaded, err)
	}

	rejected, err := service.CreateMerchant(ctx, CreateMerchantInput{OwnerUserID: g1OwnerBase + 1, Name: "Rejected Merchant"})
	if err != nil {
		t.Fatal(err)
	}
	rejected, err = service.Reject(ctx, rejected.MerchantPublicID, rejected.RowVersion)
	if err != nil || rejected.Status != MerchantRejected || !rejected.Status.Terminal() {
		t.Fatalf("reject failed merchant=%#v err=%v", rejected, err)
	}
	if _, err := service.Suspend(ctx, rejected.MerchantPublicID, MerchantRejected, rejected.RowVersion); err != ErrInvalidTransition {
		t.Fatalf("terminal transition returned %v", err)
	}

	resting, err := service.CreateMerchant(ctx, CreateMerchantInput{OwnerUserID: g1OwnerBase + 2, Name: "Resting Merchant"})
	if err != nil {
		t.Fatal(err)
	}
	resting, _ = service.Approve(ctx, resting.MerchantPublicID, resting.RowVersion)
	resting, _ = service.Rest(ctx, resting.MerchantPublicID, resting.OwnerUserID, resting.RowVersion)
	resting, err = service.Suspend(ctx, resting.MerchantPublicID, MerchantResting, resting.RowVersion)
	if err != nil || resting.Status != MerchantSuspended {
		t.Fatalf("resting suspend failed merchant=%#v err=%v", resting, err)
	}
	t.Log("lifecycle=pending_review->active->resting->active->suspended->active PASS; rejected_terminal=PASS; idempotency=PASS; stale_CAS=REJECTED")
}

func TestMerchantCompetingCASAgainstRealMySQL84(t *testing.T) {
	db := openG1MySQLWithTable(t)
	cleanupG1Owners(t, db)
	t.Cleanup(func() { cleanupG1Owners(t, db) })
	service, _ := NewMerchantService(NewRepository(db), time.Now)
	ctx := context.Background()
	merchant, err := service.CreateMerchant(ctx, CreateMerchantInput{OwnerUserID: g1OwnerBase + 3, Name: "CAS Merchant"})
	if err != nil {
		t.Fatal(err)
	}
	merchant, err = service.Approve(ctx, merchant.MerchantPublicID, merchant.RowVersion)
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errorsFound := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		_, transitionErr := service.Rest(ctx, merchant.MerchantPublicID, merchant.OwnerUserID, merchant.RowVersion)
		errorsFound <- transitionErr
	}()
	go func() {
		defer wait.Done()
		<-start
		_, transitionErr := service.Suspend(ctx, merchant.MerchantPublicID, MerchantActive, merchant.RowVersion)
		errorsFound <- transitionErr
	}()
	close(start)
	wait.Wait()
	close(errorsFound)

	var success, conflict int
	for transitionErr := range errorsFound {
		switch {
		case transitionErr == nil:
			success++
		case errors.Is(transitionErr, ErrConcurrentUpdate):
			conflict++
		default:
			t.Fatalf("unexpected competing CAS error %v", transitionErr)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("competing CAS success=%d conflict=%d", success, conflict)
	}
	loaded, err := service.FindMerchant(ctx, merchant.MerchantPublicID)
	if err != nil || loaded.RowVersion != merchant.RowVersion+1 || (loaded.Status != MerchantResting && loaded.Status != MerchantSuspended) {
		t.Fatalf("unexpected final CAS state merchant=%#v err=%v", loaded, err)
	}
	t.Logf("competing_CAS=PASS success=%d conflict=%d final_status=%s", success, conflict, loaded.Status)
}

func openG1MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TAKEOUT_G1_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TAKEOUT_G1_MYSQL_DSN to run G-1 real MySQL acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func openG1MySQLWithTable(t *testing.T) *gorm.DB {
	t.Helper()
	db := openG1MySQL(t)
	if !db.Migrator().HasTable(&Merchant{}) {
		t.Fatal("migration 016 is not applied")
	}
	return db
}

func assertG1MySQL84(t *testing.T, db *gorm.DB) {
	t.Helper()
	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.4.") {
		t.Fatalf("expected MySQL 8.4, got %s", version)
	}
}

func applyG1SQLFile(t *testing.T, db *gorm.DB, relativePath string) {
	t.Helper()
	path := filepath.Clean(relativePath)
	file, err := os.Open(path)
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
				t.Fatalf("apply %s: %v", path, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
}

func assertG1SchemaContract(t *testing.T, db *gorm.DB) {
	t.Helper()
	type column struct {
		Name         string  `gorm:"column:name"`
		Type         string  `gorm:"column:type"`
		Nullable     string  `gorm:"column:nullable"`
		DefaultValue *string `gorm:"column:default_value"`
		ExtraValue   string  `gorm:"column:extra_value"`
	}
	var columns []column
	if err := db.Raw(`SELECT column_name AS name, column_type AS type, is_nullable AS nullable,
		column_default AS default_value, extra AS extra_value
		FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='takeout_merchants'
		ORDER BY ordinal_position`).Scan(&columns).Error; err != nil {
		t.Fatal(err)
	}
	expected := map[string]struct{ columnType, nullable string }{
		"id": {"bigint unsigned", "NO"}, "merchant_public_id": {"char(26)", "NO"},
		"owner_user_id": {"bigint unsigned", "NO"}, "name": {"varchar(128)", "NO"},
		"contact_phone": {"varchar(32)", "NO"}, "location": {"varchar(128)", "NO"},
		"business_hours": {"varchar(64)", "NO"}, "logo_asset_public_id": {"char(26)", "YES"},
		"status": {"varchar(24)", "NO"}, "row_version": {"bigint unsigned", "NO"},
		"created_at": {"datetime", "NO"}, "updated_at": {"datetime", "NO"},
	}
	if len(columns) != len(expected) {
		t.Fatalf("column count=%d want=%d", len(columns), len(expected))
	}
	for _, value := range columns {
		want, ok := expected[value.Name]
		if !ok || value.Type != want.columnType || value.Nullable != want.nullable {
			t.Errorf("column %s type=%s nullable=%s", value.Name, value.Type, value.Nullable)
		}
		if value.Name == "id" && value.ExtraValue != "auto_increment" {
			t.Errorf("id extra=%q", value.ExtraValue)
		}
		if value.Name == "contact_phone" || value.Name == "location" || value.Name == "business_hours" {
			if value.DefaultValue == nil || *value.DefaultValue != "" {
				t.Errorf("column %s default=%v want empty string", value.Name, value.DefaultValue)
			}
		} else if value.DefaultValue != nil {
			t.Errorf("column %s unexpected default=%q", value.Name, *value.DefaultValue)
		}
	}
	var foreignKeys int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.key_column_usage
		WHERE table_schema=DATABASE() AND table_name='takeout_merchants' AND referenced_table_name IS NOT NULL`).Scan(&foreignKeys).Error; err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 0 {
		t.Fatalf("cross-domain foreign keys=%d", foreignKeys)
	}
	for index, columns := range map[string]string{
		"uq_takeout_merchant_public_id": "merchant_public_id",
		"idx_takeout_merchant_owner":    "owner_user_id",
		"idx_takeout_merchant_status":   "status",
	} {
		var actual string
		if err := db.Raw(`SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') FROM information_schema.statistics
			WHERE table_schema=DATABASE() AND table_name='takeout_merchants' AND index_name=?`, index).Scan(&actual).Error; err != nil {
			t.Fatal(err)
		}
		if actual != columns {
			t.Errorf("index %s columns=%q want=%q", index, actual, columns)
		}
	}
	var publicIDNonUnique int
	if err := db.Raw(`SELECT non_unique FROM information_schema.statistics
		WHERE table_schema=DATABASE() AND table_name='takeout_merchants' AND index_name='uq_takeout_merchant_public_id' LIMIT 1`).Scan(&publicIDNonUnique).Error; err != nil {
		t.Fatal(err)
	}
	if publicIDNonUnique != 0 {
		t.Fatal("merchant_public_id index is not unique")
	}
	var statusChecks int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.table_constraints
		WHERE table_schema=DATABASE() AND table_name='takeout_merchants' AND constraint_type='CHECK' AND constraint_name='chk_takeout_merchant_status'`).Scan(&statusChecks).Error; err != nil {
		t.Fatal(err)
	}
	if statusChecks != 1 {
		t.Fatalf("status check constraints=%d", statusChecks)
	}
}

func cleanupG1Owners(t *testing.T, db *gorm.DB) {
	t.Helper()
	if !db.Migrator().HasTable(&Merchant{}) {
		return
	}
	if err := db.Where("owner_user_id BETWEEN ? AND ?", g1OwnerBase, g1OwnerBase+999).Delete(&Merchant{}).Error; err != nil {
		t.Errorf("cleanup merchants: %v", err)
		return
	}
	var residue int64
	if err := db.Model(&Merchant{}).Where("owner_user_id BETWEEN ? AND ?", g1OwnerBase, g1OwnerBase+999).Count(&residue).Error; err != nil || residue != 0 {
		t.Errorf("merchant residue=%d err=%v", residue, err)
	}
}

func TestMerchantMigrationFileNameIsReservedForG1(t *testing.T) {
	if _, err := os.Stat("../../../migrations/016_takeout_merchant.sql"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("../../../migrations/rollback/016_takeout_merchant.sql"); err != nil {
		t.Fatal(err)
	}
	if files, err := filepath.Glob("../../../migrations/016_*.sql"); err != nil || len(files) != 1 {
		t.Fatalf("migration 016 files=%v err=%v", files, err)
	}
}

func TestZZMerchantAcceptanceResidueAgainstRealMySQL84(t *testing.T) {
	db := openG1MySQLWithTable(t)
	cleanupG1Owners(t, db)
	var count int64
	if err := db.Model(&Merchant{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("takeout_merchants residue=%d", count)
	}
	t.Log("takeout_merchants_residue=0")
}
