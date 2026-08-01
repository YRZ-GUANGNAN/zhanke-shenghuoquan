package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/pkg/response"
)

func TestReadinessHandlerReportsDependencyStateWithoutLeakingError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, test := range []struct {
		name       string
		check      readinessCheck
		wantStatus int
		wantBody   string
	}{
		{name: "ready", check: func(context.Context) error { return nil }, wantStatus: http.StatusOK, wantBody: "ready"},
		{name: "dependency failed", check: func(context.Context) error { return errors.New("mysql password secret") }, wantStatus: http.StatusServiceUnavailable, wantBody: "service is not ready"},
		{name: "missing check", check: nil, wantStatus: http.StatusServiceUnavailable, wantBody: "service is not ready"},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/readyz", readinessHandler(test.check))
			result := httptest.NewRecorder()
			router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if result.Code != test.wantStatus {
				t.Fatalf("status=%d want=%d", result.Code, test.wantStatus)
			}
			if body := result.Body.String(); !contains(body, test.wantBody) || contains(body, "mysql password secret") {
				t.Fatalf("unsafe or unexpected response body: %s", body)
			}
		})
	}
}

func TestReadinessHandlerRequiresEveryDependency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	checksRun := 0
	router := gin.New()
	router.GET("/readyz", readinessHandler(
		func(context.Context) error { checksRun++; return nil },
		func(context.Context) error { checksRun++; return errors.New("meilisearch api key secret") },
		func(context.Context) error { checksRun++; return nil },
	))

	result := httptest.NewRecorder()
	router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if result.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d want=%d", result.Code, http.StatusServiceUnavailable)
	}
	if checksRun != 2 {
		t.Fatalf("checksRun=%d want=2", checksRun)
	}
	if body := result.Body.String(); contains(body, "meilisearch api key secret") {
		t.Fatalf("readiness leaked dependency error: %s", body)
	}
}

func TestOperationalRoutesExposeLivenessAndNonSecretBuildMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalVersion, originalCommit, originalTime := buildVersion, buildCommit, buildTime
	t.Cleanup(func() { buildVersion, buildCommit, buildTime = originalVersion, originalCommit, originalTime })
	buildVersion, buildCommit, buildTime = "1.2.3", "abc123", "2026-08-01T00:00:00Z"

	router := gin.New()
	registerOperationalRoutes(router, nil)

	health := httptest.NewRecorder()
	router.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status=%d", health.Code)
	}

	versionResult := httptest.NewRecorder()
	router.ServeHTTP(versionResult, httptest.NewRequest(http.MethodGet, "/version", nil))
	if versionResult.Code != http.StatusOK {
		t.Fatalf("version status=%d", versionResult.Code)
	}
	var body response.Body
	if err := json.Unmarshal(versionResult.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(body.Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"1.2.3", "abc123", "2026-08-01T00:00:00Z"} {
		if !contains(string(encoded), expected) {
			t.Fatalf("version response missing %q: %s", expected, encoded)
		}
	}
}

func TestProductionBuildMetadataFailsClosed(t *testing.T) {
	originalVersion, originalCommit, originalTime := buildVersion, buildCommit, buildTime
	t.Cleanup(func() { buildVersion, buildCommit, buildTime = originalVersion, originalCommit, originalTime })

	for _, test := range []struct {
		name, version, commit, builtAt string
		wantErr                        bool
	}{
		{name: "release", version: "2026.08.01", commit: "abcdef123456", builtAt: "2026-08-01T00:00:00Z"},
		{name: "development version", version: "dev", commit: "abcdef123456", builtAt: "2026-08-01T00:00:00Z", wantErr: true},
		{name: "unknown commit", version: "2026.08.01", commit: "unknown", builtAt: "2026-08-01T00:00:00Z", wantErr: true},
		{name: "dirty commit", version: "2026.08.01", commit: "abcdef123456-dirty", builtAt: "2026-08-01T00:00:00Z", wantErr: true},
		{name: "invalid time", version: "2026.08.01", commit: "abcdef123456", builtAt: "unknown", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			buildVersion, buildCommit, buildTime = test.version, test.commit, test.builtAt
			err := validateProductionBuildMetadata("prod")
			if (err != nil) != test.wantErr {
				t.Fatalf("validateProductionBuildMetadata() error=%v wantErr=%v", err, test.wantErr)
			}
		})
	}
	buildVersion, buildCommit, buildTime = "dev", "unknown", "unknown"
	if err := validateProductionBuildMetadata("dev"); err != nil {
		t.Fatalf("development metadata must remain optional: %v", err)
	}
}

func TestCompositionRootWiresDatabaseAndSearchReadiness(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source), "registerOperationalRoutes(router, db, searchIndex.Ready)") {
		t.Fatal("composition root must wire both MySQL and Meilisearch readiness")
	}
}

func contains(value, fragment string) bool {
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
