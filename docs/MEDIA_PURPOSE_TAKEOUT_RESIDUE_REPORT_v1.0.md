# Media Purpose Takeout Residue Report v1.0

日期：2026-06-22

结论：**PASS，测试数据与临时约束残留均为 0。**

## 1. 数据残留

```text
acceptance media_assets              0
acceptance media_asset_references    0
acceptance review_projections        0
acceptance review_decisions          0
```

测试使用独立 owner 区间 `99001818～99001917`，清理顺序遵循 decision → reference → projection → asset。

## 2. Schema 残留

```text
chk_media_reference_type_takeout_guard   0
chk_media_reference_type_rollback_guard  0
canonical chk_media_reference_type       1，最终为五值
media_assets.purpose                     varchar(32)，保持原样
media_asset_references.reference_type    varchar(16)，保持原样
```

最终状态为迁移 018 已应用。没有修改任何索引、其它 CHECK 或列定义。

## 3. 文件与范围

- 临时数据库探针文件：已删除；
- worktree 临时 `node_modules` 目录联接：已删除；
- 构建产物、日志、二进制：0；
- `.env`、DSN、token、云凭证：0；
- takeout、A～F、apps、`main.go` 变更：0；
- 迁移 001～016、017 变更：0；
- P0-5 工作：0。
