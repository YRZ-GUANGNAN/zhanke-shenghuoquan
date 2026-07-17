package review

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"campus-circle/backend/internal/config"
)

type safetyRoundTripFunc func(*http.Request) (*http.Response, error)

func (f safetyRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func safetyResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func safetyClientForTest(transport safetyRoundTripFunc) *SafetyClient {
	client := NewSafetyClient(config.Config{WechatAppID: "app-id", WechatAppSecret: "app-secret"})
	client.httpClient = &http.Client{Transport: transport}
	return client
}

func tokenThen(next safetyRoundTripFunc) safetyRoundTripFunc {
	calls := 0
	return func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return safetyResponse(http.StatusOK, `{"access_token":"test-token"}`), nil
		}
		return next(r)
	}
}

var _ TextSafetyChecker = (*SafetyClient)(nil)

func TestSafetyClientShortCircuitsUnsafeInputs(t *testing.T) {
	calls := 0
	transport := safetyRoundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("must not be called")
	})
	client := safetyClientForTest(transport)
	if got := client.CheckText(context.Background(), "  ", "content"); got != "manual:wechat_openid_missing" {
		t.Fatalf("missing openid risk=%q", got)
	}
	if got := client.CheckText(context.Background(), "openid-secret", "  "); got != "manual:wechat_content_empty" {
		t.Fatalf("empty content risk=%q", got)
	}
	unconfigured := NewSafetyClient(config.Config{})
	unconfigured.httpClient = &http.Client{Transport: transport}
	if got := unconfigured.CheckText(context.Background(), "openid-secret", "content"); got != "manual:wechat_safety_unconfigured" {
		t.Fatalf("unconfigured risk=%q", got)
	}
	if calls != 0 {
		t.Fatalf("short circuit made %d HTTP calls", calls)
	}
}

func TestSafetyClientSendsTrustedIdentityAndContent(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{}{}, "request-context")
	client := safetyClientForTest(tokenThen(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/wxa/msg_sec_check" || r.URL.Query().Get("access_token") != "test-token" || r.Context() != ctx {
			t.Fatalf("unexpected safety request: method=%s url=%s contextMatch=%v", r.Method, r.URL, r.Context() == ctx)
		}
		var body struct {
			Version int    `json:"version"`
			Scene   int    `json:"scene"`
			OpenID  string `json:"openid"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Version != 2 || body.Scene != 2 || body.OpenID != "openid-secret" || body.Content != "title\ncontent" {
			t.Fatalf("unexpected body: %#v", body)
		}
		return safetyResponse(http.StatusOK, `{"errcode":0,"result":{"suggest":"pass","label":100}}`), nil
	}))
	if got := client.CheckText(ctx, "openid-secret", "title\ncontent"); got != "wechat pass" {
		t.Fatalf("risk=%q", got)
	}
}

func TestSafetyClientReturnsFixedManualRisksWithoutSecrets(t *testing.T) {
	tests := []struct {
		name      string
		want      string
		transport safetyRoundTripFunc
	}{
		{"token", "manual:wechat_token_unavailable", func(*http.Request) (*http.Response, error) { return nil, errors.New("token upstream-secret") }},
		{"request", "manual:wechat_request_failed", tokenThen(func(*http.Request) (*http.Response, error) { return nil, errors.New("body upstream-secret") })},
		{"http", "manual:wechat_http_error", tokenThen(func(*http.Request) (*http.Response, error) {
			return safetyResponse(http.StatusBadGateway, "upstream-secret"), nil
		})},
		{"decode", "manual:wechat_response_invalid", tokenThen(func(*http.Request) (*http.Response, error) {
			return safetyResponse(http.StatusOK, "upstream-secret"), nil
		})},
		{"api", "manual:wechat_api_error:87014", tokenThen(func(*http.Request) (*http.Response, error) {
			return safetyResponse(http.StatusOK, `{"errcode":87014,"errmsg":"upstream-secret"}`), nil
		})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := safetyClientForTest(test.transport).CheckText(context.Background(), "openid-secret", "content-secret")
			if got != test.want {
				t.Fatalf("risk=%q want=%q", got, test.want)
			}
			for _, secret := range []string{"openid-secret", "content-secret", "test-token", "upstream-secret"} {
				if strings.Contains(got, secret) {
					t.Fatalf("risk leaked %q: %q", secret, got)
				}
			}
		})
	}
}
