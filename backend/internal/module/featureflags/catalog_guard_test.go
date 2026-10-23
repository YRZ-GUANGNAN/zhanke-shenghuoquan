package featureflags

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMerchantCatalogRespectsTakeoutVisibilityWithoutBlockingExistingFulfillment(t *testing.T) {
	Configure(true)
	t.Cleanup(func() { Configure(false) })
	value := Snapshot()
	value.Flags[KeyTakeout] = false
	if _, err := Update(1, UpdateInput{Flags: value.Flags, PostingMode: value.PostingMode, ExpectedRowVersion: value.RowVersion}); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(GuardRequests())
	for _, path := range []string{"/api/v1/takeout/catalog/merchants", "/api/v1/takeout/catalog/merchants/:id", "/api/v1/takeout/catalog/merchants/:id/dishes", "/api/v1/takeout/buyer/orders/:id"} {
		router.GET(path, func(c *gin.Context) { c.Status(http.StatusOK) })
	}
	for _, path := range []string{"/api/v1/takeout/catalog/merchants", "/api/v1/takeout/catalog/merchants/SHOP", "/api/v1/takeout/catalog/merchants/SHOP/dishes", "/api/v1/takeout/buyer/orders/ORDER"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "unit-test")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, request)
		want := http.StatusForbidden
		if path == "/api/v1/takeout/buyer/orders/ORDER" {
			want = http.StatusOK
		}
		if w.Code != want {
			t.Fatalf("visibility for %s = %d, want %d", path, w.Code, want)
		}
	}
}
