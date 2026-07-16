package middleware

import (
	"net/http"
	"net/http/httptest"
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
