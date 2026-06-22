# Media Purpose Takeout Development Report v1.0

日期：2026-06-22

分支：`fix/media-purpose-takeout`

基线：`master@2f5ab340`

实际迁移号：`018`

结论：受控扩展完成；未处理 `purpose` 列宽漂移，未进入 P0-5。

## 1. 只读核查结论

代码基线：

```text
Purpose = post | product | avatar
Purpose.Valid() 仅接受上述三值
Asset.Purpose gorm size = 16
AssetReference.ReferenceType gorm size = 16
```

真实 MySQL 8.4.0：

```text
media_assets.purpose                    varchar(32), NOT NULL, 非 ENUM，无 purpose CHECK
media_asset_references.reference_type   varchar(16), NOT NULL, 非 ENUM
chk_media_reference_type                IN ('post','product','avatar')
```

因此新增值并非纯应用层变更，需要同步扩展 `reference_type` CHECK。`media_assets.purpose varchar(32)` 与 model `size:16` 的既有漂移已登记，但严格排除在本任务之外。

## 2. 实现内容

`backend/internal/module/media/model.go` 仅增加：

```text
PurposeDish         = "dish"
PurposeMerchantLogo = "merchant_logo"
```

`Purpose.Valid()` 更新为接受五个冻结值。`merchant_logo` 长度为 13，小于应用模型 `size:16`。

没有修改上传、Completion、Review Projection、MediaGateway、Consumer、GC 或任何其它媒体逻辑。

## 3. 迁移 018

正向迁移：`backend/migrations/018_media_reference_type_takeout.sql`

- 添加包含五值的临时保护 CHECK；
- 删除旧三值 canonical CHECK；
- 以原约束名重建五值 CHECK；
- 删除临时保护 CHECK。

回滚迁移：`backend/migrations/rollback/018_media_reference_type_takeout.sql`

- 第一步添加旧三值 rollback guard；
- 若存在 `dish/merchant_logo` 引用行，MySQL 在删除扩展约束之前拒绝该步骤；
- 无新类型引用时恢复旧三值 canonical CHECK；
- 不删除、不改写任何数据。

017 当前不存在并继续保留给 G-②。本任务没有占用或创建 017。

## 4. 范围证明

- `media_assets.purpose` 列类型保持真实库既有 `varchar(32)`；
- `reference_type` 保持 `varchar(16)`；
- 索引集合未变化；
- 其它 CHECK 未变化；
- 未修改任何其它列、约束或索引；
- 未修改 takeout、A～F、`main.go` 或迁移 001～016；
- 未新增 HTTP、路由、业务接线或前端代码。

## 5. 已知事项

- `purpose varchar(32)` 与 model `size:16` 的漂移保留给既有 68 项跨域对齐专项；本提交既不放大也不修复该漂移。
- 新 purpose 复用冻结上传/完成链路的非头像默认限制；本任务没有改变这些行为。
