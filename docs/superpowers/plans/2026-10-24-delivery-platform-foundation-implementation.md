# 外卖平台授权底座 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立三平台共用但不混用协议的应用注册、OAuth 会话、门店绑定、Token 加密、能力查询与审计底座，并在没有真实 Provider 时保持关闭。

**Architecture:** 新建 `deliveryplatform` 领域保存授权会话和门店绑定，并通过 Provider registry 注入美团、淘宝闪购、京东实现。平台密钥仍由环境/密钥管理提供，数据库只保存加密后的门店 Token；现有 `delivery` 领域只看到可信 `ExternalShopBinding`，不读取密钥。此计划不实现任何平台签名，也不把 `externalOrders` 宣告为可用。

**Tech Stack:** Go 1.25、Gin、GORM、MySQL 8.4、AES-256-GCM、React/Taro TypeScript、现有 `response`/管理员鉴权/商家身份守卫。

---

## 实施边界与文件结构

冻结依据：`docs/superpowers/specs/2026-10-24-external-platform-connection-design.md`，设计提交 `08bfef25e7a89e7eca3e08cf42cd08e81ab1d349`。

本计划创建/修改：

- Create: `backend/migrations/048_delivery_platform_connections.sql` — 授权会话、绑定、审计与回调提示表。
- Create: `backend/migrations/migration_048_test.go` — DDL 合同和独立 MySQL 重复执行测试。
- Create: `backend/internal/module/deliveryplatform/types.go` — 共用类型与 Provider 契约。
- Create: `backend/internal/module/deliveryplatform/cipher.go` — Token 认证加密。
- Create: `backend/internal/module/deliveryplatform/repository.go` — GORM 持久化、CAS、租约。
- Create: `backend/internal/module/deliveryplatform/registry.go` — 三个平台实现注册表。
- Create: `backend/internal/module/deliveryplatform/service.go` — 授权、绑定、策略和撤销服务。
- Create: `backend/internal/module/deliveryplatform/*_test.go` — 单元与 MySQL 测试。
- Create: `backend/internal/module/admin/deliveryplatform/handler.go` — 商家、OAuth 回调、后台只读路由。
- Create: `backend/internal/module/admin/deliveryplatform/handler_test.go` — 鉴权、越权、严格 JSON 和状态码测试。
- Modify: `backend/internal/config/config.go`、`config_test.go` — 显式总开关和 32 字节 Token 密钥。
- Modify: `backend/cmd/server/main.go`、`delivery_capabilities_test.go` — 组合根与真实能力表达。
- Modify: `backend/internal/module/admin/deliverymerchant/handler.go`、`types.go`、`order_source_test.go` — `order-intake` 读取真实绑定状态。
- Create: `apps/miniapp/src/services/delivery-platform.ts` — 后续 UI 使用的稳定 API 类型。

此计划不修改 `external_takeout_merchants`、导航域、支付/钱包表、配送状态机和生产环境文件。

### Task 1: 建立 migration 048 的失败合同测试

**Files:**
- Create: `backend/migrations/migration_048_test.go`
- Create: `backend/migrations/048_delivery_platform_connections.sql`

- [ ] **Step 1: 先写只读 DDL 合同测试**

```go
package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestMigration048Contract(t *testing.T) {
	raw, err := os.ReadFile("048_delivery_platform_connections.sql")
	if err != nil { t.Fatal(err) }
	source := string(raw)
	for _, token := range []string{
		"delivery_platform_authorization_sessions",
		"delivery_platform_shop_bindings",
		"delivery_platform_binding_audits",
		"delivery_platform_callbacks",
		"state_hash BINARY(32)",
		"binding_key CHAR(43)",
		"access_token_ciphertext VARBINARY(4096)",
		"refresh_lease_token CHAR(26)",
		"UNIQUE KEY uq_delivery_platform_shop",
		"CHECK (platform IN ('taobao','jd','meituan'))",
	} {
		if !strings.Contains(source, token) { t.Fatalf("migration 048 missing %q", token) }
	}
	for _, forbidden := range []string{"app_secret", "sign_key", "FOREIGN KEY", "DROP TABLE"} {
		if strings.Contains(strings.ToLower(source), strings.ToLower(forbidden)) {
			t.Fatalf("migration 048 contains forbidden %q", forbidden)
		}
	}
}
```

- [ ] **Step 2: 运行测试并确认先失败**

Run: `cd backend; go test ./migrations -run TestMigration048Contract -count=1`

Expected: FAIL，错误为找不到 `048_delivery_platform_connections.sql`。

- [ ] **Step 3: 添加完整的前向迁移**

