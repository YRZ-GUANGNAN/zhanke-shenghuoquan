# 微信媒体审核私有内容代理 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用同一个短期私有地址同时支持微信 HEAD/GET 媒体下载，正确消费平台错误与无关消息，并以全新生产图片完成严格审核验收。

**Architecture:** 新增 `WechatReviewProxy`，用 AES-256-GCM 能力令牌绑定 COS `final_key + final_version_id + expiresAt`，通过现有 API 域名流式代理固定版本对象。现有异步提交器只替换 URL signer；projection/outbox/consumer 继续复用，回调层增加官方 envelope 分流和封闭失败码。

**Tech Stack:** Go 1.25、Gin 1.10、AES-GCM、Tencent COS、MySQL/GORM、微信 `mediaCheckAsync` v2、PowerShell/SSH、微信开发者工具 `miniprogram-automator`。

---

## 基线与工作区约束

- 设计：`docs/superpowers/specs/2026-07-16-wechat-review-content-proxy-design.md`
- 当前代码 HEAD（计划编写时）：`dcb7670`
- 生产运行诊断 commit：`e1c2db8`
- 生产诊断二进制 SHA256：`1c97361908ce891864887e52fa32ab6a883634117f89ec5794d9f15eb5a83132`
- 生产 current：`/opt/campus-circle/releases/90b62c320598`
- 当前工作树已有用户改动，所有提交必须使用精确路径；禁止 `git add .`、`git add -A` 或清理用户文件。
- 不修改 `backend/migrations/**`、`apps/miniapp/src/**`、`deploy/native/**`、Nginx 配置或任何 `.env.example`。
- 本次只替换 `campus-api` binary，不轮换已经验证的微信 callback Token。

## 文件结构与责任

- Create: `backend/internal/module/media/miniappflow/wechat_review_proxy.go`
  - AES-GCM token、URL signer、HEAD/GET handler、固定版本 COS 读取。
- Create: `backend/internal/module/media/miniappflow/wechat_review_proxy_test.go`
  - token、元数据、方法、错误映射和无泄漏测试。
- Modify: `backend/internal/module/media/miniappflow/wechat_review.go`
  - `WechatReviewURLTTL` 从 10 分钟调整到 35 分钟；增加封闭平台失败码/hash helper。
- Modify: `backend/internal/module/media/miniappflow/wechat_review_test.go`
  - signer TTL 与失败码/hash 测试。
- Modify: `backend/internal/module/media/miniappflow/service.go`
  - 新增幂等 `ApplyWechatReviewFailure`，复用现有 decision outbox。
- Modify: `backend/internal/module/media/miniappflow/service_test.go`
  - submitted/reviewing/terminal/非法失败码测试。
- Modify: `backend/internal/module/media/miniappflow/wechat_callback_handler.go`
  - 严格解析 `MsgType/Event/errcode/Encrypt`，分流 result/failure/ignore。
- Modify: `backend/internal/module/media/miniappflow/wechat_callback_handler_test.go`
  - 官方成功、平台错误、无关推送、重复字段和错误映射测试。
- Modify: `backend/cmd/server/main.go`
  - 非 mock 环境构造代理，把它同时接到 submitter 与公共媒体 group。
- Modify: `backend/cmd/server/router_logging_test.go`
  - 代理精确路径的 GET/HEAD/query/recovery 脱敏。
- Modify: `backend/internal/module/media/miniappflow/boundary_test.go`
  - 公共代理与冻结鉴权媒体路由的 composition 边界。

### Task 1: 实现加密能力令牌与代理 URL signer

**Files:**
- Create: `backend/internal/module/media/miniappflow/wechat_review_proxy.go`
- Create: `backend/internal/module/media/miniappflow/wechat_review_proxy_test.go`
- Modify: `backend/internal/module/media/miniappflow/wechat_review.go`

- [ ] **Step 1: 写入会失败的 constructor、token 和 signer 测试**

在 `wechat_review_proxy_test.go` 先加入以下测试骨架；测试与实现同包，以便固定时钟：

```go
package miniappflow

import (
	"context"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
)

type reviewProxyStoreStub struct{}

func (reviewProxyStoreStub) HeadObject(context.Context, string, string) (coremedia.CompletionObject, error) {
	return coremedia.CompletionObject{}, coremedia.ErrObjectNotFound
}

func (reviewProxyStoreStub) OpenObject(context.Context, string, string) (io.ReadCloser, error) {
	return nil, coremedia.ErrObjectNotFound
}

func TestWechatReviewProxyConstructorRejectsUnsafeBaseURL(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		"", "http://api.example.test", "https://", "https://user@api.example.test",
		"https://api.example.test/base", "https://api.example.test?token=bad", "https://api.example.test#fragment",
	} {
		if _, err := NewWechatReviewProxy(WechatReviewProxyConfig{
			PublicBaseURL: value,
			AppSecret:     "wechat-app-secret-at-least-sixteen",
		}, reviewProxyStoreStub{}); err == nil {
			t.Fatalf("unsafe base URL was accepted: %q", value)
		}
	}
}

func TestWechatReviewProxyTokenBindsFinalVersionAndExpires(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	proxy, err := NewWechatReviewProxy(WechatReviewProxyConfig{
		PublicBaseURL: "https://api.example.test/",
		AppSecret:     "wechat-app-secret-at-least-sixteen",
	}, reviewProxyStoreStub{})
	if err != nil {
		t.Fatal(err)
	}
	proxy.now = func() time.Time { return now }
	key, versionID := "final/prod/product/2026/07/01K00000000000000000000001/asset.png", "version-1"
	value, err := proxy.SignedGetURL(context.Background(), key, versionID, WechatReviewURLTTL)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "https" || parsed.Host != "api.example.test" || parsed.Path != WechatReviewContentPath {
		t.Fatalf("unexpected proxy URL: %s", value)
	}
	token := parsed.Query().Get("token")
	if token == "" || strings.Contains(token, key) || strings.Contains(token, versionID) {
		t.Fatal("opaque token is missing or exposes object identity")
	}
	claims, err := proxy.openClaims(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Version != 1 || claims.ObjectKey != key || claims.VersionID != versionID ||
		claims.ExpiresAt != now.Add(WechatReviewURLTTL).Unix() {
		t.Fatalf("unexpected claims: %#v", claims)
	}

	other, err := proxy.SignedGetURL(context.Background(), key, versionID, WechatReviewURLTTL)
	if err != nil || other == value {
		t.Fatal("independent issuance must use a fresh random GCM nonce")
	}
}

func TestWechatReviewProxyRejectsTamperedExpiredAndInvalidIdentity(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	proxy, err := NewWechatReviewProxy(WechatReviewProxyConfig{
		PublicBaseURL: "https://api.example.test",
		AppSecret:     "wechat-app-secret-at-least-sixteen",
	}, reviewProxyStoreStub{})
	if err != nil {
		t.Fatal(err)
	}
	proxy.now = func() time.Time { return now }
	value, err := proxy.SignedGetURL(context.Background(), "final/prod/product/2026/07/id/image.png", "v1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(value)
	token := parsed.Query().Get("token")
	tampered := token[:len(token)-1] + "A"
	if _, err := proxy.openClaims(tampered); err == nil {
		t.Fatal("tampered token was accepted")
	}
	proxy.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := proxy.openClaims(token); err == nil {
		t.Fatal("expired token was accepted")
	}
	proxy.now = func() time.Time { return now }
	for _, identity := range [][2]string{
		{"../secret.png", "v1"}, {"final/prod/item.exe", "v1"}, {"final/prod/item.png", ""},
	} {
		if _, err := proxy.SignedGetURL(context.Background(), identity[0], identity[1], time.Minute); err == nil {
			t.Fatalf("invalid identity was accepted: %#v", identity)
		}
	}
}
```

- [ ] **Step 2: 运行定向测试并确认红灯原因**

Run:

```powershell
cd 'C:\Users\33141\Documents\New project\backend'
go test ./internal/module/media/miniappflow -run 'TestWechatReviewProxy(Constructor|Token|Rejects)' -count=1
```

Expected: FAIL，错误包含 `undefined: NewWechatReviewProxy` 或缺少相关类型，证明测试命中新组件而不是现有 COS signer。

- [ ] **Step 3: 写入最小 constructor、AES-GCM codec 与 signer 实现**

把 `WechatReviewURLTTL` 从 `wechat_review.go` 的 10 分钟改为 35 分钟。在新文件 `wechat_review_proxy.go` 写入以下核心实现；错误统一收敛为 `errInvalidWechatReviewProxy`，不得拼接 token/claims：

