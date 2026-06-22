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

const (
	catalogMerchantOne   = "01J00000000000000000000001"
	catalogMerchantTwo   = "01J00000000000000000000002"
	catalogMerchantThree = "01J00000000000000000000003"
)

var catalogAcceptanceMerchants = []string{catalogMerchantOne, catalogMerchantTwo, catalogMerchantThree}

func Test00CatalogMigrationRollbackApplyAgainstRealMySQL84(t *testing.T) {
	db := openG2MySQL(t)
	assertG2MySQL84(t, db)
	cleanupG2Catalog(t, db)
	mediaConstraintBefore := readMediaReferenceConstraint(t, db)
	for _, table := range []any{&Dish{}, &DishCategory{}} {
		if db.Migrator().HasTable(table) {
			var count int64
			if err := db.Model(table).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("refusing to roll back non-empty %T rows=%d", table, count)
			}
		}
	}
	applyCatalogSQLFile(t, db, "../../../migrations/rollback/017_takeout_catalog.sql")
	if db.Migrator().HasTable(&Dish{}) || db.Migrator().HasTable(&DishCategory{}) {
		t.Fatal("rollback 017 left catalog tables behind")
	}
	applyCatalogSQLFile(t, db, "../../../migrations/017_takeout_catalog.sql")
	if !db.Migrator().HasTable(&Dish{}) || !db.Migrator().HasTable(&DishCategory{}) {
		t.Fatal("migration 017 did not create both catalog tables")
	}
	assertG2CatalogSchema(t, db)
	if mediaConstraintAfter := readMediaReferenceConstraint(t, db); mediaConstraintAfter != mediaConstraintBefore {
		t.Fatalf("migration 017 changed frozen media CHECK before=%s after=%s", mediaConstraintBefore, mediaConstraintAfter)
	}
	t.Log("migration_017_apply_rollback_apply=PASS tables=2 foreign_keys=0 media_018_unchanged=PASS")
}

