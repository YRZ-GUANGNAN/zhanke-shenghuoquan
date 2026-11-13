package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrOfficialAccountSenderUnavailable = errors.New("official account sender unavailable")
	ErrOfficialAccountSenderRejected    = errors.New("official account sender rejected")
)

type OfficialAccountSendResult struct {
	Status            SendStatus
	ProviderMessageID string
	ErrorCode         string
}

type OfficialAccountSender struct {
	appID            string
	appSecret        string
	miniProgramAppID string
	apiBase          string
	enabled          bool
	client           *http.Client
	mu               sync.Mutex
	accessToken      string
	tokenUntil       time.Time
	now              func() time.Time
}

type OfficialAccountSenderConfig struct {
	AppID            string
	AppSecret        string
	MiniProgramAppID string
	APIBase          string
	Enabled          bool
	HTTPClient       *http.Client
}

func NewOfficialAccountSender(config OfficialAccountSenderConfig) (*OfficialAccountSender, error) {
	config.AppID = strings.TrimSpace(config.AppID)
	config.AppSecret = strings.TrimSpace(config.AppSecret)
	config.MiniProgramAppID = strings.TrimSpace(config.MiniProgramAppID)
	config.APIBase = strings.TrimRight(strings.TrimSpace(config.APIBase), "/")
	if config.APIBase == "" {
		config.APIBase = "https://api.weixin.qq.com"
	}
	parsed, err := url.Parse(config.APIBase)
	if err != nil || !validHTTPSOrLoopback(parsed) {
		return nil, ErrOfficialAccountSenderUnavailable
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &OfficialAccountSender{
		appID: config.AppID, appSecret: config.AppSecret, miniProgramAppID: config.MiniProgramAppID,
		apiBase: config.APIBase, enabled: config.Enabled, client: config.HTTPClient,
		now: func() time.Time { return time.Now().UTC() },
	}, nil
}

func (s *OfficialAccountSender) Enabled() bool {
	return s != nil && s.enabled && s.appID != "" && s.appSecret != ""
}

func (s *OfficialAccountSender) Send(ctx context.Context, message SubscribeMessage) (OfficialAccountSendResult, error) {
	if !s.Enabled() {
		return OfficialAccountSendResult{Status: SendStatusBlocked, ErrorCode: "configuration_missing"}, ErrOfficialAccountSenderUnavailable
	}
	if message.OpenID == "" || message.TemplateID == "" || message.Page == "" || len(message.Data) == 0 {
		return OfficialAccountSendResult{Status: SendStatusBlocked, ErrorCode: "invalid_payload"}, ErrSubscribePayload
	}
	token, err := s.token(ctx, false)
	if err != nil {
		return OfficialAccountSendResult{Status: SendStatusUnknown, ErrorCode: "token_unavailable"}, err
	}
	result, err := s.sendWithToken(ctx, token, message)
	if result.ErrorCode == "token_invalid" {
		token, tokenErr := s.token(ctx, true)
		if tokenErr != nil {
			return OfficialAccountSendResult{Status: SendStatusUnknown, ErrorCode: "token_unavailable"}, tokenErr
		}
		return s.sendWithToken(ctx, token, message)
	}
	return result, err
}

func (s *OfficialAccountSender) sendWithToken(ctx context.Context, token string, message SubscribeMessage) (OfficialAccountSendResult, error) {
	body, err := json.Marshal(map[string]any{
		"touser":      message.OpenID,
		"template_id": message.TemplateID,
		"miniprogram": map[string]string{
			"appid":    s.miniProgramAppID,
			"pagepath": strings.TrimPrefix(message.Page, "/"),
		},
		"data": message.Data,
	})
	if err != nil {
		return OfficialAccountSendResult{Status: SendStatusBlocked, ErrorCode: "invalid_payload"}, ErrSubscribePayload
	}
	endpoint := s.apiBase + "/cgi-bin/message/template/send?access_token=" + url.QueryEscape(token)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return OfficialAccountSendResult{Status: SendStatusBlocked, ErrorCode: "configuration_missing"}, ErrOfficialAccountSenderUnavailable
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return OfficialAccountSendResult{Status: SendStatusUnknown, ErrorCode: "transport_error"}, ErrOfficialAccountSenderUnavailable
	}
	defer response.Body.Close()
	var value struct {
		ErrCode   int    `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		MessageID int64  `json:"msgid"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&value) != nil {
		return OfficialAccountSendResult{Status: SendStatusUnknown, ErrorCode: "invalid_response"}, ErrOfficialAccountSenderRejected
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 && value.ErrCode == 0 {
		return OfficialAccountSendResult{Status: SendStatusAccepted, ProviderMessageID: strconv.FormatInt(value.MessageID, 10)}, nil
	}
	code := "provider_rejected"
	switch value.ErrCode {
	case 40001, 40014, 42001:
		code = "token_invalid"
	case 43004:
		code = "not_following"
	case 45009:
		code = "rate_limited"
	}
	return OfficialAccountSendResult{Status: SendStatusRejected, ErrorCode: code}, fmt.Errorf("%w: errcode=%d", ErrOfficialAccountSenderRejected, value.ErrCode)
}

func (s *OfficialAccountSender) token(ctx context.Context, refresh bool) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if !refresh && s.accessToken != "" && s.tokenUntil.After(now.Add(5*time.Minute)) {
		return s.accessToken, nil
	}
	query := url.Values{}
	query.Set("grant_type", "client_credential")
	query.Set("appid", s.appID)
	query.Set("secret", s.appSecret)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiBase+"/cgi-bin/token?"+query.Encode(), nil)
	if err != nil {
		return "", ErrOfficialAccountSenderUnavailable
	}
	response, err := s.client.Do(request)
	if err != nil {
		return "", ErrOfficialAccountSenderUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", ErrOfficialAccountSenderUnavailable
	}
	var value struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&value) != nil || value.ErrCode != 0 ||
		value.AccessToken == "" || value.ExpiresIn <= 0 {
		return "", ErrOfficialAccountSenderUnavailable
	}
	s.accessToken = value.AccessToken
	s.tokenUntil = now.Add(time.Duration(value.ExpiresIn) * time.Second)
	return s.accessToken, nil
}
