# 外部订单导入与自动发单 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将三平台已验签回调可靠回查为权威订单快照，按门店策略完成待确认导入、自动导入与受真实资金约束的自动发单，并提供商家和后台可审计的处置入口。

**Architecture:** Provider 专用 HTTP handler 只验证原始回调并持久保存最小 `CallbackHint`，随后立即返回 Provider 指定的 `CallbackAck`。独立查询 worker 使用加密 Token 调用 `Provider.FetchOrder`，再通过 `ExternalIntake.ReceiveVerified` 写入不可变快照并驱动版本化 workflow；导入、支付、开放骑手池和通知均在回调事务之外完成。自动发单只依赖 `VerifiedFundsPort` 的真实资金结果，端口未就绪、余额来源未核验或支付结果未知时一律 fail closed。

**Tech Stack:** Go 1.25、Gin、GORM、MySQL 8.4、现有 `delivery`/ `deliveryplatform`/ `deliverynotify` 模块、React/Taro TypeScript、Vite Admin。

---

## 依赖、边界与文件结构

冻结依据：

- `docs/superpowers/specs/2026-10-24-external-platform-connection-design.md`
- `docs/superpowers/specs/2026-10-24-delivery-payment-integrity-design.md`
- `docs/superpowers/specs/2026-10-24-delivery-notification-design.md`
- `docs/superpowers/plans/2026-10-24-delivery-platform-foundation-implementation.md`
- 三份 Provider 实施计划。

迁移编号固定为：

- `048`：平台授权、绑定、Token 和 callback 提示；
- `049`：淘宝增量补拉 checkpoint；
- `050`：权威订单快照与导入/自动发单 workflow。

本计划创建：

- `backend/migrations/050_external_order_workflows.sql`
- `backend/migrations/migration_050_test.go`
- `backend/internal/module/delivery/external_verified.go`
- `backend/internal/module/delivery/external_workflow_repository.go`
- `backend/internal/module/delivery/external_workflow_test.go`
- `backend/internal/module/delivery/zz_external_workflow_mysql_test.go`
- `backend/internal/module/deliveryplatform/callback_repository.go`
- `backend/internal/module/deliveryplatform/callback_worker.go`
- `backend/internal/module/deliveryplatform/callback_worker_test.go`
- `backend/internal/module/admin/deliveryintake/provider_handler.go`
- `backend/internal/module/admin/deliveryintake/provider_handler_test.go`
- `backend/internal/module/admin/deliveryplatform/workflow_handler.go`
- `backend/internal/module/admin/deliveryplatform/workflow_handler_test.go`
- `apps/miniapp/src/pages/merchant-platform-orders/index.config.ts`
- `apps/miniapp/src/pages/merchant-platform-orders/index.tsx`
- `apps/miniapp/src/pages/merchant-platform-orders/index.scss`
- `apps/admin/src/ExternalOrderConsole.tsx`
- `scripts/check-external-order-workflow.cjs`

本计划修改：

- `backend/internal/module/delivery/external_intake.go`
- `backend/internal/module/delivery/external_intake_repository.go`
- `backend/internal/module/delivery/dispatch_types.go`
- `backend/internal/module/delivery/dispatch_repository.go`
- `backend/internal/module/delivery/funds_service.go`
- `backend/internal/module/deliveryplatform/types.go`
- `backend/internal/module/deliveryplatform/repository.go`
- `backend/internal/module/admin/deliverynotify/consumer.go`
- `backend/internal/module/admin/deliverynotify/consumer_test.go`
- `backend/cmd/server/main.go`
- `backend/cmd/server/delivery_capabilities_test.go`
- `apps/miniapp/src/app.config.ts`
- `apps/miniapp/src/services/delivery-platform.ts`
- `apps/miniapp/src/pages/delivery-profile/index.tsx`
- `apps/miniapp/src/pages/delivery-profile/index.scss`
- `apps/admin/src/services/delivery-admin.ts`
- `apps/admin/src/DeliveryConsole.tsx`
- `apps/admin/src/App.tsx`
- `package.json`

明确不做：

- 不读取第三方 App 通知、Cookie、商家密码或客户端本地数据库。
- 不代收三平台餐费，不向三平台回写接单/配送状态。
- 不把 `external_takeout_merchants` 当成门店授权。
- 不在 callback 请求中调用官方查询、扣费或建配送单。
- 不把 mock 钱包、mock 微信支付或未知历史余额接到 `VerifiedFundsPort`。
- 不在本计划中迁移生产、启用生产开关、上传微信或 push。

### Task 1: 建立 migration 050 的权威快照和 workflow 合同

**Files:**
- Create: `backend/migrations/migration_050_test.go`
- Create: `backend/migrations/050_external_order_workflows.sql`

- [ ] **Step 1: 先写 DDL 合同失败测试**

