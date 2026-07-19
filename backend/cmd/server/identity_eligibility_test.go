package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"campus-circle/backend/internal/module/identityapplication"
	"campus-circle/backend/internal/module/takeout"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type stubRiderIdentityFinder struct {
	value takeout.Rider
	err   error
}

type stubGrantFinder struct {
	grants map[string]identityapplication.IdentityGrant
	err    error
}

func (s stubGrantFinder) FindGrant(_ context.Context, userID uint64, role identityapplication.Role) (identityapplication.IdentityGrant, bool, error) {
	if s.err != nil {
		return identityapplication.IdentityGrant{}, false, s.err
	}
	value, ok := s.grants[grantKey(userID, role)]
	return value, ok, nil
}

func grantKey(userID uint64, role identityapplication.Role) string {
	return fmt.Sprintf("%d:%s", userID, role)
}

func activeGrant(userID uint64, role identityapplication.Role) stubGrantFinder {
	return stubGrantFinder{grants: map[string]identityapplication.IdentityGrant{
		grantKey(userID, role): {UserID: userID, Role: role, Status: identityapplication.GrantActive},
	}}
}

func (s stubRiderIdentityFinder) Find(context.Context, uint64) (takeout.Rider, error) {
	return s.value, s.err
}

func TestDeliveryIdentityEligibilityFailsClosedWhenDisabled(t *testing.T) {
	eligibility := newDeliveryIdentityEligibility(false, nil, nil)
	for name, check := range map[string]func(context.Context, uint64) (bool, string, error){
		"rider": func(ctx context.Context, id uint64) (bool, string, error) {
			value, err := eligibility.RiderEligible(ctx, id)
			return value.Available, value.Status, err
		},
		"merchant": func(ctx context.Context, id uint64) (bool, string, error) {
			value, err := eligibility.MerchantEligible(ctx, id)
			return value.Available, value.Status, err
		},
	} {
		t.Run(name, func(t *testing.T) {
			available, status, err := check(context.Background(), 42)
			if err != nil || available || status != "disabled" {
				t.Fatalf("available=%v status=%q err=%v", available, status, err)
			}
		})
	}
}

func TestRiderRequiresActiveGrantAndActiveTakeoutRider(t *testing.T) {
	for name, finder := range map[string]stubRiderIdentityFinder{
		"active":  {value: takeout.Rider{Status: takeout.RiderActive}},
		"resting": {value: takeout.Rider{Status: takeout.RiderResting}},
		"missing": {err: takeout.ErrNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			result, findErr := newDeliveryIdentityEligibility(true, finder, activeGrant(42, identityapplication.RoleRider)).RiderEligible(context.Background(), 42)
			if findErr != nil {
				t.Fatal(findErr)
			}
			if name == "active" && !result.Available {
				t.Fatalf("active rider unavailable: %+v", result)
			}
			if name != "active" && result.Available {
				t.Fatalf("%s rider available: %+v", name, result)
			}
		})
	}

	missing, err := newDeliveryIdentityEligibility(true, stubRiderIdentityFinder{value: takeout.Rider{Status: takeout.RiderActive}}, stubGrantFinder{}).
		RiderEligible(context.Background(), 42)
	if err != nil || missing.Available || missing.Status != "missing" {
		t.Fatalf("missing grant=%+v err=%v", missing, err)
	}
	suspendedFinder := stubGrantFinder{grants: map[string]identityapplication.IdentityGrant{
		grantKey(42, identityapplication.RoleRider): {UserID: 42, Role: identityapplication.RoleRider, Status: identityapplication.GrantSuspended},
	}}
	suspended, err := newDeliveryIdentityEligibility(true, stubRiderIdentityFinder{value: takeout.Rider{Status: takeout.RiderActive}}, suspendedFinder).
		RiderEligible(context.Background(), 42)
	if err != nil || suspended.Available || suspended.Status != "suspended" {
		t.Fatalf("suspended grant=%+v err=%v", suspended, err)
	}
}

