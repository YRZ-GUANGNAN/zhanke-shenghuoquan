# 淘宝闪购订单适配器 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 通过淘宝闪购服务商平台正式授权，实现 OAuth、门店绑定、订单消息验证、订单补拉/回查和配送字段归一化。

**Architecture:** `deliveryprovider/taobao` 独占淘宝闪购 API 与消息合同，内部仍使用兼容枚举 `taobao`。消息推送和增量拉取汇入同一平台事件键，完整订单必须经官方查询后交给通用导入 worker；Provider 不接触配送费和骑手池。

**Tech Stack:** Go 1.25、标准库 HTTP/crypto、现有 `deliveryplatform.Provider`、MySQL callback checkpoint、`httptest`。

---

## 前置条件与文件结构

依赖：`docs/superpowers/plans/2026-10-24-delivery-platform-foundation-implementation.md`。

Create:

- `backend/internal/module/deliveryprovider/taobao/config.go`
- `backend/internal/module/deliveryprovider/taobao/auth.go`
- `backend/internal/module/deliveryprovider/taobao/signer.go`
- `backend/internal/module/deliveryprovider/taobao/message.go`
- `backend/internal/module/deliveryprovider/taobao/client.go`
- `backend/internal/module/deliveryprovider/taobao/order.go`
- `backend/internal/module/deliveryprovider/taobao/checkpoint.go`
- `backend/internal/module/deliveryprovider/taobao/*_test.go`
- `backend/internal/module/deliveryprovider/taobao/testdata/*.json`

Modify:

- `backend/migrations/049_delivery_platform_checkpoints.sql`（固定为独立迁移；不得并入或改写 migration 048）。
- `backend/internal/config/config.go`、`config_test.go`
- `backend/cmd/server/main.go`、`delivery_capabilities_test.go`

### Task 1: 冻结服务商账号内 OAuth、消息和订单合同

**Files:**
- Create: `backend/internal/module/deliveryprovider/taobao/testdata/contract_manifest.json`
- Create: `backend/internal/module/deliveryprovider/taobao/contract_manifest_test.go`

- [ ] **Step 1: 确认应用身份和权限**

在 `open.shop.ele.me` 明确当前账号属于商家自研还是多商家服务商，并取得订单查询、订单消息、商家授权、门店查询和增量补拉的正式权限。记录当前文档标题、更新时间、scope、网关地址、OAuth 参数、Token 刷新规则、消息签名/加密、成功应答、限频和错误码。

如果多商家服务商资质、协议或应用审核未通过，停止此计划；不得用单商家应用承载其他商家，也不得从商家客户端抓 Token。

- [ ] **Step 2: 保存不含秘密的合同清单和官方固定向量**

```json
{
  "provider": "taobao",
  "displayName": "淘宝闪购",
  "contractVersion": "账号内当前订单合同版本",
  "retrievedAt": "ISO-8601 日期时间",
  "oauthVectorFile": "official_oauth_response.json",
  "messageSignatureVectorFile": "official_message_signature.json",
  "orderQueryFixture": "official_order_query.json",
  "incrementalPullFixture": "official_incremental_pull.json"
}
```

上方仅定义 JSON schema，不能原样提交；`contractVersion` 和 `retrievedAt` 必须替换为当前获权文档的真实非秘密值。无法取得时停在本 Task，不执行 Task 2 及后续任务。

固定向量只能使用官方公开样例或专门测试应用的脱敏值；不得保存正式 Key、Secret、Token、商家 ID、门店 ID、顾客信息或真实订单号。

- [ ] **Step 3: 添加合同闸门测试**

```go
func TestContractManifestRequiresAllOfficialFixtures(t *testing.T) {
	raw, err := os.ReadFile("testdata/contract_manifest.json"); if err != nil { t.Fatal(err) }
	var value struct { Provider, ContractVersion, RetrievedAt, OAuthVectorFile, MessageSignatureVectorFile, OrderQueryFixture, IncrementalPullFixture string }
	if json.Unmarshal(raw, &value) != nil || value.Provider != "taobao" || value.ContractVersion == "" || value.OAuthVectorFile == "" || value.MessageSignatureVectorFile == "" || value.OrderQueryFixture == "" || value.IncrementalPullFixture == "" { t.Fatal("incomplete Taobao Flash contract") }
	for _, name := range []string{value.OAuthVectorFile, value.MessageSignatureVectorFile, value.OrderQueryFixture, value.IncrementalPullFixture} {
		if _, err := os.Stat(filepath.Join("testdata", name)); err != nil { t.Fatalf("missing fixture %s", name) }
	}
}
```

