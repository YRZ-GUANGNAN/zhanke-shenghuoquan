package takeoutbuyer

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCatalogMountsOnlyAuthenticatedReadRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(nil, nil, nil).RegisterCatalog(router.Group("/takeout/catalog", func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) }))
	routes := router.Routes()
	if len(routes) != 3 {
		t.Fatalf("catalog must have exactly three read endpoints, got %d", len(routes))
	}
	for _, route := range routes {
		if route.Method != http.MethodGet {
			t.Fatalf("catalog exposed a write: %s", route.Method)
		}
	}
	for _, path := range []string{"/takeout/catalog/merchants", "/takeout/catalog/merchants/SHOP", "/takeout/catalog/merchants/SHOP/dishes"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("catalog bypassed authentication: %s %d", path, w.Code)
		}
	}
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(method, "/takeout/catalog/orders", nil))
		if w.Code != http.StatusNotFound {
			t.Fatal("catalog activated legacy order workflow")
		}
	}
}