```sql
CREATE TABLE IF NOT EXISTS delivery_platform_authorization_sessions (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  state_hash BINARY(32) NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  merchant_user_id BIGINT UNSIGNED NOT NULL,
  platform VARCHAR(12) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  provider_app_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  redirect_id VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  pkce_verifier_ciphertext VARBINARY(1024) NULL,
  status VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  lease_token CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  lease_expires_at DATETIME(6) NULL,
  expires_at DATETIME(6) NOT NULL,
  consumed_at DATETIME(6) NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_delivery_platform_auth_state (state_hash),
  KEY idx_delivery_platform_auth_expiry (status, expires_at),
  CONSTRAINT chk_delivery_platform_auth_platform CHECK (platform IN ('taobao','jd','meituan')),
  CONSTRAINT chk_delivery_platform_auth_status CHECK (status IN ('created','exchanging','consumed','expired','failed')),
  CONSTRAINT chk_delivery_platform_auth_identity CHECK (user_id > 0 AND merchant_user_id > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE IF NOT EXISTS delivery_platform_shop_bindings (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  binding_key CHAR(43) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  merchant_user_id BIGINT UNSIGNED NOT NULL,
  platform VARCHAR(12) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  provider_app_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  source_merchant_id VARCHAR(128) COLLATE utf8mb4_0900_bin NOT NULL,
  source_shop_id VARCHAR(128) COLLATE utf8mb4_0900_bin NOT NULL,
  source_shop_name VARCHAR(128) NOT NULL DEFAULT '',
  status VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  scopes JSON NOT NULL,
  access_token_ciphertext VARBINARY(4096) NOT NULL,
  refresh_token_ciphertext VARBINARY(4096) NULL,
  credential_key_version SMALLINT UNSIGNED NOT NULL,
  access_token_expires_at DATETIME(6) NULL,
  refresh_token_expires_at DATETIME(6) NULL,
  refresh_lease_token CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  refresh_lease_expires_at DATETIME(6) NULL,
  auto_import BOOLEAN NOT NULL DEFAULT FALSE,
  auto_dispatch BOOLEAN NOT NULL DEFAULT FALSE,
  policy_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  last_callback_at DATETIME(6) NULL,
  last_query_success_at DATETIME(6) NULL,
  revoked_at DATETIME(6) NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_delivery_platform_binding_key (binding_key),
  UNIQUE KEY uq_delivery_platform_shop (platform, provider_app_key, source_shop_id),
  KEY idx_delivery_platform_merchant (merchant_user_id, platform, id),
  CONSTRAINT chk_delivery_platform_binding_platform CHECK (platform IN ('taobao','jd','meituan')),
  CONSTRAINT chk_delivery_platform_binding_status CHECK (status IN ('active','refresh_required','reauthorization_required','suspended','revoked')),
  CONSTRAINT chk_delivery_platform_policy CHECK (auto_dispatch = FALSE OR auto_import = TRUE),
  CONSTRAINT chk_delivery_platform_binding_identity CHECK (merchant_user_id > 0 AND CHAR_LENGTH(TRIM(source_shop_id)) > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE IF NOT EXISTS delivery_platform_binding_audits (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  binding_id BIGINT UNSIGNED NOT NULL,
  actor_user_id BIGINT UNSIGNED NOT NULL,
  action VARCHAR(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  before_version BIGINT UNSIGNED NOT NULL,
  after_version BIGINT UNSIGNED NOT NULL,
  reason VARCHAR(256) NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  KEY idx_delivery_platform_audit_binding (binding_id, id),
  CONSTRAINT chk_delivery_platform_audit_identity CHECK (binding_id > 0 AND actor_user_id > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;

CREATE TABLE IF NOT EXISTS delivery_platform_callbacks (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  platform VARCHAR(12) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  binding_key CHAR(43) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  provider_event_id VARCHAR(128) COLLATE utf8mb4_0900_bin NOT NULL,
  source_order_id VARCHAR(128) COLLATE utf8mb4_0900_bin NOT NULL,
  event_kind VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  occurred_at DATETIME(6) NOT NULL,
  query_hint JSON NOT NULL,
  fingerprint CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(24) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT 'pending',
  attempts INT UNSIGNED NOT NULL DEFAULT 0,
  lease_token CHAR(26) CHARACTER SET ascii COLLATE ascii_bin NULL,
  lease_expires_at DATETIME(6) NULL,
  next_attempt_at DATETIME(6) NULL,
  reason VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL DEFAULT '',
  created_at DATETIME(6) NOT NULL,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_delivery_platform_callback (platform, binding_key, provider_event_id),
  KEY idx_delivery_platform_callback_claim (status, next_attempt_at, lease_expires_at, id),
  CONSTRAINT chk_delivery_platform_callback_platform CHECK (platform IN ('taobao','jd','meituan')),
  CONSTRAINT chk_delivery_platform_callback_status CHECK (status IN ('pending','processing','retry_wait','verified','stale','needs_review','deadletter'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;
```

- [ ] **Step 4: 运行 DDL 合同测试**

Run: `cd backend; go test ./migrations -run TestMigration048Contract -count=1`

Expected: PASS。

- [ ] **Step 5: 提交迁移合同**

```powershell
git add backend/migrations/048_delivery_platform_connections.sql backend/migrations/migration_048_test.go
git commit -m "数据库：新增外卖平台授权与绑定底座"
```

### Task 2: 定义共享类型和 Provider registry

**Files:**
- Create: `backend/internal/module/deliveryplatform/types.go`
- Create: `backend/internal/module/deliveryplatform/registry.go`
- Create: `backend/internal/module/deliveryplatform/registry_test.go`

- [ ] **Step 1: 写 registry 失败测试**

```go
func TestRegistryRejectsDuplicatesAndLocalPlatform(t *testing.T) {
	if _, err := NewRegistry(stubProvider{platform: delivery.SourceLocal}); !errors.Is(err, ErrInvalidProvider) { t.Fatal(err) }
	if _, err := NewRegistry(stubProvider{platform: delivery.SourceMeituan}, stubProvider{platform: delivery.SourceMeituan}); !errors.Is(err, ErrInvalidProvider) { t.Fatal(err) }
}

func TestRegistryReportsOnlyConfiguredProviders(t *testing.T) {
	r, err := NewRegistry(stubProvider{platform: delivery.SourceMeituan, configured: true}, stubProvider{platform: delivery.SourceJD})
	if err != nil { t.Fatal(err) }
	if got := r.ConfiguredPlatforms(); !reflect.DeepEqual(got, []delivery.SourcePlatform{delivery.SourceMeituan}) { t.Fatalf("%v", got) }
}
```

