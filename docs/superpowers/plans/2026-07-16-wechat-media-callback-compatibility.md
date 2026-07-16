# 微信媒体审核回调兼容修复 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Security review amendment:** 实施期复核发现 Gin 查询串日志、重复关键字段和固定 staging/模糊回滚问题。`docs/superpowers/specs/2026-07-16-wechat-media-callback-security-addendum.md` 是强制约束；其中要求在冲突时优先于本计划原有的简单结构体解码、固定 `/tmp` staging、`find latest` 回滚和日志 grep 步骤。未完成补充设计中的测试、日志探针与 Token 再轮换前，禁止部署或判定验收通过。

**Goal:** 让生产后端安全兼容微信媒体审核回调的 JSON/XML 正文及字符串/数字 label，并以新的真实图片证明回调最终推进到 approved。

**Architecture:** 在现有 `WechatCallbackHandler` 内加入受限正文读取和纯解析函数；解析函数不依赖 `Content-Type`，JSON 合法时严格按 JSON 处理，否则尝试 XML。审核签名、Service、projection、outbox 和 consumer 状态机保持不变，任何解析歧义继续 fail-closed。

**Tech Stack:** Go 1.24、Gin、`encoding/json`、`encoding/xml`、`net/http`、Go testing、systemd、Nginx、微信开发者工具自动化、腾讯云 COS。

---

## 文件结构与责任

- Modify: `backend/internal/module/media/miniappflow/wechat_callback_handler.go`
  - 限制回调正文大小；
  - JSON/XML 内容识别；
  - 数字/字符串 label 兼容；
  - HTTP 状态映射与既有 Service 调用。
- Create: `backend/internal/module/media/miniappflow/wechat_callback_handler_test.go`
  - 解析器线协议测试；
  - 签名、正文上限、错误映射和错标 `Content-Type` 测试。
- Verify only: `backend/internal/module/media/miniappflow/service_test.go`
  - 复用既有微信结果落库与幂等测试，不修改业务状态机。
- No changes: `backend/migrations/**`、`apps/**`、公开 API、Nginx 路由和微信平台配置。

基线设计提交为 `f70c89b1`。所有实现提交只能精确暂存上述 Go 文件，禁止把当前工作区中已有的部署配置改动混入。

### Task 1: 为微信线协议变体增加纯解析器

**Files:**
- Create: `backend/internal/module/media/miniappflow/wechat_callback_handler_test.go`
- Modify: `backend/internal/module/media/miniappflow/wechat_callback_handler.go`

- [ ] **Step 1: 写入会失败的解析器测试**

新建 `backend/internal/module/media/miniappflow/wechat_callback_handler_test.go`，先写入：

```go
package miniappflow

import "testing"

func TestDecodeWechatReviewResultAcceptsWireVariants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
	}{
		{
			name: "json numeric label",
			body: `{"ToUserName":"gh_test","MsgType":"event","Event":"wxa_media_check","trace_id":"trace-json-number","result":{"suggest":"pass","label":100}}`,
		},
		{
			name: "json string label",
			body: `{"ToUserName":"gh_test","MsgType":"event","Event":"wxa_media_check","trace_id":"trace-json-string","result":{"suggest":"pass","label":"100"}}`,
		},
		{
			name: "xml label",
			body: `<xml><ToUserName>gh_test</ToUserName><MsgType>event</MsgType><Event>wxa_media_check</Event><trace_id>trace-xml</trace_id><result><suggest>pass</suggest><label>100</label></result></xml>`,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			value, err := decodeWechatReviewResult([]byte(test.body))
			if err != nil {
				t.Fatalf("decode callback: %v", err)
			}
			if value.TraceID == "" || value.Result.Suggest != "pass" || value.Result.Label == nil || int(*value.Result.Label) != 100 {
				t.Fatalf("unexpected callback: %#v", value)
			}
		})
	}
}

func TestDecodeWechatReviewResultRejectsAmbiguousInput(t *testing.T) {
	t.Parallel()
	tests := []string{
		``,
		`{}`,
		`{"trace_id":"trace-1","result":{"suggest":"","label":100}}`,
		`{"trace_id":"trace-1","result":{"suggest":"pass"}}`,
		`{"trace_id":"trace-1","result":{"suggest":"pass","label":"100x"}}`,
		`{"trace_id":"trace-1","result":{"suggest":"pass","label":1.5}}`,
		`{"trace_id":"trace-1","result":{"suggest":"pass","label":-1}}`,
		`<xml><trace_id>trace-1</trace_id><result><suggest>pass</suggest><label>bad</label></result></xml>`,
	}
	for _, body := range tests {
		if value, err := decodeWechatReviewResult([]byte(body)); err == nil {
			t.Fatalf("expected rejection, got %#v for %q", value, body)
		}
	}
}
```

