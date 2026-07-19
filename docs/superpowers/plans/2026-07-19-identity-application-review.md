# 身份申请与管理员审核 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立独立的骑手/商家申请、管理员审核、身份授权与工作台后端权限闭环。

**Architecture:** 新增 `identityapplication` 领域，使用当前申请、证明资产、只追加事件和身份授权四类持久化数据。用户接口挂载到现有 `/api/v1/auth`，管理员接口挂载到 `/api/v1/admin/identity-applications`；现有 `/auth/identity` 改为读取授权，骑手批准时在同一数据库事务内写入共享骑手资格。

**Tech Stack:** Go 1.25、Gin、GORM、MySQL 8、React 18、Taro 4、TypeScript、Zustand

---

## 文件结构

- Create: `backend/migrations/034_identity_application_review.sql` — 四张表、精确回填和媒体用途约束。
- Create: `backend/migrations/migration_034_test.go` — 前向迁移、幂等和回填合同测试。
- Create: `backend/internal/module/identityapplication/model.go` — 申请、资产、事件、授权及输入输出类型。
- Create: `backend/internal/module/identityapplication/repository.go` — 查询、提交、审核事务和分页。
- Create: `backend/internal/module/identityapplication/service.go` — 校验、状态机、自审限制和授权判断。
- Create: `backend/internal/module/identityapplication/service_test.go` — 领域状态机单元测试。
- Create: `backend/internal/module/identityapplication/handler.go` — 用户与管理员 HTTP 路由和错误映射。
- Create: `backend/internal/module/identityapplication/handler_test.go` — 严格请求体、401/403/404/409 合同测试。
- Create: `backend/internal/module/identityapplication/media.go` — 证明媒体预留、提交和取消适配器。
- Modify: `backend/internal/module/media/model.go` — 增加私有 `identity_application` 媒体用途。
- Modify: `backend/cmd/server/identity_eligibility.go` — 从授权表判断骑手和商家可用性。
- Modify: `backend/cmd/server/main.go` — 组装服务、媒体适配器和路由。
- Modify: `apps/admin/src/types.ts` — 身份申请 Admin 类型。
- Modify: `apps/admin/src/services/f-admin.ts` — 审核 API。
- Create: `apps/admin/src/IdentityApplications.tsx` — 审核列表、详情、通过与拒绝。
- Modify: `apps/admin/src/App.tsx` — 增加身份审核导航入口。
- Modify: `apps/admin/src/store.ts` — 增加 `identities` section。
- Modify: `apps/miniapp/src/types.ts` — MiniApp 申请类型。
- Modify: `apps/miniapp/src/services/a-user.ts` — 申请查询和提交 API。
- Create: `apps/miniapp/src/pages/identity-application/index.tsx` — 双身份真实申请页。
- Create: `apps/miniapp/src/pages/identity-application/index.scss` — 仅满足现有设计系统的表单样式。
- Create: `apps/miniapp/src/pages/identity-application/index.config.ts` — 页面标题。
- Modify: `apps/miniapp/src/pages/profile/index.tsx` — 未授权身份进入申请页。
- Modify: `apps/miniapp/src/app.config.ts` — 注册申请页。
- Create: `scripts/check-identity-application-contract.cjs` — MiniApp/Admin 静态合同检查。
- Modify: `package.json` — 把合同检查纳入 `verify`。

### Task 1: 前向迁移与精确回填

**Files:**
- Create: `backend/migrations/migration_034_test.go`
- Create: `backend/migrations/034_identity_application_review.sql`

- [ ] **Step 1: 写失败的迁移合同测试**

```go
package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigration034DefinesIdentityApplicationsAndGrants(t *testing.T) {
	sourceBytes, err := os.ReadFile("034_identity_application_review.sql")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)
	for _, token := range []string{
		"CREATE TABLE identity_applications",
		"CREATE TABLE identity_application_assets",
		"CREATE TABLE identity_application_events",
		"CREATE TABLE user_identity_grants",
		"UNIQUE KEY uq_identity_application_user_role",
		"UNIQUE KEY uq_identity_grant_user_role",
		"CHECK (role IN ('rider','merchant'))",
		"CHECK (status IN ('pending','approved','rejected'))",
		"SELECT tr.rider_user_id,'rider'",
		"SELECT DISTINCT o.merchant_user_id,'merchant'",
		"open_id NOT LIKE 'mock_openid\\_%'",
		"'identity_application'",
	} {
		if !strings.Contains(source, token) {
			t.Fatalf("migration 034 missing %q", token)
		}
	}
	if _, err := os.Stat(filepath.Join("rollback", "034_identity_application_review.sql")); !os.IsNotExist(err) {
		t.Fatal("migration 034 must remain forward-only")
	}
}
```