- [ ] **Step 2: 运行测试确认缺少类型**

Run: `cd backend; go test ./internal/module/deliveryplatform -run TestRegistry -count=1`

Expected: FAIL，`NewRegistry`/`Provider` 未定义。

- [ ] **Step 3: 添加完整契约类型**

```go
package deliveryplatform

type BindingStatus string
const (
	BindingActive BindingStatus = "active"
	BindingRefreshRequired BindingStatus = "refresh_required"
	BindingReauthorizationRequired BindingStatus = "reauthorization_required"
	BindingSuspended BindingStatus = "suspended"
	BindingRevoked BindingStatus = "revoked"
)

type Binding struct {
	ID uint64
	BindingKey string
	MerchantUserID uint64
	Platform delivery.SourcePlatform
	ProviderAppKey string
	SourceMerchantID string
	SourceShopID string
	SourceShopName string
	Status BindingStatus
	Scopes []string
	AutoImport bool
	AutoDispatch bool
	PolicyVersion uint64
	RowVersion uint64
	AccessTokenExpiresAt *time.Time
	RefreshTokenExpiresAt *time.Time
	LastCallbackAt *time.Time
	LastQuerySuccessAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TokenSet struct {
	AccessToken string
	RefreshToken string
	AccessTokenExpiresAt *time.Time
	RefreshTokenExpiresAt *time.Time
}

type AuthorizationRequest struct { State, RedirectURI, ProviderAppKey string }
type AuthorizationCallback struct { Code, State, RedirectURI string }
type AuthorizationGrant struct { SourceMerchantID string; Tokens TokenSet; Scopes []string }
type Shop struct { SourceShopID, SourceShopName string }
type CallbackHint struct { EventID, SourceOrderID, EventKind string; OccurredAt time.Time; QueryHint json.RawMessage }
type CallbackAck struct { Status int; ContentType string; Body []byte }
type OrderSnapshot struct { Event delivery.ExternalOrderEvent; SourceState, SourceVersion string }

type ProviderErrorClass string
const (
	ProviderErrorRetryable ProviderErrorClass = "retryable"
	ProviderErrorRateLimited ProviderErrorClass = "rate_limited"
	ProviderErrorAuthorization ProviderErrorClass = "authorization"
	ProviderErrorPermanent ProviderErrorClass = "permanent"
	ProviderErrorContract ProviderErrorClass = "contract"
)

type ClassifiedProviderError interface {
	error
	Class() ProviderErrorClass
	RetryAfter() time.Duration
}

type ProviderError struct {
	Kind ProviderErrorClass
	After time.Duration
}

func (e ProviderError) Error() string { return "delivery provider request failed: " + string(e.Kind) }
func (e ProviderError) Class() ProviderErrorClass { return e.Kind }
func (e ProviderError) RetryAfter() time.Duration { return e.After }

func NewProviderError(kind ProviderErrorClass, after time.Duration) error {
	switch kind {
	case ProviderErrorRetryable, ProviderErrorRateLimited, ProviderErrorAuthorization, ProviderErrorPermanent, ProviderErrorContract:
		return ProviderError{Kind: kind, After: max(after, 0)}
	default:
		return ProviderError{Kind: ProviderErrorContract}
	}
}

type Provider interface {
	Platform() delivery.SourcePlatform
	AppKey() string
	Configured() bool
	BuildAuthorizationURL(AuthorizationRequest) (string, error)
	ExchangeAuthorizationCode(context.Context, AuthorizationCallback) (AuthorizationGrant, error)
	Refresh(context.Context, TokenSet) (TokenSet, error)
	Revoke(context.Context, TokenSet) error
	ListShops(context.Context, TokenSet) ([]Shop, error)
	VerifyCallback(context.Context, delivery.ExternalOrderEnvelope, Binding) (CallbackHint, CallbackAck, error)
	FetchOrder(context.Context, TokenSet, Binding, CallbackHint) (OrderSnapshot, error)
}
```

- [ ] **Step 4: 实现显式 registry**

```go
type Registry struct { providers map[delivery.SourcePlatform]Provider }

func NewRegistry(values ...Provider) (*Registry, error) {
	r := &Registry{providers: make(map[delivery.SourcePlatform]Provider)}
	for _, value := range values {
		if value == nil || value.Platform() == delivery.SourceLocal || strings.TrimSpace(value.AppKey()) == "" || r.providers[value.Platform()] != nil {
			return nil, ErrInvalidProvider
		}
		r.providers[value.Platform()] = value
	}
	return r, nil
}

func (r *Registry) Provider(platform delivery.SourcePlatform) (Provider, bool) {
	value, ok := r.providers[platform]
	return value, ok && value.Configured()
}

func (r *Registry) ConfiguredPlatforms() []delivery.SourcePlatform {
	result := make([]delivery.SourcePlatform, 0, len(r.providers))
	for _, platform := range []delivery.SourcePlatform{delivery.SourceTaobao, delivery.SourceJD, delivery.SourceMeituan} {
		if value := r.providers[platform]; value != nil && value.Configured() { result = append(result, platform) }
	}
	return result
}
```

- [ ] **Step 5: 运行 registry 测试并提交**

Run: `cd backend; go test ./internal/module/deliveryplatform -run TestRegistry -count=1`

Expected: PASS。

