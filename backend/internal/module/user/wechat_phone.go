package user

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type WechatPhoneClient struct {
	appID     string
	secret    string
	http      *http.Client
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

func NewWechatPhoneClient(appID, secret string, client *http.Client) *WechatPhoneClient {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	return &WechatPhoneClient{appID: appID, secret: secret, http: client}
}

func (c *WechatPhoneClient) ResolvePhone(ctx context.Context, code string) (string, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return "", err
	}
	body, err := json.Marshal(map[string]string{"code": code})
	if err != nil {
		return "", err
	}
	endpoint := "https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token=" + url.QueryEscape(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		ErrCode   int    `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		PhoneInfo struct {
			PhoneNumber string `json:"phoneNumber"`
		} `json:"phone_info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.ErrCode != 0 || result.PhoneInfo.PhoneNumber == "" {
		return "", fmt.Errorf("wechat phone %d: %s", result.ErrCode, result.ErrMsg)
	}
	return result.PhoneInfo.PhoneNumber, nil
}

func (c *WechatPhoneClient) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.expiresAt) {
		return c.token, nil
	}
	if c.appID == "" || c.secret == "" {
		return "", ErrPhoneBindingUnavailable
	}
	endpoint := "https://api.weixin.qq.com/cgi-bin/token?grant_type=client_credential&appid=" + url.QueryEscape(c.appID) + "&secret=" + url.QueryEscape(c.secret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.AccessToken == "" {
		if result.ErrCode == 0 {
			return "", errors.New("wechat returned an empty access token")
		}
		return "", fmt.Errorf("wechat access token %d: %s", result.ErrCode, result.ErrMsg)
	}
	ttl := time.Duration(result.ExpiresIn) * time.Second
	if ttl <= time.Minute {
		ttl = 5 * time.Minute
	}
	c.token, c.expiresAt = result.AccessToken, time.Now().Add(ttl-time.Minute)
	return c.token, nil
}