- [ ] **Step 2: 运行测试确认因迁移缺失而失败**

Run: `go test ./migrations -run TestMigration034DefinesIdentityApplicationsAndGrants -count=1`

Expected: FAIL，错误包含 `034_identity_application_review.sql: The system cannot find the file specified`。

- [ ] **Step 3: 编写迁移**

迁移必须创建以下结构，并使用现有 `users`、`media_assets`、`takeout_riders`、`delivery_orders` 的实际列类型：

```sql
CREATE TABLE identity_applications (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  application_public_id CHAR(26) NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  role VARCHAR(16) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  real_name VARCHAR(64) NOT NULL,
  phone VARCHAR(32) NOT NULL,
  merchant_name VARCHAR(120) NOT NULL DEFAULT '',
  merchant_address VARCHAR(240) NOT NULL DEFAULT '',
  review_reason VARCHAR(500) NOT NULL DEFAULT '',
  reviewer_user_id BIGINT UNSIGNED NULL,
  reviewed_at DATETIME(3) NULL,
  submitted_at DATETIME(3) NOT NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_identity_application_public_id (application_public_id),
  UNIQUE KEY uq_identity_application_user_role (user_id, role),
  KEY idx_identity_application_status_submitted (status, submitted_at, id),
  CONSTRAINT chk_identity_application_role CHECK (role IN ('rider','merchant')),
  CONSTRAINT chk_identity_application_status CHECK (status IN ('pending','approved','rejected'))
);

CREATE TABLE identity_application_assets (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  application_id BIGINT UNSIGNED NOT NULL,
  asset_public_id CHAR(26) NOT NULL,
  sort_order SMALLINT UNSIGNED NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_identity_application_asset_slot (application_id, sort_order),
  UNIQUE KEY uq_identity_application_asset (application_id, asset_public_id)
);

CREATE TABLE identity_application_events (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  event_public_id CHAR(26) NOT NULL,
  application_id BIGINT UNSIGNED NOT NULL,
  actor_user_id BIGINT UNSIGNED NOT NULL,
  event_type VARCHAR(16) NOT NULL,
  status VARCHAR(16) NOT NULL,
  reason VARCHAR(500) NOT NULL DEFAULT '',
  snapshot_json JSON NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_identity_application_event_public_id (event_public_id),
  KEY idx_identity_application_event_application (application_id, created_at, id),
  CONSTRAINT chk_identity_application_event_type CHECK (event_type IN ('submitted','resubmitted','approved','rejected'))
);

CREATE TABLE user_identity_grants (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  role VARCHAR(16) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'active',
  source_application_id BIGINT UNSIGNED NULL,
  granted_at DATETIME(3) NOT NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_identity_grant_user_role (user_id, role),
  KEY idx_identity_grant_status (status, role, user_id),
  CONSTRAINT chk_identity_grant_role CHECK (role IN ('rider','merchant')),
  CONSTRAINT chk_identity_grant_status CHECK (status IN ('active','suspended'))
);
```

在同一迁移中使用以下精确回填逻辑，只回填真实、非 mock 用户：

```sql
INSERT INTO user_identity_grants
  (user_id,role,status,source_application_id,granted_at,row_version,created_at,updated_at)
SELECT tr.rider_user_id,'rider','active',NULL,NOW(3),1,NOW(3),NOW(3)
FROM takeout_riders tr
JOIN users u ON u.id=tr.rider_user_id
WHERE tr.status='active' AND u.open_id NOT LIKE 'mock_openid\\_%'
ON DUPLICATE KEY UPDATE user_id=VALUES(user_id);

INSERT INTO user_identity_grants
  (user_id,role,status,source_application_id,granted_at,row_version,created_at,updated_at)
SELECT DISTINCT o.merchant_user_id,'merchant','active',NULL,NOW(3),1,NOW(3),NOW(3)
FROM delivery_orders o
JOIN users u ON u.id=o.merchant_user_id
WHERE u.open_id NOT LIKE 'mock_openid\\_%'
ON DUPLICATE KEY UPDATE user_id=VALUES(user_id);
```

