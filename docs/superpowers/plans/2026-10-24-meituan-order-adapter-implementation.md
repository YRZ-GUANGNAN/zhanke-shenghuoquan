# 美团订单适配器 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在已完成外卖平台授权底座后，实现美团外卖门店授权、回调验签、订单回查、字段归一化和 Provider 专用应答。

**Architecture:** `deliveryprovider/meituan` 独占美团协议，向 `deliveryplatform.Provider` 暴露稳定接口。回调只验签并持久化最小提示，权威订单由 worker 调用官方查询接口取得；适配器不支付配送费、不开放骑手池。实际签名和 OAuth 合同必须先从本项目美团账号当前获权文档冻结为非秘密测试向量。

**Tech Stack:** Go 1.25 标准库 HTTP/crypto、Gin、现有 `deliveryplatform` Provider 契约、`httptest`。

---

## 前置条件与文件结构

依赖先完成：`docs/superpowers/plans/2026-10-24-delivery-platform-foundation-implementation.md`。

Create:

- `backend/internal/module/deliveryprovider/meituan/config.go`
- `backend/internal/module/deliveryprovider/meituan/signer.go`
- `backend/internal/module/deliveryprovider/meituan/auth.go`
- `backend/internal/module/deliveryprovider/meituan/client.go`
- `backend/internal/module/deliveryprovider/meituan/callback.go`
- `backend/internal/module/deliveryprovider/meituan/order.go`
- `backend/internal/module/deliveryprovider/meituan/*_test.go`
- `backend/internal/module/deliveryprovider/meituan/testdata/*.json`

Modify:

- `backend/internal/config/config.go`
- `backend/internal/config/config_test.go`
- `backend/cmd/server/main.go`
- `backend/cmd/server/delivery_capabilities_test.go`

不得引用美团跑腿、餐饮品牌直连或其他产品线的签名算法替代外卖 DeveloperId 合同。公开的 AppKey SHA1 示例不是本计划默认合同。

### Task 1: 冻结账号内官方合同并建立执行闸门

**Files:**
- Create: `backend/internal/module/deliveryprovider/meituan/testdata/contract_manifest.json`
- Create: `backend/internal/module/deliveryprovider/meituan/contract_manifest_test.go`

- [ ] **Step 1: 在美团技术服务合作中心取得本应用当前合同**

记录且只记录非秘密事实：文档标题、版本/更新时间、授权入口、redirect URI 规则、Token 字段和时效、订单查询 API、订单确认/取消消息类型、签名字段、签名算法说明、成功应答、限频和错误码。`developerId`、`signKey`、`appAuthToken`、授权码和真实订单不得写入文件或聊天。

若账号尚未获权、文档不可见或没有测试门店，停止本计划；只可继续共享底座，不能实现猜测协议。

- [ ] **Step 2: 从官方文档复制一组脱敏固定向量**

`contract_manifest.json` 必须满足此结构，字段值来自同一版本官方样例：

```json
{
  "provider": "meituan",
  "contractVersion": "账号内文档显示的稳定版本",
  "retrievedAt": "ISO-8601 日期时间",
  "callbackContentType": "application/x-www-form-urlencoded",
  "successStatus": 200,
  "successBody": "{\"data\":\"OK\"}",
  "signatureVectorFile": "official_callback_signature.json",
  "orderQueryFixture": "official_order_query.json"
}
```

上方仅定义 JSON schema，不能原样提交；`contractVersion` 和 `retrievedAt` 必须替换为账号内文档的真实非秘密值。无法取得时停在本 Task，不执行 Task 2 及后续任务。

清单不得包含任何真实标识或秘密；如官方样例含可用密钥，改用平台明确标注的公开示例并保存其来源页哈希。

- [ ] **Step 3: 添加合同闸门测试**

