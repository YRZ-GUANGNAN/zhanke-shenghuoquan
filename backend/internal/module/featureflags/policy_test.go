package featureflags

import (
	"bytes"
	"campus-circle/backend/internal/module/user"
	"context"
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
	"time"
)

type memoryStore struct {
	value   Policy
	found   bool
	loadErr error
	audits  int
}

func (s *memoryStore) Ensure(context.Context) error { return nil }
func (s *memoryStore) Load(context.Context) (Policy, bool, error) {
	return clonePolicy(s.value), s.found, s.loadErr
}
func (s *memoryStore) CreateIfMissing(_ context.Context, p Policy) error {
	s.value, s.found = clonePolicy(p), true
	return nil
}
func (s *memoryStore) Save(_ context.Context, before, after Policy) error {
	if s.value.RowVersion != before.RowVersion {
		return ErrVersionConflict
	}
	s.value = clonePolicy(after)
	s.audits++
	return nil
}

func TestPolicyDurabilityRefreshAndFailClosed(t *testing.T) {
	t.Cleanup(func() { Configure(false) })
	store := &memoryStore{}
	if err := ConfigureStore(context.Background(), true, store); err != nil {
		t.Fatal(err)
	}
	p := Snapshot()
	p.Flags[KeyTakeout] = false
	saved, err := Update(7, UpdateInput{p.Flags, "admin_only", p.RowVersion})
	if err != nil || saved.UpdatedBy != 7 || store.audits != 1 {
		t.Fatalf("save: %+v %v", saved, err)
	}
	if err := ConfigureStore(context.Background(), true, store); err != nil {
		t.Fatal(err)
	}
	if Can(KeyTakeout) {
		t.Fatal("restart reopened disabled module")
	}
	store.value.Flags[KeyForum] = false
	store.value.RowVersion++
	refresh(true)
	if Can(KeyForum) || CanPost(user.RoleSuperAdmin) {
		t.Fatal("other process update ignored")
	}
	store.loadErr = errors.New("database offline")
	refresh(true)
	for _, key := range AllKeys {
		if Can(key) {
			t.Fatalf("database error opened %s", key)
		}
	}
	if _, err := Update(7, UpdateInput{saved.Flags, "admin_only", saved.RowVersion}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unavailable write: %v", err)
	}
}

func TestPartialFlagsAndUnknownKeysCannotReopenModules(t *testing.T) {
	Configure(true)
	t.Cleanup(func() { Configure(false) })
	p := Snapshot()
	p.Flags[KeyMessages] = false
	p, err := Update(1, UpdateInput{p.Flags, "admin_only", p.RowVersion})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Update(1, UpdateInput{map[string]bool{KeyForum: true}, "all_users", p.RowVersion}); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("partial update accepted: %v", err)
	}
	flags := clonePolicy(p).Flags
	delete(flags, KeySearch)
	flags["unknown"] = true
	if _, err := Update(1, UpdateInput{flags, "all_users", p.RowVersion}); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("unknown key accepted: %v", err)
	}
	if Can(KeyMessages) || Snapshot().RowVersion != p.RowVersion {
		t.Fatal("invalid request changed persisted policy")
	}
}

func TestDependenciesAndRoleAreIndependent(t *testing.T) {
	Configure(true)
	t.Cleanup(func() { Configure(false) })
	p := Snapshot()
	p.Flags[KeyForum] = false
	if _, err := Update(1, UpdateInput{p.Flags, "all_users", p.RowVersion}); err != nil {
		t.Fatal(err)
	}
	if Can(KeyPost) || Can(KeyComment) || Can(KeyFollowing) || !Can(KeySecondhandComments) {
		t.Fatal("parent policy leaked across independent modules")
	}
	if !Snapshot().Flags[KeyPost] {
		t.Fatal("saved child setting was overwritten")
	}
	p = Snapshot()
	p.Flags[KeyForum] = true
	if _, err := Update(1, UpdateInput{p.Flags, "admin_only", p.RowVersion}); err != nil {
		t.Fatal(err)
	}
	if CanPost(user.RoleUser) || !CanPost(user.RoleAdmin) || !CanPost(user.RoleSuperAdmin) {
		t.Fatal("posting mode incorrectly grants roles")
	}
	p = Snapshot()
	p.Flags[KeyPublish] = false
	if _, err := Update(1, UpdateInput{p.Flags, "all_users", p.RowVersion}); err != nil {
		t.Fatal(err)
	}
	if CanPost(user.RoleSuperAdmin) || Can(KeySecondhandPublish) || !Can(KeyComment) {
		t.Fatal("unified publish dependencies incorrect")
	}
}