func TestMerchantRequiresActiveGrant(t *testing.T) {
	missing, err := newDeliveryIdentityEligibility(true, nil, stubGrantFinder{}).MerchantEligible(context.Background(), 88)
	if err != nil || missing.Available || missing.Status != "missing" {
		t.Fatalf("missing=%+v err=%v", missing, err)
	}
	active, err := newDeliveryIdentityEligibility(true, nil, activeGrant(88, identityapplication.RoleMerchant)).MerchantEligible(context.Background(), 88)
	if err != nil || !active.Available || active.Status != "active" {
		t.Fatalf("active=%+v err=%v", active, err)
	}
	suspendedFinder := stubGrantFinder{grants: map[string]identityapplication.IdentityGrant{
		grantKey(88, identityapplication.RoleMerchant): {UserID: 88, Role: identityapplication.RoleMerchant, Status: identityapplication.GrantSuspended},
	}}
	suspended, err := newDeliveryIdentityEligibility(true, nil, suspendedFinder).MerchantEligible(context.Background(), 88)
	if err != nil || suspended.Available || suspended.Status != "suspended" {
		t.Fatalf("suspended=%+v err=%v", suspended, err)
	}
	zero, err := newDeliveryIdentityEligibility(true, nil, activeGrant(88, identityapplication.RoleMerchant)).MerchantEligible(context.Background(), 0)
	if err != nil || zero.Available || zero.Status != "missing" {
		t.Fatalf("zero user=%+v err=%v", zero, err)
	}
}

func TestDeliveryIdentityEligibilityPropagatesRepositoryFailure(t *testing.T) {
	want := errors.New("database unavailable")
	_, err := newDeliveryIdentityEligibility(true, stubRiderIdentityFinder{err: want}, activeGrant(42, identityapplication.RoleRider)).RiderEligible(context.Background(), 42)
	if !errors.Is(err, want) {
		t.Fatalf("err=%v want=%v", err, want)
	}
	_, err = newDeliveryIdentityEligibility(true, nil, stubGrantFinder{err: want}).MerchantEligible(context.Background(), 42)
	if !errors.Is(err, want) {
		t.Fatalf("grant err=%v want=%v", err, want)
	}
}

func TestIdentityApplicationRoutesHaveSeparateUserAndAdminPolicies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	authRoutes := api.Group("/auth")
	// This existing identity switch endpoint is deliberately registered once;
	// the application route helper must not create a second /auth/identity tree.
	authRoutes.GET("/identity", func(c *gin.Context) { response.OK(c, gin.H{"ok": true}) })
	authRequired := func(c *gin.Context) {
		role := user.Role(c.GetHeader("X-Test-Role"))
		if role == "" {
			response.Fail(c, http.StatusUnauthorized, "authentication required")
			c.Abort()
			return
		}
		c.Set("domainA.user", user.User{ID: 7, Role: role, Status: user.StatusActive})
		c.Next()
	}
	registerIdentityApplicationRoutes(
		api,
		authRoutes,
		identityapplication.NewHandler(identityapplication.NewService(nil, nil, nil)),
		identityRoutePolicies{
			authRequired: authRequired, unmutedRequired: user.UnmutedRequired(), adminRequired: user.AdminRequired(),
		},
	)

	want := map[string]bool{
		http.MethodGet + " /api/v1/auth/identity":                                       false,
		http.MethodGet + " /api/v1/auth/identity-applications":                          false,
		http.MethodGet + " /api/v1/auth/identity-applications/:role":                    false,
		http.MethodPut + " /api/v1/auth/identity-applications/:role":                    false,
		http.MethodGet + " /api/v1/admin/identity-applications":                         false,
		http.MethodGet + " /api/v1/admin/identity-applications/:applicationId":          false,
		http.MethodPost + " /api/v1/admin/identity-applications/:applicationId/approve": false,
		http.MethodPost + " /api/v1/admin/identity-applications/:applicationId/reject":  false,
	}
	counts := make(map[string]int)
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		counts[key]++
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for route, found := range want {
		if !found || counts[route] != 1 {
			t.Fatalf("route %s found=%v count=%d", route, found, counts[route])
		}
	}

	endpoint := "/api/v1/admin/identity-applications"
	for name, test := range map[string]struct {
		role   user.Role
		status int
	}{
		"unauthenticated": {status: http.StatusUnauthorized},
		"ordinary user":   {role: user.RoleUser, status: http.StatusForbidden},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, endpoint, nil)
			request.Header.Set("X-Test-Role", string(test.role))
			result := httptest.NewRecorder()
			router.ServeHTTP(result, request)
			if result.Code != test.status {
				t.Fatalf("status=%d want=%d body=%s", result.Code, test.status, result.Body.String())
			}
		})
	}
}
