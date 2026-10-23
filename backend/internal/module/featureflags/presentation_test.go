package featureflags

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"campus-circle/backend/internal/module/user"
	"github.com/gin-gonic/gin"
)

func modePointer(value string) *string { return &value }

func TestPresentationRoundTripPreservesOriginalSettings(t *testing.T) {
	t.Cleanup(func() { Configure(false) })
	store := &memoryStore{}
	if err := ConfigureStore(context.Background(), true, store); err != nil {
		t.Fatal(err)
	}
	p := Snapshot()
	p.Flags[KeyTakeout] = false
	p.Flags[KeyPost] = false
	original, err := Update(7, UpdateInput{p.Flags, "all_users", p.RowVersion, nil})
	if err != nil {
		t.Fatal(err)
	}
	info, err := Update(7, UpdateInput{original.Flags, original.PostingMode, original.RowVersion, modePointer(PresentationCampusInfo)})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(info.Flags, original.Flags) || info.PostingMode != original.PostingMode {
		t.Fatal("mode selection destroyed original settings")
	}
	for _, key := range AllKeys {
		if Can(key) {
			t.Fatalf("info opened %s", key)
		}
	}
	for _, role := range []user.Role{user.RoleUser, user.RoleAdmin, user.RoleSuperAdmin} {
		if CanPost(role) || CanComment(role) {
			t.Fatal("different visitor scope by role")
		}
	}
	// A stale admin app omitting the new field must never switch the mode.
	info, err = Update(7, UpdateInput{info.Flags, info.PostingMode, info.RowVersion, nil})
	if err != nil || info.PresentationMode != PresentationCampusInfo {
		t.Fatalf("legacy update changed mode: %v", err)
	}
	if err := ConfigureStore(context.Background(), true, store); err != nil {
		t.Fatal(err)
	}
	if Snapshot().PresentationMode != PresentationCampusInfo || Can(KeySecondhand) {
		t.Fatal("restart lost the selected mode")
	}
	restored, err := Update(7, UpdateInput{info.Flags, info.PostingMode, info.RowVersion, modePointer(PresentationStandard)})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(restored.Flags, original.Flags) || restored.PostingMode != original.PostingMode {
		t.Fatal("original values did not survive round trip")
	}
	for _, key := range AllKeys {
		if Can(key) != Effective(original.Flags, key) {
			t.Fatalf("wrong restored capability: %s", key)
		}
	}
	if CanPost(user.RoleUser) || !CanComment(user.RoleUser) {
		t.Fatal("restored original posting policy incorrectly")
	}
	if store.audits != 4 {
		t.Fatalf("unexpected audits %d", store.audits)
	}
}

func TestInvalidPresentationAndConflictDoNotWrite(t *testing.T) {
	Configure(true)
	t.Cleanup(func() { Configure(false) })
	p := Snapshot()
	for _, mode := range []string{"", "review_only", "STANDARD", " standard "} {
		if _, err := Update(3, UpdateInput{p.Flags, p.PostingMode, p.RowVersion, &mode}); !errors.Is(err, ErrInvalidPresentationMode) {
			t.Fatalf("invalid mode %q accepted: %v", mode, err)
		}
	}
	if _, err := Update(3, UpdateInput{p.Flags, p.PostingMode, 0, modePointer(PresentationCampusInfo)}); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale mode change accepted: %v", err)
	}
	if !reflect.DeepEqual(Snapshot(), p) {
		t.Fatal("rejected request changed state")
	}
}

func TestPresentationPublicPolicyDiffersFromSavedAdminPolicy(t *testing.T) {
	Configure(true)
	t.Cleanup(func() { Configure(false) })
	p := Snapshot()
	if _, err := Update(3, UpdateInput{p.Flags, "all_users", p.RowVersion, modePointer(PresentationCampusInfo)}); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/features", PublicHandler)
	router.GET("/saved", AdminGetHandler)
	for _, path := range []string{"/features", "/saved"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		var body struct {
			Data Policy `json:"data"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &body) != nil {
			t.Fatalf("invalid policy response %s", path)
		}
		if body.Data.PresentationMode != PresentationCampusInfo {
			t.Fatal("mode missing from response")
		}
		if body.Data.Flags[KeyForum] != (path == "/saved") {
			t.Fatal("saved and effective policy were confused")
		}
		if path == "/features" && (bytes.Contains(w.Body.Bytes(), []byte("updatedBy")) || body.Data.PostingMode != "admin_only") {
			t.Fatal("public policy leaks raw policy")
		}
	}
	if !Snapshot().Flags[KeyForum] {
		t.Fatal("reading effective policy mutated stored flags")
	}
}

func TestPresentationModeRequiresActiveSuperAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		role   user.Role
		status user.Status
		want   int
	}{
		{user.RoleUser, user.StatusActive, 403}, {user.RoleAdmin, user.StatusActive, 403},
		{user.RoleSuperAdmin, user.StatusBanned, 403}, {user.RoleSuperAdmin, user.StatusActive, 200},
	} {
		Configure(true)
		p := Snapshot()
		body, _ := json.Marshal(UpdateInput{p.Flags, p.PostingMode, p.RowVersion, modePointer(PresentationCampusInfo)})
		router := gin.New()
		router.Use(func(c *gin.Context) { user.SetCurrentUser(c, user.User{ID: 9, Role: test.role, Status: test.status}) })
		router.PATCH("/policy", AdminUpdateHandler)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("PATCH", "/policy", bytes.NewReader(body)))
		if w.Code != test.want {
			t.Fatalf("%s %s returned %d", test.role, test.status, w.Code)
		}
		if test.want != 200 && Snapshot().PresentationMode != PresentationStandard {
			t.Fatal("unauthorized mode change persisted")
		}
	}
	Configure(false)
}

func TestCampusInfoHTTPPreservesAccountAndExistingOrders(t *testing.T) {
	Configure(true)
	t.Cleanup(func() { Configure(false) })
	p := Snapshot()
	if _, err := Update(7, UpdateInput{p.Flags, p.PostingMode, p.RowVersion, modePointer(PresentationCampusInfo)}); err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.Use(GuardRequests())
	router.NoRoute(func(c *gin.Context) { c.Status(http.StatusNoContent) })
	for _, group := range []struct {
		paths []string
		want  int
	}{
		{[]string{"/api/v1/community/posts", "/api/v1/trade/products", "/api/v1/navigation/categories", "/api/v1/im/conversations", "/api/v1/delivery/rider/orders/claimable"}, 403},
		{[]string{"/api/v1/auth/me", "/api/v1/campus-info/guides", "/api/v1/delivery/merchant/orders", "/api/v1/delivery/merchant/wallet"}, 204},
	} {
		for _, path := range group.paths {
			r := httptest.NewRequest(http.MethodGet, path, nil)
			r.Header.Set("Authorization", "Bearer test-guard-only")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)
			if w.Code != group.want {
				t.Fatalf("%s returned %d", path, w.Code)
			}
		}
	}
}