```go
func TestContractManifestIsPinnedAndNonSecret(t *testing.T) {
	raw, err := os.ReadFile("testdata/contract_manifest.json")
	if err != nil { t.Fatal(err) }
	var value struct {
		Provider, ContractVersion, RetrievedAt, CallbackContentType, SuccessBody, SignatureVectorFile, OrderQueryFixture string
		SuccessStatus int
	}
	if json.Unmarshal(raw, &value) != nil || value.Provider != "meituan" || value.ContractVersion == "" || value.RetrievedAt == "" || value.SignatureVectorFile == "" || value.OrderQueryFixture == "" { t.Fatal("incomplete Meituan contract manifest") }
	for _, forbidden := range []string{"signKey", "appAuthToken", "developerId", "accessToken", "refreshToken"} {
		if bytes.Contains(bytes.ToLower(raw), bytes.ToLower([]byte(forbidden))) { t.Fatalf("manifest contains secret-shaped field %q", forbidden) }
	}
}
```

- [ ] **Step 4: 运行闸门测试并提交**

Run: `cd backend; go test ./internal/module/deliveryprovider/meituan -run TestContractManifest -count=1`

Expected: PASS；没有真实合同清单时必须 FAIL，不能跳过。

```powershell
git add backend/internal/module/deliveryprovider/meituan/testdata backend/internal/module/deliveryprovider/meituan/contract_manifest_test.go
git commit -m "文档：冻结美团外卖接入合同"
```

### Task 2: 配置完整性与 fail-closed 构造

**Files:**
- Create: `backend/internal/module/deliveryprovider/meituan/config.go`
- Create: `backend/internal/module/deliveryprovider/meituan/config_test.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`

- [ ] **Step 1: 写配置组合测试**

```go
func TestConfigRequiresAllCredentialReferences(t *testing.T) {
	valid := Config{DeveloperID: "10001", SignKey: "unit-test-key", AppKey: "mt-main", APIBaseURL: "https://api-open-cater.meituan.com", AuthorizationBaseURL: "https://developer.meituan.com", ContractVersion: "pinned"}
	if err := valid.Validate(); err != nil { t.Fatal(err) }
	invalid := valid; invalid.SignKey = ""
	if err := invalid.Validate(); err == nil { t.Fatal("partial config accepted") }
	invalid = valid; invalid.APIBaseURL = "http://example.com"
	if err := invalid.Validate(); err == nil { t.Fatal("non-HTTPS API accepted") }
}
```

- [ ] **Step 2: 实现不可部分配置的 Provider 配置**

```go
type Config struct {
	DeveloperID string
	SignKey string
	AppKey string
	APIBaseURL string
	AuthorizationBaseURL string
	ContractVersion string
	HTTPClient *http.Client
}

func (c Config) Validate() error {
	values := []string{c.DeveloperID, c.SignKey, c.AppKey, c.APIBaseURL, c.AuthorizationBaseURL, c.ContractVersion}
	for _, value := range values { if strings.TrimSpace(value) == "" { return ErrNotConfigured } }
	for _, raw := range []string{c.APIBaseURL, c.AuthorizationBaseURL} {
		u, err := url.Parse(raw); if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil { return ErrNotConfigured }
	}
	return nil
}
```

根配置增加 `MEITUAN_DEVELOPER_ID`、`MEITUAN_SIGN_KEY`、`MEITUAN_PROVIDER_APP_KEY`、`MEITUAN_API_BASE_URL`、`MEITUAN_AUTH_BASE_URL`、`MEITUAN_CONTRACT_VERSION`。只有 `ENABLE_EXTERNAL_ORDERS=true` 且任一美团字段出现时才要求整套完整；日志不得打印 `SignKey`。

- [ ] **Step 3: 运行配置测试并提交**

Run: `cd backend; go test ./internal/config ./internal/module/deliveryprovider/meituan -run 'Test.*Config' -count=1`

Expected: PASS。

```powershell
git add backend/internal/config/config.go backend/internal/config/config_test.go backend/internal/module/deliveryprovider/meituan/config.go backend/internal/module/deliveryprovider/meituan/config_test.go
git commit -m "配置：校验美团外卖 Provider 凭据"
```

### Task 3: 以官方固定向量实现签名器

