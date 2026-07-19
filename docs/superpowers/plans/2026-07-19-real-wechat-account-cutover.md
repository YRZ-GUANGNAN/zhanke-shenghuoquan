# 真实微信账号切换与 mock 用户审计 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让真实微信 OpenID 稳定映射数据库用户号，并提供只读 mock 用户关联数据审计，不自动合并或删除历史数据。

**Architecture:** 保留现有 `wx.login` 和用户表唯一 OpenID 设计，为 code2session 增加可测试的 API base 注入并验证相同 OpenID 复用同一用户。新增只读 Go 审计命令统计 `mock_openid_*` 账号及关联业务数量；真实 AppSecret 只由用户写入未跟踪环境，代码、计划和报告均不接触密钥值。

**Tech Stack:** Go 1.25、net/http/httptest、Gin、GORM、MySQL 8、Taro 4、PowerShell

---

## 文件结构

- Modify: `backend/internal/module/user/service.go` — 注入微信 API base，保持真实 OpenID 查找/创建。
- Modify: `backend/internal/module/user/service_test.go` — 真实 code2session、同 OpenID 稳定用户号和错误响应测试。
- Modify: `backend/cmd/server/main.go` — 把 `cfg.WechatAPIBase` 传给用户服务。
- Modify: `backend/internal/config/config_test.go` — 真实/模拟配置互斥与默认关闭合同。
- Create: `backend/cmd/audit-mock-users/main.go` — 只读审计命令。
- Create: `backend/cmd/audit-mock-users/main_test.go` — 查询范围和无删除语句测试。
- Create: `docs/runbooks/REAL-WECHAT-ACCOUNT-CUTOVER.md` — 不含密钥的切换和验收步骤。
- Create at execution time, never commit: `docs/local-reports/mock-user-audit.json` — 本地审计结果。
- Preserve without staging: `.env.example`、`backend/.env.example`、`apps/miniapp/project.config.json`、`apps/miniapp/project.private.config.example.json`、`docs/G-7-QUICKSTART-LOCAL-VERIFY.md` — 这些文件已有用户修改，本计划不覆盖。

### Task 1: 可测试的真实 code2session 客户端

**Files:**
- Modify: `backend/internal/module/user/service.go`
- Modify: `backend/internal/module/user/service_test.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: 写失败的真实微信稳定用户测试**

```go
func TestRealWechatCodesWithSameOpenIDReuseStableUser(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/sns/jscode2session" || r.URL.Query().Get("appid") != "wx-real" || r.URL.Query().Get("secret") != "secret-real" {
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"openid":"real-openid-41"}`)
	}))
	defer server.Close()

	repo := newMemoryRepository()
	service := NewService(repo, testSecret, "wx-real", "secret-real", false,
		WithAppEnv("dev"), WithWechatAPIBase(server.URL))
	first, err := service.WechatLogin(context.Background(), LoginInput{Code: "code-one"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.WechatLogin(context.Background(), LoginInput{Code: "code-two"})
	if err != nil {
		t.Fatal(err)
	}
	if first.User.ID != second.User.ID || len(repo.users) != 1 || calls.Load() != 2 {
		t.Fatalf("first=%d second=%d users=%d calls=%d", first.User.ID, second.User.ID, len(repo.users), calls.Load())
	}
}
```

再增加微信返回 `errcode`、空 OpenID、非 JSON、HTTP 500 和超时测试；错误信息不得包含 AppSecret。

- [ ] **Step 2: 运行测试确认 option 缺失**

Run: `go test ./internal/module/user -run TestRealWechatCodesWithSameOpenIDReuseStableUser -count=1`

Expected: FAIL，错误包含 `undefined: WithWechatAPIBase`。

- [ ] **Step 3: 实现 API base 注入**

```go
const defaultWechatAPIBase = "https://api.weixin.qq.com"

func WithWechatAPIBase(value string) ServiceOption {
	return func(s *Service) {
		if normalized := strings.TrimRight(strings.TrimSpace(value), "/"); normalized != "" {
			s.wechatAPIBase = normalized
		}
	}
}
```

`Service` 增加 `wechatAPIBase string`，构造时默认 `defaultWechatAPIBase`。`exchangeCode` 使用：

```go
endpoint := s.wechatAPIBase + "/sns/jscode2session?appid=" + url.QueryEscape(s.wechatAppID) +
	"&secret=" + url.QueryEscape(s.wechatSecret) +
	"&js_code=" + url.QueryEscape(code) +
	"&grant_type=authorization_code"
```

请求返回后先执行以下状态和解码限制：

```go
if resp.StatusCode != http.StatusOK {
	return "", fmt.Errorf("wechat code exchange http status %d", resp.StatusCode)
}
decoder := json.NewDecoder(io.LimitReader(resp.Body, 32<<10))
if err := decoder.Decode(&result); err != nil {
	return "", errors.New("wechat code exchange returned invalid JSON")
}
```

`main.go` 的 `user.NewService` options 加 `user.WithWechatAPIBase(cfg.WechatAPIBase)`。任何日志和返回错误只包含微信 `errcode` 与经长度限制的 `errmsg`，不包含完整 URL。

