# 微信审核代理方法验收修订设计

## 1. 状态与适用范围

本设计修订 `2026-07-16-wechat-review-content-proxy-design.md` 的生产验收方法，只覆盖其中第 12.2、12.3 和第 14 节关于 `HEAD` 观测次数的要求。代理实现、能力令牌、固定版本访问、回调安全、前端 API、旧媒体兼容、数据库结构及迁移边界全部保持不变。

用户已批准继续修复和完成任务。本次不新增业务代码，不新增迁移，不轮换微信或 COS 凭据。

## 2. 生产证据与问题

部署 `20260717_000317-453dd46e5352` 已用全新 PNG 完成真实微信审核：

- 代理 `GET` 返回 200；
- 微信 callback `POST` 返回 200；
- 媒体终态为 `approved / true / final_ready / approved / consumed / none`；
- 数据库唯一行是 `final / final_ready / consumed / approved / 1`；
- 新日志窗口内代理与 callback 非 200、敏感信息和 fatal 均为 0；
- 微信本次没有发送 `HEAD`，因此 `HEAD 200` 观测次数为 0。

原验收把“代理是否支持 HEAD”与“微信本次是否选择 HEAD”绑定为同一个门禁。前者由我方代码控制，后者由外部平台下载器控制。将外部平台未选择某个可选读取方法判为代码故障，会造成已经通过真实审核的正确版本被不必要回滚。

## 3. 方案比较与决策

### 3.1 采用：能力门禁与真实平台门禁分层

- 能力门禁强制证明同一个有效能力令牌同时支持 `HEAD` 和 `GET`，且 `HEAD` 不打开正文。
- 生产平台门禁强制证明微信至少成功执行一次 `GET`、callback 成功、严格审核终态和数据库投影一致。
- 如果生产窗口实际出现 `HEAD`，所有该路径的 `HEAD` 都必须返回 200；`HEAD` 非 200 仍是回滚条件。
- 如果生产窗口没有 `HEAD` 请求，不再把计数为 0 判为故障。

这是推荐且采用的方案。它不降低 HEAD 行为要求，只移除对外部平台请求方法选择的不可控假设。

### 3.2 不采用：重复上传直到微信偶然发送 HEAD

该方案不可重复、会制造无意义审核资产和 COS 对象，也无法证明下一次发布仍会出现 HEAD。

### 3.3 不采用：人工生成生产能力令牌补齐 HEAD 日志

该方案需要额外读取对象 key/version 或引入管理探针，扩大生产凭据与对象身份接触面，并可能把人工流量误认为微信真实行为。本次不新增此类工具或端点。

## 4. 修订后的门禁模型

### 4.1 强制代码能力门禁

以下测试必须在部署前通过：

- 同一个 token 先 `HEAD` 后 `GET` 均为 200；
- 两次请求绑定同一固定 `final_key + final_version_id`；
- `HEAD` 只调用对象元数据读取，正文打开次数为 0；
- `GET` 返回固定长度和 MIME，并完整流式传输；
- 无效或过期 token 在访问 COS 前返回 404；
- `POST` 和精确路径 `OPTIONS` 返回 405，`Allow: HEAD, GET`；
- 代理路径日志不记录 query、token、对象 key/version 或底层错误。

该门禁继续由现有 `wechat_review_proxy_test.go`、路由日志测试、CORS 测试及多轮 shuffle 测试执行。任何失败都禁止部署。

### 4.2 强制公网安全门禁

在真实验收窗口开始前验证：

- 无 token `HEAD=404`；
- 无 token `GET=404`；
- `POST=405`；
- `OPTIONS=405`；
- `/healthz` 与公开商品列表正常；
- 磁盘 SHA 与运行进程 SHA 等于本次构建；
- `campus-api`、Nginx、MySQL、Meilisearch 均 active。

故意产生的 404/405 必须发生在真实验收窗口之前，避免与微信下载失败混淆。

### 4.3 强制真实微信门禁

每次部署必须创建一张全新图片，不能复用既有成功或失败资产。验收窗口只统计该图片产生的真实流量，必须满足：

- 代理 `GET 200 >= 1`；
- callback `POST 200 >= 1`；
- 代理 GET 非 200 为 0；
- callback 非 200 为 0；
- 如果观察到代理 HEAD，则 HEAD 非 200 为 0；HEAD 观测次数允许为 0；
- `state=approved`；
- `canUse=true`；
- `technicalStatus=final_ready`；
- `reviewStatus=approved`；
- `ticketStatus=consumed`；
- `recovery=none`；
- 数据库唯一行是 `final / final_ready / consumed / approved`，且 `applied_decision_version >= 1`；
- 新 Nginx/journald 区间中 token、签名、RawQuery、对象 key/version、正文和 fatal 计数均为 0。

数据库验收仍只允许使用结果资产 ULID 查询状态字段，不得查询 OpenID、对象 key/version、trace 或任何凭据。

## 5. 部署与回滚

重新部署继续使用已验证构建提交 `25231194eacde3eaaf5fb3a567f45f03787e3910` 和 SHA256 `94fa228ae9be8a301db10180ce82268b1bcdfe906a15e5561fd48cd6adcd01a5`，但必须生成新的部署 ID、staging 路径和精确备份目录；不得复用第一次部署的备份身份。

以下任一条件成立必须使用本次精确备份执行 CAS 回滚：

- 启动、健康、服务或磁盘/进程 SHA 失败；
- 真实 GET 缺失或出现非 200；
- callback 缺失或出现非 200；
- 实际观察到的 HEAD 出现非 200；
- 严格审核终态、数据库投影或日志脱敏失败；
- 30 分钟内未进入允许终态，或进入 rejected/review_failed 等失败终态。

仅 `HEAD` 请求数量为 0 不触发回滚。

## 6. 收据与清理

- 保留第一次失败验收与回滚收据，不覆盖历史文件；
- 为本次重新部署写入独立部署、窗口和结果收据；
- 所有门禁通过后，只按收据中的精确路径清理普通 staging 文件；
- 清理前校验非 symlink、owner、mode 和 SHA；
- 可以清理第一次失败部署和本次成功部署的两个精确 staging 文件，但永久保留两个精确备份与 `READY`；
- 不使用通配符、递归删除或“查找最新目录”。

## 7. 完成定义

只有以下条件全部成立才可输出 `FINAL_CLOSEOUT_OK`：

1. 同 token 的 HEAD/GET 代码能力门禁通过；
2. 新部署的公网安全、服务与 SHA 门禁通过；
3. 全新图片产生真实 GET 200 和 callback 200；
4. 严格媒体终态、数据库投影和日志脱敏全部通过；
5. 若真实窗口出现 HEAD，则全部为 200；
6. 精确 staging 清理完成、备份仍完整、源码范围和用户既有改动未被污染。

真实窗口没有 HEAD 请求不再阻止完成，也不得人工补发 HEAD 来伪造微信行为。