```powershell
git add backend/internal/module/deliveryplatform/types.go backend/internal/module/deliveryplatform/registry.go backend/internal/module/deliveryplatform/registry_test.go
git commit -m "功能：定义外卖平台 Provider 契约"
```

### Task 3: 实现 AES-GCM Token 密文

**Files:**
- Create: `backend/internal/module/deliveryplatform/cipher.go`
- Create: `backend/internal/module/deliveryplatform/cipher_test.go`

- [ ] **Step 1: 写认证加密失败测试**

```go
func TestAESGCMTokenCipherBindsCiphertextToBindingAndField(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	cipher, err := NewAESGCMTokenCipher(3, key)
	if err != nil { t.Fatal(err) }
	sealed, err := cipher.Seal("meituan", "binding-A", "access", "token-value")
	if err != nil { t.Fatal(err) }
	plain, err := cipher.Open(3, "meituan", "binding-A", "access", sealed)
	if err != nil || plain != "token-value" { t.Fatalf("plain=%q err=%v", plain, err) }
	if _, err := cipher.Open(3, "meituan", "binding-B", "access", sealed); !errors.Is(err, ErrCredentialIntegrity) { t.Fatal(err) }
	sealed[len(sealed)-1] ^= 1
	if _, err := cipher.Open(3, "meituan", "binding-A", "access", sealed); !errors.Is(err, ErrCredentialIntegrity) { t.Fatal(err) }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend; go test ./internal/module/deliveryplatform -run TestAESGCM -count=1`

Expected: FAIL，cipher 构造函数未定义。

- [ ] **Step 3: 实现固定版本 AES-256-GCM**

```go
type AESGCMTokenCipher struct { version uint16; aead cipher.AEAD }

func NewAESGCMTokenCipher(version uint16, key []byte) (*AESGCMTokenCipher, error) {
	if version == 0 || len(key) != 32 { return nil, ErrInvalidCredentialKey }
	block, err := aes.NewCipher(key)
	if err != nil { return nil, err }
	aead, err := cipher.NewGCM(block)
	if err != nil { return nil, err }
	return &AESGCMTokenCipher{version: version, aead: aead}, nil
}

func tokenAAD(platform, bindingKey, field string) []byte {
	return []byte(platform + "\x00" + bindingKey + "\x00" + field)
}

func (c *AESGCMTokenCipher) Seal(platform, bindingKey, field, plain string) ([]byte, error) {
	if plain == "" { return nil, ErrInvalidCredential }
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil { return nil, err }
	return c.aead.Seal(nonce, nonce, []byte(plain), tokenAAD(platform, bindingKey, field)), nil
}

func (c *AESGCMTokenCipher) Open(version uint16, platform, bindingKey, field string, sealed []byte) (string, error) {
	if version != c.version || len(sealed) <= c.aead.NonceSize() { return "", ErrCredentialIntegrity }
	nonce := sealed[:c.aead.NonceSize()]
	plain, err := c.aead.Open(nil, nonce, sealed[c.aead.NonceSize():], tokenAAD(platform, bindingKey, field))
	if err != nil { return "", ErrCredentialIntegrity }
	return string(plain), nil
}
```

- [ ] **Step 4: 运行加密测试并提交**

Run: `cd backend; go test ./internal/module/deliveryplatform -run TestAESGCM -count=1`

Expected: PASS。

```powershell
git add backend/internal/module/deliveryplatform/cipher.go backend/internal/module/deliveryplatform/cipher_test.go
git commit -m "安全：加密存储外卖平台门店 Token"
```

### Task 4: 实现授权会话、绑定和审计 repository

**Files:**
- Create: `backend/internal/module/deliveryplatform/repository.go`
- Create: `backend/internal/module/deliveryplatform/repository_mysql_test.go`

- [ ] **Step 1: 写并发/隔离失败测试**

```go
func TestRepositoryConsumesStateOnceAndScopesBindings(t *testing.T) {
	repo, db := openPlatformTestDB(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	session := AuthorizationSession{StateHash: sha256.Sum256([]byte("state")), UserID: 11, MerchantUserID: 21, Platform: delivery.SourceMeituan, ProviderAppKey: "mt-main", RedirectID: "merchant", Status: AuthCreated, ExpiresAt: now.Add(time.Minute), RowVersion: 1, CreatedAt: now, UpdatedAt: now}
	if err := repo.CreateAuthorizationSession(context.Background(), &session); err != nil { t.Fatal(err) }
	claimed, err := repo.ClaimAuthorizationSession(context.Background(), session.StateHash, now, time.Minute)
	if err != nil { t.Fatal(err) }
	if _, err := repo.ClaimAuthorizationSession(context.Background(), session.StateHash, now, time.Minute); !errors.Is(err, ErrAuthorizationState) { t.Fatal(err) }
	if err := repo.CompleteAuthorization(context.Background(), claimed, testGrant(), testShop(), testTokens(), 1, now); err != nil { t.Fatal(err) }
	if got, err := repo.ListMerchantBindings(context.Background(), 22); err != nil || len(got) != 0 { t.Fatalf("cross merchant leak: %v %v", got, err) }
	if got, err := repo.ListMerchantBindings(context.Background(), 21); err != nil || len(got) != 1 { t.Fatalf("binding=%v err=%v", got, err) }
	if count := tableCount(t, db, "delivery_platform_binding_audits"); count != 1 { t.Fatalf("audit count=%d", count) }
}
```

- [ ] **Step 2: 运行 isolated MySQL 测试确认失败**

