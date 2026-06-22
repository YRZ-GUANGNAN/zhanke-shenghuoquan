# 开发纪律总约束（DEVELOPMENT DISCIPLINE）

> 建议存放路径：`docs/DEVELOPMENT_DISCIPLINE.md`

本文件固化项目既有的开发与验收纪律，作为**常设治理基线**。其效力高于任何单次模块授权；单次授权与本文件冲突时，以本文件为准。每次模块授权应将本文件作为前置约束引用。

---

## 一、归属与冻结

1. 一次授权只做一个明确模块；无法归属到已登记业务域的需求不进入系统（见 `ARCHITECTURE.md`）。新增或重开业务域须先有 ADR（见 `docs/adr/`）。
2. 已冻结：媒体子系统 Module ②~⑩、迁移 `001~014`（含 `rollback/`）、A~F 各域数据库契约 / 状态机 / Repository 行为 / 已通过验收测试。冻结对象一律不得修改。
3. 模块生命周期：开发 → 真实环境验收 → PASS → 冻结；完成即停止，等待下一次专项授权，不得自行进入下一模块。
4. 新增或重开业务域，必须先有明示的架构决策记录（ADR）并据此修订 `ARCHITECTURE.md` / `DEVELOPMENT.md` / `README.md`，经 owner 接受后方可开发；禁止用与现行架构矛盾的模块绕过登记。

## 二、分层与边界

5. 固定链路：`handler → domain service → domain repository → 域自有表`。handler 不得持有 `*gorm.DB`。
6. Repository 是唯一数据库边界，仅所属域可访问其表；跨域只能调用对方 service 暴露的窄接口，禁止查询或更新对方表，禁止跨域外键。
7. 状态机、权限归属、审核规则一律在 service。
8. 复用而非重建：
   - 身份与权限走 **A**；不自建 token / 权限。
   - 图片资产走**媒体子系统**：通过 `media_asset_references` 引用、展示经 **MediaGateway**（仅 `final + approved`），不另造媒体逻辑。
   - 审核提交 **F** 统一审核流（`review_tasks`：`submitted → reviewing → approved/rejected`），F 经注册领域服务回写、不直接访问业务表。
   - 会话走 **E** 的 `ConversationGateway`，E 不读业务表、不存消息内容。

## 三、实现范式

9. 所有状态推进使用 `expected_status + row_version` 的 CAS；stale write 一律拒绝。
10. 副作用（审核、上传、跨模块通知、支付回调等）走 **Outbox → Consumer**，**Inbox 去重**，保证幂等、防重复、防乱序；崩溃可恢复；死信 / 终态明确。
11. 全程 **fail-closed**：任一条件未知或不满足，按不可见 / 不放行处理。
12. 迁移编号续接现有最大号（当前 `014`，下一条 `015`…），每条迁移必须配 `rollback/`；不得修改已冻结迁移。

## 四、验收闸门（必须真实，禁止构造）

13. 验收针对项目【真实、当前】环境：真实 MySQL 8.4、真实 STS/COS，全链路无 Mock。
14. **反规避（硬性）**：严禁为通过验收而新建合成库 / 干净实例 / 临时库并在其上验收；验收库必须是真实库。
    - 前例：真实库 `media_assets.public_id` 与冻结迁移基线漂移（真实 `varchar(32) NOT NULL`，迁移意图 `CHAR(32) NULL`），曾被“全新迁移库 PASS”掩盖——此类一律禁止。
15. 真实环境与冻结迁移出现漂移（字段、约束、缺表等），必须**停止并上报为阻断项**，申请单独授权去协调；禁止换库或绕过校验规避。
16. 残留为零：数据库残留 = 0，COS Version / DeleteMarker 残留 = 0，临时文件清理干净。
17. 闸门命令全过，缺一不可：

```
    npm run typecheck
    npm run test:backend
    npm run check:boundaries
    npm run build:miniapp
    npm run build:admin
    npm run verify
    go test ./... -count=1
```

## 五、契约与协议

18. 接口返回 `{code, message, data}`；分页 `items / page / pageSize / total`；路由根与域一一对应。
19. 不得修改既有上传协议、审核协议、状态机与 CAS 语义。

## 六、诚实与上报

20. 报告必须反映真实失败阶段与根因；PASS 必须代表真实目标环境通过——不接受构造环境得到的 PASS，不接受把未达标包装成 CONDITIONAL PASS 来推进。
21. 遇到超出授权范围的阻断，停止并请示，禁止自行扩大范围或绕行。

## 七、安全

22. 禁止提交或泄露任何凭证（`.env`、AppSecret、STS/COS Secret、DB 口令、token、私钥、签名 URL）；报告与导出一律脱敏。

## 八、每模块交付物

`Development Report`、`Acceptance Report`、`Acceptance Evidence`；有问题附 `Root Cause Analysis` 与 `Residue Report`。
