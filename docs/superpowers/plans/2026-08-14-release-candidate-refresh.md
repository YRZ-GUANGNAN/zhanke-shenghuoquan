# 发布候选刷新与交付收口实施计划

> 日期：2026-08-14
>
> 工作分支：`codex/rider-refresh-home-feed`
> 原则：保留现有业务功能与用户文件；不上传微信版本；不写共享数据库；只处理可由代码和只读验证确定的问题。

## 目标

把当前小程序、管理后台与后端收敛为一份可交给用户最终审查的发布候选：自动化检查与构建无阻塞，生产包不包含本地服务地址或静态假页面，公开服务健康状态有当日证据；必须依赖两台真机、两个真实账号、微信开发者权限或生产凭据的项目明确标记为 `PENDING`，不以构建通过代替实机验收。

## 任务 1：冻结基线与边界

**检查文件：**

- `package.json`
- `apps/miniapp/package.json`
- `apps/admin/package.json`
- `.gitignore`
- `docs/RELEASE-CANDIDATE-2026-07-31.md`

**执行：**

```powershell
git status --short
git branch --show-current
git rev-parse HEAD
git diff --check
npm run check:real-wechat-secret-hygiene
```

确认工作树仅包含本轮文件与用户自有未跟踪文档；不暂存、不编辑无关文件。若密钥卫生失败，先定位并移除仓库中的真实密钥暴露，再继续。

## 任务 2：运行全量静态契约、类型检查与安全后端测试

**执行：**

```powershell
npm run verify
```

该命令覆盖静态假页面、发布、二手留言与私信、头像、身份切换、胶囊布局、首页信息流、商家/骑手工作台、管理后台、生产部署边界、类型检查和 `scripts/test-backend-safe.cjs`。任何失败都必须归类为：

1. 当前代码阻塞：在本分支修复并重跑；
2. 既有环境阻塞：记录精确错误与验证边界，不伪造通过；
3. 外部状态阻塞：保留 `PENDING`，不擅自部署或写生产数据。

## 任务 3：生成并审计生产构建

**执行：**

```powershell
npm run build:miniapp
npm run build:admin
rg -n "127\\.0\\.0\\.1|localhost|dev-api|mock-api" apps/miniapp/dist apps/admin/dist
git diff --check
```

允许记录体积或兼容性警告，但生产包内若出现本地 API、开发模拟接口或会显示给用户的测试占位数据，视为发布阻塞并修复。构建产物不纳入提交，除非仓库现有规则明确跟踪它们。

## 任务 4：只读核验公开服务

**检查地址：**

- `https://zhanduiwang.top/healthz`

使用网络只读请求记录 HTTP 状态、响应内容与北京时间。不得通过该步骤触发发布、审核、身份变更、数据写入或管理后台操作。

## 任务 5：形成当日发布候选报告

**创建文件：**

- `docs/RELEASE-CANDIDATE-2026-08-14.md`

报告必须包含：

- 分支与提交基线；
- 每条自动化命令和真实结果；
- 生产构建与本地地址扫描结果；
- 公开健康检查的当日证据；
- 已知非阻塞警告；
- 两账号双向私信、退出重进历史、iOS/Android 胶囊和键盘、帖子/商品跨账号可见、商家/骑手权限门禁等最终人工验收清单；
- 明确声明“尚未上传微信版本”。

## 任务 6：提交交付证据

**执行：**

```powershell
git status --short
git diff -- docs/superpowers/plans/2026-08-14-release-candidate-refresh.md docs/RELEASE-CANDIDATE-2026-08-14.md
git add -- docs/superpowers/plans/2026-08-14-release-candidate-refresh.md docs/RELEASE-CANDIDATE-2026-08-14.md
git diff --cached --check
git commit -m "交付：刷新发布候选验收基线"
```

如果验证过程中修复了确定性代码阻塞，只暂存对应源码、测试与上述两份交付文档；三份用户自有未跟踪计划文档始终排除。提交后再次确认工作树中没有混入无关改动。
