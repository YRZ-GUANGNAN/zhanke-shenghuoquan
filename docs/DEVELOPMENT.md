# Development Guide

## 固定开发路径

```text
miniapp page -> services/a-user|b-community|c-trade|d-navigation|e-im|g-takeout -> /api/v1
admin page   -> services/f-admin -> /api/v1/admin
HTTP handler -> domain service -> domain repository -> domain-owned tables
```

G 域路由根为 `/api/v1/takeout`。

handler 不得持有 `*gorm.DB`。跨域访问只能调用对方 service 暴露的窄接口，禁止查询或更新对方表。

## 本地运行

```powershell
npm run dev:infra
npm run db:seed
npm run dev:backend
npm run dev:admin
npm run dev:miniapp
```

`db:seed` 按顺序执行所有 SQL。旧原型数据库会由 `003_domain_convergence.sql` 迁移到 A–F 模型；重要环境执行前仍应备份。

## 验证

```powershell
npm run typecheck
npm run test:backend
npm run check:boundaries
npm run build:miniapp
npm run build:admin
```

新增代码前先在 [ARCHITECTURE.md](ARCHITECTURE.md) 确认归属。无法归属已登记业务域（A–F、G）的需求不进入当前系统；新增业务域须先经 ADR 登记。
