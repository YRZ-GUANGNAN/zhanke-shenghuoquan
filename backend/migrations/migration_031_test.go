package migrations

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	gormMySQL "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const migration031File = "031_external_takeout_merchant_directory.sql"

var migration031DatabaseSequence uint64

func TestMigration031SourceContract(t *testing.T) {
	source, err := os.ReadFile(migration031File)
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, token := range []string{
		"system_key", "external_takeout_merchants", "operating_status",
		"uq_navigation_categories_system_key",
		"chk_navigation_items_operating_status",
		"uq_navigation_items_category_target",
		"SIGNAL SQLSTATE '45000'",
		"GET_LOCK", "RELEASE_LOCK", "SQL SECURITY INVOKER",
		"CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin",
		"character_set_name='utf8mb4'", "collation_name='utf8mb4_0900_bin'",
		"HEX(column_default)=HEX('open')",
		"CONCAT(CHAR(92),CHAR(39))",
	} {
		if !strings.Contains(text, token) {
			t.Fatalf("migration 031 missing %q", token)
		}
	}
	if _, err := os.Stat(filepath.Join("rollback", migration031File)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("migration 031 must stay forward-only, rollback stat err=%v", err)
	}
}

func TestMigration031ClaimsUniqueSafeCategory(t *testing.T) {
	for _, tc := range []struct {
		name     string
		withItem bool
	}{
		{name: "empty"},
		{name: "safe_item", withItem: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := newMigration031Database(t)
			createMigration031BaseSchema(t, db, nil)
			mustExec031(t, db, `INSERT INTO navigation_categories
				(id,name,sort,enabled,created_at,updated_at)
				VALUES (7101,'外卖商家',731,FALSE,NOW(3),NOW(3))`)
			if tc.withItem {
				mustExec031(t, db, `INSERT INTO navigation_items
					(id,category_id,title,subtitle,icon_text,app_id,path,color,sort,enabled,created_at,updated_at)
					VALUES (7201,7101,'校内餐厅','','餐','wx1234567890ABCDEF','pages/store/index','#12AbEF',10000,FALSE,NOW(3),NOW(3))`)
				mustExec031(t, db, `INSERT INTO navigation_items
					(id,category_id,title,subtitle,icon_text,app_id,path,color,sort,enabled,created_at,updated_at)
					VALUES (7202,7101,?,?,?,?,?,?,0,TRUE,NOW(3),NOW(3))`,
					strings.Repeat("界", 64), strings.Repeat("介", 120), strings.Repeat("餐", 12),
					"wxABCDEF1234567890", "pages/"+strings.Repeat("a", 234), "#ABCDEF")
			}

			applyMigration031Successfully(t, db)
			applyMigration031Successfully(t, db)

			var id, sort int64
			var enabled bool
			var systemKey string
			if err := db.QueryRow(`SELECT id,system_key,sort,enabled FROM navigation_categories WHERE name='外卖商家'`).Scan(&id, &systemKey, &sort, &enabled); err != nil {
				t.Fatal(err)
			}
			if id != 7101 || systemKey != "external_takeout_merchants" || sort != 731 || enabled {
				t.Fatalf("claimed category=(id=%d key=%q sort=%d enabled=%v), want original id/sort/disabled", id, systemKey, sort, enabled)
			}
			assertQueryCount031(t, db, 1, `SELECT COUNT(*) FROM navigation_categories WHERE system_key='external_takeout_merchants'`)
			if tc.withItem {
				assertQueryCount031(t, db, 1, `SELECT COUNT(*) FROM navigation_items WHERE id=7201 AND subtitle='' AND enabled=FALSE AND operating_status='open'`)
				assertQueryCount031(t, db, 1, `SELECT COUNT(*) FROM navigation_items WHERE id=7202 AND enabled=TRUE AND operating_status='open'`)
			}
		})
	}
}

func TestMigration031FreshInsertDefaultsEnabled(t *testing.T) {
	db, _ := newMigration031Database(t)
	createMigration031BaseSchema(t, db, nil)
	mustExec031(t, db, `INSERT INTO navigation_categories(id,name,sort,enabled) VALUES (7301,'其他分类',9,FALSE)`)

	applyMigration031Successfully(t, db)
	applyMigration031Successfully(t, db)

	var sort int64
	var enabled bool
	if err := db.QueryRow(`SELECT sort,enabled FROM navigation_categories WHERE system_key='external_takeout_merchants'`).Scan(&sort, &enabled); err != nil {
		t.Fatal(err)
	}
	if sort != 20 || !enabled {
		t.Fatalf("fresh system category sort=%d enabled=%v, want sort=20 enabled=true", sort, enabled)
	}
	assertQueryCount031(t, db, 1, `SELECT COUNT(*) FROM navigation_categories WHERE system_key='external_takeout_merchants'`)
	assertQueryCount031(t, db, 1, `SELECT COUNT(*) FROM navigation_categories WHERE id=7301 AND enabled=FALSE`)
}