func TestCatalogCategoryAndDishLifecycleAgainstRealMySQL84(t *testing.T) {
	db := openG2MySQLWithTables(t)
	cleanupG2Catalog(t, db)
	t.Cleanup(func() { cleanupG2Catalog(t, db) })
	now := time.Date(2026, 6, 22, 22, 0, 0, 0, time.UTC)
	service, _ := NewCatalogService(NewRepository(db), func() time.Time { return now })
	ctx := context.Background()

	category, err := service.CreateCategory(ctx, CreateCategoryInput{
		MerchantPublicID: catalogMerchantOne, Name: "Meals", SortOrder: 1,
	})
	if err != nil || category.RowVersion != 1 || !ValidULID(category.CategoryPublicID) {
		t.Fatalf("create category=%#v err=%v", category, err)
	}
	category, err = service.RenameCategory(ctx, category.CategoryPublicID, catalogMerchantOne, 1, "Hot Meals")
	if err != nil || category.Name != "Hot Meals" || category.RowVersion != 2 {
		t.Fatalf("rename category=%#v err=%v", category, err)
	}
	idempotentCategory, err := service.RenameCategory(ctx, category.CategoryPublicID, catalogMerchantOne, 1, "Hot Meals")
	if err != nil || idempotentCategory.RowVersion != 2 {
		t.Fatalf("idempotent rename=%#v err=%v", idempotentCategory, err)
	}
	if _, err := service.ReorderCategory(ctx, category.CategoryPublicID, catalogMerchantOne, 1, 5); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale category reorder returned %v", err)
	}
	category, err = service.ReorderCategory(ctx, category.CategoryPublicID, catalogMerchantOne, 2, 5)
	if err != nil || category.SortOrder != 5 || category.RowVersion != 3 {
		t.Fatalf("reorder category=%#v err=%v", category, err)
	}
	if _, err := service.ReorderCategory(ctx, category.CategoryPublicID, catalogMerchantTwo, 3, 6); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong merchant category update returned %v", err)
	}
	loadedCategory, err := service.FindCategory(ctx, category.CategoryPublicID)
	if err != nil || loadedCategory.ID != category.ID || loadedCategory.RowVersion != 3 {
		t.Fatalf("find category=%#v err=%v", loadedCategory, err)
	}

	imageID, _ := NewULID()
	dish, err := service.CreateDish(ctx, CreateDishInput{
		MerchantPublicID: catalogMerchantOne, CategoryPublicID: &category.CategoryPublicID,
		Name: "Rice", Description: "Campus rice", PriceCents: 1299,
		ImageAssetPublicID: &imageID, SortOrder: 1,
	})
	if err != nil || dish.Status != DishDraft || dish.RowVersion != 1 || dish.PriceCents != 1299 {
		t.Fatalf("create dish=%#v err=%v", dish, err)
	}
	updatedInput := UpdateDishInput{
		DishPublicID: dish.DishPublicID, MerchantPublicID: catalogMerchantOne,
		ExpectedStatus: DishDraft, ExpectedRowVersion: 1, CategoryPublicID: &category.CategoryPublicID,
		Name: "Rice Set", Description: "Rice and vegetables", PriceCents: 1599,
		ImageAssetPublicID: &imageID, SortOrder: 2,
	}
	dish, err = service.UpdateDish(ctx, updatedInput)
	if err != nil || dish.Name != "Rice Set" || dish.PriceCents != 1599 || dish.RowVersion != 2 {
		t.Fatalf("update dish=%#v err=%v", dish, err)
	}
	idempotentDish, err := service.UpdateDish(ctx, updatedInput)
	if err != nil || idempotentDish.RowVersion != 2 {
		t.Fatalf("idempotent update=%#v err=%v", idempotentDish, err)
	}
	staleInput := updatedInput
	staleInput.PriceCents = 1699
	if _, err := service.UpdateDish(ctx, staleInput); !errors.Is(err, ErrConcurrentUpdate) {
		t.Fatalf("stale dish update returned %v", err)
	}

	dish, err = service.PutOnShelf(ctx, dish.DishPublicID, catalogMerchantOne, DishDraft, dish.RowVersion)
	if err != nil || dish.Status != DishOnShelf || dish.RowVersion != 3 {
		t.Fatalf("put on shelf=%#v err=%v", dish, err)
	}
	idempotentDish, err = service.PutOnShelf(ctx, dish.DishPublicID, catalogMerchantOne, DishDraft, 2)
	if err != nil || idempotentDish.Status != DishOnShelf || idempotentDish.RowVersion != 3 {
		t.Fatalf("idempotent shelf=%#v err=%v", idempotentDish, err)
	}
	dish, err = service.MarkSoldOut(ctx, dish.DishPublicID, catalogMerchantOne, dish.RowVersion)
	if err != nil || dish.Status != DishSoldOut {
		t.Fatalf("sold out=%#v err=%v", dish, err)
	}
	dish, err = service.Restock(ctx, dish.DishPublicID, catalogMerchantOne, dish.RowVersion)
	if err != nil || dish.Status != DishOnShelf {
		t.Fatalf("restock=%#v err=%v", dish, err)
	}
	dish, err = service.TakeOffShelf(ctx, dish.DishPublicID, catalogMerchantOne, DishOnShelf, dish.RowVersion)
	if err != nil || dish.Status != DishOffShelf {
		t.Fatalf("take off shelf=%#v err=%v", dish, err)
	}
	dish, err = service.PutOnShelf(ctx, dish.DishPublicID, catalogMerchantOne, DishOffShelf, dish.RowVersion)
	if err != nil || dish.Status != DishOnShelf {
		t.Fatalf("put back on shelf=%#v err=%v", dish, err)
	}
	dish, err = service.TakeOffShelf(ctx, dish.DishPublicID, catalogMerchantOne, DishOnShelf, dish.RowVersion)
	if err != nil || dish.Status != DishOffShelf {
		t.Fatalf("second take off shelf=%#v err=%v", dish, err)
	}
	dish, err = service.RemoveDish(ctx, dish.DishPublicID, catalogMerchantOne, dish.RowVersion)
	if err != nil || dish.Status != DishRemoved || !dish.Status.Terminal() {
		t.Fatalf("remove=%#v err=%v", dish, err)
	}
	idempotentDish, err = service.RemoveDish(ctx, dish.DishPublicID, catalogMerchantOne, dish.RowVersion-1)
	if err != nil || idempotentDish.Status != DishRemoved || idempotentDish.RowVersion != dish.RowVersion {
		t.Fatalf("idempotent remove=%#v err=%v", idempotentDish, err)
	}
	loadedDish, err := service.FindDish(ctx, dish.DishPublicID)
	if err != nil || loadedDish.ID != dish.ID || loadedDish.Status != DishRemoved || loadedDish.PriceCents != 1599 {
		t.Fatalf("find dish=%#v err=%v", loadedDish, err)
	}
	if _, err := service.UpdateDish(ctx, UpdateDishInput{
		DishPublicID: dish.DishPublicID, MerchantPublicID: catalogMerchantOne,
		ExpectedStatus: DishRemoved, ExpectedRowVersion: dish.RowVersion, Name: "No", PriceCents: 1,
	}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("removed update returned %v", err)
	}

	second, err := service.CreateDish(ctx, CreateDishInput{MerchantPublicID: catalogMerchantTwo, Name: "Soup", PriceCents: 699})
	if err != nil {
		t.Fatal(err)
	}
	second, _ = service.PutOnShelf(ctx, second.DishPublicID, catalogMerchantTwo, DishDraft, second.RowVersion)
	second, _ = service.MarkSoldOut(ctx, second.DishPublicID, catalogMerchantTwo, second.RowVersion)
	second, err = service.TakeOffShelf(ctx, second.DishPublicID, catalogMerchantTwo, DishSoldOut, second.RowVersion)
	if err != nil || second.Status != DishOffShelf {
		t.Fatalf("sold-out takeoff=%#v err=%v", second, err)
	}
	if _, err := service.CreateDish(ctx, CreateDishInput{MerchantPublicID: catalogMerchantOne, Name: "Free", PriceCents: 0}); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("zero price returned %v", err)
	}
	badImage := "not-a-ulid"
	if _, err := service.CreateDish(ctx, CreateDishInput{
		MerchantPublicID: catalogMerchantOne, Name: "Bad Image", PriceCents: 1, ImageAssetPublicID: &badImage,
	}); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("invalid image returned %v", err)
	}
	t.Log("category_create_rename_reorder_find=PASS dish_integer_cents=PASS lifecycle_all_paths=PASS stale_CAS=REJECTED idempotency=PASS")
}