Run: `cd backend; $env:DELIVERY_PLATFORM_TEST_MYSQL_DSN='<isolated-test-dsn>'; go test ./internal/module/deliveryplatform -run TestRepository -count=1`

Expected: FAIL，repository 方法未定义。只能使用专用临时库；未提供时测试必须 SKIP，不得连接 `campus_circle`。

- [ ] **Step 3: 实现 repository 接口与事务入口**

```go
type Repository interface {
	CreateAuthorizationSession(context.Context, *AuthorizationSession) error
	ClaimAuthorizationSession(context.Context, [32]byte, time.Time, time.Duration) (AuthorizationSession, error)
	ReleaseAuthorizationSession(context.Context, AuthorizationSession, string, time.Time) error
	CompleteAuthorization(context.Context, AuthorizationSession, AuthorizationGrant, Shop, TokenSet, uint16, time.Time) (Binding, error)
	ListMerchantBindings(context.Context, uint64) ([]Binding, error)
	FindMerchantBinding(context.Context, uint64, string) (BindingRecord, error)
	FindActiveBinding(context.Context, string) (BindingRecord, error)
	UpdatePolicy(context.Context, uint64, uint64, bool, bool, uint64, uint64, string, time.Time) (Binding, error)
	Revoke(context.Context, uint64, uint64, uint64, uint64, string, time.Time) (Binding, error)
}

func (r *GormRepository) ClaimAuthorizationSession(ctx context.Context, hash [32]byte, now time.Time, lease time.Duration) (AuthorizationSession, error) {
	var value AuthorizationSession
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("state_hash = ?", hash[:]).First(&value).Error; err != nil { return ErrAuthorizationState }
		if value.Status != AuthCreated || !value.ExpiresAt.After(now) { return ErrAuthorizationState }
		token, err := delivery.NewULID(); if err != nil { return err }
		result := tx.Model(&value).Where("row_version = ?", value.RowVersion).Updates(map[string]any{"status": AuthExchanging, "lease_token": token, "lease_expires_at": now.Add(lease), "row_version": gorm.Expr("row_version + 1"), "updated_at": now})
		if result.Error != nil { return result.Error }; if result.RowsAffected != 1 { return ErrConcurrentUpdate }
		value.Status, value.LeaseToken, value.RowVersion = AuthExchanging, &token, value.RowVersion+1
		return nil
	})
	return value, err
}
```

`CompleteAuthorization` 必须在一个事务内：锁定会话、检查租约和版本、生成 32 随机字节的 base64url `binding_key`、加密 Token、upsert 同平台/应用/门店绑定、写审计、把会话置为 `consumed`。唯一键冲突只能复用同一 `merchant_user_id`，跨商家冲突返回 `ErrShopAlreadyBound`。

- [ ] **Step 4: 实现绑定策略 CAS**

```go
func validatePolicy(autoImport, autoDispatch bool) error {
	if autoDispatch && !autoImport { return ErrInvalidPolicy }
	return nil
}

func (r *GormRepository) UpdatePolicy(ctx context.Context, merchantID, connectionID uint64, autoImport, autoDispatch bool, expectedVersion, actor uint64, reason string, now time.Time) (Binding, error) {
	if merchantID == 0 || connectionID == 0 || actor == 0 || expectedVersion == 0 || validatePolicy(autoImport, autoDispatch) != nil { return Binding{}, ErrInvalidPolicy }
	var result Binding
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row BindingRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND merchant_user_id = ?", connectionID, merchantID).First(&row).Error; err != nil { return ErrBindingNotFound }
		if row.PolicyVersion != expectedVersion || row.Status == BindingRevoked { return ErrConcurrentUpdate }
		before := row.PolicyVersion
		update := tx.Model(&row).Updates(map[string]any{"auto_import": autoImport, "auto_dispatch": autoDispatch, "policy_version": gorm.Expr("policy_version + 1"), "row_version": gorm.Expr("row_version + 1"), "updated_at": now})
		if update.Error != nil { return update.Error }; if update.RowsAffected != 1 { return ErrConcurrentUpdate }
		if err := tx.Create(&BindingAudit{BindingID: row.ID, ActorUserID: actor, Action: "policy_updated", BeforeVersion: before, AfterVersion: before + 1, Reason: strings.TrimSpace(reason), CreatedAt: now}).Error; err != nil { return err }
		row.AutoImport, row.AutoDispatch, row.PolicyVersion = autoImport, autoDispatch, before+1
		result = row.View()
		return nil
	})
	return result, err
}
```

- [ ] **Step 5: 运行 MySQL 测试并提交**

Run: `cd backend; go test ./internal/module/deliveryplatform -run TestRepository -count=1`

Expected: PASS 或在无专用 DSN 时明确 SKIP。

```powershell
git add backend/internal/module/deliveryplatform/repository.go backend/internal/module/deliveryplatform/repository_mysql_test.go
git commit -m "功能：持久化平台授权与门店绑定"
```

### Task 5: 实现授权与绑定 service

**Files:**
- Create: `backend/internal/module/deliveryplatform/service.go`
- Create: `backend/internal/module/deliveryplatform/service_test.go`

- [ ] **Step 1: 写授权状态机失败测试**

