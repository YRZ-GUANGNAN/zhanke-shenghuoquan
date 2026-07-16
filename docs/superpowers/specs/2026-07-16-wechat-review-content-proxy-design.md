# 微信媒体审核私有内容代理设计

日期：2026-07-16

状态：待用户复核

关联设计：

- `docs/superpowers/specs/2026-07-16-wechat-media-callback-compatibility-design.md`
- `docs/superpowers/specs/2026-07-16-wechat-media-callback-security-addendum.md`

## 1. 背景与已确认根因

微信消息推送的新 Token 已完成验证，生产回调 GET 返回 HTTP 200。安全兼容解析器与脱敏日志上线后，新建真实图片于 2026-07-16 19:30:19 收到三路微信回调，固定诊断结果均为：

```text
structure=json_media_error category=platform_download_error
```

该分类对应微信 `mediaCheckAsync` v2 顶层 `errcode=-1008`，即检测服务器无法下载 `media_url`。同一固定版本 COS 对象已使用真实凭据验证：

- 对象存在，内容类型为 `image/png`；
- 全量 GET 返回 HTTP 200，字节数完全一致；
- 对同一个“按 GET 方法生成”的 COS 预签名 URL 发起 HEAD 返回 HTTP 403。

因此当前最强证据是：微信下载器在读取媒体前执行 HEAD 探测，而 COS 查询签名绑定 HTTP 方法，导致 GET 可用但 HEAD 被拒绝。此结论是根据生产回调与方法级探针作出的工程推断；最终以新代理上线后的真实审核结果验证。

## 2. 目标

- 为微信审核提供一个同时支持 HEAD 与 GET 的短期 HTTPS 内容地址。
- COS 存储桶与审核前对象继续保持私有。
- 每个代理令牌严格绑定一个不可变的 `final_key + final_version_id`。
- 兼容微信官方 v2 成功回调、平台错误回调和同一消息地址收到的其他推送。
- 平台错误必须进入 `review_failed` 终态，不能永久停留在 `reviewing`。
- 保持现有小程序 API、前端页面、旧媒体展示兼容和审核 fail-closed 约束。
- 不新增数据库迁移，不新增生产秘密配置。

## 3. 非目标

- 不把 COS 存储桶或单个审核对象设为公共读。
- 不切换到已停止维护的同步图片安全接口。
- 不为审核代理增加通用文件下载能力。
- 不记录代理令牌、COS key/version、回调正文、OpenID、AppID 或微信签名参数。
- 不处理安全模式加密消息；生产平台继续使用已确认的“明文模式 + JSON”。

## 4. 方案比较

### 4.1 短期私有审核代理（采用）

后端生成自有域名上的加密能力 URL。该 URL 在有效期内对同一固定版本对象同时响应 HEAD 和 GET，后端再使用服务身份从私有 COS 读取内容。

优点：不公开审核前图片；HEAD/GET 语义可控；固定版本、过期时间和日志边界可测试；无需数据库迁移或新环境变量。代价是审核下载流量会短暂经过 API 服务。

### 4.2 临时修改对象 ACL 为公共读（不采用）

实现较少，但审核前图片会在一段时间内公开，还需要可靠的 ACL 恢复任务。进程崩溃或任务失败会扩大暴露窗口，不符合现有审核安全边界。

### 4.3 切换旧同步二进制审核接口（不采用）

可以绕开 URL 下载，但接口已停止维护，并会改变当前异步 projection、outbox 和回调契约，不适合作为上线修复。

## 5. 总体架构

新增 `WechatReviewProxy`，同时承担两个窄接口：

1. 实现现有 `WechatReviewURLSigner`，供 `WechatAsyncReviewSubmitter` 生成 `media_url`；
2. 向 Gin 注册一个无用户登录、仅凭短期能力令牌授权的 HEAD/GET 路由。

公开地址固定为：

```text
{MEDIA_PUBLIC_BASE_URL}/api/v1/miniapp/media/wechat/review-content?token={opaque_token}
```

生产已有：

```text
MEDIA_PUBLIC_BASE_URL=https://zhanduiwang.top
```

构造代理时必须把该值解析为 URL，并要求 scheme 为 `https`、host 非空、无 userinfo、query 或 fragment，path 只能为空或 `/`；输出前统一去除末尾 `/`。这避免配置值把能力令牌导向其他路径或主机。

`WechatAsyncReviewSubmitter` 不再把 COS 预签名 URL 直接交给微信，而是调用代理 signer。代理在收到微信请求后，使用 `TencentCompletionStore` 对固定版本执行 HEAD 或 GET。

```text
完成上传
  -> final_key + final_version_id
  -> 生成 35 分钟加密代理令牌
  -> 提交 mediaCheckAsync
  -> 微信 HEAD 代理地址
  -> 微信 GET 代理地址
  -> 微信 POST 审核回调
  -> projection 进入 approved / rejected / review_failed
```

## 6. 能力令牌设计

### 6.1 密钥

