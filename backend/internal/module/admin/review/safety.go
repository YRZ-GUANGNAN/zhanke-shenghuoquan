package review

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"campus-circle/backend/internal/config"
)

type TextSafetyChecker interface {
	CheckText(context.Context, string, string) string
}

// SafetyClient is domain F's pre-review adapter for WeChat content safety.
type SafetyClient struct {
	appID, appSecret string
	httpClient       *http.Client
}

func NewSafetyClient(cfg config.Config) *SafetyClient {
	return &SafetyClient{appID: cfg.WechatAppID, appSecret: cfg.WechatAppSecret, httpClient: &http.Client{Timeout: 5 * time.Second}}
}

var _ TextSafetyChecker = (*SafetyClient)(nil)

func (c *SafetyClient) CheckText(ctx context.Context, openID, content string) string {
	if strings.TrimSpace(openID) == "" {
		return "manual:wechat_openid_missing"
	}
	if strings.TrimSpace(content) == "" {
		return "manual:wechat_content_empty"
	}
	if strings.TrimSpace(c.appID) == "" || strings.TrimSpace(c.appSecret) == "" {
		return "manual:wechat_safety_unconfigured"
	}
	token, err := c.accessToken(ctx)
	if err != nil {
		return "manual:wechat_token_unavailable"
	}
	body, err := json.Marshal(map[string]any{"version": 2, "scene": 2, "openid": openID, "content": content})
	if err != nil {
		return "manual:wechat_request_failed"
	}
	endpoint := "https://api.weixin.qq.com/wxa/msg_sec_check?access_token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "manual:wechat_request_failed"
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "manual:wechat_request_failed"
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "manual:wechat_http_error"
	}
	var result struct {
		ErrCode int `json:"errcode"`
		Result  struct {
			Suggest string `json:"suggest"`
			Label   int    `json:"label"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "manual:wechat_response_invalid"
	}
	if result.ErrCode != 0 {
		return fmt.Sprintf("manual:wechat_api_error:%d", result.ErrCode)
	}
	suggest := strings.ToLower(strings.TrimSpace(result.Result.Suggest))
	switch suggest {
	case "", "pass":
		return "wechat pass"
	case "review", "risky":
		return fmt.Sprintf("wechat %s label=%d", suggest, result.Result.Label)
	default:
		return "manual:wechat_response_invalid"
	}
}

func (c *SafetyClient) accessToken(ctx context.Context) (string, error) {
	endpoint := "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=" + url.QueryEscape(c.appID) + "&secret=" + url.QueryEscape(c.appSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", errors.New("wechat token http error")
	}
	var result struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.AccessToken) == "" || result.ErrCode != 0 {
		return "", errors.New("wechat token unavailable")
	}
	return result.AccessToken, nil
}