- [ ] **Step 2: 运行测试并确认失败原因正确**

Run:

```powershell
Set-Location 'C:\Users\33141\Documents\New project\backend'
go test ./internal/module/media/miniappflow -run 'TestDecodeWechatReviewResult' -count=1
```

Expected: FAIL，编译错误包含 `undefined: decodeWechatReviewResult`。

- [ ] **Step 3: 实现最小的灵活整数和双格式解析函数**

在 `wechat_callback_handler.go` 的 import 中加入 `bytes`、`encoding/json`、`errors` 和 `strconv`，保留 `encoding/xml`、`net/http`、`strings`。加入以下实现：

```go
var errInvalidWechatReviewCallback = errors.New("invalid wechat review callback")

type wechatReviewLabel int

func parseWechatReviewLabel(raw string) (wechatReviewLabel, error) {
	raw = strings.TrimSpace(raw)
	value, err := strconv.ParseInt(raw, 10, strconv.IntSize)
	if err != nil || value < 0 {
		return 0, errInvalidWechatReviewCallback
	}
	return wechatReviewLabel(value), nil
}

func (value *wechatReviewLabel) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return errInvalidWechatReviewCallback
	}
	raw := string(data)
	if data[0] == '"' {
		if err := json.Unmarshal(data, &raw); err != nil {
			return errInvalidWechatReviewCallback
		}
	}
	parsed, err := parseWechatReviewLabel(raw)
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}

func (value *wechatReviewLabel) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	var raw string
	if err := decoder.DecodeElement(&raw, &start); err != nil {
		return errInvalidWechatReviewCallback
	}
	parsed, err := parseWechatReviewLabel(raw)
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}
```

把结果结构替换为以下定义。`Label` 指针用于区分合法的 `label=0` 与字段缺失：

```go
type wechatReviewResult struct {
	TraceID string `json:"trace_id" xml:"trace_id"`
	Result  struct {
		Suggest string             `json:"suggest" xml:"suggest"`
		Label   *wechatReviewLabel `json:"label" xml:"label"`
	} `json:"result" xml:"result"`
}
```

随后加入纯解析函数：

```go
func decodeWechatReviewResult(body []byte) (wechatReviewResult, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return wechatReviewResult{}, errInvalidWechatReviewCallback
	}
	var value wechatReviewResult
	if json.Valid(body) {
		if err := json.Unmarshal(body, &value); err != nil {
			return wechatReviewResult{}, errInvalidWechatReviewCallback
		}
	} else if err := xml.Unmarshal(body, &value); err != nil {
		return wechatReviewResult{}, errInvalidWechatReviewCallback
	}
	if strings.TrimSpace(value.TraceID) == "" || strings.TrimSpace(value.Result.Suggest) == "" || value.Result.Label == nil {
		return wechatReviewResult{}, errInvalidWechatReviewCallback
	}
	return value, nil
}
```

暂时把原 callback 中传给 Service 的 label 改为 `int(*value.Result.Label)`，其余流程不改。

- [ ] **Step 4: 运行解析器测试并确认通过**

Run:

```powershell
go test ./internal/module/media/miniappflow -run 'TestDecodeWechatReviewResult' -count=1
```

Expected: PASS。

- [ ] **Step 5: 精确提交解析兼容层**

```powershell
Set-Location 'C:\Users\33141\Documents\New project'
git add -- backend/internal/module/media/miniappflow/wechat_callback_handler.go backend/internal/module/media/miniappflow/wechat_callback_handler_test.go
git diff --cached --check
git commit -m 'fix(media): 兼容微信审核回调字段表示'
```