- [ ] **Step 4: 运行并提交合同闸门**

Run: `cd backend; go test ./internal/module/deliveryprovider/taobao -run TestContractManifest -count=1`

Expected: PASS；合同/夹具不可见时必须 FAIL，不能用空 JSON 绕过。

```powershell
git add backend/internal/module/deliveryprovider/taobao/testdata backend/internal/module/deliveryprovider/taobao/contract_manifest_test.go
git commit -m "文档：冻结淘宝闪购订单接入合同"
```

### Task 2: 配置与 OAuth state/Token 合同

**Files:**
- Create: `backend/internal/module/deliveryprovider/taobao/config.go`
- Create: `backend/internal/module/deliveryprovider/taobao/auth.go`
- Create: `backend/internal/module/deliveryprovider/taobao/auth_test.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`

- [ ] **Step 1: 写配置和 OAuth 请求失败测试**

```go
func TestOAuthRequiresHTTPSFixedRedirectAndScopes(t *testing.T) {
	cfg := testConfig()
	provider, err := New(cfg); if err != nil { t.Fatal(err) }
	raw, err := provider.BuildAuthorizationURL(deliveryplatform.AuthorizationRequest{State: "state-1", RedirectURI: "https://api.example.com/api/v1/delivery/platform-auth/taobao/callback", ProviderAppKey: cfg.AppKey})
	if err != nil { t.Fatal(err) }
	u, _ := url.Parse(raw)
	if u.Scheme != "https" || u.Query().Get("state") != "state-1" || u.Query().Get("redirect_uri") == "" { t.Fatalf("url=%s", raw) }
	if _, err := provider.BuildAuthorizationURL(deliveryplatform.AuthorizationRequest{State: "state-2", RedirectURI: "https://evil.example/callback", ProviderAppKey: cfg.AppKey}); err == nil { t.Fatal("open redirect accepted") }
}
```

- [ ] **Step 2: 添加不可部分配置字段**

```go
type Config struct {
	AppKey string
	AppSecret string
	APIBaseURL string
	AuthorizationBaseURL string
	RedirectURI string
	ContractVersion string
	Scopes []string
	HTTPClient *http.Client
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.AppKey) == "" || strings.TrimSpace(c.AppSecret) == "" || strings.TrimSpace(c.ContractVersion) == "" || len(c.Scopes) == 0 { return ErrNotConfigured }
	for _, raw := range []string{c.APIBaseURL, c.AuthorizationBaseURL, c.RedirectURI} {
		u, err := url.Parse(raw); if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil { return ErrNotConfigured }
	}
	return nil
}
```

根配置读取 `TAOBAO_FLASH_APP_KEY`、`TAOBAO_FLASH_APP_SECRET`、`TAOBAO_FLASH_API_BASE_URL`、`TAOBAO_FLASH_AUTH_BASE_URL`、`TAOBAO_FLASH_REDIRECT_URI`、`TAOBAO_FLASH_CONTRACT_VERSION`、`TAOBAO_FLASH_SCOPES`。只有完整配置才注册 Provider；任何错误信息不得回显 Secret。

- [ ] **Step 3: 按冻结 OAuth 文档实现交换与刷新**

```go
func (p *Provider) ExchangeAuthorizationCode(ctx context.Context, callback deliveryplatform.AuthorizationCallback) (deliveryplatform.AuthorizationGrant, error) {
	if callback.RedirectURI != p.cfg.RedirectURI || strings.TrimSpace(callback.Code) == "" { return deliveryplatform.AuthorizationGrant{}, ErrInvalidOAuthCallback }
	request := tokenRequestFromPinnedContract(p.cfg, callback.Code)
	response, err := p.doForm(ctx, pinnedTokenPath, request)
	if err != nil { return deliveryplatform.AuthorizationGrant{}, err }
	return validateAuthorizationGrant(response)
}
```

