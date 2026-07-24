package adminauth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
)

const (
	contextAuthKind = "adminauth.kind"
	contextSession  = "adminauth.session"
	authKindBearer  = "bearer"
	authKindCookie  = "cookie"
)

type BearerAuthenticator interface {
	Authenticate(context.Context, string) (user.User, error)
}

type Middleware struct {
	sessions *Service
	bearer   BearerAuthenticator
}

func NewMiddleware(sessions *Service, bearer BearerAuthenticator) *Middleware {
	return &Middleware{sessions: sessions, bearer: bearer}
}

func (m *Middleware) Required() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		cookie, cookieErr := c.Cookie(CookieName)
		hasCookie := cookieErr == nil && cookie != ""
		hasBearer := strings.HasPrefix(header, "Bearer ") && strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")) != ""
		if hasCookie && hasBearer {
			response.Fail(c, http.StatusBadRequest, "ambiguous administrator authentication")
			c.Abort()
			return
		}
		if hasBearer {
			value, err := m.bearer.Authenticate(c, strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
			if err != nil {
				response.Fail(c, http.StatusUnauthorized, "authentication required")
				c.Abort()
				return
			}
			user.SetCurrentUser(c, value)
			c.Set(contextAuthKind, authKindBearer)
			c.Next()
			return
		}
		if !hasCookie {
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
		user.SetCurrentUser(c, authenticated.User)
		c.Set(contextAuthKind, authKindCookie)
		c.Set(contextSession, authenticated)
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