```go
package miniappflow

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
)

const (
	WechatReviewContentPath = "/api/v1/miniapp/media/wechat/review-content"

	wechatReviewProxyMaxFuture = 36 * time.Minute
	wechatReviewProxyMaxToken  = 2048
	wechatReviewProxyMaxBytes  = uint64(10 << 20)
	wechatReviewProxyKDFDomain = "campus/wechat-review-proxy/v1\x00"
	wechatReviewProxyAAD       = "campus/wechat-review-proxy:v1"
)

var errInvalidWechatReviewProxy = errors.New("invalid wechat review proxy capability")

type WechatReviewContentStore interface {
	HeadObject(context.Context, string, string) (coremedia.CompletionObject, error)
	OpenObject(context.Context, string, string) (io.ReadCloser, error)
}

type WechatReviewProxyConfig struct {
	PublicBaseURL string
	AppSecret     string
}

type WechatReviewProxy struct {
	publicBaseURL string
	store         WechatReviewContentStore
	codec         *wechatReviewProxyCodec
	now           func() time.Time
}

type wechatReviewProxyClaims struct {
	Version   int    `json:"version"`
	ExpiresAt int64  `json:"expiresAt"`
	ObjectKey string `json:"objectKey"`
	VersionID string `json:"versionId"`
}

type wechatReviewProxyCodec struct {
	aead cipher.AEAD
}

func NewWechatReviewProxy(config WechatReviewProxyConfig, store WechatReviewContentStore) (*WechatReviewProxy, error) {
	parsed, err := url.Parse(strings.TrimSpace(config.PublicBaseURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil ||
		parsed.Opaque != "" || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") || store == nil {
		return nil, errInvalidWechatReviewProxy
	}
	codec, err := newWechatReviewProxyCodec(config.AppSecret)
	if err != nil {
		return nil, err
	}
	return &WechatReviewProxy{
		publicBaseURL: parsed.Scheme + "://" + parsed.Host,
		store:         store,
		codec:         codec,
		now:           func() time.Time { return time.Now().UTC() },
	}, nil
}

func newWechatReviewProxyCodec(appSecret string) (*wechatReviewProxyCodec, error) {
	if len(strings.TrimSpace(appSecret)) < 16 {
		return nil, errInvalidWechatReviewProxy
	}
	sum := sha256.Sum256([]byte(wechatReviewProxyKDFDomain + appSecret))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &wechatReviewProxyCodec{aead: aead}, nil
}

func (c *wechatReviewProxyCodec) seal(claims wechatReviewProxyClaims) (string, error) {
	plaintext, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nil, nonce, plaintext, []byte(wechatReviewProxyAAD))
	return base64.RawURLEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func (c *wechatReviewProxyCodec) open(token string) (wechatReviewProxyClaims, error) {
	if token == "" || len(token) > wechatReviewProxyMaxToken {
		return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) < c.aead.NonceSize()+c.aead.Overhead() {
		return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
	}
	plaintext, err := c.aead.Open(nil, raw[:c.aead.NonceSize()], raw[c.aead.NonceSize():], []byte(wechatReviewProxyAAD))
	if err != nil {
		return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
	}
	return decodeWechatReviewProxyClaims(plaintext)
}

func decodeWechatReviewProxyClaims(raw []byte) (wechatReviewProxyClaims, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
	}
	var claims wechatReviewProxyClaims
	seen := map[string]bool{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		key, ok := keyToken.(string)
		if err != nil || !ok || seen[key] {
			return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
		}
		seen[key] = true
		switch key {
		case "version":
			err = decoder.Decode(&claims.Version)
		case "expiresAt":
			err = decoder.Decode(&claims.ExpiresAt)
		case "objectKey":
			err = decoder.Decode(&claims.ObjectKey)
		case "versionId":
			err = decoder.Decode(&claims.VersionID)
		default:
			return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
		}
		if err != nil {
			return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
		}
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
	}
	return claims, nil
}

func (p *WechatReviewProxy) SignedGetURL(ctx context.Context, key, versionID string, ttl time.Duration) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	if ttl <= 0 || ttl > WechatReviewURLTTL || !validWechatReviewObjectIdentity(key, versionID) {
		return "", errInvalidWechatReviewProxy
	}
	claims := wechatReviewProxyClaims{
		Version: 1, ExpiresAt: p.now().UTC().Add(ttl).Unix(), ObjectKey: key, VersionID: versionID,
	}
	token, err := p.codec.seal(claims)
	if err != nil {
		return "", err
	}
	query := url.Values{"token": {token}}
	return p.publicBaseURL + WechatReviewContentPath + "?" + query.Encode(), nil
}

func (p *WechatReviewProxy) openClaims(token string) (wechatReviewProxyClaims, error) {
	claims, err := p.codec.open(token)
	if err != nil || !validWechatReviewProxyClaims(claims, p.now().UTC()) {
		return wechatReviewProxyClaims{}, errInvalidWechatReviewProxy
	}
	return claims, nil
}

func validWechatReviewProxyClaims(claims wechatReviewProxyClaims, now time.Time) bool {
	return claims.Version == 1 && claims.ExpiresAt > now.Unix() &&
		claims.ExpiresAt <= now.Add(wechatReviewProxyMaxFuture).Unix() &&
		validWechatReviewObjectIdentity(claims.ObjectKey, claims.VersionID)
}

func validWechatReviewObjectIdentity(key, versionID string) bool {
	if key == "" || len(key) > 512 || key != strings.TrimSpace(key) ||
		versionID == "" || len(versionID) > 128 || versionID != strings.TrimSpace(versionID) ||
		strings.Contains(key, "\\") || strings.Contains(key, "//") || path.Clean(key) != key ||
		!strings.HasPrefix(key, "final/") || strings.ContainsAny(versionID, "\r\n") {
		return false
	}
	switch strings.ToLower(path.Ext(key)) {
	case ".jpg", ".jpeg", ".png", ".webp":
		return true
	default:
		return false
	}
}
```

- [ ] **Step 4: 运行 token/signer 测试并确认通过**

Run:

```powershell
go test ./internal/module/media/miniappflow -run 'TestWechatReviewProxy(Constructor|Token|Rejects)' -count=1
```

Expected: PASS；测试输出不得包含 token、object key 解密值之外的秘密或 AppSecret。

- [ ] **Step 5: 精确提交 Task 1**

```powershell
git add -- backend/internal/module/media/miniappflow/wechat_review_proxy.go backend/internal/module/media/miniappflow/wechat_review_proxy_test.go backend/internal/module/media/miniappflow/wechat_review.go
git diff --cached --check
git commit -m "feat(media): 增加微信审核私有令牌"
```

### Task 2: 实现同一 token 的 HEAD/GET 固定版本代理

**Files:**
- Modify: `backend/internal/module/media/miniappflow/wechat_review_proxy.go`
- Modify: `backend/internal/module/media/miniappflow/wechat_review_proxy_test.go`

- [ ] **Step 1: 写入会失败的 HTTP 行为测试**

在 `wechat_review_proxy_test.go` 增加可记录调用的 store，并验证同一个 URL 先 HEAD 后 GET：

```go
type recordingReviewProxyStore struct {
	object    coremedia.CompletionObject
	payload   []byte
	headErr   error
	openErr   error
	headCalls int
	openCalls int
	headKey   string
	headVer   string
	openKey   string
	openVer   string
}

func (s *recordingReviewProxyStore) HeadObject(_ context.Context, key, version string) (coremedia.CompletionObject, error) {
	s.headCalls++
	s.headKey, s.headVer = key, version
	if s.headErr != nil {
		return coremedia.CompletionObject{}, s.headErr
	}
	return s.object, nil
}

func (s *recordingReviewProxyStore) OpenObject(_ context.Context, key, version string) (io.ReadCloser, error) {
	s.openCalls++
	s.openKey, s.openVer = key, version
	if s.openErr != nil {
		return nil, s.openErr
	}
	return io.NopCloser(strings.NewReader(string(s.payload))), nil
}

func TestWechatReviewProxyUsesSameTokenForHeadAndGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	payload := []byte("safe-png-test-payload")
	key, versionID := "final/prod/product/2026/07/id/image.png", "version-7"
	store := &recordingReviewProxyStore{
		payload: payload,
		object: coremedia.CompletionObject{
			Key: key, VersionID: versionID, ContentType: "image/png", Size: uint64(len(payload)),
		},
	}
	proxy, err := NewWechatReviewProxy(WechatReviewProxyConfig{
		PublicBaseURL: "https://api.example.test", AppSecret: "wechat-app-secret-at-least-sixteen",
	}, store)
	if err != nil {
		t.Fatal(err)
	}
	value, err := proxy.SignedGetURL(context.Background(), key, versionID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(value)
	router := gin.New()
	proxy.Register(router.Group("/api/v1/miniapp/media"))

	head := httptest.NewRecorder()
	router.ServeHTTP(head, httptest.NewRequest(http.MethodHead, parsed.RequestURI(), nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 || store.headCalls != 1 || store.openCalls != 0 {
		t.Fatalf("HEAD status=%d body=%d head=%d open=%d", head.Code, head.Body.Len(), store.headCalls, store.openCalls)
	}

	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, parsed.RequestURI(), nil))
	if get.Code != http.StatusOK || get.Body.String() != string(payload) || store.headCalls != 2 || store.openCalls != 1 {
		t.Fatalf("GET status=%d body=%q head=%d open=%d", get.Code, get.Body.String(), store.headCalls, store.openCalls)
	}
	if store.headKey != key || store.headVer != versionID || store.openKey != key || store.openVer != versionID {
		t.Fatal("proxy did not pin HEAD and GET to the token version")
	}
	for name, want := range map[string]string{
		"Content-Type": "image/png", "Content-Length": strconv.Itoa(len(payload)),
		"Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff",
	} {
		if get.Header().Get(name) != want {
			t.Fatalf("header %s=%q want=%q", name, get.Header().Get(name), want)
		}
	}
}
```

再加入表驱动测试，名称必须明确：