把 `media_assets.purpose` 和 `media_asset_references.reference_type` 扩为 `VARCHAR(24)`，再将 `identity_application` 加入约束；采用迁移 033 的 guard-before-drop 顺序。

- [ ] **Step 4: 运行迁移测试**

Run: `go test ./migrations -run Migration034 -count=1`

Expected: PASS；若 `TEST_MYSQL_DSN` 可创建临时库，再加入真实应用两次仍只有四张新表且回填无重复的 MySQL 用例。

- [ ] **Step 5: 提交迁移**

```powershell
git add -- backend/migrations/034_identity_application_review.sql backend/migrations/migration_034_test.go
git commit -m "功能：增加身份申请审核数据结构"
```

### Task 2: 领域模型、校验和提交状态机

**Files:**
- Create: `backend/internal/module/identityapplication/model.go`
- Create: `backend/internal/module/identityapplication/repository.go`
- Create: `backend/internal/module/identityapplication/service.go`
- Create: `backend/internal/module/identityapplication/service_test.go`

- [ ] **Step 1: 写失败的领域状态机测试**

```go
func TestSubmitRejectAndResubmit(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(repo, nil, fixedClock)
	first, err := service.Submit(context.Background(), 41, RoleRider, SubmitInput{
		RealName: "张同学", Phone: "13800138000", AssetPublicIDs: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"},
	})
	if err != nil || first.Status != StatusPending || first.RowVersion != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	if _, err := service.Submit(context.Background(), 41, RoleRider, SubmitInput{
		RealName: "重复", Phone: "13800138000", ExpectedRowVersion: 1,
	}); !errors.Is(err, ErrApplicationPending) {
		t.Fatalf("pending resubmit err=%v", err)
	}
	repo.reject(first.ID, 7, "资料不清晰")
	second, err := service.Submit(context.Background(), 41, RoleRider, SubmitInput{
		RealName: "张同学", Phone: "13800138000", AssetPublicIDs: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"}, ExpectedRowVersion: 2,
	})
	if err != nil || second.Status != StatusPending || second.ReviewReason != "" || second.RowVersion != 3 {
		t.Fatalf("second=%+v err=%v", second, err)
	}
}
```

再增加商家缺少 `merchantName`/`merchantAddress`、手机号格式错误、无证明图片、重复资产、已批准后重提、版本冲突测试。

- [ ] **Step 2: 运行测试确认缺少领域类型**

Run: `go test ./internal/module/identityapplication -run 'TestSubmit|TestValidation' -count=1`

Expected: FAIL，错误包含 `undefined: NewService`。

- [ ] **Step 3: 实现明确的模型和状态常量**

```go
type Role string
type Status string
type GrantStatus string

const (
	RoleRider Role = "rider"
	RoleMerchant Role = "merchant"
	StatusPending Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
	GrantActive GrantStatus = "active"
	GrantSuspended GrantStatus = "suspended"
)

type SubmitInput struct {
	RealName string
	Phone string
	MerchantName string
	MerchantAddress string
	AssetPublicIDs []string
	ExpectedRowVersion uint64
}

type ApplicationView struct {
	ApplicationPublicID string `json:"applicationId"`
	UserID uint64 `json:"userId"`
	Role Role `json:"role"`
	Status Status `json:"status"`
	RealName string `json:"realName"`
	Phone string `json:"phone"`
	MerchantName string `json:"merchantName"`
	MerchantAddress string `json:"merchantAddress"`
	AssetPublicIDs []string `json:"assetIds"`
	Proofs []ProofAssetView `json:"proofs,omitempty"`
	ReviewReason string `json:"reviewReason"`
	ReviewerUserID *uint64 `json:"reviewerUserId,omitempty"`
	SubmittedAt time.Time `json:"submittedAt"`
	ReviewedAt *time.Time `json:"reviewedAt,omitempty"`
	RowVersion uint64 `json:"rowVersion"`
}

type ProofAssetView struct {
	AssetID string `json:"assetId"`
	URL string `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}
