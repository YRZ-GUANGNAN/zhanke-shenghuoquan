# G-7 本地最快拿到登录 token 指引

## 核心结论

当前完整 `cmd/server` 的无代码可行方案是：

**方案乙：填真实微信 AppID/AppSecret，在微信开发者工具里用真实 `wx.login` 换 JWT。**

当前代码下，**方案甲（只开 `ALLOW_MOCK_WECHAT_LOGIN=true`、不配真实微信凭证）不能用于完整 `cmd/server`**。原因是：

1. `config.Validate()` 在 `APP_ENV != prod` 且 `ALLOW_MOCK_WECHAT_LOGIN=true` 时，允许 `WECHAT_APP_ID` / `WECHAT_APP_SECRET` 同时为空；
2. 但 `backend/cmd/server/main.go` 会无条件初始化旧媒体链路的 `legacymedia.NewWechatSafety(cfg.WechatAppID, cfg.WechatAppSecret, ...)`；
3. `NewWechatSafety()` 要求 AppID 和 Secret 都非空，否则后端启动直接失败；
4. 如果为了绕过启动失败而填假 AppID/Secret，`user.Service.exchangeCode()` 又会因为凭证非空而走真实微信 `code2session`，不会走 mock。

所以：**要在当前代码、当前完整后端里最快拿 token，请走真实微信登录。**

## 1. 真实代码依据

### 1.1 `ALLOW_MOCK_WECHAT_LOGIN` 的配置规则

文件：`backend/internal/config/config.go`

真实读取项：

```go
WechatAppID:     env("WECHAT_APP_ID", ""),
WechatAppSecret: env("WECHAT_APP_SECRET", ""),
AllowMockWechat: envBool("ALLOW_MOCK_WECHAT_LOGIN", false),
```

真实校验规则：

```go
if (c.WechatAppID == "") != (c.WechatAppSecret == "") {
	return errors.New("WECHAT_APP_ID and WECHAT_APP_SECRET must be configured together")
}
```

本地开发分支：

```go
} else if c.WechatAppID == "" && !c.AllowMockWechat {
	return fmt.Errorf("WeChat credentials are missing; set ALLOW_MOCK_WECHAT_LOGIN=true only for local development")
}
```

含义：

- AppID 和 Secret 必须同时为空或同时有值。
- `APP_ENV != prod` 时，如果二者都为空，则必须 `ALLOW_MOCK_WECHAT_LOGIN=true` 才能通过 `config.Validate()`。
- `APP_ENV=prod` 时必须真实 AppID/Secret，且禁止 mock。

### 1.2 `/api/v1/auth/wechat-login` 的真实请求体

文件：`backend/internal/module/user/handler.go`

真实路由：

```go
rg.POST("/wechat-login", h.login)
```

完整路径由 `main.go` 注册为：

```text
POST /api/v1/auth/wechat-login
```

真实请求体字段：

```go
var req struct {
	Code     string `json:"code"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	School   string `json:"school"`
	Campus   string `json:"campus"`
}
```

真正必填的是 `code`，因为 `Service.WechatLogin()` 里会校验：

```go
if input.Code == "" {
	return LoginResult{}, errors.New("wechat code is required")
}
```

成功返回：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "token": "...JWT...",
    "user": {
      "id": 123,
      "nickname": "...",
      "role": "user",
      "status": "active"
    }
  }
}
```

JWT 有效期来自 `backend/internal/module/user/service.go`：

```go
const accessTokenTTL = 15 * time.Minute
```

所以本地验收慢一点时，401 很可能只是 token 过期，需要重新登录。

### 1.3 mock 登录真实逻辑

文件：`backend/internal/module/user/service.go`

关键代码：

```go
func (s *Service) exchangeCode(ctx context.Context, code string) (string, error) {
	if s.wechatAppID == "" || s.wechatSecret == "" {
		if !s.allowMockWechat {
			return "", ErrWechatNotConfigured
		}
		return "mock_openid_" + code, nil
	}
	endpoint := "https://api.weixin.qq.com/sns/jscode2session?appid=" + ...
```

含义：

- 只有当 `wechatAppID == ""` 或 `wechatSecret == ""` 时，才可能走 mock；
- 如果 AppID/Secret 都有值，即使 `ALLOW_MOCK_WECHAT_LOGIN=true`，也会走真实微信 `code2session`；
- mock 时 `code` 可以是任意非空字符串，最终 openid 是：

```text
mock_openid_<code>
```

但这只说明 user service 支持 mock，不代表完整 `cmd/server` 能在空微信凭证下启动。

### 1.4 完整 `cmd/server` 对微信安全客户端的启动要求

文件：`backend/cmd/server/main.go`

关键代码：

