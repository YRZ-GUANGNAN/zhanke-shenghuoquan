# 外卖商家动态目录 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把学生外卖页从六个硬编码品牌改造成后台驱动的外部小程序目录，让管理员可新增、编辑、营业/休息、隐藏/恢复、删除商家并控制目录总开关，学生仅在商家营业时使用服务端返回的 AppID/path 跳转。

**Architecture:** 继续复用导航域 D，在 `navigation_categories` 增加稳定系统标识、在 `navigation_items` 增加独立营业状态；专用管理路由按 `system_key=external_takeout_merchants` 定位分类，通用 CRUD 对系统分类 fail-closed。公开导航只返回展示中的分类与条目，MiniApp 按系统标识选取目录；旧外卖域 G、购物车、订单、支付、骑手和第三方订单回传保持关闭。

**Tech Stack:** Go 1.25、Gin 1.10、GORM 1.25/MySQL 8.4、React 18、Vite 8、Taro 4.2、TypeScript 5.4、微信 `navigateToMiniProgram`、Node.js 静态边界脚本、systemd/Nginx 原生部署

---

## 执行约束

- 工作目录固定为 `C:\Users\33141\Documents\New project`。
- 每个命令块都从仓库根目录独立执行：Go 使用 `go -C backend ...`，Node 使用 `npm.cmd ...`；不依赖上一个代码块留下的当前目录。
- 当前 shell 是 Windows PowerShell 5.1，`$ErrorActionPreference` 不会把 native 非零退出自动变成终止错误；执行器必须在每条 `npm.cmd/go/git/rg/tar/ssh/scp` 后读取 `$LASTEXITCODE`，除明确要求红灯的步骤外，任一非零立即停止，不能让后续 `git add/commit` 掩盖失败。
- 不改写用户当前未提交的 `.env.example`、`.gitignore`、`apps/miniapp/project.config.json`、`apps/miniapp/project.private.config.example.json`、`backend/.env.example`、`docs/G-7-QUICKSTART-LOCAL-VERIFY.md`、`.claude/` 和 `deploy/native/*`。
- 不修改历史迁移；新增迁移固定为 `031_external_takeout_merchant_directory.sql`。
- 本次是 forward-only schema：不创建 `backend/migrations/rollback/031_external_takeout_merchant_directory.sql`，生产异常只回滚二进制/前端版本，新增列和运营数据保留。
- 不写入假商家，不复用 `wx-map-demo` 等开发占位 AppID；真实商家数据只由管理员在部署后录入。
- 不注册 `pages/takeout`、购物车、订单、支付或商家/骑手旧页面；三个 legacy takeout 开关保持 `false`。
- 前端没有 Jest/Vitest runner。前端自动化结论只写 `typecheck/static/build PASS`；真实跨小程序跳转必须由真实 AppID/path 和真机验收决定。
- 未取得真实商家 AppID/path 或真机跳转未成功时，最终状态只能是“代码/静态验收 PASS，真机验收 PENDING”。

## 文件边界

- Create: `backend/migrations/031_external_takeout_merchant_directory.sql`
- Create: `backend/migrations/migration_031_test.go`
- Modify: `backend/internal/module/navigation/model.go`
- Modify: `backend/internal/module/navigation/repository.go`
- Modify: `backend/internal/module/navigation/service.go`
- Modify: `backend/internal/module/navigation/handler.go`
- Create: `backend/internal/module/navigation/external_takeout.go`
- Create: `backend/internal/module/navigation/external_takeout_test.go`
- Create: `backend/internal/module/navigation/repository_mysql_test.go`
- Create: `backend/internal/module/navigation/service_test.go`
- Create: `backend/internal/module/navigation/handler_test.go`
- Modify: `backend/internal/module/admin/handler.go`
- Create: `backend/internal/module/admin/external_takeout_navigation.go`
- Create: `backend/internal/module/admin/navigation_handler_test.go`
- Modify: `backend/cmd/server/legacy_takeout_gate_test.go`
- Modify: `apps/admin/src/types.ts`
- Modify: `apps/admin/src/services/f-admin.ts`
- Create: `apps/admin/src/ExternalTakeoutMerchants.tsx`
- Modify: `apps/admin/src/App.tsx`
- Modify: `apps/admin/src/styles.css`
- Modify: `apps/miniapp/src/services/d-navigation.ts`
- Modify: `apps/miniapp/src/pages/student-takeout/index.tsx`
- Modify: `apps/miniapp/src/pages/student-takeout/index.scss`
- Create: `scripts/check-external-takeout-directory.cjs`
- Modify: `scripts/check-boundaries.cjs`
- Modify: `scripts/check-miniapp-static-routes.cjs`
- Modify: `package.json`
- Modify: `docs/ARCHITECTURE.md`
- Create: `docs/EXTERNAL_TAKEOUT_MERCHANT_DIRECTORY_ACCEPTANCE.md`

不修改 `backend/cmd/server/main.go` 的装配方式、`apps/miniapp/src/app.config.ts` 的路由集合、`apps/miniapp/src/services/taro.ts`、旧 G 域实现或任何支付回调。

### Task 1: 增加 031 forward-only 迁移

**Files:**
- Create: `backend/migrations/031_external_takeout_merchant_directory.sql`
- Create: `backend/migrations/migration_031_test.go`
- Assert absent: `backend/migrations/rollback/031_external_takeout_merchant_directory.sql`

- [ ] **Step 1: 先写无需数据库也会失败的迁移契约测试**

`migration_031_test.go` 先加入静态契约；迁移文件尚不存在时必须红灯：

```go
func TestMigration031SourceContract(t *testing.T) {
	path := "031_external_takeout_merchant_directory.sql"
	source, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	text := string(source)
	for _, token := range []string{
		"system_key", "external_takeout_merchants", "operating_status",
		"uq_navigation_categories_system_key",
		"chk_navigation_items_operating_status",
		"uq_navigation_items_category_target",
		"SIGNAL SQLSTATE '45000'",
		"GET_LOCK", "RELEASE_LOCK", "SQL SECURITY INVOKER",
	} {
		if !strings.Contains(text, token) { t.Fatalf("migration 031 missing %q", token) }
	}
	if _, err := os.Stat(filepath.Join("rollback", path)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("migration 031 must stay forward-only, rollback stat err=%v", err)
	}
}
```

同一文件声明以下 MySQL 8.4 场景，每个失败场景使用独立临时数据库，禁止 in-place fallback。每个执行辅助函数都用 `t.Cleanup` 独立执行 `DROP PROCEDURE IF EXISTS migrate_external_takeout_merchant_directory`，并在测试结束断言 `information_schema.routines` 中残留数为 0；不能依赖失败的 `CALL` 之后仍会执行脚本末尾清理：

完整实现以下测试函数（不是只写无函数体的声明）：`TestMigration031ClaimsUniqueSafeCategory`、`TestMigration031FreshInsertDefaultsEnabled`、`TestMigration031RejectsWrongBaseSchemaBeforeDDL`、`TestMigration031RejectsAmbiguousCategoryBeforeDDL`、`TestMigration031RejectsUnsafeLegacyCategoryBeforeDDL`、`TestMigration031RejectsDuplicateTargetsBeforeDDL`、`TestMigration031RejectsPartialSchema`、`TestMigration031RejectsWrongAppliedSignature` 和 `TestMigration031ConstraintsAndHistoricalCompatibility`。

测试必须证明：唯一同名分类无论为空还是含有全部符合“旧数据安全子集”的条目，都被原 ID 认领并保留分类 `enabled/sort` 与条目 ID；旧数据安全子集要求文本和 path 首尾没有 MySQL 8.4 ICU `[[:space:]]`、任意位置没有控制字符，AppID/颜色/长度/排序合法，path 已是无前导斜线的规范形式且不含 query、percent-encoding、scheme、反斜线、`..` 或 fragment。任一旧条目不满足该保守子集时在 DDL 前拒绝认领，管理员须先用原通用导航能力修正或重命名分类；这既保留已批准的“唯一分类可认领”，也不让未经专用安全校验的数据直接变成外卖跳转。测试数据至少逐项包含首尾 tab、CR/LF、NBSP（U+00A0）和全角空格（U+3000），证明不依赖只能去普通空格的 `TRIM()`。无同名分类才新增 `enabled=true`；执行两次只有一个系统分类；普通其他分类的历史条目得到 `operating_status=open`；基线漂移、歧义、重复目标、partial schema 和错误签名都在补写任何对象前失败。

- [ ] **Step 2: 确认测试先失败**

Run（仓库根目录）:

```powershell
go -C backend test ./migrations -run TestMigration031SourceContract -count=1
```

Expected: FAIL，包含无法读取 `031_external_takeout_merchant_directory.sql`。

- [ ] **Step 3: 实现整体签名迁移，不逐对象静默补齐**

迁移存储过程必须按以下顺序执行：

```sql
DELIMITER $$
DROP PROCEDURE IF EXISTS migrate_external_takeout_merchant_directory$$
CREATE PROCEDURE migrate_external_takeout_merchant_directory()
SQL SECURITY INVOKER
BEGIN
  DECLARE schema_object_count INT DEFAULT 0;
  DECLARE same_name_count INT DEFAULT 0;
  DECLARE system_category_count INT DEFAULT 0;
  DECLARE duplicate_target_count INT DEFAULT 0;
  DECLARE invalid_legacy_item_count INT DEFAULT 0;
  DECLARE lock_acquired INT DEFAULT 0;
  DECLARE signature_ok INT DEFAULT 0;

  DECLARE EXIT HANDLER FOR SQLEXCEPTION
  BEGIN
    IF lock_acquired = 1 THEN
      DO RELEASE_LOCK('campus_circle:migration:031');
    END IF;
    RESIGNAL;
  END;

  SELECT GET_LOCK('campus_circle:migration:031', 10) INTO lock_acquired;
  IF lock_acquired IS NULL OR lock_acquired <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 lock unavailable';
  END IF;

  IF (SELECT COUNT(*) FROM information_schema.tables
      WHERE table_schema=DATABASE() AND table_name IN ('navigation_categories','navigation_items')) <> 2 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 base tables missing';
  END IF;

  SELECT COUNT(*) INTO signature_ok
  FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='navigation_categories' AND (
    (column_name='id' AND column_type='bigint unsigned' AND is_nullable='NO' AND column_key='PRI' AND extra LIKE '%auto_increment%') OR
    (column_name='name' AND data_type='varchar' AND character_maximum_length=64 AND is_nullable='NO') OR
    (column_name='sort' AND data_type='bigint' AND is_nullable='NO') OR
    (column_name='enabled' AND column_type='tinyint(1)' AND is_nullable='NO') OR
    (column_name='created_at' AND data_type='datetime' AND datetime_precision=3 AND is_nullable='YES') OR
    (column_name='updated_at' AND data_type='datetime' AND datetime_precision=3 AND is_nullable='YES')
  );
  IF signature_ok <> 6 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 base schema mismatch';
  END IF;

  SELECT COUNT(*) INTO signature_ok
  FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='navigation_items' AND (
    (column_name='id' AND column_type='bigint unsigned' AND is_nullable='NO' AND column_key='PRI' AND extra LIKE '%auto_increment%') OR
    (column_name='category_id' AND column_type='bigint unsigned' AND is_nullable='NO') OR
    (column_name='title' AND data_type='varchar' AND character_maximum_length=64 AND is_nullable='NO') OR
    (column_name='subtitle' AND data_type='varchar' AND character_maximum_length=120 AND is_nullable='NO') OR
    (column_name='icon_text' AND data_type='varchar' AND character_maximum_length=12 AND is_nullable='NO') OR
    (column_name='app_id' AND data_type='varchar' AND character_maximum_length=80 AND is_nullable='NO') OR
    (column_name='path' AND data_type='varchar' AND character_maximum_length=240 AND is_nullable='NO') OR
    (column_name='color' AND data_type='varchar' AND character_maximum_length=24 AND is_nullable='NO') OR
    (column_name='sort' AND data_type='bigint' AND is_nullable='NO') OR
    (column_name='enabled' AND column_type='tinyint(1)' AND is_nullable='NO') OR
    (column_name='created_at' AND data_type='datetime' AND datetime_precision=3 AND is_nullable='YES') OR
    (column_name='updated_at' AND data_type='datetime' AND datetime_precision=3 AND is_nullable='YES')
  );
  IF signature_ok <> 12 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 base schema mismatch';
  END IF;

  SELECT
    (SELECT COUNT(*) FROM information_schema.columns
      WHERE table_schema=DATABASE() AND table_name='navigation_categories' AND column_name='system_key') +
    (SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics
      WHERE table_schema=DATABASE() AND table_name='navigation_categories' AND index_name='uq_navigation_categories_system_key') +
    (SELECT COUNT(*) FROM information_schema.columns
      WHERE table_schema=DATABASE() AND table_name='navigation_items' AND column_name='operating_status') +
    (SELECT COUNT(*) FROM information_schema.table_constraints
      WHERE table_schema=DATABASE() AND table_name='navigation_items' AND constraint_name='chk_navigation_items_operating_status') +
    (SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics
      WHERE table_schema=DATABASE() AND table_name='navigation_items' AND index_name='uq_navigation_items_category_target')
  INTO schema_object_count;

  IF schema_object_count BETWEEN 1 AND 4 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 partial schema';
  END IF;

  SELECT COUNT(*) INTO same_name_count
  FROM navigation_categories WHERE name='外卖商家';
  SELECT COUNT(*) INTO duplicate_target_count FROM (
    SELECT category_id, app_id, path
    FROM navigation_items
    GROUP BY category_id, app_id, path
    HAVING COUNT(*) > 1
  ) AS duplicate_targets;

  IF same_name_count > 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 ambiguous category';
  END IF;
  IF duplicate_target_count > 0 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 duplicate navigation target';
  END IF;
  IF schema_object_count = 0 AND same_name_count = 1 THEN
    SELECT COUNT(*) INTO invalid_legacy_item_count
    FROM navigation_items AS item
    JOIN navigation_categories AS category ON category.id=item.category_id
    WHERE category.name='外卖商家' AND (
      CHAR_LENGTH(item.title) NOT BETWEEN 1 AND 64 OR
      REGEXP_LIKE(item.title,'^[[:space:]]|[[:space:]]$','c') OR item.title REGEXP '[[:cntrl:]]' OR
      CHAR_LENGTH(item.subtitle) > 120 OR
      REGEXP_LIKE(item.subtitle,'^[[:space:]]|[[:space:]]$','c') OR item.subtitle REGEXP '[[:cntrl:]]' OR
      CHAR_LENGTH(item.icon_text) NOT BETWEEN 1 AND 12 OR
      REGEXP_LIKE(item.icon_text,'^[[:space:]]|[[:space:]]$','c') OR item.icon_text REGEXP '[[:cntrl:]]' OR
      NOT REGEXP_LIKE(item.app_id,'^wx[A-Za-z0-9]{16}$','c') OR
      CHAR_LENGTH(item.path) NOT BETWEEN 1 AND 240 OR
      REGEXP_LIKE(item.path,'^[[:space:]]|[[:space:]]$','c') OR
      LEFT(item.path,1)='/' OR LOCATE('//',item.path)>0 OR LOCATE('://',item.path)>0 OR
      LOCATE(CHAR(92),item.path)>0 OR LOCATE('..',item.path)>0 OR
      LOCATE('#',item.path)>0 OR LOCATE('%',item.path)>0 OR
      LOCATE('?',item.path)>0 OR item.path REGEXP '[[:cntrl:]]' OR
      NOT REGEXP_LIKE(item.color,'^#[0-9A-Fa-f]{6}$','c') OR
      item.sort < 0 OR item.sort > 10000
    );
    IF invalid_legacy_item_count > 0 THEN
      SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 unsafe legacy category item';
    END IF;
  END IF;

  IF schema_object_count = 0 AND EXISTS (
    SELECT 1 FROM (
      SELECT index_name,
             MAX(non_unique) AS non_unique,
             GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') AS columns_in_order,
             SUM(sub_part IS NOT NULL) AS prefix_parts
      FROM information_schema.statistics
      WHERE table_schema=DATABASE() AND table_name='navigation_items'
      GROUP BY index_name
    ) AS existing_indexes
    WHERE non_unique=0
      AND columns_in_order='category_id,app_id,path'
      AND prefix_parts=0
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 conflicting target index';
  END IF;
  IF schema_object_count = 0 AND EXISTS (
    SELECT 1 FROM information_schema.table_constraints
    WHERE constraint_schema=DATABASE()
      AND constraint_name='chk_navigation_items_operating_status'
      AND (table_name<>'navigation_items' OR constraint_type<>'CHECK')
  ) THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 conflicting check name';
  END IF;

  IF schema_object_count = 0 THEN
    ALTER TABLE navigation_categories
      ADD COLUMN system_key VARCHAR(64) NULL AFTER name,
      ADD UNIQUE INDEX uq_navigation_categories_system_key (system_key);
    ALTER TABLE navigation_items
      ADD COLUMN operating_status VARCHAR(16) NOT NULL DEFAULT 'open' AFTER enabled,
      ADD CONSTRAINT chk_navigation_items_operating_status CHECK (operating_status IN ('open','resting')),
      ADD UNIQUE INDEX uq_navigation_items_category_target (category_id, app_id, path);

    IF same_name_count = 1 THEN
      UPDATE navigation_categories
      SET system_key='external_takeout_merchants'
      WHERE name='外卖商家';
    ELSE
      INSERT INTO navigation_categories(name,system_key,sort,enabled,created_at,updated_at)
      VALUES('外卖商家','external_takeout_merchants',20,TRUE,NOW(3),NOW(3));
    END IF;
  END IF;

  SELECT COUNT(*) INTO signature_ok
  FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='navigation_categories'
    AND column_name='system_key' AND data_type='varchar'
    AND character_maximum_length=64 AND is_nullable='YES';
  IF signature_ok <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 signature mismatch';
  END IF;

  SELECT COUNT(*) INTO signature_ok FROM (
    SELECT index_name,
           MAX(non_unique) AS non_unique,
           GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') AS columns_in_order,
           SUM(sub_part IS NOT NULL) AS prefix_parts
    FROM information_schema.statistics
    WHERE table_schema=DATABASE() AND table_name='navigation_categories'
      AND index_name='uq_navigation_categories_system_key'
    GROUP BY index_name
    HAVING non_unique=0 AND columns_in_order='system_key' AND prefix_parts=0
  ) AS category_key_signature;
  IF signature_ok <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 signature mismatch';
  END IF;

  SELECT COUNT(*) INTO signature_ok
  FROM information_schema.columns
  WHERE table_schema=DATABASE() AND table_name='navigation_items'
    AND column_name='operating_status' AND data_type='varchar'
    AND character_maximum_length=16 AND is_nullable='NO'
    AND column_default='open';
  IF signature_ok <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 signature mismatch';
  END IF;

  SELECT COUNT(*) INTO signature_ok
  FROM information_schema.table_constraints AS table_constraint
  JOIN information_schema.check_constraints AS check_constraint
    ON check_constraint.constraint_schema=table_constraint.constraint_schema
   AND check_constraint.constraint_name=table_constraint.constraint_name
  WHERE table_constraint.constraint_schema=DATABASE()
    AND table_constraint.table_name='navigation_items'
    AND table_constraint.constraint_name='chk_navigation_items_operating_status'
    AND table_constraint.constraint_type='CHECK'
    AND table_constraint.enforced='YES'
    AND LOWER(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(check_constraint.check_clause,'`',''),' ',''),'(',''),')',''),'_utf8mb4',''))
      = CONCAT('operating_statusin',CHAR(39),'open',CHAR(39),',',CHAR(39),'resting',CHAR(39));
  IF signature_ok <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 signature mismatch';
  END IF;

  SELECT COUNT(*) INTO signature_ok FROM (
    SELECT index_name,
           MAX(non_unique) AS non_unique,
           GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',') AS columns_in_order,
           SUM(sub_part IS NOT NULL) AS prefix_parts
    FROM information_schema.statistics
    WHERE table_schema=DATABASE() AND table_name='navigation_items'
      AND index_name='uq_navigation_items_category_target'
    GROUP BY index_name
    HAVING non_unique=0 AND columns_in_order='category_id,app_id,path' AND prefix_parts=0
  ) AS target_signature;
  IF signature_ok <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 signature mismatch';
  END IF;

  SELECT COUNT(*) INTO system_category_count
  FROM navigation_categories
  WHERE system_key='external_takeout_merchants' AND name='外卖商家';
  IF system_category_count <> 1 OR
     (SELECT COUNT(*) FROM navigation_categories WHERE name='外卖商家') <> 1 THEN
    SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'migration 031 system category mismatch';
  END IF;

  DO RELEASE_LOCK('campus_circle:migration:031');
  SET lock_acquired = 0;