不新增环境变量。使用 `WECHAT_APP_SECRET` 经过带版本域分隔的 SHA-256 派生 32 字节 AES 密钥：

```text
SHA256("campus/wechat-review-proxy/v1\x00" || WECHAT_APP_SECRET)
```

派生密钥只在进程内存在，不写日志、不写数据库。微信 AppSecret 轮换会使尚未过期的代理令牌失效；最大影响窗口受 35 分钟 TTL 限制。

### 6.2 格式

使用 AES-256-GCM：

- nonce：`crypto/rand` 生成；
- AAD：固定值 `campus/wechat-review-proxy:v1`；
- 明文：严格 JSON，包含 `version`、`expiresAt`、`objectKey`、`versionId`；
- 输出：`base64.RawURLEncoding(nonce || ciphertext)`。

令牌不得包含 OpenID、用户 ID、回调 Token、COS 凭据或媒体 URL。虽然 object key/version 被加密，服务端仍必须把它们当敏感内部标识，不得记录。

### 6.3 校验

请求到达后按以下顺序校验：

1. `token` 非空且长度不超过 2048 字符；
2. Base64URL、nonce 长度和 GCM 解密有效；
3. JSON 只允许既定字段，版本必须为 1；
4. `expiresAt` 未过期，且不得超出当前时间 36 分钟以上；
5. key/version 非空且长度分别不超过 512/128；
6. key 必须是规范相对路径、以 `final/` 开头、不得含 `..`、反斜杠或空段；
7. 扩展名仅允许 `.jpg`、`.jpeg`、`.png`、`.webp`。

任何令牌失败统一返回 404，避免形成解密或对象存在性探针。

## 7. HEAD/GET 路由行为

路由只接受 HEAD 和 GET，不使用小程序 JWT。能力令牌就是唯一授权。

### 7.1 共同校验

- 解密令牌并校验固定 key/version；
- `HeadObject(key, versionId)` 必须成功；
- 返回对象的 key/version 必须与令牌一致；
- 内容类型仅允许 `image/jpeg`、`image/png`、`image/webp`；
- 大小必须在 `1..10 MiB`，与微信接口限制一致。

响应头固定包含：

```text
Content-Type
Content-Length
Cache-Control: private, no-store
X-Content-Type-Options: nosniff
```

### 7.2 HEAD

校验和 HEAD COS 对象后直接返回 HTTP 200，不打开对象正文。

### 7.3 GET

使用相同固定 versionId 打开对象并流式返回。不得跟随客户端提供的 key、version 或 Range 参数。正常响应为 HTTP 200 完整正文。

### 7.4 错误映射

- 无效/过期令牌、对象不存在、身份不一致：404；
- 合法令牌对应的 COS 临时故障：503，允许微信重试；
- 不支持的方法：405；
- 已开始输出后的流错误只写固定低基数类别，不记录底层错误或对象标识。

## 8. 回调协议收口

当前成功 payload 解析保持兼容，同时显式解析官方外层字段：

- `MsgType`；
- `Event`；
- `errcode`；
- `trace_id`；
- `result.suggest`；
- `result.label`。

新增字段一旦参与决策，也必须使用现有“关键字段不可重复”规则；JSON 转义重复键和 XML 重复元素继续拒绝。

### 8.1 成功媒体事件

完整官方 envelope 满足：

```text
MsgType=event
Event=wxa_media_check
errcode=0（或为兼容旧包而缺失）
trace_id 非空
result.suggest 非空
result.label 为严格十进制整数
```

继续调用 `ApplyWechatReviewResult`。为保持旧媒体回调兼容，若 `MsgType` 与 `Event` 同时缺失，但 `trace_id + result` 完整有效，也按旧最小 envelope 处理。

### 8.2 平台错误媒体事件

当存在有效 `trace_id` 且 `errcode != 0`：

- `-1008` 映射固定原因 `wechat_media_download_error`；
- 其他非零值映射固定原因 `wechat_media_platform_error`；
- 不要求 `result`；
- projection 幂等进入 `review_failed`；
- 成功落库后返回 HTTP 200 `success`，避免无意义重试。

新增 Service 方法使用封闭失败原因，不把原始 `errmsg` 或任意 errcode 文本写入事件。

### 8.3 其他消息或事件

如果 envelope 明确不是 `wxa_media_check`，返回 HTTP 200 `success`，Service 调用次数为 0。同一消息推送 URL 因此不会对无关消息持续返回 400。

### 8.4 仍需拒绝的输入

- 签名错误：403，且不读取正文；
- 伪装成媒体事件但缺少 trace/result 的成功包：400；
- 重复关键字段、非法 label、畸形 JSON/XML：400；
- 加密 envelope：400，并记录固定 `encrypted_envelope` 类别；
- Service/数据库失败：500，允许微信重试。

## 9. 日志与敏感信息边界

- Nginx 继续使用 `$uri`，不记录 query。
- Gin access logger 的精确跳过路径新增：

```text
/api/v1/miniapp/media/wechat/review-content
```

