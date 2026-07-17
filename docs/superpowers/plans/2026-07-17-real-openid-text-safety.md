# 真实 OpenID 文本安全接入 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让社区发帖、评论及二手商品创建/更新都使用认证用户的真实 OpenID 执行微信文本安全检查，并将微信临时故障安全收敛到人工审核。

**Architecture:** 在 `review` 包中定义最小 `TextSafetyChecker` 接口，`*SafetyClient` 作为生产实现，业务 Handler 注入记录型 fake 做参数验证。四个写入入口只读取一次 `user.CurrentUser(c)`，同一个 `User` 同时提供业务 ID 和 OpenID；安全检查失败只生成固定 `manual:*` 风险，不改变 submitted、幂等、审核和公开可见性契约。

**Tech Stack:** Go 1.25、Gin 1.10、`net/http`、微信 `msg_sec_check`、现有 review outbox/GORM 仓储。

---

## 基线与文件边界

- 已批准设计：`docs/superpowers/specs/2026-07-17-real-openid-text-safety-design.md`；计划基线 `95e0da4`，分支 `master`。
- 工作树已有用户改动；每次只 `git add` 本任务列出的精确路径，禁止 `git add .`、`git add -A`、reset 或清理。
- 不修改 `backend/migrations/**`、`apps/miniapp/src/**`、路由、JSON 契约、状态码、环境变量或审核状态。

## 文件结构与责任

- Create `backend/internal/module/admin/review/safety_test.go`; Modify `backend/internal/module/admin/review/safety.go` 的 `TextSafetyChecker`、`SafetyClient.CheckText`、`accessToken` — 请求体、短路、故障映射与无泄漏测试。
- Create `backend/internal/module/community/handler_test.go`; Modify `backend/internal/module/community/handler.go` 的 `Handler`、`NewHandler`、`create`、`addComment` — 发帖/评论认证 OpenID、request context 和 review risk。
- Modify `backend/internal/module/trade/handler_test.go`, `backend/internal/module/trade/handler.go` 的 `Handler`、`NewHandler`、`create`、`update` — 商品认证 OpenID、submitted 与 review risk。
- Verify only: `backend/cmd/server/main.go` — 现有 `*review.SafetyClient` 自动满足接口，不改生产装配。

### Task 1: 固定 SafetyClient 的身份、请求与故障契约

**Files:**
- Create: `backend/internal/module/admin/review/safety_test.go`
- Modify: `backend/internal/module/admin/review/safety.go`

- [ ] **Step 1: 创建会失败的定向测试**

新建 `safety_test.go`，写入以下完整内容；测试与实现同包，直接替换私有 `httpClient.Transport`，不访问真实微信网络：