`tokenRequestFromPinnedContract` 和 `validateAuthorizationGrant` 的字段名、认证方式、时效单位和刷新 Token 轮换语义必须逐项来自 Task 1 文档；不兼容字段返回合同错误。HTTP 200 但业务码失败不能生成 TokenSet。所有 HTTP/传输失败通过以下共享映射返回，不保留响应原文：

```go
func classifyProviderFailure(status int, transport error) error {
	if transport != nil {
		return deliveryplatform.NewProviderError(deliveryplatform.ProviderErrorRetryable, 0)
	}
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return deliveryplatform.NewProviderError(deliveryplatform.ProviderErrorAuthorization, 0)
	case status == http.StatusTooManyRequests:
		return deliveryplatform.NewProviderError(deliveryplatform.ProviderErrorRateLimited, 0)
	case status >= 500:
		return deliveryplatform.NewProviderError(deliveryplatform.ProviderErrorRetryable, 0)
	case status == http.StatusNotFound:
		return deliveryplatform.NewProviderError(deliveryplatform.ProviderErrorPermanent, 0)
	default:
		return deliveryplatform.NewProviderError(deliveryplatform.ProviderErrorContract, 0)
	}
}
```

- [ ] **Step 4: 用 `httptest.Server` 覆盖 Token 生命周期**

测试授权码单次、错误 redirect、scope 缺失、Token 到期、刷新轮换、旧 refresh token 失效、401、429、超时、超大 body 和 error 不含 Token。

- [ ] **Step 5: 运行并提交**

Run: `cd backend; go test ./internal/config ./internal/module/deliveryprovider/taobao -run 'TestOAuth|Test.*Config|TestToken' -count=1`

Expected: PASS。

```powershell
git add backend/internal/config/config.go backend/internal/config/config_test.go backend/internal/module/deliveryprovider/taobao/config.go backend/internal/module/deliveryprovider/taobao/auth.go backend/internal/module/deliveryprovider/taobao/auth_test.go
git commit -m "功能：接入淘宝闪购 OAuth 与 Token 生命周期"
```

### Task 3: 实现 API 签名和消息验签的独立合同

**Files:**
- Create: `backend/internal/module/deliveryprovider/taobao/signer.go`
- Create: `backend/internal/module/deliveryprovider/taobao/signer_test.go`
- Create: `backend/internal/module/deliveryprovider/taobao/message.go`
- Create: `backend/internal/module/deliveryprovider/taobao/message_test.go`

- [ ] **Step 1: 写官方固定向量测试**

```go
func TestAPISignerAndMessageVerifierUseSeparateOfficialVectors(t *testing.T) {
	api := loadAPIVectors(t)
	for _, vector := range api {
		got, err := NewAPISigner(vector.PublicExampleSecret).Sign(vector.Parameters, []byte(vector.RawBody))
		if err != nil || got != vector.Expected { t.Fatalf("api %s got=%q err=%v", vector.Name, got, err) }
	}
	messages := loadMessageVectors(t)
	for _, vector := range messages {
		if err := NewMessageVerifier(vector.PublicExampleSecret).Verify(vector.Headers, []byte(vector.RawBody), vector.Signature); err != nil { t.Fatalf("message %s: %v", vector.Name, err) }
	}
}
```

- [ ] **Step 2: 实现两个明确类型**

```go
type APISigner struct { secret []byte }
type MessageVerifier struct { secret []byte }

func (v *MessageVerifier) Verify(headers http.Header, rawBody []byte, received string) error {
	expected, err := v.expected(headers, rawBody)
	if err != nil { return err }
	if subtle.ConstantTimeCompare([]byte(expected), []byte(received)) != 1 { return delivery.ErrExternalSignature }
	return nil
}
```

两个 `expected`/`Sign` 函数分别按 Task 1 的当前 API 网关和消息平台文档实现。禁止因为参数名相似而共用 canonicalization；禁止 JSON 反序列化后重组 raw body。