- Recovery 继续只记录 URL.Path、固定标记和栈。
- 代理不得记录 token、RawQuery、object key/version、COS 错误正文或响应头中的内部标识。
- 回调诊断继续只记录封闭的 `structure/category` 枚举。
- 普通路由和近似路径继续保留访问日志，防止过度跳过。

## 10. 代码边界

计划涉及：

- Create: `backend/internal/module/media/miniappflow/wechat_review_proxy.go`
- Create: `backend/internal/module/media/miniappflow/wechat_review_proxy_test.go`
- Modify: `backend/internal/module/media/miniappflow/wechat_review.go`
- Modify: `backend/internal/module/media/miniappflow/wechat_callback_handler.go`
- Modify: `backend/internal/module/media/miniappflow/wechat_callback_handler_test.go`
- Modify: `backend/internal/module/media/miniappflow/service.go`
- Modify: `backend/internal/module/media/miniappflow/service_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/router_logging_test.go`

不修改：

- `backend/migrations/**`；
- `apps/miniapp/src/**`；
- 对外小程序上传、状态、商品发布、收藏或聊天 API；
- Nginx 路由结构和 TLS 配置。

## 11. 测试策略

### 11.1 令牌与代理

- AES-GCM 往返、随机 nonce、篡改、错误密钥、过期和超长 token；
- object key 路径穿越、空 version、非法扩展名；
- HEAD 返回 200 且不打开正文；
- GET 返回固定版本完整字节；
- HEAD 与 GET 使用同一 token；
- 非法 token 不调用 COS；
- 允许的 MIME/大小边界；
- COS 临时错误映射为 503。

### 11.2 回调

- 官方完整 v2 成功 JSON；
- `errcode=-1008` 且无 result -> `review_failed + 200`；
- 其他非零 errcode -> `review_failed + 200`；
- 普通文本消息、其他 Event -> Service 0 次 + 200；
- MsgType/Event/errcode 重复或类型错误；
- 旧最小成功 envelope 保持兼容；
- terminal projection 重复回调保持幂等；
- 任意恶意正文探针不进入日志。

### 11.3 组合与日志

- `WechatAsyncReviewSubmitter` 提交的 URL 指向代理而非 COS；
- Gin 精确代理路径不记录 query，近似路径仍记录；
- `go test ./cmd/server ./internal/module/media/miniappflow -count=1`；
- `go vet ./cmd/server ./internal/module/media/miniappflow`；
- `npm run check:boundaries`；
- `git diff --check`；
- 全量媒体测试继续诚实区分共享 MySQL 脏数据环境失败。

## 12. 部署、回滚与真实验收

### 12.1 部署

1. 从已提交 commit 构建 Linux amd64 静态二进制；
2. 记录 SHA256，上传到随机私有 staging 路径；
3. 精确备份当前诊断二进制；
4. 原子替换并重启 `campus-api`；
5. 使用有限重试等待 8080 与公网 `/healthz`；
6. 校验磁盘文件与运行进程哈希一致；
7. 不轮换当前回调 Token，因为本次不扩大查询日志面，也不暴露现有 Token。

### 12.2 回滚

任一启动、健康、哈希或真实代理门禁失败，使用本次精确备份恢复诊断版本并重启。不得使用“查找最新备份”的模糊回滚。回滚后停止创建新审核资产并报告 runtime PENDING。

### 12.3 真实验收

必须创建全新测试图片，不复用已经收到 `-1008` 的资产。严格通过条件：

- 微信可对同一代理 URL 完成 HEAD 和 GET；
- 新回调 POST 为 HTTP 200，不再是 400；
- `state=approved`；
- `canUse=true`；
- `technicalStatus=final_ready`；
- `reviewStatus=approved`；
- `ticketStatus=consumed`；
- `recovery=none`；
- journald/Nginx 中无 token、`signature=`、RawQuery、COS key/version 或正文探针；
- `campus-api`、Nginx、MySQL、Meilisearch 保持 active；
- 完成后只删除精确 staging 文件，保留回滚备份。

## 13. 风险与控制

- **令牌重放：** 有效期内只允许读取同一不可变版本；35 分钟后失效，无写能力。
- **API 带宽：** 单图最大 10 MiB，微信正常只执行一次 HEAD/GET；流式传输避免整图驻留内存。
- **随机 token 探测：** 长度限制与 AEAD 在 COS 调用前完成，伪造 token 不触发对象读取。
- **AppSecret 轮换：** 最多影响轮换前 35 分钟内的审核任务，失败回调会进入 `review_failed`，不会误批准。
- **代理自身故障：** 合法 token 的临时存储故障返回 503，允许微信重试；无效能力统一 404。
- **回调歧义：** 参与决策的外层字段纳入重复检测；任何歧义保持 fail-closed。

## 14. 完成定义

只有代码门禁、部署门禁、代理 HEAD/GET 门禁、真实微信回调和严格媒体状态全部通过，方案才算完成。诊断分类命中或本地模拟通过均不能代替真实验收。