```go
package review

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"campus-circle/backend/internal/config"
)

type safetyRoundTripFunc func(*http.Request) (*http.Response, error)

func (f safetyRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func safetyResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
func safetyClientForTest(transport safetyRoundTripFunc) *SafetyClient {
	c := NewSafetyClient(config.Config{WechatAppID: "app-id", WechatAppSecret: "app-secret"})
	c.httpClient = &http.Client{Transport: transport}
	return c
}
func tokenThen(next safetyRoundTripFunc) safetyRoundTripFunc {
	calls := 0
	return func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return safetyResponse(http.StatusOK, `{"access_token":"test-token"}`), nil
		}
		return next(r)
	}
}

var _ TextSafetyChecker = (*SafetyClient)(nil)

func TestSafetyClientShortCircuitsUnsafeInputs(t *testing.T) {
	calls := 0
	transport := safetyRoundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("must not be called")
	})
	client := safetyClientForTest(transport)
	if got := client.CheckText(context.Background(), "  ", "content"); got != "manual:wechat_openid_missing" {
		t.Fatalf("missing openid risk=%q", got)
	}
	if got := client.CheckText(context.Background(), "openid-secret", "  "); got != "manual:wechat_content_empty" {
		t.Fatalf("empty content risk=%q", got)
	}
	unconfigured := NewSafetyClient(config.Config{})
	unconfigured.httpClient = &http.Client{Transport: transport}
	if got := unconfigured.CheckText(context.Background(), "openid-secret", "content"); got != "manual:wechat_safety_unconfigured" {
		t.Fatalf("unconfigured risk=%q", got)
	}
	if calls != 0 { t.Fatalf("short circuit made %d HTTP calls", calls) }
}

func TestSafetyClientSendsTrustedIdentityAndContent(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{}{}, "request-context")
	client := safetyClientForTest(tokenThen(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/wxa/msg_sec_check" || r.URL.Query().Get("access_token") != "test-token" || r.Context() != ctx {
			t.Fatalf("unexpected safety request: method=%s url=%s contextMatch=%v", r.Method, r.URL, r.Context() == ctx)
		}
		var body struct { Version, Scene int; OpenID string `json:"openid"`; Content string `json:"content"` }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil { t.Fatal(err) }
		if body.Version != 2 || body.Scene != 2 || body.OpenID != "openid-secret" || body.Content != "title\ncontent" {
			t.Fatalf("unexpected body: %#v", body)
		}
		return safetyResponse(http.StatusOK, `{"errcode":0,"result":{"suggest":"pass","label":100}}`), nil
	}))
	if got := client.CheckText(ctx, "openid-secret", "title\ncontent"); got != "wechat pass" { t.Fatalf("risk=%q", got) }
}

func TestSafetyClientReturnsFixedManualRisksWithoutSecrets(t *testing.T) {
	tests := []struct { name, want string; transport safetyRoundTripFunc }{
		{"token", "manual:wechat_token_unavailable", func(*http.Request) (*http.Response, error) { return nil, errors.New("token upstream-secret") }},
		{"request", "manual:wechat_request_failed", tokenThen(func(*http.Request) (*http.Response, error) { return nil, errors.New("body upstream-secret") })},
		{"http", "manual:wechat_http_error", tokenThen(func(*http.Request) (*http.Response, error) { return safetyResponse(http.StatusBadGateway, "upstream-secret"), nil })},
		{"decode", "manual:wechat_response_invalid", tokenThen(func(*http.Request) (*http.Response, error) { return safetyResponse(http.StatusOK, "upstream-secret"), nil })},
		{"api", "manual:wechat_api_error:87014", tokenThen(func(*http.Request) (*http.Response, error) { return safetyResponse(http.StatusOK, `{"errcode":87014,"errmsg":"upstream-secret"}`), nil })},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safetyClientForTest(tt.transport).CheckText(context.Background(), "openid-secret", "content-secret")
			if got != tt.want { t.Fatalf("risk=%q want=%q", got, tt.want) }
			for _, secret := range []string{"openid-secret", "content-secret", "test-token", "upstream-secret"} {
				if strings.Contains(got, secret) { t.Fatalf("risk leaked %q: %q", secret, got) }
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试确认红灯**

Run（工作目录 `backend`）: `go test ./internal/module/admin/review -run 'TestSafetyClient' -count=1`

Expected: FAIL，首先包含 `undefined: TextSafetyChecker`；暂时定义接口后仍会因现有 `wechat safety skipped` 或泄漏原始错误而 FAIL。

- [ ] **Step 3: 写入最小实现**

在 `safety.go` 导入中增加 `strings`，在 `SafetyClient` 前增加：

```go
type TextSafetyChecker interface {
	CheckText(context.Context, string, string) string
}

var _ TextSafetyChecker = (*SafetyClient)(nil)
```

用以下完整实现替换 `CheckText` 和 `accessToken`：

```go
func (c *SafetyClient) CheckText(ctx context.Context, openID, content string) string {
	if strings.TrimSpace(openID) == "" { return "manual:wechat_openid_missing" }
	if strings.TrimSpace(content) == "" { return "manual:wechat_content_empty" }
	if strings.TrimSpace(c.appID) == "" || strings.TrimSpace(c.appSecret) == "" { return "manual:wechat_safety_unconfigured" }
	token, err := c.accessToken(ctx)
	if err != nil { return "manual:wechat_token_unavailable" }
	body, err := json.Marshal(map[string]any{"version": 2, "scene": 2, "openid": openID, "content": content})
	if err != nil { return "manual:wechat_request_failed" }
	endpoint := "https://api.weixin.qq.com/wxa/msg_sec_check?access_token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil { return "manual:wechat_request_failed" }
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil { return "manual:wechat_request_failed" }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return "manual:wechat_http_error" }
	var result struct {
		ErrCode int `json:"errcode"`
		Result struct { Suggest string `json:"suggest"`; Label int `json:"label"` } `json:"result"`
	}
	if json.NewDecoder(resp.Body).Decode(&result) != nil { return "manual:wechat_response_invalid" }
	if result.ErrCode != 0 { return fmt.Sprintf("manual:wechat_api_error:%d", result.ErrCode) }
	suggest := strings.ToLower(strings.TrimSpace(result.Result.Suggest))
	switch suggest {
	case "", "pass": return "wechat pass"
	case "review", "risky": return fmt.Sprintf("wechat %s label=%d", suggest, result.Result.Label)
	default: return "manual:wechat_response_invalid"
	}
}

