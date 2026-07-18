package main

import (
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/admin"
	"campus-circle/backend/internal/module/admin/review"
	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/module/navigation"
	"campus-circle/backend/internal/module/user"
)

func TestLegacyTakeoutDisabledSkipsRuntimeConstruction(t *testing.T) {
	runtime, err := newLegacyTakeoutRuntime(config.Config{EnableLegacyTakeout: false}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if runtime != nil {
		t.Fatal("disabled legacy takeout constructed a runtime")
	}
}

func TestLegacyTakeoutRouteGate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{name: "disabled", enabled: false, want: false},
		{name: "enabled", enabled: true, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := &gorm.DB{}
			reviews := review.NewService(review.NewRepository(db))
			mediaService := coremedia.NewService(coremedia.NewRepository(db))
			runtime, err := newLegacyTakeoutRuntime(config.Config{EnableLegacyTakeout: test.enabled}, db, reviews, mediaService)
			if err != nil {
				t.Fatal(err)
			}
			users := user.NewService(user.NewRepository(db), "test-secret-that-is-longer-than-thirty-two-characters", "", "", true)
			router := gin.New()
			api := router.Group("/api/v1")
			navigationService := navigation.NewService(navigation.NewRepository(db))
			navigation.NewHandler(navigationService).Register(api.Group("/navigation"))
			admin.NewHandler(nil, nil, users, nil, nil, navigationService).Register(api)
			registerLegacyTakeoutRoutes(runtime, api, users, reviews, mediaService)

			if !routeExists(router.Routes(), "GET", "/api/v1/navigation/categories") {
				t.Fatal("public navigation route must stay mounted")
			}
			externalRoutes := []struct{ method, path string }{
				{"GET", "/api/v1/admin/navigation/external-takeout-merchants"},
				{"POST", "/api/v1/admin/navigation/external-takeout-merchants"},
				{"PATCH", "/api/v1/admin/navigation/external-takeout-merchants/category-visibility"},
				{"PUT", "/api/v1/admin/navigation/external-takeout-merchants/:id"},
				{"DELETE", "/api/v1/admin/navigation/external-takeout-merchants/:id"},
				{"PATCH", "/api/v1/admin/navigation/external-takeout-merchants/:id/operating-status"},
				{"PATCH", "/api/v1/admin/navigation/external-takeout-merchants/:id/visibility"},
			}
			for _, route := range externalRoutes {
				if !routeExists(router.Routes(), route.method, route.path) {
					t.Fatalf("external directory route missing: %s %s", route.method, route.path)
				}
			}

			got := routeExists(router.Routes(), "GET", "/api/v1/takeout/buyer/merchants")
			if got != test.want {
				t.Fatalf("buyer route mounted=%v, want %v", got, test.want)
			}
			got = routeExists(router.Routes(), "POST", "/api/v1/takeout/payments/wechat/notify")
			if got != test.want {
				t.Fatalf("payment callback mounted=%v, want %v", got, test.want)
			}
			got = routeExists(router.Routes(), "GET", "/api/v1/admin/takeout/merchants")
			if got != test.want {
				t.Fatalf("admin route mounted=%v, want %v", got, test.want)
			}
		})
	}
}

func routeExists(routes gin.RoutesInfo, method, path string) bool {
	for _, route := range routes {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}
