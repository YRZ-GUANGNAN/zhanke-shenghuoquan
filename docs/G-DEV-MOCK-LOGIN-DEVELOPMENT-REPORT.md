# DEV-MOCK-LOGIN Development Report

## Scope

This patch is limited to enabling local development mock WeChat login when the backend is explicitly configured with:

- `APP_ENV != prod`
- `ALLOW_MOCK_WECHAT_LOGIN=true`
- `WECHAT_APP_ID=` empty
- `WECHAT_APP_SECRET=` empty

No production path, user login exchange logic, config validation rule, new media module, takeout module, frontend code, migration, or business domain logic was modified.

## Baseline and Gate Check

- Baseline: `master@b2d12d2c docs(G-7): document local login token path`
- `g7a-ui-buyer-baseline`: present
- `rg "db\.AutoMigrate\(" backend/cmd/server`: 0 matches
- Initial worktree status: clean

## File-Level Verification

### `backend/internal/config/config.go`

- `AllowMockWechat` is read from `ALLOW_MOCK_WECHAT_LOGIN` using `envBool(..., false)`.
- `WECHAT_APP_ID` and `WECHAT_APP_SECRET` must be configured together.
- In `prod`, WeChat credentials are required and mock WeChat login is forbidden.
- In non-prod, empty WeChat credentials are accepted only when `AllowMockWechat == true`.
- This patch does not change config validation.

### `backend/internal/module/user/service.go`

- `exchangeCode()` already supports the local mock path.
- When WeChat credentials are empty and `allowMockWechat == true`, it returns `mock_openid_<code>`.
- This patch does not change user login logic.

### `backend/cmd/server/main.go`

- Before this patch, `legacymedia.NewWechatSafety(cfg.WechatAppID, cfg.WechatAppSecret, cfg.WechatAPIBase)` was called unconditionally.
- `wechatMediaSafety` is injected into the legacy media service:
  `legacymedia.NewService(..., wechatMediaSafety, ...)`.
- That legacy media service is used by auth avatar binding, community, and trade legacy media flows.

### `backend/internal/pkg/media/wechat.go` and `service.go`

- `NewWechatSafety()` requires non-empty AppID and Secret.
- `WechatSafety.Submit(ctx, openID, mediaURL) (string, error)` calls WeChat media safety and returns a trace ID.
- The legacy media interface is:
  `type SafetyChecker interface { Submit(context.Context, string, string) (string, error) }`.

## Implementation

### Changed

- `backend/cmd/server/main.go`
  - Replaced unconditional `NewWechatSafety(...)` with `newWechatMediaSafety(cfg)`.
  - Added a helper that injects local no-op safety only under the exact local mock condition.
  - All other cases still call the real `legacymedia.NewWechatSafety(...)`.

### Added

- `backend/internal/pkg/media/noop_safety.go`
  - Adds `LocalNoopSafety`.
  - Implements the same `SafetyChecker` interface.
  - Returns a local `local-noop-...` trace ID.
  - Does not call any WeChat network endpoint.

- `backend/cmd/server/main_test.go`
  - Verifies no-op injection only happens for local mock mode with empty credentials.
  - Verifies real `WechatSafety` path is kept when credentials are present.
  - Verifies empty credentials outside local mock fail.
  - Verifies production empty credentials still use the real path and fail.

- `backend/internal/pkg/media/noop_safety_test.go`
  - Verifies local no-op returns a local trace ID.
  - Verifies empty openID/media URL are rejected.

## Boundary Statement

No files outside the authorized scope were changed except documentation reports under `docs/`.

Production behavior remains unchanged because the real WeChat safety initializer is still used whenever the exact local mock condition is not satisfied.

