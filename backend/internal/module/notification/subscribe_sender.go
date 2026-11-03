package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
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
	appID            string
	appSecret        string
	apiBase          string
	miniprogramState string
	enabled          bool
	client           *http.Client
	now              func() time.Time
	refreshSkew      time.Duration

	mu          sync.Mutex
	accessToken string
	tokenUntil  time.Time
}

type subscribeSenderConfig struct {
	AppID            string
	AppSecret        string
	APIBase          string
	MiniprogramState string
	Enabled          bool
	HTTPClient       *http.Client
	Now              func() time.Time
	RefreshSkew      time.Duration
}

func NewSubscribeSender(cfg config.Config, clients ...*http.Client) (*SubscribeSender, error) {
	var client *http.Client
	if len(clients) > 0 {
		client = clients[0]
	}
	if cfg.WechatAPIBase != "" && cfg.WechatAPIBase != "https://api.weixin.qq.com" {
		return nil, errors.New("WeChat subscribe sender requires official HTTPS API base")
	}
	return newSubscribeSender(subscribeSenderConfig{
		AppID:            cfg.WechatAppID,
		AppSecret:        cfg.WechatAppSecret,
		APIBase:          cfg.WechatAPIBase,
		MiniprogramState: cfg.WechatSubscribeMiniprogramState,
		Enabled:          cfg.EnableWechatSubscribeNotifications && cfg.EnableNotificationSending,
		HTTPClient:       client,
		RefreshSkew:      5 * time.Minute,
	})
}

