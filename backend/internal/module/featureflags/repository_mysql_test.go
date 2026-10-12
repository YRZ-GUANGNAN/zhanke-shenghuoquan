package featureflags

import (
	"context"
	"encoding/json"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"os"
	"strings"
	"testing"
)

func TestGormTemporaryMySQLPersistenceAndAudit(t *testing.T) {
	dsn := os.Getenv("FEATURE_POLICY_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires an explicitly isolated local test database")
	}
	config, err := driver.ParseDSN(dsn)
	if err != nil || !strings.HasPrefix(config.DBName, "campus_feature_policy_test_") || (config.Addr != "127.0.0.1:33308" && config.Addr != "127.0.0.1:3306" && config.Addr != "localhost:3306") {
		t.Fatal("only an isolated local test database is allowed")
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
		t.Fatal("startup unexpectedly created schema")
	}
	for _, migration := range []string{"038_feature_policy.sql", "039_feature_policy_modules.sql"} {
		source, err := os.ReadFile("../../../migrations/" + migration)
		if err != nil {
			t.Fatal(err)
		}
		for _, statement := range strings.Split(string(source), ";") {
			if strings.TrimSpace(statement) != "" {
				if err := db.Exec(statement).Error; err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	t.Cleanup(func() { Configure(false) })
	if err := ConfigureStore(ctx, true, store); err != nil {
		t.Fatal(err)
	}
	initial := Snapshot()
	initial.Flags[KeyTakeout] = false
	initial.Flags[KeyFavorites] = false
	next, err := Update(42, UpdateInput{initial.Flags, "admin_only", initial.RowVersion})
	if err != nil {
		t.Fatal(err)
	}
	if err := ConfigureStore(ctx, true, NewGormStore(db)); err != nil {
		t.Fatal(err)
	}
	if Can(KeyTakeout) || Can(KeyFavorites) || Snapshot().RowVersion != next.RowVersion {
		t.Fatal("database restart lost flags")
	}
	var audits []AuditRow
	if err := db.Find(&audits).Error; err != nil {
		t.Fatal(err)
	}
	var audited Policy
	if len(audits) != 1 || audits[0].ActorID != 42 || json.Unmarshal([]byte(audits[0].AfterJSON), &audited) != nil || audited.Flags[KeyFavorites] {
		t.Fatal("audit does not reflect saved values")
	}
	other := clonePolicy(next)
	other.RowVersion++
	if err := store.Save(ctx, next, other); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, next, other); err != ErrVersionConflict {
		t.Fatalf("stale writer accepted: %v", err)
	}
	var count int64
	db.Model(&AuditRow{}).Count(&count)
	if count != 2 {
		t.Fatal("conflict wrote an audit record")
	}
	value, err := ReadPolicy(true); if err != nil { t.Fatal(err) }
	for _, key := range AllKeys {
		for _, enabled := range []bool{false, true} {
			flags := clonePolicy(value).Flags; flags[key] = enabled
			value, err = Update(42, UpdateInput{flags, "admin_only", value.RowVersion}); if err != nil { t.Fatal(err) }
			loaded, found, err := store.Load(ctx)
			if err != nil || !found || loaded.Flags[key] != enabled { t.Fatalf("%s=%v did not persist: %v", key, enabled, err) }
		}
	}
}
