package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"campus-circle/backend/internal/config"
)

type SendStatus string

const (
	SendStatusAccepted = "accepted"
	SendStatusBlocked  = "blocked"
	SendStatusUnknown  = "unknown"
	SendStatusRejected = "rejected"
)

var (
	ErrSubscribeBlocked   = errors.New("wechat subscribe sender is blocked")
	ErrSubscribeRejected  = errors.New("wechat subscribe request was rejected")
	ErrSubscribeUnknown   = errors.New("wechat subscribe request result is unknown")
	ErrSubscribePayload   = errors.New("invalid wechat subscribe payload")
	ErrSubscribeTransport = errors.New("wechat subscribe transport failed")
)

// SubscribeTemplateValue is the only value shape accepted for a template
// message. It intentionally has no fields for arbitrary provider metadata.
type SubscribeTemplateValue struct {
	Value string `json:"value"`
}

// SubscribeMessage contains server-resolved recipient and template data. The
// openid is never included in logs or errors produced by this package.
type SubscribeMessage struct {
	TaskID     string                            `json:"-"`
	OpenID     string                            `json:"touser"`
	TemplateID string                            `json:"template_id"`
	Page       string                            `json:"page,omitempty"`
	Data       map[string]SubscribeTemplateValue `json:"data"`
}

type SubscribeSendResult struct {
	Status            string
	ProviderMessageID string
	ErrorCode         string
}

// Sender is deliberately independent of delivery/user/im/community. The
// composition root can adapt its notification delivery lease to this shape.
type Sender interface {
	Enabled() bool
	Send(context.Context, SubscribeMessage) (SubscribeSendResult, error)
}

type SubscribeSender struct {
	appID       string
	appSecret   string
	apiBase     string
	enabled     bool
	client      *http.Client
	now         func() time.Time
	refreshSkew time.Duration

	mu          sync.Mutex
	accessToken string
	tokenUntil  time.Time
}

type subscribeSenderConfig struct {
	AppID       string
	AppSecret   string
	APIBase     string
	Enabled     bool
	HTTPClient  *http.Client
	Now         func() time.Time
	RefreshSkew time.Duration
}

func NewSubscribeSender(cfg config.Config, clients ...*http.Client) (*SubscribeSender, error) {
	var client *http.Client
	if len(clients) > 0 {
		client = clients[0]
	}
	return newSubscribeSender(subscribeSenderConfig{
		AppID:       cfg.WechatAppID,
		AppSecret:   cfg.WechatAppSecret,
		APIBase:     cfg.WechatAPIBase,
		Enabled:     cfg.EnableWechatSubscribeNotifications && cfg.EnableNotificationSending,
		HTTPClient:  client,
		RefreshSkew: 5 * time.Minute,
	})
}

func newSubscribeSender(cfg subscribeSenderConfig) (*SubscribeSender, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.APIBase), "/")
	if base == "" {
		base = "https://api.weixin.qq.com"
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("invalid WeChat API base URL")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 8 * time.Second}
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	if cfg.RefreshSkew <= 0 {
		cfg.RefreshSkew = 5 * time.Minute
	}
	return &SubscribeSender{
		appID: cfg.AppID, appSecret: cfg.AppSecret, apiBase: base,
		enabled: cfg.Enabled,
		client:  cfg.HTTPClient, now: cfg.Now, refreshSkew: cfg.RefreshSkew,
		// A sender with incomplete credentials is intentionally constructed but
		// disabled, so workers can record a blocked task without sending.
		accessToken: "",
	}, nil
}

func (s *SubscribeSender) Enabled() bool {
	return s != nil && s.enabled && s.appID != "" && s.appSecret != ""
}

// EnabledForConfig reports the configured feature state. Enabled is kept
// credential based so a malformed runtime configuration can never send.
func (s *SubscribeSender) EnabledForConfig(cfg config.Config) bool {
	return s.Enabled() && cfg.EnableWechatSubscribeNotifications && cfg.EnableNotificationSending
}

func (s *SubscribeSender) Send(ctx context.Context, message SubscribeMessage) (SubscribeSendResult, error) {
	if s == nil || !s.Enabled() {
		return SubscribeSendResult{Status: SendStatusBlocked, ErrorCode: "configuration_missing"}, ErrSubscribeBlocked
	}
	if err := validateSubscribeMessage(message); err != nil {
		return SubscribeSendResult{Status: SendStatusBlocked, ErrorCode: "invalid_payload"}, fmt.Errorf("%w: %v", ErrSubscribePayload, err)
	}
	token, err := s.token(ctx)
	if err != nil {
		return SubscribeSendResult{Status: errStatus(err), ErrorCode: errorCode(err)}, err
	}
	body, err := json.Marshal(message)
	if err != nil {
		return SubscribeSendResult{Status: SendStatusBlocked, ErrorCode: "invalid_payload"}, ErrSubscribePayload
	}
	endpoint := s.apiBase + "/cgi-bin/message/subscribe/send"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return SubscribeSendResult{Status: SendStatusUnknown, ErrorCode: "request_build_failed"}, ErrSubscribeUnknown
	}
	query := request.URL.Query()
	query.Set("access_token", token)
	request.URL.RawQuery = query.Encode()
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
			return SubscribeSendResult{Status: SendStatusUnknown, ErrorCode: "timeout"}, fmt.Errorf("%w: timeout", ErrSubscribeUnknown)
		}
		return SubscribeSendResult{Status: SendStatusUnknown, ErrorCode: "transport"}, fmt.Errorf("%w: transport", ErrSubscribeUnknown)
	}
	defer response.Body.Close()
	var result wechatResponse
	if err := decodeLimited(response.Body, &result); err != nil {
		return SubscribeSendResult{Status: SendStatusUnknown, ErrorCode: "invalid_response"}, ErrSubscribeUnknown
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if result.ErrCode == 0 {
			return SubscribeSendResult{Status: SendStatusUnknown, ErrorCode: "http_error"}, ErrSubscribeUnknown
		}
	}
	if result.ErrCode != 0 {
		mapped := mapWechatError(result.ErrCode)
		if mapped == "token_invalid" {
			s.invalidateToken(token)
		}
		status := SendStatusRejected
		if mapped == "rate_limited" {
			status = SendStatusUnknown
		}
		return SubscribeSendResult{Status: status, ErrorCode: mapped}, providerError(status, mapped)
	}
	return SubscribeSendResult{Status: SendStatusAccepted, ProviderMessageID: result.MsgID}, nil
}