Expected: 提交只包含上述两个文件。

### Task 2: 让 HTTP 处理器安全地忽略错误 Content-Type

**Files:**
- Modify: `backend/internal/module/media/miniappflow/wechat_callback_handler.go`
- Modify: `backend/internal/module/media/miniappflow/wechat_callback_handler_test.go`

- [ ] **Step 1: 添加会失败的 HTTP 行为测试**

把测试文件 import 扩展为：

```go
import (
	"context"
	"crypto/sha1" // #nosec G505 -- the WeChat protocol mandates SHA-1.
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"

	coremedia "campus-circle/backend/internal/module/media"
	"github.com/gin-gonic/gin"
)
```

并在已有解析器测试后追加：

```go
type recordingWechatReviewApplier struct {
	calls   int
	traceID string
	suggest string
	label   int
	err     error
}

func (value *recordingWechatReviewApplier) ApplyWechatReviewResult(_ context.Context, traceID, suggest string, label int) error {
	value.calls++
	value.traceID = traceID
	value.suggest = suggest
	value.label = label
	return value.err
}

func signedWechatReviewRequest(body, contentType, token string, validSignature bool) *http.Request {
	timestamp, nonce := "1710000000", "nonce-1"
	parts := []string{token, timestamp, nonce}
	sort.Strings(parts)
	sum := sha1.Sum([]byte(strings.Join(parts, "")))
	signature := hex.EncodeToString(sum[:])
	if !validSignature {
		signature = strings.Repeat("0", 40)
	}
	query := url.Values{
		"timestamp": {timestamp},
		"nonce":     {nonce},
		"signature": {signature},
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/miniapp/media/wechat/callback?"+query.Encode(), strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	return request
}

func runWechatReviewCallback(t *testing.T, applier *recordingWechatReviewApplier, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	handler := &WechatCallbackHandler{service: applier, callbackToken: "callback-token-strong"}
	handler.Register(router.Group("/api/v1/miniapp/media"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestWechatCallbackIgnoresMisleadingContentType(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		body        string
		contentType string
		traceID     string
	}{
		{
			name:        "json marked as xml",
			body:        `{"trace_id":"trace-json","result":{"suggest":"pass","label":"100"}}`,
			contentType: "application/xml",
			traceID:     "trace-json",
		},
		{
			name:        "xml marked as text",
			body:        `<xml><trace_id>trace-xml</trace_id><result><suggest>pass</suggest><label>100</label></result></xml>`,
			contentType: "text/plain",
			traceID:     "trace-xml",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			applier := &recordingWechatReviewApplier{}
			request := signedWechatReviewRequest(test.body, test.contentType, "callback-token-strong", true)
			response := runWechatReviewCallback(t, applier, request)
			if response.Code != http.StatusOK || response.Body.String() != "success" {
				t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
			}
			if applier.calls != 1 || applier.traceID != test.traceID || applier.suggest != "pass" || applier.label != 100 {
				t.Fatalf("unexpected call: %#v", applier)
			}
		})
	}
}

func TestWechatCallbackRejectsBadSignatureBeforeReadingBody(t *testing.T) {
	t.Parallel()
	applier := &recordingWechatReviewApplier{}
	request := signedWechatReviewRequest(`{"trace_id":"trace-1","result":{"suggest":"pass","label":100}}`, "application/json", "callback-token-strong", false)
	response := runWechatReviewCallback(t, applier, request)
	if response.Code != http.StatusForbidden || applier.calls != 0 {
		t.Fatalf("status=%d calls=%d", response.Code, applier.calls)
	}
}

func TestWechatCallbackRejectsOversizedBody(t *testing.T) {
	t.Parallel()
	applier := &recordingWechatReviewApplier{}
	body := `{"trace_id":"trace-1","result":{"suggest":"pass","label":100},"padding":"` + strings.Repeat("x", int(maxWechatReviewCallbackBodyBytes)) + `"}`
	request := signedWechatReviewRequest(body, "application/json", "callback-token-strong", true)
	response := runWechatReviewCallback(t, applier, request)
	if response.Code != http.StatusBadRequest || applier.calls != 0 {
		t.Fatalf("status=%d calls=%d", response.Code, applier.calls)
	}
}

func TestWechatCallbackMapsServiceFailureToRetryableError(t *testing.T) {
	t.Parallel()
	applier := &recordingWechatReviewApplier{err: errors.New("projection unavailable")}
	request := signedWechatReviewRequest(`{"trace_id":"trace-1","result":{"suggest":"pass","label":100}}`, "application/json", "callback-token-strong", true)
	response := runWechatReviewCallback(t, applier, request)
	if response.Code != http.StatusInternalServerError || applier.calls != 1 {
		t.Fatalf("status=%d calls=%d", response.Code, applier.calls)
	}
}

func TestApplyWechatReviewResultKeepsTerminalProjectionIdempotent(t *testing.T) {
	t.Parallel()
	traceID := "wechat-terminal-trace"
	reviewTaskID, err := WechatReviewTaskID(traceID)
	if err != nil {
		t.Fatal(err)
	}
	projection := &projectionStub{value: coremedia.ReviewProjection{
		ProjectionID: testULID(t),
		AssetID: 41,
		Status: coremedia.ProjectionApproved,
		ReviewTaskID: &reviewTaskID,
		AppliedDecisionVersion: 1,
		RowVersion: 4,
	}}
	outbox := &outboxStub{}
	service, err := NewService(
		&ticketStub{}, &bundleStub{}, &completionStub{}, &assetStub{}, projection,
		outbox, &consumerStub{}, &statusStub{}, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ApplyWechatReviewResult(context.Background(), traceID, "pass", 100); err != nil {
		t.Fatal(err)
	}
	if len(outbox.values) != 0 {
		t.Fatalf("terminal callback appended events: %#v", outbox.values)
	}
}
```