```go
func TestServiceAuthorizationDoesNotTrustClientShop(t *testing.T) {
	repo := newMemoryRepository()
	provider := &stubProvider{platform: delivery.SourceMeituan, configured: true, shops: []Shop{{SourceShopID: "official-shop", SourceShopName: "官方门店"}}}
	registry, _ := NewRegistry(provider)
	service := NewService(repo, registry, testCipher(), ServiceConfig{Redirects: map[string]string{"merchant": "https://api.example.com/api/v1/delivery/platform-auth/meituan/callback"}})
	begin, err := service.BeginAuthorization(context.Background(), 7, 9, delivery.SourceMeituan, "merchant")
	if err != nil || begin.AuthorizationURL == "" { t.Fatal(err) }
	result, err := service.CompleteAuthorization(context.Background(), delivery.SourceMeituan, "code", begin.State, "merchant")
	if err != nil || len(result.Shops) != 1 || result.Shops[0].SourceShopID != "official-shop" { t.Fatalf("%+v %v", result, err) }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend; go test ./internal/module/deliveryplatform -run TestServiceAuthorization -count=1`

Expected: FAIL，Service 未定义。

- [ ] **Step 3: 实现 begin/complete/select-shop 三阶段服务**

```go
type BeginAuthorizationResult struct { State, AuthorizationURL string; ExpiresAt time.Time }
type CompleteAuthorizationResult struct { SessionID uint64; Shops []Shop }

func (s *Service) BeginAuthorization(ctx context.Context, userID, merchantID uint64, platform delivery.SourcePlatform, redirectID string) (BeginAuthorizationResult, error) {
	provider, ok := s.registry.Provider(platform)
	redirectURI := s.redirects[redirectID]
	if userID == 0 || merchantID == 0 || !ok || redirectURI == "" { return BeginAuthorizationResult{}, ErrProviderUnavailable }
	raw := make([]byte, 32); if _, err := io.ReadFull(rand.Reader, raw); err != nil { return BeginAuthorizationResult{}, err }
	state := base64.RawURLEncoding.EncodeToString(raw); hash := sha256.Sum256([]byte(state)); now := s.now().UTC()
	session := AuthorizationSession{StateHash: hash, UserID: userID, MerchantUserID: merchantID, Platform: platform, ProviderAppKey: provider.AppKey(), RedirectID: redirectID, Status: AuthCreated, ExpiresAt: now.Add(10*time.Minute), RowVersion: 1, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateAuthorizationSession(ctx, &session); err != nil { return BeginAuthorizationResult{}, err }
	url, err := provider.BuildAuthorizationURL(AuthorizationRequest{State: state, RedirectURI: redirectURI, ProviderAppKey: provider.AppKey()})
	if err != nil { return BeginAuthorizationResult{}, err }
	return BeginAuthorizationResult{State: state, AuthorizationURL: url, ExpiresAt: session.ExpiresAt}, nil
}
```

`CompleteAuthorization` 只交换授权码并列出官方门店，把 grant 暂存于短时服务端会话；`SelectShop` 接受 `sessionID + sourceShopID`，必须从此前官方 `ListShops` 结果中选择，再调用 repository 的单事务 `CompleteAuthorization`。不得接受客户端提交 Token、商家 ID、店名或任意 redirect URI。

- [ ] **Step 4: 实现可信 binding resolver**

```go
type Resolver struct { repo Repository }

func (r Resolver) ResolveExternalBinding(ctx context.Context, key string) (delivery.ExternalShopBinding, error) {
	record, err := r.repo.FindActiveBinding(ctx, strings.TrimSpace(key))
	if err != nil || record.Status != BindingActive { return delivery.ExternalShopBinding{}, delivery.ErrExternalBinding }
	return delivery.ExternalShopBinding{MerchantUserID: record.MerchantUserID, Platform: record.Platform, SourceShopID: record.SourceShopID}, nil
}
```

- [ ] **Step 5: 运行 service 测试并提交**

Run: `cd backend; go test ./internal/module/deliveryplatform -run 'TestService|TestResolver' -count=1`

Expected: PASS。

```powershell
git add backend/internal/module/deliveryplatform/service.go backend/internal/module/deliveryplatform/service_test.go
git commit -m "功能：完成平台授权和门店绑定状态机"
```

### Task 6: 暴露商家与后台 HTTP 合同

**Files:**
- Create: `backend/internal/module/admin/deliveryplatform/handler.go`
- Create: `backend/internal/module/admin/deliveryplatform/handler_test.go`

- [ ] **Step 1: 写 HTTP 失败测试**

```go
func TestHandlerScopesMerchantAndRejectsUnknownFields(t *testing.T) {
	router, service := testRouter(77)
	assertStatus(t, router, http.MethodGet, "/merchant/platform-connections", "", http.StatusOK)
	assertStatus(t, router, http.MethodPost, "/merchant/platform-connections/meituan/authorize", `{"redirectId":"merchant","merchantUserId":999}`, http.StatusBadRequest)
	assertStatus(t, router, http.MethodPatch, "/merchant/platform-connections/17/policy", `{"autoImport":false,"autoDispatch":true,"expectedPolicyVersion":1,"reason":"test"}`, http.StatusBadRequest)
	if service.lastMerchantID != 77 { t.Fatalf("merchant=%d", service.lastMerchantID) }
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd backend; go test ./internal/module/admin/deliveryplatform -count=1`

Expected: FAIL，Handler 未定义。

- [ ] **Step 3: 实现严格 JSON 和路由**

