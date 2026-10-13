package featureflags

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"campus-circle/backend/internal/module/user"
	"github.com/gin-gonic/gin"
)

func TestCampusInfoScopeCannotReopenStoredBusiness(t *testing.T) {
	for _, policyEnabled := range []bool{false, true} {
		Configure(policyEnabled)
		RestrictToCampusInfo()
		for _, key := range AllKeys {
			if Can(key) {
				t.Fatalf("scope reopened %s with policy enabled=%v", key, policyEnabled)
			}
		}
		if CanPost(user.RoleSuperAdmin) || CanComment(user.RoleSuperAdmin) {
			t.Fatal("admin must have the same release scope")
		}
		value, err := ReadPolicy(true)
		if err != nil || value.ReleaseScope != CampusInfoScope || !value.Enabled {
			t.Fatalf("invalid scope: %+v %v", value, err)
		}
		if !state.policy.Flags[KeySecondhand] {
			t.Fatal("scope mutated stored policy")
		}
	}
	Configure(true)
	t.Cleanup(func() { Configure(false) })
	RestrictToCampusInfo()
	for _, key := range AllKeys {
		value := Snapshot()
		value.Flags[key] = true
		_, err := Update(1, UpdateInput{Flags: value.Flags, PostingMode: "admin_only", ExpectedRowVersion: value.RowVersion})
		if !errors.Is(err, ErrReleaseRestricted) {
			t.Fatalf("write reopened %s: %v", key, err)
		}
	}
	value := Snapshot()
	_, err := Update(1, UpdateInput{Flags: value.Flags, PostingMode: "all_users", ExpectedRowVersion: value.RowVersion})
	if !errors.Is(err, ErrReleaseRestricted) {
		t.Fatalf("posting mode bypass: %v", err)
	}
}

func TestCampusInfoHTTPBlocksDiscoveryPreservesAccountAndExistingOrders(t *testing.T) {
	Configure(true)
	t.Cleanup(func() { Configure(false) })
	RestrictToCampusInfo()
	router := gin.New()
	router.Use(GuardRequests())
	router.NoRoute(func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, path := range []string{"/api/v1/community/posts", "/api/v1/trade/products", "/api/v1/navigation/categories", "/api/v1/im/conversations", "/api/v1/delivery/rider/orders/claimable"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("Authorization", "Bearer test-guard-only")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s returned %d", path, w.Code)
		}
	}
	for _, path := range []string{"/api/v1/auth/me", "/api/v1/campus-info/guides", "/api/v1/delivery/merchant/orders", "/api/v1/delivery/merchant/wallet"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("Authorization", "Bearer test-guard-only")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != http.StatusNoContent {
			t.Fatalf("existing work blocked: %s %d", path, w.Code)
		}
	}
}