```go
func TestWechatReviewProxyRejectsBeforeStorage(t *testing.T)        // missing/extra/tampered/expired/oversized query
func TestWechatReviewProxyRejectsObjectIdentityMIMEAndSizeMismatch(t *testing.T)
func TestWechatReviewProxyMapsNotFoundAndTemporaryStorageErrors(t *testing.T)
func TestWechatReviewProxyRejectsUnsupportedMethods(t *testing.T)
func TestWechatReviewProxyResponsesDoNotLeakSensitiveValues(t *testing.T)
```

这些测试分别断言：无效 token 不调用 store；`ErrObjectNotFound -> 404`；其他 store error 或 nil body -> 503；POST -> 405 且 `Allow: HEAD, GET`；错误 body 为空且不含 token/key/version/error 文本。

- [ ] **Step 2: 运行测试并确认路由尚不存在**

```powershell
go test ./internal/module/media/miniappflow -run 'TestWechatReviewProxy(Uses|Rejects|Maps|Responses)' -count=1
```

Expected: FAIL，错误包含 `proxy.Register undefined` 或请求返回 404。

- [ ] **Step 3: 加入完整 handler 与固定错误映射**

在 `wechat_review_proxy.go` 补充以下实现和 imports：

```go
func (p *WechatReviewProxy) Register(group *gin.RouterGroup) {
	group.Any("/wechat/review-content", p.serve)
}

func (p *WechatReviewProxy) serve(c *gin.Context) {
	if c.Request.Method != http.MethodHead && c.Request.Method != http.MethodGet {
		c.Header("Allow", "HEAD, GET")
		c.Status(http.StatusMethodNotAllowed)
		return
	}
	values := c.Request.URL.Query()
	tokens, ok := values["token"]
	if !ok || len(values) != 1 || len(tokens) != 1 || tokens[0] == "" || len(tokens[0]) > wechatReviewProxyMaxToken {
		c.Status(http.StatusNotFound)
		return
	}
	claims, err := p.openClaims(tokens[0])
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	object, err := p.store.HeadObject(c.Request.Context(), claims.ObjectKey, claims.VersionID)
	if err != nil {
		p.writeStorageError(c, err)
		return
	}
	if object.Key != claims.ObjectKey || object.VersionID != claims.VersionID ||
		!allowedWechatReviewMIME(object.ContentType) || object.Size == 0 || object.Size > wechatReviewProxyMaxBytes {
		c.Status(http.StatusNotFound)
		return
	}
	if c.Request.Method == http.MethodHead {
		writeWechatReviewContentHeaders(c, object)
		c.Status(http.StatusOK)
		return
	}
	body, err := p.store.OpenObject(c.Request.Context(), claims.ObjectKey, claims.VersionID)
	if err != nil || body == nil {
		if body != nil {
			_ = body.Close()
		}
		if err == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		p.writeStorageError(c, err)
		return
	}
	defer body.Close()
	writeWechatReviewContentHeaders(c, object)
	c.Status(http.StatusOK)
	_, _ = io.CopyN(c.Writer, body, int64(object.Size))
}

func (p *WechatReviewProxy) writeStorageError(c *gin.Context, err error) {
	if errors.Is(err, coremedia.ErrObjectNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	c.Status(http.StatusServiceUnavailable)
}

func writeWechatReviewContentHeaders(c *gin.Context, object coremedia.CompletionObject) {
	c.Header("Content-Type", object.ContentType)
	c.Header("Content-Length", strconv.FormatUint(object.Size, 10))
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
}

func allowedWechatReviewMIME(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}
```

不得调用 `c.Error(err)`、不得在错误 body 或日志中写底层错误、token、key 或 version。

- [ ] **Step 4: 运行代理全套测试与 vet**

```powershell
gofmt -w internal/module/media/miniappflow/wechat_review_proxy.go internal/module/media/miniappflow/wechat_review_proxy_test.go
go test ./internal/module/media/miniappflow -run 'TestWechatReviewProxy' -count=1
go vet ./internal/module/media/miniappflow
```

Expected: 全部 PASS，HEAD 的 `openCalls=0`，GET 字节与固定版本完全一致。

- [ ] **Step 5: 精确提交 Task 2**

```powershell
git add -- backend/internal/module/media/miniappflow/wechat_review_proxy.go backend/internal/module/media/miniappflow/wechat_review_proxy_test.go
git diff --cached --check
git commit -m "feat(media): 提供微信审核私有内容代理"
```

### Task 3: 接入 composition root 并封闭代理查询日志

**Files:**
- Modify: `backend/cmd/server/main.go`
- Modify: `backend/cmd/server/router_logging_test.go`
- Modify: `backend/internal/module/media/miniappflow/boundary_test.go`
- Modify: `backend/internal/module/media/miniappflow/wechat_review_test.go`

- [ ] **Step 1: 先扩展日志、边界和 TTL 测试**

在 `router_logging_test.go` 的表格增加 `method` 字段，并加入：

```go
{
	name: "review proxy get", method: http.MethodGet,
	target: "/api/v1/miniapp/media/wechat/review-content?token=proxy-secret",
	wantLogged: false,
},
{
	name: "review proxy head", method: http.MethodHead,
	target: "/api/v1/miniapp/media/wechat/review-content?token=proxy-secret",
	wantLogged: false,
},
{
	name: "similar review proxy path", method: http.MethodGet,
	target: "/api/v1/miniapp/media/wechat/review-content/near?token=ordinary-marker",
	marker: "token=ordinary-marker", wantLogged: true,
},
```

构造 request 时使用 `test.method`；现有未设置 method 的 case 统一补 `http.MethodGet`。Recovery 表再加精确代理 panic case，并断言 `proxy-secret`、`token=` 和 recovered 值都不出现。

在 `boundary_test.go` 增加 source assertion：公共 `publicMedia := api.Group("/miniapp/media")` 同时注册 callback/proxy，而带 `AuthRequired()+UnmutedRequired()` 的 frozen group 仍存在。

在 `wechat_review_test.go` 增加：

```go
func TestWechatReviewURLTTLAllowsOfficialCallbackWindow(t *testing.T) {
	if WechatReviewURLTTL != 35*time.Minute {
		t.Fatalf("ttl=%s want=35m", WechatReviewURLTTL)
	}
}
```

- [ ] **Step 2: 运行测试确认当前接线与 SkipPaths 不满足**

```powershell
go test ./cmd/server -run 'TestRouter' -count=1
go test ./internal/module/media/miniappflow -run 'TestModule10Boundary|TestWechatReviewURLTTL' -count=1
```

Expected: 至少一项 FAIL；精确代理 query 仍进入 Gin access log，composition source 尚无 proxy 注册。

- [ ] **Step 3: 构造真实模式代理，避免 typed-nil signer**

在 `main.go` 用独立接口变量接线，禁止把 nil `*WechatReviewProxy` 直接装入接口：

```go
var wechatReviewProxy *miniappflow.WechatReviewProxy
var wechatReviewSigner miniappflow.WechatReviewURLSigner
if !localMockWechatMode(cfg) {
	wechatReviewProxy, err = miniappflow.NewWechatReviewProxy(
		miniappflow.WechatReviewProxyConfig{
			PublicBaseURL: cfg.MediaPublicBaseURL,
			AppSecret:     cfg.WechatAppSecret,
		},
		completionStore,
	)
	if err != nil {
		log.Fatalf("configure Module 10 WeChat review content proxy: %v", err)
	}
	wechatReviewSigner = wechatReviewProxy
}

wechatReviewSubmitter, err := newWechatReviewSubmitter(cfg, wechatReviewSigner)
if err != nil {
	log.Fatalf("configure Module 10 WeChat image review: %v", err)
}
```

`newWechatReviewSubmitter` 的 local mock 早退保持不变；真实模式收到 nil signer 必须由 `NewWechatAsyncReviewSubmitter` 返回错误。

路由改为复用同一个公共 group：

```go
publicMedia := api.Group("/miniapp/media")
miniappflow.NewWechatCallbackHandler(miniappMedia, cfg.WechatCallbackToken).Register(publicMedia)
if wechatReviewProxy != nil {
	wechatReviewProxy.Register(publicMedia)
}

miniappMediaRoutes := api.Group("/miniapp/media", userService.AuthRequired(), user.UnmutedRequired())
```

- [ ] **Step 4: 把精确代理路径加入 Gin SkipPaths**

在 `newRouter` 现有两条 callback 后增加：

```go
miniappflow.WechatReviewContentPath,
```

不得改成 prefix skip；近似路径必须继续记录。Nginx 已使用 `$uri`，本任务不修改 Nginx。

- [ ] **Step 5: 运行组合测试并精确提交**

```powershell
gofmt -w cmd/server/main.go cmd/server/router_logging_test.go internal/module/media/miniappflow/boundary_test.go internal/module/media/miniappflow/wechat_review_test.go
go test ./cmd/server -run 'TestRouter' -count=1
go test ./internal/module/media/miniappflow -run 'TestModule10Boundary|TestWechatReviewURLTTL|TestWechatAsyncReview' -count=1
go vet ./cmd/server ./internal/module/media/miniappflow
git add -- backend/cmd/server/main.go backend/cmd/server/router_logging_test.go backend/internal/module/media/miniappflow/boundary_test.go backend/internal/module/media/miniappflow/wechat_review_test.go
git diff --cached --check
git commit -m "feat(server): 接入微信审核内容代理"
```

Expected: PASS；只提交上列四个文件。

