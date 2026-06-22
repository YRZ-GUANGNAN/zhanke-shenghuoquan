# P0-4 Module ③ Development Report v1.0

日期：2026-06-21  
范围：Upload Ticket、原子配额预占与腾讯云 STS 精确单 PUT 授权  
状态：开发完成；未进入 Module ④～⑩。

## 1. 实现内容

新增独立的 Module ③ 组件：

- `upload_types.go`：票据输入、配额策略、上传授权对象和领域错误；
- `upload_repository.go`：票据/配额事务、CAS、幂等、过期与并发配额；
- `upload_service.go`：票据创建、staging Key、授权、uploaded/consumed/cancelled/expired 编排；
- `upload_authorizer.go`：腾讯云 STS 精确 Key、仅 PutObject 的临时策略及服务端预签名；
- `010_media_upload_ticket_service.sql`：配额和过期扫描索引及对应 rollback；
- 单元测试、真实 MySQL 8.4 生命周期测试和可选真实 STS 测试。

没有新增 HTTP Handler、路由、业务域接线或前端页面。

## 2. 数据与事务边界

### 2.1 创建票据

一次数据库事务完成：

```text
用户级 GET_LOCK
  -> 锁定 owner 的 quota reservations
  -> 分钟/小时/日/active 配额检查
  -> media_assets INSERT
  -> media_upload_tickets INSERT
  -> media_quota_reservations INSERT
  -> COMMIT
  -> RELEASE_LOCK
```

- `media_upload_tickets.asset_id` 唯一，保证资产与票据一对一；
- `media_quota_reservations.ticket_id` 唯一，保证票据与配额一对一；
- 资产、票据和 reservation 任一写入失败时整体回滚；
- 用户级 MySQL 命名锁只负责并发串行化，配额事实仍只来自 `media_quota_reservations`；
- 命名锁在固定连接上获取并在事务结束后释放，释放使用独立短超时上下文。

### 2.2 幂等

- `(owner_id, idempotency_key)` 命中且 purpose/size/MD5/MIME 相同：返回原资产、票据与 reservation；
- 相同幂等键但声明不同：返回 `ErrIdempotencyConflict`；
- 过期重试必须使用新幂等键，从而创建新 ULID 资产、新票据和包含 128-bit nonce 的新 staging Key；
- staging Key 不暴露 owner 内部 ID，格式为：

```text
staging/<env>/<ownerShard>/<ownerHash>/<assetULID>/<128bitNonce>.<verifiedExtension>
```

## 3. 配额策略

默认策略与冻结设计一致：

| 窗口 | 数量 | 字节 |
| --- | ---: | ---: |
| active | 20 | 200 MiB |
| 每分钟 | 12 | 100 MiB |
| 每小时 | 60 | 500 MiB |
| 每 24 小时 | 200 | 1 GiB |

窗口统计来自 quota reservations 的状态、`created_at`、`count_reserved` 和 `bytes_reserved`。active reservation 在 consumed 时原子转为 committed，在 cancelled/expired 时原子转为 released。

## 4. 票据状态机与 CAS

```text
created -> authorization_issued -> uploaded -> consumed
   |              |                 |
   +-------------> expired <--------+
   +-------------> cancelled <------+ 
```

- 所有状态 UPDATE 同时匹配主键、`expected_status` 和 `row_version`；
- 成功后执行 `row_version = row_version + 1`；
- `credential_issue_count` 仅在 `created -> authorization_issued` 的同一 CAS 中加一；
- 数据库 CHECK 继续保证 `credential_issue_count <= 1`；
- consume/cancel/expire 在同一事务更新 ticket 与 reservation；
- uploaded、consumed、cancelled 重复提交按目标终态幂等返回；
- authorization 不重放 URL，第二次签发直接拒绝。

## 5. 腾讯云 STS 与精确单 PUT

后端长期身份只用于调用 STS。生成的临时策略：

- action 仅 `name/cos:PutObject`；
- resource 精确到一个 staging Key；
- 不授予 GetObject、DeleteObject、ListBucket、ACL、CopyObject 或任何 multipart 权限；
- STS 临时 SecretKey 仅存在于后端内存，用于生成预签名 URL，不进入返回对象、不写数据库、不写日志；
- 客户端仅获得 HTTPS PUT URL、短 TTL、Content-Type、Content-MD5、Content-Length 和 `x-cos-forbid-overwrite:true`；
- 签名后再次校验 bucket host、对象 path、临时 token、签名和全部必需 signed headers；
- URL TTL 不超过 120 秒，也不超过票据剩余有效期。

若 STS/签名失败，已占用的一次授权票据立即 CAS 到 cancelled，并释放 reservation；不会降级为长期密钥或宽泛 STS 权限。

## 6. 数据库变更

正向迁移：`backend/migrations/010_media_upload_ticket_service.sql`  
回滚迁移：`backend/migrations/rollback/010_media_upload_ticket_service.sql`

新增索引：

- `idx_media_quota_owner_created(owner_id, created_at, status)`；
- `idx_media_ticket_expiry_scan(status, expires_at, ticket_id)`。

未新增表，未修改 A/B/C/F 域表，未新增跨域外键。

## 7. 测试结果

| 项目 | 结果 |
| --- | --- |
| Migration 010 | PASS |
| Rollback 010 | PASS |
| Rollback 后重新 Migration | PASS |
| MySQL 版本 | 8.4.0 |
| 创建三表原子事务 | PASS |
| 相同声明幂等返回 | PASS |
| 幂等键声明冲突 | PASS |
| owner 越权拒绝 | PASS |
| 授权次数限制 | PASS |
| uploaded→consumed + quota committed | PASS |
| cancel/expire + quota released | PASS |
| 过期后新票据/新资产/新 Key | PASS |
| 20 路并发配额测试 | PASS：5 成功、15 quota rejected、0 unexpected、active=5 |
| 测试数据清理 | PASS：残留 0 |
| 精确签名结构测试 | PASS |
| `go test ./...` | PASS |
| `npm run verify` | PASS |

## 8. 明确未实现

- COS Head/校验、CopyObject 和 final 对象；
- 内容安全审核或审核投影；
- MediaGateway；
- Outbox/Inbox 消费；
- GC；
- HTTP API、小程序上传页、管理端；
- 社区、交易、用户或后台业务接线。

## 9. 已知问题

- 当前环境没有新的腾讯云长期测试配置，真实 STS 测试按规则 SKIP；此前临时凭证已过期且未复用。Gate-0 的真实精确单 PUT结论仍作为入口策略基线，但新 Module ③ 适配器需在获得新测试配置后再跑一次真实 STS 用例。
- `uploaded` 在本模块仅表示上传完成声明，不代表 COS 内容已可信校验；Module ④ 验证前不得把它视为 approved/final，也不得公开展示。
- 用户级 `GET_LOCK` 是 MySQL 8.4 专用并发机制；数据库迁移到其他引擎前必须重新评审原子配额方案。
- 旧 `internal/pkg/media` 仍未接入新服务；本阶段没有修改 composition root，避免同时启用两套写路径。