func TestMigration031RejectsWrongBaseSchemaBeforeDDL(t *testing.T) {
	tests := []struct {
		name       string
		table      string
		column     string
		definition string
	}{
		{name: "category_id_signed", table: "navigation_categories", column: "id", definition: "id BIGINT AUTO_INCREMENT PRIMARY KEY"},
		{name: "category_id_not_auto_increment", table: "navigation_categories", column: "id", definition: "id BIGINT UNSIGNED PRIMARY KEY"},
		{name: "category_name_length", table: "navigation_categories", column: "name", definition: "name VARCHAR(63) NOT NULL DEFAULT ''"},
		{name: "category_sort_type", table: "navigation_categories", column: "sort", definition: "sort INT NOT NULL DEFAULT 0"},
		{name: "category_sort_unsigned", table: "navigation_categories", column: "sort", definition: "sort BIGINT UNSIGNED NOT NULL DEFAULT 0"},
		{name: "category_enabled_type", table: "navigation_categories", column: "enabled", definition: "enabled TINYINT UNSIGNED NOT NULL DEFAULT 1"},
		{name: "category_created_precision", table: "navigation_categories", column: "created_at", definition: "created_at DATETIME(6) NULL"},
		{name: "category_updated_precision", table: "navigation_categories", column: "updated_at", definition: "updated_at DATETIME(6) NULL"},
		{name: "item_id_signed", table: "navigation_items", column: "id", definition: "id BIGINT AUTO_INCREMENT PRIMARY KEY"},
		{name: "item_id_not_auto_increment", table: "navigation_items", column: "id", definition: "id BIGINT UNSIGNED PRIMARY KEY"},
		{name: "item_category_id_signed", table: "navigation_items", column: "category_id", definition: "category_id BIGINT NOT NULL"},
		{name: "item_title_length", table: "navigation_items", column: "title", definition: "title VARCHAR(63) NOT NULL DEFAULT ''"},
		{name: "item_subtitle_length", table: "navigation_items", column: "subtitle", definition: "subtitle VARCHAR(119) NOT NULL DEFAULT ''"},
		{name: "item_icon_text_length", table: "navigation_items", column: "icon_text", definition: "icon_text VARCHAR(11) NOT NULL DEFAULT ''"},
		{name: "item_app_id_length", table: "navigation_items", column: "app_id", definition: "app_id VARCHAR(79) NOT NULL"},
		{name: "item_path_length", table: "navigation_items", column: "path", definition: "path VARCHAR(239) NOT NULL"},
		{name: "item_color_length", table: "navigation_items", column: "color", definition: "color VARCHAR(23) NOT NULL DEFAULT ''"},
		{name: "item_sort_type", table: "navigation_items", column: "sort", definition: "sort INT NOT NULL DEFAULT 0"},
		{name: "item_sort_unsigned", table: "navigation_items", column: "sort", definition: "sort BIGINT UNSIGNED NOT NULL DEFAULT 0"},
		{name: "item_enabled_type", table: "navigation_items", column: "enabled", definition: "enabled TINYINT UNSIGNED NOT NULL DEFAULT 1"},
		{name: "item_created_precision", table: "navigation_items", column: "created_at", definition: "created_at DATETIME(6) NULL"},
		{name: "item_updated_precision", table: "navigation_items", column: "updated_at", definition: "updated_at DATETIME(6) NULL"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := newMigration031Database(t)
			createMigration031BaseSchema(t, db, map[string]string{tc.table + "." + tc.column: tc.definition})
			err := executeMigration031(t, db)
			assertMigration031Error(t, err, "migration 031 base schema mismatch")
			assertMigration031ObjectCount(t, db, 0)
		})
	}
}

func TestMigration031RejectsAmbiguousCategoryBeforeDDL(t *testing.T) {
	db, _ := newMigration031Database(t)
	createMigration031BaseSchema(t, db, nil)
	mustExec031(t, db, `INSERT INTO navigation_categories(name,sort,enabled) VALUES ('外卖商家',1,TRUE),('外卖商家',2,FALSE)`)

	err := executeMigration031(t, db)
	assertMigration031Error(t, err, "migration 031 ambiguous category")
	assertMigration031ObjectCount(t, db, 0)
}

func TestMigration031RejectsUnsafeLegacyCategoryBeforeDDL(t *testing.T) {
	tests := []struct {
		name   string
		column string
		value  any
	}{
		{name: "title_leading_tab", column: "title", value: "\t商家"},
		{name: "title_trailing_tab", column: "title", value: "商家\t"},
		{name: "subtitle_leading_cr", column: "subtitle", value: "\r简介"},
		{name: "subtitle_trailing_lf", column: "subtitle", value: "简介\n"},
		{name: "icon_control_inside", column: "icon_text", value: "餐\x00馆"},
		{name: "title_leading_nbsp", column: "title", value: "\u00a0商家"},
		{name: "title_trailing_nbsp", column: "title", value: "商家\u00a0"},
		{name: "path_leading_fullwidth_space", column: "path", value: "\u3000pages/store/index"},
		{name: "path_trailing_fullwidth_space", column: "path", value: "pages/store/index\u3000"},
		{name: "title_empty", column: "title", value: ""},
		{name: "icon_empty", column: "icon_text", value: ""},
		{name: "appid_invalid", column: "app_id", value: "WX1234567890ABCDEF"},
		{name: "path_empty", column: "path", value: ""},
		{name: "path_leading_slash", column: "path", value: "/pages/store/index"},
		{name: "path_double_slash", column: "path", value: "pages//store/index"},
		{name: "path_scheme", column: "path", value: "https://example.test/path"},
		{name: "path_backslash", column: "path", value: `pages\store\index`},
		{name: "path_parent_segment", column: "path", value: "pages/../index"},
		{name: "path_fragment", column: "path", value: "pages/store/index#menu"},
		{name: "path_percent_encoding", column: "path", value: "pages/store/%69ndex"},
		{name: "path_query", column: "path", value: "pages/store/index?store=1"},
		{name: "path_control_inside", column: "path", value: "pages/store/\nindex"},
		{name: "color_invalid", column: "color", value: "blue"},
		{name: "sort_negative", column: "sort", value: int64(-1)},
		{name: "sort_too_large", column: "sort", value: int64(10001)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := newMigration031Database(t)
			createMigration031BaseSchema(t, db, nil)
			mustExec031(t, db, `INSERT INTO navigation_categories(id,name,sort,enabled) VALUES (7401,'外卖商家',20,TRUE)`)
			values := map[string]any{
				"title": "安全商家", "subtitle": "安全简介", "icon_text": "餐",
				"app_id": "wx1234567890ABCDEF", "path": "pages/store/index",
				"color": "#12AbEF", "sort": int64(100),
			}
			values[tc.column] = tc.value
			_, err := db.Exec(`INSERT INTO navigation_items
				(category_id,title,subtitle,icon_text,app_id,path,color,sort,enabled)
				VALUES (7401,?,?,?,?,?,?,?,TRUE)`,
				values["title"], values["subtitle"], values["icon_text"], values["app_id"],
				values["path"], values["color"], values["sort"])
			if err != nil {
				t.Fatalf("seed unsafe legacy item: %v", err)
			}

			err = executeMigration031(t, db)
			assertMigration031Error(t, err, "migration 031 unsafe legacy category item")
			assertMigration031ObjectCount(t, db, 0)
		})
	}
}