END$$
CALL migrate_external_takeout_merchant_directory()$$
DROP PROCEDURE migrate_external_takeout_merchant_directory$$
DELIMITER ;
```

完整签名查询必须验证：

- `system_key`: `VARCHAR(64) NULL`；
- `uq_navigation_categories_system_key`: UNIQUE，列序仅 `system_key`，无前缀索引；
- `operating_status`: `VARCHAR(16) NOT NULL DEFAULT 'open'`；
- `chk_navigation_items_operating_status`: 规范化表达式只允许 `open/resting`；
- `uq_navigation_items_category_target`: UNIQUE，列序精确为 `category_id,app_id,path`，无前缀索引。

若五个对象全部存在，仅验证签名和系统分类后 no-op；不得把已关闭的分类重新改为 `enabled=true`。MySQL DDL 自动提交，中途异常留下 partial 时下一次必须继续拒绝，由人工核查，不自动修复。

`TestMigration031RejectsWrongBaseSchemaBeforeDDL` 必须表驱动修改两个表上述全部基线列，至少逐项覆盖主键自增/unsigned、展示字段长度、排序类型、布尔列和时间精度；每个场景都断言五个新对象数量仍为 0，不能只漂移 `app_id/path`。

- [ ] **Step 4: 完成真实 MySQL 8.4 测试**

复用 `migration_030_test.go` 的 SQL delimiter parser，但新增返回 `error` 的执行辅助函数，供预期失败场景断言。测试 DSN 使用独立变量，不允许退回共享数据库：

```powershell
if (-not $env:EXTERNAL_TAKEOUT_DIRECTORY_MYSQL_DSN) { throw 'EXTERNAL_TAKEOUT_DIRECTORY_MYSQL_DSN 未配置' }
go -C backend test ./migrations -run TestMigration031 -count=1 -v
```

Expected: PASS，并输出 `migration_031_apply_apply_and_fail_closed=PASS mysql=8.4.x`。未配置 DSN 的 SKIP 不得记为迁移验收 PASS。

- [ ] **Step 5: 提交迁移**

```powershell
git add backend/migrations/031_external_takeout_merchant_directory.sql backend/migrations/migration_031_test.go
git commit -m "feat: 增加外卖商家目录迁移"
```

### Task 2: 定义导航模型、公开 DTO 与严格校验

**Files:**
- Modify: `backend/internal/module/navigation/model.go`
- Create: `backend/internal/module/navigation/external_takeout.go`
- Create: `backend/internal/module/navigation/external_takeout_test.go`

- [ ] **Step 1: 写输入规范化和状态失败测试**

表驱动测试至少覆盖：文本 trim、AppID 18 位、单个前导 `/` 归一化、scheme/host、`//`、反斜杠、`..`、fragment、控制字符、无效 percent-encoding、重复及大小写变体敏感 query key、UTF-8 长度边界、颜色、排序 `0/10000` 和越界、`open/resting`。

敏感 query key 固定为：`token`、`secret`、`openid`、`session`、`authorization`；使用 `url.ParseQuery` 解码后再做大小写无关比较，不能只查原始字符串。

- [ ] **Step 2: 确认测试先失败**

Run（仓库根目录）:

```powershell
go -C backend test ./internal/module/navigation -run 'TestNormalizeExternalTakeout|TestOperatingStatus' -count=1
```

Expected: FAIL，包含待新增类型或函数未定义。

- [ ] **Step 3: 增加模型和只含允许字段的 DTO**

`model.go` 增加：

```go
type OperatingStatus string

const (
	SystemKeyExternalTakeoutMerchants = "external_takeout_merchants"
	OperatingStatusOpen    OperatingStatus = "open"
	OperatingStatusResting OperatingStatus = "resting"
)

type Category struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:64"`
	SystemKey *string   `json:"systemKey,omitempty" gorm:"size:64;uniqueIndex:uq_navigation_categories_system_key"`
	Sort      int       `json:"sort" gorm:"index"`
	Enabled   bool      `json:"enabled" gorm:"default:true"`
	Items     []Item    `json:"items"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Item struct {
	// 保留现有字段
	OperatingStatus OperatingStatus `json:"operatingStatus" gorm:"size:16;not null;default:open"`
}
```

`external_takeout.go` 增加：

```go
type ExternalTakeoutMerchantInput struct {
	Title, Subtitle, IconText, AppID, Path, Color string
	Sort int
}

type ExternalTakeoutMerchant struct {
	ID uint `json:"id"`
	Title string `json:"title"`
	Subtitle string `json:"subtitle"`
	IconText string `json:"iconText"`
	AppID string `json:"appId"`
	Path string `json:"path"`
	Color string `json:"color"`
	Sort int `json:"sort"`
	Enabled bool `json:"enabled"`
	OperatingStatus OperatingStatus `json:"operatingStatus"`
}

type ExternalTakeoutDirectory struct {
	CategoryEnabled bool `json:"categoryEnabled"`
	Items []ExternalTakeoutMerchant `json:"items"`
}

type PublicItem struct {
	ID uint `json:"id"`
	Title string `json:"title"`
	Subtitle string `json:"subtitle"`
	IconText string `json:"iconText"`
	AppID string `json:"appId"`
	Path string `json:"path"`
	Color string `json:"color"`
	Sort int `json:"sort"`
	OperatingStatus OperatingStatus `json:"operatingStatus"`
}

type PublicCategory struct {
	ID uint `json:"id"`
	Name string `json:"name"`
	SystemKey *string `json:"systemKey,omitempty"`
	Sort int `json:"sort"`
	Items []PublicItem `json:"items"`
}
```

公开 DTO 不包含 `enabled`、`createdAt`、`updatedAt`、`categoryId` 或管理信息；公开树只会映射已启用数据。

- [ ] **Step 4: 实现纯规范化函数**

```go
func normalizeExternalTakeoutMerchantInput(input ExternalTakeoutMerchantInput) (ExternalTakeoutMerchantInput, error)
func normalizeMiniProgramPath(raw string) (string, error)
func validOperatingStatus(value OperatingStatus) bool
```

规则固定：`title` 必填且最多 64 rune；`subtitle` 最多 120 rune；`iconText` 必填且最多 12 rune；AppID 匹配 `^wx[A-Za-z0-9]{16}$`；颜色匹配 `^#[0-9A-Fa-f]{6}$`；排序为整数 `0..10000`。路径允许一个前导 `/` 并存为无前导斜杠形式，规范化后最多 240 rune；分别对 path component 执行 `url.PathUnescape`、对 query 执行 `url.ParseQuery` 后再校验，拒绝 scheme/host、`//`、反斜杠、`..`、`#`、控制字符和空路径。测试必须包含 `%2e%2e`、`%5c`、`%00`、`%2f%2f` 及大小写变体，证明检查的是解码后内容。

- [ ] **Step 5: 验证并提交**

```powershell
go -C backend test ./internal/module/navigation -run 'TestNormalizeExternalTakeout|TestOperatingStatus' -count=1
git add backend/internal/module/navigation/model.go backend/internal/module/navigation/external_takeout.go backend/internal/module/navigation/external_takeout_test.go
git commit -m "feat: 定义外卖商家目录模型与校验"
```

Expected: 测试 PASS；错误文本不包含完整 path、query value 或任何敏感值。

### Task 3: 实现 Repository 零值写入和系统分类范围约束

**Files:**
- Modify: `backend/internal/module/navigation/repository.go`
- Create: `backend/internal/module/navigation/repository_mysql_test.go`

- [ ] **Step 1: 写真实 Repository 失败测试**

测试必须覆盖：`enabled=false`、`sort=0`、`subtitle=''`、`resting` 真正落库；隐藏项仍在后台列表但不在 `Tree(true)`；分类关闭后公开树无该分类；重复目标映射冲突；专用更新/删除若 ID 属于普通分类则 fail-closed；不存在返回 `ErrNotFound`。

- [ ] **Step 2: 扩展 Repository 接口**

```go
var (
	ErrNotFound       = errors.New("navigation entry not found")
	ErrDuplicateTarget = errors.New("duplicate navigation target")
)

type Repository interface {
	Tree(context.Context, bool) ([]Category, error)
	FindCategory(context.Context, uint) (Category, bool, error)
	FindCategoryBySystemKey(context.Context, string) (Category, bool, error)
	FindItem(context.Context, uint) (Item, bool, error)
	ListItemsByCategory(context.Context, uint) ([]Item, error)
	DuplicateTarget(context.Context, uint, string, string, uint) (bool, error)
	CreateCategory(context.Context, *Category) error
	UpdateCategory(context.Context, uint, map[string]any) error
	DeleteCategory(context.Context, uint) error
	CreateItem(context.Context, *Item) error
	UpdateItem(context.Context, uint, map[string]any) error
	DeleteItem(context.Context, uint) error
	UpdateItemInCategory(context.Context, uint, uint, map[string]any) error
	DeleteItemInCategory(context.Context, uint, uint) error
}
```

`UpdateCategory` 和 `UpdateItem` 必须使用显式 map，禁止继续使用会跳过零值的 `Updates(struct)`：