- [ ] **Step 3: 增加篡改、编码和重放测试**

覆盖 query 顺序、空值、重复参数、UTF-8、body 空格、时间窗、topic、应用 ID、消息 ID、签名大小写和密文认证失败。

- [ ] **Step 4: 运行并提交**

Run: `cd backend; go test ./internal/module/deliveryprovider/taobao -run 'TestAPISigner|TestMessageVerifier' -count=1`

Expected: PASS。

```powershell
git add backend/internal/module/deliveryprovider/taobao/signer.go backend/internal/module/deliveryprovider/taobao/signer_test.go backend/internal/module/deliveryprovider/taobao/message.go backend/internal/module/deliveryprovider/taobao/message_test.go
git commit -m "安全：实现淘宝闪购 API 与消息签名合同"
```

### Task 4: 实现消息回调、Provider 应答和最小提示

**Files:**
- Modify: `backend/internal/module/deliveryprovider/taobao/message.go`
- Modify: `backend/internal/module/deliveryprovider/taobao/message_test.go`

- [ ] **Step 1: 写回调提示测试**

```go
func TestVerifyCallbackStoresOnlyOrderLocator(t *testing.T) {
	p, envelope, binding := officialMessageFixture(t)
	hint, ack, err := p.VerifyCallback(context.Background(), envelope, binding)
	if err != nil { t.Fatal(err) }
	if hint.EventID == "" || hint.SourceOrderID == "" || hint.OccurredAt.IsZero() { t.Fatalf("hint=%+v", hint) }
	if ack.Status != pinnedSuccessStatus() || len(ack.Body) == 0 { t.Fatalf("ack=%+v", ack) }
	for _, forbidden := range [][]byte{[]byte("phone"), []byte("address"), []byte("token")} { if bytes.Contains(bytes.ToLower(hint.QueryHint), forbidden) { t.Fatalf("query hint leaked %q", forbidden) } }
}
```

- [ ] **Step 2: 实现严格消息解析**

只接受合同中列出的订单 topic/type。核对应用、商家/门店定位、消息 ID、发生时间和签名/解密结果；把 `eventId`、`sourceOrderId`、`eventKind`、`occurredAt` 和官方查询所需的非敏感游标写入 `CallbackHint`。Provider 专用成功应答直接来自冻结合同，不复用 `response.OK`。

- [ ] **Step 3: 测试重复、错店、未知 topic 和过期消息**

Run: `cd backend; go test ./internal/module/deliveryprovider/taobao -run TestVerifyCallback -count=1`

Expected: PASS。

- [ ] **Step 4: 提交回调处理**

```powershell
git add backend/internal/module/deliveryprovider/taobao/message.go backend/internal/module/deliveryprovider/taobao/message_test.go
git commit -m "功能：验证并接收淘宝闪购订单消息"
```

### Task 5: 实现权威订单查询和字段归一化

**Files:**
- Create: `backend/internal/module/deliveryprovider/taobao/client.go`
- Create: `backend/internal/module/deliveryprovider/taobao/order.go`
- Create: `backend/internal/module/deliveryprovider/taobao/order_test.go`

- [ ] **Step 1: 写订单查询夹具测试**

```go
func TestFetchOrderPreservesLongOrderIDAndPickupCode(t *testing.T) {
	p, server := providerWithFixture(t, "testdata/official_order_query.json"); defer server.Close()
	binding := testBinding()
	result, err := p.FetchOrder(context.Background(), testTokens(), binding, deliveryplatform.CallbackHint{EventID: "event-1", SourceOrderID: "1234567890123456789", EventKind: "created"})
	if err != nil { t.Fatal(err) }
	if result.Event.Source.SourceOrderID == nil || *result.Event.Source.SourceOrderID != "1234567890123456789" { t.Fatalf("source=%+v", result.Event.Source) }
	if result.Event.Source.PickupCode == "" || result.Event.Recipient.Address == "" { t.Fatalf("event=%+v", result.Event) }
	if result.SourceVersion == "" || result.SourceState == "" { t.Fatalf("missing version/state: %+v", result) }
}
```

