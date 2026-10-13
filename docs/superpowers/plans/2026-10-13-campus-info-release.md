# 校园资讯版本实施计划

> 已撤销，禁止继续执行本计划的固定范围、上传或提审步骤。用户已纠正需求，后续按 `2026-10-13-independent-campus-module.md` 实施。此文保留为历史记录。

**目标：**交付真实可读的校园指南和固定业务范围，并完成本次代码审核提交。

**架构：**独立 campusinfo 只读内容模块；featureflags 发行范围交集；小程序目录/详情和已有账号服务；Admin 显示不可复开的范围。

**技术栈：**Go/Gin、Taro/React、TypeScript、现有 Admin 和 MySQL。

## 任务

- [ ] `backend/internal/module/campusinfo/{content.json,handler.go,handler_test.go}`：嵌入五篇核验内容，GET `/campus-info/guides` 与 `/:id`；非法/未知 ID 返回 404，内容缺损启动失败。
- [ ] `backend/internal/module/featureflags/{service.go,release.go,release_test.go,handler.go}` 与 `backend/cmd/server/main.go`：启用 `campus_info`，所有可选能力 false；存储原值保留，写入 true 返回 400，历史订单 API 维持既有权限。
- [ ] `apps/admin/src/{FeaturePolicyConsole.tsx,types.ts}`：展示发行范围并锁定开关，保留服务器异常/冲突提示。
- [ ] `apps/miniapp/src/services/{feature-policy-model.ts,campus-info.ts,share-model.ts}`：本包范围交集，内容读取与载荷校验，安全指南分享。
- [ ] `apps/miniapp/src/pages/{home,campus-guide}`：目录、分类、加载/空/错误/无效链接、全文、来源复制、反馈和首页返回；复用胶囊与底栏。
- [ ] `apps/miniapp/src/pages/{about,legal,settings}`：真实服务范围和设置显隐。原始账号注销行为保持不变。
- [ ] 同步路由、边界与旧首页检查；新契约断言禁止首页导入 UGC/商品/聊天服务，验证全开远端策略也不能复开。不得保留死代码满足失效的断言。
- [ ] 运行 `npm.cmd run test:backend`、相关 Node 检查、两端 typecheck/build 和 `npm.cmd run verify`。MySQL 专用测试仅使用临时隔离实例。
- [ ] 真实开发工具验证首页和详情、复制来源、分享路由、受限旧链接；保存截图。
- [ ] 独立 release 部署 API/Admin，核验版本、健康、内容和权限；构建上传 `2026.10.13.1`（未占用时），备注“优化体验”；如实填写微信审核并回读状态。
- [ ] 精确提交本轮文件并推送当前分支，记录证据、回退路径和未完成设备验收。

## 核心行为断言

```ts
const response = { flags: Object.fromEntries(FEATURE_KEYS.map(key => [key, true])), enabled: true, postingMode: 'all_users', rowVersion: 1 }
const view = parseFeatureVisibility(response)
for (const key of FEATURE_KEYS) assert.equal(view[key], false)
assert.equal(view.available, true)
```

```go
Configure(true)
RestrictToCampusInfo()
for _, key := range AllKeys {
    if Can(key) { t.Fatalf("release reopened %s", key) }
}
```

自审：每个设计目标均对应内容、范围、UI、文案、验证或发布任务；不引入新权限和无关业务重构。