```go
func (r *GormRepository) UpdateItem(ctx context.Context, id uint, changes map[string]any) error {
	result:=r.db.WithContext(ctx).Model(&Item{}).Where("id = ?",id).Updates(changes)
	return r.resolveItemUpdateResult(ctx,result,id,0)
}
func (r *GormRepository) UpdateItemInCategory(ctx context.Context,id,categoryID uint,changes map[string]any) error {
	result:=r.db.WithContext(ctx).Model(&Item{}).Where("id = ? AND category_id = ?",id,categoryID).Updates(changes)
	return r.resolveItemUpdateResult(ctx,result,id,categoryID)
}
func (r *GormRepository) DeleteItemInCategory(ctx context.Context,id,categoryID uint) error {
	result:=r.db.WithContext(ctx).Where("id = ? AND category_id = ?",id,categoryID).Delete(&Item{})
	if result.Error!=nil{return mapNavigationWriteError(result.Error)}
	if result.RowsAffected==0{return ErrNotFound}
	return nil
}
```

Service 构造允许字段 map；`UpdateCategory` 的白名单固定只有 `name/sort/enabled`，`UpdateItem` 也只接收各自既有普通字段或专用 service 明确传入的状态字段。Repository 再复制一次白名单后才执行 SQL，任何调用方传入的 `system_key/systemKey` 都不得进入更新 map。专用 service 按 `system_key` 取得 category ID 后必须调用 scoped update/delete，使最终 SQL 原子包含 `id + category_id`，不能只做先读后写。

Repository 与 service 测试必须各覆盖一次：对普通分类提交 `systemKey='external_takeout_merchants'` 或直接传入 `map[string]any{"system_key": ...}` 后，分类其他允许字段按既定策略更新，但数据库 `system_key` 仍为 `NULL`；系统标识只能由 031 迁移写入。

MySQL 错误只在命中本次唯一索引时映射冲突：

```go
func mapNavigationWriteError(err error)error{
	var mysqlErr *mysql.MySQLError
	if errors.As(err,&mysqlErr)&&mysqlErr.Number==1062&&strings.Contains(mysqlErr.Message,"uq_navigation_items_category_target"){
		return ErrDuplicateTarget
	}
	return err
}
```

主键或其他唯一键冲突不得误报 `ErrDuplicateTarget`。创建时 service 强制 `ID=0`。`RowsAffected==0` 的普通/scoped UPDATE 不能直接判定 404：先按相同 scope 查询；记录存在表示幂等同值更新成功，确实不存在才返回 `ErrNotFound`。DELETE 的 0 行仍为 `ErrNotFound`。

普通导航兼容规则：旧 JSON 创建不含 `operatingStatus` 时补 `open`；旧 JSON 更新不含该字段时先读取并保留原状态。普通完整更新不得把空字符串写入 CHECK；只有专用状态 PATCH 改变营业状态。`ListItemsByCategory` 固定 `ORDER BY sort ASC,id ASC`。

- [ ] **Step 3: 运行 Repository 真实门禁**

```powershell
if (-not $env:NAVIGATION_DIRECTORY_MYSQL_DSN) { throw 'NAVIGATION_DIRECTORY_MYSQL_DSN 未配置' }
go -C backend test ./internal/module/navigation -run 'TestExternalTakeoutRepository|TestNavigationZeroValueUpdates' -count=1 -v
```

测试必须从 DSN 连接到 MySQL server 后创建独立临时数据库，执行 `001 + 031`，结束时删除；创建临时库失败就 FAIL，禁止使用 DSN 原数据库或 in-place fallback。Expected: PASS；未配置 DSN 的 SKIP 不能替代 Task 1 的 MySQL 8.4 migration PASS。另加断言：旧 JSON 缺 `operatingStatus` 仍可创建/更新、同值 PUT 成功、scoped ID 属于普通分类时返回 `ErrNotFound`。

- [ ] **Step 4: 提交 Repository**

```powershell
git add backend/internal/module/navigation/repository.go backend/internal/module/navigation/repository_mysql_test.go
git commit -m "feat: 实现外卖目录持久化与零值更新"
```

### Task 4: 实现专用 Service、公开 DTO 和通用 CRUD 围栏

**Files:**
- Modify: `backend/internal/module/navigation/service.go`
- Modify: `backend/internal/module/navigation/handler.go`
- Create: `backend/internal/module/navigation/service_test.go`
- Create: `backend/internal/module/navigation/handler_test.go`

- [ ] **Step 1: 建立内存仓储并写失败测试**

`service_test.go` 的内存仓储实现 Task 3 的完整接口，测试：

- 创建固定写入系统分类并强制 `enabled=true/open`；
- 完整编辑只改七个资料字段，保留营业和展示状态；
- 恢复显示不把 `resting` 改成 `open`；
- duplicate、missing、非法输入和仓储失败；
- 通用 Create/Update/Delete category/item 无法重命名、删除、移入或移出系统分类；
- `PublicTree` 只返回 enabled 分类/items，普通历史导航仍兼容。

- [ ] **Step 2: 定义哨兵和专用 Service 方法**

```go
var (
	ErrInvalidNavigationInput = errors.New("invalid navigation input")
	ErrInvalidExternalTakeoutInput = errors.New("invalid external takeout merchant")
	ErrSystemManaged = errors.New("navigation entry is system managed")
)

func (s *Service) ExternalTakeoutDirectory(context.Context) (ExternalTakeoutDirectory, error)
func (s *Service) CreateExternalTakeoutMerchant(context.Context, ExternalTakeoutMerchantInput) (ExternalTakeoutMerchant, error)
func (s *Service) UpdateExternalTakeoutMerchant(context.Context, uint, ExternalTakeoutMerchantInput) (ExternalTakeoutMerchant, error)
func (s *Service) DeleteExternalTakeoutMerchant(context.Context, uint) error
func (s *Service) SetExternalTakeoutOperatingStatus(context.Context, uint, OperatingStatus) error
func (s *Service) SetExternalTakeoutVisibility(context.Context, uint, bool) error
func (s *Service) SetExternalTakeoutCategoryVisibility(context.Context, bool) error
```

每个 item 写操作都先解析系统分类，再验证 item 的 `CategoryID` 等于该分类 ID。创建、编辑先规范化并调用 `DuplicateTarget` 预查；数据库唯一键继续承担竞态兜底。专用 GET 无论分类是否 `enabled`，都返回分类开关和包括隐藏项在内的全部后台数据。

通用 CRUD 规则固定：

```go
// CreateCategory 永远丢弃客户端 SystemKey。
v.SystemKey = nil

// UpdateCategory 永远只构造 name/sort/enabled map；客户端 SystemKey 不得下沉。
// Update/DeleteCategory 命中非空 SystemKey -> ErrSystemManaged。
// CreateItem 的目标分类有 SystemKey -> ErrSystemManaged。
// Update/DeleteItem 的现有分类或目标分类有 SystemKey -> ErrSystemManaged。
// 普通 CreateItem 显式强制 OperatingStatusOpen；普通 UpdateItem 不把客户端 operatingStatus 放入 map，保留原值。
```

兼容测试必须用迁移前的普通导航 JSON（不含 `systemKey/operatingStatus`）证明创建得到 `open`、更新仍保留 `open`；另提交伪造 `operatingStatus='resting'` 证明普通 CRUD 不能把该字段当作绕过专用状态接口的入口。

普通 category/item 的参数错误必须用 `%w` 包装 `ErrInvalidNavigationInput`；仓储错误保持原 error。这样 HTTP 层能把普通输入错误稳定映射为 `400`，把未知数据库错误映射为固定 `500`，不再沿用“任意 error 都当作 400 并返回原文”的旧行为。

- [ ] **Step 3: 把公开响应映射为只读 DTO**

`PublicTree` 返回 `[]PublicCategory`，`handler.go` 路由不变：

```go
func (s *Service) PublicTree(ctx context.Context) ([]PublicCategory, error) {
	values, err := s.repo.Tree(ctx, true)
	if err != nil { return nil, err }
	return toPublicCategories(values), nil
}
```

Handler 测试必须断言：`systemKey/operatingStatus` 存在；`createdAt/updatedAt/categoryId/enabled` 不在 JSON；隐藏项和禁用分类不返回；仓储 `SELECT secret` 故障只返回固定 `500 list navigation failed`。

- [ ] **Step 4: 验证并提交**

```powershell
go -C backend test ./internal/module/navigation -run 'TestExternalTakeoutService|TestSystemCategoryGuard|TestPublicTree|TestPublicNavigationHandler' -count=1
git add backend/internal/module/navigation/service.go backend/internal/module/navigation/handler.go backend/internal/module/navigation/service_test.go backend/internal/module/navigation/handler_test.go
git commit -m "feat: 增加外卖目录服务与公开契约"
```

Expected: PASS；普通导航旧数据不受外卖严格 AppID/path 校验追溯影响。

### Task 5: 增加七条 Admin 专用路由和安全错误映射

**Files:**
- Modify: `backend/internal/module/admin/handler.go`
- Create: `backend/internal/module/admin/external_takeout_navigation.go`
- Create: `backend/internal/module/admin/navigation_handler_test.go`

- [ ] **Step 1: 写路由、权限和 DTO 失败测试**

测试精确覆盖：未登录 `401`、普通用户 `403`、`admin` 与 `superadmin` 成功；创建默认 `open+enabled`；编辑不改状态；两个状态 PATCH 互不干扰；分类总开关；删除；普通 CRUD 绕行 `409`；未知 DB 错误 `500` 且响应无 SQL/AppID/path。

严格 DTO 对 `id/categoryId/enabled/operatingStatus/systemKey/createdAt/updatedAt/items/extraData` 等未知字段返回 `400`，不能忽略后继续写。

- [ ] **Step 2: 确认测试先失败**

```powershell
go -C backend test ./internal/module/admin -run TestExternalTakeoutNavigationAdmin -count=1
```

Expected: FAIL，路由为 `404` 或 handler 方法未定义。

- [ ] **Step 3: 注册静态优先的七条路由**

`handler.go` 在现有 `secured` 组中加入；`category-visibility` 必须在任何可能吞掉它的动态路由前注册：

```go
secured.GET("/navigation/external-takeout-merchants", h.externalTakeoutMerchants)
secured.POST("/navigation/external-takeout-merchants", h.createExternalTakeoutMerchant)
secured.PATCH("/navigation/external-takeout-merchants/category-visibility", h.setExternalTakeoutCategoryVisibility)
secured.PUT("/navigation/external-takeout-merchants/:id", h.updateExternalTakeoutMerchant)
secured.DELETE("/navigation/external-takeout-merchants/:id", h.deleteExternalTakeoutMerchant)
secured.PATCH("/navigation/external-takeout-merchants/:id/operating-status", h.setExternalTakeoutOperatingStatus)
secured.PATCH("/navigation/external-takeout-merchants/:id/visibility", h.setExternalTakeoutVisibility)
```

- [ ] **Step 4: 使用严格 JSON decoder 和最小 DTO**

```go
type externalTakeoutMerchantRequest struct {
	Title string `json:"title"`
	Subtitle string `json:"subtitle"`
	IconText string `json:"iconText"`
	AppID string `json:"appId"`
	Path string `json:"path"`
	Color string `json:"color"`
	Sort int `json:"sort"`
}
type operatingStatusRequest struct { OperatingStatus *navigation.OperatingStatus `json:"operatingStatus"` }
type visibilityRequest struct { Enabled *bool `json:"enabled"` }
```

`decodeStrictJSON` 使用 `http.MaxBytesReader`、`json.Decoder.DisallowUnknownFields()`，限制 32 KiB 且只允许一个 JSON 对象。两个布尔 PATCH 使用 `*bool`，确保缺字段和显式 `false` 可区分。

错误映射固定：

```go
switch {
case errors.Is(err, navigation.ErrInvalidExternalTakeoutInput):
	response.Fail(c, http.StatusBadRequest, err.Error())
case errors.Is(err, navigation.ErrInvalidNavigationInput):
	response.Fail(c, http.StatusBadRequest, err.Error())
case errors.Is(err, navigation.ErrNotFound):
	response.Fail(c, http.StatusNotFound, "navigation entry not found")
case errors.Is(err, navigation.ErrDuplicateTarget), errors.Is(err, navigation.ErrSystemManaged):
	response.Fail(c, http.StatusConflict, err.Error())
case err != nil:
	response.Fail(c, http.StatusInternalServerError, "navigation operation failed")
default:
	response.OK(c, data)
}
```

POST 成功使用 `201`，其余成功使用 `200`。所有写操作都由现有 `AuthRequired + AdminRequired` 保护，不增加 superadmin-only 限制。

现有通用 navigation handlers 也必须改用该安全映射：参数错误 `400`、不存在 `404`、系统分类/重复目标 `409`、未知仓储错误固定 `500 navigation operation failed`。`createCategory/createItem` 不得继续直接把任意 `err.Error()` 作为 `400` 返回。

- [ ] **Step 5: 验证并提交**

```powershell
go -C backend test ./internal/module/admin -run TestExternalTakeoutNavigationAdmin -count=1
git add backend/internal/module/admin/handler.go backend/internal/module/admin/external_takeout_navigation.go backend/internal/module/admin/navigation_handler_test.go
git commit -m "feat: 增加外卖商家目录管理接口"
```

### Task 6: 接入 Admin 与 MiniApp 的类型化 API 契约

**Files:**
- Modify: `apps/admin/src/types.ts`
- Modify: `apps/admin/src/services/f-admin.ts`
- Modify: `apps/miniapp/src/services/d-navigation.ts`

- [ ] **Step 1: 增加 Admin 类型与带状态错误**

```ts
export type ExternalTakeoutOperatingStatus='open'|'resting'
export interface ExternalTakeoutMerchant{id:number;title:string;subtitle:string;iconText:string;appId:string;path:string;color:string;sort:number;enabled:boolean;operatingStatus:ExternalTakeoutOperatingStatus}
export interface ExternalTakeoutMerchantDirectory{categoryEnabled:boolean;items:ExternalTakeoutMerchant[]}
export interface ExternalTakeoutMerchantInput{title:string;subtitle:string;iconText:string;appId:string;path:string;color:string;sort:number}
```

以上 interface 放在 `types.ts`。运行时错误 class 放在 `f-admin.ts`，不能通过现有 `import type` 引入：

```ts
export class AdminAPIError extends Error {
	constructor(message:string,readonly status:number){super(message);this.name='AdminAPIError'}
}
```

`api()` 在 HTTP 或协议失败时抛 `AdminAPIError`，不得自动重试写请求。`ExternalTakeoutMerchants.tsx` 从 `f-admin.ts` 以值导入该 class。

- [ ] **Step 2: 增加七个 Admin client 方法**