```go
func (h *Handler) RegisterMerchant(group *gin.RouterGroup) {
	group.GET("/platform-connections", h.list)
	group.POST("/platform-connections/:platform/authorize", h.authorize)
	group.POST("/platform-connections/:platform/select-shop", h.selectShop)
	group.PATCH("/platform-connections/:connectionId/policy", h.updatePolicy)
	group.POST("/platform-connections/:connectionId/revoke", h.revoke)
}

func decodeStrict(c *gin.Context, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32*1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil { return err }
	if decoder.Decode(&struct{}{}) != io.EOF { return errors.New("trailing data") }
	return nil
}

func (h *Handler) updatePolicy(c *gin.Context) {
	var req struct { AutoImport bool `json:"autoImport"`; AutoDispatch bool `json:"autoDispatch"`; ExpectedPolicyVersion uint64 `json:"expectedPolicyVersion"`; Reason string `json:"reason"` }
	if decodeStrict(c, &req) != nil { response.Fail(c, http.StatusBadRequest, "invalid request"); return }
	connectionID, err := strconv.ParseUint(c.Param("connectionId"), 10, 64)
	if err != nil || connectionID == 0 { response.Fail(c, http.StatusBadRequest, "invalid connection"); return }
	value, err := h.service.UpdatePolicy(c, h.currentUserID(c), connectionID, req.AutoImport, req.AutoDispatch, req.ExpectedPolicyVersion, req.Reason)
	h.write(c, value, err)
}
```

OAuth callback注册在无需会话 cookie 的固定路由 `/delivery/platform-auth/:platform/callback`，只接受 `code/state` 和平台错误字段；state 由 service 验证。成功只返回无脚本、无 Token 的 UTF-8 HTML：“授权已确认，请返回小程序选择门店”。后台路由只返回脱敏状态、时间和错误分类。

- [ ] **Step 4: 运行 handler 测试并提交**

Run: `cd backend; go test ./internal/module/admin/deliveryplatform -count=1`

Expected: PASS，包含 401/403/404/409/503 映射。

```powershell
git add backend/internal/module/admin/deliveryplatform/handler.go backend/internal/module/admin/deliveryplatform/handler_test.go
git commit -m "接口：开放外卖平台授权与策略管理"
```

### Task 7: 配置与组合根保持 fail closed

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/delivery_capabilities_test.go`

- [ ] **Step 1: 写配置失败测试**

```go
func TestExternalOrderConfigurationRequiresCompleteTokenKey(t *testing.T) {
	base := validDevConfig()
	base.EnableExternalOrders = true
	if err := base.Validate(); err == nil { t.Fatal("missing token key accepted") }
	base.DeliveryPlatformTokenKeyBase64 = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 31))
	if err := base.Validate(); err == nil { t.Fatal("31-byte token key accepted") }
	base.DeliveryPlatformTokenKeyBase64 = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	if err := base.Validate(); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 2: 添加显式配置字段和校验**

```go
type Config struct {
	// existing fields...
	EnableExternalOrders bool
	DeliveryPlatformTokenKeyBase64 string
	DeliveryPlatformTokenKeyVersion uint16
}

func (c Config) DeliveryPlatformTokenKey() ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(c.DeliveryPlatformTokenKeyBase64))
	if err != nil || len(decoded) != 32 || c.DeliveryPlatformTokenKeyVersion == 0 { return nil, errors.New("external order token key must be 32 bytes with a non-zero version") }
	return decoded, nil
}
```

`Load()` 读取 `ENABLE_EXTERNAL_ORDERS`、`DELIVERY_PLATFORM_TOKEN_KEY_B64`、`DELIVERY_PLATFORM_TOKEN_KEY_VERSION`。禁用时不要求密钥；启用时密钥不完整直接启动失败。测试不得打印解码后的密钥。

- [ ] **Step 3: 组合空 registry 但不宣告已接通**

```go
registry, err := deliveryplatform.NewRegistry()
if err != nil { return nil, fmt.Errorf("configure delivery platform registry: %w", err) }
if cfg.EnableExternalOrders {
	key, err := cfg.DeliveryPlatformTokenKey(); if err != nil { return nil, err }
	cipher, err := deliveryplatform.NewAESGCMTokenCipher(cfg.DeliveryPlatformTokenKeyVersion, key); if err != nil { return nil, err }
	runtime.platformService = deliveryplatform.NewService(deliveryplatform.NewRepository(db, cipher), registry, cipher, platformServiceConfig(cfg))
}
```

`runtime.externalIntake` 在本计划仍保持 `nil`。`/delivery/capabilities` 返回兼容字段 `externalOrders:false`，另增加 `externalPlatforms:[]` 和 `externalPlatformConfigured:false`。只有后续导入 worker 和至少一个已配置 Provider 同时存在时才允许 `externalOrders:true`。

- [ ] **Step 4: 运行配置和能力测试**

Run: `cd backend; go test ./internal/config ./cmd/server -run 'TestExternalOrderConfiguration|TestDeliveryCapabilities' -count=1`

Expected: PASS；空 registry 不被报告为已接通。

- [ ] **Step 5: 提交组合根**

```powershell
git add backend/internal/config/config.go backend/internal/config/config_test.go backend/cmd/server/main.go backend/cmd/server/delivery_capabilities_test.go
git commit -m "配置：以关闭状态装配外卖平台授权底座"
```

### Task 8: 让商家能力接口读取真实绑定

**Files:**
- Modify: `backend/internal/module/admin/deliverymerchant/handler.go`
- Modify: `backend/internal/module/admin/deliverymerchant/types.go`
- Modify: `backend/internal/module/admin/deliverymerchant/order_source_test.go`
- Create: `apps/miniapp/src/services/delivery-platform.ts`

- [ ] **Step 1: 写 order-intake 真实状态测试**