func newSubscribeSender(cfg subscribeSenderConfig) (*SubscribeSender, error) {
	if cfg.MiniprogramState == "" {
		cfg.MiniprogramState = "formal"
	}
	switch cfg.MiniprogramState {
	case "developer", "trial", "formal":
	default:
		return nil, errors.New("WeChat subscribe miniprogram state must be developer, trial or formal")
	}
	base := strings.TrimRight(strings.TrimSpace(cfg.APIBase), "/")
	if base == "" {
		base = "https://api.weixin.qq.com"
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.User != nil || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || !((parsed.Scheme == "https" && parsed.Host == "api.weixin.qq.com") || (parsed.Scheme == "http" && net.ParseIP(parsed.Hostname()) != nil && net.ParseIP(parsed.Hostname()).IsLoopback())) {
		return nil, errors.New("invalid WeChat API base URL")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 8 * time.Second}
	}
	// Never follow redirects: the access token is carried in the query string.
	// A redirecting provider must not receive it at another host.
	copyClient := *cfg.HTTPClient
	copyClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	cfg.HTTPClient = &copyClient
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	if cfg.RefreshSkew <= 0 {
		cfg.RefreshSkew = 5 * time.Minute
	}
	return &SubscribeSender{
		appID: cfg.AppID, appSecret: cfg.AppSecret, apiBase: base,
		miniprogramState: cfg.MiniprogramState,
		enabled:          cfg.Enabled,
		client:           cfg.HTTPClient, now: cfg.Now, refreshSkew: cfg.RefreshSkew,
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
	// The server owns release targeting; callers and persisted templates cannot
	// override it. Explicit formal preserves the previous WeChat default.
	body, err := json.Marshal(struct {
		SubscribeMessage
		MiniprogramState string `json:"miniprogram_state"`
		Lang             string `json:"lang"`
	}{SubscribeMessage: message, MiniprogramState: s.miniprogramState, Lang: "zh_CN"})
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
	result, present, err := decodeWechatResponse(response.Body)
	if err != nil || !present {
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

		return SubscribeSendResult{Status: status, ErrorCode: mapped}, providerError(status, mapped)
	}
	return SubscribeSendResult{Status: SendStatusAccepted, ProviderMessageID: result.MsgID}, nil
}

type wechatResponse struct {
	ErrCode     int64  `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	MsgID       string `json:"msgid"`
	AccessToken string `json:"access_token"`
}

func (s *SubscribeSender) token(ctx context.Context) (string, error) {
	now := s.now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.accessToken != "" && now.Before(s.tokenUntil) {
		token := s.accessToken
		return token, nil
	}

	endpoint := s.apiBase + "/cgi-bin/token?grant_type=client_credential&appid=" + url.QueryEscape(s.appID) + "&secret=" + url.QueryEscape(s.appSecret)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("%w: token_request", ErrSubscribeBlocked)
	}
	response, err := s.client.Do(request)
	if err != nil {
		if isTimeout(err) || errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("%w: token_timeout", ErrSubscribeBlocked)
		}
		return "", fmt.Errorf("%w: token_transport", ErrSubscribeBlocked)
	}
	defer response.Body.Close()
	var raw map[string]json.RawMessage
	if err := decodeLimited(response.Body, &raw); err != nil {
		return "", fmt.Errorf("%w: token_invalid_response", ErrSubscribeBlocked)
	}
	var result wechatResponse
	if value, ok := raw["errcode"]; ok {
		if string(value) == "null" || json.Unmarshal(value, &result.ErrCode) != nil {
			return "", fmt.Errorf("%w: token_invalid_response", ErrSubscribeBlocked)
		}
	}
	_ = json.Unmarshal(raw["errmsg"], &result.ErrMsg)
	_ = json.Unmarshal(raw["access_token"], &result.AccessToken)
	var expiresIn int64
	if value, ok := raw["expires_in"]; ok {
		_ = json.Unmarshal(value, &expiresIn)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || result.ErrCode != 0 || result.AccessToken == "" {
		if result.ErrCode != 0 {
			mapped := mapWechatError(result.ErrCode)
			return "", fmt.Errorf("%w: %s", ErrSubscribeBlocked, mapped)
		}
		return "", fmt.Errorf("%w: token_unavailable", ErrSubscribeBlocked)
	}
	if expiresIn <= 0 || expiresIn > math.MaxInt64/int64(time.Second) || expiresIn > 7*24*60*60 {
		return "", fmt.Errorf("%w: token_expiry_invalid", ErrSubscribeBlocked)
	}
	expires := time.Duration(expiresIn) * time.Second
	until := now.Add(expires - s.refreshSkew)
	if !until.After(now) {
		until = now.Add(expires / 2)
	}
	s.accessToken = result.AccessToken
	s.tokenUntil = until
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
	if message.Page != "" && (!strings.HasPrefix(message.Page, "/") || strings.HasPrefix(message.Page, "//") || utf8.RuneCountInString(message.Page) > 512 || strings.IndexByte(message.Page, '\n') >= 0 || strings.IndexByte(message.Page, '\r') >= 0) {
		return errors.New("invalid page")
	}
	if len(message.Data) == 0 || len(message.Data) > 20 {
		return errors.New("too many template fields")
	}
	for key, value := range message.Data {
		if !validField(key, 32) || !validTemplateValue(key, value.Value) {
			return errors.New("invalid template field")
		}
	}
	return nil
}

func validTemplateValue(key, value string) bool {
	max := 32
	switch {
	case strings.HasPrefix(key, "thing"):
		max = 20
	case strings.HasPrefix(key, "name"):
		max = 10
	case strings.HasPrefix(key, "phrase"):
		max = 5
	case strings.HasPrefix(key, "time"), strings.HasPrefix(key, "date"):
		max = 20
	case strings.HasPrefix(key, "phone_number"):
		max = 20
	case strings.HasPrefix(key, "character_string"):
		max = 32
	}
	return validField(value, max)
}

func validField(value string, max int) bool {
	value = strings.TrimSpace(value)
	return value != "" && utf8.RuneCountInString(value) <= max && !strings.ContainsAny(value, "\r\n")
}

func decodeLimited(reader io.Reader, target any) error {
	decoder := json.NewDecoder(io.LimitReader(reader, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("invalid trailing JSON")
	}
	return nil
}

func decodeWechatResponse(reader io.Reader) (wechatResponse, bool, error) {
	var raw map[string]json.RawMessage
	if err := decodeLimited(reader, &raw); err != nil {
		return wechatResponse{}, false, err
	}
	value, present := raw["errcode"]
	if !present {
		return wechatResponse{}, false, nil
	}
	var result wechatResponse
	if err := json.Unmarshal(value, &result.ErrCode); err != nil || string(value) == "null" {
		return wechatResponse{}, true, errors.New("invalid error code")
	}
	_ = json.Unmarshal(raw["errmsg"], &result.ErrMsg)
	_ = json.Unmarshal(raw["msgid"], &result.MsgID)
	return result, true, nil
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

// ValidateTemplateSnapshot validates the immutable server template shape used
// by workers, without recipient secrets. It accepts no extra JSON fields.
func ValidateTemplateSnapshot(raw json.RawMessage) error {
	var snapshot struct {
		TemplateID string                            `json:"templateId"`
		Page       string                            `json:"page"`
		Data       map[string]SubscribeTemplateValue `json:"data"`
	}
	if err := decodeLimited(bytes.NewReader(raw), &snapshot); err != nil {
		return ErrSubscribePayload
	}
	return validateSubscribeMessage(SubscribeMessage{OpenID: "validated-server-recipient", TemplateID: snapshot.TemplateID, Page: snapshot.Page, Data: snapshot.Data})
}
