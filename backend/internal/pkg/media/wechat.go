package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type WechatSafety struct {
	appID, appSecret string
	apiBase          string
	httpClient       *http.Client
	mu               sync.Mutex
	token            string
	tokenExpiresAt   time.Time
}

func NewWechatSafety(appID, appSecret, apiBase string) (*WechatSafety, error) {
	if appID == "" || appSecret == "" {
		return nil, errors.New("WeChat app id and secret are required for media safety")
	}
	if apiBase == "" {
		apiBase = "https://api.weixin.qq.com"
	}
	return &WechatSafety{appID: appID, appSecret: appSecret, apiBase: strings.TrimRight(apiBase, "/"), httpClient: &http.Client{Timeout: 10 * time.Second}}, nil
}

func (c *WechatSafety) Submit(ctx context.Context, openID, mediaURL string) (string, error) {
	if openID == "" || mediaURL == "" {
		return "", errors.New("openid and media url are required")
	}
	token, err := c.accessToken(ctx)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(map[string]any{"media_url": mediaURL, "media_type": 2, "version": 2, "scene": 2, "openid": openID})
	if err != nil {
		return "", err
	}
	endpoint := c.apiBase + "/wxa/media_check_async?access_token=" + url.QueryEscape(token)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		TraceID string `json:"trace_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.ErrCode != 0 || result.TraceID == "" {
		return "", fmt.Errorf("wechat media check %d: %s", result.ErrCode, result.ErrMsg)
	}
	return result.TraceID, nil
}

func (c *WechatSafety) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExpiresAt) {
		return c.token, nil
	}
	endpoint := c.apiBase + "/cgi-bin/token?grant_type=client_credential&appid=" + url.QueryEscape(c.appID) + "&secret=" + url.QueryEscape(c.appSecret)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("wechat token %d: %s", result.ErrCode, result.ErrMsg)
	}
	ttl := time.Duration(result.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	c.token, c.tokenExpiresAt = result.AccessToken, time.Now().Add(ttl-5*time.Minute)
	return c.token, nil
}

func VerifyWechatSignature(token, timestamp, nonce, signature string) bool {
	if token == "" || timestamp == "" || nonce == "" || signature == "" {
		return false
	}
	values := []string{token, timestamp, nonce}
	sort.Strings(values)
	return sha1Hex(strings.Join(values, "")) == strings.ToLower(signature)
}

var _ SafetyChecker = (*WechatSafety)(nil)