```go
package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestMigration050Contract(t *testing.T) {
	raw, err := os.ReadFile("050_external_order_workflows.sql")
	if err != nil { t.Fatal(err) }
	source := string(raw)
	for _, token := range []string{
		"delivery_external_order_snapshots",
		"delivery_external_order_workflows",
		"source_version VARCHAR(128)",
		"policy_version BIGINT UNSIGNED",
		"normalized_payload JSON",
		"tombstoned_at DATETIME(6)",
		"lease_token CHAR(26)",
		"UNIQUE KEY uq_delivery_external_snapshot_version",
		"UNIQUE KEY uq_delivery_external_workflow_order",
		"CHECK (platform IN ('taobao','jd','meituan'))",
	} {
		if !strings.Contains(source, token) { t.Fatalf("migration 050 missing %q", token) }
	}
	for _, forbidden := range []string{"app_secret", "access_token", "refresh_token", "raw_payload", "FOREIGN KEY", "DROP TABLE"} {
		if strings.Contains(strings.ToLower(source), strings.ToLower(forbidden)) {
			t.Fatalf("migration 050 contains forbidden %q", forbidden)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认文件尚不存在**

Run: `cd backend; go test ./migrations -run TestMigration050Contract -count=1`

Expected: FAIL，错误为找不到 `050_external_order_workflows.sql`。

- [ ] **Step 3: 添加完整前向迁移**

```sql
CREATE TABLE IF NOT EXISTS delivery_external_order_snapshots (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  binding_id BIGINT UNSIGNED NOT NULL,
  binding_key CHAR(43) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  merchant_user_id BIGINT UNSIGNED NOT NULL,
  platform VARCHAR(12) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  provider_event_id VARCHAR(128) COLLATE utf8mb4_0900_bin NOT NULL,
  source_shop_id VARCHAR(128) COLLATE utf8mb4_0900_bin NOT NULL,
  source_order_id VARCHAR(128) COLLATE utf8mb4_0900_bin NOT NULL,
  source_version VARCHAR(128) COLLATE utf8mb4_0900_bin NOT NULL,
  source_state VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  event_type VARCHAR(12) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_updated_at DATETIME(6) NOT NULL,
  queried_at DATETIME(6) NOT NULL,
  policy_version BIGINT UNSIGNED NOT NULL,
  auto_import BOOLEAN NOT NULL,
  auto_dispatch BOOLEAN NOT NULL,
  normalized_payload JSON NOT NULL,
  fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_delivery_external_snapshot_version
    (platform, binding_key, source_order_id, source_version),
  UNIQUE KEY uq_delivery_external_snapshot_event
    (platform, binding_key, provider_event_id),
  KEY idx_delivery_external_snapshot_order
    (merchant_user_id, platform, source_shop_id, source_order_id, id),
  CONSTRAINT chk_delivery_external_snapshot_platform
    CHECK (platform IN ('taobao','jd','meituan')),
  CONSTRAINT chk_delivery_external_snapshot_event
    CHECK (event_type IN ('created','updated','cancelled')),
  CONSTRAINT chk_delivery_external_snapshot_policy
    CHECK (auto_dispatch = FALSE OR auto_import = TRUE)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE IF NOT EXISTS delivery_external_order_workflows (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  binding_id BIGINT UNSIGNED NOT NULL,
  binding_key CHAR(43) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  merchant_user_id BIGINT UNSIGNED NOT NULL,
  platform VARCHAR(12) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_shop_id VARCHAR(128) COLLATE utf8mb4_0900_bin NOT NULL,
  source_order_id VARCHAR(128) COLLATE utf8mb4_0900_bin NOT NULL,
  current_snapshot_id BIGINT UNSIGNED NOT NULL,
  current_source_version VARCHAR(128) COLLATE utf8mb4_0900_bin NOT NULL,
  current_source_updated_at DATETIME(6) NOT NULL,
  current_fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  policy_version BIGINT UNSIGNED NOT NULL,
  auto_import BOOLEAN NOT NULL,
  auto_dispatch BOOLEAN NOT NULL,
  status VARCHAR(40) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  reason VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  order_public_id CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  tombstoned_at DATETIME(6) NULL,
  attempts INT UNSIGNED NOT NULL DEFAULT 0,
  next_attempt_at DATETIME(6) NULL,
  lease_token CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  lease_expires_at DATETIME(6) NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_delivery_external_workflow_order
    (platform, binding_key, source_order_id),
  KEY idx_delivery_external_workflow_claim
    (status, next_attempt_at, lease_expires_at, id),
  KEY idx_delivery_external_workflow_merchant
    (merchant_user_id, status, id),
  CONSTRAINT chk_delivery_external_workflow_platform
    CHECK (platform IN ('taobao','jd','meituan')),
  CONSTRAINT chk_delivery_external_workflow_status CHECK (status IN (
    'awaiting_confirmation','pending','processing','retry_wait',
    'awaiting_verified_funds','imported','dispatched','cancelled',
    'needs_review','deadletter'
  )),
  CONSTRAINT chk_delivery_external_workflow_policy
    CHECK (auto_dispatch = FALSE OR auto_import = TRUE)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;
```

该迁移不改写 `delivery_external_inbox` 历史行；旧 inbox 在 migration 050 后仍可查询。snapshot 只保存规范化履约字段，不保存商品明细、餐费、原始 callback 或签名。

- [ ] **Step 4: 加入独立 MySQL apply/apply 合同测试**

`migration_050_test.go` 使用 `DELIVERY_PLATFORM_TEST_MYSQL_DSN`，并先断言库名包含 `test`。在同一空库连续执行 migration 045、046、048、050 两次，然后检查两张新表、唯一索引和 CHECK；未提供变量时 `t.Skip`，不得读取应用默认 DSN。

Run: `cd backend; $env:DELIVERY_PLATFORM_TEST_MYSQL_DSN='<isolated-test-dsn>'; go test ./migrations -run TestMigration050 -count=1`

Expected: PASS；第二次执行不新增重复对象。

- [ ] **Step 5: 提交迁移**

```powershell
git add backend/migrations/050_external_order_workflows.sql backend/migrations/migration_050_test.go
git commit -m "数据库：新增外部订单快照与导入流程"
```

### Task 2: 定义已回查订单边界并替换不安全的直接 Receive

**Files:**
- Create: `backend/internal/module/delivery/external_verified.go`
- Modify: `backend/internal/module/delivery/external_intake.go`
- Test: `backend/internal/module/delivery/external_workflow_test.go`

- [ ] **Step 1: 写已验证边界失败测试**

```go
func TestReceiveVerifiedRejectsUnverifiedAndPolicyMismatch(t *testing.T) {
	intake := newWorkflowIntake(t)
	base := verifiedOrderFixture()
	base.VerifiedAt = time.Time{}
	if _, err := intake.ReceiveVerified(context.Background(), base); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("zero verification accepted: %v", err)
	}
	base = verifiedOrderFixture()
	base.AutoImport = false
	base.AutoDispatch = true
	if _, err := intake.ReceiveVerified(context.Background(), base); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("invalid policy accepted: %v", err)
	}
}

func TestReceiveVerifiedDoesNotExposeLegacyReceive(t *testing.T) {
	typ := reflect.TypeOf((*ExternalIntake)(nil))
	if _, ok := typ.MethodByName("Receive"); ok {
		t.Fatal("raw callback Receive remains public")
	}
}
```

- [ ] **Step 2: 运行测试确认新方法缺失**

Run: `cd backend; go test ./internal/module/delivery -run TestReceiveVerified -count=1`

Expected: FAIL，`VerifiedExternalOrder` 或 `ReceiveVerified` 未定义。

- [ ] **Step 3: 添加无平台依赖的可信输入类型**

```go
type VerifiedExternalOrder struct {
	BindingID      uint64
	BindingKey     string
	MerchantUserID uint64
	Platform       SourcePlatform
	SourceShopID   string
	ProviderEventID string
	SourceVersion  string
	SourceState    string
	SourceUpdatedAt time.Time
	QueriedAt      time.Time
	VerifiedAt     time.Time
	PolicyVersion  uint64
	AutoImport     bool
	AutoDispatch   bool
	Event          ExternalOrderEvent
}

