package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSAllowsExactOptionsPathToReachHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const sensitivePath = "/api/v1/miniapp/media/wechat/review-content"
	router := gin.New()
	router.Use(CORS(sensitivePath))
	router.Any(sensitivePath, func(c *gin.Context) {
		c.Header("Allow", "HEAD, GET")
		c.Status(http.StatusMethodNotAllowed)
	})
	router.Any(sensitivePath+"/near", func(c *gin.Context) {
		t.Fatal("near path OPTIONS should be handled by CORS")
	})

	exact := httptest.NewRecorder()
	router.ServeHTTP(exact, httptest.NewRequest(http.MethodOptions, sensitivePath+"?token=opaque", nil))
	if exact.Code != http.StatusMethodNotAllowed || exact.Header().Get("Allow") != "HEAD, GET" {
		t.Fatalf("exact OPTIONS status=%d allow=%q", exact.Code, exact.Header().Get("Allow"))
	}

	near := httptest.NewRecorder()
	router.ServeHTTP(near, httptest.NewRequest(http.MethodOptions, sensitivePath+"/near", nil))
	if near.Code != http.StatusNoContent {
		t.Fatalf("near OPTIONS status=%d want=%d", near.Code, http.StatusNoContent)
	}
}

func TestCORSReflectsOnlyExplicitlyAllowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORSWithOrigins([]string{"https://admin.example.com"}))
	router.GET("/resource", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	allowedRequest := httptest.NewRequest(http.MethodGet, "/resource", nil)
	allowedRequest.Header.Set("Origin", "https://admin.example.com")
	allowed := httptest.NewRecorder()
	router.ServeHTTP(allowed, allowedRequest)
	if got := allowed.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example.com" {
		t.Fatalf("allowed origin=%q", got)
	}
	if got := allowed.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("credentials=%q", got)
	}
	if got := allowed.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("vary=%q", got)
	}

	blockedRequest := httptest.NewRequest(http.MethodGet, "/resource", nil)
	blockedRequest.Header.Set("Origin", "https://evil.example.com")
	blocked := httptest.NewRecorder()
	router.ServeHTTP(blocked, blockedRequest)
	if got := blocked.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("blocked origin was reflected: %q", got)
	}
	if got := blocked.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("blocked request received credentials header: %q", got)
	}
	if got := blocked.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("blocked request vary=%q", got)
	}
}

func TestCORSPreflightAllowsAdminCSRFHeaderOnlyForConfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORSWithOrigins([]string{"https://admin.example.com"}))
	router.POST("/resource", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	allowedRequest := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	allowedRequest.Header.Set("Origin", "https://admin.example.com")
	allowedRequest.Header.Set("Access-Control-Request-Method", http.MethodPost)
	allowedRequest.Header.Set("Access-Control-Request-Headers", "X-CSRF-Token")
	allowed := httptest.NewRecorder()
	router.ServeHTTP(allowed, allowedRequest)
	if allowed.Code != http.StatusNoContent {
		t.Fatalf("allowed preflight status=%d", allowed.Code)
	}
	if got := allowed.Header().Get("Access-Control-Allow-Headers"); !containsHeaderToken(got, "X-CSRF-Token") {
		t.Fatalf("allowed headers=%q", got)
	}

	blockedRequest := httptest.NewRequest(http.MethodOptions, "/resource", nil)
	blockedRequest.Header.Set("Origin", "https://evil.example.com")
	blocked := httptest.NewRecorder()
	router.ServeHTTP(blocked, blockedRequest)
	if blocked.Code != http.StatusNoContent || blocked.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("blocked preflight status=%d origin=%q", blocked.Code, blocked.Header().Get("Access-Control-Allow-Origin"))
	}
}

func containsHeaderToken(value string, token string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}

func TestCORSWithoutOriginDoesNotEmitBrowserCredentialsHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORSWithOrigins(nil))
	router.GET("/resource", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/resource", nil))
	if response.Header().Get("Access-Control-Allow-Origin") != "" || response.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatal("same-origin request unexpectedly received cross-origin credentials headers")
	}
	if got := response.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("same-origin request vary=%q", got)
	}
}
