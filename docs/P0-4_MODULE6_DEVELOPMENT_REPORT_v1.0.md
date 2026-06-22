# P0-4 Module ⑥ MediaGateway Development Report v1.0

日期：2026-06-21  
模块：MediaGateway（媒体统一读取网关）  
开发状态：**Development Complete**  
范围结论：仅完成 Module ⑥；未进入 Module ⑦～⑩。

## 1. 实现内容

本模块新增媒体域统一只读入口，聚合以下既有事实：

- `media_assets` 技术完成状态和公开媒体元数据；
- `media_review_projections` 审核投影状态；
- `media_asset_references` 引用状态、父对象可见性和撤销 fence。

Gateway 提供：

- 媒体可见性判断；
- 已可见媒体元数据查询；
- 审核状态投影只读查询；
- 单个引用展示视图构建；
- 父对象多图展示视图列表。

没有注册 HTTP API、Gin Route、Controller、管理端或小程序页面。

## 2. 可见性规则

仅当以下条件全部成立时返回媒体展示 DTO：

```text
asset.technical_status = final_ready
AND asset.storage_stage = final
AND fixed final identity exists
AND delete_requested_at/deleted_at IS NULL
AND projection.status = approved
AND reference.status = active
AND reference.parent_visibility = visible
AND reference.revocation_fence IS NULL
AND reference.reference_type = asset.purpose
```

任何字段缺失、未知或不匹配都按不可见处理。`reviewing/rejected/review_failed`、隐藏父对象、失效引用、撤销 fence、非 final 或删除中的资源均 fail-closed。

## 3. 只读 DTO

新增 DTO：

- `MediaVisibilityView`
- `MediaMetadataView`
- `ReviewProjectionView`
- `MediaReferenceView`
- `MediaDisplayView`

公开 DTO 只包含媒体公开 ID、purpose、MIME、字节数、尺寸、审核状态和引用定位信息，不包含：

- staging key/version；
- final key/version；
- Bucket、Region、COS URL；
- Secret、STS、Credential；
- owner、upload ticket、quota；
- review task ID 或审核 payload。

由于本阶段禁止 HTTP 与外部网络访问，DTO 不生成签名 URL，也不调用 COS GET/HEAD。

## 4. Repository 设计

新增独立只读接口：

```go
type MediaGatewayReadRepository interface {
    FindGatewayAssetSnapshots(ctx, mediaID)
    FindGatewayReferenceSnapshots(ctx, referenceType, referenceID)
}
```

实现特征：

- 仅使用 `SELECT + JOIN + Scan`；
- 使用显式字段白名单；
- SQL 只判断 Final identity 是否存在，不读取 key/version 值；
- Repository 外没有 GORM 访问；
- 没有事务、锁、Create、Update、Delete、Save 或 Exec。

## 5. 内部 Service 接口

```go
CheckVisibility(ctx, mediaID)
GetVisibleMetadata(ctx, mediaID)
GetReviewProjection(ctx, mediaID)
BuildDisplayView(ctx, referenceType, referenceID, slot)
ListDisplayViews(ctx, referenceType, referenceID)
```

审核投影查询返回状态视图，不返回媒体内容；媒体元数据和展示视图只有在完整可见性公式通过后才返回。

## 6. 数据库变更

**无数据库变更。**

- 没有新增迁移；
- 没有修改 009～012；
- 没有修改任何表、索引、约束或状态机。

## 7. 代码清单

新增生产代码：

- `gateway_types.go`
- `gateway_repository.go`
- `gateway_service.go`

新增测试：

- `gateway_service_test.go`
- `gateway_repository_mysql_test.go`
- `gateway_boundary_test.go`

未修改 Module ②～⑤既有生产文件或测试。

## 8. 测试结果

| 测试项 | 结果 |
| --- | --- |
| 可见性完整正向条件 | PASS |
| 非 final / Final identity 缺失 | PASS（不可见） |
| 未 approved / rejected | PASS（不可见） |
| 引用 inactive/revoked | PASS（不可见） |
| 父对象 hidden | PASS（不可见） |
| revocation fence | PASS（不可见） |
| purpose 不匹配 | PASS（不可见） |
| DTO 敏感字段扫描 | PASS |
| Repository 只读能力扫描 | PASS |
| 真实 MySQL Repository/Service | PASS（8.4.0） |
| 调用前后持久化记录逐字段一致 | PASS |
| 测试残留 | PASS（0） |
| `go test ./internal/module/media` | PASS |
| `go test ./... -count=1` | PASS |
| `npm run verify` | PASS |

## 9. 已知限制

- 本阶段只构建展示元数据视图，不提供图片字节流、重定向或签名 URL；这些能力需要后续独立授权。
- `GetReviewProjection` 是媒体域内部只读接口，当前没有 HTTP 暴露；未来接线时仍需在调用层实施用户/管理员权限控制。
- 没有实现引用写入、fence 创建/解除、Consumer 或 GC。

## 10. 开发结论

Module ⑥开发完成。实现保持严格只读、字段最小化与 fail-closed，没有修改冻结模块，没有启动 Module ⑦～⑩。