type ExternalWorkflowReceipt struct {
	SnapshotID   uint64
	WorkflowID   uint64
	Status       string
	Reason       string
	OrderPublicID string
	Duplicate    bool
}
```

`VerifiedExternalOrder` 位于 `delivery`，不导入 `deliveryplatform`，避免领域依赖环。校验固定要求：绑定/商家/策略版本非零；平台不是 `local`；`BindingKey`、门店、事件、版本和状态非空；`VerifiedAt` 与 `QueriedAt` 为 UTC 且不晚于当前时间五分钟；`autoDispatch` 不能绕过 `autoImport`；事件必须再次经过 `NormalizeExternalOrderEvent`。

- [ ] **Step 4: 删除原始回调入口并实现 `ReceiveVerified`**

```go
func (s *ExternalIntake) ReceiveVerified(ctx context.Context, value VerifiedExternalOrder) (ExternalWorkflowReceipt, error) {
	value, err := normalizeVerifiedExternalOrder(value, s.now().UTC())
	if err != nil { return ExternalWorkflowReceipt{}, err }
	payload, err := json.Marshal(value.Event)
	if err != nil { return ExternalWorkflowReceipt{}, err }
	digest := sha256.Sum256(payload)
	return s.receiveVerified(ctx, value, payload, hex.EncodeToString(digest[:]))
}
```

删除 `ExternalIntake.Receive` 及 `adapters map[SourcePlatform]ExternalOrderAdapter`。将构造函数改为：

```go
func NewExternalIntake(
	db *gorm.DB,
	fees FeePolicyProvider,
	funds VerifiedFundsPort,
	cfg ExternalIntakeConfig,
) (*ExternalIntake, error)
```

`db` 和 `fees` 必须存在；`funds` 可为 nil，表示自动发单永远停在 `awaiting_verified_funds`。不能用 nil 自动替换为 mock。

- [ ] **Step 5: 运行并提交可信边界**

Run: `cd backend; go test ./internal/module/delivery -run 'TestReceiveVerified|TestNormalizeVerified' -count=1`

Expected: PASS。

```powershell
git add backend/internal/module/delivery/external_verified.go backend/internal/module/delivery/external_intake.go backend/internal/module/delivery/external_workflow_test.go
git commit -m "重构：仅接收已回查的外部订单"
```

### Task 3: 实现版本收敛、取消 tombstone 和 workflow 租约

**Files:**
- Create: `backend/internal/module/delivery/external_workflow_repository.go`
- Modify: `backend/internal/module/delivery/external_intake_repository.go`
- Test: `backend/internal/module/delivery/external_workflow_test.go`
- Test: `backend/internal/module/delivery/zz_external_workflow_mysql_test.go`

- [ ] **Step 1: 写乱序矩阵失败测试**

```go
func TestExternalWorkflowConvergesVersionsAndCancellation(t *testing.T) {
	cases := []struct {
		name string
		first, second VerifiedExternalOrder
		wantStatus, wantReason string
	}{
		{"newer update wins", fixtureVersion("10", ExternalOrderCreated), fixtureVersion("11", ExternalOrderUpdated), "awaiting_confirmation", ""},
		{"older create is stale", fixtureVersion("11", ExternalOrderUpdated), fixtureVersion("10", ExternalOrderCreated), "awaiting_confirmation", "stale_snapshot"},
		{"cancel first creates tombstone", fixtureVersion("20", ExternalOrderCancelled), fixtureVersion("19", ExternalOrderCreated), "cancelled", "stale_after_cancel"},
		{"same version different digest quarantined", fixtureVersion("30", ExternalOrderCreated), changedFixtureVersion("30"), "needs_review", "source_version_conflict"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			intake := newWorkflowIntake(t)
			if _, err := intake.ReceiveVerified(context.Background(), tc.first); err != nil { t.Fatal(err) }
			got, err := intake.ReceiveVerified(context.Background(), tc.second)
			if err != nil { t.Fatal(err) }
			if got.Status != tc.wantStatus || got.Reason != tc.wantReason {
				t.Fatalf("got=%+v", got)
			}
		})
	}
}
```

- [ ] **Step 2: 定义 source 版本比较器**

```go
type ExternalVersionOrder int
const (
	ExternalVersionOlder ExternalVersionOrder = -1
	ExternalVersionSame ExternalVersionOrder = 0
	ExternalVersionNewer ExternalVersionOrder = 1
	ExternalVersionIncomparable ExternalVersionOrder = 2
)