- [ ] **Step 2: 运行 HTTP 测试并确认现有实现失败**

```powershell
Set-Location 'C:\Users\33141\Documents\New project\backend'
go test ./internal/module/media/miniappflow -run 'TestWechatCallback' -count=1
```

Expected: FAIL；至少包含 `cannot use applier` 或错标 `Content-Type` 返回 400，证明测试击中了当前耦合点。

- [ ] **Step 3: 加入可测试 Service 边界和 256 KiB 正文上限**

在 handler 文件 import 中加入 `context` 和 `io`。加入接口并把字段类型改为接口，构造函数签名仍接收 `*Service`：

```go
const maxWechatReviewCallbackBodyBytes int64 = 256 << 10

type wechatReviewResultApplier interface {
	ApplyWechatReviewResult(context.Context, string, string, int) error
}

type WechatCallbackHandler struct {
	service       wechatReviewResultApplier
	callbackToken string
}
```

用以下实现完整替换 `callback` 方法：

```go
func (h *WechatCallbackHandler) callback(c *gin.Context) {
	if !h.valid(c) {
		c.Status(http.StatusForbidden)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxWechatReviewCallbackBodyBytes)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	value, err := decodeWechatReviewResult(body)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := h.service.ApplyWechatReviewResult(
		c.Request.Context(),
		value.TraceID,
		value.Result.Suggest,
		int(*value.Result.Label),
	); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.String(http.StatusOK, "success")
}
```

删除旧 callback 中基于 `Content-Type` 的 JSON/XML 分支。不得添加原始正文日志。

- [ ] **Step 4: 运行处理器与 Service 回归测试**

```powershell
go test ./internal/module/media/miniappflow -run 'TestWechatCallback|TestDecodeWechatReviewResult|TestApplyWechatReviewResult' -count=1
```

Expected: PASS。

- [ ] **Step 5: 提交 HTTP 安全兼容改动**

```powershell
Set-Location 'C:\Users\33141\Documents\New project'
git add -- backend/internal/module/media/miniappflow/wechat_callback_handler.go backend/internal/module/media/miniappflow/wechat_callback_handler_test.go
git diff --cached --check
git commit -m 'fix(media): 安全解析微信审核回调正文'
```

Expected: 提交只包含两个媒体回调文件。

### Task 3: 执行本地交付门禁

**Files:**
- Verify: `backend/internal/module/media/miniappflow/**`
- Verify unchanged: `backend/migrations/**`

- [ ] **Step 1: 运行 focused media 测试**