- [ ] **Step 4: 运行用户与服务器测试**

Run: `go test ./internal/module/user ./cmd/server -count=1`

Expected: PASS。

- [ ] **Step 5: 提交真实登录客户端**

```powershell
git add -- backend/internal/module/user/service.go backend/internal/module/user/service_test.go backend/cmd/server/main.go
git commit -m "修复：稳定映射真实微信用户号"
```

### Task 2: 配置安全合同

**Files:**
- Modify: `backend/internal/config/config_test.go`
- Create: `scripts/check-real-wechat-secret-hygiene.cjs`
- Modify: `package.json`

- [ ] **Step 1: 写配置和密钥卫生测试**

```go
func TestRealWechatConfigurationDisablesMockFallback(t *testing.T) {
	t.Setenv("APP_ENV", "dev")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("WECHAT_APP_ID", "wx0dacef976daa8e55")
	t.Setenv("WECHAT_APP_SECRET", "test-secret-not-for-production")
	t.Setenv("ALLOW_MOCK_WECHAT_LOGIN", "false")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AllowMockWechat || cfg.WechatAppID != "wx0dacef976daa8e55" {
		t.Fatalf("cfg=%+v", cfg)
	}
}
```

密钥卫生脚本读取 Git 跟踪文件，拒绝出现 `WECHAT_APP_SECRET=` 后的非空值，并拒绝文档出现真实密钥形态：

```js
const cp=require('node:child_process')
const fs=require('node:fs')
const files=cp.execFileSync('git',['ls-files'],{encoding:'utf8'}).trim().split(/\r?\n/).filter(Boolean)
const configurationFiles=files.filter((file)=>(/(^|\/)\.env($|\.)/.test(file)||/project(?:\.private)?\.config.*\.json$/.test(file)))
for(const file of configurationFiles){
  const text=fs.readFileSync(file,'utf8')
  if(/^WECHAT_APP_SECRET=.+$/m.test(text))throw new Error(`tracked WeChat secret in ${file}`)
}
console.log('real wechat secret hygiene: ok')
```

- [ ] **Step 2: 运行测试**

Run: `go test ./internal/config -run 'TestRealWechatConfiguration|TestProduction' -count=1`

Expected: PASS；现有 config 已满足成对配置和生产禁 mock 合同。

Run: `node scripts/check-real-wechat-secret-hygiene.cjs`

Expected: `real wechat secret hygiene: ok`。

- [ ] **Step 3: 把卫生检查加入根验证**

在 `package.json` 增加 `check:real-wechat-secret-hygiene`，并放入 `verify` 的类型检查之前。不要编辑或暂存当前已有用户改动的五个配置文件。

- [ ] **Step 4: 运行根验证并提交**

Run: `npm.cmd run check:real-wechat-secret-hygiene`

Expected: PASS。

```powershell
git add -- backend/internal/config/config_test.go scripts/check-real-wechat-secret-hygiene.cjs package.json
git commit -m "测试：阻止微信密钥进入版本库"
```

### Task 3: mock 用户只读审计命令

**Files:**
- Create: `backend/cmd/audit-mock-users/main.go`
- Create: `backend/cmd/audit-mock-users/main_test.go`

- [ ] **Step 1: 写失败的审计查询测试**

```go
func TestAuditQueriesAreReadOnlyAndCountBusinessRelations(t *testing.T) {
	queries := auditQueries()
	joined := strings.ToUpper(strings.Join(queries, "\n"))
	for _, forbidden := range []string{"DELETE ", "UPDATE ", "INSERT ", "DROP ", "TRUNCATE "} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("mutating query found: %s", forbidden)
		}
	}
	for _, required := range []string{"mock_openid\\_%", "posts", "products", "im_conversations", "delivery_orders"} {
		if !strings.Contains(strings.Join(queries, "\n"), required) {
			t.Fatalf("missing relation %s", required)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认命令缺失**

Run: `go test ./cmd/audit-mock-users -count=1`

Expected: FAIL，目录或 `auditQueries` 尚不存在。

- [ ] **Step 3: 实现只读 JSON 报告**

```go
type auditRow struct {
	UserID uint64 `json:"userId"`
	Nickname string `json:"nickname"`
	Posts int64 `json:"posts"`
	Products int64 `json:"products"`
	IMRelations int64 `json:"imRelations"`
	DeliveryOrders int64 `json:"deliveryOrders"`
	IdentityApplications int64 `json:"identityApplications"`
	SafeToConsiderForCleanup bool `json:"safeToConsiderForCleanup"`
}

