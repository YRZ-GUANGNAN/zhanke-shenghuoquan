package main

import (
	"testing"

	"campus-circle/backend/internal/config"
	legacymedia "campus-circle/backend/internal/pkg/media"
)

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