```go
type connectionReaderStub struct { values []deliveryplatform.Binding }
func (s connectionReaderStub) ListMerchantBindings(context.Context, uint64) ([]deliveryplatform.Binding, error) { return s.values, nil }

func TestOrderIntakeReportsActiveBindingWithoutEnablingDispatch(t *testing.T) {
	h := NewHandler(nil, func(*gin.Context) uint64 { return 7 }, false, connectionReaderStub{values: []deliveryplatform.Binding{{Platform: delivery.SourceMeituan, Status: deliveryplatform.BindingActive, AutoImport: false, AutoDispatch: false}}})
	// GET response: meituan.connected=true, externalAvailable=false.
}
```

- [ ] **Step 2: 修改兼容 DTO**

```go
type OrderIntakePlatformView struct {
	Platform delivery.SourcePlatform `json:"platform"`
	Label string `json:"label"`
	Configured bool `json:"configured"`
	Connected bool `json:"connected"`
	Status string `json:"status"`
	AutoImport bool `json:"autoImport"`
	AutoDispatch bool `json:"autoDispatch"`
}
```

`externalAvailable` 只在至少一个绑定 active 且后端导入 worker 可用时为 true；仅完成授权仍为 false。读取失败返回 503，不把未知状态降级成“未连接”。

- [ ] **Step 3: 添加前端稳定类型与调用**

```ts
import { request } from './api'
import type { SourcePlatform } from '../utils/delivery-order-source'

export interface DeliveryPlatformConnection {
  connectionId: number
  platform: Exclude<SourcePlatform, 'local'>
  shopName: string
  status: 'active' | 'refresh_required' | 'reauthorization_required' | 'suspended' | 'revoked'
  autoImport: boolean
  autoDispatch: boolean
  policyVersion: number
}

export const deliveryPlatformAPI = {
  list: () => request<DeliveryPlatformConnection[]>('/delivery/merchant/platform-connections', { url: '', timeout: 10000 }),
  beginAuthorization: (platform: DeliveryPlatformConnection['platform']) => request<{ authorizationUrl: string; expiresAt: string }>(`/delivery/merchant/platform-connections/${platform}/authorize`, { url: '', method: 'POST', data: { redirectId: 'merchant' }, timeout: 10000 }),
  updatePolicy: (value: DeliveryPlatformConnection, autoImport: boolean, autoDispatch: boolean, reason: string) => request<DeliveryPlatformConnection>(`/delivery/merchant/platform-connections/${value.connectionId}/policy`, { url: '', method: 'PATCH', data: { autoImport, autoDispatch, expectedPolicyVersion: value.policyVersion, reason }, timeout: 10000 })
}
```

- [ ] **Step 4: 运行 Go 与前端类型检查**

Run: `cd backend; go test ./internal/module/admin/deliverymerchant -run TestOrderIntake -count=1`

Expected: PASS。

Run: `npm --workspace apps/miniapp run typecheck`

Expected: PASS。

- [ ] **Step 5: 提交能力合同**

```powershell
git add backend/internal/module/admin/deliverymerchant/handler.go backend/internal/module/admin/deliverymerchant/types.go backend/internal/module/admin/deliverymerchant/order_source_test.go apps/miniapp/src/services/delivery-platform.ts
git commit -m "接口：返回商家平台连接真实状态"
```

### Task 9: 完整验证与边界审查

**Files:**
- Modify if needed: `scripts/check-real-wechat-secret-hygiene.cjs`
- Modify if needed: `scripts/check-boundaries.cjs`

- [ ] **Step 1: 增加秘密卫生检查词表**

```js
const forbiddenExternalCredentials = [
  /MEITUAN_SIGN_KEY\s*=\s*\S+/,
  /TAOBAO_FLASH_APP_SECRET\s*=\s*\S+/,
  /JD_MERCHANT_APP_SECRET\s*=\s*\S+/,
  /access_token_ciphertext[^\n]*['"][A-Za-z0-9_-]{20,}['"]/
]
```

检查器只扫描被 Git 跟踪的源码/文档，不读取真实环境变量值。

- [ ] **Step 2: 运行聚焦测试**

```powershell
cd backend
go test ./migrations ./internal/module/deliveryplatform ./internal/module/admin/deliveryplatform ./internal/module/admin/deliverymerchant ./cmd/server -count=1
```

Expected: PASS；独立 MySQL 未配置的测试明确 SKIP。

- [ ] **Step 3: 运行全量静态与构建验收**

```powershell
cd "C:\Users\33141\Documents\New project"
npm --workspace apps/miniapp run typecheck
npm run check:boundaries
npm run check:real-wechat-secret-hygiene
npm run test:backend
npm run build:miniapp
git diff --check
```

Expected: 全部 exit 0；`build:miniapp` 只允许既有体积/顺序警告。

- [ ] **Step 4: 最终提交**

```powershell
git add scripts/check-real-wechat-secret-hygiene.cjs scripts/check-boundaries.cjs
git commit -m "测试：约束平台凭据与领域边界"
```

如果两个脚本无需修改，跳过该 `git add/commit`，不得制造空提交。

## 完成标准

- 数据库可安全重复应用 migration 048，且没有明文凭据列。
- 授权 state 单次、短时、绑定用户/商家；Token 使用 AES-GCM 且绑定 AAD。
- 门店唯一归属、策略 CAS、撤销和审计均有测试。
- 没有 Provider 时所有真实订单能力保持关闭。
- 原 `external_takeout_merchants` 目录与配送平台授权完全分离。
- 未部署、未迁移生产库、未填入真实密钥、未声称三平台已接通。