**Files:**
- Create: `backend/internal/module/deliveryprovider/meituan/signer.go`
- Create: `backend/internal/module/deliveryprovider/meituan/signer_test.go`
- Create: `backend/internal/module/deliveryprovider/meituan/testdata/official_callback_signature.json`

- [ ] **Step 1: 写固定向量测试**

```go
type signatureVector struct {
	Name string `json:"name"`
	RawBody string `json:"rawBody"`
	Parameters map[string]string `json:"parameters"`
	PublicExampleKey string `json:"publicExampleKey"`
	Expected string `json:"expected"`
}

func TestSignerMatchesPinnedOfficialVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/official_callback_signature.json"); if err != nil { t.Fatal(err) }
	var vectors []signatureVector; if json.Unmarshal(raw, &vectors) != nil || len(vectors) == 0 { t.Fatal("invalid vectors") }
	for _, vector := range vectors {
		got, err := NewSigner(vector.PublicExampleKey).Sign([]byte(vector.RawBody), vector.Parameters)
		if err != nil || got != vector.Expected { t.Fatalf("%s got=%q err=%v", vector.Name, got, err) }
	}
}
```

- [ ] **Step 2: 运行测试确认签名器缺失**

Run: `cd backend; go test ./internal/module/deliveryprovider/meituan -run TestSignerMatchesPinnedOfficialVectors -count=1`

Expected: FAIL，`NewSigner` 未定义。

- [ ] **Step 3: 按 Task 1 冻结的 DeveloperId 外卖签名章节逐字实现**

实现必须满足这些代码级约束：

```go
type Signer struct { secret []byte }

func NewSigner(secret string) *Signer { return &Signer{secret: []byte(secret)} }

func (s *Signer) Verify(rawBody []byte, parameters map[string]string, received string) error {
	expected, err := s.Sign(rawBody, parameters)
	if err != nil { return err }
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(expected)), []byte(strings.ToLower(strings.TrimSpace(received)))) != 1 { return delivery.ErrExternalSignature }
	return nil
}
```

`Sign` 的参数筛选、排序、Secret 前后位置、body 是否参与、编码和摘要算法只能来自冻结合同；不得复用公开的“餐饮品牌直连 AppKey SHA1”代码，除非账号内外卖 DeveloperId 文档明确声明相同且固定向量通过。`rawBody` 必须使用 HTTP 收到的原始字节，不得反序列化后重组。

- [ ] **Step 4: 增加篡改与常量比较测试**

```go
func TestSignerRejectsBodyParameterAndSignatureMutation(t *testing.T) {
	vector := loadFirstVector(t)
	signer := NewSigner(vector.PublicExampleKey)
	if err := signer.Verify([]byte(vector.RawBody+" "), vector.Parameters, vector.Expected); !errors.Is(err, delivery.ErrExternalSignature) { t.Fatal(err) }
	changed := maps.Clone(vector.Parameters); changed["ePoiId"] += "x"
	if err := signer.Verify([]byte(vector.RawBody), changed, vector.Expected); !errors.Is(err, delivery.ErrExternalSignature) { t.Fatal(err) }
	if err := signer.Verify([]byte(vector.RawBody), vector.Parameters, vector.Expected+"0"); !errors.Is(err, delivery.ErrExternalSignature) { t.Fatal(err) }
}
```

- [ ] **Step 5: 运行签名测试并提交**

Run: `cd backend; go test ./internal/module/deliveryprovider/meituan -run TestSigner -count=1`

Expected: PASS。

```powershell
git add backend/internal/module/deliveryprovider/meituan/signer.go backend/internal/module/deliveryprovider/meituan/signer_test.go backend/internal/module/deliveryprovider/meituan/testdata/official_callback_signature.json
git commit -m "安全：实现美团外卖回调签名验证"
```

### Task 4: 实现授权、Token 与门店查询

**Files:**
- Create: `backend/internal/module/deliveryprovider/meituan/auth.go`
- Create: `backend/internal/module/deliveryprovider/meituan/auth_test.go`

- [ ] **Step 1: 写官方授权 URL 和 Token 响应测试**