```go
wechatMediaSafety, err := legacymedia.NewWechatSafety(cfg.WechatAppID, cfg.WechatAppSecret, cfg.WechatAPIBase)
if err != nil {
	log.Fatalf("configure WeChat media safety: %v", err)
}
```

文件：`backend/internal/pkg/media/wechat.go`

关键代码：

```go
func NewWechatSafety(appID, appSecret, apiBase string) (*WechatSafety, error) {
	if appID == "" || appSecret == "" {
		return nil, errors.New("WeChat app id and secret are required for media safety")
	}
```

含义：

- 完整 `cmd/server` 不是只启动 auth；
- 它还会启动旧 media 的微信内容安全客户端；
- 所以 AppID/Secret 为空时，即使 `config.Validate()` 允许 mock，后端仍会在 `NewWechatSafety()` 处 fatal。

## 2. 推荐方案：真实微信凭证 + DevTools `wx.login`

### 2.1 backend/.env 确认

`backend/.env` 里必须有：

```dotenv
APP_ENV=dev
WECHAT_APP_ID=真实小程序AppID
WECHAT_APP_SECRET=真实小程序AppSecret
```

`ALLOW_MOCK_WECHAT_LOGIN` 可以不填，或填：

```dotenv
ALLOW_MOCK_WECHAT_LOGIN=false
```

注意：微信开发者工具项目 AppID 必须和后端 `.env` 的 `WECHAT_APP_ID` 是同一个小程序。

可只读核对 AppID：

```powershell
Select-String apps/miniapp/project.config.json -Pattern '"appid"'
Select-String backend/.env -Pattern '^WECHAT_APP_ID='
```

不要把 `WECHAT_APP_SECRET` 打印到报告或聊天里。

### 2.2 在微信开发者工具里一键登录并写入 token

确保后端健康：

```powershell
Invoke-RestMethod http://127.0.0.1:8080/healthz
```

预期：

```json
{"code":0,"message":"ok","data":{"status":"ok"}}
```

然后在微信开发者工具 Console 执行：

```javascript
wx.login({
  success(login) {
    console.log('wx.login code =', login.code)
    wx.request({
      url: 'http://127.0.0.1:8080/api/v1/auth/wechat-login',
      method: 'POST',
      header: { 'content-type': 'application/json' },
      data: {
        code: login.code,
        nickname: 'Local Buyer',
        school: 'Local',
        campus: 'Local'
      },
      success(res) {
        console.log('wechat-login response =', res.data)
        const token = res.data && res.data.data && res.data.data.token
        if (!token) {
          console.error('No token in response')
          return
        }
        wx.setStorageSync('domainA.token', token)
        console.log('domainA.token saved, length =', token.length)
      },
      fail(err) {
        console.error('wechat-login request failed', err)
      }
    })
  },
  fail(err) {
    console.error('wx.login failed', err)
  }
})
```

这一步成功后，小程序前端会自动从 storage key 读取 token：

```text
domainA.token
```

代码位置：`apps/miniapp/src/services/api.ts`

```ts
const token = Taro.getStorageSync<string>('domainA.token')
...
Authorization: `Bearer ${token}`
```

### 2.3 验证 token 可用

在微信开发者工具 Console 执行：

```javascript
wx.request({
  url: 'http://127.0.0.1:8080/api/v1/auth/me',
  method: 'GET',
  header: {
    Authorization: 'Bearer ' + wx.getStorageSync('domainA.token')
  },
  success(res) {
    console.log('/auth/me =', res.statusCode, res.data)
  },
  fail(err) {
    console.error('/auth/me failed', err)
  }
})
```

预期：

```text
statusCode = 200
data.code = 0
data.data.id 有值
```

### 2.4 PowerShell 备选路径

如果你更想把 `wx.login` 的 code 复制出来，再用 PowerShell 换 token：

1. 微信开发者工具 Console：

```javascript
wx.login({ success: r => console.log(r.code) })
```

2. 立即复制 code。它短时有效且通常只能用一次。

3. PowerShell：

```powershell
$code = "<粘贴wx.login得到的code>"
$body = @{
  code = $code
  nickname = "Local Buyer"
  school = "Local"
  campus = "Local"
} | ConvertTo-Json -Compress

$res = Invoke-RestMethod `
  -Method Post `
  -Uri "http://127.0.0.1:8080/api/v1/auth/wechat-login" `
  -ContentType "application/json" `
  -Body $body

$token = $res.data.token
$token
```

4. 微信开发者工具 Console：

```javascript
wx.setStorageSync('domainA.token', '<粘贴上一步token>')
```

## 3. 当前本机现象怎么解释

当前 `backend/.env` 已配置 `WECHAT_APP_ID` 和 `WECHAT_APP_SECRET`，所以随便传一个 fake code 不会走 mock，而是会走真实微信 `code2session`。

