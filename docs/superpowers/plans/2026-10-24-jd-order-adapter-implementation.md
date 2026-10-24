# 京东新商家开放平台订单适配器 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 面向 `open.jd.com` 新商家开放平台实现商家授权、消息验证、订单回查和配送字段归一化，并禁止新代码误用即将下线的旧秒送协议。

**Architecture:** `deliveryprovider/jd` 只实现新平台已获权合同；旧 `opendj.jd.com` 仅作为迁移检测对象，不提供默认 adapter。Provider 先持久接收经验证的订单提示，再异步查询权威订单；新旧平台不得同时导入同一门店。

**Tech Stack:** Go 1.25、标准库 HTTP/crypto、现有 `deliveryplatform.Provider`、`httptest`、仓库边界检查脚本。

---

## 前置条件与文件结构

依赖：`docs/superpowers/plans/2026-10-24-delivery-platform-foundation-implementation.md`。

Create:

- `backend/internal/module/deliveryprovider/jd/config.go`
- `backend/internal/module/deliveryprovider/jd/auth.go`
- `backend/internal/module/deliveryprovider/jd/signer.go`
- `backend/internal/module/deliveryprovider/jd/message.go`
- `backend/internal/module/deliveryprovider/jd/client.go`
- `backend/internal/module/deliveryprovider/jd/order.go`
- `backend/internal/module/deliveryprovider/jd/provider.go`
- `backend/internal/module/deliveryprovider/jd/*_test.go`
- `backend/internal/module/deliveryprovider/jd/testdata/*.json`
- `scripts/check-jd-new-platform.cjs`

Modify:

- `backend/internal/config/config.go`、`config_test.go`
- `backend/cmd/server/main.go`、`delivery_capabilities_test.go`
- `package.json`

### Task 1: 冻结新平台账号内合同并拒绝旧协议替代

**Files:**
- Create: `backend/internal/module/deliveryprovider/jd/testdata/contract_manifest.json`
- Create: `backend/internal/module/deliveryprovider/jd/contract_manifest_test.go`

- [ ] **Step 1: 在 `open.jd.com` 完成应用/授权资料获取**

确定本项目属于商家自研还是多商家开发者/服务商；取得新平台的应用创建、OAuth、scope、订单消息、订单查询、回调应答、签名、沙箱、上线、限频和错误码文档。记录文档标题、版本和获取日期，不记录 AppSecret、Token、商家号、门店号或真实订单。

若只能看到旧 `opendj.jd.com` 文档，停止此计划。旧平台的 `app_key/sign/timestamp` 形态不能作为新平台实现依据。

- [ ] **Step 2: 保存新平台非秘密清单与固定向量**

```json
{
  "provider": "jd",
  "platform": "open.jd.com",
  "contractVersion": "新商家开放平台账号内当前版本",
  "retrievedAt": "ISO-8601 日期时间",
  "oauthFixture": "official_oauth_response.json",
  "signatureFixture": "official_api_signature.json",
  "messageFixture": "official_order_message.json",
  "orderQueryFixture": "official_order_query.json"
}
```

上方仅定义 JSON schema，不能原样提交；`contractVersion` 和 `retrievedAt` 必须替换为 `open.jd.com` 账号内当前文档的真实非秘密值。无法取得时停在本 Task，不执行 Task 2 及后续任务。

- [ ] **Step 3: 添加新平台闸门测试**

```go
func TestContractManifestPinsNewMerchantPlatform(t *testing.T) {
	raw, err := os.ReadFile("testdata/contract_manifest.json"); if err != nil { t.Fatal(err) }
	if bytes.Contains(bytes.ToLower(raw), []byte("opendj.jd.com")) { t.Fatal("legacy JD contract cannot activate new provider") }
	var value struct { Provider, Platform, ContractVersion, RetrievedAt, OAuthFixture, SignatureFixture, MessageFixture, OrderQueryFixture string }
	if json.Unmarshal(raw, &value) != nil || value.Provider != "jd" || value.Platform != "open.jd.com" || value.ContractVersion == "" { t.Fatal("invalid new JD contract") }
	for _, file := range []string{value.OAuthFixture, value.SignatureFixture, value.MessageFixture, value.OrderQueryFixture} { if _, err := os.Stat(filepath.Join("testdata", file)); err != nil { t.Fatalf("missing fixture %s", file) } }
}
```

- [ ] **Step 4: 运行并提交闸门**

Run: `cd backend; go test ./internal/module/deliveryprovider/jd -run TestContractManifest -count=1`