type ExternalVersionComparator interface {
	Compare(platform SourcePlatform, currentVersion string, currentAt time.Time, incomingVersion string, incomingAt time.Time) ExternalVersionOrder
}
```

三份 Provider 必须产生稳定 `SourceVersion`。纯十进制版本按任意精度整数比较；非十进制版本只有完全相同或官方时间戳严格不同时可比较；版本不同且时间相同进入 `ExternalVersionIncomparable`。接收时间不得参与业务新旧判断。

- [ ] **Step 3: 实现单事务 upsert**

`receiveVerified` 在一个事务中：

1. 以 `platform + binding_key + provider_event_id` 或版本唯一键插入 immutable snapshot。
2. 锁定 `delivery_external_order_workflows` 当前行。
3. 首个取消写 `cancelled` 与 `tombstoned_at`；迟到的旧 created 记 stale，不复活。
4. 新版本替换 current snapshot；相同版本不同 digest 写 `needs_review/source_version_conflict`。
5. 首次 created/updated 根据快照中的策略写 `awaiting_confirmation` 或 `pending`。
6. workflow 已有关联配送单时不创建第二个；只推进更新/取消路径。
7. 策略值取回调回查时保存的 `PolicyVersion`，不在之后偷读最新开关改变历史决定。

claim 使用 `FOR UPDATE SKIP LOCKED`，可认领 `pending`、到期 `retry_wait` 和租约过期 `processing`；完成与失败均校验 `id + row_version + lease_token`。

- [ ] **Step 4: 用独立 MySQL 验证并发**

`zz_external_workflow_mysql_test.go` 同时启动 20 个 goroutine 提交同一 created、旧 update、新 cancel 和重复 cancel，断言：

- snapshot 数量等于唯一事件/版本数；
- 只有一个 workflow；
- cancel tombstone 保留；
- 配送单最多一条；
- 过期租约可回收，未过期租约不可抢；
- 跨商家绑定不能覆盖 workflow；
- 错误文本不含联系人、地址或 Token。

Run: `cd backend; $env:DELIVERY_PLATFORM_TEST_MYSQL_DSN='<isolated-test-dsn>'; go test ./internal/module/delivery -run 'TestExternalWorkflowMySQL|TestExternalWorkflowConverges' -count=1`

Expected: PASS；未提供独立 DSN 的 MySQL 用例明确 SKIP。

- [ ] **Step 5: 提交收敛仓储**

```powershell
git add backend/internal/module/delivery/external_workflow_repository.go backend/internal/module/delivery/external_intake_repository.go backend/internal/module/delivery/external_workflow_test.go backend/internal/module/delivery/zz_external_workflow_mysql_test.go
git commit -m "功能：收敛外部订单版本与取消事件"
```

### Task 4: 持久接收 Provider 回调并返回专用应答

**Files:**
- Create: `backend/internal/module/deliveryplatform/callback_repository.go`
- Create: `backend/internal/module/admin/deliveryintake/provider_handler.go`
- Create: `backend/internal/module/admin/deliveryintake/provider_handler_test.go`
- Modify: `backend/internal/module/deliveryplatform/repository.go`
- Modify: `backend/internal/module/admin/deliveryintake/handler.go`

- [ ] **Step 1: 写 handler 的 durability-before-ack 测试**

```go
func TestProviderHandlerPersistsHintBeforeWritingPinnedAck(t *testing.T) {
	provider := &stubProvider{
		hint: deliveryplatform.CallbackHint{
			EventID: "event-1", SourceOrderID: "order-15",
			EventKind: "created", OccurredAt: time.Date(2026, 10, 24, 8, 0, 0, 0, time.UTC),
			QueryHint: json.RawMessage(`{"cursor":"safe-locator"}`),
		},
		ack: deliveryplatform.CallbackAck{Status: 200, ContentType: "application/json", Body: []byte(`{"data":"OK"}`)},
	}
	repo := &recordingCallbackRepository{}
	router := providerRouter(provider, repo)
	response := performRaw(router, "POST", "/api/v1/delivery/external-orders/meituan/binding-key/events", "application/x-www-form-urlencoded", "orderId=order-15")
	if response.Code != 200 || response.Body.String() != `{"data":"OK"}` { t.Fatalf("%d %s", response.Code, response.Body.String()) }
	if !repo.persistedBeforeAck { t.Fatal("ack written before callback persistence") }
}
```

- [ ] **Step 2: 为 repository 增加 callback 原子方法**

```go
type CallbackRecord struct {
	ID uint64
	Platform delivery.SourcePlatform
	BindingKey string
	ProviderEventID string
	SourceOrderID string
	EventKind string
	OccurredAt time.Time
	QueryHint json.RawMessage
	Fingerprint string
	Status string
	Attempts uint32
	LeaseToken *string
	LeaseExpiresAt *time.Time
	NextAttemptAt *time.Time
	Reason string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CallbackRepository interface {
	InsertCallback(context.Context, *CallbackRecord) (duplicate bool, err error)
	ClaimCallback(context.Context, time.Time, time.Duration) (CallbackRecord, bool, error)
	CompleteCallback(context.Context, CallbackRecord, string, string, time.Time) error
	RetryCallback(context.Context, CallbackRecord, string, time.Time, uint32, time.Time) error
}
```

重复事件必须比较 `platform`、`bindingKey`、`providerEventID` 和 fingerprint；相同 ID 不同摘要返回 `ErrCallbackConflict`。repository 不保存原始 body、Header、签名、Token、联系人或地址。

- [ ] **Step 3: 实现 Provider 专用 handler**

```go
func (h *ProviderHandler) receive(c *gin.Context) {
	platform, key := delivery.SourcePlatform(c.Param("platform")), c.Param("bindingKey")
	provider, ok := h.registry.Provider(platform)
	if !ok { response.Fail(c, http.StatusServiceUnavailable, "provider unavailable"); return }
	binding, envelope, err := h.readVerifiedEnvelope(c, key, platform)
	if err != nil { h.writeSafeError(c, err); return }
	hint, ack, err := provider.VerifyCallback(c.Request.Context(), envelope, binding)
	if err != nil { h.writeSafeError(c, err); return }
	if _, err := h.callbacks.Persist(c.Request.Context(), platform, key, hint); err != nil {
		h.writeSafeError(c, err)
		return
	}
	c.Data(ack.Status, ack.ContentType, ack.Body)
}
```

`readVerifiedEnvelope` 限制 body 为 1 MiB、query 为 8 KiB、禁止未知平台、按 `bindingKey` 查询 active binding，并保留验签所需原始字节。成功 response 必须逐字使用 Provider 返回的 `CallbackAck`；删除旧 handler 的通用 `response.OK` callback 路径。商家 receipt 查询路由仍保留。

- [ ] **Step 4: 覆盖安全失败**

测试：错误签名、过期时间、binding 不存在、平台错配、超大 body、相同 ID 不同摘要、持久化失败、Provider ack 非 JSON、返回内容不泄露校验细节。回调未持久化时不能返回成功 ack。

Run: `cd backend; go test ./internal/module/admin/deliveryintake ./internal/module/deliveryplatform -run 'TestProviderHandler|TestCallbackRepository' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交 callback 桥接**

```powershell
git add backend/internal/module/deliveryplatform/callback_repository.go backend/internal/module/deliveryplatform/repository.go backend/internal/module/admin/deliveryintake/provider_handler.go backend/internal/module/admin/deliveryintake/provider_handler_test.go backend/internal/module/admin/deliveryintake/handler.go
git commit -m "接口：持久接收三平台订单回调"
```

### Task 5: 实现主动回查 worker、Token 刷新协调和错误分类

**Files:**
- Create: `backend/internal/module/deliveryplatform/callback_worker.go`
- Create: `backend/internal/module/deliveryplatform/callback_worker_test.go`
- Modify: `backend/internal/module/deliveryplatform/types.go` — 只增加统一错误提取 helper，共享错误类型已由 foundation 定义。
- Modify: `backend/internal/module/deliveryplatform/repository.go`

- [ ] **Step 1: 使用 foundation 的共享错误类型并增加统一提取 helper**

```go
func ClassifyProviderError(err error) (ProviderErrorClass, time.Duration) {
	var classified ClassifiedProviderError
	if !errors.As(err, &classified) {
		return ProviderErrorContract, 0
	}
	after := classified.RetryAfter()
	if after < 0 { after = 0 }
	return classified.Class(), after
}
```

foundation 已定义 `ProviderErrorClass`、`ClassifiedProviderError` 和 `NewProviderError`。三份 Provider 计划必须将 HTTP 429/官方限频映射为 `rate_limited`，超时/5xx 为 `retryable`，401/Token 失效为 `authorization`，门店/订单明确不存在为 `permanent`，未知字段/状态/签名合同变化为 `contract`。错误消息只保留稳定分类，不带 URL query、Token 或响应原文。

- [ ] **Step 2: 写 worker 失败矩阵**

```go
func TestCallbackWorkerFetchesAuthorityBeforeReceive(t *testing.T) {
	provider := &stubProvider{snapshot: officialSnapshotFixture()}
	receiver := &recordingVerifiedReceiver{}
	worker := newCallbackWorker(provider, receiver)
	result, err := worker.ProcessNext(context.Background())
	if err != nil || !result.Processed { t.Fatalf("%+v %v", result, err) }
	if provider.fetchCalls != 1 || receiver.calls != 1 { t.Fatalf("fetch=%d receive=%d", provider.fetchCalls, receiver.calls) }
	if receiver.value.Event.Source.SourceOrderID == nil { t.Fatal("authority snapshot missing") }
}

func TestCallbackWorkerClassifiesFailureWithoutImport(t *testing.T) {
	for _, class := range []ProviderErrorClass{
		ProviderErrorRetryable, ProviderErrorRateLimited,
		ProviderErrorAuthorization, ProviderErrorPermanent, ProviderErrorContract,
	} {
		t.Run(string(class), func(t *testing.T) {
			worker, receiver, repo := workerWithProviderError(class)
			_, _ = worker.ProcessNext(context.Background())
			if receiver.calls != 0 { t.Fatal("unverified order reached intake") }
			assertCallbackTerminalOrRetry(t, repo.record, class)
		})
	}
}
```

- [ ] **Step 3: 实现 worker**

```go
type VerifiedReceiver interface {
	ReceiveVerified(context.Context, delivery.VerifiedExternalOrder) (delivery.ExternalWorkflowReceipt, error)
}

type CallbackWorker struct {
	callbacks CallbackRepository
	bindings Repository
	registry *Registry
	receiver VerifiedReceiver
	now func() time.Time
	lease time.Duration
	maxAttempts uint32
}
```

`ProcessNext` 顺序固定：

1. claim callback；
2. 读取同一 binding 的 active `BindingRecord` 并解密 Token；
3. 找到同平台且 AppKey 相同的 Provider；
4. 调用 `FetchOrder`；
5. 核对返回事件的门店、完整订单号和平台；
6. 生成 `delivery.VerifiedExternalOrder`，复制 callback 对应时刻的 `PolicyVersion`、`AutoImport`、`AutoDispatch`；
7. 调用 `ReceiveVerified`；
8. 成功后将 callback 标记 `verified`。

授权错误只尝试一次受租约保护的 Token 刷新；刷新成功后以新 Token 回查同一订单，刷新失败把 binding 置为 `reauthorization_required` 并将 callback 标记 `needs_review/authorization_expired`。限频尊重 `RetryAfter`，其他可重试错误使用 2、4、8、16、32、60 秒上限退避；永久不存在为 `needs_review/source_order_unavailable`；合同错误为 `needs_review/provider_contract_changed`；超过八次为 `deadletter/query_exhausted`。

- [ ] **Step 4: 运行并提交 worker**

Run: `cd backend; go test ./internal/module/deliveryplatform -run 'TestCallbackWorker|TestTokenRefreshLease' -count=1`

Expected: PASS，测试 HTTP server 之外无网络访问。

```powershell
git add backend/internal/module/deliveryplatform/types.go backend/internal/module/deliveryplatform/repository.go backend/internal/module/deliveryplatform/callback_worker.go backend/internal/module/deliveryplatform/callback_worker_test.go
git commit -m "功能：回查并验证三平台权威订单"
```

### Task 6: 实现人工确认、自动导入与更新/取消状态机

**Files:**
- Modify: `backend/internal/module/delivery/external_intake.go`
- Modify: `backend/internal/module/delivery/external_workflow_repository.go`
- Modify: `backend/internal/module/delivery/dispatch_repository.go`
- Test: `backend/internal/module/delivery/external_workflow_test.go`
- Test: `backend/internal/module/delivery/zz_external_workflow_mysql_test.go`

- [ ] **Step 1: 写策略与状态矩阵**

```go
func TestWorkflowPolicyMatrix(t *testing.T) {
	cases := []struct {
		name string
		autoImport, autoDispatch bool
		wantStatus string
		wantOrders int
	}{
		{"manual confirmation", false, false, "awaiting_confirmation", 0},
		{"automatic import", true, false, "imported", 1},
		{"automatic dispatch awaits funds", true, true, "awaiting_verified_funds", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			intake := newWorkflowIntakeWithPolicy(t, tc.autoImport, tc.autoDispatch, nil)
			receipt := receiveAndDrain(t, intake, verifiedOrderFixture())
			if receipt.Status != tc.wantStatus || countOrders(t) != tc.wantOrders {
				t.Fatalf("receipt=%+v orders=%d", receipt, countOrders(t))
			}
		})
	}
}
```

- [ ] **Step 2: 实现确认和导入命令**

```go
func (s *ExternalIntake) ConfirmImport(ctx context.Context, merchantID, workflowID, expectedVersion uint64) (ExternalWorkflowReceipt, error)
func (s *ExternalIntake) ProcessNext(ctx context.Context) (ExternalWorkflowReceipt, bool, error)
func (s *ExternalIntake) Requery(ctx context.Context, merchantID, workflowID, expectedVersion uint64) error
```

`ConfirmImport` 只允许 workflow 所属商家从 `awaiting_confirmation` 推进 `pending`；它不接受联系人、地址、费用、平台、门店或订单号。导入重用 `DispatchService.CreateOrder` 的服务端费用快照，`MerchantRequestID` 从稳定的 workflow ID 派生为 ULID 幂等键；新配送单只能是 `pending_fee_payment`。

- [ ] **Step 3: 实现可信更新与取消**

状态规则写为显式 switch：

```go
func nextExternalWorkflowAction(order Order, event ExternalOrderEventType) externalAction {
	switch event {
	case ExternalOrderUpdated:
		if order.Status == StatusPendingFeePayment || order.Status == StatusPaymentFailed {
			return externalActionUpdateUnpublished
		}
		return externalActionReview
	case ExternalOrderCancelled:
		switch order.Status {
		case StatusPendingFeePayment, StatusPaymentFailed:
			return externalActionCancel
		case StatusReadyForClaim:
			return externalActionCancelAndRefund
		case StatusClaimed, StatusToPickup:
			return externalActionReleaseAndReview
		case StatusPickedUp, StatusDelivering, StatusDelivered:
			return externalActionReview
		}
	}
	return externalActionReview
}
```

更新只允许同步取送联系人、地址和备注，并要求 workflow 当前版本和订单 `row_version` 同时匹配。取消不删除订单：未开放骑手池可取消；已开放无人接单走既有退款；已接单未取货先进入异常处置，不自动伪造骑手已释放；已取货/已送达只写 `needs_review/source_cancel_after_fulfillment`。

- [ ] **Step 4: 验证重复导入与退款边界**

测试同订单重复 created、重复确认、导入事务回滚、费用配置缺失、取消与骑手 claim 竞争、取消与支付 callback 竞争、更新与人工编辑竞争。每种竞争只有合法状态获胜；失败方保留稳定 reason，不覆盖已支付、已接单或已送达事实。

Run: `cd backend; $env:DELIVERY_PLATFORM_TEST_MYSQL_DSN='<isolated-test-dsn>'; go test ./internal/module/delivery -run 'TestWorkflowPolicyMatrix|TestExternalImport|TestExternalCancel' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交导入流程**

```powershell
git add backend/internal/module/delivery/external_intake.go backend/internal/module/delivery/external_workflow_repository.go backend/internal/module/delivery/dispatch_repository.go backend/internal/module/delivery/external_workflow_test.go backend/internal/module/delivery/zz_external_workflow_mysql_test.go
git commit -m "功能：按门店策略导入外部配送单"
```

### Task 7: 用真实资金端口约束自动发单并写业务 outbox

**Files:**
- Modify: `backend/internal/module/delivery/external_verified.go`
- Modify: `backend/internal/module/delivery/external_intake.go`
- Modify: `backend/internal/module/delivery/funds_service.go`
- Modify: `backend/internal/module/delivery/dispatch_types.go`
- Modify: `backend/internal/module/delivery/dispatch_repository.go`
- Modify: `backend/internal/module/admin/deliverynotify/consumer.go`
- Modify: `backend/internal/module/admin/deliverynotify/consumer_test.go`

- [ ] **Step 1: 定义不能被 mock 满足的资金端口**

```go
type VerifiedFundsDecision string
const (
	VerifiedFundsPaid VerifiedFundsDecision = "paid"
	VerifiedFundsInsufficient VerifiedFundsDecision = "insufficient"
	VerifiedFundsUnknown VerifiedFundsDecision = "unknown"
)

type VerifiedFundsResult struct {
	Decision VerifiedFundsDecision
	Order Order
	ReferencePublicID string
}

type VerifiedFundsPort interface {
	Ready() bool
	PayDeliveryFee(
		context.Context,
		uint64,
		string,
		uint64,
		string,
	) (VerifiedFundsResult, error)
}
```

`PayDeliveryFee` 参数依次为商家 ID、配送单公开 ID、订单 row version、稳定幂等 ULID。只有支付完整性计划实现“已核实充值批次/真实微信支付”后才能提供 production adapter。`PaymentMock`、`ConfirmMockFee`、只看 `available_balance_cents` 的 adapter 均不得实现此接口。

- [ ] **Step 2: 写 fail-closed 测试**

```go
func TestAutoDispatchRequiresVerifiedFunds(t *testing.T) {
	for _, test := range []struct {
		name string
		port VerifiedFundsPort
		wantStatus, wantReason string
	}{
		{"missing", nil, "awaiting_verified_funds", "verified_funds_unavailable"},
		{"not ready", stubFunds{ready: false}, "awaiting_verified_funds", "verified_funds_unavailable"},
		{"insufficient", stubFunds{ready: true, decision: VerifiedFundsInsufficient}, "awaiting_verified_funds", "insufficient_verified_balance"},
		{"unknown", stubFunds{ready: true, decision: VerifiedFundsUnknown}, "needs_review", "payment_result_unknown"},
		{"paid", stubFunds{ready: true, decision: VerifiedFundsPaid}, "dispatched", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := runAutoDispatch(t, test.port)
			if got.Status != test.wantStatus || got.Reason != test.wantReason { t.Fatalf("%+v", got) }
		})
	}
}
```

- [ ] **Step 3: 实现单事务资金后状态推进**

真实资金 adapter 完成支付时，必须在同一数据库事务内写真实资金引用、把订单从 `pending_fee_payment` 推进 `ready_for_claim`、写 `RiderPoolOpened` outbox，并把 workflow 置为 `dispatched`。若真实资金动作在外部网络后结果未知，workflow 为 `needs_review/payment_result_unknown`，worker 只能查询原支付，不能换幂等键重扣。

- [ ] **Step 4: 增加外部订单事件**

```go
const (
	EventExternalOrderAwaitingConfirmation = "ExternalOrderAwaitingConfirmation"
	EventExternalOrderImported = "ExternalOrderImported"
	EventExternalAuthorizationExpired = "ExternalAuthorizationExpired"
	EventExternalVerifiedFundsInsufficient = "ExternalVerifiedFundsInsufficient"
	EventExternalCancellationConflict = "ExternalCancellationConflict"
)