func TestCatalogCompetingDishCASAgainstRealMySQL84(t *testing.T) {
	db := openG2MySQLWithTables(t)
	cleanupG2Catalog(t, db)
	t.Cleanup(func() { cleanupG2Catalog(t, db) })
	service, _ := NewCatalogService(NewRepository(db), time.Now)
	ctx := context.Background()
	dish, err := service.CreateDish(ctx, CreateDishInput{MerchantPublicID: catalogMerchantThree, Name: "CAS Dish", PriceCents: 999})
	if err != nil {
		t.Fatal(err)
	}
	dish, err = service.PutOnShelf(ctx, dish.DishPublicID, catalogMerchantThree, DishDraft, dish.RowVersion)
	if err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		<-start
		_, transitionErr := service.MarkSoldOut(ctx, dish.DishPublicID, catalogMerchantThree, dish.RowVersion)
		results <- transitionErr
	}()
	go func() {
		defer wait.Done()
		<-start
		_, transitionErr := service.TakeOffShelf(ctx, dish.DishPublicID, catalogMerchantThree, DishOnShelf, dish.RowVersion)
		results <- transitionErr
	}()
	close(start)
	wait.Wait()
	close(results)

	var success, conflict int
	for result := range results {
		switch {
		case result == nil:
			success++
		case errors.Is(result, ErrConcurrentUpdate):
			conflict++
		default:
			t.Fatalf("unexpected competing CAS error %v", result)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("competing CAS success=%d conflict=%d", success, conflict)
	}
	loaded, err := service.FindDish(ctx, dish.DishPublicID)
	if err != nil || loaded.RowVersion != dish.RowVersion+1 || (loaded.Status != DishSoldOut && loaded.Status != DishOffShelf) {
		t.Fatalf("competing final dish=%#v err=%v", loaded, err)
	}
	t.Logf("competing_dish_CAS=PASS success=%d conflict=%d final_status=%s", success, conflict, loaded.Status)
}