```ts
externalTakeoutMerchants:()=>api<ExternalTakeoutMerchantDirectory>('/admin/navigation/external-takeout-merchants'),
createExternalTakeoutMerchant:(input:ExternalTakeoutMerchantInput)=>api<ExternalTakeoutMerchant>('/admin/navigation/external-takeout-merchants',{method:'POST',body:JSON.stringify(input)}),
updateExternalTakeoutMerchant:(id:number,input:ExternalTakeoutMerchantInput)=>api<ExternalTakeoutMerchant>(`/admin/navigation/external-takeout-merchants/${id}`,{method:'PUT',body:JSON.stringify(input)}),
deleteExternalTakeoutMerchant:(id:number)=>api<{deleted:boolean}>(`/admin/navigation/external-takeout-merchants/${id}`,{method:'DELETE'}),
setExternalTakeoutOperatingStatus:(id:number,operatingStatus:ExternalTakeoutOperatingStatus)=>api<{updated:boolean}>(`/admin/navigation/external-takeout-merchants/${id}/operating-status`,{method:'PATCH',body:JSON.stringify({operatingStatus})}),
setExternalTakeoutVisibility:(id:number,enabled:boolean)=>api<{updated:boolean}>(`/admin/navigation/external-takeout-merchants/${id}/visibility`,{method:'PATCH',body:JSON.stringify({enabled})}),
setExternalTakeoutCategoryVisibility:(enabled:boolean)=>api<{updated:boolean}>('/admin/navigation/external-takeout-merchants/category-visibility',{method:'PATCH',body:JSON.stringify({enabled})})
```

- [ ] **Step 3: 保留普通导航并增加按系统标识筛选的 MiniApp client**

`d-navigation.ts` 保留 `listNavigationItems()`，但必须让通用“校园工具”列表排除系统外卖分类；该页面会无条件调用 `navigateToMiniProgram`，若继续扁平化全部分类会让 `resting` 商家绕过专用页面门禁。复用同一个系统键常量，并新增：

```ts
export type NavigationOperatingStatus='open'|'resting'
export interface ExternalTakeoutMerchantLink{id:string;title:string;subtitle:string;iconText:string;appId:string;path:string;color:string;sort:number;operatingStatus:NavigationOperatingStatus}
interface BackendItem{id:number|string;title:string;subtitle:string;iconText:string;appId:string;path:string;color:string;sort:number;operatingStatus:NavigationOperatingStatus}
interface BackendCategory{id:number|string;name:string;systemKey?:string;sort:number;items:BackendItem[]}
const externalTakeoutSystemKey='external_takeout_merchants' as const

export function listNavigationCategories():Promise<BackendCategory[]>{
	return request<BackendCategory[]>('/navigation/categories')
}
export async function listNavigationItems():Promise<ToolLink[]>{
	const categories=await listNavigationCategories()
	return categories
		.filter((category)=>category.systemKey!==externalTakeoutSystemKey)
		.flatMap((category)=>category.items.map((item)=>({
			id:String(item.id),title:item.title,subtitle:item.subtitle,iconText:item.iconText,
			appId:item.appId,path:item.path,color:item.color
		})))
}
export async function listExternalTakeoutMerchants():Promise<ExternalTakeoutMerchantLink[]>{
	const categories=await listNavigationCategories()
	const category=categories.find((value)=>value.systemKey===externalTakeoutSystemKey)
	return (category?.items||[]).map((item)=>({
		id:String(item.id),title:item.title,subtitle:item.subtitle,iconText:item.iconText,
		appId:item.appId,path:item.path,color:item.color,sort:item.sort,
		operatingStatus:item.operatingStatus
	}))
}
```

不得按中文名称或数据库 ID 筛选，不重新猜测排序，不导入 G/Delivery/支付/身份服务。`listNavigationItems()` 的排除和 `listExternalTakeoutMerchants()` 的选取都必须使用 `externalTakeoutSystemKey`；不得只在页面视觉上隐藏，否则通用工具页仍可绕过休息门禁。

- [ ] **Step 4: 类型检查并提交**

```powershell
npm.cmd --prefix apps/admin run typecheck
npm.cmd --prefix apps/miniapp run typecheck
git add apps/admin/src/types.ts apps/admin/src/services/f-admin.ts apps/miniapp/src/services/d-navigation.ts
git commit -m "feat: 接入外卖商家目录前端契约"
```

### Task 7: 增加 Admin 外卖商家管理面板

**Files:**
- Create: `apps/admin/src/ExternalTakeoutMerchants.tsx`
- Modify: `apps/admin/src/App.tsx`
- Modify: `apps/admin/src/styles.css`

- [ ] **Step 1: 创建独立组件和默认表单**

```ts
const emptyDraft:ExternalTakeoutMerchantInput={
	title:'',subtitle:'',iconText:'',appId:'',path:'',color:'#1677FF',sort:0
}
```

组件状态固定包含 `directory/loading/error/editingId/draft/saving/busyAction`。新建表单只显示七个资料字段，不显示营业或展示状态。客户端先执行与服务端一致的基础校验，但服务端结果为最终依据。

- [ ] **Step 2: 实现“写成功后重拉，失败保留现场”**

所有 mutation 共享以下模式：

```ts
async function mutate(key:string,operation:()=>Promise<unknown>):Promise<boolean>{
	setBusyAction(key);setError('')
	try{await operation();await load();return true}
	catch(error){setError(adminErrorMessage(error));return false}
	finally{setBusyAction('')}
}

function adminErrorMessage(error:unknown):string{
	if(error instanceof AdminAPIError){
		if(error.status===401)return'登录已失效，请重新登录'
		if(error.status===403)return'当前账号没有管理权限'
		if(error.status===409)return error.message||'商家配置冲突，请刷新后重试'
		if(error.status>=500)return'服务暂时不可用，请稍后重试'
		return error.message
	}
	return error instanceof Error?error.message:'操作失败，请重试'
}
```

`load()` 遇到错误必须 reject，让 `mutate()` 返回 `false`。保存表单仅在 `await mutate(...) === true` 后清空 `draft` 和 `editingId`；任何失败保留用户输入。状态切换、隐藏/恢复、删除和分类总开关也不得清空当前编辑表单。

操作精确映射：

- `open → resting`: 暂停营业；
- `resting → open`: 开始营业；
- `enabled=true → false`: 隐藏；
- `enabled=false → true`: 恢复显示且不改营业状态；
- 删除先 `window.confirm`；
- 分类总开关单独调用 `category-visibility`。

分类总开关必须绑定服务端状态和专用 client，不得从条目数量推测：

```tsx
<button disabled={Boolean(busyAction)} onClick={()=>void mutate('category-visibility',()=>adminAPI.setExternalTakeoutCategoryVisibility(!directory.categoryEnabled))}>
	{directory.categoryEnabled?'关闭学生端目录':'开启学生端目录'}
</button>
```

`401/403/409/500` 显示明确错误，不重放写入、不清空表单、不乐观删除列表行。

- [ ] **Step 3: 接入导航管理而不复用旧外卖后台**

`App.tsx` 的 `Navigation` 面板顶部渲染 `<ExternalTakeoutMerchants />`，下面保留普通导航只读表格。新组件不得导入 `TakeoutConsole`、`takeout-admin` 或 `legacyTakeoutEnabled`；旧 `TakeoutConsole` 仍只由旧开关控制。

- [ ] **Step 4: 增加独立样式**

只新增 `.external-merchant-*` 类；可复用通用 `.panel`、`.table-scroll` 和 button token，不复用 `.takeout-*` 业务类。必须包含：响应式双列表单、文字头像颜色、状态 badge、错误框、禁用按钮和窄屏行操作换行。

- [ ] **Step 5: 验证并提交**

```powershell
npm.cmd --prefix apps/admin run typecheck
npm.cmd --prefix apps/admin run build
git add apps/admin/src/ExternalTakeoutMerchants.tsx apps/admin/src/App.tsx apps/admin/src/styles.css
git commit -m "feat: 增加外卖商家目录管理面板"
```

Expected: typecheck/build PASS；新增成功后界面来自重新 GET 的 `open+enabled` 服务端结果，隐藏项仍在后台可见。

### Task 8: 把学生外卖页改成动态四态和真实跳转

**Files:**
- Modify: `apps/miniapp/src/pages/student-takeout/index.tsx`
- Modify: `apps/miniapp/src/pages/student-takeout/index.scss`
- Keep: `apps/miniapp/src/services/taro.ts`

- [ ] **Step 1: 删除所有静态商家和错误承诺文案**

删除 `MerchantStatus`、`MerchantCard`、`const merchants`、六个品牌、六个 tone 样式和 `showMiniProgramToast`。提示文案改为“点击进入商家小程序，商品、价格、下单与售后由商家小程序提供”，不得宣称平台接收订单或自动生成配送单。

- [ ] **Step 2: 实现可重试四态与陈旧请求保护**

先补齐 imports：

```ts
import {useCallback,useEffect,useRef,useState} from 'react'
import {useDidShow} from '@tarojs/taro'
import {listExternalTakeoutMerchants,type ExternalTakeoutMerchantLink} from '../../services/d-navigation'
```

```ts
type DirectoryStatus='loading'|'empty'|'ready'|'error'
const [merchants,setMerchants]=useState<ExternalTakeoutMerchantLink[]>([])
const [status,setStatus]=useState<DirectoryStatus>('loading')
const requestVersion=useRef(0)

const load=useCallback(async()=>{
	const version=++requestVersion.current
	setStatus('loading')
	try{
		const values=await listExternalTakeoutMerchants()
		if(version!==requestVersion.current)return
		setMerchants(values);setStatus(values.length?'ready':'empty')
	}catch{
		if(version!==requestVersion.current)return
		setMerchants([]);setStatus('error')
	}
},[])

useDidShow(()=>{void load()})
useEffect(()=>()=>{requestVersion.current+=1},[])
```

四态文案固定为：`正在读取合作商家`、`暂无合作商家`、`商家加载失败，点击重试`、服务端商家列表。API 失败不得回退静态样例。

JSX 必须把 retry 真实绑定到 `load()`，而不是只显示文案：

```tsx
{status==='loading'&&<View className='student-takeout-state'><Text>正在读取合作商家</Text></View>}
{status==='empty'&&<View className='student-takeout-state'><Text>暂无合作商家</Text></View>}
{status==='error'&&<View className='student-takeout-state student-takeout-state--retry' onClick={()=>void load()}><Text>商家加载失败，点击重试</Text></View>}
{status==='ready'&&<View className='student-takeout-list'>{merchants.map(renderMerchant)}</View>}
```

`renderMerchant` 必须把卡片点击绑定到 `()=>void openMerchant(merchant)`，状态文案只由 `operatingStatus` 决定；不能另建前端营业状态来源。

- [ ] **Step 3: 实现休息门禁和单次跳转锁**

```ts
const jumpingRef=useRef<string|null>(null)
const [jumpingMerchantId,setJumpingMerchantId]=useState<string|null>(null)

async function openMerchant(merchant:ExternalTakeoutMerchantLink){
	if(merchant.operatingStatus==='resting'){
		await Taro.showToast({title:'商家休息中',icon:'none'});return
	}
	if(jumpingRef.current)return
	jumpingRef.current=merchant.id;setJumpingMerchantId(merchant.id)
	try{
		await Taro.navigateToMiniProgram({appId:merchant.appId,path:merchant.path})
	}catch{
		await Taro.showToast({title:'商家小程序暂时无法打开',icon:'none'})
	}finally{
		jumpingRef.current=null;setJumpingMerchantId(null)
	}
}
```

`appId/path` 只来自本次 API 对象，页面不读 router query 覆盖。休息卡片保留但微信 API 调用次数为 0；营业卡片连点只调用一次；成功前后都不显示“已跳转”假状态。

- [ ] **Step 4: 更新动态视觉状态**

头像用 `style={{backgroundColor:merchant.color}}`；删除 tone 渐变。休息卡片增加禁用视觉与“休息中”，营业卡片显示“营业中”，跳转中按钮显示“打开中”。新增 `.student-takeout-state`、`--retry`、`--resting` 和 disabled action 样式。

- [ ] **Step 5: 验证并提交**

```powershell
npm.cmd --prefix apps/miniapp run typecheck
npm.cmd run check:miniapp-static-routes
npm.cmd run build:miniapp
git add apps/miniapp/src/pages/student-takeout/index.tsx apps/miniapp/src/pages/student-takeout/index.scss
git commit -m "feat: 外卖页接入动态商家小程序目录"
```

Expected: typecheck/static/build PASS；没有静态 fallback，没有 `/takeout`、购物车、订单或支付调用。

### Task 9: 锁定边界、旧外卖关闭状态与文档

**Files:**
- Create: `scripts/check-external-takeout-directory.cjs`
- Modify: `scripts/check-boundaries.cjs`
- Modify: `scripts/check-miniapp-static-routes.cjs`
- Modify: `package.json`
- Modify: `backend/cmd/server/legacy_takeout_gate_test.go`
- Modify: `docs/ARCHITECTURE.md`

- [ ] **Step 1: 增加外卖目录静态契约脚本**

脚本逐项读取源码并失败退出，至少断言：

```js
assertIncludes(takeoutPage,'listExternalTakeoutMerchants')
assertIncludes(takeoutPage,'navigateToMiniProgram')
assertExcludes(takeoutPage,'const merchants')
for(const brand of ['茶知味','华莱士','正新鸡排','一点点','塔斯汀','蜜雪冰城'])assertExcludes(takeoutPage,brand)
for(const forbidden of ['g-takeout','stores/cart','pages/takeout/','getCurrentInstance'])assertExcludes(takeoutPage,forbidden)
assertIncludes(navigationClient,'external_takeout_merchants')
assertIncludes(navigationClient,'listNavigationItems')
assertIncludes(navigationClient,'category.systemKey!==externalTakeoutSystemKey')
assertIncludes(navigationClient,'value.systemKey===externalTakeoutSystemKey')
for(const forbidden of ['g-takeout','delivery','payment','a-user'])assertExcludes(navigationClient,forbidden)
for(const path of [
	'/admin/navigation/external-takeout-merchants',
	'/operating-status','/visibility','/category-visibility'
])assertIncludes(adminClient,path)
assertExcludes(adminPanel,'TakeoutConsole')
assertExcludes(adminPanel,'takeout-admin')
assertIncludes(adminPanel,'setExternalTakeoutCategoryVisibility')
assertIncludes(adminPanel,'setExternalTakeoutOperatingStatus')
assertIncludes(adminPanel,'setExternalTakeoutVisibility')
assertIncludes(adminPanel,'directory.categoryEnabled')
assertIncludes(adminPanel,'await load()')
assertIncludes(takeoutPage,"merchant.operatingStatus==='resting'")
if(takeoutPage.indexOf("merchant.operatingStatus==='resting'")>takeoutPage.indexOf('navigateToMiniProgram')){
	throw new Error('resting guard must run before navigateToMiniProgram')
}
assertIncludes(appSource,'<ExternalTakeoutMerchants')
assertIncludes(appSource,'legacyTakeoutEnabled')
assertIncludes(appSource,'<TakeoutConsole')
```

其中 `adminPanel` 指独立组件、`appSource` 指 `App.tsx`。同时验证 `pages/student-takeout/index` 已注册，而 `pages/takeout/index`、cart/orders/order-detail/merchant/rider 旧页全部未注册。

- [ ] **Step 2: 补齐 D 域边界和 server gate**

`check-boundaries.cjs` 的导航禁用导入扩展为：

```js
if (/internal\/module\/(user|community|trade|im|admin|media|takeout|delivery)/.test(navigationSource)) {
	failures.push('domain D imports another business domain')
}
```

`legacy_takeout_gate_test.go` 在 `EnableLegacyTakeout=false` 下同时断言：