### Task 4: 在服务层幂等记录微信平台审核失败

**Files:**
- Modify: `backend/internal/module/media/miniappflow/wechat_review.go`
- Modify: `backend/internal/module/media/miniappflow/wechat_review_test.go`
- Modify: `backend/internal/module/media/miniappflow/service.go`
- Modify: `backend/internal/module/media/miniappflow/service_test.go`

- [ ] **Step 1: 先写封闭失败码、hash 与服务状态迁移测试**

在 `wechat_review_test.go` 增加：

```go
func TestWechatFailurePayloadHashBindsAllInputs(t *testing.T) {
	base := WechatFailurePayloadHash("trace-1", wechatMediaDownloadFailureCode, 42, 1)
	if base == "" || base != WechatFailurePayloadHash("trace-1", wechatMediaDownloadFailureCode, 42, 1) {
		t.Fatal("failure payload hash must be deterministic")
	}
	values := []string{
		WechatFailurePayloadHash("trace-2", wechatMediaDownloadFailureCode, 42, 1),
		WechatFailurePayloadHash("trace-1", wechatMediaPlatformFailureCode, 42, 1),
		WechatFailurePayloadHash("trace-1", wechatMediaDownloadFailureCode, 43, 1),
		WechatFailurePayloadHash("trace-1", wechatMediaDownloadFailureCode, 42, 2),
	}
	for _, value := range values {
		if value == base {
			t.Fatal("failure payload hash did not bind every input")
		}
	}
}

func TestWechatMediaFailureCodesAreSealed(t *testing.T) {
	for _, value := range []string{wechatMediaDownloadFailureCode, wechatMediaPlatformFailureCode} {
		if !validWechatMediaFailureCode(value) {
			t.Fatalf("sealed code rejected: %q", value)
		}
	}
	for _, value := range []string{"", "-1008", "wechat_unknown_suggest", " wechat_media_download_error"} {
		if validWechatMediaFailureCode(value) {
			t.Fatalf("unsealed code accepted: %q", value)
		}
	}
}
```

在 `service_test.go` 复用现有 `projectionStub/outboxStub/consumerStub`，增加以下四组测试：

```go
func TestApplyWechatReviewFailureRecordsDownloadErrorFromSubmitted(t *testing.T)
func TestApplyWechatReviewFailureRecordsPlatformErrorFromReviewing(t *testing.T)
func TestApplyWechatReviewFailureIgnoresTerminalProjection(t *testing.T)
func TestApplyWechatReviewFailureRejectsUnknownReason(t *testing.T)
```

前两组必须各自产生且只产生一个 `ConsumerOperationDecision`，解码 payload 后逐项断言：

```go
if decision.Decision != coremedia.ProjectionReviewFailed ||
	decision.ResultCode != failureCode ||
	decision.ReviewTaskID != reviewTaskID ||
	decision.ExpectedStatus != startingStatus ||
	decision.DecisionVersion != 1 || decision.PayloadHash == "" {
	t.Fatalf("unexpected failure decision: %#v", decision)
}
```

`submitted` 与 `reviewing` 都直接进入已有的 `review_failed` transition，不要先追加 reviewing event；这覆盖平台错误早于 reviewing consumer 完成的竞态。consumer stub 在消费 decision 后把 projection 改为 `ProjectionReviewFailed`，再次应用同一失败必须保持 outbox 数量和 consumer 调用数不变。终态表至少覆盖 `approved/rejected/review_failed`，断言 outbox 为空且 consumer 未调用。非法原因必须返回 `ErrInvalidRequest`，且在查 projection 之前退出。

- [ ] **Step 2: 运行测试确认新服务能力尚不存在**

```powershell
Set-Location 'C:\Users\33141\Documents\New project\backend'
go test ./internal/module/media/miniappflow -run 'TestWechatFailure|TestWechatMediaFailure|TestApplyWechatReviewFailure' -count=1
```

Expected: FAIL，缺少封闭失败码、hash helper 和 `ApplyWechatReviewFailure`。

- [ ] **Step 3: 增加封闭失败码和确定性 payload hash**

在 `wechat_review.go` 增加；常量保持包内私有，禁止把微信原始 `errcode` 或 `errmsg` 写入持久层：

```go
const (
	wechatMediaDownloadFailureCode = "wechat_media_download_error"
	wechatMediaPlatformFailureCode = "wechat_media_platform_error"
)

func validWechatMediaFailureCode(value string) bool {
	switch value {
	case wechatMediaDownloadFailureCode, wechatMediaPlatformFailureCode:
		return true
	default:
		return false
	}
}

func WechatFailurePayloadHash(traceID, failureCode string, reviewTaskID, decisionVersion uint64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf(
		"wechat-failure:%s:%s:%d:%d:%s",
		strings.TrimSpace(traceID), failureCode, reviewTaskID, decisionVersion,
		coremedia.ProjectionReviewFailed,
	)))
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: 实现 `ApplyWechatReviewFailure` 并复用现有 projection/outbox consumer**

在 `service.go` 紧邻 `ApplyWechatReviewResult` 增加：

```go
func (s *Service) ApplyWechatReviewFailure(ctx context.Context, traceID, failureCode string) error {
	if !validWechatMediaFailureCode(failureCode) {
		return ErrInvalidRequest
	}
	reviewTaskID, err := WechatReviewTaskID(traceID)
	if err != nil {
		return err
	}
	projection, err := s.projections.FindByReviewTaskID(ctx, reviewTaskID)
	if err != nil {
		return err
	}
	if projection.Status.Terminal() {
		return nil
	}
	if (projection.Status != coremedia.ProjectionSubmitted &&
		projection.Status != coremedia.ProjectionReviewing) ||
		projection.ReviewTaskID == nil ||
		*projection.ReviewTaskID != reviewTaskID {
		return coremedia.ErrReviewInvariant
	}
	if err := s.appendWechatFailureDecision(ctx, projection, traceID, failureCode); err != nil {
		return err
	}
	return s.consumeProjectionEvent(ctx)
}
```

再增加唯一的新事件 helper：

```go
func (s *Service) appendWechatFailureDecision(
	ctx context.Context,
	projection coremedia.ReviewProjection,
	traceID, failureCode string,
) error {
	if projection.ReviewTaskID == nil || *projection.ReviewTaskID == 0 ||
		!validWechatMediaFailureCode(failureCode) {
		return coremedia.ErrReviewInvariant
	}
	decisionVersion := projection.AppliedDecisionVersion + 1
	payload, err := json.Marshal(coremedia.ConsumerEventPayload{
		ProjectionID: projection.ProjectionID,
		ReviewTaskID: *projection.ReviewTaskID,
		ExpectedRowVersion: projection.RowVersion,
		ExpectedStatus: projection.Status,
		DecisionVersion: decisionVersion,
		Decision: coremedia.ProjectionReviewFailed,
		PayloadHash: WechatFailurePayloadHash(
			traceID, failureCode, *projection.ReviewTaskID, decisionVersion,
		),
		ResultCode: failureCode,
	})
	if err != nil {
		return err
	}
	return s.appendProjectionEvent(ctx, projection, coremedia.ConsumerOperationDecision, payload)
}
```

不修改 projection reducer、outbox consumer、repository 或迁移；现有 `ConsumerOperationDecision` 已能把上述 payload 投影为 `review_failed`。

- [ ] **Step 5: 运行服务测试、静态检查并精确提交**

```powershell
Set-Location 'C:\Users\33141\Documents\New project\backend'
gofmt -w internal/module/media/miniappflow/wechat_review.go internal/module/media/miniappflow/wechat_review_test.go internal/module/media/miniappflow/service.go internal/module/media/miniappflow/service_test.go
go test ./internal/module/media/miniappflow -run 'TestWechatFailure|TestWechatMediaFailure|TestApplyWechatReviewFailure|TestApplyWechatReviewResult' -count=1
go vet ./internal/module/media/miniappflow
Set-Location 'C:\Users\33141\Documents\New project'
git add -- backend/internal/module/media/miniappflow/wechat_review.go backend/internal/module/media/miniappflow/wechat_review_test.go backend/internal/module/media/miniappflow/service.go backend/internal/module/media/miniappflow/service_test.go
git diff --cached --check
git commit -m "fix(media): 记录微信审核平台失败"
```

Expected: PASS；只提交这四个文件。Task 1/3 对 `wechat_review.go` 和测试的改动会与本任务一起位于精确 staged diff 中，不得带入其他工作区文件。

### Task 5: 严格解析并分流微信官方回调信封

**Files:**
- Modify: `backend/internal/module/media/miniappflow/wechat_callback_handler.go`
- Modify: `backend/internal/module/media/miniappflow/wechat_callback_handler_test.go`
- Verify only: `backend/internal/module/media/miniappflow/wechat_callback_diagnostic.go`
- Verify only: `backend/internal/module/media/miniappflow/wechat_callback_diagnostic_test.go`

- [ ] **Step 1: 先增加官方成功、平台失败、无关推送和歧义输入测试**

在 `wechat_callback_handler_test.go` 扩展 recording stub：

```go
type recordingWechatReviewApplier struct {
	traceID, suggest string
	label            int
	failureTraceID   string
	failureCode      string
	resultCalls      int
	failureCalls     int
	err              error
}

