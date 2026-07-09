# P0 Identity Switch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add server-owned student/rider/merchant identity switching, persist the current identity, enforce takeout rider/merchant eligibility, and replace the miniapp profile's local `activeRole`.

**Architecture:** Domain A owns the current identity on `users.current_identity` and exposes `/api/v1/auth/identity`. Domain A depends only on a narrow eligibility interface; `backend/cmd/server/main.go` wires takeout rider/merchant identity services into that interface so `user` does not import `takeout`. The miniapp keeps using `domainA.token` and replaces local role switching with `GET/POST /auth/identity`.

**Tech Stack:** Go 1.25, Gin, GORM, MySQL 8.4 migrations, React/Taro, TypeScript, existing `code/message/data` response envelope.

---

### Task 1: Migration And Domain Model

**Files:**
- Create: `backend/migrations/029_user_current_identity.sql`
- Create: `backend/migrations/rollback/029_user_current_identity.sql`
- Modify: `backend/internal/module/user/model.go`
- Modify: `backend/internal/module/user/repository.go`

- [ ] **Step 1: Add failing model/repository expectations**

Add tests in `backend/internal/module/user/service_test.go` that expect `IdentityStudent`, `IdentityRider`, `IdentityMerchant`, defaulting to `student`, and repository updates through a new `UpdateCurrentIdentity` method.

- [ ] **Step 2: Add migration 029**

Add a guarded migration that adds `current_identity VARCHAR(24) NOT NULL DEFAULT 'student'` to `users`, backfills empty values, and adds a CHECK for `student/rider/merchant`.

- [ ] **Step 3: Add rollback 029**

Add a rollback that drops `users.current_identity` only when it exists. No cross-domain foreign keys.

- [ ] **Step 4: Extend user model/repository**

Add:

```go
type Identity string

const (
	IdentityStudent  Identity = "student"
	IdentityRider    Identity = "rider"
	IdentityMerchant Identity = "merchant"
)
```

Add `CurrentIdentity Identity` to `User`, and add `UpdateCurrentIdentity(context.Context, uint, Identity) error` to the repository interface and GORM implementation.

### Task 2: Identity Service And Handler

**Files:**
- Create: `backend/internal/module/user/identity.go`
- Modify: `backend/internal/module/user/service.go`
- Modify: `backend/internal/module/user/handler.go`
- Modify: `backend/internal/module/user/service_test.go`

- [ ] **Step 1: Define eligibility port**

Add a user-owned interface:

```go
type IdentityEligibility interface {
	RiderEligible(context.Context, uint64) (IdentityEligibilityResult, error)
	MerchantEligible(context.Context, uint64) (IdentityEligibilityResult, error)
}
```

Use `Allowed`, `Reason`, and optional `Status` in the result.

- [ ] **Step 2: Implement summary**

`IdentitySummary(ctx, userID)` returns:

```json
{
  "currentIdentity": "student",
  "identities": [
    {"identity":"student","available":true,"reason":"","status":"active"},
    {"identity":"rider","available":false,"reason":"rider qualification required","status":"missing"},
    {"identity":"merchant","available":false,"reason":"merchant must be active","status":"pending_review"}
  ]
}
```

If stored identity is invalid or unavailable, fail closed to `student` in the response and persist correction only during explicit switch if needed.

- [ ] **Step 3: Implement switch**

`SwitchIdentity(ctx, userID, target)` validates target. `student` always succeeds. `rider` and `merchant` require eligibility `Allowed=true`; otherwise return `ErrIdentityUnavailable`.

- [ ] **Step 4: Register HTTP**

Under `/auth`, add:

```go
rg.GET("/identity", h.service.AuthRequired(), h.identity)
rg.POST("/identity", h.service.AuthRequired(), h.switchIdentity)
```

Map invalid target to 400, unavailable identity to 403, missing user to 404, and success to the existing response envelope.

### Task 3: Takeout Eligibility Adapter And Composition

**Files:**
- Create: `backend/cmd/server/identity_eligibility.go`
- Modify: `backend/cmd/server/main.go`
- Create or modify: `backend/cmd/server/identity_http_mysql_test.go`

- [ ] **Step 1: Implement adapter in composition layer**

Create an adapter in `cmd/server` that imports both `user` and `takeout`. It calls:

- `takeout.RiderIdentityService.Find(ctx, userID)` and allows only `takeout.RiderActive`.
- `takeout.MerchantIdentityService.FindByOwnerUserID(ctx, userID)` and allows only `takeout.MerchantActive`.

`takeout.ErrNotFound` maps to missing qualification, not 500.

- [ ] **Step 2: Wire main.go**

Build takeout identity services from `takeout.NewRepository(db)` in the composition root and pass the adapter into `user.NewService` via a new option, without changing delivery/takeout runtimes.

- [ ] **Step 3: Add real HTTP/MySQL test**

Test unauthenticated 401, default student summary, rider/merchant unavailable 403, active rider switch success, inactive rider 403, active merchant switch success, non-active merchant 403, and `users.role` unchanged.

### Task 4: Miniapp Identity Client And UI Wiring

**Files:**
- Modify: `apps/miniapp/src/services/a-user.ts`
- Modify: `apps/miniapp/src/types.ts`
- Modify: `apps/miniapp/src/pages/profile/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-rider/index.tsx`
- Modify: `apps/miniapp/src/pages/merchant-home/index.tsx`

- [ ] **Step 1: Add identity types and service methods**

Add `IdentityRoleKey`, `IdentityOption`, `IdentitySummary`, `currentIdentity()`, and `switchIdentity(identity)`.

- [ ] **Step 2: Replace profile local activeRole**

Load identity summary on mount. Use backend `currentIdentity` for the active card. Confirming a switch calls `switchIdentity`, updates state from the returned summary, and redirects to rider/merchant/student page.

- [ ] **Step 3: Connect rider/merchant switch-back**

The existing "切回学生" modal in rider and merchant pages calls `switchIdentity('student')` before redirecting to `/pages/profile/index`.

- [ ] **Step 4: Keep current static UI intact**

Preserve the new `/pages/my-content` stats navigation and existing static cards. For unavailable identities, let backend be the source of truth; show the returned error toast on 403.

### Task 5: Verification And Reports

**Files:**
- Create: `docs/P0-IDENTITY-SWITCH-BACKEND-DEVELOPMENT-REPORT.md`
- Create: `docs/P0-IDENTITY-SWITCH-BACKEND-ACCEPTANCE-REPORT.md`
- Create: `docs/P0-IDENTITY-SWITCH-BACKEND-RESIDUE-REPORT.md`

- [ ] **Step 1: Run focused tests**

Run:

```powershell
cd backend
go test ./internal/module/user ./cmd/server
```

- [ ] **Step 2: Run frontend checks**

Run:

```powershell
npm --workspace apps/miniapp run typecheck
npm --workspace apps/miniapp run build:weapp
```

- [ ] **Step 3: Run repo gates**

Run:

```powershell
npm run check:boundaries
npm run typecheck
npm run test:backend
npm run verify
```

- [ ] **Step 4: Document acceptance**

Reports must include baseline hash, migration 029, qualification authority, API contract, frontend fake-state removal, commands run, any skipped real-MySQL tests with required env var, and residue status.

- [ ] **Step 5: Commit**

Commit on `feat/p0-identity-switch` with a Chinese message and leave the worktree clean.
