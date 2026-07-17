package navigation

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestExternalTakeoutRepository(t *testing.T) {
	repository := requireNavigationRepositoryMySQL(t)
	ctx := context.Background()

	systemCategory, found, err := repository.FindCategoryBySystemKey(ctx, SystemKeyExternalTakeoutMerchants)
	if err != nil || !found {
		t.Fatalf("find system category: found=%v err=%v", found, err)
	}
	if systemCategory.SystemKey == nil || *systemCategory.SystemKey != SystemKeyExternalTakeoutMerchants {
		t.Fatalf("unexpected system category key: %#v", systemCategory.SystemKey)
	}
	byID, found, err := repository.FindCategory(ctx, systemCategory.ID)
	if err != nil || !found || byID.ID != systemCategory.ID {
		t.Fatalf("find category by id: value=%+v found=%v err=%v", byID, found, err)
	}
	if _, found, err := repository.FindCategory(ctx, ^uint(0)); err != nil || found {
		t.Fatalf("missing category lookup: found=%v err=%v", found, err)
	}

	ordinary := Category{Name: "普通导航", Sort: 30, Enabled: true}
	if err := repository.CreateCategory(ctx, &ordinary); err != nil {
		t.Fatalf("create ordinary category: %v", err)
	}
	first := Item{
		CategoryID: ordinary.ID, Title: "第一项", Subtitle: "A", IconText: "A",
		AppID: "wx1234567890ABCDEF", Path: "pages/first/index", Color: "#112233",
		Sort: 10, Enabled: true, OperatingStatus: OperatingStatusOpen,
	}
	second := Item{
		CategoryID: ordinary.ID, Title: "第二项", Subtitle: "B", IconText: "B",
		AppID: "wx1234567890ABCDE0", Path: "pages/second/index", Color: "#445566",
		Sort: 10, Enabled: true, OperatingStatus: OperatingStatusOpen,
	}
	if err := repository.CreateItem(ctx, &first); err != nil {
		t.Fatalf("create first item: %v", err)
	}
	if err := repository.CreateItem(ctx, &second); err != nil {
		t.Fatalf("create second item: %v", err)
	}

	items, err := repository.ListItemsByCategory(ctx, ordinary.ID)
	if err != nil {
		t.Fatalf("list ordinary items: %v", err)
	}
	if len(items) != 2 || items[0].ID != first.ID || items[1].ID != second.ID {
		t.Fatalf("items are not ordered by sort,id: %+v", items)
	}
	duplicate, err := repository.DuplicateTarget(ctx, ordinary.ID, first.AppID, first.Path, 0)
	if err != nil || !duplicate {
		t.Fatalf("detect duplicate target: duplicate=%v err=%v", duplicate, err)
	}
	duplicate, err = repository.DuplicateTarget(ctx, ordinary.ID, first.AppID, first.Path, first.ID)
	if err != nil || duplicate {
		t.Fatalf("exclude current target: duplicate=%v err=%v", duplicate, err)
	}

	duplicateItem := first
	duplicateItem.ID = 0
	duplicateItem.Title = "重复目标"
	if err := repository.CreateItem(ctx, &duplicateItem); !errors.Is(err, ErrDuplicateTarget) {
		t.Fatalf("duplicate create error = %v, want ErrDuplicateTarget", err)
	}
	if err := repository.UpdateItem(ctx, second.ID, map[string]any{
		"app_id": first.AppID,
		"path":   first.Path,
	}); !errors.Is(err, ErrDuplicateTarget) {
		t.Fatalf("duplicate update error = %v, want ErrDuplicateTarget", err)
	}

	duplicateSystemKey := SystemKeyExternalTakeoutMerchants
	categoryConflict := Category{Name: "冲突分类", SystemKey: &duplicateSystemKey, Sort: 31, Enabled: true}
	if err := repository.CreateCategory(ctx, &categoryConflict); err == nil || errors.Is(err, ErrDuplicateTarget) {
		t.Fatalf("system key conflict must remain its original error: %v", err)
	}
	primaryConflict := second
	primaryConflict.ID = first.ID
	primaryConflict.AppID = "wx1234567890ABCDE1"
	primaryConflict.Path = "pages/primary/index"
	if err := repository.CreateItem(ctx, &primaryConflict); err == nil || errors.Is(err, ErrDuplicateTarget) {
		t.Fatalf("primary key conflict must remain its original error: %v", err)
	}

	if err := repository.UpdateItemInCategory(ctx, first.ID, systemCategory.ID, map[string]any{"title": "越权更新"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong-scope update error = %v, want ErrNotFound", err)
	}
	if err := repository.UpdateItemInCategory(ctx, first.ID, 0, map[string]any{"title": "零分类越权更新"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("zero-category scoped update error = %v, want ErrNotFound", err)
	}
	if err := repository.UpdateItemInCategory(ctx, first.ID, 0, map[string]any{"category_id": ordinary.ID}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("zero-category scoped no-op error = %v, want ErrNotFound", err)
	}
	if err := repository.DeleteItemInCategory(ctx, first.ID, systemCategory.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong-scope delete error = %v, want ErrNotFound", err)
	}
	if err := repository.DeleteItemInCategory(ctx, first.ID, 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("zero-category scoped delete error = %v, want ErrNotFound", err)
	}
	loadedFirst, found, err := repository.FindItem(ctx, first.ID)
	if err != nil || !found || loadedFirst.Title != first.Title {
		t.Fatalf("wrong-scope write changed item: value=%+v found=%v err=%v", loadedFirst, found, err)
	}

	systemItem := Item{
		CategoryID: systemCategory.ID, Title: "系统商家", Subtitle: "营业中", IconText: "餐",
		AppID: "wx1234567890ABCDE2", Path: "pages/store/index?id=1", Color: "#778899",
		Sort: 20, Enabled: true, OperatingStatus: OperatingStatusOpen,
	}
	if err := repository.CreateItem(ctx, &systemItem); err != nil {
		t.Fatalf("create system item: %v", err)
	}
	if err := repository.UpdateItemInCategory(ctx, systemItem.ID, systemCategory.ID, map[string]any{
		"title":            "系统商家（休息）",
		"enabled":          false,
		"operating_status": OperatingStatusResting,
		"category_id":      ordinary.ID,
	}); err != nil {
		t.Fatalf("scoped update system item: %v", err)
	}
	loadedSystem, found, err := repository.FindItem(ctx, systemItem.ID)
	if err != nil || !found {
		t.Fatalf("find scoped item: found=%v err=%v", found, err)
	}
	if loadedSystem.CategoryID != systemCategory.ID || loadedSystem.Title != "系统商家（休息）" ||
		loadedSystem.Enabled || loadedSystem.OperatingStatus != OperatingStatusResting {
		t.Fatalf("unexpected scoped update result: %+v", loadedSystem)
	}
	if err := repository.UpdateItemInCategory(ctx, systemItem.ID, systemCategory.ID, map[string]any{
		"title":            loadedSystem.Title,
		"enabled":          loadedSystem.Enabled,
		"operating_status": loadedSystem.OperatingStatus,
	}); err != nil {
		t.Fatalf("same-value scoped update: %v", err)
	}
	if err := repository.DeleteItemInCategory(ctx, systemItem.ID, systemCategory.ID); err != nil {
		t.Fatalf("delete scoped item: %v", err)
	}
	if err := repository.DeleteItemInCategory(ctx, systemItem.ID, systemCategory.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second scoped delete error = %v, want ErrNotFound", err)
	}

	if err := repository.UpdateItem(ctx, first.ID, map[string]any{"title": first.Title}); err != nil {
		t.Fatalf("same-value generic update: %v", err)
	}
	if err := repository.UpdateItem(ctx, ^uint(0), map[string]any{"title": "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing item update error = %v, want ErrNotFound", err)
	}
	if err := repository.DeleteItem(ctx, ^uint(0)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing item delete error = %v, want ErrNotFound", err)
	}
	if err := repository.UpdateCategory(ctx, ^uint(0), map[string]any{"name": "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing category update error = %v, want ErrNotFound", err)
	}
	if err := repository.DeleteCategory(ctx, ^uint(0)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing category delete error = %v, want ErrNotFound", err)
	}
}

func TestNavigationZeroValueUpdates(t *testing.T) {
	repository := requireNavigationRepositoryMySQL(t)
	service := NewService(repository)
	ctx := context.Background()

	forgedSystemKey := SystemKeyExternalTakeoutMerchants
	ordinary, err := service.CreateCategory(ctx, Category{
		ID: 999, Name: "可见普通分类", SystemKey: &forgedSystemKey, Sort: 40, Enabled: true,
	})
	if err != nil {
		t.Fatalf("create ordinary category through service: %v", err)
	}
	if ordinary.ID == 0 || ordinary.ID == 999 || ordinary.SystemKey != nil {
		t.Fatalf("service did not clear category identity/system key: %+v", ordinary)
	}

	oldCreateJSON := fmt.Sprintf(`{
		"id":999,"categoryId":%d,"title":"旧版商家","subtitle":"原副标题","iconText":"旧",
		"appId":"wx1234567890ABCDE3","path":"pages/legacy/index","color":"#ABCDEF","sort":8,"enabled":true
	}`, ordinary.ID)
	var oldCreate Item
	if err := json.Unmarshal([]byte(oldCreateJSON), &oldCreate); err != nil {
		t.Fatalf("decode old create JSON: %v", err)
	}
	created, err := service.CreateItem(ctx, oldCreate)
	if err != nil {
		t.Fatalf("create old JSON item: %v", err)
	}
	if created.ID == 0 || created.ID == 999 || created.OperatingStatus != OperatingStatusOpen {
		t.Fatalf("old create did not get server identity/open status: %+v", created)
	}

	explicitResting := oldCreate
	explicitResting.ID = 1234
	explicitResting.AppID = "wx1234567890ABCDE4"
	explicitResting.Path = "pages/forged-resting/index"
	explicitResting.OperatingStatus = OperatingStatusResting
	forcedOpen, err := service.CreateItem(ctx, explicitResting)
	if err != nil {
		t.Fatalf("create item with forged resting status: %v", err)
	}
	if forcedOpen.OperatingStatus != OperatingStatusOpen {
		t.Fatalf("generic create accepted forged operating status: %+v", forcedOpen)
	}
	if err := repository.DeleteItem(ctx, forcedOpen.ID); err != nil {
		t.Fatalf("delete guard test item: %v", err)
	}

	systemCategory, found, err := repository.FindCategoryBySystemKey(ctx, SystemKeyExternalTakeoutMerchants)
	if err != nil || !found {
		t.Fatalf("find system category: found=%v err=%v", found, err)
	}
	if err := repository.UpdateItemInCategory(ctx, created.ID, ordinary.ID, map[string]any{
		"subtitle":         "",
		"sort":             0,
		"enabled":          false,
		"operating_status": OperatingStatusResting,
		"category_id":      systemCategory.ID,
	}); err != nil {
		t.Fatalf("persist item zero values: %v", err)
	}
	loaded, found, err := repository.FindItem(ctx, created.ID)
	if err != nil || !found {
		t.Fatalf("find zero-value item: found=%v err=%v", found, err)
	}
	if loaded.CategoryID != ordinary.ID || loaded.Subtitle != "" || loaded.Sort != 0 || loaded.Enabled ||
		loaded.OperatingStatus != OperatingStatusResting {
		t.Fatalf("zero values/status did not persist: %+v", loaded)
	}

	adminItems, err := repository.ListItemsByCategory(ctx, ordinary.ID)
	if err != nil || len(adminItems) != 1 || adminItems[0].ID != created.ID {
		t.Fatalf("hidden item missing from admin list: items=%+v err=%v", adminItems, err)
	}
	publicTree, err := repository.Tree(ctx, true)
	if err != nil {
		t.Fatalf("load public tree: %v", err)
	}
	publicCategory := categoryFromTree(publicTree, ordinary.ID)
	if publicCategory == nil || len(publicCategory.Items) != 0 {
		t.Fatalf("hidden item leaked or category missing from public tree: %+v", publicCategory)
	}

	oldUpdateJSON := fmt.Sprintf(`{
		"id":%d,"categoryId":%d,"title":"旧版更新","subtitle":"","iconText":"旧",
		"appId":"wx1234567890ABCDE3","path":"pages/legacy/index","color":"#ABCDEF","sort":0,"enabled":false
	}`, created.ID, ordinary.ID)
	var oldUpdate Item
	if err := json.Unmarshal([]byte(oldUpdateJSON), &oldUpdate); err != nil {
		t.Fatalf("decode old update JSON: %v", err)
	}
	if err := service.UpdateItem(ctx, oldUpdate); err != nil {
		t.Fatalf("update old JSON item: %v", err)
	}
	if err := service.UpdateItem(ctx, oldUpdate); err != nil {
		t.Fatalf("same-value old JSON update: %v", err)
	}
	loaded, found, err = repository.FindItem(ctx, created.ID)
	if err != nil || !found || loaded.OperatingStatus != OperatingStatusResting || loaded.Title != "旧版更新" {
		t.Fatalf("old update changed operating status: value=%+v found=%v err=%v", loaded, found, err)
	}

	if err := repository.UpdateCategory(ctx, ordinary.ID, map[string]any{
		"name":       "仓储白名单分类",
		"sort":       0,
		"enabled":    true,
		"system_key": SystemKeyExternalTakeoutMerchants,
		"systemKey":  SystemKeyExternalTakeoutMerchants,
	}); err != nil {
		t.Fatalf("repository category whitelist update: %v", err)
	}
	afterRepositoryUpdate, found, err := repository.FindCategory(ctx, ordinary.ID)
	if err != nil || !found || afterRepositoryUpdate.SystemKey != nil || afterRepositoryUpdate.Sort != 0 || !afterRepositoryUpdate.Enabled {
		t.Fatalf("repository whitelist failed: value=%+v found=%v err=%v", afterRepositoryUpdate, found, err)
	}
	if err := repository.UpdateCategory(ctx, ordinary.ID, map[string]any{"system_key": forgedSystemKey}); err != nil {
		t.Fatalf("system-key-only update should be an existing-row no-op: %v", err)
	}
	if err := repository.UpdateCategory(ctx, ^uint(0), map[string]any{"system_key": forgedSystemKey}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing system-key-only update error = %v, want ErrNotFound", err)
	}

	if err := service.UpdateCategory(ctx, Category{
		ID: ordinary.ID, Name: "服务白名单分类", SystemKey: &forgedSystemKey, Sort: 0, Enabled: false,
	}); err != nil {
		t.Fatalf("service category whitelist update: %v", err)
	}
	if err := service.UpdateCategory(ctx, Category{
		ID: ordinary.ID, Name: "服务白名单分类", SystemKey: &forgedSystemKey, Sort: 0, Enabled: false,
	}); err != nil {
		t.Fatalf("same-value category update: %v", err)
	}
	closed, found, err := repository.FindCategory(ctx, ordinary.ID)
	if err != nil || !found || closed.SystemKey != nil || closed.Sort != 0 || closed.Enabled {
		t.Fatalf("service whitelist/zero update failed: value=%+v found=%v err=%v", closed, found, err)
	}
	publicTree, err = repository.Tree(ctx, true)
	if err != nil {
		t.Fatalf("reload public tree: %v", err)
	}
	if categoryFromTree(publicTree, ordinary.ID) != nil {
		t.Fatalf("disabled category leaked into public tree: %+v", publicTree)
	}
	adminTree, err := repository.Tree(ctx, false)
	if err != nil {
		t.Fatalf("load admin tree: %v", err)
	}
	adminCategory := categoryFromTree(adminTree, ordinary.ID)
	if adminCategory == nil || len(adminCategory.Items) != 1 || adminCategory.Items[0].ID != created.ID {
		t.Fatalf("disabled category/hidden item missing from admin tree: %+v", adminCategory)
	}
}

func TestMapNavigationWriteErrorOnlyMapsExactTargetIndex(t *testing.T) {
	target := fmt.Errorf("wrapped: %w", &mysqlDriver.MySQLError{
		Number:  1062,
		Message: "Duplicate entry for key 'navigation_items.uq_navigation_items_category_target'",
	})
	if got := mapNavigationWriteError(target); !errors.Is(got, ErrDuplicateTarget) {
		t.Fatalf("target index error = %v, want ErrDuplicateTarget", got)
	}

	for _, message := range []string{
		"Duplicate entry for key 'PRIMARY'",
		"Duplicate entry for key 'navigation_categories.uq_navigation_categories_system_key'",
		"Duplicate entry for key 'uq_navigation_items_category_target_shadow'",
		"Duplicate entry 'uq_navigation_items_category_target' for key 'navigation_categories.uq_navigation_categories_system_key'",
	} {
		err := &mysqlDriver.MySQLError{Number: 1062, Message: message}
		if got := mapNavigationWriteError(err); got != err {
			t.Fatalf("unrelated duplicate error %q was mapped to %v", message, got)
		}
	}
	nonDuplicate := &mysqlDriver.MySQLError{Number: 1048, Message: "column cannot be null"}
	if got := mapNavigationWriteError(nonDuplicate); got != nonDuplicate {
		t.Fatalf("non-1062 error was mapped to %v", got)
	}
}

func TestNavigationUpdateWhitelists(t *testing.T) {
	category := filterNavigationChanges(map[string]any{
		"name": "allowed", "sort": 0, "enabled": false,
		"system_key": "blocked", "systemKey": "blocked", "updated_at": time.Now(),
	}, categoryUpdateColumns)
	if len(category) != 3 || category["name"] != "allowed" || category["sort"] != 0 || category["enabled"] != false {
		t.Fatalf("unexpected category whitelist result: %#v", category)
	}
	if _, ok := category["system_key"]; ok {
		t.Fatalf("category whitelist accepted system_key: %#v", category)
	}

	generic := filterNavigationChanges(map[string]any{
		"category_id": uint(9), "title": "allowed", "operating_status": OperatingStatusResting,
		"system_key": "blocked", "created_at": time.Now(),
	}, itemUpdateColumns)
	if len(generic) != 3 || generic["category_id"] != uint(9) || generic["title"] != "allowed" ||
		generic["operating_status"] != OperatingStatusResting {
		t.Fatalf("unexpected generic item whitelist result: %#v", generic)
	}

	scoped := filterNavigationChanges(map[string]any{
		"category_id": uint(9), "title": "allowed", "operating_status": OperatingStatusResting,
		"system_key": "blocked",
	}, scopedItemUpdateColumns)
	if len(scoped) != 2 || scoped["title"] != "allowed" || scoped["operating_status"] != OperatingStatusResting {
		t.Fatalf("unexpected scoped item whitelist result: %#v", scoped)
	}
	if _, ok := scoped["category_id"]; ok {
		t.Fatalf("scoped whitelist accepted category_id: %#v", scoped)
	}
}

func categoryFromTree(values []Category, id uint) *Category {
	for index := range values {
		if values[index].ID == id {
			return &values[index]
		}
	}
	return nil
}

func requireNavigationRepositoryMySQL(t *testing.T) *GormRepository {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("NAVIGATION_DIRECTORY_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set NAVIGATION_DIRECTORY_MYSQL_DSN to run repository tests against an independent temporary MySQL 8.4 database")
	}

	serverConfig, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse NAVIGATION_DIRECTORY_MYSQL_DSN: %v", err)
	}
	serverConfig.DBName = ""
	serverConfig.MultiStatements = false
	serverConfig.ClientFoundRows = false
	serverDB, err := sql.Open("mysql", serverConfig.FormatDSN())
	if err != nil {
		t.Fatalf("open MySQL server connection: %v", err)
	}
	t.Cleanup(func() { _ = serverDB.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := serverDB.PingContext(ctx); err != nil {
		t.Fatalf("ping MySQL server: %v", err)
	}
	var version string
	if err := serverDB.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version); err != nil {
		t.Fatalf("read MySQL version: %v", err)
	}
	if !strings.HasPrefix(version, "8.4.") {
		t.Fatalf("NAVIGATION_DIRECTORY_MYSQL_DSN must point to MySQL 8.4, got %q", version)
	}

	databaseName := fmt.Sprintf("navigation_directory_%d", time.Now().UnixNano())
	createDatabase := "CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"
	if _, err := serverDB.ExecContext(ctx, createDatabase); err != nil {
		t.Fatalf("create independent temporary database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := serverDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`"); err != nil {
			t.Errorf("drop temporary database %s: %v", databaseName, err)
		}
	})

	databaseConfig, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse database DSN: %v", err)
	}
	databaseConfig.DBName = databaseName
	databaseConfig.MultiStatements = false
	databaseConfig.ClientFoundRows = false
	databaseConfig.ParseTime = true
	databaseDB, err := sql.Open("mysql", databaseConfig.FormatDSN())
	if err != nil {
		t.Fatalf("open temporary database: %v", err)
	}
	t.Cleanup(func() { _ = databaseDB.Close() })
	if err := databaseDB.PingContext(ctx); err != nil {
		t.Fatalf("ping temporary database: %v", err)
	}

	migrationConn, err := databaseDB.Conn(ctx)
	if err != nil {
		t.Fatalf("reserve migration connection: %v", err)
	}
	defer func() { _ = migrationConn.Close() }()
	applyNavigationMigration001(t, ctx, migrationConn)
	applyNavigationMigration031(t, ctx, migrationConn)

	fixedNow := time.Date(2026, time.July, 17, 12, 0, 0, 0, time.UTC)
	db, err := gorm.Open(gormMySQL.New(gormMySQL.Config{Conn: databaseDB}), &gorm.Config{
		NowFunc: func() time.Time { return fixedNow },
		Logger:  logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open GORM temporary database: %v", err)
	}
	return NewRepository(db)
}

