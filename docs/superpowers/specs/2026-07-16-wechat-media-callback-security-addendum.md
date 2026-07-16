# 微信媒体审核回调安全复核补充设计

日期：2026-07-16

状态：实施期强制纠偏；补充内容直接落实已确认设计中的“解析歧义 fail-closed”和“不得记录签名查询串”，不改变产品范围或公开契约。

关联设计：`docs/superpowers/specs/2026-07-16-wechat-media-callback-compatibility-design.md`

## 1. 复核证据

独立安全复核确认两项发布阻断：

1. `backend/cmd/server/main.go` 使用 `gin.Default()`。Gin v1.10 默认 Logger 会把 `RawQuery` 拼入路径，生产 journald 自 17:00 起已出现 5 条包含媒体回调 `signature=` 的访问日志。HTTPS Nginx 使用 `$uri`，未出现该泄漏。
2. Go 标准 JSON/XML 结构体解码会接受重复字段并通常以后值覆盖。重复 `trace_id`、`result`、`suggest` 或 `label` 属于有歧义输入，不能进入审核 Service。

另有一项配置缺口：Nginx 80 端口 server 未显式使用 `campus` 日志格式，可能继承默认 combined 格式并记录完整 `$request`。

## 2. 决策

### 2.1 重复关键字段必须拒绝

JSON 和 XML 都采用 token 流式解析。只对协议关键字段实施唯一性约束：

- 根对象/根元素：最多一个 `trace_id`、最多一个 `result`；
- `result` 内：最多一个 `suggest`、最多一个 `label`；
- 任一关键字段重复即返回 HTTP 400，Service 调用次数为 0；
- 未知普通字段继续忽略；
- JSON 合法但语义非法时不得回退 XML；
- JSON 转义键名解码后参与重复判断，例如 `trace\u005fid` 与 `trace_id` 视为同一字段；
- XML 关键字段只允许位于预期层级，嵌套在未知元素中的同名字段不得提升为有效值；
- XML DTD、directive 与处理指令全部拒绝，不启用外部实体。

`label` 继续只接受非负十进制整数或仅含十进制整数的 JSON 字符串。缺失、`null`、正负号、小数、指数、布尔、对象、数组和溢出值全部拒绝。

### 2.2 Gin 不记录媒体回调查询串

用 `gin.New()` 替代 `gin.Default()`，按相同顺序显式挂载：

1. `gin.LoggerWithConfig`；
2. `gin.Recovery()`；
3. 现有 CORS middleware。

Logger 使用 `SkipPaths` 精确跳过：

- `/api/v1/miniapp/media/wechat/callback`；
- `/api/v1/media/wechat/callback`。

普通路由和近似路径继续使用 Gin 默认日志。媒体回调的路径、状态和耗时仍由 Nginx `campus` 格式记录，且该格式使用 `$uri`，不包含查询参数。

默认 `gin.Recovery()` 在 panic、尤其 broken-pipe 分支可能转储完整请求行，因此同样不能直接使用。Recovery 改为 `gin.CustomRecoveryWithWriter(io.Discard, ...)`：

- Gin 内置请求转储写入 `io.Discard`；
- 自定义恢复日志只记录 `URL.Path`、固定 `panic_recovered` 标记与 `debug.Stack()`；
- 不记录 RawQuery、Authorization、原始 recovered 值或请求正文；
- broken-pipe 分支允许不输出恢复详情，以保密优先；
- 两条精确回调、普通路由和 broken-pipe 均使用带假 `signature=` 的 panic 探针测试。

### 2.3 Nginx 80/443 统一安全日志格式

80 端口 server 显式增加：

```nginx
access_log /var/log/nginx/campus.access.log campus buffer=32k flush=5s;
error_log /var/log/nginx/campus.error.log warn;
```

这保证 HTTP 重定向前的请求也不会把签名查询串写入默认 combined 日志。443 配置保持现状。

### 2.4 上线后再次轮换回调 Token

现有 journald 中已经保存旧签名元组。修复日志后必须轮换 Token，使历史签名失效：

1. 先部署并验证 Gin 与 HTTP/HTTPS Nginx 均不记录探针查询串；
2. 服务器生成新的 32 位纯字母数字 Token，同时更新两个现有 env 文件；
3. 不在终端或聊天中输出 Token，只复制到 Windows 剪贴板；
4. 重启 API；
5. 用户在微信公众平台粘贴新 Token，其他选项保持明文模式 + JSON；
6. 完成 GET 验证后重新创建测试图片；
7. 不删除旧 journal，Token 轮换即完成失效处置。

本次不增加 timestamp 新鲜度窗口或 nonce 持久化，因为微信重试时序没有在当前项目中形成可靠契约；错误设置窗口可能拒绝合法延迟回调。日志消除与 Token 轮换足以关闭本次已确认的暴露面。

## 3. 文件范围

- Modify: `backend/internal/module/media/miniappflow/wechat_callback_handler.go`
- Modify: `backend/internal/module/media/miniappflow/wechat_callback_handler_test.go`
- Modify: `backend/cmd/server/main.go`
- Create: `backend/cmd/server/router_logging_test.go`
- Modify: `deploy/native/nginx-zhanduiwang.top.conf`

不修改数据库迁移、前端、API 路径、状态机或微信控制台除 Token 外的配置。

## 4. 测试门禁

- JSON/XML 重复根字段与 result 字段全部返回 400；
- JSON 转义重复键返回 400；
- XML directive/DTD 返回 400；
- 错误签名严格不读取正文；
- 256 KiB 正文边界覆盖；
- 未知 suggestion 通过真实 Service 进入 `review_failed`，不能 approved；
- approved/rejected/review_failed 三种终态重复回调均幂等；
- Gin 日志探针不出现在两条精确媒体回调路径，普通路由与近似路径仍记录；
- Nginx 80/443 日志新增探针均不包含查询串；
- `backend/migrations/**` 零 diff。

## 5. 发布与回滚纠偏

- staging 使用 `/home/admin` 下带随机部署 ID 的私有普通文件，不使用固定 `/tmp` 路径；
- 部署开始时生成并在本地保留唯一 deploy ID、精确 stage 和精确 backup 路径；
- `current` 必须解析到 `/opt/campus-circle/releases/*`，目标二进制必须是普通文件且非 symlink；
- 记录实现提交、current release、部署前 SHA、staging SHA 和部署后 SHA；
- 备份副本 SHA 必须等于部署前 SHA；
- 回滚只接受本次精确 backup，不通过 `find latest` 猜测；
- journal 检查必须区分 SSH/journalctl 失败与匹配计数，只有远端成功且 `FATAL_COUNT=0` 才通过；
- staging 在本地/公网健康与日志检查全部通过后才删除。

## 6. 验收标准

- 上述代码与配置测试通过；
- 新 API 和 Nginx 配置安全部署，回滚证据完整；
- 日志探针证明新日志不含 `signature=`；
- 回调 Token 再次轮换并由微信平台 GET 验证成功；
- 新建真实图片收到 POST 200，并严格达到 `approved + canUse + final_ready + consumed + recovery=none`；
- 任何重复、畸形或语义不明确正文都不调用审核 Service。
