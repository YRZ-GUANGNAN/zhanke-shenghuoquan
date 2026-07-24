package adminauth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSameOriginRequiresMatchingSchemeAndHost(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "https://zhanduiwang.top/api/v1/admin/example", nil)
	request.Host = "zhanduiwang.top"
	request.Header.Set("Origin", "https://zhanduiwang.top")
	request.Header.Set("X-Forwarded-Proto", "https")
	if !sameOrigin(request) {
		t.Fatal("matching production origin must pass")
	}
	request.Header.Set("Origin", "https://evil.example")
	if sameOrigin(request) {
		t.Fatal("cross-origin request must fail")
	}
	request.Header.Del("Origin")
	if sameOrigin(request) {
		t.Fatal("missing origin must fail for cookie mutation")
	}
}

func TestSessionCookieSecurityAttributes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "https://zhanduiwang.top/api/v1/admin-auth/login", nil)
	setSessionCookie(context, "secret", 3600)
	header := recorder.Header().Get("Set-Cookie")
	for _, token := range []string{
		CookieName + "=secret",
		"Path=/",
		"Max-Age=3600",
		"HttpOnly",
		"Secure",
		"SameSite=Strict",
	} {
		if !strings.Contains(header, token) {
			t.Fatalf("cookie missing %q: %s", token, header)
		}
	}
	if strings.Contains(strings.ToLower(header), "domain=") {
		t.Fatalf("__Host cookie must not set Domain: %s", header)
	}
}