Expected: PASS；缺新平台文档/夹具时必须 FAIL。

```powershell
git add backend/internal/module/deliveryprovider/jd/testdata backend/internal/module/deliveryprovider/jd/contract_manifest_test.go
git commit -m "文档：冻结京东新商家开放平台合同"
```

### Task 2: 配置中强制新平台主机和环境隔离

**Files:**
- Create: `backend/internal/module/deliveryprovider/jd/config.go`
- Create: `backend/internal/module/deliveryprovider/jd/config_test.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`

- [ ] **Step 1: 写旧主机与环境错配测试**

```go
func TestConfigRejectsLegacyHostAndUnknownEnvironment(t *testing.T) {
	valid := testConfig()
	if err := valid.Validate(); err != nil { t.Fatal(err) }
	legacy := valid; legacy.APIBaseURL = "https://opendj.jd.com/api"
	if err := legacy.Validate(); err == nil { t.Fatal("legacy host accepted") }
	unknown := valid; unknown.Environment = "staging"
	if err := unknown.Validate(); err == nil { t.Fatal("unknown environment accepted") }
}
```

- [ ] **Step 2: 实现配置**

```go
type Config struct {
	AppKey string
	AppSecret string
	APIBaseURL string
	AuthorizationBaseURL string
	RedirectURI string
	ContractVersion string
	Environment string
	Scopes []string
	HTTPClient *http.Client
}

func (c Config) Validate() error {
	if c.AppKey == "" || c.AppSecret == "" || c.ContractVersion == "" || (c.Environment != "sandbox" && c.Environment != "production") || len(c.Scopes) == 0 { return ErrNotConfigured }
	for _, raw := range []string{c.APIBaseURL, c.AuthorizationBaseURL, c.RedirectURI} {
		u, err := url.Parse(raw); if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || strings.EqualFold(u.Host, "opendj.jd.com") { return ErrNotConfigured }
	}
	return nil
}
```

根配置读取 `JD_MERCHANT_APP_KEY`、`JD_MERCHANT_APP_SECRET`、`JD_MERCHANT_API_BASE_URL`、`JD_MERCHANT_AUTH_BASE_URL`、`JD_MERCHANT_REDIRECT_URI`、`JD_MERCHANT_CONTRACT_VERSION`、`JD_MERCHANT_ENVIRONMENT`、`JD_MERCHANT_SCOPES`。sandbox 与 production 配置必须各自完整且不能混用 Token。

- [ ] **Step 3: 运行并提交配置**

Run: `cd backend; go test ./internal/config ./internal/module/deliveryprovider/jd -run 'Test.*Config' -count=1`

Expected: PASS。

```powershell
git add backend/internal/config/config.go backend/internal/config/config_test.go backend/internal/module/deliveryprovider/jd/config.go backend/internal/module/deliveryprovider/jd/config_test.go
git commit -m "配置：限定京东新商家开放平台环境"
```

### Task 3: 实现新平台 OAuth 和 Token 生命周期

**Files:**
- Create: `backend/internal/module/deliveryprovider/jd/auth.go`
- Create: `backend/internal/module/deliveryprovider/jd/auth_test.go`

- [ ] **Step 1: 写 state/redirect/scope 测试**

```go
func TestAuthorizationURLUsesPinnedNewPlatformContract(t *testing.T) {
	p, err := New(testConfig()); if err != nil { t.Fatal(err) }
	raw, err := p.BuildAuthorizationURL(deliveryplatform.AuthorizationRequest{State: "state-value", RedirectURI: testConfig().RedirectURI, ProviderAppKey: testConfig().AppKey})
	if err != nil { t.Fatal(err) }
	u, _ := url.Parse(raw)
	if u.Host != mustHost(testConfig().AuthorizationBaseURL) || u.Query().Get("state") != "state-value" { t.Fatalf("url=%s", raw) }
	if strings.Contains(raw, "opendj.jd.com") { t.Fatal("legacy authorization URL") }
}
```

- [ ] **Step 2: 实现合同固定的授权方法**