- [ ] **Step 2: 实现查询与显式状态映射**

```go
func toExternalEvent(raw officialOrder, binding deliveryplatform.Binding, eventID string) (delivery.ExternalOrderEvent, error) {
	eventType, ok := stateMap[raw.Status]
	if !ok { return delivery.ExternalOrderEvent{}, deliveryplatform.NewProviderError(deliveryplatform.ProviderErrorContract, 0) }
	shop, orderID := binding.SourceShopID, raw.OrderID
	return delivery.NormalizeExternalOrderEvent(delivery.SourceTaobao, delivery.ExternalShopBinding{MerchantUserID: binding.MerchantUserID, Platform: delivery.SourceTaobao, SourceShopID: shop}, delivery.ExternalOrderEvent{
		EventID: eventID, EventType: eventType, OccurredAt: raw.UpdatedAt,
		Source: delivery.SourceMetadata{SourcePlatform: delivery.SourceTaobao, SourceShopID: &shop, SourceOrderID: &orderID, PickupCode: raw.PickupCode},
		Pickup: delivery.Contact{Name: raw.ShopName, Phone: raw.ShopPhone, Address: raw.ShopAddress},
		Recipient: delivery.Contact{Name: raw.RecipientName, Phone: raw.RecipientPhone, Address: raw.RecipientAddress},
		Remark: raw.DeliveryRemark,
	})
}
```

`officialOrder` 字段名由冻结订单查询夹具确定；`OrderSnapshot.SourceVersion` 使用官方订单版本，若合同只有更新时间则使用规范化 UTC RFC3339Nano，`SourceState` 保留官方状态枚举。金额、优惠、商品和结算字段不得进入 `ExternalOrderEvent`。未知状态、门店错配、缺取送信息、失效隐私号分别返回稳定错误分类。

- [ ] **Step 3: 覆盖失败和乱序状态**

测试 19 位订单号、前导零取餐码、错误门店、旧更新时间、取消先到、未知状态、401、429、业务错误和网络未知；用 `errors.As` 断言它们分别进入 `permanent`、`contract`、`authorization`、`rate_limited` 或 `retryable`，且错误文本不含订单响应和 Token。

- [ ] **Step 4: 运行并提交**

Run: `cd backend; go test ./internal/module/deliveryprovider/taobao -run 'TestFetchOrder|TestToExternalEvent' -count=1`

Expected: PASS。

```powershell
git add backend/internal/module/deliveryprovider/taobao/client.go backend/internal/module/deliveryprovider/taobao/order.go backend/internal/module/deliveryprovider/taobao/order_test.go
git commit -m "功能：回查并归一化淘宝闪购订单"
```

### Task 6: 实现增量补拉 checkpoint

**Files:**
- Create: `backend/internal/module/deliveryprovider/taobao/checkpoint.go`
- Create: `backend/internal/module/deliveryprovider/taobao/checkpoint_test.go`
- Create or Modify: `backend/migrations/049_delivery_platform_checkpoints.sql`

- [ ] **Step 1: 添加 checkpoint 表**

```sql
CREATE TABLE IF NOT EXISTS delivery_platform_checkpoints (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  binding_key CHAR(43) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  stream VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  cursor_ciphertext VARBINARY(2048) NOT NULL,
  cursor_key_version SMALLINT UNSIGNED NOT NULL,
  watermark_at DATETIME(6) NULL,
  row_version BIGINT UNSIGNED NOT NULL DEFAULT 1,
  updated_at DATETIME(6) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_delivery_platform_checkpoint (binding_key, stream)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_bin;
```

始终使用 migration 049；即使 foundation 尚未执行，也按 048 → 049 的顺序实施，不能把 checkpoint 表并入 048。

- [ ] **Step 2: 写分页中断恢复测试**

