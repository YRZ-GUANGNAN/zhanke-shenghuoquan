package media

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestTencentCompletionStoreSignedGetURLIncludesFixedVersion(t *testing.T) {
	store, err := NewTencentCompletionStore(TencentCompletionStoreConfig{
		SecretID: "AKIDEXAMPLE", SecretKey: "SECRETEXAMPLE",
		Bucket: "campus-test-1250000000", Region: "ap-guangzhou",
	})
	if err != nil {
		t.Fatal(err)
	}
	value, err := store.SignedGetURL(context.Background(), "final/display/example.webp", "final-version-1", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("versionId") != "final-version-1" {
		t.Fatalf("signed URL missing fixed version: %s", value)
	}
	if parsed.Query().Get("q-signature") == "" || parsed.Query().Get("q-sign-time") == "" {
		t.Fatalf("signed URL missing Tencent COS signature params: %s", value)
	}
	if !strings.Contains(parsed.Path, "/final/display/example.webp") {
		t.Fatalf("signed URL path=%s", parsed.Path)
	}
}

func TestTencentCompletionStoreSignedGetURLRejectsIncompleteIdentity(t *testing.T) {
	store, err := NewTencentCompletionStore(TencentCompletionStoreConfig{
		SecretID: "AKIDEXAMPLE", SecretKey: "SECRETEXAMPLE",
		Bucket: "campus-test-1250000000", Region: "ap-guangzhou",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SignedGetURL(context.Background(), "final/display/example.webp", "", 5*time.Minute); err == nil {
		t.Fatal("empty version must fail closed")
	}
	if _, err := store.SignedGetURL(context.Background(), "", "final-version-1", 5*time.Minute); err == nil {
		t.Fatal("empty key must fail closed")
	}
	if _, err := store.SignedGetURL(context.Background(), "final/display/example.webp", "final-version-1", 0); err == nil {
		t.Fatal("non-positive ttl must fail closed")
	}
}