func applyNavigationMigration001(t *testing.T, ctx context.Context, conn *sql.Conn) {
	t.Helper()
	script := readNavigationMigration(t, "001_init.sql")
	applyNavigationDelimitedScript(t, ctx, conn, script, "001")
}

func applyNavigationMigration031(t *testing.T, ctx context.Context, conn *sql.Conn) {
	t.Helper()
	script := readNavigationMigration(t, "031_external_takeout_merchant_directory.sql")
	applyNavigationDelimitedScript(t, ctx, conn, script, "031")
}

func applyNavigationDelimitedScript(t *testing.T, ctx context.Context, conn *sql.Conn, script, migration string) {
	t.Helper()
	scanner := bufio.NewScanner(strings.NewReader(script))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	delimiter := ";"
	var statement strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "DELIMITER ") {
			delimiter = strings.TrimSpace(trimmed[len("DELIMITER "):])
			if delimiter == "" {
				t.Fatalf("migration %s contains an empty delimiter", migration)
			}
			continue
		}
		statement.WriteString(line)
		statement.WriteByte('\n')
		if !strings.HasSuffix(trimmed, delimiter) {
			continue
		}

		sqlText := strings.TrimSpace(statement.String())
		sqlText = strings.TrimSpace(strings.TrimSuffix(sqlText, delimiter))
		statement.Reset()
		if sqlText == "" {
			continue
		}
		if _, err := conn.ExecContext(ctx, sqlText); err != nil {
			t.Fatalf("apply migration %s statement: %v", migration, err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan migration %s: %v", migration, err)
	}
	if strings.TrimSpace(statement.String()) != "" {
		t.Fatalf("migration %s ended with an unterminated statement", migration)
	}
}

func readNavigationMigration(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "migrations", name)
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(contents)
}