```go
func TestAuthorizationUsesFixedRedirectAndState(t *testing.T) {
	provider := newProviderForTest(t)
	raw, err := provider.BuildAuthorizationURL(deliveryplatform.AuthorizationRequest{State: "state-value", RedirectURI: "https://api.example.com/callback", ProviderAppKey: "mt-main"})
	if err != nil { t.Fatal(err) }
	u, _ := url.Parse(raw)
	if u.Scheme != "https" || u.Query().Get("state") != "state-value" || u.Query().Get("redirect_uri") != "https://api.example.com/callback" { t.Fatalf("url=%s", raw) }
}
```

- [ ] **Step 2: 实现授权方法**

`BuildAuthorizationURL`、`ExchangeAuthorizationCode`、`Refresh`、`Revoke`、`ListShops` 的路径、字段和 HTTP 方法逐项取自 Task 1 合同。共享要求：固定 host、禁止任意 redirect、15 秒超时、响应体上限 1 MiB、HTTP 与业务码双重校验、Token 不进入 error。

```go
func decodeLimitedJSON(response *http.Response, target any) error {
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 { return classifyHTTP(response.StatusCode) }
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func classifyHTTP(status int) error {
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

- [ ] **Step 3: 用 `httptest.Server` 覆盖授权失败分类**

测试 200+业务失败、401、429、500、超时、超大响应、缺 Token、scope 不足、门店为空和 Token 刷新轮换；断言错误中不含返回的测试 Token，并用 `errors.As(err, &deliveryplatform.ClassifiedProviderError)` 核对分类。网络超时包装为 `ProviderErrorRetryable`，200+未知业务码包装为 `ProviderErrorContract`。

- [ ] **Step 4: 运行授权测试并提交**

Run: `cd backend; go test ./internal/module/deliveryprovider/meituan -run 'TestAuthorization|TestToken|TestShop' -count=1`

Expected: PASS。

```powershell
git add backend/internal/module/deliveryprovider/meituan/auth.go backend/internal/module/deliveryprovider/meituan/auth_test.go
git commit -m "功能：接入美团门店授权与 Token 生命周期"
```

### Task 5: 实现 Provider 专用回调解析与应答

**Files:**
- Create: `backend/internal/module/deliveryprovider/meituan/callback.go`
- Create: `backend/internal/module/deliveryprovider/meituan/callback_test.go`

- [ ] **Step 1: 写订单确认消息测试**

```go
func TestVerifyCallbackReturnsDurabilityAckAndMinimalHint(t *testing.T) {
	provider, envelope, binding := officialCallbackFixture(t)
	hint, ack, err := provider.VerifyCallback(context.Background(), envelope, binding)
	if err != nil { t.Fatal(err) }
	if hint.EventID == "" || hint.SourceOrderID == "" || hint.EventKind == "" { t.Fatalf("hint=%+v", hint) }
	if ack.Status != http.StatusOK || string(ack.Body) != `{"data":"OK"}` { t.Fatalf("ack=%+v", ack) }
	if bytes.Contains(hint.QueryHint, []byte("recipient_phone")) { t.Fatal("callback hint retained customer contact") }
}
```

- [ ] **Step 2: 实现 form 解析、心跳和严格字段验证**

```go
func parseSingle(values url.Values, key string) (string, error) {
	items := values[key]
	if len(items) != 1 || strings.TrimSpace(items[0]) == "" { return "", delivery.ErrInvalidRecord }
	return items[0], nil
}

