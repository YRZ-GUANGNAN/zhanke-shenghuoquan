# D-0 Legacy Takeout Feature Gate Development Report

## 1. Scope and baseline

- Baseline: `master@3563ab79`
- Parent functional baseline: `1fc26659 feat(G-4): add payment scaffold with isolated mock payment`
- Implementation commit: `484975e fix(D-0): gate legacy takeout behind fail-closed feature flags`
- Branch: `feat/d0-legacy-gate`
- Scope: feature gating only. No Delivery domain implementation was introduced.

Preflight passed:

- `g4-payment-baseline` exists.
- `backend/cmd/server` contains zero `db.AutoMigrate(` calls.
- The main worktree was clean before the D-0 worktree was created.
- Migrations 016-022 and every legacy takeout domain/transport implementation file were preserved.

## 2. Verified injection points

The pre-development file audit confirmed:

1. Legacy runtime construction was unconditional in `backend/cmd/server/main.go`.
2. Takeout merchant/dish review targets were unconditionally registered in the shared F review service.
3. Admin, merchant, buyer and rider routes were unconditionally mounted.
4. The unauthenticated WeChat Pay callback was unconditionally mounted.
5. Production configuration unconditionally required complete legacy WeChat Pay credentials.
6. MiniApp entries were injected by `components.tsx` and `pages/profile/index.tsx`; `app.config.ts` owns page registration.
7. Taro build constants are supplied through `apps/miniapp/config/dev.ts` and `prod.ts`.
8. Admin navigation was injected by `apps/admin/src/App.tsx`; Vite exposes `VITE_*` at build time.

## 3. Backend implementation

### 3.1 Configuration

Added:

- `ENABLE_LEGACY_TAKEOUT`, default `false`.
- `ENABLE_DELIVERY`, default `true`, read-only placeholder for D-1.

The production requirement for complete legacy WeChat Pay credentials now applies only when `ENABLE_LEGACY_TAKEOUT=true`.

Existing safety rules remain unchanged:

- Production mock login is still forbidden.
- Production mock payment is still forbidden.
- Mock payment with real or partial payment credentials is still rejected.
- Partial payment credentials remain rejected.

### 3.2 Runtime gate

Legacy services are grouped in a private `legacyTakeoutRuntime`.

`newLegacyTakeoutRuntime(...)` returns `nil` before touching the database, payment provider or F targets when the gate is disabled. When enabled, constructors execute in the original order with their original startup error messages.

The runtime contains:

- merchant and merchant identity services;
- catalog service;
- order and payment services;
- payment provider;
- rider domain and rider transport services;
- admin/buyer query services;
- withdrawal service.

The merchant and dish F targets are registered only inside enabled runtime construction.

`registerLegacyTakeoutRoutes(...)` is a no-op for a nil runtime. When enabled it mounts the unchanged handlers for:

- `/api/v1/admin/takeout/*`
- `/api/v1/takeout/merchant/*`
- `/api/v1/takeout/buyer/*`
- `/api/v1/takeout/rider/*`
- `/api/v1/takeout/payments/wechat/notify`

## 4. Frontend implementation

### 4.1 MiniApp

Added build constant `TARO_APP_ENABLE_LEGACY_TAKEOUT`.

Default false hides:

- the bottom `外卖` tab;
- `Merchant Center`;
- `Rider Center`.

The legacy pages remain registered in `app.config.ts` and all page/service/store source files remain unchanged.

### 4.2 Admin

Added `VITE_ENABLE_LEGACY_TAKEOUT`.

Default false removes the `外卖运营` navigation item and prevents the takeout console branch from rendering. `TakeoutConsole.tsx` and `services/takeout-admin.ts` remain intact.

## 5. Changed files

```text
backend/internal/config/config.go
backend/internal/config/config_test.go
backend/cmd/server/main.go
backend/cmd/server/legacy_takeout_gate_test.go
backend/.env.example
apps/miniapp/config/dev.ts
apps/miniapp/config/prod.ts
apps/miniapp/src/feature-flags.ts
apps/miniapp/src/components.tsx
apps/miniapp/src/pages/profile/index.tsx
apps/admin/src/feature-flags.ts
apps/admin/src/vite-env.d.ts
apps/admin/src/App.tsx
```

## 6. Explicitly unchanged

`git diff` confirmed zero changes under:

- `backend/internal/module/takeout/**`
- `backend/internal/module/admin/takeoutbuyer/**`
- `backend/internal/module/admin/takeoutmerchant/**`
- `backend/internal/module/admin/takeoutrider/**`
- `backend/internal/module/admin/takeoutpayment/**`
- `backend/internal/module/admin/takeoutconsole/**`
- `backend/migrations/**`

No table, migration, state machine, repository, service or legacy HTTP contract was deleted or modified.