```go
func TestPullCheckpointAdvancesOnlyAfterWholePagePersists(t *testing.T) {
	store := newCheckpointStore(t)
	puller := NewPuller(providerWithTwoPages(t), store, callbackSinkFailingOn("order-2"))
	if err := puller.RunOnce(context.Background(), "binding-A"); err == nil { t.Fatal("expected sink failure") }
	if got := store.Cursor("binding-A"); got != "" { t.Fatalf("cursor advanced to %q", got) }
	puller.sink = callbackSinkOK()
	if err := puller.RunOnce(context.Background(), "binding-A"); err != nil { t.Fatal(err) }
	if got := store.Cursor("binding-A"); got != "page-2" { t.Fatalf("cursor=%q", got) }
}
```

- [ ] **Step 3: 实现租约、分页和推拉去重**

每个绑定/stream 只能有一个租约持有者；一页所有订单提示持久化后才 CAS 推进 cursor。推送与拉取使用相同 `provider_event_id/source_order_id/source_version` 幂等空间。cursor 如含敏感值使用 foundation Token cipher 和 AAD `taobao|binding|checkpoint`。

- [ ] **Step 4: 运行并提交**

Run: `cd backend; go test ./internal/module/deliveryprovider/taobao -run TestPullCheckpoint -count=1`

Expected: PASS。

```powershell
git add backend/migrations/049_delivery_platform_checkpoints.sql backend/internal/module/deliveryprovider/taobao/checkpoint.go backend/internal/module/deliveryprovider/taobao/checkpoint_test.go
git commit -m "功能：补偿拉取淘宝闪购遗漏订单"
```

### Task 7: 注册 Provider 并保持自动化关闭

**Files:**
- Create: `backend/internal/module/deliveryprovider/taobao/provider.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/delivery_capabilities_test.go`

- [ ] **Step 1: 聚合 Provider**

```go
type Provider struct { cfg Config; apiSigner *APISigner; messageVerifier *MessageVerifier; client *http.Client }

func New(cfg Config) (*Provider, error) {
	if err := cfg.Validate(); err != nil { return nil, err }
	client := cfg.HTTPClient; if client == nil { client = secureHTTPClient(15 * time.Second) }
	return &Provider{cfg: cfg, apiSigner: NewAPISigner(cfg.AppSecret), messageVerifier: NewMessageVerifier(cfg.AppSecret), client: client}, nil
}

func (p *Provider) Platform() delivery.SourcePlatform { return delivery.SourceTaobao }
func (p *Provider) AppKey() string { return p.cfg.AppKey }
func (p *Provider) Configured() bool { return p != nil && p.cfg.Validate() == nil }
```

- [ ] **Step 2: 组合根只注册完整配置**

空配置跳过；部分配置启动失败；完整配置只使 `externalPlatforms` 出现 `taobao`。通用 callback worker 未落地前 `externalOrders` 保持 false，绑定默认 `autoImport=false/autoDispatch=false`。

- [ ] **Step 3: 运行和提交**

Run: `cd backend; go test ./internal/module/deliveryprovider/taobao ./internal/module/deliveryplatform ./cmd/server -count=1`

Expected: PASS。

```powershell
git add backend/internal/module/deliveryprovider/taobao/provider.go backend/cmd/server/main.go backend/cmd/server/delivery_capabilities_test.go
git commit -m "功能：注册淘宝闪购订单 Provider"
```

### Task 8: 最终验证

- [ ] **Step 1: 执行完整验收**

```powershell
cd "C:\Users\33141\Documents\New project\backend"
go test ./internal/module/deliveryprovider/taobao ./internal/module/deliveryplatform ./cmd/server -count=1
go test ./...
cd ..
npm run check:boundaries
npm run check:real-wechat-secret-hygiene
git diff --check
```

Expected: 全部 exit 0；真实服务商联调未配置时只能 SKIP，不得命中生产。

- [ ] **Step 2: 分开汇报证据**

分别汇报固定向量、OAuth 测试、消息测试、补拉测试、沙箱、生产授权、自动导入和自动发单。只有前四项自动化通过不等于生产接通。

## 完成标准

- 当前账号正式合同和非秘密向量已冻结。
- OAuth/Token、API 签名、消息验签分别实现并测试。
- 推送与增量补拉不会重复导入或跳过失败页。
- 订单先权威查询再归一化；未知状态 fail closed。
- 没有真实授权时 Provider 不注册；自动导入/发单保持关闭。