func successAck() deliveryplatform.CallbackAck {
	return deliveryplatform.CallbackAck{Status: http.StatusOK, ContentType: "application/json; charset=utf-8", Body: []byte(`{"data":"OK"}`)}
}
```

只允许合同清单列出的订单创建/确认/更新/取消消息类型。核对 `developerId`、`ePoiId`、`businessId`、可选业务编码、`msgId` 和秒级时间窗；心跳空包使用官方要求的独立应答，不插入订单提示。签名失败、门店错配、重复关键字段不得返回成功。

- [ ] **Step 3: 覆盖重放和乱序输入**

测试 timestamp 过期/未来、重复 `msgId`、错误 `ePoiId`、错误 developerId、未知 msgType、缺 `order_id`、URL 编码差异和超过 1 MiB body。

- [ ] **Step 4: 运行回调测试并提交**

Run: `cd backend; go test ./internal/module/deliveryprovider/meituan -run TestVerifyCallback -count=1`

Expected: PASS。

```powershell
git add backend/internal/module/deliveryprovider/meituan/callback.go backend/internal/module/deliveryprovider/meituan/callback_test.go
git commit -m "功能：验证并接收美团订单回调"
```

### Task 6: 实现订单回查和字段归一化

**Files:**
- Create: `backend/internal/module/deliveryprovider/meituan/client.go`
- Create: `backend/internal/module/deliveryprovider/meituan/order.go`
- Create: `backend/internal/module/deliveryprovider/meituan/order_test.go`
- Create: `backend/internal/module/deliveryprovider/meituan/testdata/official_order_query.json`

- [ ] **Step 1: 写权威订单快照测试**

```go
func TestFetchOrderPreservesIdentifiersAndMapsDeliveryFields(t *testing.T) {
	provider, server := providerWithOrderFixture(t, "testdata/official_order_query.json"); defer server.Close()
	snapshot, err := provider.FetchOrder(context.Background(), testTokens(), testBinding(), deliveryplatform.CallbackHint{EventID: "msg-1", SourceOrderID: "658769783771776", EventKind: "confirmed"})
	if err != nil { t.Fatal(err) }
	if snapshot.Event.Source.SourceOrderID == nil || *snapshot.Event.Source.SourceOrderID != "658769783771776" { t.Fatalf("source=%+v", snapshot.Event.Source) }
	if snapshot.Event.Source.SourceShopID == nil || *snapshot.Event.Source.SourceShopID != testBinding().SourceShopID { t.Fatal("shop mismatch") }
	if snapshot.Event.Recipient.Address == "" || snapshot.Event.Pickup.Address == "" { t.Fatal("missing route") }
	if snapshot.SourceVersion == "" || snapshot.SourceState == "" { t.Fatalf("missing version/state: %+v", snapshot) }
}
```

- [ ] **Step 2: 实现官方查询请求与状态映射**

请求使用完整订单号字符串和门店 `appAuthToken`；签名输入来自同一份序列化参数。响应先验证业务码、订单 ID、门店，再映射：

```go
func normalizeEvent(kind string, order officialOrder, binding deliveryplatform.Binding, eventID string) (delivery.ExternalOrderEvent, error) {
	eventType, ok := eventTypeByOfficialState[kind+":"+order.Status]
	if !ok { return delivery.ExternalOrderEvent{}, deliveryplatform.NewProviderError(deliveryplatform.ProviderErrorContract, 0) }
	shop, orderID := binding.SourceShopID, order.ID
	return delivery.NormalizeExternalOrderEvent(delivery.SourceMeituan, delivery.ExternalShopBinding{MerchantUserID: binding.MerchantUserID, Platform: delivery.SourceMeituan, SourceShopID: shop}, delivery.ExternalOrderEvent{
		EventID: eventID, EventType: eventType, OccurredAt: order.UpdatedAt,
		Source: delivery.SourceMetadata{SourcePlatform: delivery.SourceMeituan, SourceShopID: &shop, SourceOrderID: &orderID, PickupCode: order.PickupCode},
		Pickup: delivery.Contact{Name: order.ShopName, Phone: order.ShopPhone, Address: order.ShopAddress},
		Recipient: delivery.Contact{Name: order.RecipientName, Phone: order.RecipientPhone, Address: order.RecipientAddress},
		Remark: order.DeliveryRemark,
	})
}
```

`officialOrder` 字段必须逐个来自 Task 1 的当前订单查询文档；`OrderSnapshot.SourceVersion` 使用官方可比较订单版本，缺版本时使用官方 `update_time` 的 UTC RFC3339Nano 字符串，`SourceState` 保存合同中的原状态枚举。未知状态返回共享 contract error，不得默认 created。订单价格/结算字段不写入配送模型。

- [ ] **Step 3: 测试门店错配、缺字段和状态乱序**

覆盖完整订单号精度、前导零取餐码、门店错配、隐私号、缺地址、未知状态、旧 `update_time`、429、Token 失效和超时；分别断言 `permanent`、`contract`、`rate_limited`、`authorization`、`retryable` 分类。

- [ ] **Step 4: 运行订单测试并提交**

Run: `cd backend; go test ./internal/module/deliveryprovider/meituan -run 'TestFetchOrder|TestNormalize' -count=1`

Expected: PASS。

```powershell
git add backend/internal/module/deliveryprovider/meituan/client.go backend/internal/module/deliveryprovider/meituan/order.go backend/internal/module/deliveryprovider/meituan/order_test.go backend/internal/module/deliveryprovider/meituan/testdata/official_order_query.json
git commit -m "功能：回查并归一化美团外卖订单"
```

### Task 7: 注册 Provider 但保持导入开关关闭

**Files:**
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/delivery_capabilities_test.go`
- Create: `backend/internal/module/deliveryprovider/meituan/provider.go`