func (c *SafetyClient) accessToken(ctx context.Context) (string, error) {
	endpoint := "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=" + url.QueryEscape(c.appID) + "&secret=" + url.QueryEscape(c.appSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil { return "", err }
	resp, err := c.httpClient.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 { return "", errors.New("wechat token http error") }
	var result struct { AccessToken string `json:"access_token"`; ErrCode int `json:"errcode"` }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil { return "", err }
	if strings.TrimSpace(result.AccessToken) == "" || result.ErrCode != 0 { return "", errors.New("wechat token unavailable") }
	return result.AccessToken, nil
}
```

同时从 `safety.go` 删除已不再使用的 `ErrMsg` 字段，并在 import 中增加 `errors`。

- [ ] **Step 4: 运行定向测试并格式化**

Run（工作目录 `backend`）:

```powershell
gofmt -w internal/module/admin/review/safety.go internal/module/admin/review/safety_test.go
go test ./internal/module/admin/review -run 'TestSafetyClient' -count=1
```

Expected: PASS，且没有外部网络请求。

- [ ] **Step 5: 提交 review 适配器**

```powershell
git add internal/module/admin/review/safety.go internal/module/admin/review/safety_test.go
git commit -m "feat: 收敛微信文本安全风险"
```

### Task 2: 社区发帖与评论传入真实 OpenID

**Files:**
- Create: `backend/internal/module/community/handler_test.go`
- Modify: `backend/internal/module/community/handler.go`

- [ ] **Step 1: 写入会失败的两入口 Handler 测试**

创建 `handler_test.go`，写入记录 fake 与以下完整测试（复用同包 `service_test.go` 的 memory repo/recorder）：

```go
package community

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
)

type communitySafetyCall struct { ctx context.Context; openID, content string }
type communitySafetyRecorder struct { risk string; calls []communitySafetyCall }
func (r *communitySafetyRecorder) CheckText(ctx context.Context, openID, content string) string {
	r.calls = append(r.calls, communitySafetyCall{ctx, openID, content})
	return r.risk
}

func TestCommunityHandlersUseAuthenticatedOpenID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &communityMemoryRepo{posts: map[uint]Post{9: {ID: 9, UserID: 1, ReviewStatus: review.StatusApproved}}}
	reviews := &submissionRecorder{}
	checker := &communitySafetyRecorder{risk: "manual:wechat_request_failed"}
	handler := NewHandler(NewService(repo, profileDirectory{}, reviews), nil, checker)
	router := gin.New()
	current := user.User{ID: 7, OpenID: "openid-community-secret"}
	router.POST("/posts", func(c *gin.Context) { c.Set("domainA.user", current); handler.create(c) })
	router.POST("/posts/:id/comments", func(c *gin.Context) { c.Set("domainA.user", current); handler.addComment(c) })
	tests := []struct { path, body, content, objectType string }{
		{"/posts", `{"title":"title","content":"content"}`, "title\ncontent", "post"},
		{"/posts/9/comments", `{"content":"comment"}`, "comment", "comment"},
	}
	for _, tt := range tests {
		requestContext := context.WithValue(context.Background(), struct{}{}, tt.objectType)
		req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body)).WithContext(requestContext)
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		beforeCalls, beforeReviews := len(checker.calls), len(reviews.submissions)
		router.ServeHTTP(response, req)
		if response.Code != http.StatusCreated { t.Fatalf("%s status=%d body=%s", tt.objectType, response.Code, response.Body.String()) }
		call := checker.calls[beforeCalls]
		if call.ctx != requestContext || call.openID != current.OpenID || call.content != tt.content {
			t.Fatalf("%s safety call=%#v", tt.objectType, call)
		}
		submission := reviews.submissions[beforeReviews]
		if submission.ObjectType != tt.objectType || submission.SubmitterID != current.ID || submission.Risk != checker.risk {
			t.Fatalf("%s submission=%#v", tt.objectType, submission)
		}
		if strings.Contains(response.Body.String(), current.OpenID) { t.Fatalf("%s response leaked openid", tt.objectType) }
	}
}
```

- [ ] **Step 2: 确认测试因具体类型依赖而失败**

Run: `go test ./internal/module/community -run TestCommunityHandlersUseAuthenticatedOpenID -count=1`

Expected: FAIL，包含 `cannot use checker ... as *review.SafetyClient`。

- [ ] **Step 3: 最小替换 Handler 依赖与两个写入块**

```go
type Handler struct {
	service *Service
	users   *user.Service
	safety  review.TextSafetyChecker
}

