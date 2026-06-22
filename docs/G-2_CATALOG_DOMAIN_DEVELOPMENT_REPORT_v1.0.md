# G-② Catalog Domain Development Report v1.0

日期：2026-06-22

分支：`feat/g2-catalog`

基线：`master@9489cc49`

实现提交：`9a021c88 feat(G-2): add catalog domain core and migration 017`

结论：开发完成，范围止于 G-② 后端领域核心；未进入 G-③。

## 1. 前置门禁

| 检查项 | 实际结果 | 结论 |
| --- | --- | --- |
| master 顶部 | `9489cc49` | PASS |
| 基线标签 | `g1-merchant-baseline`、`media-purpose-takeout` 均存在 | PASS |
| 迁移编号 | 开发前最大为 018，017 不存在 | PASS |
| 启动期 DDL | `backend/cmd/server` 中 `db.AutoMigrate(` 为 0 | PASS |
| 主工作区 | clean，位于 master | PASS |

## 2. 实现内容

在 `backend/internal/module/takeout` 新增：

- Catalog 实体：`DishCategory`、`Dish`；
- `DishStatus`、冻结状态图及合法性/终态判断；
- `CatalogRepository` 接口；
- 基于既有 `GormRepository` 的 Catalog 实现；
- `CatalogService` 分类与菜品领域能力；
- 单元、边界和真实 MySQL 8.4 集成测试。

既有 `repository.go` 仅在原文件末尾追加 Catalog GORM 实现；G-① 原有 84 行、接口、状态机、Service 和行为均未修改。这样继续满足既有 G-① 边界测试对 GORM 访问文件的冻结约束。

没有新增 HTTP、Gin Route、Controller、Admin、小程序、F 审核、E 会话、支付或媒体调用。

## 3. 数据模型

### takeout_dish_categories

- G 内部主键：`BIGINT UNSIGNED AUTO_INCREMENT`；
- 公共身份：`category_public_id CHAR(26)` 唯一；
- 商户逻辑引用：`merchant_public_id CHAR(26)`，零外键；
- 名称、排序、`row_version`、UTC 时间戳。

### takeout_dishes

- G 内部主键：`BIGINT UNSIGNED AUTO_INCREMENT`；
- 公共身份：`dish_public_id CHAR(26)` 唯一；
- 商户、分类、图片均为逻辑引用，零外键；
- `price_cents BIGINT UNSIGNED`，Go 类型为 `uint64`，禁止浮点/小数金额；
- 冻结状态、排序、`row_version`、UTC 时间戳。

## 4. 状态机与并发

```text
draft     -> on_shelf
on_shelf  -> sold_out | off_shelf
sold_out  -> on_shelf | off_shelf
off_shelf -> on_shelf | removed
removed   -> terminal
```

菜品写操作以 `expected_status + expected_row_version` 执行 CAS；分类无状态字段，因此以 `expected_row_version` 执行乐观更新。Repository 在事务内使用 `SELECT ... FOR UPDATE`，更新时执行 `row_version = row_version + 1`；影响行数为 0 时返回 `ErrConcurrentUpdate`。目标状态或目标内容已经达到时幂等返回，不重复递增版本。

## 5. 迁移

- 正向：`backend/migrations/017_takeout_catalog.sql`
- 回滚：`backend/migrations/rollback/017_takeout_catalog.sql`
- 新表：`takeout_dish_categories`、`takeout_dishes`
- 外键：0
- 迁移 001～016、018：未修改

SHA-256：

```text
017 forward  7f9962e7fbfa010b5e8cf01ff1cdc5304283022a1215d7849c9ce806acf8a798
017 rollback 61103685bd8d5873b738bb47993c6b92ef93cbdfbbf519f77927210a277aad03
```

## 6. 内部领域能力

分类：`CreateCategory`、`RenameCategory`、`ReorderCategory`、`FindCategory`。

菜品：`CreateDish`、`UpdateDish`、`PutOnShelf`、`MarkSoldOut`、`Restock`、`TakeOffShelf`、`RemoveDish`、`FindDish`。

逻辑引用仅校验格式，不读取商户或媒体表；存在性验证留给未来获授权的组合层。

## 7. 已知事项

- 本模块不验证 `merchant_public_id`、`category_public_id` 或 `image_asset_public_id` 对应实体是否存在，这是冻结边界的预期行为；
- 本模块没有对外入口，权限认证与业务接线尚不可达；
- MiniApp 构建保留既有 `vendors.js` 体积提示，但编译成功，且本模块未修改前端。

