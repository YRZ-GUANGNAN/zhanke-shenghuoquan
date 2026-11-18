package adminauth

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/pkg/response"
)

const (
	contextAuthKind = "adminauth.kind"
	contextSession  = "adminauth.session"
	contextAccount  = "adminauth.account"
	authKindCookie  = "cookie"
)

type Middleware struct {
	sessions *Service
}

func NewMiddleware(sessions *Service) *Middleware {
	return &Middleware{sessions: sessions}
}

// Required 只接受后台管理员会话 Cookie（__Host- 前缀 + HttpOnly + Secure + SameSite=Strict）。
// 小程序登录态（Authorization: Bearer）不再被后台 API 接受。
func (m *Middleware) Required() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, cookieErr := c.Cookie(CookieName)
		if cookieErr != nil || strings.TrimSpace(cookie) == "" {
			response.Fail(c, http.StatusUnauthorized, "authentication required")
			c.Abort()
			return
		}
		authenticated, err := m.sessions.Authenticate(c, cookie)
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, "authentication required")
			c.Abort()
			return
		}
		if mutationMethod(c.Request.Method) {
			if !sameOrigin(c.Request) || m.sessions.VerifyCSRF(authenticated.Session, c.GetHeader("X-CSRF-Token")) != nil {
				response.Fail(c, http.StatusForbidden, "request origin or csrf token invalid")
				c.Abort()
				return
			}
		}
		c.Set(contextAccount, authenticated.Account)
		c.Set(contextSession, authenticated)
		c.Set(contextAuthKind, authKindCookie)
		c.Next()
	}
}

// AdminRequired 校验管理员账号处于启用状态且角色合法（admin/superadmin）。
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		account, ok := CurrentAccount(c)
		if !ok || account.Status != StatusActive || !account.Role.Valid() {
			response.Fail(c, http.StatusForbidden, "admin required")
			c.Abort()
			return
		}
		c.Next()
	}
}

// SuperAdminRequired 校验管理员账号是启用中的超级管理员。
func SuperAdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		account, ok := CurrentAccount(c)
		if !ok || account.Status != StatusActive || account.Role != RoleSuperAdmin {
			response.Fail(c, http.StatusForbidden, "superadmin required")
			c.Abort()
			return
		}
		c.Next()
	}
}

func mutationMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

func sameOrigin(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		return false
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if !strings.EqualFold(parsed.Host, request.Host) {
		return false
	}
	proto := strings.ToLower(strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")))
	if proto == "" {
		if request.TLS != nil {
			proto = "https"
		} else {
			proto = "http"
		}
	}
	return strings.EqualFold(parsed.Scheme, proto)
}

// CurrentAccount 返回当前请求的管理员账号（仅后台会话路由可用）。
func CurrentAccount(c *gin.Context) (Account, bool) {
	value, exists := c.Get(contextAccount)
	if !exists {
		return Account{}, false
	}
	account, ok := value.(Account)
	if !ok {
		return Account{}, false
	}
	return account, true
}

// SetCurrentAccount 写入管理员账号上下文。生产路径由 Required 中间件调用；
// 测试夹具也用它模拟已认证的管理员。
func SetCurrentAccount(c *gin.Context, account Account) {
	c.Set(contextAccount, account)
}

// CurrentAccountID 返回当前管理员账号 id；无管理员上下文时返回 0。
func CurrentAccountID(c *gin.Context) uint64 {
	account, ok := CurrentAccount(c)
	if !ok {
		return 0
	}
	return account.ID
}

// CurrentAccountRole 返回当前管理员角色；无管理员上下文时返回空。
func CurrentAccountRole(c *gin.Context) Role {
	account, ok := CurrentAccount(c)
	if !ok {
		return ""
	}
	return account.Role
}

func AuthenticatedFromContext(c *gin.Context) (AuthenticatedSession, error) {
	value, exists := c.Get(contextSession)
	if !exists {
		return AuthenticatedSession{}, ErrInvalidSession
	}
	authenticated, ok := value.(AuthenticatedSession)
	if !ok {
		return AuthenticatedSession{}, errors.New("invalid administrator session context")
	}
	return authenticated, nil
}
