# 配送来源后端上线实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
> 用户已授权接通后端。本次由当前会话按检查点直接执行，不派新代理；附属执行技能不可用时使用现有工具。

**Goal:** 让已上传的2026.10.16.4小程序连接支持订单来源、取餐号和接入能力的真实后端。

**Architecture:** 从生产提交dcf0aa228f71建立独立发布分支，仅复制b419240的配送后端差异及其验证脚本。045已在生产完成，不重复迁移。保留旧release并备份当前数据库，通过原子软链接切换和健康检查完成发布；失败恢复旧后端，不回滚新增列或业务数据。

**Tech Stack:** Go、MySQL、原生systemd部署、SSH、Python、PowerShell。

## 发布边界

- 源分支：`codex/delivery-api-release-20261016`；原开发分支继续保留在a494工作区。
- 应用版本：`2026.10.16.4`；当前生产版本 `2026.10.15.1 / dcf0aa228f71`。
- 目标主机：`admin@8.138.227.107`，域名 `zhanduiwang.top`。
- 原release：`/opt/campus-circle/releases/20261015-media-dcf0aa2`。
- 新release：`/opt/campus-circle/releases/20261016-delivery-api`。
- 远端暂存：`/home/admin/campus-delivery-api-20261016`。
- 新增备份：`/var/backups/campus-circle/20261016-delivery-api-before.sql.gz`。
- 发布脚本：`artifacts/release-delivery-api-20261016.py`；发布回执：`artifacts/backend-release-2026.10.16.4.json`。
- 不引入当前开发分支中未上线的菜品/商家Logo图片审核改动，不修改开关、角色、支付配置或DNS。
- 淘宝闪购、京东和美团接口授权/适配器尚未接入，能力接口必须如实返回未接入。

## Task 1: 准备可追踪源码

- [x] 核对当前生产版本、045结果、旧二进制哈希和工作区隔离。
- [x] 复制配送模型、仓库、服务、商家/骑手/管理DTO、045迁移及其测试，共15个backend文件；内容精确匹配b419240。
- [x] 复制 `scripts/test-delivery-source-mysql.cjs`，只允许127.0.0.1:33308独立MySQL，不读取生产凭据。
- [x] 确认 `git diff dcf0aa228f71 -- backend/internal/module/media` 为空，其余backend差异与配送范围一致。
- [ ] 保存中文提交；固定构建入口将该提交和版本写入二进制。

## Task 2: 本地验证和构建

```powershell
node scripts/test-backend-safe.cjs
node scripts/test-delivery-source-mysql.cjs
git diff --check
.\scripts\build-native-backend.ps1 -Version '2026.10.16.4' -OutputPath 'artifacts/campus-api-delivery-20261016'
```

- [x] 安全后端测试通过；共享MySQL环境变量均清空。
- [x] 独立MySQL验证旧单兼容、字段入库/读回、商家隔离、权限、重复请求与并发去重，随后只清理本次临时库。
- [ ] 构建Linux amd64版本并计算SHA256，上传后再次比对。

## Task 3: 远端预检和切换

- [ ] 发布脚本默认只读；检查既有release/二进制、私有MySQL配置、045字段/索引/约束、应用连接同一数据库及磁盘容量。
- [ ] 保存功能策略、分享配置、运行环境及订单摘要；接口探针仅发GET，不伪造登录或写测试订单。
- [ ] 备份全部数据库，校验完整解压CRC、delivery_orders建表语句和导出完成标记。
- [ ] 将二进制安装到新的独立release，记录版本、提交和哈希，保留旧版本。
- [ ] 原子切换current并重启campus-api；使用有限重试检查版本、healthz、主机readyz、未登录商家/骑手及管理接口401。
- [ ] 检查策略、分享配置、运行环境和原订单内容保持一致。任何发布后验证失败，恢复旧current并重启，保留045结构和备份。

## Task 4: 独立验收

- [ ] 公网 `/version` 返回2026.10.16.4和实际发布提交；`/healthz`成功，主机 `/readyz` 成功。
- [ ] 新增 `/api/v1/delivery/merchant/order-intake` 从旧版404变为受保护的401，证实生产路由已注册且未开放匿名访问。
- [ ] 保存服务器与本地回执，并明确真实微信商家发单/骑手接单验收尚待完成。
- [ ] 不因后端发布而自动切换微信体验版、送审或公开发布小程序。

## 自审

此次发布接通自有配送后端，不声称第三方平台已经授权或自动收单。数据库迁移与应用发布分别记录；没有将401检查等同于已登录用户完整业务验收。若服务需要回退，仅恢复保留的旧二进制，不覆盖线上数据。