func openG2MySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TAKEOUT_G2_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TAKEOUT_G2_MYSQL_DSN to run G-2 real MySQL acceptance")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func openG2MySQLWithTables(t *testing.T) *gorm.DB {
	t.Helper()
	db := openG2MySQL(t)
	if !db.Migrator().HasTable(&Dish{}) || !db.Migrator().HasTable(&DishCategory{}) {
		t.Fatal("migration 017 is not applied")
	}
	return db
}

func assertG2MySQL84(t *testing.T, db *gorm.DB) {
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

func applyCatalogSQLFile(t *testing.T, db *gorm.DB, relativePath string) {
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

func assertG2CatalogSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	assertG2TableColumns(t, db, "takeout_dish_categories", map[string]catalogColumnExpectation{
		"id":                 {"bigint unsigned", "NO", nil, "auto_increment"},
		"category_public_id": {"char(26)", "NO", nil, ""},
		"merchant_public_id": {"char(26)", "NO", nil, ""},
		"name":               {"varchar(64)", "NO", nil, ""},
		"sort_order":         {"int", "NO", stringPointerForCatalog("0"), ""},
		"row_version":        {"bigint unsigned", "NO", nil, ""},
		"created_at":         {"datetime", "NO", nil, ""},
		"updated_at":         {"datetime", "NO", nil, ""},
	})
	assertG2TableColumns(t, db, "takeout_dishes", map[string]catalogColumnExpectation{
		"id":                    {"bigint unsigned", "NO", nil, "auto_increment"},
		"dish_public_id":        {"char(26)", "NO", nil, ""},
		"merchant_public_id":    {"char(26)", "NO", nil, ""},
		"category_public_id":    {"char(26)", "YES", nil, ""},
		"name":                  {"varchar(128)", "NO", nil, ""},
		"description":           {"varchar(512)", "NO", stringPointerForCatalog(""), ""},
		"price_cents":           {"bigint unsigned", "NO", nil, ""},
		"image_asset_public_id": {"char(26)", "YES", nil, ""},
		"status":                {"varchar(24)", "NO", nil, ""},
		"sort_order":            {"int", "NO", stringPointerForCatalog("0"), ""},
		"row_version":           {"bigint unsigned", "NO", nil, ""},
		"created_at":            {"datetime", "NO", nil, ""},
		"updated_at":            {"datetime", "NO", nil, ""},
	})
	for table, expectedIndexes := range map[string]map[string]string{
		"takeout_dish_categories": {
			"uq_takeout_category_public_id": "category_public_id",
			"idx_takeout_category_merchant": "merchant_public_id",
		},
		"takeout_dishes": {
			"uq_takeout_dish_public_id": "dish_public_id",
			"idx_takeout_dish_merchant": "merchant_public_id",
			"idx_takeout_dish_status":   "status",
		},
	} {
		for index, expectedColumns := range expectedIndexes {
			var actual string
			if err := db.Raw(`SELECT GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',')
				FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name=? AND index_name=?`, table, index).Scan(&actual).Error; err != nil {
				t.Fatal(err)
			}
			if actual != expectedColumns {
				t.Errorf("%s index %s columns=%s want=%s", table, index, actual, expectedColumns)
			}
		}
	}
	var publicIDUnique int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema=DATABASE()
		AND ((table_name='takeout_dish_categories' AND index_name='uq_takeout_category_public_id' AND non_unique=0)
		OR (table_name='takeout_dishes' AND index_name='uq_takeout_dish_public_id' AND non_unique=0))`).Scan(&publicIDUnique).Error; err != nil {
		t.Fatal(err)
	}
	if publicIDUnique != 2 {
		t.Fatalf("unique public ID indexes=%d want=2", publicIDUnique)
	}
	var foreignKeys int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.key_column_usage WHERE table_schema=DATABASE()
		AND table_name IN ('takeout_dish_categories','takeout_dishes') AND referenced_table_name IS NOT NULL`).Scan(&foreignKeys).Error; err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 0 {
		t.Fatalf("catalog foreign keys=%d", foreignKeys)
	}
	var checkClause string
	if err := db.Raw(`SELECT cc.check_clause FROM information_schema.table_constraints tc
		JOIN information_schema.check_constraints cc ON cc.constraint_schema=tc.constraint_schema AND cc.constraint_name=tc.constraint_name
		WHERE tc.table_schema=DATABASE() AND tc.table_name='takeout_dishes' AND tc.constraint_name='chk_takeout_dish_status'`).Scan(&checkClause).Error; err != nil {
		t.Fatal(err)
	}
	for _, status := range []DishStatus{DishDraft, DishOnShelf, DishSoldOut, DishOffShelf, DishRemoved} {
		if !strings.Contains(checkClause, string(status)) {
			t.Errorf("dish status CHECK missing %s: %s", status, checkClause)
		}
	}
}