func TestRequestGuardPreservesExistingOrderWork(t *testing.T) {
	Configure(true)
	t.Cleanup(func() { Configure(false) })
	p := Snapshot()
	for key := range p.Flags {
		p.Flags[key] = false
	}
	Set(p)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(GuardRequests())
	blocked := []struct{ method, path string }{
		{"GET", "/api/v1/community/posts/7"}, {"GET", "/api/v1/trade/products/7"},
		{"PUT", "/api/v1/auth/identity-applications/rider"}, {"POST", "/api/v1/delivery/merchant/orders"},
		{"POST", "/api/v1/delivery/rider/orders/7/claim"}, {"GET", "/api/v1/delivery/rider/orders/claimable"},
		{"POST", "/api/v1/takeout/buyer/orders"}, {"PUT", "/api/v1/community/posts/7/like"},
	}
	allowed := []struct{ method, path string }{
		{"GET", "/api/v1/delivery/merchant/orders"}, {"PATCH", "/api/v1/delivery/rider/orders/7/delivered"},
		{"POST", "/api/v1/delivery/merchant/orders/7/refunds"}, {"GET", "/api/v1/delivery/merchant/wallet"},
		{"GET", "/api/v1/delivery/orders/7/conversation"}, {"GET", "/api/v1/admin/feature-policy"}, {"GET", "/api/v1/auth/me"},
	}
	for _, route := range append(blocked, allowed...) {
		router.Handle(route.method, route.path, func(c *gin.Context) { c.Status(204) })
	}
	for _, group := range []struct {
		routes []struct{ method, path string }
		status int
	}{{blocked, 403}, {allowed, 204}} {
		for _, route := range group.routes {
			req := httptest.NewRequest(route.method, route.path, nil)
			req.Header.Set("Authorization", "Bearer test")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != group.status {
				t.Fatalf("%s %s: %d", route.method, route.path, rec.Code)
			}
		}
	}
}

func TestAdminAuthorizationAndPublicPayload(t *testing.T) {
	Configure(true)
	t.Cleanup(func() { Configure(false) })
	gin.SetMode(gin.TestMode)
	for _, role := range []user.Role{user.RoleUser, user.RoleAdmin, user.RoleSuperAdmin} {
		router := gin.New()
		router.Use(func(c *gin.Context) { user.SetCurrentUser(c, user.User{ID: 3, Role: role, Status: user.StatusActive}) })
		router.PATCH("/policy", AdminUpdateHandler)
		p := Snapshot()
		data, _ := json.Marshal(UpdateInput{p.Flags, "all_users", p.RowVersion})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest("PATCH", "/policy", bytes.NewReader(data)))
		expected := 403
		if role == user.RoleSuperAdmin {
			expected = 200
		}
		if rec.Code != expected {
			t.Fatalf("role %s: %d", role, rec.Code)
		}
	}
	router := gin.New()
	router.GET("/features", PublicHandler)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/features", nil))
	if bytes.Contains(rec.Body.Bytes(), []byte("updatedBy")) || bytes.Contains(rec.Body.Bytes(), []byte("labEnabled")) || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("public response leaked administrator details or cached policy")
	}
}

func TestRefreshCacheBound(t *testing.T) {
	t.Cleanup(func() { Configure(false) })
	store := &memoryStore{}
	if err := ConfigureStore(context.Background(), true, store); err != nil {
		t.Fatal(err)
	}
	store.value.Flags[KeyForum] = false
	state.Lock()
	state.refreshedAt = time.Now().Add(-3 * time.Second)
	state.Unlock()
	if Can(KeyForum) {
		t.Fatal("cache did not refresh within two seconds")
	}
}