```

实现 `Repository` 的 `FindCurrent`、`ListCurrent`、`SaveSubmission`、`ListAdmin`、`FindDetail`，服务层统一 `strings.TrimSpace`，手机号使用 `^1[3-9][0-9]{9}$`，证明图片数量限定 1–6。

- [ ] **Step 4: 运行领域测试**

Run: `go test ./internal/module/identityapplication -run 'TestSubmit|TestValidation' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交领域状态机**

```powershell
git add -- backend/internal/module/identityapplication/model.go backend/internal/module/identityapplication/repository.go backend/internal/module/identityapplication/service.go backend/internal/module/identityapplication/service_test.go
git commit -m "功能：实现身份申请状态机"
```

### Task 3: 私有证明媒体绑定

**Files:**
- Modify: `backend/internal/module/media/model.go`
- Create: `backend/internal/module/media/purpose_identity_application_test.go`
- Create: `backend/internal/module/identityapplication/media.go`
- Modify: `apps/miniapp/src/services/media.ts`

- [ ] **Step 1: 写失败的媒体隐私测试**

```go
func TestIdentityApplicationPurposeIsPrivate(t *testing.T) {
	if !PurposeIdentityApplication.Valid() {
		t.Fatal("identity application purpose must be valid")
	}
	if PurposeIdentityApplication.PubliclyResolvable() {
		t.Fatal("identity application proof must stay private")
	}
}
```

- [ ] **Step 2: 运行测试确认常量缺失**

Run: `go test ./internal/module/media -run TestIdentityApplicationPurposeIsPrivate -count=1`

Expected: FAIL，错误包含 `undefined: PurposeIdentityApplication`。

- [ ] **Step 3: 增加媒体用途与绑定适配器**

在 `media/model.go` 增加：

```go
const PurposeIdentityApplication Purpose = "identity_application"
```

把它加入 `Purpose.Valid()`，但不要加入 `PubliclyResolvable()`。同时把 `Asset.Purpose` 和 `AssetReference.ReferenceType` 的 GORM `size` 从 16 改为 24，与迁移后的实际列宽一致。在 MiniApp `MediaPurpose`、`MediaStatus.purpose`、`MAX_EDGE` 和 `domainPurpose()` 加入同名值。

`identityapplication/media.go` 实现：

```go
type MediaReservation struct {
	Token string
	AssetPublicIDs []string
}

type ApplicationMedia interface {
	Reserve(context.Context, uint64, []string) (MediaReservation, error)
	Commit(context.Context, uint64, MediaReservation, string) error
	Cancel(context.Context, uint64, MediaReservation) error
	ResolveAdminProofs(context.Context, []string) ([]ProofAssetView, error)
}
```

适配现有 `ReferenceBindingService.Reserve/Commit/Cancel`，`OwnerID` 必须等于申请用户，`ReferenceType` 和 `Purpose` 都使用 `identity_application`。`ResolveAdminProofs` 只能从已绑定到当前申请的 asset ID 调用；逐个通过 `media.Repository.FindAsset` 校验 purpose、最终对象 key/version 和审核通过状态，再通过 `media.DisplayURLSigner.SignedGetURL` 生成 5 分钟 URL。该方法只在 Admin 鉴权完成后的详情接口调用，不增加公开解析路由。

- [ ] **Step 4: 运行媒体与 MiniApp 类型检查**

Run: `go test ./internal/module/media ./internal/module/identityapplication -count=1`

Expected: PASS。

Run: `npm.cmd --prefix apps/miniapp run typecheck`

Expected: PASS。

- [ ] **Step 5: 提交媒体支持**

```powershell
git add -- backend/internal/module/media/model.go backend/internal/module/media/purpose_identity_application_test.go backend/internal/module/identityapplication/media.go apps/miniapp/src/services/media.ts
git commit -m "功能：支持身份申请证明媒体"
```

### Task 4: 用户申请 HTTP 合同