```powershell
Set-Location 'C:\Users\33141\Documents\New project\backend'
go test ./cmd/server ./internal/module/media/... -count=1
```

Expected: PASS。

- [ ] **Step 2: 运行 backend 全量测试并诚实区分环境失败**

```powershell
go test ./... -count=1
```

Expected: 正常环境 PASS。若共享 MySQL 验收库仍非空或再次发生既有死锁，只记录准确包名和错误，禁止清空用户的共享数据库，也不得把 focused PASS 描述为全量 PASS。

- [ ] **Step 3: 运行仓库边界和差异门禁**

```powershell
Set-Location 'C:\Users\33141\Documents\New project'
git diff --check
npm run check:boundaries
git diff --exit-code f70c89b1..HEAD -- backend/migrations
```

Expected:

- `git diff --check`: PASS；
- `npm run check:boundaries`: PASS；
- migrations diff: 空。

- [ ] **Step 4: 核对提交和工作区隔离**

```powershell
git log -3 --oneline
git status --short --branch
git diff --name-only f70c89b1..HEAD
```

Expected: 实现提交只增加两个 Go 文件的差异；原有 `.env.example`、`.gitignore`、miniapp 配置和 `deploy/native/` 工作区改动仍保持原状，未被误提交。

### Task 4: 构建并安全部署新的 API 二进制

**Files:**
- Build artifact: `.release/campus-api-callback-fix`
- Server backup: `/opt/cutover-backups/callback-fix-YYYYMMDD_HHMMSS/campus-api`，目录名由部署命令使用服务器时间生成。
- Replace: `/opt/campus-circle/current/bin/campus-api`

- [ ] **Step 1: 构建 Linux amd64 静态二进制**

```powershell
$ErrorActionPreference='Stop'
Set-Location 'C:\Users\33141\Documents\New project\backend'
$env:CGO_ENABLED='0'
$env:GOOS='linux'
$env:GOARCH='amd64'
go build -trimpath -ldflags='-s -w' -o '..\.release\campus-api-callback-fix' .\cmd\server
if ($LASTEXITCODE -ne 0) { throw 'API 编译失败' }
Get-FileHash '..\.release\campus-api-callback-fix' -Algorithm SHA256
```

Expected: 编译成功并打印一个 64 位 SHA256；不修改源码。

- [ ] **Step 2: 上传临时文件并校验 SHA256**

```powershell
$ErrorActionPreference='Stop'
Set-Location 'C:\Users\33141\Documents\New project'
$key='C:\Users\33141\.ssh\id_ed25519_zhandui'
$artifact='.release\campus-api-callback-fix'
$localSha=(Get-FileHash $artifact -Algorithm SHA256).Hash.ToLowerInvariant()
scp -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes $artifact 'admin@8.138.227.107:/tmp/campus-api-callback-fix'
if ($LASTEXITCODE -ne 0) { throw '上传失败' }
$remoteSha=(ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes admin@8.138.227.107 "sha256sum /tmp/campus-api-callback-fix | cut -d' ' -f1").Trim()
if ($remoteSha -ne $localSha) { throw "SHA256 不一致 local=$localSha remote=$remoteSha" }
Write-Output "SHA256_OK $localSha"
```

Expected: `SHA256_OK`，不输出任何环境密钥。

- [ ] **Step 3: 备份并原子替换 API**

```powershell
$key='C:\Users\33141\.ssh\id_ed25519_zhandui'
$remote=@'
set -euo pipefail
stamp="$(date +%Y%m%d_%H%M%S)"
backup="/opt/cutover-backups/callback-fix-${stamp}"
install -d -o root -g root -m 0700 "$backup"
cp -a /opt/campus-circle/current/bin/campus-api "$backup/campus-api"
systemctl status campus-api --no-pager >"$backup/campus-api.status.before.txt"
install -o root -g campus -m 0750 /tmp/campus-api-callback-fix /opt/campus-circle/current/bin/campus-api.new
mv -f /opt/campus-circle/current/bin/campus-api.new /opt/campus-circle/current/bin/campus-api
systemctl restart campus-api
for i in $(seq 1 30); do
  if systemctl is-active --quiet campus-api && curl -fsS --max-time 2 http://127.0.0.1:8080/healthz >/dev/null; then
    break
  fi
  sleep 1
done
systemctl is-active --quiet campus-api
curl -fsS --max-time 5 http://127.0.0.1:8080/healthz >/dev/null
systemctl status campus-api --no-pager >"$backup/campus-api.status.after.txt"
sha256sum /opt/campus-circle/current/bin/campus-api >"$backup/campus-api.sha256"
rm -f /tmp/campus-api-callback-fix
echo "DEPLOY_OK backup=$backup"
'@
$bytes=[Text.Encoding]::UTF8.GetBytes($remote)
$encoded=[Convert]::ToBase64String($bytes)
ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes admin@8.138.227.107 "echo '$encoded' | base64 -d | sudo -n bash"
if ($LASTEXITCODE -ne 0) { throw '服务器替换失败，执行回滚步骤' }
```