func (value *recordingWechatReviewApplier) ApplyWechatReviewFailure(
	_ context.Context, traceID, failureCode string,
) error {
	value.failureCalls++
	value.failureTraceID = traceID
	value.failureCode = failureCode
	return value.err
}
```

把现有 `ApplyWechatReviewResult` stub 增加 `resultCalls++`。然后新增表驱动测试：

```go
func TestWechatReviewCallbackAppliesOfficialSuccessJSONAndXML(t *testing.T)
func TestWechatReviewCallbackMapsDownloadAndPlatformErrors(t *testing.T)
func TestWechatReviewCallbackIgnoresUnrelatedPushes(t *testing.T)
func TestWechatReviewCallbackRejectsAmbiguousOfficialEnvelope(t *testing.T)
func TestWechatReviewCallbackReturns500WhenFailureApplicationFails(t *testing.T)
func TestWechatReviewCallbackKeepsLegacyMinimalSuccessEnvelope(t *testing.T)
```

至少使用这些 payload：

```json
{"MsgType":"event","Event":"wxa_media_check","errcode":0,"trace_id":"trace-ok","result":{"suggest":"pass","label":100}}
{"MsgType":"event","Event":"wxa_media_check","errcode":-1008,"trace_id":"trace-download"}
{"MsgType":"event","Event":"wxa_media_check","errcode":-1001,"trace_id":"trace-platform"}
{"MsgType":"text","Content":"hello"}
{"MsgType":"event","Event":"subscribe","FromUserName":"opaque"}
```

以及 XML 等价物。断言矩阵：

| 输入 | HTTP | result calls | failure calls | 失败码 |
|---|---:|---:|---:|---|
| 官方 `errcode=0` + 完整 result | 200 | 1 | 0 | — |
| 官方 `errcode=-1008`，无 result | 200 | 0 | 1 | `wechat_media_download_error` |
| 官方其他非零 errcode | 200 | 0 | 1 | `wechat_media_platform_error` |
| 普通消息或其他 Event | 200 | 0 | 0 | — |
| service failure | 500 | 0 | 1 | 封闭码 |
| 旧 `trace_id + result` | 200 | 1 | 0 | — |

歧义/恶意输入表必须覆盖 JSON 与 XML 中 `MsgType`、`Event`、`errcode`、`Encrypt` 的重复字段，错误标量类型，`errcode` 的浮点/指数/字符串形式，`Encrypt` envelope，以及 `Event=wxa_media_check` 但 `MsgType!=event`。这些输入都必须 400 且 service 调用为 0。响应与测试日志不得含 trace、原始 errcode、Encrypt 内容或 body。

- [ ] **Step 2: 运行回调测试确认官方 envelope 尚未被支持**

```powershell
Set-Location 'C:\Users\33141\Documents\New project\backend'
go test ./internal/module/media/miniappflow -run 'TestWechatReviewCallback' -count=1
```

Expected: 新测试 FAIL；当前解析器只理解旧 `trace_id/result` 信封。

- [ ] **Step 3: 增加 envelope 与 disposition 类型，保留旧 decoder 入口**

在 handler 中改为：

```go
type wechatReviewResultApplier interface {
	ApplyWechatReviewResult(context.Context, string, string, int) error
	ApplyWechatReviewFailure(context.Context, string, string) error
}

type wechatCallbackDisposition uint8

const (
	wechatCallbackApplyResult wechatCallbackDisposition = iota
	wechatCallbackApplyFailure
	wechatCallbackIgnore
)

type wechatReviewResult struct {
	MsgType  *string
	Event    *string
	ErrCode  *int64
	Encrypted bool
	TraceID  string
	Result struct {
		Suggest string
		Label   *wechatReviewLabel
	}
}

type decodedWechatCallback struct {
	Disposition wechatCallbackDisposition
	Value       wechatReviewResult
	FailureCode string
}
```

`decodeWechatReviewResult` 继续作为旧测试和诊断代码的“只接受成功 result”兼容入口：

```go
func decodeWechatReviewResult(body []byte) (wechatReviewResult, error) {
	decoded, err := decodeWechatReviewCallback(body)
	if err != nil || decoded.Disposition != wechatCallbackApplyResult {
		return wechatReviewResult{}, errInvalidWechatReviewCallback
	}
	return decoded.Value, nil
}
```

- [ ] **Step 4: 扩展 JSON/XML tokenizer，严格检测新增协议字段**

把当前 JSON 和 XML decoder 拆为 `decodeWechatReviewEnvelope`，只负责结构解析，不在其中强制 `result` 存在。外层已存在的 duplicate-field tokenizer 保留，并为四个新字段分别增加 `seen` 标志：

```go
case "MsgType":
	if seenMsgType { return wechatReviewResult{}, errInvalidWechatReviewCallback }
	seenMsgType = true
	value.MsgType = new(string)
	*value.MsgType, err = decodeWechatJSONString(decoder)
case "Event":
	if seenEvent { return wechatReviewResult{}, errInvalidWechatReviewCallback }
	seenEvent = true
	value.Event = new(string)
	*value.Event, err = decodeWechatJSONString(decoder)
case "errcode":
	if seenErrCode { return wechatReviewResult{}, errInvalidWechatReviewCallback }
	seenErrCode = true
	var parsed int64
	parsed, err = decodeWechatJSONInt64(decoder)
	if err == nil { value.ErrCode = &parsed }
case "Encrypt":
	if seenEncrypt { return wechatReviewResult{}, errInvalidWechatReviewCallback }
	seenEncrypt = true
	_, err = decodeWechatJSONString(decoder)
	value.Encrypted = err == nil
```

整数 helper 只接受可选负号加十进制数字；拒绝正号、小数、指数、字符串、布尔值、null 与溢出：

```go
func parseWechatSignedInteger(raw string) (int64, error) {
	if raw == "" {
		return 0, errInvalidWechatReviewCallback
	}
	index := 0
	if raw[0] == '-' {
		index = 1
		if len(raw) == 1 {
			return 0, errInvalidWechatReviewCallback
		}
	}
	for ; index < len(raw); index++ {
		if raw[index] < '0' || raw[index] > '9' {
			return 0, errInvalidWechatReviewCallback
		}
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, errInvalidWechatReviewCallback
	}
	return parsed, nil
}

func decodeWechatJSONInt64(decoder *json.Decoder) (int64, error) {
	token, err := decoder.Token()
	if err != nil {
		return 0, errInvalidWechatReviewCallback
	}
	value, ok := token.(json.Number)
	if !ok {
		return 0, errInvalidWechatReviewCallback
	}
	return parseWechatSignedInteger(string(value))
}
```

XML root 使用相同 seen flags；`errcode` 先 `strings.TrimSpace` 再调用 `parseWechatSignedInteger`，`Encrypt` 只允许纯标量并令 `Encrypted=true`。把 `MsgType/Event/errcode/Encrypt` 加入 `isWechatReviewCriticalXMLName`，因此这些字段藏在未知嵌套节点里也会 fail-closed。`result` 内仍只允许唯一的 `suggest/label`，旧有 DTD、directive、processing instruction、嵌套 critical field和尾随根节点拒绝逻辑全部保留。

- [ ] **Step 5: 实现成功、失败、忽略三向分类**

```go
func decodeWechatReviewCallback(body []byte) (decodedWechatCallback, error) {
	value, err := decodeWechatReviewEnvelope(body)
	if err != nil || value.Encrypted {
		return decodedWechatCallback{}, errInvalidWechatReviewCallback
	}
	msgType, event := "", ""
	if value.MsgType != nil { msgType = strings.TrimSpace(*value.MsgType) }
	if value.Event != nil { event = strings.TrimSpace(*value.Event) }

	if value.Event != nil && event != "wxa_media_check" {
		return decodedWechatCallback{Disposition: wechatCallbackIgnore, Value: value}, nil
	}
	if value.MsgType != nil && msgType != "event" {
		if event == "wxa_media_check" {
			return decodedWechatCallback{}, errInvalidWechatReviewCallback
		}
		return decodedWechatCallback{Disposition: wechatCallbackIgnore, Value: value}, nil
	}

	legacy := value.MsgType == nil && value.Event == nil
	if legacy {
		if value.ErrCode != nil || validateWechatReviewResult(value) != nil {
			return decodedWechatCallback{}, errInvalidWechatReviewCallback
		}
		return decodedWechatCallback{Disposition: wechatCallbackApplyResult, Value: value}, nil
	}
	if value.MsgType == nil || value.Event == nil || msgType != "event" ||
		event != "wxa_media_check" || strings.TrimSpace(value.TraceID) == "" {
		return decodedWechatCallback{}, errInvalidWechatReviewCallback
	}
	if value.ErrCode != nil && *value.ErrCode != 0 {
		failureCode := wechatMediaPlatformFailureCode
		if *value.ErrCode == -1008 {
			failureCode = wechatMediaDownloadFailureCode
		}
		return decodedWechatCallback{
			Disposition: wechatCallbackApplyFailure,
			Value: value,
			FailureCode: failureCode,
		}, nil
	}
	if validateWechatReviewResult(value) != nil {
		return decodedWechatCallback{}, errInvalidWechatReviewCallback
	}
	return decodedWechatCallback{Disposition: wechatCallbackApplyResult, Value: value}, nil
}
```

- [ ] **Step 6: 在 HTTP handler 中分流，所有已消费推送统一返回 200**

签名校验与 256 KiB body 上限保持原样。把当前单一路径替换为：

```go
decoded, err := decodeWechatReviewCallback(body)
if err != nil {
	logWechatCallbackRejection(body)
	c.Status(http.StatusBadRequest)
	return
}