**Files:**
- Create: `backend/internal/module/identityapplication/handler.go`
- Create: `backend/internal/module/identityapplication/handler_test.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: 写失败的用户路由测试**

测试以下精确行为：未登录三个路由均为 401；首次 `PUT /api/v1/auth/identity-applications/rider` 返回 200 和 `pending`；待审核重复提交为 409；未知字段、`null`、数组、两个 JSON 对象和超过 32 KiB 请求体为 400；他人媒体为 409。

```go
routes := []struct{ method, path string }{
	{http.MethodGet, "/api/v1/auth/identity-applications"},
	{http.MethodGet, "/api/v1/auth/identity-applications/rider"},
	{http.MethodPut, "/api/v1/auth/identity-applications/rider"},
}
```

- [ ] **Step 2: 运行测试确认路由不存在**

Run: `go test ./internal/module/identityapplication -run TestUserIdentityApplicationHTTP -count=1`

Expected: FAIL，首个认证请求为 404 而不是 401。

- [ ] **Step 3: 实现严格解码和用户路由**

```go
type submitRequest struct {
	RealName string `json:"realName"`
	Phone string `json:"phone"`
	MerchantName string `json:"merchantName"`
	MerchantAddress string `json:"merchantAddress"`
	AssetIDs []string `json:"assetIds"`
	ExpectedRowVersion uint64 `json:"expectedRowVersion"`
}

func (h *Handler) RegisterUser(rg *gin.RouterGroup) {
	rg.GET("/identity-applications", h.listMine)
	rg.GET("/identity-applications/:role", h.getMine)
	rg.PUT("/identity-applications/:role", h.submit)
}
```

使用 `http.MaxBytesReader` 32 KiB、`json.Decoder.DisallowUnknownFields()`，解码一次对象后必须读到 EOF。映射：校验 400、未认证 401、媒体/状态/版本冲突 409、不存在 404、未知错误 500。

- [ ] **Step 4: 运行用户 HTTP 测试**

Run: `go test ./internal/module/identityapplication -run TestUserIdentityApplicationHTTP -count=1`

Expected: PASS。

- [ ] **Step 5: 提交用户接口**

```powershell
git add -- backend/internal/module/identityapplication/handler.go backend/internal/module/identityapplication/handler_test.go backend/cmd/server/main.go
git commit -m "功能：开放身份申请用户接口"
```

### Task 5: 原子审核、授权和自审禁止

**Files:**
- Modify: `backend/internal/module/identityapplication/repository.go`
- Modify: `backend/internal/module/identityapplication/service.go`
- Modify: `backend/internal/module/identityapplication/service_test.go`
- Modify: `backend/internal/module/identityapplication/handler.go`
- Modify: `backend/internal/module/identityapplication/handler_test.go`
- Modify: `backend/internal/module/takeout/rider_identity.go`

- [ ] **Step 1: 写失败的审核事务测试**

```go
func TestApproveIsAtomicAndForbidsSelfReview(t *testing.T) {
	repo := newMemoryRepositoryWithPending(41, RoleRider)
	service := NewService(repo, nil, fixedClock)
	app := repo.current(41, RoleRider)
	if _, err := service.Approve(context.Background(), 41, app.ApplicationPublicID, app.RowVersion); !errors.Is(err, ErrSelfReview) {
		t.Fatalf("self review err=%v", err)
	}
	repo.failRiderProvision = true
	if _, err := service.Approve(context.Background(), 7, app.ApplicationPublicID, app.RowVersion); err == nil {
		t.Fatal("provision failure was accepted")
	}
	if repo.current(41, RoleRider).Status != StatusPending || repo.hasGrant(41, RoleRider) {
		t.Fatal("failed approval left partial state")
	}
}
```

增加拒绝原因必填、重复审核 409、版本冲突 409、批准商家写 grant、批准骑手同时写 active rider 资格和 grant 的测试。

- [ ] **Step 2: 运行测试确认审核方法缺失**

Run: `go test ./internal/module/identityapplication -run 'TestApprove|TestReject' -count=1`

Expected: FAIL，错误包含 `service.Approve undefined`。

- [ ] **Step 3: 实现审核事务接口**

```go
type DecisionInput struct {
	ReviewerUserID uint64
	ApplicationPublicID string
	ExpectedRowVersion uint64
	Reason string
}