Expected: `DEPLOY_OK`，并给出不含秘密的备份目录。

- [ ] **Step 4: 验证公网服务和错误日志**

```powershell
$health=Invoke-RestMethod -Uri 'https://zhanduiwang.top/healthz' -TimeoutSec 20
$products=Invoke-RestMethod -Uri 'https://zhanduiwang.top/api/v1/trade/products?page=1&pageSize=1' -TimeoutSec 20
if ($health.code -ne 0 -or $health.data.status -ne 'ok') { throw '公网健康检查失败' }
if ($products.code -ne 0) { throw '公开商品列表失败' }
$key='C:\Users\33141\.ssh\id_ed25519_zhandui'
ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes admin@8.138.227.107 "sudo -n journalctl -u campus-api --since '5 minutes ago' --no-pager | grep -Ei 'panic|fatal|bind: address|permission denied'"
if ($LASTEXITCODE -eq 0) { throw 'API 日志出现致命错误' }
Write-Output 'PUBLIC_SMOKE_OK'
```

Expected: `PUBLIC_SMOKE_OK`。

### Task 5: 重新执行真实微信媒体审核验收

**Files:**
- Runtime only: 微信开发者工具当前项目会话
- Runtime only: `.release/automator/node_modules/miniprogram-automator`
- No source changes

- [ ] **Step 1: 开启当前项目自动化端口**

```powershell
& 'D:\3314\微信web开发者工具\cli.bat' auto `
  --project 'C:\Users\33141\Documents\New project\apps\miniapp' `
  --auto-port 9420 `
  --trust-project `
  --lang zh
if ($LASTEXITCODE -ne 0) { throw '微信开发者工具自动化启动失败' }
```

Expected: AppID 显示 `wx961bc6699396f25b` 且 `auto` 成功。

- [ ] **Step 2: 使用内存测试图走真实授权、COS、complete 和轮询**

运行下面的完整脚本。它只输出状态和资产 ULID，不输出登录 Token 或签名 URL：