type wechatResponse struct {
	ErrCode int64  `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	MsgID   string `json:"msgid"`
}

func (s *SubscribeSender) token(ctx context.Context) (string, error) {
	now := s.now().UTC()
	s.mu.Lock()
	if s.accessToken != "" && now.Before(s.tokenUntil) {
		token := s.accessToken
		s.mu.Unlock()
		return token, nil
	}
	s.mu.Unlock()

	endpoint := s.apiBase + "/cgi-bin/token?grant_type=client_credential&appid=" + url.QueryEscape(s.appID) + "&secret=" + url.QueryEscape(s.appSecret)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("%w: request", ErrSubscribeUnknown)
	}
	response, err := s.client.Do(request)
	if err != nil {
		if isTimeout(err) || errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("%w: timeout", ErrSubscribeUnknown)
		}
		return "", fmt.Errorf("%w: transport", ErrSubscribeUnknown)
	}
	defer response.Body.Close()
	var result struct {
		wechatResponse
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := decodeLimited(response.Body, &result); err != nil {
		return "", ErrSubscribeUnknown
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || result.ErrCode != 0 || result.AccessToken == "" {
		if result.ErrCode != 0 {
			mapped := mapWechatError(result.ErrCode)
			status := SendStatusRejected
			if mapped == "rate_limited" {
				status = SendStatusUnknown
			}
			return "", providerError(status, mapped)
		}
		return "", ErrSubscribeUnknown
	}
	expires := time.Duration(result.ExpiresIn) * time.Second
	if expires <= 0 {
		expires = time.Hour
	}
	until := now.Add(expires - s.refreshSkew)
	if !until.After(now) {
		until = now.Add(expires / 2)
	}
	s.mu.Lock()
	s.accessToken = result.AccessToken
	s.tokenUntil = until
	s.mu.Unlock()
	return result.AccessToken, nil
}

func (s *SubscribeSender) invalidateToken(token string) {
	s.mu.Lock()
	if s.accessToken == token {
		s.accessToken = ""
		s.tokenUntil = time.Time{}
	}
	s.mu.Unlock()
}

func validateSubscribeMessage(message SubscribeMessage) error {
	if !validField(message.OpenID, 128) || strings.IndexFunc(message.OpenID, func(r rune) bool { return r == '\r' || r == '\n' || r == ' ' || r == '\t' }) >= 0 {
		return errors.New("invalid recipient")
	}
	if !validField(message.TemplateID, 128) {
		return errors.New("invalid template")
	}
	if utf8.RuneCountInString(message.Page) > 512 || strings.IndexByte(message.Page, '\n') >= 0 || strings.IndexByte(message.Page, '\r') >= 0 {
		return errors.New("invalid page")
	}
	if len(message.Data) > 20 {
		return errors.New("too many template fields")
	}
	for key, value := range message.Data {
		if !validField(key, 32) || !validField(value.Value, 256) {
			return errors.New("invalid template field")
		}
	}
	return nil
}

func validField(value string, max int) bool {
	value = strings.TrimSpace(value)
	return value != "" && utf8.RuneCountInString(value) <= max && !strings.ContainsAny(value, "\r\n")
}

func decodeLimited(reader io.Reader, target any) error {
	decoder := json.NewDecoder(io.LimitReader(reader, 32<<10))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func mapWechatError(code int64) string {
	switch code {
	case 40001, 40014, 42001:
		return "token_invalid"
	case 40003:
		return "openid_invalid"
	case 40037, 40039, 41028, 41029:
		return "template_invalid"
	case 43101:
		return "subscription_rejected"
	case 45009, 45011:
		return "rate_limited"
	default:
		return fmt.Sprintf("wechat_error_%d", code)
	}
}

func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func providerError(status, code string) error {
	switch status {
	case SendStatusBlocked:
		return fmt.Errorf("%w: %s", ErrSubscribeBlocked, code)
	case SendStatusUnknown:
		return fmt.Errorf("%w: %s", ErrSubscribeUnknown, code)
	default:
		return fmt.Errorf("%w: %s", ErrSubscribeRejected, code)
	}
}

func errStatus(err error) string {
	switch {
	case errors.Is(err, ErrSubscribeBlocked):
		return SendStatusBlocked
	case errors.Is(err, ErrSubscribeRejected):
		return SendStatusRejected
	default:
		return SendStatusUnknown
	}
}

func errorCode(err error) string {
	message := err.Error()
	for _, code := range []string{"configuration_missing", "token_invalid", "openid_invalid", "template_invalid", "subscription_rejected", "rate_limited", "timeout", "transport", "request", "invalid_response"} {
		if strings.Contains(message, code) {
			return code
		}
	}
	return "provider_error"
}
