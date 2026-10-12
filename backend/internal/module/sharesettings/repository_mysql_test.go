package sharesettings

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestTemporaryMySQLPersistenceCASAndAuditRollback(t *testing.T) {
	dsn := os.Getenv("SHARE_SETTINGS_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires an explicitly isolated local database")
	}
	config, err := driver.ParseDSN(dsn)
	if err != nil || !strings.HasPrefix(config.DBName, "campus_share_test_") || config.Addr != "127.0.0.1:33308" {
		t.Fatal("only a dedicated local share test database is permitted")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	ctx := context.Background()
	store := NewGormStore(db)
	if err := store.Ensure(ctx); err == nil {
		t.Fatal("test database must initially be empty")
	}
	source, err := os.ReadFile("../../../migrations/040_share_settings.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range strings.Split(string(source), ";") {
		if strings.TrimSpace(query) != "" {
			if err := db.Exec(query).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := store.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	initial, err := store.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	changed := initial.Cards
	changed.HomeTitle = "保存后的标题"
	changed.PreferContentImage = false
	next, err := store.Save(ctx, 42, UpdateInput{changed, initial.RowVersion})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := NewGormStore(db).Load(ctx)
	if err != nil || loaded.HomeTitle != changed.HomeTitle || loaded.PreferContentImage || loaded.RowVersion != next.RowVersion {
		t.Fatalf("persistent fields mismatch: %+v %v", loaded, err)
	}
	var audits []AuditRow
	db.Find(&audits)
	if len(audits) != 1 || audits[0].ActorID != 42 || !strings.Contains(audits[0].AfterJSON, changed.HomeTitle) {
		t.Fatal("audit missing")
	}
	var group sync.WaitGroup
	outcomes := make(chan error, 2)
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := store.Save(ctx, 42, UpdateInput{changed, next.RowVersion})
			outcomes <- err
		}()
	}
	group.Wait()
	close(outcomes)
	wins, conflicts := 0, 0
	for err := range outcomes {
		if err == nil {
			wins++
		} else if errors.Is(err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatalf("concurrent CAS: wins=%d conflicts=%d", wins, conflicts)
	}
	beforeFailure, _ := store.Load(ctx)
	if err := db.Exec("RENAME TABLE share_settings_audit_logs TO share_settings_audit_logs_unavailable").Error; err != nil {
		t.Fatal(err)
	}
	changed.HomeTitle = "必须回滚"
	if _, err := store.Save(ctx, 42, UpdateInput{changed, beforeFailure.RowVersion}); err == nil {
		t.Fatal("missing audit table accepted")
	}
	afterFailure, _ := store.Load(ctx)
	if beforeFailure != afterFailure {
		t.Fatal("failed audit did not roll back configuration")
	}
	if err := db.Exec("RENAME TABLE share_settings_audit_logs_unavailable TO share_settings_audit_logs").Error; err != nil {
		t.Fatal(err)
	}
}
