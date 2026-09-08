# 小程序 AppID 切换 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将当前 MiniApp 工程及后端部署模板统一切换到 `wx065d8f3ce4a056ee`。

**Architecture:** AppID 仍由微信工程 JSON 和后端环境变量分别承载；本次只同步各自的默认配置值，不引入新的运行时配置层。历史证据、测试样例与外部商家跳转数据保持不变。

**Tech Stack:** Taro 4、微信小程序项目配置、Go 环境变量配置、PowerShell/Node.js 静态验证。

---

### Task 1: 更新当前 AppID 配置

**Files:**
- Modify: `apps/miniapp/project.config.json`
- Modify: `apps/miniapp/project.private.config.example.json`
- Modify: `.env.example`
- Modify: `backend/.env.example`

- [ ] **Step 1: 将四处当前配置的旧 AppID 替换为新 AppID**

目标值均为：

```text
wx065d8f3ce4a056ee
```

- [ ] **Step 2: 验证 JSON 与精确配置值**

Run:

```powershell
node -e "JSON.parse(require('fs').readFileSync('apps/miniapp/project.config.json','utf8')); JSON.parse(require('fs').readFileSync('apps/miniapp/project.private.config.example.json','utf8'))"
Select-String -Path '.env.example','backend/.env.example','apps/miniapp/project.config.json','apps/miniapp/project.private.config.example.json' -Pattern 'wx065d8f3ce4a056ee'
```

Expected: Node 退出码为 0，四个文件各命中一次新 AppID。

### Task 2: 执行 MiniApp 回归验证

**Files:**
- Verify only: `apps/miniapp/**`

- [ ] **Step 1: 运行类型检查**

Run: `npm --workspace apps/miniapp run typecheck`

Expected: PASS。

- [ ] **Step 2: 运行生产构建**

Run: `npm --workspace apps/miniapp run build:weapp`

Expected: Taro 微信小程序构建成功，`dist/` 仍由项目既有规则管理。

### Task 3: 审核并提交

**Files:**
- Verify: all changed files

- [ ] **Step 1: 检查差异与用户已有改动隔离**

Run: `git diff --check` 和 `git status --short`

Expected: 本次文件无空白错误；用户已有四个 MiniApp UI 脏文件和 `artifacts/` 不进入本次提交。

- [ ] **Step 2: 精确暂存并提交**

```powershell
git add -- .env.example backend/.env.example apps/miniapp/project.config.json apps/miniapp/project.private.config.example.json docs/superpowers/specs/2026-09-08-miniapp-appid-cutover-design.md docs/superpowers/plans/2026-09-08-miniapp-appid-cutover.md
git commit -m "配置：切换小程序 AppID"
```

Expected: 提交成功，未暂存的用户文件保持原状。