```powershell
$ErrorActionPreference='Stop'
Set-Location 'C:\Users\33141\Documents\New project'
$script=@'
const crypto = require('crypto');
const fs = require('fs');
const zlib = require('zlib');
const automator = require('./.release/automator/node_modules/miniprogram-automator');

function crc32(buffer) {
  let crc = 0xffffffff;
  for (const byte of buffer) {
    crc ^= byte;
    for (let bit = 0; bit < 8; bit += 1) crc = (crc >>> 1) ^ ((crc & 1) ? 0xedb88320 : 0);
  }
  return (crc ^ 0xffffffff) >>> 0;
}
function chunk(type, data) {
  const typeBytes = Buffer.from(type, 'ascii');
  const length = Buffer.alloc(4);
  const checksum = Buffer.alloc(4);
  length.writeUInt32BE(data.length, 0);
  checksum.writeUInt32BE(crc32(Buffer.concat([typeBytes, data])), 0);
  return Buffer.concat([length, typeBytes, data, checksum]);
}
function safePng() {
  const width = 512, height = 512;
  const raw = Buffer.alloc((width * 4 + 1) * height);
  for (let y = 0; y < height; y += 1) {
    const row = y * (width * 4 + 1);
    for (let x = 0; x < width; x += 1) {
      let r = 224, g = 239, b = 250;
      if ((x - 390) ** 2 + (y - 110) ** 2 < 52 ** 2) { r = 255; g = 198; b = 76; }
      if (y > 330 + Math.floor(18 * Math.sin(x / 45))) { r = 92; g = 167; b = 118; }
      const p = row + 1 + x * 4;
      raw[p] = r; raw[p + 1] = g; raw[p + 2] = b; raw[p + 3] = 255;
    }
  }
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width, 0);
  header.writeUInt32BE(height, 4);
  header[8] = 8; header[9] = 6;
  return Buffer.concat([
    Buffer.from([137,80,78,71,13,10,26,10]),
    chunk('IHDR', header),
    chunk('IDAT', zlib.deflateSync(raw, { level: 9 })),
    chunk('IEND', Buffer.alloc(0))
  ]);
}
const sleep = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));

async function main() {
  const miniProgram = await automator.connect({ wsEndpoint: 'ws://127.0.0.1:9420' });
  try {
    const token = await miniProgram.callWxMethod('getStorageSync', 'domainA.token');
    if (typeof token !== 'string' || token.length < 32) throw new Error('AUTH_TOKEN_NOT_AVAILABLE');
    const base = 'https://zhanduiwang.top/api/v1';
    async function api(path, options = {}) {
      const response = await fetch(base + path, {
        ...options,
        headers: {
          'content-type': 'application/json',
          Authorization: `Bearer ${token}`,
          ...(options.headers || {})
        }
      });
      const body = await response.json();
      if (!response.ok || body.code !== 0) throw new Error(`API_FAILED path=${path} status=${response.status} code=${body.code}`);
      return body.data;
    }
    const me = await api('/auth/me');
    if (!me || !me.id) throw new Error('AUTH_ME_FAILED');
    const image = safePng();
    const md5 = crypto.createHash('md5').update(image).digest('base64');
    const session = await api('/miniapp/media/uploads', {
      method: 'POST',
      body: JSON.stringify({
        purpose: 'product',
        sizeBytes: image.length,
        contentMD5: md5,
        contentType: 'image/png',
        idempotencyKey: `product_image:${Date.now()}:${crypto.randomBytes(6).toString('hex')}`
      })
    });
    console.log(`AUTHORIZE_OK asset=${session.assetId} bytes=${image.length}`);
    const put = await fetch(session.uploadUrl, {
      method: 'PUT',
      body: image,
      headers: {
        'Content-Type': session.requiredHeaders.contentType,
        'Content-MD5': session.requiredHeaders.contentMD5,
        'Content-Length': String(session.requiredHeaders.contentLength),
        'x-cos-forbid-overwrite': session.requiredHeaders.forbidOverwrite
      }
    });
    if (!put.ok) throw new Error(`COS_PUT_FAILED status=${put.status}`);
    const completed = await api(`/miniapp/media/uploads/${session.ticketId}/complete`, { method: 'POST', body: '{}' });
    console.log(`COMPLETE_OK state=${completed.state} review=${completed.reviewStatus}`);
    const deadline = Date.now() + 300000;
    while (Date.now() < deadline) {
      const status = await api(`/miniapp/media/assets/${session.assetId}`);
      console.log(`MEDIA_STATUS state=${status.state} review=${status.reviewStatus} canUse=${status.canUse}`);
      if (status.state === 'approved' && status.canUse === true && status.technicalStatus === 'final_ready' && status.reviewStatus === 'approved' && status.ticketStatus === 'consumed' && status.recovery === 'none') {
        fs.writeFileSync(
          '.release/automator/latest-media-acceptance.json',
          JSON.stringify({ assetId: session.assetId, ticketId: session.ticketId, approvedAt: new Date().toISOString() }),
          { encoding: 'utf8', mode: 0o600 }
        );
        console.log(`MEDIA_APPROVED asset=${session.assetId} ticket=${session.ticketId}`);
        return;
      }
      if (['rejected','review_failed','expired','upload_failed','deleted'].includes(status.state)) throw new Error(`MEDIA_TERMINAL state=${status.state}`);
      await sleep(2000);
    }
    throw new Error(`MEDIA_TIMEOUT asset=${session.assetId}`);
  } finally {
    miniProgram.disconnect();
  }
}
main().catch(error => { console.error(error.message || String(error)); process.exitCode = 1; });
'@
$script | & 'D:\3314\微信web开发者工具\node-18.exe' -
if ($LASTEXITCODE -ne 0) { throw '真实媒体验收失败' }
```