type ExternalOrderNotificationPayload struct {
	MerchantUserID uint64 `json:"merchantUserId"`
	WorkflowID uint64 `json:"workflowId"`
	OrderPublicID string `json:"orderPublicId,omitempty"`
	Platform SourcePlatform `json:"platform"`
	MaskedSourceOrderID string `json:"maskedSourceOrderId"`
	Reason string `json:"reason,omitempty"`
}
```

payload 不含姓名、电话、地址、Token 或完整平台订单号。workflow 状态改变与 outbox 插入同事务，唯一键仍为 aggregate/version/event。扩展 `deliverynotify.Consumer` 的解码分派，使这些事件进入商家通知展开端口；若通用通知计划尚未提供 sender，记录为 `blocked_channel_not_configured`，不得标记 `succeeded` 或发送到骑手 WeCom 群。

- [ ] **Step 5: 运行并提交资金闸门和事件**

Run: `cd backend; go test ./internal/module/delivery ./internal/module/admin/deliverynotify -run 'TestAutoDispatch|TestExternalOrderNotification' -count=1`

Expected: PASS；搜索测试确认 `PaymentMock` 没有实现 `VerifiedFundsPort`。

```powershell
git add backend/internal/module/delivery/external_verified.go backend/internal/module/delivery/external_intake.go backend/internal/module/delivery/funds_service.go backend/internal/module/delivery/dispatch_types.go backend/internal/module/delivery/dispatch_repository.go backend/internal/module/admin/deliverynotify/consumer.go backend/internal/module/admin/deliverynotify/consumer_test.go
git commit -m "安全：以真实资金闸门约束自动发单"
```

### Task 8: 暴露商家 workflow 和后台只读处置接口

**Files:**
- Create: `backend/internal/module/admin/deliveryplatform/workflow_handler.go`
- Create: `backend/internal/module/admin/deliveryplatform/workflow_handler_test.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: 写权限和命令白名单测试**

