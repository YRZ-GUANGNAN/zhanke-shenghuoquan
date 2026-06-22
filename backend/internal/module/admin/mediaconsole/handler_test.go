package mediaconsole

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func TestMediaConsolePermissionModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service, _ := NewQueryService(&queryRepositoryStub{})
	router := gin.New()
	auth := func(c *gin.Context) {
		role := user.Role(c.GetHeader("X-Test-Role"))
		if role == "" {
			response.Fail(c, http.StatusUnauthorized, "authentication required")
			c.Abort()
			return
		}
		c.Set("domainA.user", user.User{ID: 1, Role: role, Status: user.StatusActive})
		c.Next()
	}
	group := router.Group("/api/v1/admin/media", auth, user.AdminRequired())
	NewHandler(service).Register(group)

	for _, test := range []struct {
		role string
		want int
	}{{"", http.StatusUnauthorized}, {string(user.RoleUser), http.StatusForbidden}, {string(user.RoleAdmin), http.StatusOK}, {string(user.RoleSuperAdmin), http.StatusOK}} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/media/summary", nil)
		request.Header.Set("X-Test-Role", test.role)
		responseRecorder := httptest.NewRecorder()
		router.ServeHTTP(responseRecorder, request)
		if responseRecorder.Code != test.want {
			t.Fatalf("role=%q status=%d body=%s", test.role, responseRecorder.Code, responseRecorder.Body.String())
		}
	}
	for _, route := range router.Routes() {
		if strings.HasPrefix(route.Path, "/api/v1/admin/media") && route.Method != http.MethodGet {
			t.Fatalf("write route exposed: %s %s", route.Method, route.Path)
		}
	}
}

func TestMediaConsoleHandlerRejectsInvalidQueryAndHidesInternalErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := &queryRepositoryStub{}
	service, _ := NewQueryService(repository)
	router := gin.New()
	NewHandler(service).Register(router.Group("/api/v1/admin/media"))

	invalid := httptest.NewRecorder()
	router.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/api/v1/admin/media/assets?page=0", nil))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status=%d body=%s", invalid.Code, invalid.Body.String())
	}
	repository.err = errors.New("dsn password=secret internal sql")
	failed := httptest.NewRecorder()
	router.ServeHTTP(failed, httptest.NewRequest(http.MethodGet, "/api/v1/admin/media/summary", nil))
	if failed.Code != http.StatusInternalServerError || strings.Contains(failed.Body.String(), "password") || strings.Contains(failed.Body.String(), "sql") {
		t.Fatalf("internal error leaked: %s", failed.Body.String())
	}
}
