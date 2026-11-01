package notification

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/config"
)

func TestSubscribeSenderDefaultsToBlocked(t *testing.T) {
	sender, err := NewSubscribeSender(config.Config{
		WechatAppID: "wx-app", WechatAppSecret: "secret",
		WechatAPIBase: "https://api.weixin.qq.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if sender.Enabled() {
		t.Fatal("sender must remain disabled unless both notification switches are enabled")
	}
	result, err := sender.Send(context.Background(), SubscribeMessage{OpenID: "openid", TemplateID: "template", Data: map[string]SubscribeTemplateValue{"thing1": {Value: "status"}}})
	if !errors.Is(err, ErrSubscribeBlocked) || result.Status != SendStatusBlocked || result.ErrorCode != "configuration_missing" {
		t.Fatalf("blocked sender result=%+v err=%v", result, err)
	}
}

func TestSubscribeSenderMapsProviderErrorsWithoutSensitiveData(t *testing.T) {
	cases := []struct {
		name       string
		errCode    int
		wantStatus string
		wantCode   string
	}{
		{name: "template", errCode: 40037, wantStatus: SendStatusRejected, wantCode: "template_invalid"},
		{name: "token", errCode: 40001, wantStatus: SendStatusRejected, wantCode: "token_invalid"},
		{name: "rate", errCode: 45009, wantStatus: SendStatusRejected, wantCode: "rate_limited"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/cgi-bin/token" {
					_, _ = w.Write([]byte(`{"access_token":"token-for-test","expires_in":7200}`))
					return
				}
				_, _ = w.Write([]byte(`{"errcode":` + strconv.Itoa(test.errCode) + `,"errmsg":"provider detail must not escape"}`))
			}))
			defer server.Close()
			sender, err := newSubscribeSender(subscribeSenderConfig{
				AppID: "wx-app", AppSecret: "secret", APIBase: server.URL,
				Enabled: true, HTTPClient: server.Client(), RefreshSkew: time.Minute,
			})
			if err != nil {
				t.Fatal(err)
			}
			result, err := sender.Send(context.Background(), SubscribeMessage{OpenID: "openid-private", TemplateID: "template-key", Data: map[string]SubscribeTemplateValue{"thing1": {Value: "订单状态"}}})
			if result.Status != test.wantStatus || result.ErrorCode != test.wantCode {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if strings.Contains(err.Error(), "openid-private") || strings.Contains(err.Error(), "provider detail") {
				t.Fatalf("sensitive provider data escaped error: %v", err)
			}
		})
	}
}

func TestSubscribeSenderTimeoutIsUnknown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/token" {
			_, _ = w.Write([]byte(`{"access_token":"token-for-test","expires_in":7200}`))
			return
		}
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()
	client := server.Client()
	client.Timeout = 10 * time.Millisecond
	sender, err := newSubscribeSender(subscribeSenderConfig{AppID: "wx-app", AppSecret: "secret", APIBase: server.URL, Enabled: true, HTTPClient: client})
	if err != nil {
		t.Fatal(err)
	}
	result, err := sender.Send(context.Background(), SubscribeMessage{OpenID: "openid-private", TemplateID: "template-key", Data: map[string]SubscribeTemplateValue{"thing1": {Value: "status"}}})
	if result.Status != SendStatusUnknown || result.ErrorCode != "timeout" || !errors.Is(err, ErrSubscribeUnknown) {
		t.Fatalf("timeout result=%+v err=%v", result, err)
	}
}

func TestSubscribeSenderRejectsOversizedPayload(t *testing.T) {
	sender, err := newSubscribeSender(subscribeSenderConfig{AppID: "wx-app", AppSecret: "secret", APIBase: "https://api.weixin.qq.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	result, err := sender.Send(context.Background(), SubscribeMessage{OpenID: "openid", TemplateID: "template", Data: map[string]SubscribeTemplateValue{"thing1": {Value: strings.Repeat("x", 257)}}})
	if result.Status != SendStatusBlocked || result.ErrorCode != "invalid_payload" || !errors.Is(err, ErrSubscribePayload) {
		t.Fatalf("invalid payload result=%+v err=%v", result, err)
	}
}

func TestSubscribeSenderRejectsExternalPageAndMissingErrorCode(t *testing.T) {
	sender, err := newSubscribeSender(subscribeSenderConfig{AppID: "wx-app", AppSecret: "secret", APIBase: "https://api.weixin.qq.com", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	result, err := sender.Send(context.Background(), SubscribeMessage{OpenID: "openid", TemplateID: "template", Page: "https://evil.example/redirect"})
	if result.Status != SendStatusBlocked || !errors.Is(err, ErrSubscribePayload) {
		t.Fatalf("external page result=%+v err=%v", result, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cgi-bin/token" {
			_, _ = w.Write([]byte(`{"access_token":"token-for-test","expires_in":7200}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	sender, err = newSubscribeSender(subscribeSenderConfig{AppID: "wx-app", AppSecret: "secret", APIBase: server.URL, Enabled: true, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	result, err = sender.Send(context.Background(), SubscribeMessage{OpenID: "openid", TemplateID: "template", Data: map[string]SubscribeTemplateValue{"thing1": {Value: "status"}}})
	if result.Status != SendStatusUnknown || result.ErrorCode != "invalid_response" || !errors.Is(err, ErrSubscribeUnknown) {
		t.Fatalf("missing errcode result=%+v err=%v", result, err)
	}
}

func TestSubscribeSenderCopiesClientAndForcesNoRedirect(t *testing.T) {
	redirects := 0
	shared := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { redirects++; return nil }}
	sender, err := newSubscribeSender(subscribeSenderConfig{AppID: "app", AppSecret: "secret", APIBase: "https://api.weixin.qq.com", HTTPClient: shared})
	if err != nil {
		t.Fatal(err)
	}
	if sender.client == shared {
		t.Fatal("shared client mutated")
	}
	if err := sender.client.CheckRedirect(nil, nil); err != http.ErrUseLastResponse {
		t.Fatal("redirect was permitted")
	}
	if err := shared.CheckRedirect(nil, nil); err != nil || redirects != 1 {
		t.Fatal("caller redirect policy changed")
	}
	for _, base := range []string{"http://api.weixin.qq.com", "https://api.weixin.qq.com:443", "https://secret@api.weixin.qq.com", "https://evil.example", "http://169.254.169.254"} {
		if _, err := NewSubscribeSender(config.Config{WechatAPIBase: base}); err == nil {
			t.Fatalf("unsafe API base accepted: %s", base)
		}
	}
}

func TestSubscribeSenderRejectsNullErrorCodeAndTrailingJSON(t *testing.T) {
	for _, raw := range []string{`{"errcode":null}`, `{"errcode":0} {}`, `{"errcode":"0"}`} {
		_, present, err := decodeWechatResponse(strings.NewReader(raw))
		if present && err == nil {
			t.Fatalf("ambiguous response accepted: %s", raw)
		}
	}
	if ValidateTemplateSnapshot([]byte(`{"templateId":"template","page":"/pages/notifications/index","data":{}}`)) == nil {
		t.Fatal("empty data accepted")
	}
	if err := ValidateTemplateSnapshot([]byte(`{"templateId":"template","page":"/pages/notifications/index","data":{"thing1":{"value":"status"}}}`)); err != nil {
		t.Fatal(err)
	}
}
