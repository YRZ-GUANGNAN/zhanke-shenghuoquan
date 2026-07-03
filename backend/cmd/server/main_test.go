package main

import (
	"context"
	"testing"
	"time"

	"campus-circle/backend/internal/config"
	"campus-circle/backend/internal/module/media/miniappflow"
	legacymedia "campus-circle/backend/internal/pkg/media"
)

type moduleReviewSignerStub struct{}

func (moduleReviewSignerStub) SignedGetURL(context.Context, string, string, time.Duration) (string, error) {
	return "https://cos.example.com/final.webp?sign=short", nil
}

func TestNewWechatMediaSafetyUsesNoopOnlyForLocalMockWithoutCredentials(t *testing.T) {
	checker, err := newWechatMediaSafety(config.Config{AppEnv: "dev", AllowMockWechat: true})
	if err != nil {
		t.Fatalf("newWechatMediaSafety returned error: %v", err)
	}
	if _, ok := checker.(*legacymedia.LocalNoopSafety); !ok {
		t.Fatalf("expected LocalNoopSafety, got %T", checker)
	}
}

func TestNewWechatMediaSafetyKeepsRealPathWhenCredentialsProvided(t *testing.T) {
	checker, err := newWechatMediaSafety(config.Config{
		AppEnv: "dev", WechatAppID: "wx-test", WechatAppSecret: "secret-test", AllowMockWechat: true,
	})
	if err != nil {
		t.Fatalf("newWechatMediaSafety returned error: %v", err)
	}
	if _, ok := checker.(*legacymedia.WechatSafety); !ok {
		t.Fatalf("expected WechatSafety, got %T", checker)
	}
}

func TestNewWechatMediaSafetyRejectsNonMockEmptyCredentials(t *testing.T) {
	if _, err := newWechatMediaSafety(config.Config{AppEnv: "dev"}); err == nil {
		t.Fatal("expected empty non-mock credentials to fail")
	}
}

func TestNewWechatMediaSafetyKeepsProductionPath(t *testing.T) {
	if _, err := newWechatMediaSafety(config.Config{AppEnv: "prod", AllowMockWechat: true}); err == nil {
		t.Fatal("expected production empty credentials to use real path and fail")
	}
}

func TestNewWechatReviewSubmitterUsesNoopOnlyForLocalMockWithoutCredentials(t *testing.T) {
	submitter, err := newWechatReviewSubmitter(config.Config{AppEnv: "dev", AllowMockWechat: true}, moduleReviewSignerStub{})
	if err != nil {
		t.Fatalf("newWechatReviewSubmitter returned error: %v", err)
	}
	if submitter != nil {
		t.Fatalf("expected nil submitter in local mock mode, got %T", submitter)
	}
}

func TestNewWechatReviewSubmitterKeepsRealPathWhenCredentialsProvided(t *testing.T) {
	submitter, err := newWechatReviewSubmitter(config.Config{
		AppEnv: "dev", WechatAppID: "wx-test", WechatAppSecret: "secret-test", AllowMockWechat: true,
	}, moduleReviewSignerStub{})
	if err != nil {
		t.Fatalf("newWechatReviewSubmitter returned error: %v", err)
	}
	if _, ok := submitter.(*miniappflow.WechatAsyncReviewSubmitter); !ok {
		t.Fatalf("expected WechatAsyncReviewSubmitter, got %T", submitter)
	}
}

func TestNewWechatReviewSubmitterRejectsNonMockEmptyCredentials(t *testing.T) {
	if _, err := newWechatReviewSubmitter(config.Config{AppEnv: "dev"}, moduleReviewSignerStub{}); err == nil {
		t.Fatal("expected empty non-mock credentials to fail")
	}
}

func TestNewWechatReviewSubmitterKeepsProductionPath(t *testing.T) {
	if _, err := newWechatReviewSubmitter(config.Config{AppEnv: "prod", AllowMockWechat: true}, moduleReviewSignerStub{}); err == nil {
		t.Fatal("expected production empty credentials to use real path and fail")
	}
}
