package featureflags

import (
	"context"
	"encoding/json"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"os"
	"reflect"
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
	for _, migration := range []string{"038_feature_policy.sql", "039_feature_policy_modules.sql", "041_feature_policy_presentation.sql"} {
		if migration == "041_feature_policy_presentation.sql" {
			if err := store.Ensure(ctx); err == nil {
				t.Fatal("missing presentation migration accepted")
			}
			if err := db.Exec(`INSERT INTO feature_policy_configs (id, forum, posting_mode, row_version, updated_by, updated_at, created_at, extra_flags) VALUES (1, FALSE, 'all_users', 7, 42, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6), '{"search":true,"likes":true,"favorites":true,"notifications":true,"identity_application":true,"publish":true,"secondhand_comments":true}')`).Error; err != nil {
				t.Fatal(err)
			}
		}
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
	if initial.PresentationMode != PresentationStandard || initial.RowVersion != 7 || initial.Flags[KeyForum] || initial.PostingMode != "all_users" || initial.UpdatedBy != 42 {
		t.Fatal("additive migration modified original policy")
	}
	initial.Flags[KeyTakeout] = false
	initial.Flags[KeyFavorites] = false
	next, err := Update(42, UpdateInput{initial.Flags, "admin_only", initial.RowVersion, nil})
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
	value, err := ReadPolicy(true)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range AllKeys {
		for _, enabled := range []bool{false, true} {
			flags := clonePolicy(value).Flags
			flags[key] = enabled
			value, err = Update(42, UpdateInput{flags, "admin_only", value.RowVersion, nil})
			if err != nil {
				t.Fatal(err)
			}
			loaded, found, err := store.Load(ctx)
			if err != nil || !found || loaded.Flags[key] != enabled {
				t.Fatalf("%s=%v did not persist: %v", key, enabled, err)
			}
		}
	}
	value.Flags[KeyMessages] = false
	value.Flags[KeyPost] = false
	original, err := Update(42, UpdateInput{value.Flags, "all_users", value.RowVersion, nil})
	if err != nil {
		t.Fatal(err)
	}
	info, err := Update(42, UpdateInput{original.Flags, original.PostingMode, original.RowVersion, modePointer(PresentationCampusInfo)})
	if err != nil {
		t.Fatal(err)
	}
	if err := ConfigureStore(ctx, true, NewGormStore(db)); err != nil {
		t.Fatal(err)
	}
	if Snapshot().PresentationMode != PresentationCampusInfo || Can(KeySecondhand) {
		t.Fatal("mode lost after database reload")
	}
	loaded, found, err := store.Load(ctx)
	if err != nil || !found || !reflect.DeepEqual(loaded.Flags, original.Flags) || loaded.PostingMode != original.PostingMode {
		t.Fatal("stored original settings overwritten")
	}
	restored, err := Update(42, UpdateInput{loaded.Flags, loaded.PostingMode, info.RowVersion, modePointer(PresentationStandard)})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.Flags, original.Flags) || Can(KeyPost) || !Can(KeySecondhand) {
		t.Fatal("mode round trip restored wrong values")
	}
	// Audit failure must roll back the mode and version in the same transaction.
	if err := db.Exec("RENAME TABLE feature_policy_audit_logs TO feature_policy_audit_logs_test_hidden").Error; err != nil {
		t.Fatal(err)
	}
	_, saveErr := Update(42, UpdateInput{restored.Flags, restored.PostingMode, restored.RowVersion, modePointer(PresentationCampusInfo)})
	if err := db.Exec("RENAME TABLE feature_policy_audit_logs_test_hidden TO feature_policy_audit_logs").Error; err != nil {
		t.Fatal(err)
	}
	if saveErr == nil {
		t.Fatal("mode changed without audit")
	}
	loaded, _, err = store.Load(ctx)
	if err != nil || loaded.RowVersion != restored.RowVersion || loaded.PresentationMode != PresentationStandard || !reflect.DeepEqual(loaded.Flags, restored.Flags) {
		t.Fatal("audit failure did not roll back policy")
	}
	var last AuditRow
	if err := db.Last(&last).Error; err != nil {
		t.Fatal(err)
	}
	var beforeMode, afterMode Policy
	if json.Unmarshal([]byte(last.BeforeJSON), &beforeMode) != nil || json.Unmarshal([]byte(last.AfterJSON), &afterMode) != nil || beforeMode.PresentationMode != PresentationCampusInfo || afterMode.PresentationMode != PresentationStandard {
		t.Fatal("audit omitted mode transition")
	}
}