```go
func TestWorkflowHandlerScopesMerchantAndAdminCommands(t *testing.T) {
	merchantRouter := workflowMerchantRouter(21)
	assertStatus(t, merchantRouter, "GET", "/merchant/external-order-workflows?status=awaiting_confirmation", "", 200)
	assertStatus(t, merchantRouter, "POST", "/merchant/external-order-workflows/44/confirm-import", `{"expectedRowVersion":3}`, 200)
	assertStatus(t, merchantRouter, "POST", "/merchant/external-order-workflows/44/confirm-import", `{"expectedRowVersion":3,"paid":true}`, 400)
	assertNoCrossMerchantRows(t, merchantRouter, 22)

	adminRouter := workflowAdminRouter(permissionDeliveryOps)
	assertStatus(t, adminRouter, "POST", "/admin/delivery/external-orders/44/requery", `{"expectedRowVersion":3,"reason":"核对平台原单"}`, 200)
	assertRouteMissing(t, adminRouter, "POST", "/admin/delivery/external-orders/44/mark-paid")
	assertRouteMissing(t, adminRouter, "POST", "/admin/delivery/external-orders/44/mark-dispatched")
}
```

- [ ] **Step 2: 实现商家合同**

路由：

```text
GET    /api/v1/delivery/merchant/platform-connections
PATCH  /api/v1/delivery/merchant/platform-connections/:connectionId/policy
GET    /api/v1/delivery/merchant/external-order-workflows
GET    /api/v1/delivery/merchant/external-order-workflows/:id
POST   /api/v1/delivery/merchant/external-order-workflows/:id/confirm-import
POST   /api/v1/delivery/merchant/external-order-workflows/:id/requery
```

列表状态白名单为 `awaiting_confirmation`、`awaiting_verified_funds`、`imported`、`dispatched`、`cancelled`、`needs_review`、`deadletter`。响应返回脱敏 source order ID、平台、门店名、状态、稳定 reason、配送单公开 ID、策略版本、rowVersion 和时间；不返回 bindingKey、Token、完整电话/地址或原始 payload。

- [ ] **Step 3: 实现后台合同**

```text
GET   /api/v1/admin/delivery/external-orders
GET   /api/v1/admin/delivery/external-orders/:id
POST  /api/v1/admin/delivery/external-orders/:id/requery
```

后台 requery 只把当前 workflow 关联的 source order 定位符重新加入 callback query 队列，要求管理员配送权限、CSRF、原因和 expectedRowVersion。没有“设为已验证”“设为已付款”“设为已派发”路由。

- [ ] **Step 4: 运行并提交 API**

Run: `cd backend; go test ./internal/module/admin/deliveryplatform ./cmd/server -run 'TestWorkflowHandler|TestDeliveryRoutes' -count=1`

Expected: PASS，包含 400/401/403/404/409/503。

```powershell
git add backend/internal/module/admin/deliveryplatform/workflow_handler.go backend/internal/module/admin/deliveryplatform/workflow_handler_test.go backend/cmd/server/main.go
git commit -m "接口：开放外部订单确认与重新查询"
```

### Task 9: 增加商家端连接/导入页面并保持现有视觉