type catalogColumnExpectation struct {
	columnType, nullable string
	defaultValue         *string
	extra                string
}

func assertG2TableColumns(t *testing.T, db *gorm.DB, table string, expected map[string]catalogColumnExpectation) {
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
		column_default AS default_value, extra AS extra_value FROM information_schema.columns
		WHERE table_schema=DATABASE() AND table_name=? ORDER BY ordinal_position`, table).Scan(&columns).Error; err != nil {
		t.Fatal(err)
	}
	if len(columns) != len(expected) {
		t.Fatalf("%s column count=%d want=%d", table, len(columns), len(expected))
	}
	for _, actual := range columns {
		want, ok := expected[actual.Name]
		if !ok || actual.Type != want.columnType || actual.Nullable != want.nullable || actual.ExtraValue != want.extra ||
			!equalCatalogStringPointers(actual.DefaultValue, want.defaultValue) {
			t.Errorf("%s.%s got type=%s nullable=%s default=%v extra=%s want=%#v", table, actual.Name,
				actual.Type, actual.Nullable, actual.DefaultValue, actual.ExtraValue, want)
		}
	}
}

func readMediaReferenceConstraint(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var clause string
	if err := db.Raw(`SELECT cc.check_clause FROM information_schema.table_constraints tc
		JOIN information_schema.check_constraints cc ON cc.constraint_schema=tc.constraint_schema AND cc.constraint_name=tc.constraint_name
		WHERE tc.table_schema=DATABASE() AND tc.table_name='media_asset_references'
		AND tc.constraint_name='chk_media_reference_type'`).Scan(&clause).Error; err != nil {
		t.Fatal(err)
	}
	if clause == "" {
		t.Fatal("frozen media reference constraint missing")
	}
	return clause
}

func cleanupG2Catalog(t *testing.T, db *gorm.DB) {
	t.Helper()
	if db.Migrator().HasTable(&Dish{}) {
		if err := db.Where("merchant_public_id IN ?", catalogAcceptanceMerchants).Delete(&Dish{}).Error; err != nil {
			t.Errorf("cleanup dishes: %v", err)
		}
	}
	if db.Migrator().HasTable(&DishCategory{}) {
		if err := db.Where("merchant_public_id IN ?", catalogAcceptanceMerchants).Delete(&DishCategory{}).Error; err != nil {
			t.Errorf("cleanup categories: %v", err)
		}
	}
}

func stringPointerForCatalog(value string) *string { return &value }

func equalCatalogStringPointers(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func TestZZCatalogAcceptanceResidueAgainstRealMySQL84(t *testing.T) {
	db := openG2MySQLWithTables(t)
	cleanupG2Catalog(t, db)
	var categories, dishes int64
	if err := db.Model(&DishCategory{}).Count(&categories).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&Dish{}).Count(&dishes).Error; err != nil {
		t.Fatal(err)
	}
	if categories != 0 || dishes != 0 {
		t.Fatalf("catalog residue categories=%d dishes=%d", categories, dishes)
	}
	t.Log("catalog_residue categories=0 dishes=0")
}