- public `/api/v1/navigation/categories` 已注册；
- 七条 external takeout admin 路由已注册；
- legacy buyer/merchant/rider/admin/payment callback 均未注册。

外部目录注册不得读取 `EnableLegacyTakeout`。

测试 router 必须显式装配新路由，不能只调用现有 `registerLegacyTakeoutRoutes`：创建 navigation memory repository/service，调用 `navigation.NewHandler(navigationService).Register(api.Group("/navigation"))`，并调用 `admin.NewHandler(nil,nil,userService,nil,nil,navigationService).Register(api)`。随后对七个 method/path 逐一使用 `routeExists` 断言；payment callback 必须检查真实方法 `POST /api/v1/takeout/payments/wechat/notify`，不能用 GET 404 代替。

- [ ] **Step 3: 接入根门禁并更新架构文档**

`package.json` 增加：

```json
"check:external-takeout-directory": "node scripts/check-external-takeout-directory.cjs"
```

并把 `verify` 改为先运行该脚本和静态路由，再执行 boundaries/typecheck/backend tests。

`docs/ARCHITECTURE.md` 明确 D 只保存分类、`systemKey`、展示字段、AppID/path、展示开关和营业展示状态；D 不依赖任何其他业务域，也不保存订单、支付、菜单、配送或第三方回传。

- [ ] **Step 4: 运行边界阶梯并提交**

```powershell
npm.cmd run check:external-takeout-directory
npm.cmd run check:miniapp-static-routes
npm.cmd run check:boundaries
go -C backend test ./cmd/server -run 'TestLegacyTakeout' -count=1
git add scripts/check-external-takeout-directory.cjs scripts/check-boundaries.cjs scripts/check-miniapp-static-routes.cjs package.json backend/cmd/server/legacy_takeout_gate_test.go docs/ARCHITECTURE.md
git commit -m "test: 锁定外卖目录与旧外卖隔离边界"
```

Expected: 所有命令退出码 0；静态路由仍报告当前白名单数量，旧 G 路由保持关闭。

### Task 10: 完整代码门禁与验收记录

**Files:**
- Create: `docs/EXTERNAL_TAKEOUT_MERCHANT_DIRECTORY_ACCEPTANCE.md`

- [ ] **Step 1: 运行完整验证阶梯**

```powershell
npm.cmd run check:external-takeout-directory
if($LASTEXITCODE -ne 0){throw '外卖目录静态契约失败'}
npm.cmd run check:miniapp-static-routes
if($LASTEXITCODE -ne 0){throw 'MiniApp 静态路由门禁失败'}
npm.cmd run check:boundaries
if($LASTEXITCODE -ne 0){throw '架构边界门禁失败'}
npm.cmd run typecheck
if($LASTEXITCODE -ne 0){throw 'TypeScript 类型检查失败'}
npm.cmd run verify
if($LASTEXITCODE -ne 0){throw '根 verify 失败'}
npm.cmd run build
if($LASTEXITCODE -ne 0){throw '根生产构建失败'}
go -C backend test ./...
if($LASTEXITCODE -ne 0){throw '后端全量测试失败'}
go -C backend vet ./...
if($LASTEXITCODE -ne 0){throw 'go vet 失败'}
git diff --check
if($LASTEXITCODE -ne 0){throw 'git diff --check 失败'}
```

Expected: 全部退出码 0。条件集成测试的 SKIP 必须单列；Task 1 的 MySQL 8.4 migration 门禁必须有真实 PASS，不能由全仓测试中的 SKIP 替代。

- [ ] **Step 2: 验证生产构建开关**