type DecisionRepository interface {
	Approve(context.Context, DecisionInput) (ApplicationView, error)
	Reject(context.Context, DecisionInput) (ApplicationView, error)
}
```

GORM `Approve` 必须在一个 `db.Transaction` 中锁定申请，比较 `pending + row_version`，禁止 `user_id == reviewer_user_id`，写 `user_identity_grants`。骑手分支执行以下完整调用写共享骑手资格，最后更新申请和追加事件：

```go
riderService, err := takeout.NewRiderIdentityService(takeout.NewRepository(tx), s.now)
if err != nil {
	return err
}
if _, err := riderService.RegisterApproved(ctx, application.UserID, application.RealName, application.Phone); err != nil {
	return err
}
```

`Reject` 在事务内更新原因、审核人、审核时间和事件。

- [ ] **Step 4: 增加并验证管理员路由**

```go
func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.GET("", h.listAdmin)
	rg.GET("/:applicationId", h.getAdmin)
	rg.POST("/:applicationId/approve", h.approve)
	rg.POST("/:applicationId/reject", h.reject)
}
```

Run: `go test ./internal/module/identityapplication -count=1`

Expected: PASS，覆盖 401、403、管理员成功、自审 403、冲突 409。

- [ ] **Step 5: 提交审核闭环**

```powershell
git add -- backend/internal/module/identityapplication backend/internal/module/takeout/rider_identity.go
git commit -m "功能：完成身份审核与授权事务"
```

### Task 6: 身份可用性与服务器组装

**Files:**
- Modify: `backend/cmd/server/identity_eligibility.go`
- Modify: `backend/cmd/server/identity_eligibility_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/identity_http_mysql_test.go`

- [ ] **Step 1: 改写失败测试，要求 grant 而非默认商家权限**

```go
func TestMerchantRequiresActiveGrant(t *testing.T) {
	eligibility := newDeliveryIdentityEligibility(true, riderFinderStub{}, grantFinderStub{})
	missing, err := eligibility.MerchantEligible(context.Background(), 88)
	if err != nil || missing.Available || missing.Status != "missing" {
		t.Fatalf("missing=%+v err=%v", missing, err)
	}
	eligibility = newDeliveryIdentityEligibility(true, riderFinderStub{}, grantFinderStub{
		grants: map[string]identityapplication.GrantStatus{"88:merchant": identityapplication.GrantActive},
	})
	active, err := eligibility.MerchantEligible(context.Background(), 88)
	if err != nil || !active.Available || active.Status != "active" {
		t.Fatalf("active=%+v err=%v", active, err)
	}
}
```

- [ ] **Step 2: 运行测试确认当前商家仍默认开放**

Run: `go test ./cmd/server -run 'TestMerchantRequiresActiveGrant|TestRiderRequires' -count=1`

Expected: FAIL，当前实现对任意非零用户返回商家 `active`。

- [ ] **Step 3: 注入授权查询并组装路由**

`deliveryIdentityEligibility` 同时持有 `grantFinder`。商家要求 active grant；骑手要求 active grant 且共享骑手状态 active。`main.go` 创建一个 identity application service，并注册：

```go
identityHandler.RegisterUser(api.Group("/auth", userService.AuthRequired(), user.UnmutedRequired()))
identityHandler.RegisterAdmin(api.Group("/admin/identity-applications", userService.AuthRequired(), user.AdminRequired()))
```

确保 `/auth/identity` 的已有 handler 仍只注册一次。

- [ ] **Step 4: 运行身份与配送回归测试**

Run: `go test ./cmd/server ./internal/module/user ./internal/module/delivery/... -count=1`

Expected: PASS；未授权商家切换返回 403，批准后可以切换。

- [ ] **Step 5: 提交授权接线**

```powershell
git add -- backend/cmd/server/identity_eligibility.go backend/cmd/server/identity_eligibility_test.go backend/cmd/server/main.go backend/cmd/server/identity_http_mysql_test.go
git commit -m "功能：以审核授权控制身份切换"
```

### Task 7: Admin 身份审核界面

**Files:**
- Modify: `apps/admin/src/types.ts`
- Modify: `apps/admin/src/services/f-admin.ts`
- Modify: `apps/admin/src/store.ts`
- Create: `apps/admin/src/IdentityApplications.tsx`
- Modify: `apps/admin/src/App.tsx`

- [ ] **Step 1: 定义精确类型和 API**

```ts
export type AdminSection='dashboard'|'users'|'identities'|'moderation'|'media'|'delivery'|'takeout'|'community'|'trade'|'reports'|'navigation'
export type IdentityApplicationRole='rider'|'merchant'
export type IdentityApplicationStatus='pending'|'approved'|'rejected'
export interface IdentityApplicationEvent{eventId:string;eventType:'submitted'|'resubmitted'|'approved'|'rejected';actorUserId:number;status:IdentityApplicationStatus;reason:string;createdAt:string}
export interface IdentityApplicationProof{assetId:string;url:string;expiresAt:string}
export interface IdentityApplicationRow{applicationId:string;userId:number;role:IdentityApplicationRole;status:IdentityApplicationStatus;realName:string;phone:string;merchantName:string;merchantAddress:string;assetIds:string[];proofs?:IdentityApplicationProof[];reviewReason:string;reviewerUserId?:number;submittedAt:string;reviewedAt?:string;rowVersion:number;events?:IdentityApplicationEvent[]}
export interface IdentityApplicationQuery{page?:number;pageSize?:number;role?:IdentityApplicationRole;status?:IdentityApplicationStatus}
```

在 `adminAPI` 增加 list/detail/approve/reject，所有路径使用 `/admin/identity-applications`，审核 body 总是发送 `expectedRowVersion`，拒绝同时发送 `reason`。

- [ ] **Step 2: 运行类型检查确认 section 和组件尚未接线**

Run: `npm.cmd --prefix apps/admin run typecheck`

Expected: 在加入 `identities` 导航但未扩展 `AdminSection` 时 FAIL，错误指向 `AdminSection`。

- [ ] **Step 3: 实现审核组件**

`IdentityApplications.tsx` 必须提供角色/状态筛选、列表选择、详情、证明图片、事件、通过确认、拒绝原因输入。证明图片只使用 Admin 详情响应的短期 `proofs[].url`，不得调用公开媒体解析接口。按钮只在 `pending` 显示；409 时显示“该申请已被其他管理员处理”并刷新详情。

```tsx
const decide=async(action:'approve'|'reject')=>{
  if(!selected)return
  if(action==='reject'&&!reason.trim()){setError('拒绝原因不能为空');return}
  setBusy(true)
  try{
    const next=action==='approve'
      ?await adminAPI.approveIdentityApplication(selected.applicationId,selected.rowVersion)
      :await adminAPI.rejectIdentityApplication(selected.applicationId,selected.rowVersion,reason.trim())
    setSelected(next);setReason('');await load()
  }catch(error){
    if(error instanceof AdminAPIError&&error.status===409){setError('该申请已被其他管理员处理');await load()}
    else setError(error instanceof Error?error.message:'审核失败')
  }finally{setBusy(false)}
}
```

- [ ] **Step 4: 运行 Admin 验证**

Run: `npm.cmd --prefix apps/admin run typecheck`

Expected: PASS。

Run: `npm.cmd --prefix apps/admin run build`

Expected: PASS，Vite 产出 `apps/admin/dist`。

- [ ] **Step 5: 提交 Admin 界面**

```powershell
git add -- apps/admin/src/types.ts apps/admin/src/services/f-admin.ts apps/admin/src/store.ts apps/admin/src/IdentityApplications.tsx apps/admin/src/App.tsx
git commit -m "功能：增加身份申请审核后台"
```

### Task 8: MiniApp 身份申请页与“我的”入口

**Files:**
- Modify: `apps/miniapp/src/types.ts`
- Modify: `apps/miniapp/src/services/a-user.ts`
- Create: `apps/miniapp/src/pages/identity-application/index.tsx`
- Create: `apps/miniapp/src/pages/identity-application/index.scss`
- Create: `apps/miniapp/src/pages/identity-application/index.config.ts`
- Modify: `apps/miniapp/src/pages/profile/index.tsx`
- Modify: `apps/miniapp/src/app.config.ts`
- Create: `scripts/check-identity-application-contract.cjs`
- Modify: `package.json`

- [ ] **Step 1: 写失败的静态业务合同**

```js
const fs=require('node:fs')
const assert=require('node:assert/strict')
const read=(path)=>fs.readFileSync(path,'utf8')
const app=read('apps/miniapp/src/app.config.ts')
const profile=read('apps/miniapp/src/pages/profile/index.tsx')
const page=read('apps/miniapp/src/pages/identity-application/index.tsx')
assert.match(app,/pages\/identity-application\/index/)
assert.match(profile,/pages\/identity-application\/index\?role=/)
assert.match(page,/chooseAndUploadImages\('identity_application'/)
assert.match(page,/pending/)
assert.match(page,/rejected/)
console.log('identity application contract: ok')
```

- [ ] **Step 2: 运行合同确认页面缺失**

Run: `node scripts/check-identity-application-contract.cjs`

Expected: FAIL，错误包含 `ENOENT` 和 `pages/identity-application/index.tsx`。

- [ ] **Step 3: 增加客户端类型、服务和页面**

先在 `apps/miniapp/src/types.ts` 增加完整客户端类型：

```ts
export type IdentityApplicationRole='rider'|'merchant'
export type IdentityApplicationStatus='pending'|'approved'|'rejected'
export interface IdentityApplicationView{
  applicationId:string
  userId:number
  role:IdentityApplicationRole
  status:IdentityApplicationStatus
  realName:string
  phone:string
  merchantName:string
  merchantAddress:string
  assetIds:string[]
  reviewReason:string
  submittedAt:string
  reviewedAt?:string
  rowVersion:number
}
export interface IdentityApplicationSubmitInput{
  realName:string
  phone:string
  merchantName:string
  merchantAddress:string
  assetIds:string[]
  expectedRowVersion:number
}
```

服务接口：

```ts
export function getIdentityApplication(role:'rider'|'merchant'):Promise<IdentityApplicationView>{return request(`/auth/identity-applications/${role}`)}
export function submitIdentityApplication(role:'rider'|'merchant',input:IdentityApplicationSubmitInput):Promise<IdentityApplicationView>{return request(`/auth/identity-applications/${role}`,{url:'',method:'PUT',data:input,timeout:20000})}
```

页面从 `Taro.getCurrentInstance().router.params.role` 读取角色，仅接受 `rider`/`merchant`；加载现状；`pending` 只读显示；`rejected` 回填表单并显示原因；提交前等待所有证明图片 `waitForMediaApproval()` 成功。商家显示名称和地址字段，骑手不显示。

Profile 未授权点击逻辑改为：

```ts
if (!availability?.available) {
  if (role.key === 'rider' || role.key === 'merchant') {
    void Taro.navigateTo({ url: `/pages/identity-application/index?role=${role.key}` })
    return
  }
  showPlaceholder(unavailableMessage(availability))
  return
}
```

- [ ] **Step 4: 运行 MiniApp 合同、类型检查和构建**

Run: `node scripts/check-identity-application-contract.cjs`

Expected: `identity application contract: ok`。

Run: `npm.cmd --prefix apps/miniapp run typecheck`

Expected: PASS。

Run: `npm.cmd --prefix apps/miniapp run build:weapp`

Expected: PASS。

- [ ] **Step 5: 提交 MiniApp 申请闭环**

```powershell
git add -- apps/miniapp/src/types.ts apps/miniapp/src/services/a-user.ts apps/miniapp/src/pages/identity-application apps/miniapp/src/pages/profile/index.tsx apps/miniapp/src/app.config.ts scripts/check-identity-application-contract.cjs package.json
git commit -m "功能：增加骑手商家身份申请入口"
```

### Task 9: 全链路验证与交付边界

**Files:**
- Create: `docs/IDENTITY-APPLICATION-ACCEPTANCE-REPORT.md`

- [ ] **Step 1: 运行完整自动验证**

Run: `go test ./... -count=1`

Expected: PASS；若临时 MySQL 权限不可用，只允许 migration 034 的真实临时库用例显示明确 skip，静态合同必须 PASS。

Run: `npm.cmd run verify`

Expected: PASS。

- [ ] **Step 2: 本地 HTTP 冒烟**

依次验证：普通用户提交骑手申请；普通用户审核返回 403；管理员拒绝且原因回传；用户重新提交；管理员通过；`GET /auth/identity` 的骑手 `available=true`；未审核商家保持 `available=false`。

- [ ] **Step 3: 写验收报告**

报告必须逐项记录命令、PASS/FAIL/PENDING，并把“真实证明图片上传”“Admin 浏览器人工操作”“微信体验版身份切换”在未执行时保持 `PENDING`，不得用类型检查替代。

- [ ] **Step 4: 提交验收报告**

```powershell
git add -- docs/IDENTITY-APPLICATION-ACCEPTANCE-REPORT.md
git commit -m "文档：记录身份申请审核验收结果"
```
