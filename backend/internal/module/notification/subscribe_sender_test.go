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
	result, err := sender.Send(context.Background(), SubscribeMessage{OpenID: "openid", TemplateID: "template"})
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
		{name: "rate", errCode: 45009, wantStatus: SendStatusUnknown, wantCode: "rate_limited"},
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
	result, err := sender.Send(context.Background(), SubscribeMessage{OpenID: "openid-private", TemplateID: "template-key"})
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