```go
func (p *Provider) ExchangeAuthorizationCode(ctx context.Context, callback deliveryplatform.AuthorizationCallback) (deliveryplatform.AuthorizationGrant, error) {
	if callback.RedirectURI != p.cfg.RedirectURI || callback.Code == "" { return deliveryplatform.AuthorizationGrant{}, ErrInvalidOAuthCallback }
	response, err := p.exchangeWithPinnedContract(ctx, callback.Code)
	if err != nil { return deliveryplatform.AuthorizationGrant{}, err }
	if response.Environment != p.cfg.Environment || response.AccessToken == "" || !containsAll(response.Scopes, p.cfg.Scopes) { return deliveryplatform.AuthorizationGrant{}, ErrAuthorizationScope }
	return deliveryplatform.AuthorizationGrant{SourceMerchantID: response.MerchantID, Tokens: response.TokenSet(), Scopes: response.Scopes}, nil
}
```

`exchangeWithPinnedContract`、`Refresh`、`Revoke`、`ListShops` 的字段、认证方式和时效单位逐项来自 Task 1 新平台文档。不能导入旧平台 Token DTO。HTTP 401/403 返回 `ProviderErrorAuthorization`，429 返回 `ProviderErrorRateLimited`，5xx/超时返回 `ProviderErrorRetryable`，明确不存在返回 `ProviderErrorPermanent`，未知业务码返回 `ProviderErrorContract`；只用 `deliveryplatform.NewProviderError` 构造，错误文本不包含响应原文。

- [ ] **Step 3: 覆盖 Token 轮换和环境隔离**

使用 `httptest.Server` 测试错误 state 由共享 service 拒绝、错误 redirect、scope 缺失、sandbox Token 响应到 production、刷新轮换、撤权、401、429、超时和 error 不含 Token。

- [ ] **Step 4: 运行并提交**

Run: `cd backend; go test ./internal/module/deliveryprovider/jd -run 'TestAuthorization|TestToken|TestShop' -count=1`

Expected: PASS。

```powershell
git add backend/internal/module/deliveryprovider/jd/auth.go backend/internal/module/deliveryprovider/jd/auth_test.go
git commit -m "功能：接入京东新平台商家授权"
```

### Task 4: 实现新平台 API 签名和消息验证

**Files:**
- Create: `backend/internal/module/deliveryprovider/jd/signer.go`
- Create: `backend/internal/module/deliveryprovider/jd/signer_test.go`
- Create: `backend/internal/module/deliveryprovider/jd/message.go`
- Create: `backend/internal/module/deliveryprovider/jd/message_test.go`

- [ ] **Step 1: 写官方固定向量测试并证明旧向量不兼容**

```go
func TestSignerMatchesNewPlatformVectorsAndRejectsLegacyVector(t *testing.T) {
	for _, vector := range loadNewPlatformSignatureVectors(t) {
		got, err := NewSigner(vector.PublicExampleSecret).Sign(vector.Parameters, []byte(vector.RawBody))
		if err != nil || got != vector.Expected { t.Fatalf("%s got=%q err=%v", vector.Name, got, err) }
	}
	legacy := loadLegacyControlVector(t)
	if got, _ := NewSigner(legacy.PublicExampleSecret).Sign(legacy.Parameters, []byte(legacy.RawBody)); got == legacy.Expected { t.Fatal("new signer unexpectedly implements legacy contract") }
}
```

- [ ] **Step 2: 实现新平台 signer/verifier**

```go
type Signer struct { secret []byte }

func (s *Signer) Verify(parameters map[string]string, rawBody []byte, received string) error {
	expected, err := s.Sign(parameters, rawBody); if err != nil { return err }
	if subtle.ConstantTimeCompare([]byte(expected), []byte(received)) != 1 { return delivery.ErrExternalSignature }
	return nil
}
```

`Sign` 的参数白名单、排序、编码、Secret 位置和算法只来自新平台固定向量。消息验证若使用另一合同，使用独立 `MessageVerifier`；不得调用旧 `opendj` helper。

- [ ] **Step 3: 测试篡改、时间窗、nonce、topic 和环境**

Run: `cd backend; go test ./internal/module/deliveryprovider/jd -run 'TestSigner|TestMessageVerifier' -count=1`

Expected: PASS。

- [ ] **Step 4: 提交签名合同**

```powershell
git add backend/internal/module/deliveryprovider/jd/signer.go backend/internal/module/deliveryprovider/jd/signer_test.go backend/internal/module/deliveryprovider/jd/message.go backend/internal/module/deliveryprovider/jd/message_test.go
git commit -m "安全：实现京东新平台签名与消息验证"
```

### Task 5: 实现回调应答和权威订单回查