```powershell
$env:TARO_APP_API_BASE='https://zhanduiwang.top/api/v1'
$env:TARO_APP_ENABLE_LEGACY_TAKEOUT='false'
$env:VITE_ENABLE_LEGACY_TAKEOUT='false'
npm.cmd run build
if($LASTEXITCODE -ne 0){throw '生产开关构建失败'}
$productionApi=rg -n "https://zhanduiwang.top/api/v1" apps/miniapp/dist
if($LASTEXITCODE -ne 0){throw 'MiniApp 产物缺少正式 HTTPS API 基址'}
$developmentApi=rg -n "https?://(localhost|127\.0\.0\.1)|ws://(localhost|127\.0\.0\.1)" apps/miniapp/dist
if($LASTEXITCODE -eq 0){throw "MiniApp 产物仍包含本地 API/WSS 基址`n$developmentApi"}
if($LASTEXITCODE -gt 1){throw '无法检查 MiniApp 本地基址'}
$legacyAdmin=rg -n "外卖运营|/admin/takeout" apps/admin/dist/assets
if($LASTEXITCODE -eq 0){throw "生产 Admin bundle 仍包含旧外卖入口或 API`n$legacyAdmin"}
if($LASTEXITCODE -gt 1){throw '无法检查生产 Admin bundle'}
```

Expected: build PASS；产物包含正式 HTTPS API 基址，旧 G 开关为 false；Admin bundle 不含“外卖运营”入口或 `/admin/takeout` API。若门禁失败，修正旧组件的条件编译/入口装配后重建，不能仅删除检查。

- [ ] **Step 3: 写验收记录，不夸大真机结论**

验收文档记录：提交 SHA、迁移测试 MySQL 版本、各命令退出结果、四态静态契约结果、legacy gate、未完成项。此阶段尚未做开发者工具/真机四态时，明确写“运行时四态、休息零跳转和营业单次跳转 PENDING”，不得把静态脚本写成模拟 PASS。不得记录 Token、JWT、OpenID、secret、完整敏感 query 或数据库 DSN。

- [ ] **Step 4: 提交验收文档**

```powershell
git add docs/EXTERNAL_TAKEOUT_MERCHANT_DIRECTORY_ACCEPTANCE.md
git commit -m "docs: 记录外卖商家目录代码验收"
```

### Task 11: 按固定顺序部署、真机验收并进入正式发布

**Files:**
- Build only: `.release/campus-api-external-takeout`
- Build only: `.release/campus-admin-external-takeout.tgz`
- Build only: `.release/miniapp-external-takeout.tgz`
- Build only: `.release/external-takeout-release-hashes.csv`
- Upload only: `backend/migrations/031_external_takeout_merchant_directory.sql`
- Modify after runtime: `docs/EXTERNAL_TAKEOUT_MERCHANT_DIRECTORY_ACCEPTANCE.md`

- [ ] **Step 1: 确认真机验收硬前提和允许的发布边界**

业务方必须提供至少一个真实、非敏感的商家正式版 AppID 和可达 path，才能执行真实商家写入和真机跳转。不得使用本平台 AppID、开发占位 AppID、secret 或 Token 代替。

没有真实目标时仍可部署 schema、后端、Admin 和空目录 MiniApp，但只执行无数据写的健康/权限/空态 smoke；新增、编辑、营业/休息、隐藏/恢复、删除、跳转、微信审核与正式发布全部记录为 PENDING。

- [ ] **Step 2: 生成 deployId、四份发布物和可复验 SHA manifest**

```powershell
$ErrorActionPreference='Stop'
$deployId=Get-Date -Format 'yyyyMMdd_HHmmss'
if($deployId -notmatch '^\d{8}_\d{6}$'){throw 'deployId 非法'}
New-Item -ItemType Directory -Force '.release' | Out-Null
$staleArtifacts=@(
  '.release\campus-api-external-takeout',
  '.release\campus-admin-external-takeout.tgz',
  '.release\miniapp-external-takeout.tgz',
  '.release\external-takeout-release-hashes.csv',
  '.release\external-takeout-deploy-id.txt'
)
Remove-Item -Force -ErrorAction SilentlyContinue $staleArtifacts
$env:GOOS='linux';$env:GOARCH='amd64';$env:CGO_ENABLED='0'
go -C backend build -trimpath -ldflags='-s -w' -o '..\.release\campus-api-external-takeout' .\cmd\server
if($LASTEXITCODE -ne 0){throw 'Linux 后端构建失败'}
$env:TARO_APP_API_BASE='https://zhanduiwang.top/api/v1'
$env:TARO_APP_ENABLE_LEGACY_TAKEOUT='false'
$env:VITE_ENABLE_LEGACY_TAKEOUT='false'
npm.cmd run build
if($LASTEXITCODE -ne 0){throw '前端生产构建失败'}
tar.exe -czf '.release\campus-admin-external-takeout.tgz' -C 'apps\admin\dist' .
if($LASTEXITCODE -ne 0){throw 'Admin 归档失败'}
tar.exe -czf '.release\miniapp-external-takeout.tgz' -C 'apps\miniapp' 'project.config.json' 'dist'
if($LASTEXITCODE -ne 0){throw 'MiniApp 归档失败'}
$artifacts=@(
  '.release\campus-api-external-takeout',
  '.release\campus-admin-external-takeout.tgz',
  '.release\miniapp-external-takeout.tgz',
  'backend\migrations\031_external_takeout_merchant_directory.sql'
)
$artifacts|ForEach-Object{
  $file=Get-Item -LiteralPath $_ -ErrorAction Stop
  if($file.Length -le 0){throw "发布物为空：$_"}
}
$hashes=$artifacts|ForEach-Object{Get-FileHash $_ -Algorithm SHA256}
if(@($hashes).Count -ne 4 -or @($hashes|Where-Object{$_.Hash -notmatch '^[0-9A-Fa-f]{64}$'}).Count -ne 0){throw '发布物 SHA 集合非法'}
$hashes|Select-Object Path,Hash|Export-Csv '.release\external-takeout-release-hashes.csv' -NoTypeInformation -Encoding UTF8
$hashes|Format-Table Path,Hash
Set-Content '.release\external-takeout-deploy-id.txt' $deployId -Encoding ascii
```

Expected: 四个 SHA256 均非空；manifest、deployId 和三个构建产物均位于被忽略的 `.release`，不含凭据。

- [ ] **Step 3: 上传 staging、核验远端 SHA 并记录两个旧 symlink**

```powershell
$ErrorActionPreference='Stop'
$deployId=(Get-Content -Raw '.release\external-takeout-deploy-id.txt').Trim()
if($deployId -notmatch '^\d{8}_\d{6}$'){throw 'deployId 非法'}
$key='C:\Users\33141\.ssh\id_ed25519_zhandui'
$target='admin@8.138.227.107'
$ssh=@('-i',$key,'-o','BatchMode=yes','-o','IdentitiesOnly=yes','-o','StrictHostKeyChecking=yes')
$stage="/home/admin/.external-takeout-$deployId"
& ssh @ssh $target "install -d -m 700 '$stage'"
if($LASTEXITCODE -ne 0){throw '创建 staging 失败'}
& scp @ssh '.release\campus-api-external-takeout' '.release\campus-admin-external-takeout.tgz' '.release\miniapp-external-takeout.tgz' 'backend\migrations\031_external_takeout_merchant_directory.sql' "${target}:$stage/"
if($LASTEXITCODE -ne 0){throw '上传发布物失败'}
$oldRelease=(& ssh @ssh $target "readlink -f /opt/campus-circle/current").Trim()
$oldAdmin=(& ssh @ssh $target "readlink -f /var/www/campus-admin/current").Trim()
if($oldRelease -notmatch '^/opt/campus-circle/releases/[A-Za-z0-9._-]+$'){throw "后端 current 异常：$oldRelease"}
if($oldAdmin -notmatch '^/var/www/campus-admin/releases/[A-Za-z0-9._-]+$'){throw "Admin current 异常：$oldAdmin"}
Set-Content '.release\external-takeout-old-backend.txt' $oldRelease -Encoding ascii
Set-Content '.release\external-takeout-old-admin.txt' $oldAdmin -Encoding ascii
$remoteHashes=& ssh @ssh $target "cd '$stage' && sha256sum campus-api-external-takeout campus-admin-external-takeout.tgz miniapp-external-takeout.tgz 031_external_takeout_merchant_directory.sql"
if($LASTEXITCODE -ne 0){throw '远端 SHA 读取失败'}
$expectedNames=@(
  'campus-api-external-takeout',
  'campus-admin-external-takeout.tgz',
  'miniapp-external-takeout.tgz',
  '031_external_takeout_merchant_directory.sql'
)
$manifestRows=@(Import-Csv '.release\external-takeout-release-hashes.csv')
if($manifestRows.Count -ne 4){throw '本地 manifest 行数不是 4'}
$localByName=@{}
$manifestRows|ForEach-Object{
  $name=Split-Path $_.Path -Leaf
  $hash=$_.Hash.ToLowerInvariant()
  if($name -notin $expectedNames -or $localByName.ContainsKey($name) -or $hash -notmatch '^[0-9a-f]{64}$'){throw "本地 manifest 非法：$name"}
  $localByName[$name]=$hash
}
if(@($remoteHashes).Count -ne 4){throw '远端 SHA 行数不是 4'}
$remoteByName=@{}
$remoteHashes|ForEach-Object{
  if($_ -notmatch '^([0-9a-f]{64})\s+\*?(.+)$'){throw "远端 SHA 行非法：$_"}
  if($Matches[2] -notin $expectedNames -or $remoteByName.ContainsKey($Matches[2])){throw "远端 SHA 文件集合非法：$($Matches[2])"}
  $remoteByName[$Matches[2]]=$Matches[1]
}
foreach($name in $expectedNames){
  if(-not $localByName.ContainsKey($name) -or -not $remoteByName.ContainsKey($name) -or $remoteByName[$name] -ne $localByName[$name]){throw "SHA 集合或值不一致：$name"}
}
$preflightScript=@'
set -eu
old_release="$1"; old_admin="$2"
test -L /opt/campus-circle/current
test -L /var/www/campus-admin/current
test "$(readlink -f /opt/campus-circle/current)" = "$old_release"
test "$(readlink -f /var/www/campus-admin/current)" = "$old_admin"
test -d "$old_release" && test -d "$old_admin"
systemctl is-active --quiet campus-api
systemctl is-active --quiet nginx
systemctl is-active --quiet mysql
test -r /etc/campus-circle/mysql-admin.cnf
mode="$(stat -c %a /etc/campus-circle/mysql-admin.cnf)"
case "$mode" in 400|440|600|640) ;; *) exit 23;; esac
grep -qx 'ENABLE_LEGACY_TAKEOUT=false' /etc/campus-circle/campus-api.env
api_pid="$(systemctl show -p MainPID --value campus-api)"
test "$api_pid" -gt 1
tr '\0' '\n' <"/proc/$api_pid/environ" | grep -qx 'ENABLE_LEGACY_TAKEOUT=false'
curl -fsS --max-time 5 http://127.0.0.1:8080/healthz >/dev/null
mysql_version="$(mysql --defaults-extra-file=/etc/campus-circle/mysql-admin.cnf --batch --skip-column-names -e 'SELECT VERSION()')"
case "$mysql_version" in 8.4.*) ;; *) echo "unsupported MySQL $mysql_version" >&2; exit 24;; esac
test "$(mysql --defaults-extra-file=/etc/campus-circle/mysql-admin.cnf --batch --skip-column-names campus_circle -e "SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND trigger_name LIKE 'external_takeout_rollback_guard_%'")" = 0
systemctl is-active --quiet campus-meilisearch || echo 'WARN campus-meilisearch inactive'
echo PREFLIGHT_OK
'@
$preflightScript=$preflightScript -replace "`r`n?","`n"
$preflightEncoded=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($preflightScript))
$preflight=& ssh @ssh $target "echo '$preflightEncoded' | base64 -d | sudo -n bash -s -- '$oldRelease' '$oldAdmin'"
if($LASTEXITCODE -ne 0 -or $preflight -notcontains 'PREFLIGHT_OK'){throw '服务器预检失败'}
$preflight
```

Expected: 四份 SHA 全部一致并输出 `PREFLIGHT_OK`。任一不一致立即停止，权限为 `0700` 的 staging 保留作只读诊断，确认原因后再清理。`campus-meilisearch` 异常只记全站告警，不阻止本目录迁移。

- [ ] **Step 4: 停止 API 写入、备份、执行 031 并原子切换新的后端 release**

创建 `.release/deploy-external-takeout-backend.sh`（不加入 Git），完整内容如下；脚本通过部署锁和 current CAS 防止覆盖漂移版本，API 停止期间阻断旧 Admin 并发写入：

```bash
#!/usr/bin/env bash
set -euo pipefail
deploy_id="$1"; expected_old="$2"; binary_sha="$3"; migration_sha="$4"
[[ "$deploy_id" =~ ^[0-9]{8}_[0-9]{6}$ ]]
[[ "$binary_sha" =~ ^[0-9a-f]{64}$ && "$migration_sha" =~ ^[0-9a-f]{64}$ ]]
stage="/home/admin/.external-takeout-$deploy_id"
new_release="/opt/campus-circle/releases/$deploy_id"
backup="/opt/cutover-backups/external-takeout-$deploy_id"
mysql_cli=(mysql --defaults-extra-file=/etc/campus-circle/mysql-admin.cnf --batch --skip-column-names)
wait_local_health(){
  local attempt
  for ((attempt=1;attempt<=30;attempt++)); do
    if systemctl is-active --quiet campus-api && curl -fsS --max-time 3 http://127.0.0.1:8080/healthz >/dev/null; then return 0; fi
    sleep 1
  done
  return 1
}
stop_api_and_confirm_inactive(){
  local attempt
  systemctl stop campus-api || return 1
  for ((attempt=1;attempt<=10;attempt++)); do
    if ! systemctl is-active --quiet campus-api; then return 0; fi
    sleep 1
  done
  return 1
}
status_code(){ curl -sS -o /dev/null -w '%{http_code}' --max-time 10 "$@"; }
exec 9>/run/lock/campus-api-deploy.lock
flock -n 9
actual_old="$(readlink -f /opt/campus-circle/current)"
[[ -L /opt/campus-circle/current ]]
[[ "$actual_old" == "$expected_old" ]]
[[ "$actual_old" =~ ^/opt/campus-circle/releases/[A-Za-z0-9._-]+$ ]]
[[ -d "$actual_old" && -x "$actual_old/bin/campus-api" && -d "$stage" ]]
[[ ! -e "$new_release" && ! -L "$new_release" && ! -e "$backup" && ! -L "$backup" ]]
[[ "$(sha256sum "$stage/campus-api-external-takeout" | cut -d' ' -f1)" == "$binary_sha" ]]
[[ "$(sha256sum "$stage/031_external_takeout_merchant_directory.sql" | cut -d' ' -f1)" == "$migration_sha" ]]
[[ "$("${mysql_cli[@]}" -e 'SELECT VERSION()')" == 8.4.* ]]
[[ "$("${mysql_cli[@]}" campus_circle -e 'SELECT 1')" == 1 ]]
wait_local_health
install -d -o root -g root -m 0700 "$backup"
cp -a "$actual_old/bin/campus-api" "$backup/campus-api"
printf '%s\n' "$actual_old" >"$backup/old-release"
sha256sum "$backup/campus-api" >"$backup/campus-api.sha256"
success=0; stopped=0; switched=0; guard_installed=0
install_legacy_guard(){
  local object_count category_state category_count category_enabled
  object_count="$("${mysql_cli[@]}" campus_circle -e "SELECT (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='navigation_categories' AND column_name='system_key')+(SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='navigation_categories' AND index_name='uq_navigation_categories_system_key')+(SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='navigation_items' AND column_name='operating_status')+(SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema=DATABASE() AND table_name='navigation_items' AND constraint_name='chk_navigation_items_operating_status')+(SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='navigation_items' AND index_name='uq_navigation_items_category_target')")" || return 1
  if [[ "$object_count" != 5 ]]; then return 0; fi
  category_state="$("${mysql_cli[@]}" campus_circle -e "SELECT COUNT(*),COALESCE(MAX(enabled),0) FROM navigation_categories WHERE system_key='external_takeout_merchants'")" || return 1
  IFS=$'\t' read -r category_count category_enabled <<<"$category_state"
  [[ "$category_count" == 1 && "$category_enabled" =~ ^[01]$ ]]
  printf '%s\n' "$category_enabled" >"$backup/category-enabled-before-legacy-rollback"
  "${mysql_cli[@]}" campus_circle <<'SQL'
UPDATE navigation_categories SET enabled=FALSE WHERE system_key='external_takeout_merchants';
DELIMITER $$
CREATE TRIGGER external_takeout_rollback_guard_category_update BEFORE UPDATE ON navigation_categories FOR EACH ROW
BEGIN IF OLD.system_key='external_takeout_merchants' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='protected navigation unavailable during rollback'; END IF; END$$
CREATE TRIGGER external_takeout_rollback_guard_category_delete BEFORE DELETE ON navigation_categories FOR EACH ROW
BEGIN IF OLD.system_key='external_takeout_merchants' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='protected navigation unavailable during rollback'; END IF; END$$
CREATE TRIGGER external_takeout_rollback_guard_item_insert BEFORE INSERT ON navigation_items FOR EACH ROW
BEGIN IF EXISTS(SELECT 1 FROM navigation_categories WHERE id=NEW.category_id AND system_key='external_takeout_merchants') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='protected navigation unavailable during rollback'; END IF; END$$
CREATE TRIGGER external_takeout_rollback_guard_item_update BEFORE UPDATE ON navigation_items FOR EACH ROW
BEGIN IF EXISTS(SELECT 1 FROM navigation_categories WHERE id IN (OLD.category_id,NEW.category_id) AND system_key='external_takeout_merchants') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='protected navigation unavailable during rollback'; END IF; END$$
CREATE TRIGGER external_takeout_rollback_guard_item_delete BEFORE DELETE ON navigation_items FOR EACH ROW
BEGIN IF EXISTS(SELECT 1 FROM navigation_categories WHERE id=OLD.category_id AND system_key='external_takeout_merchants') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='protected navigation unavailable during rollback'; END IF; END$$
DELIMITER ;
SQL
  [[ "$("${mysql_cli[@]}" campus_circle -e "SELECT enabled FROM navigation_categories WHERE system_key='external_takeout_merchants'")" == 0 ]] || return 1
  [[ "$("${mysql_cli[@]}" campus_circle -e "SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND trigger_name LIKE 'external_takeout_rollback_guard_%'")" == 5 ]] || return 1
  guard_installed=1
}
rollback(){
  local original_status=$? current
  trap - EXIT
  if [[ "$success" == 1 ]]; then exit "$original_status"; fi
  if ! stop_api_and_confirm_inactive; then
    printf 'BACKEND_AUTO_ROLLBACK_STOP_FAILED current_and_database_unchanged=1 manual_intervention=1\n' >&2
    exit "$original_status"
  fi
  if ! "${mysql_cli[@]}" campus_circle -e 'DROP PROCEDURE IF EXISTS migrate_external_takeout_merchant_directory'; then
    printf 'BACKEND_AUTO_ROLLBACK_PROCEDURE_CLEANUP_FAILED api=stopped\n' >&2
    exit "$original_status"
  fi
  if ! install_legacy_guard; then
    printf 'BACKEND_AUTO_ROLLBACK_GUARD_FAILED api=stopped\n' >&2
    exit "$original_status"
  fi
  current="$(readlink -f /opt/campus-circle/current 2>/dev/null || true)"
  if [[ "$switched" == 1 ]]; then
    if [[ "$current" != "$new_release" ]]; then
      printf 'BACKEND_AUTO_ROLLBACK_CAS_FAILED current=%s api=stopped\n' "$current" >&2
      exit "$original_status"
    fi
    [[ ! -e "/opt/campus-circle/current.rollback-$deploy_id" && ! -L "/opt/campus-circle/current.rollback-$deploy_id" ]]
    ln -s "$actual_old" "/opt/campus-circle/current.rollback-$deploy_id"
    mv -Tf "/opt/campus-circle/current.rollback-$deploy_id" /opt/campus-circle/current
  elif [[ "$current" != "$actual_old" ]]; then
    printf 'BACKEND_AUTO_ROLLBACK_CAS_FAILED current=%s api=stopped\n' "$current" >&2
    exit "$original_status"
  fi
  if systemctl restart campus-api && wait_local_health; then
    if [[ "$guard_installed" == 1 ]]; then
      [[ "$("${mysql_cli[@]}" campus_circle -e "SELECT enabled FROM navigation_categories WHERE system_key='external_takeout_merchants'")" == 0 ]] || {
        if stop_api_and_confirm_inactive; then
          printf 'BACKEND_AUTO_ROLLBACK_CATEGORY_NOT_HIDDEN api=stopped\n' >&2
        else
          printf 'BACKEND_AUTO_ROLLBACK_CATEGORY_NOT_HIDDEN_STOP_FAILED manual_intervention=1\n' >&2
        fi
        exit "$original_status"
      }
    fi
    printf 'BACKEND_AUTO_ROLLBACK_OK release=%s guard=%s\n' "$actual_old" "$guard_installed" >&2
  else
    if stop_api_and_confirm_inactive; then
      printf 'BACKEND_AUTO_ROLLBACK_HEALTH_FAILED release=%s api=stopped\n' "$actual_old" >&2
    else
      printf 'BACKEND_AUTO_ROLLBACK_HEALTH_FAILED_STOP_FAILED release=%s manual_intervention=1\n' "$actual_old" >&2
    fi
  fi
  exit "$original_status"
}
trap rollback EXIT
stop_api_and_confirm_inactive
stopped=1
mysql --defaults-extra-file=/etc/campus-circle/mysql-admin.cnf --batch --skip-column-names campus_circle <<SQL
CREATE TABLE navigation_categories_backup_${deploy_id} LIKE navigation_categories;
INSERT INTO navigation_categories_backup_${deploy_id} SELECT * FROM navigation_categories;
CREATE TABLE navigation_items_backup_${deploy_id} LIKE navigation_items;
INSERT INTO navigation_items_backup_${deploy_id} SELECT * FROM navigation_items;
SQL
counts="$("${mysql_cli[@]}" campus_circle -e "SELECT (SELECT COUNT(*) FROM navigation_categories),(SELECT COUNT(*) FROM navigation_categories_backup_${deploy_id}),(SELECT COUNT(*) FROM navigation_items),(SELECT COUNT(*) FROM navigation_items_backup_${deploy_id})")"
IFS=$'\t' read -r categories_live categories_backup items_live items_backup <<<"$counts"
[[ "$categories_live" == "$categories_backup" && "$items_live" == "$items_backup" ]]
mysql --defaults-extra-file=/etc/campus-circle/mysql-admin.cnf campus_circle <"$stage/031_external_takeout_merchant_directory.sql"
schema_state="$("${mysql_cli[@]}" campus_circle -e "SELECT (SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='navigation_categories' AND column_name='system_key')+(SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='navigation_categories' AND index_name='uq_navigation_categories_system_key')+(SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='navigation_items' AND column_name='operating_status')+(SELECT COUNT(*) FROM information_schema.table_constraints WHERE constraint_schema=DATABASE() AND table_name='navigation_items' AND constraint_name='chk_navigation_items_operating_status')+(SELECT COUNT(DISTINCT index_name) FROM information_schema.statistics WHERE table_schema=DATABASE() AND table_name='navigation_items' AND index_name='uq_navigation_items_category_target'),(SELECT COUNT(*) FROM navigation_categories WHERE system_key='external_takeout_merchants' AND name='外卖商家')")"
IFS=$'\t' read -r schema_objects system_categories <<<"$schema_state"
[[ "$schema_objects" == 5 && "$system_categories" == 1 ]]
[[ "$("${mysql_cli[@]}" campus_circle -e "SELECT COUNT(*) FROM information_schema.routines WHERE routine_schema=DATABASE() AND routine_type='PROCEDURE' AND routine_name='migrate_external_takeout_merchant_directory'")" == 0 ]]
cp -a "$actual_old" "$new_release"
install -o root -g campus -m 0750 "$stage/campus-api-external-takeout" "$new_release/bin/campus-api"
[[ "$(sha256sum "$new_release/bin/campus-api" | cut -d' ' -f1)" == "$binary_sha" ]]
[[ "$(readlink -f /opt/campus-circle/current)" == "$actual_old" ]]
[[ ! -e "/opt/campus-circle/current.next-$deploy_id" && ! -L "/opt/campus-circle/current.next-$deploy_id" ]]
ln -s "$new_release" "/opt/campus-circle/current.next-$deploy_id"
[[ "$(readlink -f /opt/campus-circle/current)" == "$actual_old" ]]
mv -Tf "/opt/campus-circle/current.next-$deploy_id" /opt/campus-circle/current
switched=1
systemctl start campus-api
wait_local_health
[[ "$(status_code https://zhanduiwang.top/healthz)" == 200 ]]
[[ "$(status_code https://zhanduiwang.top/api/v1/navigation/categories)" == 200 ]]
[[ "$(status_code https://zhanduiwang.top/api/v1/admin/navigation/external-takeout-merchants)" == 401 ]]
for legacy_get in \
  https://zhanduiwang.top/api/v1/takeout/buyer/merchants \
  https://zhanduiwang.top/api/v1/admin/takeout/merchants \
  https://zhanduiwang.top/api/v1/takeout/merchant/me \
  https://zhanduiwang.top/api/v1/takeout/rider/me; do
  [[ "$(status_code "$legacy_get")" == 404 ]]