type auditReport struct {
	GeneratedAt time.Time `json:"generatedAt"`
	Users []auditRow `json:"users"`
	Total int `json:"total"`
	OptionalTablesMissing []string `json:"optionalTablesMissing"`
}
```

命令只接受 `MYSQL_DSN` 和可选 `-output`，查询 `users.open_id LIKE 'mock_openid\\_%'`，通过 `posts.user_id`、`products.seller_id`、`im_conversations.buyer_id/seller_id` 和 `delivery_orders.merchant_user_id/rider_user_id` 的分组子查询统计数量。先通过 `information_schema.tables` 检查可选的 `identity_applications` 是否存在；存在时按 `user_id` 统计，不存在时填 0 并把表名加入 `OptionalTablesMissing`。`SafeToConsiderForCleanup` 只有所有关联计数均为 0 时为 true。报告不输出 OpenID、手机号、token 或消息正文；命令不提供删除参数。

- [ ] **Step 4: 运行命令测试和本地审计**

Run: `go test ./cmd/audit-mock-users -count=1`

Expected: PASS。

Run: `go run ./cmd/audit-mock-users -output ../docs/local-reports/mock-user-audit.json`

Expected: 生成 JSON，并输出 `mock users audited: N`；`docs/local-reports/` 必须保持 Git 忽略或明确不暂存。

- [ ] **Step 5: 提交只读审计命令**

```powershell
git add -- backend/cmd/audit-mock-users/main.go backend/cmd/audit-mock-users/main_test.go
git commit -m "工具：增加虚拟用户只读审计"
```

### Task 4: 真实账号切换运行手册

**Files:**
- Create: `docs/runbooks/REAL-WECHAT-ACCOUNT-CUTOVER.md`

- [ ] **Step 1: 编写不含密钥的运行手册**

手册包含以下精确 PowerShell 步骤，用户只在本机编辑 `backend/.env`：

```powershell
Select-String -LiteralPath 'apps/miniapp/project.config.json' -Pattern '"appid"'
Select-String -LiteralPath 'backend/.env' -Pattern '^(APP_ENV|WECHAT_APP_ID|ALLOW_MOCK_WECHAT_LOGIN)='
```

要求值为 `APP_ENV=dev`、与开发者工具一致的 `WECHAT_APP_ID`、`ALLOW_MOCK_WECHAT_LOGIN=false`。`WECHAT_APP_SECRET` 必须填写但禁止通过命令打印。

启动与健康检查：

```powershell
go run ./cmd/server
Invoke-RestMethod -Uri 'http://127.0.0.1:8080/healthz'
```

开发者工具清理旧 token 后重新编译；连续两次启动记录“我的”用户号，要求相同。失败排查仅显示微信 `errcode`，不复制 Secret。

- [ ] **Step 2: 明确旧 mock 数据边界**

手册写明：审计报告不授权删除；无关联数据也必须另行确认后才可清理；有业务关联的数据必须先设计显式归属迁移；本任务不执行合并、改绑或删除。

- [ ] **Step 3: 运行文档与密钥卫生检查**

Run: `node scripts/check-real-wechat-secret-hygiene.cjs`

Expected: PASS。

Run: `rg -n "WECHAT_APP_SECRET=.+" docs/runbooks/REAL-WECHAT-ACCOUNT-CUTOVER.md`

Expected: 无输出。

- [ ] **Step 4: 提交运行手册**

```powershell
git add -- docs/runbooks/REAL-WECHAT-ACCOUNT-CUTOVER.md
git commit -m "文档：增加真实微信账号切换手册"
```

### Task 5: 自动验证与真实微信验收

**Files:**
- Create: `docs/REAL-WECHAT-ACCOUNT-ACCEPTANCE-REPORT.md`

- [ ] **Step 1: 运行自动验证**

Run: `go test ./internal/module/user ./internal/config ./cmd/audit-mock-users ./cmd/server -count=1`

Expected: PASS。

Run: `npm.cmd run check:real-wechat-secret-hygiene`

Expected: PASS。

- [ ] **Step 2: 检查当前运行配置但不输出 Secret**

```powershell
$lines=Get-Content -LiteralPath 'backend/.env' -Encoding UTF8
$appID=($lines|Where-Object{$_ -match '^WECHAT_APP_ID='}|Select-Object -First 1)
$mock=($lines|Where-Object{$_ -match '^ALLOW_MOCK_WECHAT_LOGIN='}|Select-Object -First 1)
$hasSecret=[bool]($lines|Where-Object{$_ -match '^WECHAT_APP_SECRET=.+$'})
Write-Output $appID
Write-Output $mock
Write-Output "WECHAT_APP_SECRET_PRESENT=$hasSecret"
```

Expected: AppID 与项目一致、mock=false、Secret present=true；绝不打印 Secret 值。

- [ ] **Step 3: 真实微信开发者工具验收**

删除 `domainA.token`，重新编译两次，每次记录页面显示的用户号；两次必须相同。后台数据库只允许用用户号查询并确认一条用户，不在截图或报告记录 OpenID。若缺少 Secret 或微信接口拒绝请求，此项标记 `PENDING`。

- [ ] **Step 4: 写验收报告**

报告记录自动测试、配置存在性、第一次用户号、第二次用户号是否一致、mock 审计报告总数。不得记录 OpenID、Secret、token、手机号或消息正文。

- [ ] **Step 5: 提交非敏感验收报告**

```powershell
git add -- docs/REAL-WECHAT-ACCOUNT-ACCEPTANCE-REPORT.md
git commit -m "文档：记录真实微信账号验收结果"
```
