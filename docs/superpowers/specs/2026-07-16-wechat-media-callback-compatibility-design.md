# 微信媒体审核回调兼容修复设计

日期：2026-07-16

状态：方案已确认，等待书面复核

## 1. 背景与现场证据

生产小程序已经完成消息推送服务器配置，微信平台对
`GET /api/v1/miniapp/media/wechat/callback` 的服务器验证返回 HTTP 200。

配置生效后重新创建了真实商品图片资产
`01KXN3VWM68KDFK4KTQECTZZ31`。该资产已完成 COS 直传、固定版本落盘和微信异步审核提交，状态达到：

- `storage_stage=final`；
- `technical_status=final_ready`；
- 审核投影处于待结果状态；
- 商品侧仍不可使用该图片。

2026-07-16 17:24:35，微信服务器从多个地址向生产回调端点发送 POST，请求均通过了 Token 签名校验，但正文解析返回 HTTP 400。由此可以排除域名、HTTPS、Nginx 路由、回调 Token 和签名算法问题，故障边界收敛到实际回调正文与当前严格解析器之间的兼容差异。

当前处理器存在两个脆弱点：

1. 根据 `Content-Type` 二选一使用 JSON 或 XML 解码器；正文格式与请求头不一致时无法恢复。
2. JSON 的 `result.label` 只接受整数；微信消息推送链路可能把同一数值表示为 JSON 数字或十进制字符串。

## 2. 目标

- 同时兼容微信媒体审核回调的 JSON 和 XML 正文。
- JSON `result.label` 同时接受整数和十进制整数字符串。
- 不依赖 `Content-Type` 才能识别正文格式。
- 保持签名、`trace_id`、`suggest` 和审核状态机的现有安全边界。
- 解析失败、字段歧义或服务落库失败时继续 fail-closed，不得错误批准图片。
- 使用新的真实图片完成微信回调、审核通过和可用状态验收。

## 3. 非目标

- 不修改前端 API、页面或构建产物。
- 不修改数据库表、列、索引或迁移。
- 不改变微信 `suggest` 到审核决策的映射规则。
- 不放宽图片固定版本、所有者、用途、引用或公开访问规则。
- 不记录原始回调正文、OpenID、COS 地址或其他敏感数据。
- 不复用配置回调前创建的旧待审资产作为最终通过证据。

## 4. 方案比较与选择

### 4.1 方案一：后端双格式兼容解析（采用）

读取一次受限大小的正文，不依赖请求头，先尝试 JSON，再尝试 XML；JSON 数值字段接受数字或十进制字符串。该方案直接消除协议表示差异，能够继续使用平台当前的“明文模式 + JSON”配置，且不会改变业务契约。

### 4.2 方案二：把微信平台切换为 XML（不采用）

该方案可能绕开 JSON 数值类型差异，但仍依赖平台请求头与正文始终一致。它只是配置层规避，无法修复服务端已知脆弱点，也会让后续维护依赖外部控制台状态。

### 4.3 方案三：临时记录原始回调正文（不采用）

该方案诊断信息最多，但正文可能包含用户标识、审核结果和其他消息数据，增加敏感数据残留与清理风险。现有服务器证据和协议结构已经足以设计确定性兼容修复，无需记录原文。

## 5. 解析器设计

### 5.1 请求处理顺序

回调处理严格按以下顺序执行：

1. 使用现有 `WECHAT_CALLBACK_TOKEN` 校验 `timestamp`、`nonce` 和 `signature`。
2. 签名失败立即返回 HTTP 403，不读取或解析正文。
3. 对正文施加 256 KiB 上限并读取一次；空正文或超限返回 HTTP 400。
4. 将正文交给独立的纯解析函数，不使用 `Content-Type` 决定格式。
5. 去除首尾空白后先尝试 JSON；只有正文不是合法 JSON 时才尝试 XML。
6. 校验 `trace_id` 和 `result.suggest` 非空，并将 `label` 转为整数。
7. 调用现有 `ApplyWechatReviewResult`，沿用审核投影、outbox 和 consumer 状态机。
8. 成功返回 HTTP 200 与纯文本 `success`；业务应用失败返回 HTTP 500。

`Content-Type` 仅作为可观测信息，不参与正确性判断，也不记录原始正文。

### 5.2 JSON 数值兼容

JSON 解析使用一个局部的灵活整数类型，只接受以下两类输入：

```json
{"result":{"suggest":"pass","label":100}}
```

```json
{"result":{"suggest":"pass","label":"100"}}
```

字符串必须去除首尾空白后仍是完整的十进制整数。空字符串、小数、指数、布尔值、对象、数组、越界值或混合字符全部返回解析错误。解析器不对未知字段报错，以兼容微信附带的 `MsgType`、`Event`、`detail`、`errcode` 等协议字段。

### 5.3 XML 兼容