done
[[ "$(status_code -H 'Content-Type: application/json' --data '{}' https://zhanduiwang.top/api/v1/takeout/payments/wechat/notify)" == 404 ]]
success=1
trap - EXIT
printf 'BACKEND_DEPLOY_OK old=%s new=%s backup=%s\n' "$actual_old" "$new_release" "$backup"
```

编码上传并执行：

```powershell
$deployId=(Get-Content -Raw '.release\external-takeout-deploy-id.txt').Trim()
$oldRelease=(Get-Content -Raw '.release\external-takeout-old-backend.txt').Trim()
function Get-ReleaseHash([string]$leaf){
  $rows=@(Import-Csv '.release\external-takeout-release-hashes.csv'|Where-Object{(Split-Path $_.Path -Leaf) -eq $leaf})
  if($rows.Count -ne 1){throw "manifest 文件不唯一：$leaf"}
  $hash=$rows[0].Hash.ToLowerInvariant()
  if($hash -notmatch '^[0-9a-f]{64}$'){throw "manifest SHA 非法：$leaf"}
  return $hash
}
$backendSha=Get-ReleaseHash 'campus-api-external-takeout'
$migrationSha=Get-ReleaseHash '031_external_takeout_merchant_directory.sql'
$scriptText=(Get-Content -Raw '.release\deploy-external-takeout-backend.sh') -replace "`r`n?","`n"
$script=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($scriptText))
$key='C:\Users\33141\.ssh\id_ed25519_zhandui';$target='admin@8.138.227.107'
ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes $target "echo '$script' | base64 -d | sudo -n bash -s -- '$deployId' '$oldRelease' '$backendSha' '$migrationSha'"
if($LASTEXITCODE -ne 0){throw '迁移或后端切换失败；检查脚本 trap 的旧 release 恢复结果'}
```

Expected: 备份行数成对一致，031、helper procedure 零残留、30 秒本机健康检查和无凭据语义 smoke 全部成功，输出 `BACKEND_DEPLOY_OK`。若失败，trap 先独立清理 `SQL SECURITY INVOKER` procedure，再只在 current 仍指向本次新 release 时切回旧 binary；完整 031 已存在时先关闭系统分类并安装五个数据库 rollback guard，防止旧通用 CRUD 改写受保护数据。旧 binary 是“目录隐藏、导航写保护”的降级恢复点，不是功能兼容版本；031 新列、运营数据和备份表保留。procedure/guard 清理、guard 安装或旧健康检查失败时 API 保持停止并进入事故处置，不得宣称已回滚。

- [ ] **Step 5: 先验证新后端，再原子部署 Admin**

```powershell
function Assert-HttpStatus([string]$method,[string]$uri,[int]$expected,[bool]$jsonBody=$false){
  $parameters=@{Method=$method;Uri=$uri;TimeoutSec=20;UseBasicParsing=$true;ErrorAction='Stop'}
  if($jsonBody){$parameters.ContentType='application/json';$parameters.Body='{}'}
  try{$status=[int](Invoke-WebRequest @parameters).StatusCode}
  catch{
    if($null -eq $_.Exception.Response){throw}
    $status=[int]$_.Exception.Response.StatusCode
  }
  if($status -ne $expected){throw "HTTP 状态异常：$method $uri expected=$expected actual=$status"}
}
Assert-HttpStatus 'GET' 'https://zhanduiwang.top/healthz' 200
Assert-HttpStatus 'GET' 'https://zhanduiwang.top/api/v1/navigation/categories' 200
Assert-HttpStatus 'GET' 'https://zhanduiwang.top/api/v1/admin/navigation/external-takeout-merchants' 401
foreach($probe in @(
  @{Method='GET';Uri='https://zhanduiwang.top/api/v1/takeout/buyer/merchants'},
  @{Method='GET';Uri='https://zhanduiwang.top/api/v1/admin/takeout/merchants'},
  @{Method='GET';Uri='https://zhanduiwang.top/api/v1/takeout/merchant/me'},
  @{Method='GET';Uri='https://zhanduiwang.top/api/v1/takeout/rider/me'},
  @{Method='POST';Uri='https://zhanduiwang.top/api/v1/takeout/payments/wechat/notify';JsonBody=$true}
)){
  Assert-HttpStatus $probe.Method $probe.Uri 404 ([bool]$probe.JsonBody)
}
```

管理员 Token 只从本机安全环境变量 `$env:ZHANDUI_ADMIN_TOKEN` 读取，调用专用 GET 验证 `200`，不得打印 Token。没有真实商家数据时不执行 POST/PUT/PATCH/DELETE：

```powershell
if([string]::IsNullOrWhiteSpace($env:ZHANDUI_ADMIN_TOKEN)){throw '缺少 ZHANDUI_ADMIN_TOKEN'}
$adminHeaders=@{Authorization="Bearer $($env:ZHANDUI_ADMIN_TOKEN)"}
try{$adminResponse=Invoke-WebRequest -Uri 'https://zhanduiwang.top/api/v1/admin/navigation/external-takeout-merchants' -Headers $adminHeaders -TimeoutSec 20 -UseBasicParsing -ErrorAction Stop}
catch{throw '专用 Admin GET 失败（错误细节已隐藏，避免泄露认证信息）'}
if([int]$adminResponse.StatusCode -ne 200){throw "专用 Admin GET 异常：$([int]$adminResponse.StatusCode)"}
Remove-Variable adminHeaders,adminResponse
Write-Output 'ADMIN_AUTHENTICATED_GET_OK'
```

以上无凭据 smoke 已在 Step 4 的远端事务中执行一次，本机再次执行用于保留 HTTPS 入口证据。若管理员 GET 因 Token 缺失/过期失败，停在“新后端已部署、Admin 未部署”并更新验收记录；若使用已确认有效的管理员 Token 仍非 `200`，禁止继续部署 Admin，按事故判断决定修复新版本或执行 Step 6 的受保护降级回滚，不得直接切换旧 binary。

Admin 部署使用新的 `/var/www/campus-admin/releases/$deployId`，不得覆盖旧目录。创建 `.release/deploy-external-takeout-admin.sh`（不加入 Git）：

```bash
#!/usr/bin/env bash
set -euo pipefail
deploy_id="$1"; expected_old="$2"; archive_sha="$3"
[[ "$deploy_id" =~ ^[0-9]{8}_[0-9]{6}$ ]]
[[ "$archive_sha" =~ ^[0-9a-f]{64}$ ]]
stage="/home/admin/.external-takeout-$deploy_id"
new_admin="/var/www/campus-admin/releases/$deploy_id"
exec 9>/run/lock/campus-admin-deploy.lock
flock -n 9
actual_old="$(readlink -f /var/www/campus-admin/current)"
[[ -L /var/www/campus-admin/current ]]
[[ "$actual_old" == "$expected_old" ]]
[[ "$actual_old" =~ ^/var/www/campus-admin/releases/[A-Za-z0-9._-]+$ ]]
[[ -d "$actual_old" && -d "$stage" && ! -e "$new_admin" && ! -L "$new_admin" ]]
[[ "$(sha256sum "$stage/campus-admin-external-takeout.tgz" | cut -d' ' -f1)" == "$archive_sha" ]]
archive_entries="$(tar -tzf "$stage/campus-admin-external-takeout.tgz")"
while IFS= read -r entry; do
  [[ -n "$entry" ]]
  case "$entry" in /*|../*|*/../*|*/..) exit 31;; esac
done <<<"$archive_entries"
if tar -tvzf "$stage/campus-admin-external-takeout.tgz" | awk '$1 ~ /^[lh]/ { found=1 } END { exit(found ? 0 : 1) }'; then exit 32; fi
install -d -o root -g www-data -m 0750 "$new_admin"
tar --no-same-owner --no-same-permissions -xzf "$stage/campus-admin-external-takeout.tgz" -C "$new_admin"
chown -R root:www-data "$new_admin"
find "$new_admin" -type d -exec chmod 0750 {} +
find "$new_admin" -type f -exec chmod 0640 {} +
test -f "$new_admin/index.html"
success=0; switched=0
rollback(){
  local original_status=$? current
  trap - EXIT
  if [[ "$success" == 1 ]]; then exit "$original_status"; fi
  current="$(readlink -f /var/www/campus-admin/current 2>/dev/null || true)"
  if [[ "$switched" == 1 && "$current" == "$new_admin" ]]; then
    [[ ! -e "/var/www/campus-admin/current.rollback-$deploy_id" && ! -L "/var/www/campus-admin/current.rollback-$deploy_id" ]]
    ln -s "$actual_old" "/var/www/campus-admin/current.rollback-$deploy_id"
    mv -Tf "/var/www/campus-admin/current.rollback-$deploy_id" /var/www/campus-admin/current
    if [[ "$(readlink -f /var/www/campus-admin/current)" == "$actual_old" ]] && curl -fsS --max-time 10 https://zhanduiwang.top/ >/dev/null; then
      printf 'ADMIN_AUTO_ROLLBACK_OK release=%s\n' "$actual_old" >&2
    else
      printf 'ADMIN_AUTO_ROLLBACK_HEALTH_FAILED release=%s\n' "$actual_old" >&2
    fi
  elif [[ "$switched" == 1 ]]; then
    printf 'ADMIN_AUTO_ROLLBACK_CAS_FAILED current=%s\n' "$current" >&2
  fi
  exit "$original_status"
}
trap rollback EXIT
nginx -t
[[ "$(readlink -f /var/www/campus-admin/current)" == "$actual_old" ]]
[[ ! -e "/var/www/campus-admin/current.next-$deploy_id" && ! -L "/var/www/campus-admin/current.next-$deploy_id" ]]
ln -s "$new_admin" "/var/www/campus-admin/current.next-$deploy_id"
[[ "$(readlink -f /var/www/campus-admin/current)" == "$actual_old" ]]
mv -Tf "/var/www/campus-admin/current.next-$deploy_id" /var/www/campus-admin/current
switched=1
nginx -t
curl -fsS --max-time 10 https://zhanduiwang.top/ >/dev/null
success=1
trap - EXIT
printf 'ADMIN_DEPLOY_OK old=%s new=%s\n' "$actual_old" "$new_admin"
```

执行：

```powershell
$deployId=(Get-Content -Raw '.release\external-takeout-deploy-id.txt').Trim()
$oldAdmin=(Get-Content -Raw '.release\external-takeout-old-admin.txt').Trim()
function Get-ReleaseHash([string]$leaf){
  $rows=@(Import-Csv '.release\external-takeout-release-hashes.csv'|Where-Object{(Split-Path $_.Path -Leaf) -eq $leaf})
  if($rows.Count -ne 1){throw "manifest 文件不唯一：$leaf"}
  $hash=$rows[0].Hash.ToLowerInvariant()
  if($hash -notmatch '^[0-9a-f]{64}$'){throw "manifest SHA 非法：$leaf"}
  return $hash
}
$adminSha=Get-ReleaseHash 'campus-admin-external-takeout.tgz'
$scriptText=(Get-Content -Raw '.release\deploy-external-takeout-admin.sh') -replace "`r`n?","`n"
$script=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($scriptText))
$key='C:\Users\33141\.ssh\id_ed25519_zhandui';$target='admin@8.138.227.107'
ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes $target "echo '$script' | base64 -d | sudo -n bash -s -- '$deployId' '$oldAdmin' '$adminSha'"
if($LASTEXITCODE -ne 0){throw 'Admin 原子切换失败；检查旧 symlink 恢复结果'}
```

Expected: 输出 `ADMIN_DEPLOY_OK`；失败则切回 `$oldAdmin`。成功后保留旧 symlink 目标作为独立回滚点。

- [ ] **Step 6: 固化部署后人工回滚脚本，健康路径只验证而不执行**

创建 `.release/rollback-external-takeout.sh`（不加入 Git）。旧 backend 不认识 `systemKey/operatingStatus`，因此脚本必须先关闭系统分类并安装与 Step 4 相同的五个数据库 guard，再切旧 backend/Admin；它不删除 031 新列、系统分类、商家运营数据或备份表：

```bash
#!/usr/bin/env bash
set -euo pipefail
deploy_id="$1"; old_backend="$2"; old_admin="$3"
[[ "$deploy_id" =~ ^[0-9]{8}_[0-9]{6}$ ]]
new_backend="/opt/campus-circle/releases/$deploy_id"
new_admin="/var/www/campus-admin/releases/$deploy_id"
backup="/opt/cutover-backups/external-takeout-$deploy_id"
mysql_cli=(mysql --defaults-extra-file=/etc/campus-circle/mysql-admin.cnf --batch --skip-column-names)
[[ "$old_backend" =~ ^/opt/campus-circle/releases/[A-Za-z0-9._-]+$ ]]
[[ "$old_admin" =~ ^/var/www/campus-admin/releases/[A-Za-z0-9._-]+$ ]]
[[ -d "$old_backend" && -d "$new_backend" && -d "$old_admin" && -d "$backup" ]]
[[ -L /opt/campus-circle/current && -L /var/www/campus-admin/current ]]
exec 9>/run/lock/campus-api-deploy.lock
flock -n 9
exec 8>/run/lock/campus-admin-deploy.lock
flock -n 8
[[ "$(readlink -f /opt/campus-circle/current)" == "$new_backend" ]]
current_admin="$(readlink -f /var/www/campus-admin/current)"
[[ "$current_admin" == "$new_admin" || "$current_admin" == "$old_admin" ]]
if [[ "$current_admin" == "$new_admin" ]]; then [[ -d "$new_admin" ]]; fi
[[ "$("${mysql_cli[@]}" campus_circle -e "SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND trigger_name LIKE 'external_takeout_rollback_guard_%'")" == 0 ]]
category_enabled="$("${mysql_cli[@]}" campus_circle -e "SELECT enabled FROM navigation_categories WHERE system_key='external_takeout_merchants'")"
[[ "$category_enabled" =~ ^[01]$ ]]
printf '%s\n' "$category_enabled" >"$backup/category-enabled-before-manual-rollback"
wait_local_health(){
  local attempt
  for ((attempt=1;attempt<=30;attempt++)); do
    if systemctl is-active --quiet campus-api && curl -fsS --max-time 3 http://127.0.0.1:8080/healthz >/dev/null; then return 0; fi
    sleep 1
  done
  return 1
}
stop_api_and_confirm_inactive(){
  local attempt
  systemctl stop campus-api || return 1
  for ((attempt=1;attempt<=10;attempt++)); do
    if ! systemctl is-active --quiet campus-api; then return 0; fi
    sleep 1
  done
  return 1
}
success=0; backend_switched=0; admin_switched=0; guard_started=0
restore_new(){
  if [[ "$success" != 1 ]]; then
    set +e
    restore_ready=1
    if ! stop_api_and_confirm_inactive; then restore_ready=0; fi
    if [[ "$restore_ready" == 1 && "$backend_switched" == 1 && "$(readlink -f /opt/campus-circle/current)" == "$old_backend" ]]; then
      if [[ ! -e "/opt/campus-circle/current.restore-$deploy_id" && ! -L "/opt/campus-circle/current.restore-$deploy_id" ]]; then
        ln -s "$new_backend" "/opt/campus-circle/current.restore-$deploy_id"
        mv -Tf "/opt/campus-circle/current.restore-$deploy_id" /opt/campus-circle/current
      else
        restore_ready=0
      fi
    fi
    if [[ "$restore_ready" == 1 && "$admin_switched" == 1 && "$(readlink -f /var/www/campus-admin/current)" == "$old_admin" ]]; then
      if [[ ! -e "/var/www/campus-admin/current.restore-$deploy_id" && ! -L "/var/www/campus-admin/current.restore-$deploy_id" ]]; then
        ln -s "$new_admin" "/var/www/campus-admin/current.restore-$deploy_id"
        mv -Tf "/var/www/campus-admin/current.restore-$deploy_id" /var/www/campus-admin/current
      else
        restore_ready=0
      fi
    fi
    [[ "$(readlink -f /opt/campus-circle/current 2>/dev/null)" == "$new_backend" ]] || restore_ready=0
    if [[ "$admin_switched" == 1 ]]; then [[ "$(readlink -f /var/www/campus-admin/current 2>/dev/null)" == "$new_admin" ]] || restore_ready=0; fi
    if [[ "$restore_ready" == 1 && "$guard_started" == 1 ]]; then
      if ! "${mysql_cli[@]}" campus_circle -e "DROP TRIGGER IF EXISTS external_takeout_rollback_guard_category_update; DROP TRIGGER IF EXISTS external_takeout_rollback_guard_category_delete; DROP TRIGGER IF EXISTS external_takeout_rollback_guard_item_insert; DROP TRIGGER IF EXISTS external_takeout_rollback_guard_item_update; DROP TRIGGER IF EXISTS external_takeout_rollback_guard_item_delete; UPDATE navigation_categories SET enabled=$category_enabled WHERE system_key='external_takeout_merchants';"; then restore_ready=0; fi
    fi
    if [[ "$restore_ready" == 1 && "$guard_started" == 1 ]]; then
      [[ "$("${mysql_cli[@]}" campus_circle -e "SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND trigger_name LIKE 'external_takeout_rollback_guard_%'")" == 0 ]] || restore_ready=0
      [[ "$("${mysql_cli[@]}" campus_circle -e "SELECT enabled FROM navigation_categories WHERE system_key='external_takeout_merchants'")" == "$category_enabled" ]] || restore_ready=0
    fi
    if [[ "$restore_ready" != 1 ]]; then
      printf 'RESTORE_NEW_AFTER_FAILED_ROLLBACK_ABORTED protection_retained=1 manual_intervention=1\n' >&2
    elif systemctl restart campus-api && wait_local_health; then
      printf 'RESTORE_NEW_AFTER_FAILED_ROLLBACK_OK release=%s\n' "$new_backend" >&2
    else
      if stop_api_and_confirm_inactive; then
        printf 'RESTORE_NEW_AFTER_FAILED_ROLLBACK_HEALTH_FAILED release=%s api=stopped\n' "$new_backend" >&2
      else
        printf 'RESTORE_NEW_AFTER_FAILED_ROLLBACK_HEALTH_FAILED_STOP_FAILED release=%s manual_intervention=1\n' "$new_backend" >&2
      fi
    fi
  fi
}
trap restore_new EXIT
stop_api_and_confirm_inactive
guard_started=1
"${mysql_cli[@]}" campus_circle <<'SQL'
UPDATE navigation_categories SET enabled=FALSE WHERE system_key='external_takeout_merchants';
DELIMITER $$
CREATE TRIGGER external_takeout_rollback_guard_category_update BEFORE UPDATE ON navigation_categories FOR EACH ROW
BEGIN IF OLD.system_key='external_takeout_merchants' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='protected navigation unavailable during rollback'; END IF; END$$
CREATE TRIGGER external_takeout_rollback_guard_category_delete BEFORE DELETE ON navigation_categories FOR EACH ROW
BEGIN IF OLD.system_key='external_takeout_merchants' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='protected navigation unavailable during rollback'; END IF; END$$
CREATE TRIGGER external_takeout_rollback_guard_item_insert BEFORE INSERT ON navigation_items FOR EACH ROW
BEGIN IF EXISTS(SELECT 1 FROM navigation_categories WHERE id=NEW.category_id AND system_key='external_takeout_merchants') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='protected navigation unavailable during rollback'; END IF; END$$
CREATE TRIGGER external_takeout_rollback_guard_item_update BEFORE UPDATE ON navigation_items FOR EACH ROW
BEGIN IF EXISTS(SELECT 1 FROM navigation_categories WHERE id IN (OLD.category_id,NEW.category_id) AND system_key='external_takeout_merchants') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='protected navigation unavailable during rollback'; END IF; END$$
CREATE TRIGGER external_takeout_rollback_guard_item_delete BEFORE DELETE ON navigation_items FOR EACH ROW
BEGIN IF EXISTS(SELECT 1 FROM navigation_categories WHERE id=OLD.category_id AND system_key='external_takeout_merchants') THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='protected navigation unavailable during rollback'; END IF; END$$
DELIMITER ;
SQL
[[ "$("${mysql_cli[@]}" campus_circle -e "SELECT enabled FROM navigation_categories WHERE system_key='external_takeout_merchants'")" == 0 ]]
[[ "$("${mysql_cli[@]}" campus_circle -e "SELECT COUNT(*) FROM information_schema.triggers WHERE trigger_schema=DATABASE() AND trigger_name LIKE 'external_takeout_rollback_guard_%'")" == 5 ]]
if [[ "$current_admin" == "$new_admin" ]]; then
  [[ ! -e "/var/www/campus-admin/current.manual-$deploy_id" && ! -L "/var/www/campus-admin/current.manual-$deploy_id" ]]
  ln -s "$old_admin" "/var/www/campus-admin/current.manual-$deploy_id"
  [[ "$(readlink -f /var/www/campus-admin/current)" == "$new_admin" ]]
  mv -Tf "/var/www/campus-admin/current.manual-$deploy_id" /var/www/campus-admin/current
  admin_switched=1