func TestMigration031RejectsDuplicateTargetsBeforeDDL(t *testing.T) {
	db, _ := newMigration031Database(t)
	createMigration031BaseSchema(t, db, nil)
	mustExec031(t, db, `INSERT INTO navigation_categories(id,name,sort,enabled) VALUES (7501,'普通分类',20,TRUE)`)
	mustExec031(t, db, `INSERT INTO navigation_items
		(category_id,title,subtitle,icon_text,app_id,path,color,sort,enabled)
		VALUES
		(7501,'商家一','','一','wx1234567890ABCDEF','pages/store/index','#112233',1,TRUE),
		(7501,'商家二','','二','wx1234567890ABCDEF','pages/store/index','#445566',2,TRUE)`)

	err := executeMigration031(t, db)
	assertMigration031Error(t, err, "migration 031 duplicate navigation target")
	assertMigration031ObjectCount(t, db, 0)
}

func TestMigration031RejectsConflictingObjectsBeforeDDL(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup string
		want  string
	}{
		{
			name:  "equivalent_target_index_with_other_name",
			setup: `ALTER TABLE navigation_items ADD UNIQUE INDEX uq_existing_navigation_target (category_id,app_id,path)`,
			want:  "migration 031 conflicting target index",
		},
		{
			name: "check_name_on_other_table",
			setup: `CREATE TABLE navigation_check_name_conflict (
				value INT NOT NULL,
				CONSTRAINT chk_navigation_items_operating_status CHECK (value >= 0)
			) ENGINE=InnoDB`,
			want: "migration 031 conflicting check name",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := newMigration031Database(t)
			createMigration031BaseSchema(t, db, nil)
			mustExec031(t, db, tc.setup)
			assertMigration031ObjectCount(t, db, 0)

			err := executeMigration031(t, db)
			assertMigration031Error(t, err, tc.want)
			assertMigration031ObjectCount(t, db, 0)
		})
	}
}

func TestMigration031RejectsPartialSchema(t *testing.T) {
	tests := []struct {
		name  string
		setup []string
	}{
		{name: "system_key_column_only", setup: []string{
			`ALTER TABLE navigation_categories ADD COLUMN system_key VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NULL AFTER name`,
		}},
		{name: "category_pair_only", setup: []string{
			`ALTER TABLE navigation_categories ADD COLUMN system_key VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NULL AFTER name, ADD UNIQUE INDEX uq_navigation_categories_system_key (system_key)`,
		}},
		{name: "operating_status_column_only", setup: []string{
			`ALTER TABLE navigation_items ADD COLUMN operating_status VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL DEFAULT 'open' AFTER enabled`,
		}},
		{name: "operating_status_and_check", setup: []string{
			`ALTER TABLE navigation_items ADD COLUMN operating_status VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL DEFAULT 'open' AFTER enabled, ADD CONSTRAINT chk_navigation_items_operating_status CHECK (operating_status IN ('open','resting'))`,
		}},
		{name: "four_of_five_objects", setup: []string{
			`ALTER TABLE navigation_categories ADD COLUMN system_key VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NULL AFTER name, ADD UNIQUE INDEX uq_navigation_categories_system_key (system_key)`,
			`ALTER TABLE navigation_items ADD COLUMN operating_status VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL DEFAULT 'open' AFTER enabled, ADD UNIQUE INDEX uq_navigation_items_category_target (category_id,app_id,path)`,
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := newMigration031Database(t)
			createMigration031BaseSchema(t, db, nil)
			for _, statement := range tc.setup {
				mustExec031(t, db, statement)
			}
			before := migration031ObjectCount(t, db)
			if before < 1 || before > 4 {
				t.Fatalf("test setup object count=%d, want partial 1..4", before)
			}

			for attempt := 1; attempt <= 2; attempt++ {
				err := executeMigration031(t, db)
				assertMigration031Error(t, err, "migration 031 partial schema")
				assertMigration031ObjectCount(t, db, before)
			}
		})
	}
}