func NewHandler(service *Service, users *user.Service, safety review.TextSafetyChecker) *Handler {
	return &Handler{service: service, users: users, safety: safety}
}
```

`create` 中用以下块替换原 `risk/input`：

```go
currentUser := user.CurrentUser(c)
risk := h.safety.CheckText(c.Request.Context(), currentUser.OpenID, req.Title+"\n"+req.Content)
input := CreatePostInput{UserID: currentUser.ID, Title: req.Title, Content: req.Content, Topic: req.Topic, ImageAssetIDs: req.ImageAssetIDs}
```

`addComment` 在幂等键校验后读取一次用户，并在 keyed/非 keyed 两分支共用：

```go
currentUser := user.CurrentUser(c)
risk := h.safety.CheckText(c.Request.Context(), currentUser.OpenID, req.Content)
if keyed {
	result, err := h.service.AddCommentKeyed(c, id, currentUser.ID, req.Content, clientRequestID, risk)
	writeKeyedCreate(c, result.Value, result.Replayed, err)
	return
}
result, err := h.service.AddComment(c, id, currentUser.ID, req.Content, risk)
```

- [ ] **Step 4: 格式化、运行社区包全测试并提交**

```powershell
gofmt -w internal/module/community/handler.go internal/module/community/handler_test.go
go test ./internal/module/community -count=1
git add internal/module/community/handler.go internal/module/community/handler_test.go
git commit -m "feat: 社区文本检查使用认证 OpenID"
```

Expected: PASS；现有 `TestCommunityKeyedCreateUsesCanonicalHashAndStableOutboxPayload`、`TestCommunityKeyedCommentCreatesOneOutboxAndReplays` 和两个 keyed HTTP status 测试仍 PASS。

### Task 3: 二手商品创建与更新传入真实 OpenID

**Files:**
- Modify: `backend/internal/module/trade/handler_test.go`
- Modify: `backend/internal/module/trade/handler.go`

- [ ] **Step 1: 先扩展 Handler 测试**

在 `handler_test.go` import 中增加 `strings`、`review`、`user`，并追加：

```go
type tradeSafetyCall struct { ctx context.Context; openID, content string }
type tradeSafetyRecorder struct { risk string; calls []tradeSafetyCall }
func (r *tradeSafetyRecorder) CheckText(ctx context.Context, openID, content string) string {
	r.calls = append(r.calls, tradeSafetyCall{ctx, openID, content})
	return r.risk
}
type tradeReviewRecorder struct { submissions []review.Submission }
func (r *tradeReviewRecorder) Submit(_ context.Context, value review.Submission) (review.Task, error) {
	r.submissions = append(r.submissions, value)
	return review.Task{ObjectType: value.ObjectType, ObjectID: value.ObjectID}, nil
}

