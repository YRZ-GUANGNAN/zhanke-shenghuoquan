# DEV Local Experience Development Report

Date: 2026-06-25  
Branch: `feat/dev-local-experience`  
Base: `3d603864 feat(G-7c): add rider miniapp center`  
Final commit: see owner handoff message / `git log --oneline -1`

## Scope

This is a local-development-only patch. It fixes two local mock-mode friction points:

1. local mock WeChat login JWT expires too quickly during manual demos
2. local mock media uploads remain at review projection `submitted`, requiring manual SQL before images become visible

The changes are active only under the exact local mock condition:

```text
APP_ENV != "prod"
ALLOW_MOCK_WECHAT_LOGIN = true
WECHAT_APP_ID = ""
WECHAT_APP_SECRET = ""
```

Production and credentialed paths remain unchanged.

## Pre-gate verification

- `git log --oneline -1 master` returned `3d603864 feat(G-7c): add rider miniapp center`.
- Required tags exist:
  - `g7c-ui-rider-baseline`
  - `dev-mock-login-baseline`
- `rg "db\.AutoMigrate\(" backend/cmd/server` returned zero matches.
- Main worktree was clean before the worktree branch was created.

## File-level findings

### Token TTL

Verified in `backend/internal/module/user/service.go`:

- previous `accessTokenTTL` was fixed at `15 * time.Minute`
- JWT is generated inside `WechatLogin`
- `user.Service` had `wechatAppID`, `wechatSecret`, and `allowMockWechat`, but did not know `APP_ENV`

Patch:

- added `WithAppEnv(appEnv string)` option
- added `localMockTokenTTL = 12 * time.Hour`
- `WechatLogin` now selects TTL via `accessTokenTTL()`
- local mock condition is the same four-condition gate used by the existing local no-op WeChat media safety path
- all non-local-mock paths still use `15 * time.Minute`

### New media review projection

Verified in `backend/internal/module/media/miniappflow/service.go` and media review projection files:

- upload completion calls `EnsureProjection`
- pending projection appends a `ReviewProjectionSubmitted` outbox event
- consumer moves projection from `pending` to `submitted`
- no existing local mock path moved the projection from `submitted` to `approved`
- `MediaGateway` visibility requires `ProjectionApproved` plus active/visible reference and final-ready asset

Patch:

- added `WithLocalMockAutoApproval(enabled bool)` to `miniappflow.Service`
- main composition root enables it only through the same local mock four-condition gate
- local mock auto-approval appends and consumes normal media events:
  - `ReviewProjectionReviewing`
  - `ReviewProjectionDecision` with `decision=approved`
- it does not directly update projection rows
- it uses existing `ReviewProjectionService` and `MediaConsumer` boundaries
- production and credentialed paths do not pass this option and keep existing submitted/wait-for-real-review behavior

## Implementation files

Changed files:

- `backend/cmd/server/main.go`
- `backend/internal/module/user/service.go`
- `backend/internal/module/user/service_test.go`
- `backend/internal/module/media/miniappflow/service.go`
- `backend/internal/module/media/miniappflow/service_test.go`
- `backend/internal/module/media/miniappflow/mysql_external_test.go`
- `backend/internal/module/media/miniappflow/wechat_server_external_test.go`

Explicitly unchanged:

- config validation rules
- real WeChat safety implementation
- mock login exchange behavior
- takeout buyer/merchant/rider business logic
- frontend code
- migrations
- package manifests

## Safety notes

- `APP_ENV=prod` never uses the local mock branch.
- Any non-empty WeChat AppID/Secret keeps the normal 15-minute token and normal media review flow.
- `prod + ALLOW_MOCK_WECHAT_LOGIN=true + empty WeChat credentials` is still rejected by existing `config.Validate()`.
- Local media auto-approval is implemented through existing outbox/consumer/review projection machinery, preserving CAS and idempotency semantics.