**Files:**
- Create: `apps/miniapp/src/pages/merchant-platform-orders/index.config.ts`
- Create: `apps/miniapp/src/pages/merchant-platform-orders/index.tsx`
- Create: `apps/miniapp/src/pages/merchant-platform-orders/index.scss`
- Modify: `apps/miniapp/src/app.config.ts`
- Modify: `apps/miniapp/src/services/delivery-platform.ts`
- Modify: `apps/miniapp/src/pages/delivery-profile/index.tsx`
- Modify: `apps/miniapp/src/pages/delivery-profile/index.scss`

- [ ] **Step 1: 固定前端类型**

```ts
export type ExternalPlatform = 'taobao' | 'jd' | 'meituan'
export type ExternalWorkflowStatus =
  | 'awaiting_confirmation'
  | 'awaiting_verified_funds'
  | 'imported'
  | 'dispatched'
  | 'cancelled'
  | 'needs_review'
  | 'deadletter'

export interface PlatformConnection {
  connectionId: number
  platform: ExternalPlatform
  label: string
  configured: boolean
  authorized: boolean
  callbackHealthy: boolean
  queryHealthy: boolean
  bindingStatus: 'active' | 'refresh_required' | 'reauthorization_required' | 'suspended' | 'revoked'
  sourceShopName: string
  autoImport: boolean
  autoDispatch: boolean
  policyVersion: number
  blockingReason: string
}

export interface ExternalOrderWorkflow {
  id: number
  platform: ExternalPlatform
  sourceShopName: string
  maskedSourceOrderId: string
  status: ExternalWorkflowStatus
  reason: string
  orderPublicId: string
  policyVersion: number
  rowVersion: number
  updatedAt: string
}
```

- [ ] **Step 2: 实现 service**

```ts
export const listPlatformConnections = () =>
  request<PlatformConnection[]>('/delivery/merchant/platform-connections', { url: '', timeout: 10000 })

export const updatePlatformPolicy = (
  connectionId: number,
  value: { autoImport: boolean; autoDispatch: boolean; expectedPolicyVersion: number; reason: string }
) => request<PlatformConnection>(`/delivery/merchant/platform-connections/${connectionId}/policy`, {
  url: '', method: 'PATCH', data: value, timeout: 10000
})

export const listExternalOrderWorkflows = (status?: ExternalWorkflowStatus) =>
  request<Page<ExternalOrderWorkflow>>(
    `/delivery/merchant/external-order-workflows?page=1&pageSize=50${status ? `&status=${encodeURIComponent(status)}` : ''}`,
    { url: '', timeout: 10000 }
  )

export const confirmExternalOrderImport = (value: ExternalOrderWorkflow) =>
  request<ExternalOrderWorkflow>(`/delivery/merchant/external-order-workflows/${value.id}/confirm-import`, {
    url: '', method: 'POST', data: { expectedRowVersion: value.rowVersion }, timeout: 10000
  })

export const requeryExternalOrder = (value: ExternalOrderWorkflow) =>
  request<{ queued: true }>(`/delivery/merchant/external-order-workflows/${value.id}/requery`, {
    url: '', method: 'POST', data: { expectedRowVersion: value.rowVersion }, timeout: 10000
  })
```

- [ ] **Step 3: 实现页面状态与操作**

新页面沿用 `design-tokens.scss`、`reference-layout.scss` 和商家端蓝色调。顶部展示三平台真实状态；中部两个开关明确解释：

- 自动导入关闭：平台订单验证后由商家确认；
- 自动发单关闭：导入后等待真实配送费；
- 自动发单开启：仅在真实资金可用时生效。

`autoDispatch` 开启时同时提交 `autoImport=true`；关闭 `autoImport` 时同时提交 `autoDispatch=false`。开关请求期间禁用重复触发，409 后刷新服务端状态。列表显示待确认、待资金、待处理、已导入、已发单和冲突；只对允许状态展示“确认导入”或“重新查询”。加载、空态、错误态统一使用现有 `DeliveryOrderLoading` 和 `DeliveryOrderState`。

- [ ] **Step 4: 接入“我的”且不改工作台**

在 `delivery-profile` 的商家“配送资金”之后添加一条“平台订单接入”，跳转 `/pages/merchant-platform-orders/index`。骑手角色不显示；不修改 `merchant-home` 卡片、双列布局、手势、配色或订单状态。

- [ ] **Step 5: typecheck、构建并提交**

Run:

```powershell
npm --workspace apps/miniapp run typecheck
npm run build:miniapp
```

Expected: exit 0；允许既有 webpack 体积/顺序 warning。

```powershell
git add apps/miniapp/src/app.config.ts apps/miniapp/src/services/delivery-platform.ts apps/miniapp/src/pages/merchant-platform-orders apps/miniapp/src/pages/delivery-profile/index.tsx apps/miniapp/src/pages/delivery-profile/index.scss
git commit -m "界面：增加商家平台订单接入管理"
```

### Task 10: 增加后台可观测性，不提供伪造状态按钮

**Files:**
- Create: `apps/admin/src/ExternalOrderConsole.tsx`
- Modify: `apps/admin/src/services/delivery-admin.ts`
- Modify: `apps/admin/src/DeliveryConsole.tsx`
- Modify: `apps/admin/src/App.tsx`

- [ ] **Step 1: 扩展后台类型和 API**

```ts
export interface ExternalOrderWorkflowAdmin {
  id: number
  merchantUserId: number
  platform: 'taobao' | 'jd' | 'meituan'
  sourceShopName: string
  maskedSourceOrderId: string
  status: string
  reason: string
  orderPublicId: string
  policyVersion: number
  rowVersion: number
  attempts: number
  nextAttemptAt?: string
  updatedAt: string
}
```

在 `deliveryAdminAPI` 增加 `externalOrders(query)`、`externalOrder(id)` 和 `requeryExternalOrder(id, expectedRowVersion, reason)`。不增加任何 `markVerified`、`markPaid` 或 `markDispatched` 方法。

- [ ] **Step 2: 实现控制台**

`ExternalOrderConsole` 使用现有 `AdminPageHeader`、`AdminToolbar`、`AdminTableFrame`、`AdminStatusBadge`、`AdminPagination` 和 `AdminDialog`。筛选平台/状态/商家 ID，列表只显示脱敏订单号、阶段、原因、尝试次数和下次重试；详情不显示完整地址/电话。只有 `needs_review`、`deadletter`、`reauthorization_required` 对应记录显示“重新查询原单”，提交时要求填写原因并携带 row version。

- [ ] **Step 3: 接入现有配送控制台导航**

在配送控制台加入“外部订单”子页，不创建第二套配送管理导航，不修改 `ExternalTakeoutMerchants`。保留原订单/资金/骑手结算页面。

- [ ] **Step 4: typecheck 并提交**

Run: `npm --workspace apps/admin run typecheck`

Expected: PASS。

```powershell
git add apps/admin/src/ExternalOrderConsole.tsx apps/admin/src/services/delivery-admin.ts apps/admin/src/DeliveryConsole.tsx apps/admin/src/App.tsx
git commit -m "后台：增加外部订单异常处置视图"
```

### Task 11: 组合根、worker 生命周期和能力开关