XML 使用独立的线性结构解析 `trace_id`、`result/suggest` 和 `result/label`。标签值按十进制整数解析。正文即使使用 `text/plain`、`application/octet-stream` 或缺失 `Content-Type`，仍应按内容正确识别。

禁止启用任何外部实体、文件读取或网络解析能力。Go 标准库 XML 解码器保持默认安全行为。

### 5.4 审核决策保持不变

本修复不改变现有映射：

- `pass` → `approved`；
- `risky` 或 `review` → `rejected`；
- 其他非空 suggestion → `review_failed`；
- 空 suggestion、空 trace、非法 label 或无法定位审核任务 → 不批准。

`final_ready` 仍只代表固定 COS 版本已准备完毕。只有审核投影成为 `approved`，客户端状态才允许同时满足 `state=approved` 和 `canUse=true`。

## 6. 代码边界

预计只修改：

- `backend/internal/module/media/miniappflow/wechat_callback_handler.go`；
- 新增或补充同包回调处理器测试文件。

解析辅助类型和函数保持在 `miniappflow` 包内，不导出新的公共 API。`Service`、repository、迁移、路由地址、配置项和前端调用全部保持不变。

## 7. 错误与安全语义

| 场景 | HTTP | 数据状态 |
| --- | --- | --- |
| 签名缺失或错误 | 403 | 不读取正文，不写数据库 |
| 正文空、超限或 JSON/XML 均无法解析 | 400 | 不写数据库 |
| `trace_id`/`suggest` 缺失或 label 非法 | 400 | 不写数据库 |
| 审核任务不存在或状态机拒绝 | 500 | 不错误批准，允许微信重试 |
| 合法重复终态回调 | 200 | 现有服务幂等返回 |
| 合法新审核结果 | 200 | 通过 outbox/consumer 推进投影 |

日志只允许记录响应状态、解析格式类别和不含正文的错误类别；不得记录 Token、签名查询串、原始正文、OpenID、trace ID、COS key/version 或签名 URL。

## 8. 测试设计

### 8.1 解析器与 HTTP 单元测试

- 合法 JSON，`label` 为整数。
- 合法 JSON，`label` 为十进制字符串。
- JSON 包含微信标准额外字段时仍能解析。
- 合法 XML 且 `Content-Type` 为 XML。
- 合法 XML 且 `Content-Type` 为 `text/plain` 或缺失。
- 合法 JSON 且请求头误标为 XML时仍按正文解析。
- 错误签名返回 403，服务不被调用。
- 空正文、超限正文、畸形 JSON/XML 返回 400。
- 空 trace、空 suggest、非法或越界 label 返回 400。
- service 返回错误时为 500；成功时为 200 `success`。
- 已终态审核结果的重复回调保持幂等。

### 8.2 本地交付门禁

- `go test ./internal/module/media/miniappflow -count=1`；
- `go test ./cmd/server ./internal/module/media/... -count=1`；
- `git diff --check`；
- `npm run check:boundaries`；
- 确认 `backend/migrations/**` 零 diff。

共享 MySQL 全量测试若仍受现有脏验收库影响，应明确报告该环境限制，不得把局部通过描述为全量通过。

## 9. 部署与真实验收

1. 在本地为 Linux amd64 重新构建静态 API 二进制并记录 SHA256。
2. 在服务器备份当前 API 二进制和 systemd 状态。
3. 原子替换 `/opt/campus-circle/current/bin/campus-api`，重启 `campus-api`。
4. 验证 systemd active、回环 `/healthz`、公网 HTTPS `/healthz` 和公开商品列表。
5. 使用当前真实微信登录重新创建一张无敏感商品测试图。
6. 完成上传授权、COS PUT、complete，并观察新审核任务。
7. 微信 POST 回调必须返回 200，不再出现 400 重试。
8. 资产最终必须同时满足：
   - `state=approved`；
   - `canUse=true`；
   - `technicalStatus=final_ready`；
   - `reviewStatus=approved`；
   - `ticketStatus=consumed`；
   - `recovery=none`。
9. 只有上述条件全部满足后，才继续带图商品创建和管理员审核发布验收。

## 10. 回滚

本修复不含数据库变更。若新二进制启动失败、健康检查失败或回调行为异常：

1. 停止新 API 进程；
2. 恢复部署前备份的 API 二进制；
3. 重启 `campus-api`；
4. 重新验证回环与公网健康检查；
5. 保留待审资产为不可用状态，不做人工批准或数据库直改。

回滚不会影响 Nginx、MySQL、Meilisearch、微信平台配置或 COS 固定对象。

## 11. 验收标准

- 所有新增回调兼容测试通过。
- 本地交付门禁通过，或对既有共享环境阻塞作诚实区分。
- 服务器更新后服务与公开健康检查正常。
- 新建真实测试图片收到微信回调 HTTP 200。
- 新资产严格达到 approved/canUse/final_ready/consumed/none 组合。
- 没有数据库迁移、前端 API 变更或原始回调正文日志。
- 任何非法、模糊或未知正文都不能让图片变为可用。