fi
[[ ! -e "/opt/campus-circle/current.manual-$deploy_id" && ! -L "/opt/campus-circle/current.manual-$deploy_id" ]]
ln -s "$old_backend" "/opt/campus-circle/current.manual-$deploy_id"
[[ "$(readlink -f /opt/campus-circle/current)" == "$new_backend" ]]
mv -Tf "/opt/campus-circle/current.manual-$deploy_id" /opt/campus-circle/current
backend_switched=1
systemctl start campus-api
wait_local_health
nginx -t
curl -fsS --max-time 10 https://zhanduiwang.top/ >/dev/null
[[ "$("${mysql_cli[@]}" campus_circle -e "SELECT enabled FROM navigation_categories WHERE system_key='external_takeout_merchants'")" == 0 ]]
success=1
trap - EXIT
printf 'MANUAL_ROLLBACK_OK backend=%s admin=%s schema=forward-only guard=active directory=hidden\n' "$old_backend" "$(readlink -f /var/www/campus-admin/current)"
```

规范化为 LF 并在远端只做 Bash 语法校验，正常发布路径不得执行回滚：

```powershell
$deployId=(Get-Content -Raw '.release\external-takeout-deploy-id.txt').Trim()
$rollbackText=(Get-Content -Raw '.release\rollback-external-takeout.sh') -replace "`r`n?","`n"
$rollbackEncoded=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($rollbackText))
$key='C:\Users\33141\.ssh\id_ed25519_zhandui';$target='admin@8.138.227.107'
ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes $target "echo '$rollbackEncoded' | base64 -d | bash -n"
if($LASTEXITCODE -ne 0){throw '人工回滚脚本语法校验失败'}
Write-Output 'MANUAL_ROLLBACK_READY_NOT_EXECUTED'
```

只有后续 UAT 发现必须回退的回归，且负责人明确授权后，才执行：

```powershell
$deployId=(Get-Content -Raw '.release\external-takeout-deploy-id.txt').Trim()
$oldRelease=(Get-Content -Raw '.release\external-takeout-old-backend.txt').Trim()
$oldAdmin=(Get-Content -Raw '.release\external-takeout-old-admin.txt').Trim()
$rollbackText=(Get-Content -Raw '.release\rollback-external-takeout.sh') -replace "`r`n?","`n"
$rollbackEncoded=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($rollbackText))
$key='C:\Users\33141\.ssh\id_ed25519_zhandui';$target='admin@8.138.227.107'
ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes $target "echo '$rollbackEncoded' | base64 -d | sudo -n bash -s -- '$deployId' '$oldRelease' '$oldAdmin'"
if($LASTEXITCODE -ne 0){throw '人工回滚失败，停止后续发布并检查 current symlink 与 campus-api'}
```

Expected: 健康路径只输出 `MANUAL_ROLLBACK_READY_NOT_EXECUTED`；真正执行时必须输出 `MANUAL_ROLLBACK_OK ... guard=active directory=hidden`，并在数据库确认系统分类 `enabled=0`、五个 guard 均存在。若回滚健康检查失败，trap 切回新 release、删除 guard 并恢复原分类开关；数据库始终保留 forward-only schema。成功进入旧版本降级态后，不得手工删除 guard 或开启分类；恢复服务必须另发修复版新 backend，先在分类关闭状态验证，再受控移除 guard、恢复记录的原开关并重做 UAT。

- [ ] **Step 7: 从已校验归档创建 MiniApp 本地预览并验证无数据四态基线**

先从 manifest 复验本机归档，再解压成独立、不可混入后续构建结果的开发者工具导入目录：

```powershell
$deployId=(Get-Content -Raw '.release\external-takeout-deploy-id.txt').Trim()
$miniRows=@(Import-Csv '.release\external-takeout-release-hashes.csv'|Where-Object{(Split-Path $_.Path -Leaf) -eq 'miniapp-external-takeout.tgz'})
if($miniRows.Count -ne 1){throw 'MiniApp manifest 条目不唯一'}
$expectedMiniSha=$miniRows[0].Hash.ToLowerInvariant()
if($expectedMiniSha -notmatch '^[0-9a-f]{64}$'){throw 'MiniApp manifest SHA 非法'}
$actualMiniSha=(Get-FileHash '.release\miniapp-external-takeout.tgz' -Algorithm SHA256).Hash.ToLowerInvariant()
if($actualMiniSha -ne $expectedMiniSha){throw 'MiniApp 归档 SHA 不一致'}
$uploadDir=Join-Path (Resolve-Path '.release') "miniapp-upload-$deployId"
if(Test-Path $uploadDir){throw "MiniApp 导入目录已存在：$uploadDir"}
New-Item -ItemType Directory $uploadDir | Out-Null
tar.exe -xzf '.release\miniapp-external-takeout.tgz' -C $uploadDir
if($LASTEXITCODE -ne 0 -or -not (Test-Path (Join-Path $uploadDir 'project.config.json')) -or -not (Test-Path (Join-Path $uploadDir 'dist\app.json'))){throw 'MiniApp 归档解压或结构校验失败'}
Write-Output "WECHAT_DEVTOOLS_IMPORT=$uploadDir"
```

微信开发者工具导入上一步输出的 `WECHAT_DEVTOOLS_IMPORT` 绝对路径（归档内 `project.config.json` 的 `miniprogramRoot` 指向 `dist/`），只创建本地预览/真机调试码，不上传体验版；在验收记录写入对应 manifest SHA。依次验证：请求进行时可见 `loading`；目录为空时进入 `empty`；断网时进入 `error`；恢复网络点击 retry 回到 `empty`。非空 `ready`、跳转计数和真实跳转必须等 Step 8 新增真实商家后再验证。

- [ ] **Step 8: 完成真实商家 CRUD 和真机闭环，或明确保持 PENDING**

仅当真实 AppID/path 已提供时执行：

1. Admin 新增真实商家（若已有经确认的真实条目则选定它），确认返回 `open+enabled`，学生端无需重新发版即可进入非空 `ready`。
2. 在开发者工具 Console 临时替换原生 API；Taro 4.2 只等待注入的 `options.success/fail`，所以 mock 不返回 rejected Promise，而是保存并延迟调用 `options.fail`：

   ```js
   const originalNavigateToMiniProgram=wx.navigateToMiniProgram
   const takeoutJumpCalls=[]
   let releaseTakeoutJump
   wx.navigateToMiniProgram=(options)=>{
     takeoutJumpCalls.push({appId:options.appId,path:options.path})
     releaseTakeoutJump=()=>options.fail?.({errMsg:'navigateToMiniProgram:fail UAT intercepted'})
   }
   const restoreTakeoutJumpMock=()=>{
     releaseTakeoutJump?.()
     setTimeout(()=>{wx.navigateToMiniProgram=originalNavigateToMiniProgram},0)
   }
   ```

3. 先设为 `resting` 并点击，断言 `takeoutJumpCalls.length===0`；再设为 `open`，快速连点两次，在调用 `releaseTakeoutJump` 前断言长度只增加 1，且记录的 `appId/path` 与专用 Admin GET 完全一致。随后执行 `restoreTakeoutJumpMock()`，等待页面 busy 状态解除再做下一步；这才是“休息零调用/营业单次调用”运行证据。
4. 使用本地预览真机点击营业商家，打开目标正式小程序和指定 path。
5. 改资料后重新进入学生页，显示新值。
6. 改为休息，卡片保留且不调用跳转。
7. 隐藏单个商家后学生页消失；恢复后仍保持原休息状态。
8. 关闭系统分类总开关，确认整个目录立即进入 `empty`；重新开启后商家恢复且仍为原休息状态。
9. 重新营业后可跳转；删除后二次确认并从学生端消失。
10. 使用同一个已授权真实目标重新创建最终基线商家，确认最终状态为 `open+enabled`，供体验版和正式版回归；不得留下为了测试而失效的目标。
11. 无权限或目标不可达时只显示通用安全错误。
12. 页面文案不暗示平台承接第三方订单、支付或配送。

缺真实目标、目标未关联或真机失败时不得执行假数据写入，状态保持“代码/静态验收 PASS，运行时/真机 PENDING”，并记录单一阻塞原因。

- [ ] **Step 9: 微信审核、正式发布和正式版回归**

只有 Step 8 全部 PASS 且已保留一个 `open+enabled` 的真实基线商家后，才在微信开发者工具上传带明确版本号的体验包，在微信公众平台提交审核。审核通过后由有发布权限的账号执行“发布”，再用正式版重复验证目录读取、营业跳转、休息零跳转、单项隐藏/恢复、分类总开关关闭/恢复和 legacy 路由关闭；结束时恢复基线商家与分类均为启用状态。

体验版/预览版通过不等于上线；未审核或未发布时必须写“体验版验收完成，正式发布 PENDING”。审核被拒时记录拒绝原因并返回对应代码/配置任务，不绕过审核。

正式版回归失败时，先用系统分类总开关隐藏整个外卖目录止损，再由有权限账号在微信公众平台按当时可用的官方版本管理流程回退小程序；仅当问题涉及后端或 Admin 时，另经授权执行 Step 6。不得把 MiniApp 平台回退伪装成服务器脚本已完成。

- [ ] **Step 10: 无论 PASS 或 PENDING 都更新验收文档并提交**

`docs/EXTERNAL_TAKEOUT_MERCHANT_DIRECTORY_ACCEPTANCE.md` 追加：deployId、四个 SHA、旧/新 backend/admin release、迁移结果、运行时四态、跳转计数、真机、微信审核/发布状态和回滚点；不记录凭据、OpenID、DSN 或敏感 query。

```powershell
git add docs/EXTERNAL_TAKEOUT_MERCHANT_DIRECTORY_ACCEPTANCE.md
git commit -m "docs: 更新外卖商家目录上线验收状态"
```

- [ ] **Step 11: 最终范围检查**

```powershell
git status --short
git log --oneline -12
```

Expected: 用户原有脏文件仍原样保留；`.release` 仅含被忽略的发布/验收产物；没有 031 rollback、假商家数据、旧外卖路由或凭据进入 Git。只有微信正式版发布并完成回归时才能写“已上线”；否则准确保留对应 PENDING。