**Files:**
- Modify: `backend/internal/module/deliveryprovider/jd/message.go`
- Create: `backend/internal/module/deliveryprovider/jd/client.go`
- Create: `backend/internal/module/deliveryprovider/jd/order.go`
- Create: `backend/internal/module/deliveryprovider/jd/order_test.go`

- [ ] **Step 1: 写回调最小提示测试**

```go
func TestVerifyCallbackReturnsPinnedAckWithoutCustomerData(t *testing.T) {
	p, envelope, binding := officialMessageFixture(t)
	hint, ack, err := p.VerifyCallback(context.Background(), envelope, binding)
	if err != nil { t.Fatal(err) }
	if hint.EventID == "" || hint.SourceOrderID == "" || hint.EventKind == "" { t.Fatalf("hint=%+v", hint) }
	if ack.Status != pinnedAckStatus() || len(ack.Body) == 0 { t.Fatalf("ack=%+v", ack) }
	if bytes.Contains(bytes.ToLower(hint.QueryHint), []byte("address")) { t.Fatal("customer data stored in query hint") }
}
```

- [ ] **Step 2: 实现消息解析与 Provider 应答**

校验新平台应用、环境、商家、门店、topic、消息 ID、发生时间和签名/解密。只保存查询定位符；成功应答逐字来自 Task 1 新平台文档，不能使用旧平台 `{"code":"0","msg":"success","data":""}`，除非新平台合同本身明确要求且固定夹具证明。

- [ ] **Step 3: 写订单查询与归一化测试**

```go
func TestFetchOrderNormalizesOnlyFulfillmentFields(t *testing.T) {
	p, server := providerWithOrderFixture(t); defer server.Close()
	binding := testBinding()
	result, err := p.FetchOrder(context.Background(), testTokens(), binding, deliveryplatform.CallbackHint{EventID: "event-1", SourceOrderID: "jd-order-00015", EventKind: "created"})
	if err != nil { t.Fatal(err) }
	if result.Event.Source.SourceOrderID == nil || *result.Event.Source.SourceOrderID != "jd-order-00015" || result.Event.Source.PickupCode != "0015" { t.Fatalf("event=%+v", result.Event) }
	if result.Event.Pickup.Address == "" || result.Event.Recipient.Address == "" { t.Fatal("missing route") }
	if result.SourceVersion == "" || result.SourceState == "" { t.Fatalf("missing version/state: %+v", result) }
}
```

- [ ] **Step 4: 实现显式状态映射**

```go
func normalize(raw officialOrder, binding deliveryplatform.Binding, eventID string) (delivery.ExternalOrderEvent, error) {
	kind, ok := newPlatformStateMap[raw.Status]
	if !ok { return delivery.ExternalOrderEvent{}, deliveryplatform.NewProviderError(deliveryplatform.ProviderErrorContract, 0) }
	shop, orderID := binding.SourceShopID, raw.OrderID
	return delivery.NormalizeExternalOrderEvent(delivery.SourceJD, delivery.ExternalShopBinding{MerchantUserID: binding.MerchantUserID, Platform: delivery.SourceJD, SourceShopID: shop}, delivery.ExternalOrderEvent{
		EventID: eventID, EventType: kind, OccurredAt: raw.UpdatedAt,
		Source: delivery.SourceMetadata{SourcePlatform: delivery.SourceJD, SourceShopID: &shop, SourceOrderID: &orderID, PickupCode: raw.PickupCode},
		Pickup: delivery.Contact{Name: raw.ShopName, Phone: raw.ShopPhone, Address: raw.ShopAddress},
		Recipient: delivery.Contact{Name: raw.RecipientName, Phone: raw.RecipientPhone, Address: raw.RecipientAddress}, Remark: raw.DeliveryRemark,
	})
}
```

`officialOrder` 只使用新平台夹具字段；`OrderSnapshot.SourceVersion` 使用新平台官方可比较版本，只有更新时间时规范化为 UTC RFC3339Nano，`SourceState` 保留新平台状态枚举。测试完整订单号、前导零、门店错配、未知状态、取消先到、缺地址、隐私号、401/429/超时和 HTTP 200+业务失败；用 `errors.As` 核对共享错误分类，确保 callback worker 不靠字符串判断重试。

- [ ] **Step 5: 运行并提交**

Run: `cd backend; go test ./internal/module/deliveryprovider/jd -run 'TestVerifyCallback|TestFetchOrder|TestNormalize' -count=1`

Expected: PASS。