switch decoded.Disposition {
case wechatCallbackIgnore:
	structure, category := diagnoseWechatCallback(body)
	logWechatCallbackCategory(structure, category)
	c.String(http.StatusOK, "success")
	return
case wechatCallbackApplyFailure:
	structure, category := diagnoseWechatCallback(body)
	logWechatCallbackCategory(structure, category)
	err = h.service.ApplyWechatReviewFailure(
		c.Request.Context(), decoded.Value.TraceID, decoded.FailureCode,
	)
case wechatCallbackApplyResult:
	err = h.service.ApplyWechatReviewResult(
		c.Request.Context(), decoded.Value.TraceID,
		decoded.Value.Result.Suggest, int(*decoded.Value.Result.Label),
	)
default:
	c.Status(http.StatusBadRequest)
	return
}
if err != nil {
	c.Status(http.StatusInternalServerError)
	return
}
c.String(http.StatusOK, "success")
```

不记录 `trace_id`、原始 `errcode`、body、query 或失败的 service error；平台错误和无关推送继续只写既有封闭 `structure/category` 枚举，平台非零 errcode 不再被当成 malformed callback。

- [ ] **Step 7: 运行回调回归、vet 并精确提交**

```powershell
Set-Location 'C:\Users\33141\Documents\New project\backend'
gofmt -w internal/module/media/miniappflow/wechat_callback_handler.go internal/module/media/miniappflow/wechat_callback_handler_test.go
go test ./internal/module/media/miniappflow -run 'TestWechatReviewCallback|TestDecodeWechatReview|TestWechatCallback|TestApplyWechatReview' -count=1
go test ./internal/module/media/miniappflow -run 'TestWechatCallbackDiagnostic' -count=1
go vet ./internal/module/media/miniappflow
Set-Location 'C:\Users\33141\Documents\New project'
git add -- backend/internal/module/media/miniappflow/wechat_callback_handler.go backend/internal/module/media/miniappflow/wechat_callback_handler_test.go
git diff --cached --check
git commit -m "fix(media): 收口微信审核回调分流"
```

Expected: PASS；diagnostic 回归仍通过，只提交 handler 与测试。平台 `-1008` 现在进入幂等 `review_failed`，不再产生 400；无关消息 200 且 service 调用为 0。

### Task 6: 执行完整验证门禁并构建生产二进制

**Files:**
- Verify: `backend/internal/module/media/miniappflow/**`
- Verify: `backend/cmd/server/**`
- Verify: `backend/migrations/**` 保持零差异
- Build artifact: `.release/campus-api-review-proxy`
- No source changes expected

- [ ] **Step 1: 检查格式、工作区和实现提交边界**

```powershell
$ErrorActionPreference='Stop'
Set-Location 'C:\Users\33141\Documents\New project'
$expected=@(
  'backend/cmd/server/main.go',
  'backend/cmd/server/router_logging_test.go',
  'backend/internal/module/media/miniappflow/boundary_test.go',
  'backend/internal/module/media/miniappflow/service.go',
  'backend/internal/module/media/miniappflow/service_test.go',
  'backend/internal/module/media/miniappflow/wechat_callback_handler.go',
  'backend/internal/module/media/miniappflow/wechat_callback_handler_test.go',
  'backend/internal/module/media/miniappflow/wechat_review.go',
  'backend/internal/module/media/miniappflow/wechat_review_proxy.go',
  'backend/internal/module/media/miniappflow/wechat_review_proxy_test.go',
  'backend/internal/module/media/miniappflow/wechat_review_test.go'
)
$actual=@(git diff --name-only dcb7670..HEAD | Where-Object { $_ -like 'backend/*' })
$unexpected=@($actual | Where-Object { $_ -notin $expected })
if ($unexpected.Count -gt 0) { $unexpected; throw '实现提交越过批准边界' }
if ((git diff --name-only dcb7670..HEAD -- backend/migrations)) { throw '检测到未批准的迁移变更' }
git diff --check dcb7670..HEAD
if ($LASTEXITCODE -ne 0) { throw '提交差异存在空白错误' }

Set-Location 'C:\Users\33141\Documents\New project\backend'
$goFiles=@(
  'cmd/server/main.go',
  'cmd/server/router_logging_test.go',
  'internal/module/media/miniappflow/boundary_test.go',
  'internal/module/media/miniappflow/service.go',
  'internal/module/media/miniappflow/service_test.go',
  'internal/module/media/miniappflow/wechat_callback_handler.go',
  'internal/module/media/miniappflow/wechat_callback_handler_test.go',
  'internal/module/media/miniappflow/wechat_review.go',
  'internal/module/media/miniappflow/wechat_review_proxy.go',
  'internal/module/media/miniappflow/wechat_review_proxy_test.go',
  'internal/module/media/miniappflow/wechat_review_test.go'
)
$unformatted=@(gofmt -l $goFiles)
if ($unformatted.Count -gt 0) { $unformatted; throw 'Go 文件尚未 gofmt' }
```

Expected: 无越界文件、无迁移、无 whitespace/gofmt 问题。原有未提交的 `.env.example`、`.gitignore`、miniapp 私有配置、quickstart 和 `deploy/native/` 文件仍原样保留，不纳入 `$actual` 的已提交实现 diff。

- [ ] **Step 2: 运行代理、回调、服务和 composition 的定向回归**

```powershell
Set-Location 'C:\Users\33141\Documents\New project\backend'
go test ./internal/module/media/miniappflow -run 'TestWechatReviewProxy|TestWechatAsyncReview|TestWechatReviewCallback|TestDecodeWechatReview|TestWechatCallback|TestApplyWechatReview|TestModule10Boundary' -count=1
if ($LASTEXITCODE -ne 0) { throw '微信审核定向回归失败' }
go test ./cmd/server -run 'TestRouter|TestNewWechatReview' -count=1
if ($LASTEXITCODE -ne 0) { throw '服务接线与日志回归失败' }
go test ./cmd/server ./internal/module/media/miniappflow -count=1
if ($LASTEXITCODE -ne 0) { throw '变更包完整回归失败' }
go vet ./cmd/server ./internal/module/media/miniappflow
if ($LASTEXITCODE -ne 0) { throw 'go vet 失败' }
```

Expected: 全部 PASS。任何代理、回调、服务或 composition 测试失败都阻止构建和部署。

- [ ] **Step 3: 运行仓库级门禁并诚实记录共享环境失败**

```powershell
Set-Location 'C:\Users\33141\Documents\New project'
npm run check:boundaries
if ($LASTEXITCODE -ne 0) { throw '边界门禁失败' }
npm run verify
$verifyExit=$LASTEXITCODE
Write-Output "REPO_VERIFY_EXIT=$verifyExit"
Set-Location 'C:\Users\33141\Documents\New project\backend'
go test ./... -count=1
$allGoExit=$LASTEXITCODE
Write-Output "GO_ALL_EXIT=$allGoExit"
```

Expected: 首选 `REPO_VERIFY_EXIT=0` 且 `GO_ALL_EXIT=0`。若完整测试仅因仓库既有、可复现的共享 MySQL 测试数据库隔离问题失败，必须保存原始失败包名与错误，报告为 `targeted code PASS / full shared-DB runtime PENDING`；不得改数据库、删表、清测试数据或把失败写成通过。只允许在 Task 1–5 的定向测试、vet、边界门禁全部 PASS，且失败确认不涉及本次包时继续上线前构建。

- [ ] **Step 4: 构建 Linux amd64 静态二进制并记录不可变身份**

```powershell
$ErrorActionPreference='Stop'
Set-Location 'C:\Users\33141\Documents\New project'
New-Item -ItemType Directory -Force -Path '.release' | Out-Null
$artifact=(Resolve-Path '.release').Path + '\campus-api-review-proxy'
$commit=(git rev-parse HEAD).Trim()
if ($commit -notmatch '^[0-9a-f]{40}$') { throw 'Git commit 非法' }
Set-Location '.\backend'
$env:CGO_ENABLED='0'
$env:GOOS='linux'
$env:GOARCH='amd64'
go build -trimpath -ldflags='-s -w' -o $artifact .\cmd\server
if ($LASTEXITCODE -ne 0) { throw 'API 编译失败' }
$file=Get-Item -LiteralPath $artifact
$sha=(Get-FileHash -LiteralPath $artifact -Algorithm SHA256).Hash.ToLowerInvariant()
if ($file.Length -le 0 -or $sha -notmatch '^[0-9a-f]{64}$') { throw '构建产物无效' }
Write-Output "BUILD_OK commit=$commit bytes=$($file.Length) sha256=$sha artifact=$artifact"
```

Expected: `BUILD_OK`；记录 commit、字节数和 SHA256。构建过程不读取、不输出 AppSecret、callback Token、COS 密钥或服务器环境文件。

### Task 7: 以唯一部署 ID 安全替换生产 API

**Files:**
- Local artifact: `.release/campus-api-review-proxy`
- Remote stage: `/home/admin/.campus-api-review-proxy-<deploy-id>`
- Remote backup: `/opt/cutover-backups/review-proxy-<deploy-id>/campus-api`
- Replace: `/opt/campus-circle/current/bin/campus-api`
- No Nginx, environment, database, Meilisearch, COS or WeChat platform changes

- [ ] **Step 1: 生成唯一部署 ID，确认 stage 不存在后上传**

以下步骤在同一个 PowerShell 会话执行，并保留 `$deployId/$stage/$localSha` 供后续步骤使用：

```powershell
$ErrorActionPreference='Stop'
Set-Location 'C:\Users\33141\Documents\New project'
$key='C:\Users\33141\.ssh\id_ed25519_zhandui'
$target='admin@8.138.227.107'
$artifact=(Resolve-Path '.release\campus-api-review-proxy').Path
$localSha=(Get-FileHash -LiteralPath $artifact -Algorithm SHA256).Hash.ToLowerInvariant()
$deployId="$(Get-Date -Format yyyyMMdd_HHmmss)-$([guid]::NewGuid().ToString('N').Substring(0,12))"
if ($deployId -notmatch '^[0-9]{8}_[0-9]{6}-[0-9a-f]{12}$') { throw '部署 ID 非法' }
$stage="/home/admin/.campus-api-review-proxy-$deployId"

$absent="set -euo pipefail; test ! -e '$stage'; echo STAGE_PATH_READY"
$encoded=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($absent))
ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes $target "echo '$encoded' | base64 -d | bash"
if ($LASTEXITCODE -ne 0) { throw '唯一 stage 已存在，重新生成部署 ID' }

scp -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -- $artifact "${target}:$stage"
if ($LASTEXITCODE -ne 0) { throw '上传生产二进制失败' }
```

Expected: `STAGE_PATH_READY`，随后 scp 成功。禁止上传到 `/tmp` 或复用旧 stage；stage 名必须与本次 `$deployId` 一一对应。

- [ ] **Step 2: 将 stage 封为 0600 并执行只读生产预检**

```powershell
$preflight=@'
set -euo pipefail
stage='__STAGE__'
expected_new='__NEW_SHA__'
expected_current='1c97361908ce891864887e52fa32ab6a883634117f89ec5794d9f15eb5a83132'

exec 9>/run/lock/campus-api-deploy.lock
flock -n 9

test -f "$stage"
test ! -L "$stage"
test "$(stat -c %U "$stage")" = admin
chmod 0600 "$stage"
test "$(stat -c %a "$stage")" = 600
test "$(sha256sum "$stage" | awk '{print $1}')" = "$expected_new"

release="$(readlink -f /opt/campus-circle/current)"
case "$release" in /opt/campus-circle/releases/*/*) exit 31 ;; /opt/campus-circle/releases/*) ;; *) exit 31 ;; esac
current="$release/bin/campus-api"
test -f "$current"
test ! -L "$current"
test "$(sha256sum "$current" | awk '{print $1}')" = "$expected_current"

for unit in campus-api nginx mysql meilisearch; do
  systemctl is-active --quiet "$unit"
done
curl -fsS --max-time 5 http://127.0.0.1:8080/healthz >/dev/null
nginx -T 2>&1 | grep -F '"$request_method $uri $server_protocol"' >/dev/null
echo "PREFLIGHT_OK current=$release current_sha=$expected_current stage=$stage new_sha=$expected_new"
'@
$preflight=$preflight.Replace('__STAGE__',$stage).Replace('__NEW_SHA__',$localSha)
$encoded=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($preflight))
ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes $target "echo '$encoded' | base64 -d | sudo -n bash"
if ($LASTEXITCODE -ne 0) { throw '生产预检失败；保持现状，不替换二进制' }
```

Expected: `PREFLIGHT_OK`，current 仍解析为 `/opt/campus-circle/releases/90b62c320598`，旧 SHA 与诊断版完全一致，Nginx 日志格式仍使用 `$uri`。若 current/SHA 已漂移，停止并重新审计，不得覆盖未知版本。

- [ ] **Step 3: 创建本次精确备份、原子替换并在错误时自动回滚**

```powershell
$remote=@'
set -eEuo pipefail
deploy_id='__DEPLOY_ID__'
stage='__STAGE__'
expected_new='__NEW_SHA__'
expected_current='1c97361908ce891864887e52fa32ab6a883634117f89ec5794d9f15eb5a83132'
release="$(readlink -f /opt/campus-circle/current)"
case "$release" in /opt/campus-circle/releases/*/*) exit 41 ;; /opt/campus-circle/releases/*) ;; *) exit 41 ;; esac
current="$release/bin/campus-api"
backup="/opt/cutover-backups/review-proxy-$deploy_id"
candidate="$current.new-$deploy_id"

exec 9>/run/lock/campus-api-deploy.lock
flock -n 9

test -f "$stage" && test ! -L "$stage"
test "$(stat -c %U "$stage")" = admin
test "$(stat -c %a "$stage")" = 600
test "$(sha256sum "$stage" | awk '{print $1}')" = "$expected_new"
test "$(sha256sum "$current" | awk '{print $1}')" = "$expected_current"
test ! -e "$backup"
test ! -e "$candidate"

install -d -o root -g root -m 0700 "$backup"
cp -a -- "$current" "$backup/campus-api"
systemctl status campus-api --no-pager >"$backup/campus-api.status.before.txt"
sha256sum "$backup/campus-api" >"$backup/campus-api.sha256.before"
backup_ready=1
printf 'deploy_id=%s\nrelease=%s\nbefore_sha=%s\nartifact_sha=%s\n' "$deploy_id" "$release" "$expected_current" "$expected_new" >"$backup/manifest.txt"
chmod 0600 "$backup/manifest.txt"
: >"$backup/READY"
chmod 0600 "$backup/READY"

rollback() {
  rc=$?
  trap - ERR
  set +e
  rm -f -- "$candidate"
  if [ "$backup_ready" -eq 1 ]; then
    live_sha="$(sha256sum "$current" 2>/dev/null | awk '{print $1}')"
    if [ "$live_sha" = "$expected_new" ]; then
      install -o root -g campus -m 0750 "$backup/campus-api" "$current.rollback-$deploy_id"
      mv -fT -- "$current.rollback-$deploy_id" "$current"
      systemctl restart campus-api
    else
      echo "ROLLBACK_REFUSED reason=live_sha_cas" >&2
      exit "$rc"
    fi
    for _ in $(seq 1 45); do
      systemctl is-active --quiet campus-api && \
        curl -fsS --max-time 2 http://127.0.0.1:8080/healthz >/dev/null && break
      sleep 1
    done
    if systemctl is-active --quiet campus-api && \
      curl -fsS --max-time 5 http://127.0.0.1:8080/healthz >/dev/null; then
      echo "ROLLBACK_OK deploy_id=$deploy_id backup=$backup"
    else
      echo "ROLLBACK_FAILED deploy_id=$deploy_id backup=$backup" >&2
    fi
  fi
  exit "$rc"
}
trap rollback ERR

install -o root -g campus -m 0750 "$stage" "$candidate"
test "$(sha256sum "$candidate" | awk '{print $1}')" = "$expected_new"
mv -fT -- "$candidate" "$current"
systemctl restart campus-api
ready=0
for _ in $(seq 1 45); do
  if systemctl is-active --quiet campus-api && \
    curl -fsS --max-time 2 http://127.0.0.1:8080/healthz >/dev/null; then
    ready=1
    break
  fi
  sleep 1
done
test "$ready" -eq 1

disk_sha="$(sha256sum "$current" | awk '{print $1}')"
pid="$(systemctl show campus-api -p MainPID --value)"
case "$pid" in ''|*[!0-9]*) exit 42 ;; esac
test "$pid" -gt 1
process_sha="$(sha256sum "/proc/$pid/exe" | awk '{print $1}')"
test "$disk_sha" = "$expected_new"
test "$process_sha" = "$expected_new"
for unit in campus-api nginx mysql meilisearch; do
  systemctl is-active --quiet "$unit"
done
curl -fsS --max-time 5 http://127.0.0.1:8080/healthz >/dev/null
systemctl status campus-api --no-pager >"$backup/campus-api.status.after.txt"
printf '%s  %s\n' "$disk_sha" "$current" >"$backup/campus-api.sha256.after"
trap - ERR
echo "DEPLOY_OK deploy_id=$deploy_id backup=$backup stage=$stage release=$release sha256=$disk_sha"
'@
$remote=$remote.Replace('__DEPLOY_ID__',$deployId).Replace('__STAGE__',$stage).Replace('__NEW_SHA__',$localSha)
$encoded=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($remote))
$deployOutput=ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes $target "echo '$encoded' | base64 -d | sudo -n bash"
$deployOutput
if ($LASTEXITCODE -ne 0) { throw '部署失败；检查上方 ROLLBACK_OK/ROLLBACK_FAILED 后停止验收' }
if (($deployOutput -join "`n") -notmatch [regex]::Escape("DEPLOY_OK deploy_id=$deployId")) { throw '缺少部署成功凭据' }
```

Expected: 只出现本次 `DEPLOY_OK deploy_id=... backup=... stage=...`；磁盘和进程 SHA 都等于 `$localSha`。stage 暂时保留到 Task 8 验收通过；备份永久保留。本步骤不使用“最新备份”发现逻辑，失败回滚只引用本次 `$backup`。

- [ ] **Step 4: 验证公网基础能力且不触发业务写入**

```powershell
$health=Invoke-RestMethod -Uri 'https://zhanduiwang.top/healthz' -TimeoutSec 20
$products=Invoke-RestMethod -Uri 'https://zhanduiwang.top/api/v1/trade/products?page=1&pageSize=1' -TimeoutSec 20
if ($health.code -ne 0 -or $health.data.status -ne 'ok') { throw '公网健康检查失败' }
if ($products.code -ne 0) { throw '公开商品列表失败' }
$fatal=ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes $target "sudo -n journalctl -u campus-api --since '5 minutes ago' --no-pager | grep -Ei 'panic|fatal|bind: address|permission denied' || true"
if (($fatal -join "`n").Trim() -ne '') { $fatal; throw '新 API 日志出现致命错误' }
Write-Output 'PUBLIC_SMOKE_OK'
```

Expected: `PUBLIC_SMOKE_OK`。若失败，使用 Task 7 本次输出的精确 backup 路径执行同一原子恢复逻辑；不得通过搜索“最新目录”选择备份。

### Task 8: 真实媒体验收与最终收口

**Files:**
- Runtime only: 微信开发者工具 `ws://127.0.0.1:9420` 与 `.release/automator/node_modules/miniprogram-automator`
- Runtime receipt: `.release/review-proxy-acceptance-window.json`
- Runtime result: `.release/automator/review-proxy-<deploy-id>.json`
- No source changes；只允许删除本次收据指定的 staging，永久保留 exact backup。

- [ ] **Step 1: 启动导入项目并记录新日志窗口**

```powershell
$ErrorActionPreference='Stop'
& 'D:\3314\微信web开发者工具\cli.bat' auto --project 'C:\Users\33141\Documents\New project\apps\miniapp' --auto-port 9420 --trust-project --lang zh
if ($LASTEXITCODE -ne 0) { throw '微信开发者工具自动化启动失败' }
Set-Location 'C:\Users\33141\Documents\New project'
$deploy=Get-Content -Raw '.release\review-proxy-deploy.json' | ConvertFrom-Json
if ($deploy.deployed -ne $true) { throw '部署收据不是成功状态' }
$epoch=[DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$key='C:\Users\33141\.ssh\id_ed25519_zhandui'
$line=(ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes admin@8.138.227.107 "sudo -n wc -l < /var/log/nginx/campus.access.log").Trim()
if ($line -notmatch '^\d+$') { throw 'Nginx 起始行号非法' }
New-Item -ItemType Directory -Force '.release\automator' | Out-Null
[ordered]@{deployId=[string]$deploy.deployId; acceptanceEpoch=$epoch; nginxStartLine=[int64]$line; resultPath=".release/automator/review-proxy-$($deploy.deployId).json"} | ConvertTo-Json | Set-Content -Encoding UTF8 '.release\review-proxy-acceptance-window.json'
Write-Output "ACCEPTANCE_WINDOW_READY epoch=$epoch nginx_line=$line"
```

不得读取或打印 `domainA.token`、AppSecret、签名 URL；窗口必须晚于本次部署，不能复用先前 `-1008` 资产。

- [ ] **Step 2: 使用全新 PNG 执行真实授权、COS PUT、complete 和轮询**

用 `D:\3314\微信web开发者工具\node-18.exe` 运行下面的 Node 脚本。脚本只在内存中使用 `domainA.token`，输出状态和 ULID，不写入 token 或 URL：

```javascript
const crypto = require('crypto');
const fs = require('fs');
const automator = require('./.release/automator/node_modules/miniprogram-automator');
const image = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=', 'base64');
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
async function main() {
  const output = process.env.REVIEW_PROXY_ACCEPTANCE_OUTPUT;
  const app = await automator.connect({wsEndpoint: 'ws://127.0.0.1:9420'});
  try {
    const token = await app.callWxMethod('getStorageSync', 'domainA.token');
    if (typeof token !== 'string' || token.length < 32) throw Error('AUTH_TOKEN_NOT_AVAILABLE');
    const base = 'https://zhanduiwang.top/api/v1';
    async function api(path, options = {}) {
      const response = await fetch(base + path, {...options, headers: {'content-type': 'application/json', Authorization: `Bearer ${token}`, ...(options.headers || {})}});
      const body = await response.json();
      if (!response.ok || body.code !== 0) throw Error(`API_FAILED ${path} ${response.status} ${body.code}`);
      return body.data;
    }
    await api('/auth/me');
    const session = await api('/miniapp/media/uploads', {method: 'POST', body: JSON.stringify({purpose: 'product', sizeBytes: image.length, contentMD5: crypto.createHash('md5').update(image).digest('base64'), contentType: 'image/png', idempotencyKey: `review_proxy:${Date.now()}:${crypto.randomBytes(6).toString('hex')}`})});
    console.log(`AUTHORIZE_OK asset=${session.assetId}`);
    const put = await fetch(session.uploadUrl, {method: 'PUT', body: image, headers: {'Content-Type': session.requiredHeaders.contentType, 'Content-MD5': session.requiredHeaders.contentMD5, 'Content-Length': String(session.requiredHeaders.contentLength), 'x-cos-forbid-overwrite': session.requiredHeaders.forbidOverwrite}});
    if (!put.ok) throw Error(`COS_PUT_FAILED ${put.status}`);
    const complete = await api(`/miniapp/media/uploads/${session.ticketId}/complete`, {method: 'POST', body: '{}'});
    console.log(`COMPLETE_OK state=${complete.state}`);
    const deadline = Date.now() + 30 * 60 * 1000;
    while (Date.now() < deadline) {
      const status = await api(`/miniapp/media/assets/${session.assetId}`);
      console.log(`MEDIA_STATUS state=${status.state} review=${status.reviewStatus} canUse=${status.canUse}`);
      if (status.state === 'approved' && status.canUse === true && status.technicalStatus === 'final_ready' && status.reviewStatus === 'approved' && status.ticketStatus === 'consumed' && status.recovery === 'none') {
        fs.writeFileSync(output, JSON.stringify({assetId: session.assetId, ticketId: session.ticketId, state: status.state, canUse: status.canUse, technicalStatus: status.technicalStatus, reviewStatus: status.reviewStatus, ticketStatus: status.ticketStatus, recovery: status.recovery}), {encoding: 'utf8', mode: 0o600});
        console.log(`MEDIA_APPROVED asset=${session.assetId} ticket=${session.ticketId}`); return;
      }
      if (['rejected', 'review_failed', 'expired', 'upload_failed', 'deleted'].includes(status.state)) throw Error(`MEDIA_TERMINAL ${status.state}`);
      await sleep(5000);
    }
    throw Error(`MEDIA_TIMEOUT asset=${session.assetId}`);
  } finally { await Promise.resolve(app.disconnect()); }
}
main().catch(error => { console.error(error.message || String(error)); process.exitCode = 1; });
```

运行前把 `$env:REVIEW_PROXY_ACCEPTANCE_OUTPUT` 指向窗口中的新 result path。期望依次出现 `AUTHORIZE_OK`、`COMPLETE_OK`、`MEDIA_APPROVED`，且严格状态为 `approved / true / final_ready / approved / consumed / none`。30 分钟无终态只能记 `runtime PENDING`，不得人工批准或改库。

- [ ] **Step 3: 只检查新日志区间、方法语义和只读数据库状态**

以窗口 `nginxStartLine` 为边界，在服务器执行 `tail -n +$((nginxStartLine+1)) /var/log/nginx/campus.access.log`，并断言：代理 `HEAD` 200 至少 1 次、代理 `GET` 200 至少 1 次、新微信 callback `POST` 200 至少 1 次；代理和 callback 的非 200 都为 0。对新增 Nginx/journald 区间执行 `grep -Eic 'token=|signature=|RawQuery|versionId=|final/'`，敏感计数与 `panic|fatal|bind: address|permission denied` 计数都必须为 0。调用代理无效 token 还必须保持 `HEAD/GET=404`、`POST=405`。

使用结果文件中的资产 ULID 做唯一只读 SQL：

```sql
SELECT a.asset_public_id,a.storage_stage,a.technical_status,t.status,p.status,p.applied_decision_version
FROM media_assets a JOIN media_upload_tickets t ON t.asset_id=a.id JOIN media_review_projections p ON p.asset_id=a.id
WHERE a.asset_public_id='$assetId';
```

唯一行必须为 `final / final_ready / consumed / approved` 且 `applied_decision_version >= 1`；不得查询 OpenID、对象 key/version、trace 或任何凭据。

- [ ] **Step 4: 全部门禁通过后精确清理 staging 并收口**

再次校验 live SHA 等于本次 artifact SHA，staging 为 `$deploy.stageDir` 指定的普通文件、owner `admin`、mode `600`、非 symlink，然后只执行 `rm -f -- "$($deploy.stageDir)/campus-api"` 和 `rmdir -- "$($deploy.stageDir)"`。确认 `$deploy.backup/campus-api` 与 `$deploy.backup/READY` 仍存在，写回 `stagingCleaned=true`，并检查源码范围无新增改动、暂存区为空、四项服务 active。

任一代理、回调、日志、哈希、服务门禁失败立即执行 Task 7 Step 4 的 exact CAS rollback；不得搜索“最新备份”。最终只有在 `HEAD/GET/callback` 门禁、严格 approved 状态、数据库行和 `FINAL_CLOSEOUT_OK` 全部成立后才能宣布完成；超时或审核拒绝均报告 `runtime PENDING`/失败并保留 backup。