func TestTradeHandlersUseAuthenticatedOpenID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newTradeMemoryRepo(Product{ID: 9, SellerID: 7, Title: "old", Status: StatusSelling, ReviewStatus: review.StatusApproved})
	reviews := &tradeReviewRecorder{}
	checker := &tradeSafetyRecorder{risk: "manual:wechat_request_failed"}
	handler := NewHandler(NewService(repo, tradeUsers{}, reviews, &tradeConversations{}, &tradeSearchFake{}), nil, checker)
	router := gin.New()
	current := user.User{ID: 7, OpenID: "openid-trade-secret"}
	router.POST("/products", func(c *gin.Context) { c.Set("domainA.user", current); handler.create(c) })
	router.PUT("/products/:id", func(c *gin.Context) { c.Set("domainA.user", current); handler.update(c) })
	tests := []struct { method, path, body, content string }{
		{http.MethodPost, "/products", `{"title":"new","description":"create","price":100}`, "new\ncreate"},
		{http.MethodPut, "/products/9", `{"title":"updated","description":"replace","price":200}`, "updated\nreplace"},
	}
	for _, tt := range tests {
		requestContext := context.WithValue(context.Background(), struct{}{}, tt.path)
		req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)).WithContext(requestContext)
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		beforeCalls, beforeReviews := len(checker.calls), len(reviews.submissions)
		router.ServeHTTP(response, req)
		wantStatus := http.StatusOK
		if tt.method == http.MethodPost { wantStatus = http.StatusCreated }
		if response.Code != wantStatus { t.Fatalf("%s status=%d body=%s", tt.path, response.Code, response.Body.String()) }
		call := checker.calls[beforeCalls]
		if call.ctx != requestContext || call.openID != current.OpenID || call.content != tt.content { t.Fatalf("%s call=%#v", tt.path, call) }
		submission := reviews.submissions[beforeReviews]
		if submission.ObjectType != "product" || submission.SubmitterID != current.ID || submission.Risk != checker.risk { t.Fatalf("%s submission=%#v", tt.path, submission) }
		if strings.Contains(response.Body.String(), current.OpenID) { t.Fatalf("%s response leaked openid", tt.path) }
	}
	if repo.products[9].ReviewStatus != review.StatusSubmitted { t.Fatalf("update review status=%s", repo.products[9].ReviewStatus) }
	if _, err := repo.Find(context.Background(), 9, true); !errors.Is(err, ErrNotFound) { t.Fatalf("updated product became public: %v", err) }
}
```

- [ ] **Step 2: 确认红灯**

Run: `go test ./internal/module/trade -run TestTradeHandlersUseAuthenticatedOpenID -count=1`

Expected: FAIL，包含 `cannot use checker ... as *review.SafetyClient`。

- [ ] **Step 3: 最小替换 Handler 实现**

将 `Handler.safety` 与 `NewHandler` 第三参数替换为 `review.TextSafetyChecker`。`create` 的 input/risk 块替换为：

```go
currentUser := user.CurrentUser(c)
input := toInput(req, currentUser.ID)
risk := h.safety.CheckText(c.Request.Context(), currentUser.OpenID, req.Title+"\n"+req.Description)
```

`update` 的 input/risk 块替换为：

```go
currentUser := user.CurrentUser(c)
input := toInput(req, currentUser.ID)
risk := h.safety.CheckText(c.Request.Context(), currentUser.OpenID, req.Title+"\n"+req.Description)
```

- [ ] **Step 4: 格式化、运行 trade 包全测试并提交**

```powershell
gofmt -w internal/module/trade/handler.go internal/module/trade/handler_test.go
go test ./internal/module/trade -count=1
git add internal/module/trade/handler.go internal/module/trade/handler_test.go
git commit -m "feat: 二手文本检查使用认证 OpenID"
```

Expected: PASS；现有 `TestCreateKeyedFirstReplayConflictAndStableOutboxPayload`、更新重提交、未批准不可公开及批准后搜索测试仍 PASS。

### Task 4: 生产装配与全仓回归门禁

- [ ] **Step 1: 验证生产装配编译**

Run: `go test ./internal/module/admin/review ./internal/module/community ./internal/module/trade ./cmd/server -count=1`

Expected: 四个包 PASS；`main.go` 中现有 `safety := review.NewSafetyClient(cfg)` 可直接注入两个 Handler，无需运行配置改动。

- [ ] **Step 2: 执行后端全量测试**

Run: `go test ./...`

Expected: PASS；幂等重放不增加对象、媒体预留、outbox 或审核任务副作用。

- [ ] **Step 3: 执行静态检查**

Run（工作目录 `backend`）: `go vet ./...`

Run（仓库根目录）:

```powershell
git diff --check
git status --short
```

Expected: `go vet` 和 `git diff --check` 零错误；`git status --short` 只显示实施者未提交的本计划文件或原有用户改动，不出现 migration/前端/API 契约变更。

- [ ] **Step 4: 最终敏感信息与边界扫描**

Run（仓库根目录）:

```powershell
rg -n 'CheckText\(c, ""|CheckText\(c\.Request\.Context\(\), ""|wechat (token|request|decode|error).*\+' backend/internal/module
git diff --name-only -- backend/migrations apps/miniapp/src
```

Expected: 第一条无命中，第二条无输出；OpenID、正文、token 和微信原始错误不进入 risk、日志或 API 响应。若门禁暴露本计划回归问题，回到所属 Task 用精确路径修复、重跑并 amend 该 Task 提交，不创建空提交。
