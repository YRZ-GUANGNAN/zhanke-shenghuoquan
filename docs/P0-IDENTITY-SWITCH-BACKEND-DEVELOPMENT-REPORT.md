# P0 身份切换后端 Development Report

## 1. 基线与范围

- Branch: `feat/p0-identity-switch`
- Base commit: `a3d5ee16 feat(ui): 新增学生端我的内容静态页`
- Scope:
  - Domain A `users.current_identity` 当前身份持久化。
  - `/api/v1/auth/identity` GET/POST。
  - 服务端校验 `student/rider/merchant` 可切换资格。
  - 小程序 profile / rider / merchant 三处身份切换从本地假状态接入后端。
- Out of scope:
  - 不修改 `users.role`。
  - 不修改 delivery/community/trade 业务域核心。
  - 不新增订单、支付、IM 消息或社交功能。

## 2. 资格权威来源

- `student`: 默认永远可用。
- `rider`: `takeout_riders.rider_user_id = users.id` 且 `status = 'active'`。
- `merchant`: `takeout_merchants.owner_user_id = users.id` 且 `status = 'active'`。

配送域已有骑手资格 adapter 复用 `takeout.RiderIdentityService.RequireActive`；本次身份切换沿用同一事实源。`delivery/merchant` 当前没有独立商家资格表，商家资格以 takeout 商家 active 状态为准。

## 3. 后端实现

- 新增迁移：
  - `backend/migrations/029_user_current_identity.sql`
  - `backend/migrations/rollback/029_user_current_identity.sql`
- `users` 新增字段：
  - `current_identity VARCHAR(24) NOT NULL DEFAULT 'student'`
  - CHECK: `student/rider/merchant`
- `backend/internal/module/user/identity.go`
  - 新增 `Identity` 枚举。
  - 新增 `IdentityEligibility` 窄接口。
  - 新增 `IdentitySummary` 与切换逻辑。
- `backend/internal/module/user`
  - `User.CurrentIdentity`
  - `Repository.UpdateCurrentIdentity`
  - `Service.SwitchIdentity`
  - `Handler` 注册 `/auth/identity` GET/POST。
- `backend/cmd/server/identity_eligibility.go`
  - 组合根 adapter 调用 takeout 身份服务。
  - `user` 包不 import `takeout`，跨域仍走窄接口。

## 4. 前端实现

- `apps/miniapp/src/types.ts`
  - 新增 `IdentityRoleKey`、`IdentityOption`、`IdentitySummary`。
- `apps/miniapp/src/services/a-user.ts`
  - 新增 `currentIdentity()`。
  - 新增 `switchIdentity(identity)`。
- `apps/miniapp/src/pages/profile/index.tsx`
  - 页面加载时 GET `/auth/identity`。
  - 确认切换时 POST `/auth/identity`。
  - 成功后按后端 `currentIdentity` 跳转。
  - 保留 `my-content` 静态页跳转。
- `apps/miniapp/src/pages/delivery-rider/index.tsx`
  - “切回学生”调用后端切换接口。
- `apps/miniapp/src/pages/merchant-home/index.tsx`
  - “切回学生”调用后端切换接口。

## 5. API 契约

`GET /api/v1/auth/identity`

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "currentIdentity": "student",
    "identities": [
      {"identity": "student", "available": true, "status": "active"},
      {"identity": "rider", "available": false, "reason": "rider qualification required", "status": "missing"},
      {"identity": "merchant", "available": false, "reason": "merchant qualification required", "status": "missing"}
    ]
  }
}
```

`POST /api/v1/auth/identity`

```json
{"identity": "rider"}
```

- 200: 切换成功，返回 `IdentitySummary`。
- 400: identity 非 `student/rider/merchant`。
- 401: 未登录。
- 403: 无资格或非 active。
- 404: 用户不存在。

## 6. 边界结论

- Domain A 只持久化当前身份视图，不写 G 表。
- takeout 资格查询在 `cmd/server` composition root adapter 内完成。
- `user` 包没有引入 `takeout/delivery/community/trade`。
- 未改 delivery/community/trade 核心文件。