我用 fake code 对当前本地后端试过：

```text
POST /api/v1/auth/wechat-login
code = codex-local-fake-code
```

结果是：

```text
401
```

这符合真实代码行为：凭证非空时不会 mock，fake code 会被微信拒绝。

因此 owner 当前应在微信开发者工具里用真实 `wx.login` code，而不是随手填字符串。

## 4. 常见失败与处理

### 4.1 `/auth/wechat-login` 返回 401

常见原因：

1. `wx.login` code 过期或已经被用过；
2. 微信开发者工具项目 AppID 与 `backend/.env` 的 `WECHAT_APP_ID` 不一致；
3. `WECHAT_APP_SECRET` 填错；
4. 后端没有重启，仍使用旧 `.env`；
5. 系统代理或网络导致后端访问微信 `api.weixin.qq.com` 失败。

处理：

```javascript
wx.login({ success: r => console.log(r.code) })
```

重新拿一个新 code，马上换 token。

### 4.2 `/auth/me` 返回 401

常见原因：

1. 没有写入 `domainA.token`；
2. token 已过期；
3. 用户被封禁；
4. `token_version` 被更新，旧 token 被撤销。

处理：

```javascript
wx.removeStorageSync('domainA.token')
```

重新执行本指引第 2.2 节。

### 4.3 小程序请求本地后端失败

检查：

```powershell
Invoke-RestMethod http://127.0.0.1:8080/healthz
```

微信开发者工具里也要关闭“校验合法域名、web-view、TLS 版本以及 HTTPS 证书”。

### 4.4 token 拿到了，但上传/外卖还是 401

确认小程序 storage 里是同一个 key：

```javascript
wx.getStorageSync('domainA.token')
```

如果 token 是空字符串，重新执行：

```javascript
wx.setStorageSync('domainA.token', '<JWT>')
```

## 5. 如果一定要 mock 登录，当前代码需要的最小补丁

这不是本任务要实施的内容，只是结论。

当前代码里 `config.Validate()` 对 mock 已经基本够用：`APP_ENV=dev + ALLOW_MOCK_WECHAT_LOGIN=true + WECHAT_APP_ID/SECRET 同时为空` 可以过配置校验。

真正阻断完整后端启动的是 `main.go` 无条件初始化旧 media 的 `NewWechatSafety()`。

最小调整方向是：

1. 在 `APP_ENV != prod` 且 `ALLOW_MOCK_WECHAT_LOGIN=true` 且 `WECHAT_APP_ID/SECRET` 为空时，不调用 `legacymedia.NewWechatSafety()`；
2. 给旧 media service 注入一个本地 no-op `SafetyChecker`，例如 `Submit()` 返回一个本地 trace id；
3. 保持生产环境仍然强制真实 AppID/Secret；
4. 不改变 `user.Service.exchangeCode()` 的生产行为。

补丁后才可以使用：

```dotenv
APP_ENV=dev
WECHAT_APP_ID=
WECHAT_APP_SECRET=
ALLOW_MOCK_WECHAT_LOGIN=true
```

然后任意非空 code：

```powershell
$body = @{
  code = "local-buyer-001"
  nickname = "Local Buyer"
  school = "Local"
  campus = "Local"
} | ConvertTo-Json -Compress

Invoke-RestMethod `
  -Method Post `
  -Uri "http://127.0.0.1:8080/api/v1/auth/wechat-login" `
  -ContentType "application/json" `
  -Body $body
```

在未做这个补丁前，不要把“mock 开关 + 空微信凭证”当成完整 `cmd/server` 的可用路径。

## 最短操作卡片

1. 确认后端 `.env` 有真实：

```dotenv
WECHAT_APP_ID=真实小程序AppID
WECHAT_APP_SECRET=真实小程序AppSecret
```

2. 后端健康：

```powershell
Invoke-RestMethod http://127.0.0.1:8080/healthz
```

3. 微信开发者工具 Console 执行：

```javascript
wx.login({
  success(login) {
    wx.request({
      url: 'http://127.0.0.1:8080/api/v1/auth/wechat-login',
      method: 'POST',
      header: { 'content-type': 'application/json' },
      data: { code: login.code, nickname: 'Local Buyer', school: 'Local', campus: 'Local' },
      success(res) {
        const token = res.data?.data?.token
        console.log(res.data)
        if (token) wx.setStorageSync('domainA.token', token)
      }
    })
  }
})
```

4. 验证：

```javascript
wx.request({
  url: 'http://127.0.0.1:8080/api/v1/auth/me',
  header: { Authorization: 'Bearer ' + wx.getStorageSync('domainA.token') },
  success: console.log
})
```

成功后，上传图片、发帖、外卖接口都会带上同一个 `domainA.token`。
