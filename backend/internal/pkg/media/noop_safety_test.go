package media

import (
	"context"
	"strings"
	"testing"
)

func TestLocalNoopSafetySubmitReturnsLocalTrace(t *testing.T) {
	client := NewLocalNoopSafety()
	trace, err := client.Submit(context.Background(), "mock-openid", "https://example.test/media.jpg")
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if !strings.HasPrefix(trace, "local-noop-") {
		t.Fatalf("unexpected trace id: %q", trace)
	}
}

func TestLocalNoopSafetySubmitValidatesInputs(t *testing.T) {
	client := NewLocalNoopSafety()
	if _, err := client.Submit(context.Background(), "", "https://example.test/media.jpg"); err == nil {
		t.Fatal("expected empty openid to fail")
	}
	if _, err := client.Submit(context.Background(), "mock-openid", ""); err == nil {
		t.Fatal("expected empty media URL to fail")
	}
}
