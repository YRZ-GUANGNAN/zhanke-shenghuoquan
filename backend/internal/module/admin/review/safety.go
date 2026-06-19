package review

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"campus-circle/backend/internal/config"
)

// SafetyClient is domain F's pre-review adapter for WeChat content safety.
type SafetyClient struct {
	appID, appSecret string
	httpClient       *http.Client
}

func NewSafetyClient(cfg config.Config) *SafetyClient {
	return &SafetyClient{appID: cfg.WechatAppID, appSecret: cfg.WechatAppSecret, httpClient: &http.Client{Timeout: 5 * time.Second}}
}

func (c *SafetyClient) CheckText(ctx context.Context, openID, content string) string {
	if c.appID == "" || c.appSecret == "" || content == "" {
		return "wechat safety skipped"
	}
	token, err := c.accessToken(ctx)
	if err != nil {
		return "wechat token error: " + err.Error()
	}
	body, _ := json.Marshal(map[string]any{"version": 2, "scene": 2, "openid": openID, "content": content})
	endpoint := "https://api.weixin.qq.com/wxa/msg_sec_check?access_token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "wechat request error: " + err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "wechat request error: " + err.Error()
	}
	defer resp.Body.Close()
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		Result  struct {
			Suggest string `json:"suggest"`
			Label   int    `json:"label"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "wechat decode error: " + err.Error()
	}
	if result.ErrCode != 0 {
		return fmt.Sprintf("wechat error %d: %s", result.ErrCode, result.ErrMsg)
	}
	if result.Result.Suggest == "" {
		return "wechat pass"
	}
	return fmt.Sprintf("wechat %s label=%d", result.Result.Suggest, result.Result.Label)
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
	var result struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("%d %s", result.ErrCode, result.ErrMsg)
	}
	return result.AccessToken, nil
}