func TestMigration031RejectsWrongAppliedSignature(t *testing.T) {
	tests := []struct {
		name  string
		setup []string
		seed  []string
		want  string
	}{
		{name: "system_key_length", setup: []string{
			`ALTER TABLE navigation_categories ADD COLUMN system_key VARCHAR(63) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NULL AFTER name, ADD UNIQUE INDEX uq_navigation_categories_system_key (system_key)`,
			migration031CorrectItemObjects,
		}},
		{name: "system_key_not_null", setup: []string{
			`ALTER TABLE navigation_categories ADD COLUMN system_key VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL AFTER name, ADD UNIQUE INDEX uq_navigation_categories_system_key (system_key)`,
			migration031CorrectItemObjects,
		}},
		{name: "system_key_prefix_index", setup: []string{
			`ALTER TABLE navigation_categories ADD COLUMN system_key VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NULL AFTER name, ADD UNIQUE INDEX uq_navigation_categories_system_key (system_key(16))`,
			migration031CorrectItemObjects,
		}},
		{name: "system_key_named_index_non_unique", setup: []string{
			`ALTER TABLE navigation_categories ADD COLUMN system_key VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NULL AFTER name, ADD INDEX uq_navigation_categories_system_key (system_key)`,
			migration031CorrectItemObjects,
		}},
		{name: "system_key_pad_space_collation", setup: []string{
			`ALTER TABLE navigation_categories ADD COLUMN system_key VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NULL AFTER name, ADD UNIQUE INDEX uq_navigation_categories_system_key (system_key)`,
			migration031CorrectItemObjects,
		}},
		{name: "operating_status_default", setup: []string{
			migration031CorrectCategoryObjects,
			`ALTER TABLE navigation_items ADD COLUMN operating_status VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL DEFAULT 'resting' AFTER enabled, ADD CONSTRAINT chk_navigation_items_operating_status CHECK (operating_status IN ('open','resting')), ADD UNIQUE INDEX uq_navigation_items_category_target (category_id,app_id,path)`,
		}},
		{name: "operating_status_uppercase_default", setup: []string{
			migration031CorrectCategoryObjects,
			`ALTER TABLE navigation_items ADD COLUMN operating_status VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL DEFAULT 'OPEN' AFTER enabled, ADD CONSTRAINT chk_navigation_items_operating_status CHECK (operating_status IN ('open','resting','OPEN')), ADD UNIQUE INDEX uq_navigation_items_category_target (category_id,app_id,path)`,
		}},
		{name: "operating_status_length", setup: []string{
			migration031CorrectCategoryObjects,
			`ALTER TABLE navigation_items ADD COLUMN operating_status VARCHAR(15) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL DEFAULT 'open' AFTER enabled, ADD CONSTRAINT chk_navigation_items_operating_status CHECK (operating_status IN ('open','resting')), ADD UNIQUE INDEX uq_navigation_items_category_target (category_id,app_id,path)`,
		}},
		{name: "operating_status_nullable", setup: []string{
			migration031CorrectCategoryObjects,
			`ALTER TABLE navigation_items ADD COLUMN operating_status VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NULL DEFAULT 'open' AFTER enabled, ADD CONSTRAINT chk_navigation_items_operating_status CHECK (operating_status IN ('open','resting')), ADD UNIQUE INDEX uq_navigation_items_category_target (category_id,app_id,path)`,
		}},
		{name: "operating_status_pad_space_collation", setup: []string{
			migration031CorrectCategoryObjects,
			`ALTER TABLE navigation_items ADD COLUMN operating_status VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL DEFAULT 'open' AFTER enabled, ADD CONSTRAINT chk_navigation_items_operating_status CHECK (operating_status IN ('open','resting')), ADD UNIQUE INDEX uq_navigation_items_category_target (category_id,app_id,path)`,
		}},
		{name: "check_expression", setup: []string{
			migration031CorrectCategoryObjects,
			`ALTER TABLE navigation_items ADD COLUMN operating_status VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL DEFAULT 'open' AFTER enabled, ADD CONSTRAINT chk_navigation_items_operating_status CHECK (operating_status IN ('open','resting','closed')), ADD UNIQUE INDEX uq_navigation_items_category_target (category_id,app_id,path)`,
		}},
		{name: "check_uppercase_literals", setup: []string{
			migration031CorrectCategoryObjects,
			`ALTER TABLE navigation_items ADD COLUMN operating_status VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL DEFAULT 'open' AFTER enabled, ADD CONSTRAINT chk_navigation_items_operating_status CHECK (operating_status IN ('OPEN','RESTING')), ADD UNIQUE INDEX uq_navigation_items_category_target (category_id,app_id,path)`,
		}},
		{name: "check_not_enforced", setup: []string{
			migration031CorrectCategoryObjects,
			`ALTER TABLE navigation_items ADD COLUMN operating_status VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL DEFAULT 'open' AFTER enabled, ADD CONSTRAINT chk_navigation_items_operating_status CHECK (operating_status IN ('open','resting')) NOT ENFORCED, ADD UNIQUE INDEX uq_navigation_items_category_target (category_id,app_id,path)`,
		}},
		{name: "target_index_order", setup: []string{
			migration031CorrectCategoryObjects,
			`ALTER TABLE navigation_items ADD COLUMN operating_status VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL DEFAULT 'open' AFTER enabled, ADD CONSTRAINT chk_navigation_items_operating_status CHECK (operating_status IN ('open','resting')), ADD UNIQUE INDEX uq_navigation_items_category_target (category_id,path,app_id)`,
		}},
		{name: "target_named_index_non_unique", setup: []string{
			migration031CorrectCategoryObjects,
			`ALTER TABLE navigation_items ADD COLUMN operating_status VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL DEFAULT 'open' AFTER enabled, ADD CONSTRAINT chk_navigation_items_operating_status CHECK (operating_status IN ('open','resting')), ADD INDEX uq_navigation_items_category_target (category_id,app_id,path)`,
		}},
		{name: "target_prefix_index", setup: []string{
			migration031CorrectCategoryObjects,
			`ALTER TABLE navigation_items ADD COLUMN operating_status VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL DEFAULT 'open' AFTER enabled, ADD CONSTRAINT chk_navigation_items_operating_status CHECK (operating_status IN ('open','resting')), ADD UNIQUE INDEX uq_navigation_items_category_target (category_id,app_id,path(32))`,
		}},
		{name: "uppercase_system_key_value", setup: []string{
			migration031CorrectCategoryObjects,
			migration031CorrectItemObjects,
		}, seed: []string{
			`INSERT INTO navigation_categories(name,system_key,sort,enabled) VALUES ('外卖商家','EXTERNAL_TAKEOUT_MERCHANTS',20,TRUE)`,
		}, want: "migration 031 system category mismatch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := newMigration031Database(t)
			createMigration031BaseSchema(t, db, nil)
			for _, statement := range tc.setup {
				mustExec031(t, db, statement)
			}
			for _, statement := range tc.seed {
				mustExec031(t, db, statement)
			}
			assertMigration031ObjectCount(t, db, 5)

			err := executeMigration031(t, db)
			want := tc.want
			if want == "" {
				want = "migration 031 signature mismatch"
			}
			assertMigration031Error(t, err, want)
			assertMigration031ObjectCount(t, db, 5)
			if tc.name != "uppercase_system_key_value" {
				assertQueryCount031(t, db, 0, `SELECT COUNT(*) FROM navigation_categories WHERE name='外卖商家'`)
			}
		})
	}
}

