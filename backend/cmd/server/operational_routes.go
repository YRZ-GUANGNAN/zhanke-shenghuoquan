package main

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"campus-circle/backend/internal/pkg/response"
)

var (
	buildVersion     = "dev"
	buildCommit      = "unknown"
	buildTime        = "unknown"
	gitCommitPattern = regexp.MustCompile(`^[0-9a-f]{7,64}$`)
)

type readinessCheck func(context.Context) error

func validateProductionBuildMetadata(appEnv string) error {
	if appEnv != "prod" {
		return nil
	}
	version := strings.TrimSpace(buildVersion)
	commit := strings.TrimSpace(buildCommit)
	builtAt := strings.TrimSpace(buildTime)
	if version == "" || version == "dev" || !gitCommitPattern.MatchString(commit) || strings.HasSuffix(commit, "-dirty") {
		return errors.New("production requires traceable release build metadata")
	}
	if _, err := time.Parse(time.RFC3339Nano, builtAt); err != nil {
		return errors.New("production requires a valid release build time")
	}
	return nil
}

func registerOperationalRoutes(router *gin.Engine, db *gorm.DB, dependencyChecks ...readinessCheck) {
	router.GET("/healthz", func(c *gin.Context) {
		response.OK(c, gin.H{"status": "ok"})
	})

	var check readinessCheck = func(context.Context) error {
		return errors.New("database handle is unavailable")
	}
	if db != nil {
		if sqlDB, err := db.DB(); err == nil {
			check = sqlDB.PingContext
		}
	}
	checks := append([]readinessCheck{check}, dependencyChecks...)
	router.GET("/readyz", readinessHandler(checks...))
	router.GET("/version", func(c *gin.Context) {
		response.OK(c, gin.H{
			"version": buildVersion,
			"commit":  buildCommit,
			"builtAt": buildTime,
		})
	})
}

func readinessHandler(checks ...readinessCheck) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(checks) == 0 {
			response.Fail(c, http.StatusServiceUnavailable, "service is not ready")
			return
		}
		for _, check := range checks {
			if check == nil {
				response.Fail(c, http.StatusServiceUnavailable, "service is not ready")
				return
			}
			ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			err := check(ctx)
			cancel()
			if err != nil {
				response.Fail(c, http.StatusServiceUnavailable, "service is not ready")
				return
			}
		}
		response.OK(c, gin.H{"status": "ready"})
	}
}
