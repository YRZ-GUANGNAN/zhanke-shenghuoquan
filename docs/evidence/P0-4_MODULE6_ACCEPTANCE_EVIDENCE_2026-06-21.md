# P0-4 Module ⑥ Acceptance Evidence

日期：2026-06-21  
范围：MediaGateway 只读 Repository 与 Service  
敏感信息：不包含 DSN、Secret、COS identity 或用户数据。

## 1. 真实 MySQL 8.4

执行：

```text
go test ./internal/module/media -run 'MediaGateway' -count=1 -v
```

脱敏结果：

```text
real database version: 8.4.0
PASS TestMediaGatewayRepositoryAndServiceAgainstMySQL84
PASS TestMediaGatewayFiltersUnapprovedHiddenAndRevokedAgainstMySQL84
PASS TestZZMediaGatewayDatabaseResidueAgainstMySQL84
PASS TestMediaGatewayVisibilityFailClosedMatrix
PASS TestMediaGatewayBuildsOnlyApprovedViews
PASS TestMediaGatewayReviewProjectionAndDTOSecrecy
PASS TestMediaGatewayProductionBoundaryIsReadOnly
PASS TestMediaGatewaySnapshotHasNoStorageIdentityFields
```

## 2. 只读证据

在真实 MySQL 中，调用以下全部 Gateway 服务前后分别重新读取 asset、projection、reference：

- CheckVisibility
- GetVisibleMetadata
- GetReviewProjection
- BuildDisplayView
- ListDisplayViews

三类记录逐字段 `DeepEqual`，包括状态、row_version、updated_at、Final identity、审核投影和引用状态，结果完全一致。

生产文件静态能力扫描结果：

```text
PASS MediaGateway production write/external-capability scan: 0 matches
```

扫描禁止项包括 Create、Save、Update、Delete、Exec、Transaction、COS Put/Copy/Delete、STS、HTTP 与 Gin。

## 3. 可见性矩阵

| 条件 | 预期 | 结果 |
| --- | --- | --- |
| final_ready + final + approved + active + visible | visible | PASS |
| technical status 非 final_ready | not_ready | PASS |
| fixed final identity 缺失 | not_ready | PASS |
| delete requested/deleted | not_ready | PASS |
| projection 非 approved | not_approved | PASS |
| reference 非 active | reference_inactive | PASS |
| parent hidden | parent_hidden | PASS |
| revocation fence 存在 | revoked | PASS |
| purpose 不匹配 | purpose_mismatch | PASS |

真实 MySQL 父对象包含四个引用（approved、rejected、hidden、fenced），列表只返回 approved 且 visible 的一个视图。

## 4. DTO 泄漏检查

序列化后的媒体展示 DTO 不包含：

```text
staging, bucket, objectKey, finalKey, versionId,
secret, ticket, credential
```

Repository 内部快照结构也不承载 StagingKey、FinalKey、StagingVersion、FinalVersion、Secret 或 TicketID。

## 5. 冻结基线

开发前后核对 Module ②～⑤核心生产文件及 009～012 迁移 SHA-256：

```text
PASS frozen Module 2-5 baseline unchanged: 18 files
PASS Module 6 added no migration
```

既有测试未修改。

## 6. 零残留

验收 owner：`99_000_601`、`99_000_602`。

| 表 | 残留 |
| --- | ---: |
| media_assets | 0 |
| media_review_projections | 0 |
| media_asset_references | 0 |
| media_review_decisions | 0 |

## 7. 全量回归

```text
go test ./internal/module/media -count=1  PASS
go test ./... -count=1                    PASS
npm run verify                            PASS
  check:boundaries                        PASS
  admin typecheck                         PASS
  miniapp typecheck                       PASS
  backend go test ./...                   PASS
```