func TestMigration031ConstraintsAndHistoricalCompatibility(t *testing.T) {
	db, version := newMigration031Database(t)
	if err := applyMigration031Script(db, "001_init.sql"); err != nil {
		t.Fatalf("apply historical base migration: %v", err)
	}
	mustExec031(t, db, `INSERT INTO navigation_categories(id,name,sort,enabled) VALUES (7601,'历史分类',10,TRUE)`)
	mustExec031(t, db, `INSERT INTO navigation_items
		(id,category_id,title,subtitle,icon_text,app_id,path,color,sort,enabled,created_at,updated_at)
		VALUES (7602,7601,'历史条目','','旧','wx1234567890ABCDEF','pages/history/index','#123456',10,TRUE,NOW(3),NOW(3))`)

	applyMigration031Successfully(t, db)
	assertMigration031Signature(t, db)
	assertQueryCount031(t, db, 1, `SELECT COUNT(*) FROM navigation_items WHERE id=7602 AND operating_status='open'`)

	legacyDB, err := gorm.Open(gormMySQL.New(gormMySQL.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open legacy GORM projection: %v", err)
	}
	var historical migration031LegacyNavigationItem
	if err := legacyDB.First(&historical, "id = ?", 7602).Error; err != nil {
		t.Fatalf("read with legacy navigation item projection: %v", err)
	}
	if historical.ID != 7602 || historical.Title != "历史条目" || historical.AppID != "wx1234567890ABCDEF" {
		t.Fatalf("legacy projection read unexpected row: %+v", historical)
	}
	mustExec031(t, db, `UPDATE navigation_items SET operating_status='resting' WHERE id=7602`)
	historical.Title = "旧结构更新后"
	historical.Enabled = false
	if err := legacyDB.Save(&historical).Error; err != nil {
		t.Fatalf("update with legacy navigation item struct: %v", err)
	}
	assertQueryCount031(t, db, 1, `SELECT COUNT(*) FROM navigation_items
		WHERE id=7602 AND title='旧结构更新后' AND enabled=FALSE AND operating_status='resting'`)

	// This create uses a GORM struct containing only historical fields. The new
	// column must be omitted so its exact lowercase default remains authoritative.
	createdByLegacyStruct := migration031LegacyNavigationItem{
		ID: 7603, CategoryID: 7601, Title: "迁移后旧写法", IconText: "兼",
		AppID: "wxABCDEF1234567890", Path: "pages/history/second",
		Color: "#654321", Sort: 11, Enabled: true,
	}
	if err := legacyDB.Create(&createdByLegacyStruct).Error; err != nil {
		t.Fatalf("create with legacy navigation item struct: %v", err)
	}
	assertQueryCount031(t, db, 1, `SELECT COUNT(*) FROM navigation_items WHERE id=7603 AND operating_status='open'`)

	for _, invalidStatus := range []string{"closed", "OPEN", "open "} {
		if _, err := db.Exec(`UPDATE navigation_items SET operating_status=? WHERE id=7602`, invalidStatus); err == nil {
			t.Fatalf("operating status check accepted %q", invalidStatus)
		}
	}
	assertQueryCount031(t, db, 1, `SELECT COUNT(*) FROM navigation_items WHERE id=7602 AND operating_status='resting'`)
	if _, err := db.Exec(`INSERT INTO navigation_categories(name,system_key,sort,enabled) VALUES ('重复系统分类','external_takeout_merchants',1,TRUE)`); err == nil {
		t.Fatal("system_key unique index accepted duplicate key")
	}
	if _, err := db.Exec(`INSERT INTO navigation_items
		(category_id,title,subtitle,icon_text,app_id,path,color,sort,enabled)
		VALUES (7601,'重复目标','','重','wx1234567890ABCDEF','pages/history/index','#111111',12,TRUE)`); err == nil {
		t.Fatal("target unique index accepted duplicate category/app_id/path")
	}
	for _, category := range []migration031LegacyNavigationCategory{
		{ID: 7610, Name: "旧结构空系统键一", Sort: 1, Enabled: true},
		{ID: 7611, Name: "旧结构空系统键二", Sort: 2, Enabled: true},
	} {
		if err := legacyDB.Create(&category).Error; err != nil {
			t.Fatalf("create category with legacy GORM struct: %v", err)
		}
	}
	assertQueryCount031(t, db, 2, `SELECT COUNT(*) FROM navigation_categories WHERE id IN (7610,7611) AND system_key IS NULL`)

	mustExec031(t, db, `UPDATE navigation_categories SET enabled=FALSE WHERE system_key='external_takeout_merchants'`)
	applyMigration031Successfully(t, db)
	assertQueryCount031(t, db, 1, `SELECT COUNT(*) FROM navigation_categories WHERE system_key='external_takeout_merchants' AND enabled=FALSE`)
	assertQueryCount031(t, db, 1, `SELECT COUNT(*) FROM navigation_categories WHERE system_key='external_takeout_merchants'`)
	assertMigration031Signature(t, db)
	t.Logf("migration_031_apply_apply_and_fail_closed=PASS mysql=%s", version)
}

const migration031CorrectCategoryObjects = `ALTER TABLE navigation_categories
	ADD COLUMN system_key VARCHAR(64) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NULL AFTER name,
	ADD UNIQUE INDEX uq_navigation_categories_system_key (system_key)`

const migration031CorrectItemObjects = `ALTER TABLE navigation_items
	ADD COLUMN operating_status VARCHAR(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_bin NOT NULL DEFAULT 'open' AFTER enabled,
	ADD CONSTRAINT chk_navigation_items_operating_status CHECK (operating_status IN ('open','resting')),
	ADD UNIQUE INDEX uq_navigation_items_category_target (category_id,app_id,path)`

// migration031LegacyNavigationItem intentionally mirrors the pre-031 GORM
// projection and has no OperatingStatus field.
type migration031LegacyNavigationItem struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	CategoryID uint      `json:"categoryId" gorm:"index"`
	Title      string    `json:"title" gorm:"size:64"`
	Subtitle   string    `json:"subtitle" gorm:"size:120"`
	IconText   string    `json:"iconText" gorm:"size:12"`
	AppID      string    `json:"appId" gorm:"size:80"`
	Path       string    `json:"path" gorm:"size:240"`
	Color      string    `json:"color" gorm:"size:24"`
	Sort       int       `json:"sort" gorm:"index"`
	Enabled    bool      `json:"enabled" gorm:"default:true"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (migration031LegacyNavigationItem) TableName() string {
	return "navigation_items"
}

type migration031LegacyNavigationCategory struct {
	ID        uint                               `json:"id" gorm:"primaryKey"`
	Name      string                             `json:"name" gorm:"size:64"`
	Sort      int                                `json:"sort" gorm:"index"`
	Enabled   bool                               `json:"enabled" gorm:"default:true"`
	Items     []migration031LegacyNavigationItem `json:"items"`
	CreatedAt time.Time                          `json:"createdAt"`
	UpdatedAt time.Time                          `json:"updatedAt"`
}

func (migration031LegacyNavigationCategory) TableName() string {
	return "navigation_categories"
}

type migration031Column struct {
	name       string
	definition string
}

func createMigration031BaseSchema(t *testing.T, db *sql.DB, overrides map[string]string) {
	t.Helper()
	categories := []migration031Column{
		{name: "id", definition: "id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY"},
		{name: "name", definition: "name VARCHAR(64) NOT NULL DEFAULT ''"},
		{name: "sort", definition: "sort BIGINT NOT NULL DEFAULT 0"},
		{name: "enabled", definition: "enabled BOOLEAN NOT NULL DEFAULT TRUE"},
		{name: "created_at", definition: "created_at DATETIME(3) NULL"},
		{name: "updated_at", definition: "updated_at DATETIME(3) NULL"},
	}
	items := []migration031Column{
		{name: "id", definition: "id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY"},
		{name: "category_id", definition: "category_id BIGINT UNSIGNED NOT NULL"},
		{name: "title", definition: "title VARCHAR(64) NOT NULL DEFAULT ''"},
		{name: "subtitle", definition: "subtitle VARCHAR(120) NOT NULL DEFAULT ''"},
		{name: "icon_text", definition: "icon_text VARCHAR(12) NOT NULL DEFAULT ''"},
		{name: "app_id", definition: "app_id VARCHAR(80) NOT NULL"},
		{name: "path", definition: "path VARCHAR(240) NOT NULL"},
		{name: "color", definition: "color VARCHAR(24) NOT NULL DEFAULT ''"},
		{name: "sort", definition: "sort BIGINT NOT NULL DEFAULT 0"},
		{name: "enabled", definition: "enabled BOOLEAN NOT NULL DEFAULT TRUE"},
		{name: "created_at", definition: "created_at DATETIME(3) NULL"},
		{name: "updated_at", definition: "updated_at DATETIME(3) NULL"},
	}
	render := func(table string, columns []migration031Column, indexes ...string) string {
		parts := make([]string, 0, len(columns)+len(indexes))
		for _, column := range columns {
			definition := column.definition
			if override, ok := overrides[table+"."+column.name]; ok {
				definition = override
			}
			parts = append(parts, definition)
		}
		parts = append(parts, indexes...)
		return fmt.Sprintf("CREATE TABLE %s (%s) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci", table, strings.Join(parts, ","))
	}
	mustExec031(t, db, render("navigation_categories", categories, "INDEX idx_navigation_categories_sort (sort)"))
	mustExec031(t, db, render("navigation_items", items,
		"INDEX idx_navigation_items_category_id (category_id)",
		"INDEX idx_navigation_items_sort (sort)"))
}

func newMigration031Database(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("EXTERNAL_TAKEOUT_DIRECTORY_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set EXTERNAL_TAKEOUT_DIRECTORY_MYSQL_DSN to run migration 031 against a dedicated MySQL 8.4 server")
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
		t.Fatalf("connect MySQL for migration 031: %v", err)
	}
	var version string
	if err := server.QueryRow("SELECT VERSION()").Scan(&version); err != nil {
		server.Close()
		t.Fatal(err)
	}
	if !strings.HasPrefix(version, "8.4.") {
		server.Close()
		t.Fatalf("expected MySQL 8.4, got %s", version)
	}
	sequence := atomic.AddUint64(&migration031DatabaseSequence, 1)
	databaseName := fmt.Sprintf("external_takeout_m031_%d_%d", time.Now().UnixNano(), sequence)
	if _, err := server.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		server.Close()
		t.Fatalf("create independent migration 031 database (in-place fallback is forbidden): %v", err)
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
			t.Errorf("close migration 031 database: %v", err)
		}
		if _, err := server.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`"); err != nil {
			t.Errorf("drop independent migration 031 database: %v", err)
		}
		if err := server.Close(); err != nil {
			t.Errorf("close migration 031 server connection: %v", err)
		}
	})
	return db, version
}