- [ ] **Step 1: 实现 Provider 聚合类型**

```go
type Provider struct { cfg Config; signer *Signer; client *http.Client }

func New(cfg Config) (*Provider, error) {
	if err := cfg.Validate(); err != nil { return nil, err }
	client := cfg.HTTPClient; if client == nil { client = &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }} }
	return &Provider{cfg: cfg, signer: NewSigner(cfg.SignKey), client: client}, nil
}

func (p *Provider) Platform() delivery.SourcePlatform { return delivery.SourceMeituan }
func (p *Provider) AppKey() string { return p.cfg.AppKey }
func (p *Provider) Configured() bool { return p != nil && p.cfg.Validate() == nil }
```

- [ ] **Step 2: 在组合根仅对完整配置注册**

配置完全为空时跳过美团 Provider；部分配置在启动校验中失败。注册成功后 `externalPlatforms` 可包含 `meituan`，但在通用导入 worker 尚未完成前 `externalOrders` 仍为 false，商家 `autoImport/autoDispatch` 默认 false。

- [ ] **Step 3: 运行包、能力和边界测试**

```powershell
cd backend
go test ./internal/module/deliveryprovider/meituan ./internal/module/deliveryplatform ./cmd/server -count=1
```

Expected: PASS；空/部分配置不注册，完整测试配置只报告 Provider configured。

- [ ] **Step 4: 提交注册代码**

```powershell
git add backend/internal/module/deliveryprovider/meituan/provider.go backend/cmd/server/main.go backend/cmd/server/delivery_capabilities_test.go
git commit -m "功能：注册美团外卖订单 Provider"
```

### Task 8: 最终验证

- [ ] **Step 1: 运行完整后端与秘密卫生检查**

```powershell
cd "C:\Users\33141\Documents\New project\backend"
go test ./internal/module/deliveryprovider/meituan ./internal/module/deliveryplatform ./internal/module/admin/deliveryplatform ./cmd/server -count=1
go test ./...
cd ..
npm run check:boundaries
npm run check:real-wechat-secret-hygiene
git diff --check
```

Expected: 全部 exit 0；需要真实美团账号的联调测试必须以独立 build tag/环境变量 SKIP，不能访问生产。

- [ ] **Step 2: 记录真实联调仍未完成的状态**

在提交汇报中分别写明：代码固定向量 PASS、沙箱 PENDING 或 PASS、生产门店授权 PENDING、自动导入关闭、自动发单关闭。不得把 Provider 注册写成“三端已接通”。

## 完成标准

- 账号内当前合同已冻结且无秘密入库。
- 官方固定签名向量、回调应答和订单查询夹具全部通过。
- 美团回调只持久接收提示；订单由官方查询回查。
- 门店错配、未知状态、Token 失效和限频 fail closed。
- 此计划结束时仍未自动扣款、未开放骑手池、未部署生产。