**Files:**
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/delivery_capabilities_test.go`
- Create: `scripts/check-external-order-workflow.cjs`
- Modify: `package.json`

- [ ] **Step 1: 写能力门禁测试**

```go
func TestExternalOrderCapabilityRequiresCompleteRuntime(t *testing.T) {
	cases := []struct {
		name string
		runtime *deliveryRuntime
		want bool
	}{
		{"nil", nil, false},
		{"provider only", &deliveryRuntime{platformRegistry: configuredRegistry()}, false},
		{"intake only", &deliveryRuntime{externalIntake: &delivery.ExternalIntake{}}, false},
		{"worker only", &deliveryRuntime{platformCallbackWorker: &deliveryplatform.CallbackWorker{}}, false},
		{"complete", &deliveryRuntime{
			platformRegistry: configuredRegistry(),
			externalIntake: &delivery.ExternalIntake{},
			platformCallbackWorker: &deliveryplatform.CallbackWorker{},
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := externalOrdersReady(tc.runtime); got != tc.want { t.Fatalf("got=%v", got) }
		})
	}
}
```

- [ ] **Step 2: 组合真实依赖**

`newDeliveryRuntime` 只在以下全部成立时设置 `externalIntake` 和启动 worker：

1. `EnableExternalOrders=true`；
2. Token AES key 合法；
3. registry 至少一个 Provider `Configured()`；
4. migration 048/050 所需 repository 可用；
5. callback worker 和 verified intake 构造成功。

worker 使用 server 根 context 启动，shutdown 时取消并等待退出；panic 由已有进程监督策略处理，不自动把能力报告为健康。Provider 缺一项配置时该 Provider 不注册；不能加载 stub。

- [ ] **Step 3: 真实表达能力**

```go
func externalOrdersReady(runtime *deliveryRuntime) bool {
	return runtime != nil &&
		runtime.platformRegistry != nil &&
		len(runtime.platformRegistry.ConfiguredPlatforms()) > 0 &&
		runtime.externalIntake != nil &&
		runtime.platformCallbackWorker != nil
}
```

`GET /api/v1/delivery/capabilities` 的 `externalOrders` 使用该函数。另返回每个平台的 `configured`，但 `authorized`、`callbackHealthy`、`queryHealthy` 按门店连接接口返回，不能由全局配置推断。

- [ ] **Step 4: 添加静态边界脚本**

`scripts/check-external-order-workflow.cjs` 扫描新增模块并失败于：

- `external_takeout_merchants` 出现在 `deliveryplatform` 或 workflow 源码；
- `PaymentMock`、`ConfirmMockFee` 出现在 `external_verified.go` 或 callback worker；
- callback handler 调用 `CreateOrder`、`PayOrderWithWallet`、`FetchOrder`；
- 前端出现 Secret/Token 字段；
- admin service 出现 `markVerified`、`markPaid`、`markDispatched`；
- 京东 Provider 出现 `opendj.jd.com`。

在 `package.json` 注册：

```json
"check:external-order-workflow": "node scripts/check-external-order-workflow.cjs"
```

- [ ] **Step 5: 运行并提交**

Run:

```powershell
cd backend
go test ./cmd/server ./internal/module/deliveryplatform ./internal/module/admin/deliveryintake -count=1
cd ..
npm run check:external-order-workflow
```

Expected: PASS。

```powershell
git add backend/cmd/server/main.go backend/cmd/server/delivery_capabilities_test.go scripts/check-external-order-workflow.cjs package.json
git commit -m "运行时：接通外部订单 worker 与能力门禁"
```

### Task 12: 全量自动化验收和阶段证据

- [ ] **Step 1: 运行 Go 定向测试**

```powershell
cd "C:/Users/33141/Documents/New project/backend"
go test ./internal/module/delivery ./internal/module/deliveryplatform ./internal/module/deliveryprovider/meituan ./internal/module/deliveryprovider/taobao ./internal/module/deliveryprovider/jd ./internal/module/admin/deliveryintake ./internal/module/admin/deliveryplatform ./internal/module/admin/deliverynotify ./cmd/server -count=1
```

Expected: PASS；未配置平台账号时只运行离线固定向量，不访问生产。

- [ ] **Step 2: 运行独立 MySQL 测试**

```powershell
$env:DELIVERY_PLATFORM_TEST_MYSQL_DSN='<isolated-test-dsn>'
go test ./migrations ./internal/module/delivery ./internal/module/deliveryplatform -run 'TestMigration050|TestExternalWorkflowMySQL|TestCallbackRepositoryMySQL' -count=1
Remove-Item Env:DELIVERY_PLATFORM_TEST_MYSQL_DSN
```

Expected: PASS。变量不存在时结果为 SKIP，必须在汇报中单列，不能写成通过。

- [ ] **Step 3: 运行仓库全量和前端验收**

```powershell
go test ./...
cd ..
npm --workspace apps/miniapp run typecheck
npm --workspace apps/admin run typecheck
npm run check:external-order-workflow
npm run check:boundaries
npm run check:real-wechat-secret-hygiene
node scripts/check-delivery-stage-workbench.cjs
npm run test:delivery-stage-workbench
node scripts/check-private-messaging.cjs
npm run build:miniapp
git diff --check
```

Expected: 全部 exit 0；`build:miniapp` 只允许既有 webpack 体积/顺序 warning。

- [ ] **Step 4: 人工检查 320/390 宽与后台**

商家端检查 320 与 390 CSS px：长门店名、三平台状态、两个策略开关、50 条列表、空态、错误态、409 刷新、无支付能力说明；点击区域不少于 44PX，不遮挡胶囊。后台检查脱敏订单号、筛选、分页、长 reason、无伪造状态按钮。原 `merchant-home`、骑手工作台、私信与发布页截图对比无变化。

- [ ] **Step 5: 分阶段汇报，不越级宣称**

报告分别列出：

1. schema/单元/MySQL/前端构建；
2. 三平台合同固定向量；
3. 各平台沙箱授权、回调和回查；
4. 影子比对；
5. 人工导入；
6. 自动导入；
7. 真实资金 adapter；
8. 自动发单；
9. 通知渠道；
10. 生产迁移与健康。

任一项未执行写 `PENDING` 或 `SKIP`。代码完成不等于沙箱接通，沙箱接通不等于生产授权，回调成功不等于配送单创建，配送单创建不等于已付款或已派发。

## 完成标准

- callback 在平台时限内持久接收并返回各自专用 ack，不在请求内查单/扣费。
- worker 只把主动回查的权威快照交给 `ReceiveVerified`。
- created/updated/cancelled 可按版本收敛；取消 tombstone 不被旧创建复活。
- `autoImport=false` 等商家确认，`autoImport=true` 才创建 `pending_fee_payment`。
- `autoDispatch=true` 仍需真实资金端口；缺失、余额不足和结果未知都 fail closed。
- 商家与后台只能执行允许的确认/重新查询动作，不能伪造验证、付款或派发。
- outbox 事件与业务状态同事务，通知发送不决定业务提交。
- 至少一个真实 Provider、worker 和 intake 同时就绪时才报告 `externalOrders=true`。
- 生产开关默认关闭，本计划本身不执行部署、迁移或真实扣款。
