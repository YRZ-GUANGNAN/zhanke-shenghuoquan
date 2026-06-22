# Media Purpose Takeout Acceptance Evidence — 2026-06-22

## 1. 基线与迁移号

```text
branch=fix/media-purpose-takeout
base=2f5ab340
existing_max_migration=016
017_exists=false
actual_migration=018_media_reference_type_takeout.sql
```

## 2. 只读真实库核查

```text
mysql_version=8.4.0
media_assets.purpose data_type=varchar column_type=varchar(32) nullable=NO
media_asset_references.reference_type data_type=varchar column_type=varchar(16) nullable=NO
chk_media_reference_type=reference_type IN ('post','product','avatar')
purpose_or_reference_check_count=1
```

该结果证明迁移前 reference_type 有 DB 级门禁；同时记录但不处理 purpose 列宽漂移。

## 3. 迁移与 rollback

执行：

```powershell
cd backend
$env:MEDIA_PURPOSE_TAKEOUT_MYSQL_DSN = '<redacted real MySQL DSN>'
go test ./internal/module/media `
  -run '^Test00MediaReferenceTypeTakeoutMigrationAgainstRealMySQL84$' `
  -count=1 -v
```

结果：

```text
real_mysql_version=8.4.0
rollback_with_new_reference=FAIL_CLOSED
mysql_error=3819 Check constraint chk_media_reference_type_rollback_guard is violated
migration_apply_rollback_apply=PASS
purpose_column_unchanged=varchar(32)
reference_type_column=varchar(16)
final_constraint=post,product,avatar,dish,merchant_logo
temporary_guard_constraints=0
```

## 4. 两类真实链路

```text
purpose=dish
  asset_create=PASS
  reference_bind=PASS
  projection=approved
  gateway_visible=PASS

purpose=merchant_logo
  asset_create=PASS
  reference_bind=PASS
  projection=approved
  gateway_visible=PASS
```

调用的是既有 `Service`、`ReviewProjectionService` 与 `MediaGateway`，没有新增替代实现或 Mock 数据库。

## 5. 全量回归

```text
go test ./... -count=1       PASS
npm run verify               PASS
  check:boundaries           PASS
  admin typecheck            PASS
  miniapp typecheck          PASS
  backend tests              PASS
```

## 6. SHA-256

```text
018_media_reference_type_takeout.sql
46d70df17aa3700065309b6a949125ddf10b42be6f14ab27f3e93b19f864aee4

rollback/018_media_reference_type_takeout.sql
f5897da647489f608271db32db8c9870512d9c2d51d38167e370dbeaebd15217
```

所有 DSN、口令和凭证均已排除。