Expected: 依次出现 `AUTHORIZE_OK`、`COMPLETE_OK` 和 `MEDIA_APPROVED`；严格状态组合全部成立。

- [ ] **Step 3: 从 Nginx 与数据库交叉验证回调结果**

```powershell
$key='C:\Users\33141\.ssh\id_ed25519_zhandui'
ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes admin@8.138.227.107 "sudo -n sh -c 'grep /api/v1/miniapp/media/wechat/callback /var/log/nginx/campus.access.log | tail -n 10'"
```

Expected: 新产生的微信 `POST /api/v1/miniapp/media/wechat/callback` 为 HTTP 200，不再是 400。

读取脚本写入的非敏感验收结果，严格校验 ULID 后执行只读 SQL；不查询 OpenID、对象 key、version 或 trace：

```powershell
$ErrorActionPreference='Stop'
Set-Location 'C:\Users\33141\Documents\New project'
$acceptance=Get-Content -Raw -Encoding UTF8 '.release\automator\latest-media-acceptance.json' | ConvertFrom-Json
$assetId=[string]$acceptance.assetId
if ($assetId -notmatch '^[0-9A-HJKMNP-TV-Z]{26}$') { throw "验收资产 ID 非法：$assetId" }
$sql=@"
SELECT
  a.asset_public_id,
  a.storage_stage,
  a.technical_status,
  t.status AS ticket_status,
  p.status AS projection_status,
  p.applied_decision_version
FROM media_assets AS a
JOIN media_upload_tickets AS t ON t.asset_id = a.id
JOIN media_review_projections AS p ON p.asset_id = a.id
WHERE a.asset_public_id = '$assetId';
"@
$encoded=[Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($sql))
$key='C:\Users\33141\.ssh\id_ed25519_zhandui'
ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes admin@8.138.227.107 "echo '$encoded' | base64 -d | sudo -n mysql --defaults-extra-file=/etc/campus-circle/mysql-admin.cnf -N -B campus_circle"
if ($LASTEXITCODE -ne 0) { throw '媒体状态 SQL 验证失败' }
```

Expected: `final / final_ready / consumed / approved`，且 `applied_decision_version >= 1`。执行时将资产 ULID 作为受控 SQL 参数，不拼接其他用户输入。

- [ ] **Step 4: 记录验收边界并进入既有商品流程**

验收报告必须明确：

- callback code PASS；
- production deploy PASS；
- WeChat callback POST 200 PASS；
- new media strict approved PASS；
- 未对待审资产做人工批准或数据库直改；
- 带图商品创建、首位 superadmin 初始化和商品审核发布属于后续既有流程，需单独确认权限操作后继续。

如果 5 分钟内仍未收到新回调，状态记录为 `runtime PENDING`，保留服务运行并只读检查日志；不得把超时描述为通过。

## 回滚命令

若 Task 4 后健康检查或 Task 5 回调行为异常，使用 Task 4 输出的备份目录执行：

```powershell
$key='C:\Users\33141\.ssh\id_ed25519_zhandui'
$backup=(ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes admin@8.138.227.107 "sudo -n find /opt/cutover-backups -maxdepth 1 -type d -name 'callback-fix-*' -printf '%T@ %p\n' | sort -nr | head -n1 | cut -d' ' -f2-").Trim()
if ($backup -notmatch '^/opt/cutover-backups/callback-fix-[0-9_]+$') { throw "备份目录异常：$backup" }
ssh -i $key -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes admin@8.138.227.107 "sudo -n install -o root -g campus -m 0750 '$backup/campus-api' /opt/campus-circle/current/bin/campus-api && sudo -n systemctl restart campus-api && sudo -n systemctl is-active campus-api && curl -fsS http://127.0.0.1:8080/healthz"
if ($LASTEXITCODE -ne 0) { throw 'API 回滚失败' }
```

Expected: `active` 且回环健康检查成功。回滚不修改微信平台、Nginx、MySQL、Meilisearch或 COS 数据。