func executeMigration031(t *testing.T, db *sql.DB) error {
	t.Helper()
	t.Cleanup(func() {
		if _, err := db.Exec(`DROP PROCEDURE IF EXISTS migrate_external_takeout_merchant_directory`); err != nil {
			t.Errorf("cleanup migration 031 procedure: %v", err)
			return
		}
		var count int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.routines
			WHERE routine_schema=DATABASE() AND routine_type='PROCEDURE'
			AND routine_name='migrate_external_takeout_merchant_directory'`).Scan(&count); err != nil {
			t.Errorf("count migration 031 procedures after cleanup: %v", err)
			return
		}
		if count != 0 {
			t.Errorf("migration 031 procedure count after cleanup=%d, want 0", count)
		}
	})
	ctx := context.Background()
	migrationConnection, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve migration 031 connection: %w", err)
	}
	defer migrationConnection.Close()
	err = applyMigration031ScriptWithExec(migration031File, func(query string) (sql.Result, error) {
		return migrationConnection.ExecContext(ctx, query)
	})
	assertMigration031LockAvailableFromIndependentConnection(t, db, migrationConnection)
	return err
}

func applyMigration031Successfully(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := executeMigration031(t, db); err != nil {
		t.Fatalf("apply migration 031: %v", err)
	}
	assertQueryCount031(t, db, 0, `SELECT COUNT(*) FROM information_schema.routines
		WHERE routine_schema=DATABASE() AND routine_type='PROCEDURE'
		AND routine_name='migrate_external_takeout_merchant_directory'`)
}

func applyMigration031Script(db *sql.DB, relativePath string) error {
	return applyMigration031ScriptWithExec(relativePath, func(query string) (sql.Result, error) {
		return db.Exec(query)
	})
}

func applyMigration031ScriptWithExec(relativePath string, exec func(string) (sql.Result, error)) error {
	file, err := os.Open(filepath.Clean(relativePath))
	if err != nil {
		return err
	}
	defer file.Close()

	delimiter := ";"
	var statement strings.Builder
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") && strings.TrimSpace(statement.String()) == "" {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(trimmed), "DELIMITER ") {
			if strings.TrimSpace(statement.String()) != "" {
				return fmt.Errorf("%s changes delimiter with pending SQL", relativePath)
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
		if _, err := exec(query); err != nil {
			return fmt.Errorf("apply %s: %w\nSQL:\n%s", relativePath, err, query)
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(statement.String()) != "" {
		return fmt.Errorf("%s has unterminated SQL", relativePath)
	}
	return nil
}

func assertMigration031LockAvailableFromIndependentConnection(t *testing.T, db *sql.DB, migrationConnection *sql.Conn) {
	t.Helper()
	// migrationConnection stays reserved while the probe is made, so database/sql
	// must establish or reserve a different physical connection for this check.
	ctx := context.Background()
	probe, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("reserve independent migration 031 lock probe: %v", err)
	}
	defer probe.Close()
	var migrationConnectionID, probeConnectionID int64
	if err := migrationConnection.QueryRowContext(ctx, `SELECT CONNECTION_ID()`).Scan(&migrationConnectionID); err != nil {
		t.Fatalf("read migration 031 connection id: %v", err)
	}
	if err := probe.QueryRowContext(ctx, `SELECT CONNECTION_ID()`).Scan(&probeConnectionID); err != nil {
		t.Fatalf("read migration 031 probe connection id: %v", err)
	}
	if migrationConnectionID == probeConnectionID {
		t.Fatalf("migration 031 lock probe reused migration connection id=%d", migrationConnectionID)
	}
	var acquired sql.NullInt64
	if err := probe.QueryRowContext(ctx, `SELECT GET_LOCK('campus_circle:migration:031',0)`).Scan(&acquired); err != nil {
		t.Fatalf("probe migration 031 lock release: %v", err)
	}
	if !acquired.Valid || acquired.Int64 != 1 {
		t.Fatalf("migration 031 leaked named lock; independent GET_LOCK=%v", acquired)
	}
	var released sql.NullInt64
	if err := probe.QueryRowContext(ctx, `SELECT RELEASE_LOCK('campus_circle:migration:031')`).Scan(&released); err != nil {
		t.Fatalf("release migration 031 probe lock: %v", err)
	}
	if !released.Valid || released.Int64 != 1 {
		t.Fatalf("migration 031 probe lock release=%v, want 1", released)
	}
}

func assertMigration031Error(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("migration 031 unexpectedly succeeded; want error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("migration 031 error=%q, want substring %q", err, want)
	}
}

func assertMigration031ObjectCount(t *testing.T, db *sql.DB, want int64) {
	t.Helper()
	got := migration031ObjectCount(t, db)
	if got != want {
		t.Fatalf("migration 031 schema object count=%d, want %d", got, want)
	}
}

func migration031ObjectCount(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	var count int64
	err := db.QueryRow(`SELECT
		(SELECT COUNT(*) FROM information_schema.columns
		 WHERE table_schema=DATABASE() AND table_name='navigation_categories' AND column_name='system_key') +
		(SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics
		 WHERE table_schema=DATABASE() AND table_name='navigation_categories' AND index_name='uq_navigation_categories_system_key') +
		(SELECT COUNT(*) FROM information_schema.columns
		 WHERE table_schema=DATABASE() AND table_name='navigation_items' AND column_name='operating_status') +
		(SELECT COUNT(*) FROM information_schema.table_constraints
		 WHERE constraint_schema=DATABASE() AND table_name='navigation_items' AND constraint_name='chk_navigation_items_operating_status') +
		(SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics
		 WHERE table_schema=DATABASE() AND table_name='navigation_items' AND index_name='uq_navigation_items_category_target')`).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func assertMigration031Signature(t *testing.T, db *sql.DB) {
	t.Helper()
	assertMigration031ObjectCount(t, db, 5)
	assertQueryCount031(t, db, 1, `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema=DATABASE() AND table_name='navigation_categories'
		AND column_name='system_key' AND data_type='varchar'
		AND character_maximum_length=64 AND is_nullable='YES'
		AND character_set_name='utf8mb4' AND collation_name='utf8mb4_0900_bin'`)
	assertIndexShape031(t, db, "navigation_categories", "uq_navigation_categories_system_key", []string{"system_key"}, true)
	assertQueryCount031(t, db, 1, `SELECT COUNT(*) FROM information_schema.columns
		WHERE table_schema=DATABASE() AND table_name='navigation_items'
		AND column_name='operating_status' AND data_type='varchar'
		AND character_maximum_length=16 AND is_nullable='NO'
		AND character_set_name='utf8mb4' AND collation_name='utf8mb4_0900_bin'
		AND HEX(column_default)=HEX('open')`)
	assertQueryCount031(t, db, 1, `SELECT COUNT(*)
		FROM information_schema.table_constraints AS tc
		JOIN information_schema.check_constraints AS cc
		  ON cc.constraint_schema=tc.constraint_schema AND cc.constraint_name=tc.constraint_name
		WHERE tc.constraint_schema=DATABASE() AND tc.table_name='navigation_items'
		  AND tc.constraint_name='chk_navigation_items_operating_status'
		  AND tc.constraint_type='CHECK' AND tc.enforced='YES'
		  AND REPLACE(
		    LOWER(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(cc.check_clause,CHAR(96),''),' ',''),'(',''),')',''),'_utf8mb4','')),
		    CONCAT(CHAR(92),CHAR(39)),
		    CHAR(39)
		  )
		    = CONCAT('operating_statusin',CHAR(39),'open',CHAR(39),',',CHAR(39),'resting',CHAR(39))
		  AND LOCATE(
		    HEX(CONCAT(CHAR(39),'open',CHAR(39))),
		    HEX(REPLACE(cc.check_clause,CONCAT(CHAR(92),CHAR(39)),CHAR(39)))
		  ) > 0
		  AND LOCATE(
		    HEX(CONCAT(CHAR(39),'resting',CHAR(39))),
		    HEX(REPLACE(cc.check_clause,CONCAT(CHAR(92),CHAR(39)),CHAR(39)))
		  ) > 0`)
	assertIndexShape031(t, db, "navigation_items", "uq_navigation_items_category_target", []string{"category_id", "app_id", "path"}, true)
}

func assertIndexShape031(t *testing.T, db *sql.DB, table, index string, wantColumns []string, unique bool) {
	t.Helper()
	rows, err := db.Query(`SELECT column_name,non_unique,sub_part FROM information_schema.statistics
		WHERE table_schema=DATABASE() AND table_name=? AND index_name=? ORDER BY seq_in_index`, table, index)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var columns []string
	wantNonUnique := 1
	if unique {
		wantNonUnique = 0
	}
	for rows.Next() {
		var column string
		var nonUnique int
		var subPart sql.NullInt64
		if err := rows.Scan(&column, &nonUnique, &subPart); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, column)
		if nonUnique != wantNonUnique {
			t.Errorf("%s.%s non_unique=%d want=%d", table, index, nonUnique, wantNonUnique)
		}
		if subPart.Valid {
			t.Errorf("%s.%s column %s uses prefix length %d", table, index, column, subPart.Int64)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(columns, ",") != strings.Join(wantColumns, ",") {
		t.Errorf("%s.%s columns=%v want=%v", table, index, columns, wantColumns)
	}
}

func assertQueryCount031(t *testing.T, db *sql.DB, want int64, query string, args ...any) {
	t.Helper()
	var got int64
	if err := db.QueryRow(query, args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("query count=%d want=%d query=%s args=%v", got, want, query, args)
	}
}

func mustExec031(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("exec migration 031 test SQL: %v\nSQL:\n%s", err, query)
	}
}
