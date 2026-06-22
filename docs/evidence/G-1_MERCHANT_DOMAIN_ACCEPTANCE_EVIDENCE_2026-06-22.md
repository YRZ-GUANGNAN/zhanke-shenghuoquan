# G-① Merchant Domain Acceptance Evidence — 2026-06-22

## 1. Git 与 schema 门禁

```text
branch: feat/g1-merchant
base:   bc309b6b
required tags: p0-4-media-baseline, infra-automigrate-retired
pre-development max migration: 015
pre-development 016 exists: false
backend/cmd/server db.AutoMigrate count: 0
```

## 2. 真实 MySQL 8.4

执行方式（DSN 从本机环境读取且未输出）：

```powershell
cd backend
$env:TAKEOUT_G1_MYSQL_DSN = '<redacted real MySQL DSN>'
go test ./internal/module/takeout -count=1 -v
```

脱敏结果：

```text
=== RUN   Test00MerchantMigrationRollbackApplyAgainstRealMySQL84
real_mysql_version=8.4.0
migration_apply_rollback_apply=PASS schema_contract=PASS cross_domain_fk=0
--- PASS

=== RUN   TestMerchantLifecycleCASIdempotencyAndLookupAgainstRealMySQL84
lifecycle=pending_review->active->resting->active->suspended->active PASS
rejected_terminal=PASS idempotency=PASS stale_CAS=REJECTED
--- PASS

=== RUN   TestMerchantCompetingCASAgainstRealMySQL84
competing_CAS=PASS success=1 conflict=1 final_status=resting
--- PASS

=== RUN   TestZZMerchantAcceptanceResidueAgainstRealMySQL84
takeout_merchants_residue=0
--- PASS
```

竞争测试的最终状态可为 `resting` 或 `suspended`；验收不依赖调度顺序，只要求恰好一项成功且另一项返回 `ErrConcurrentUpdate`。

## 3. 迁移结构断言

真实库断言项：

```text
columns=12
id=bigint unsigned, auto_increment
merchant_public_id=char(26), NOT NULL
owner_user_id=bigint unsigned, NOT NULL
logo_asset_public_id=char(26), NULL
row_version=bigint unsigned, NOT NULL
required indexes=3/3
public_id unique=YES
status check constraint=1
cross-domain foreign keys=0
```

执行顺序：

```text
existing empty table guard
-> rollback 016
-> assert table absent
-> apply 016
-> assert complete schema contract
-> leave 016 applied
```

## 4. 边界证据

```text
G production cross-domain imports: 0
G production Gin/net/http imports: 0
migration 016 FOREIGN KEY/REFERENCES: 0
backend/cmd/server db.AutoMigrate count: 0
HTTP routes/controllers added: 0
apps/** changes: 0
```

`npm run check:boundaries` 已登记 G 并验证 G 核心不得导入 A～F、Media 或 HTTP。

## 5. 回归与构建

```text
go test ./... -count=1                 PASS
npm run verify                         PASS
  check:boundaries                     PASS
  admin typecheck                      PASS
  miniapp typecheck                    PASS
  backend tests                        PASS
go build ./...                         PASS
npm run build                          PASS
  Taro MiniApp production build        PASS
  Admin Vite production build          PASS
```

MiniApp 构建报告既有 `vendors.js` 体积提示，但编译成功；没有因 G-① 新增前端代码。

## 6. 迁移校验值

```text
016_takeout_merchant.sql
a67e5aa2e890613192888f1e9a7dcf78b953e5d0e13755571e4b42e6b092e70f

rollback/016_takeout_merchant.sql
6de68cc4d039d2929e74a4c53618df3791d17b5b5e081262de827b0731f0c5a8
```

证据不含 DSN、口令、token 或云凭证。