```powershell
git add backend/internal/module/deliveryprovider/jd/message.go backend/internal/module/deliveryprovider/jd/client.go backend/internal/module/deliveryprovider/jd/order.go backend/internal/module/deliveryprovider/jd/order_test.go
git commit -m "功能：回查并归一化京东新平台订单"
```

### Task 6: 添加旧平台静态禁止规则

**Files:**
- Create: `scripts/check-jd-new-platform.cjs`
- Modify: `package.json`

- [ ] **Step 1: 写静态检查**

```js
const fs = require('node:fs')
const path = require('node:path')

const root = path.resolve(__dirname, '..')
const targets = [
  path.join(root, 'backend', 'internal', 'module', 'deliveryprovider', 'jd'),
  path.join(root, 'backend', 'internal', 'config')
]
const forbidden = [/opendj\.jd\.com/i, /legacy.*sign/i, /old.*token/i]
const files = targets.flatMap(dir => fs.readdirSync(dir, { recursive: true }).filter(name => /\.(go|json)$/.test(name)).map(name => path.join(dir, name)))
for (const file of files) {
  const source = fs.readFileSync(file, 'utf8')
  if (file.endsWith('contract_manifest_test.go')) continue
  for (const pattern of forbidden) if (pattern.test(source)) throw new Error(`${path.relative(root, file)} contains ${pattern}`)
}
console.log('PASS 京东订单 Provider 仅使用新商家开放平台合同')
```

- [ ] **Step 2: 注册脚本并运行**

在 `package.json` scripts 加：

```json
"check:jd-new-platform": "node scripts/check-jd-new-platform.cjs"
```

Run: `npm run check:jd-new-platform`

Expected: PASS。

- [ ] **Step 3: 提交边界检查**

```powershell
git add scripts/check-jd-new-platform.cjs package.json
git commit -m "测试：禁止京东订单接入回退旧平台"
```

### Task 7: 注册 Provider，自动导入保持关闭

**Files:**
- Create: `backend/internal/module/deliveryprovider/jd/provider.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/delivery_capabilities_test.go`

- [ ] **Step 1: 实现 Provider 构造**

```go
type Provider struct { cfg Config; signer *Signer; client *http.Client }

func New(cfg Config) (*Provider, error) {
	if err := cfg.Validate(); err != nil { return nil, err }
	client := cfg.HTTPClient; if client == nil { client = secureHTTPClient(15 * time.Second) }
	return &Provider{cfg: cfg, signer: NewSigner(cfg.AppSecret), client: client}, nil
}
func (p *Provider) Platform() delivery.SourcePlatform { return delivery.SourceJD }
func (p *Provider) AppKey() string { return p.cfg.AppKey }
func (p *Provider) Configured() bool { return p != nil && p.cfg.Validate() == nil }
```

- [ ] **Step 2: 组合根只注册完整新平台配置**

空配置跳过；部分/旧主机配置启动失败。注册后 `externalPlatforms` 可出现 `jd`，但通用导入 worker 未完成时 `externalOrders` 仍为 false，绑定开关默认关闭。

- [ ] **Step 3: 运行并提交**

Run: `cd backend; go test ./internal/module/deliveryprovider/jd ./internal/module/deliveryplatform ./cmd/server -count=1`

Expected: PASS。

```powershell
git add backend/internal/module/deliveryprovider/jd/provider.go backend/cmd/server/main.go backend/cmd/server/delivery_capabilities_test.go
git commit -m "功能：注册京东新平台订单 Provider"
```

### Task 8: 最终验证

- [ ] **Step 1: 执行完整验收**

```powershell
cd "C:\Users\33141\Documents\New project\backend"
go test ./internal/module/deliveryprovider/jd ./internal/module/deliveryplatform ./cmd/server -count=1
go test ./...
cd ..
npm run check:jd-new-platform
npm run check:boundaries
npm run check:real-wechat-secret-hygiene
git diff --check
```

Expected: 全部 exit 0；真实平台测试未配置时明确 SKIP，不能访问生产。

- [ ] **Step 2: 分离代码、沙箱和生产状态**

汇报必须分别列出：新平台固定向量、OAuth/消息/查询自动化、沙箱、应用上线、生产商家授权、自动导入、自动发单。旧平台测试结果不计入新平台通过。

## 完成标准

- 仅使用 `open.jd.com` 新平台当前合同。
- OAuth、签名、消息应答和查询均有官方非秘密固定向量。
- 订单门店、环境、状态和版本严格校验。
- 旧平台 host/签名不能进入新 Provider。
- Provider 注册不自动启用导入或扣费，未部署生产。
