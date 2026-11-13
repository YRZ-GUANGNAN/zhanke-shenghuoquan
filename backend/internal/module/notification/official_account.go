package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/pkg/response"
)

var (
	ErrOfficialAccountUnavailable = errors.New("official account notifications unavailable")
	ErrOfficialAccountNotBound    = errors.New("official account is not bound")
	ErrOfficialAccountState       = errors.New("invalid official account oauth state")
)

type OfficialAccountBindingRepository interface {
	FindOfficialAccountBinding(context.Context, uint64, string) (OfficialAccountBinding, error)
	UpsertOfficialAccountBinding(context.Context, OfficialAccountBinding, time.Time) (OfficialAccountBinding, error)
	UpdateOfficialAccountFollowState(context.Context, uint64, string, string, time.Time) error
	UnbindOfficialAccount(context.Context, uint64, string) error
	CountOfficialAccountBindings(context.Context, string, string) (int64, error)
}

type OfficialAccountConfig struct {
	Enabled     bool
	AppID       string
	AppSecret   string
	APIBase     string
	RedirectURL string
	StateSecret string
	StateTTL    time.Duration
	HTTPClient  *http.Client
}

type OfficialAccountService struct {
	repository OfficialAccountBindingRepository
	config     OfficialAccountConfig
	now        func() time.Time
}

func NewOfficialAccountService(repository OfficialAccountBindingRepository, config OfficialAccountConfig) (*OfficialAccountService, error) {
	config.AppID = strings.TrimSpace(config.AppID)
	config.AppSecret = strings.TrimSpace(config.AppSecret)
	config.APIBase = strings.TrimRight(strings.TrimSpace(config.APIBase), "/")
	config.RedirectURL = strings.TrimSpace(config.RedirectURL)
	config.StateSecret = strings.TrimSpace(config.StateSecret)
	if config.StateTTL <= 0 {
		config.StateTTL = 10 * time.Minute
	}
	if config.HTTPClient == nil {
		config.HTTPClient = &http.Client{Timeout: 8 * time.Second}
	}
	service := &OfficialAccountService{repository: repository, config: config, now: func() time.Time { return time.Now().UTC() }}
	if !config.Enabled {
		return service, nil
	}
	apiBase, err := url.Parse(config.APIBase)
	if err != nil || config.AppID == "" || config.AppSecret == "" || len(config.StateSecret) < 32 ||
		!validHTTPSOrLoopback(apiBase) {
		return nil, ErrOfficialAccountUnavailable
	}
	redirect, err := url.Parse(config.RedirectURL)
	if err != nil || redirect.Scheme != "https" || redirect.Host == "" || redirect.User != nil ||
		redirect.RawQuery != "" || redirect.Fragment != "" {
		return nil, ErrOfficialAccountUnavailable
	}
	return service, nil
}

func validHTTPSOrLoopback(value *url.URL) bool {
	if value == nil || value.User != nil || value.Path != "" || value.RawQuery != "" || value.Fragment != "" {
		return false
	}
	if value.Scheme == "https" && value.Host == "api.weixin.qq.com" {
		return true
	}
	return value.Scheme == "http" && (value.Hostname() == "127.0.0.1" || value.Hostname() == "localhost")
}

func (s *OfficialAccountService) Enabled() bool {
	return s != nil && s.config.Enabled && s.repository != nil
}

type OfficialAccountStatus struct {
	Enabled     bool   `json:"enabled"`
	Bound       bool   `json:"bound"`
	FollowState string `json:"followState,omitempty"`
}

func (s *OfficialAccountService) Status(ctx context.Context, userID uint64) (OfficialAccountStatus, error) {
	if userID == 0 {
		return OfficialAccountStatus{}, ErrInvalidRecord
	}
	if !s.Enabled() {
		return OfficialAccountStatus{Enabled: false, Bound: false}, nil
	}
	binding, err := s.repository.FindOfficialAccountBinding(ctx, userID, s.config.AppID)
	if errors.Is(err, ErrOfficialAccountNotBound) {
		return OfficialAccountStatus{Enabled: true, Bound: false}, nil
	}
	if err != nil {
		return OfficialAccountStatus{}, err
	}
	return OfficialAccountStatus{Enabled: true, Bound: true, FollowState: binding.FollowState}, nil
}

func (s *OfficialAccountService) OAuthURL(userID uint64) (string, error) {
	if !s.Enabled() || userID == 0 {
		return "", ErrOfficialAccountUnavailable
	}
	state := s.signState(userID, s.now().Add(s.config.StateTTL))
	query := url.Values{}
	query.Set("appid", s.config.AppID)
	query.Set("redirect_uri", s.config.RedirectURL)
	query.Set("response_type", "code")
	query.Set("scope", "snsapi_base")
	query.Set("state", state)
	return "https://open.weixin.qq.com/connect/oauth2/authorize?" + query.Encode() + "#wechat_redirect", nil
}

func (s *OfficialAccountService) CompleteBinding(ctx context.Context, code, state string) (uint64, error) {
	if !s.Enabled() {
		return 0, ErrOfficialAccountUnavailable
	}
	userID, err := s.verifyState(state)
	if err != nil || strings.TrimSpace(code) == "" {
		return 0, ErrOfficialAccountState
	}
	query := url.Values{}
	query.Set("appid", s.config.AppID)
	query.Set("secret", s.config.AppSecret)
	query.Set("code", strings.TrimSpace(code))
	query.Set("grant_type", "authorization_code")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.config.APIBase+"/sns/oauth2/access_token?"+query.Encode(), nil)
	if err != nil {
		return 0, ErrOfficialAccountUnavailable
	}
	httpResponse, err := s.config.HTTPClient.Do(request)
	if err != nil {
		return 0, ErrOfficialAccountUnavailable
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return 0, ErrOfficialAccountUnavailable
	}
	var result struct {
		OpenID  string `json:"openid"`
		ErrCode int    `json:"errcode"`
	}
	if json.NewDecoder(io.LimitReader(httpResponse.Body, 64<<10)).Decode(&result) != nil || result.ErrCode != 0 || result.OpenID == "" {
		return 0, ErrOfficialAccountUnavailable
	}
	now := s.now()
	binding := OfficialAccountBinding{UserID: userID, AppID: s.config.AppID, OpenID: result.OpenID,
		FollowState: "unknown", RowVersion: 1, BoundAt: now, UpdatedAt: now}
	if _, err := s.repository.UpsertOfficialAccountBinding(ctx, binding, now); err != nil {
		return 0, err
	}
	return userID, nil
}

func (s *OfficialAccountService) Unbind(ctx context.Context, userID uint64) error {
	if !s.Enabled() || userID == 0 {
		return ErrOfficialAccountUnavailable
	}
	return s.repository.UnbindOfficialAccount(ctx, userID, s.config.AppID)
}

func (s *OfficialAccountService) LookupOpenID(ctx context.Context, userID uint64) (string, error) {
	if !s.Enabled() {
		return "", ErrOfficialAccountUnavailable
	}
	binding, err := s.repository.FindOfficialAccountBinding(ctx, userID, s.config.AppID)
	if err != nil {
		return "", err
	}
	return binding.OpenID, nil
}

func (s *OfficialAccountService) MarkFollowState(ctx context.Context, userID uint64, state string) error {
	if !s.Enabled() {
		return ErrOfficialAccountUnavailable
	}
	return s.repository.UpdateOfficialAccountFollowState(ctx, userID, s.config.AppID, state, s.now())
}

func (s *OfficialAccountService) CountBindings(ctx context.Context, followState string) (int64, error) {
	if !s.Enabled() {
		return 0, nil
	}
	return s.repository.CountOfficialAccountBindings(ctx, s.config.AppID, followState)
}

func (s *OfficialAccountService) signState(userID uint64, expires time.Time) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatUint(userID, 10) + ":" + strconv.FormatInt(expires.Unix(), 10)))
	mac := hmac.New(sha256.New, []byte(s.config.StateSecret))
	_, _ = mac.Write([]byte(payload))
	return payload + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *OfficialAccountService) verifyState(state string) (uint64, error) {
	payload, signature, ok := strings.Cut(strings.TrimSpace(state), ".")
	if !ok || payload == "" || signature == "" {
		return 0, ErrOfficialAccountState
	}
	decodedSignature, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return 0, ErrOfficialAccountState
	}
	mac := hmac.New(sha256.New, []byte(s.config.StateSecret))
	_, _ = mac.Write([]byte(payload))
	if !hmac.Equal(decodedSignature, mac.Sum(nil)) {
		return 0, ErrOfficialAccountState
	}
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return 0, ErrOfficialAccountState
	}
	userText, expiryText, ok := strings.Cut(string(decoded), ":")
	if !ok {
		return 0, ErrOfficialAccountState
	}
	userID, userErr := strconv.ParseUint(userText, 10, 64)
	expiry, expiryErr := strconv.ParseInt(expiryText, 10, 64)
	if userErr != nil || expiryErr != nil || userID == 0 || !time.Unix(expiry, 0).After(s.now()) {
		return 0, ErrOfficialAccountState
	}
	return userID, nil
}

type OfficialAccountHandler struct {
	service       *OfficialAccountService
	currentUserID func(*gin.Context) uint64
}

func NewOfficialAccountHandler(service *OfficialAccountService, currentUserID func(*gin.Context) uint64) *OfficialAccountHandler {
	return &OfficialAccountHandler{service: service, currentUserID: currentUserID}
}

func (h *OfficialAccountHandler) Register(api *gin.RouterGroup, authentication gin.HandlerFunc) {
	if h == nil || h.service == nil {
		return
	}
	group := api.Group("/notification/official-account")
	group.GET("/callback", h.callback)
	authed := group.Group("", authentication)
	authed.GET("/status", h.status)
	authed.POST("/oauth-url", h.oauthURL)
	authed.DELETE("/binding", h.unbind)
}

func (h *OfficialAccountHandler) status(c *gin.Context) {
	value, err := h.service.Status(c.Request.Context(), h.currentUserID(c))
	if err != nil {
		response.Fail(c, http.StatusServiceUnavailable, "official account status unavailable")
		return
	}
	response.OK(c, value)
}

func (h *OfficialAccountHandler) oauthURL(c *gin.Context) {
	value, err := h.service.OAuthURL(h.currentUserID(c))
	if err != nil {
		response.Fail(c, http.StatusServiceUnavailable, "official account binding unavailable")
		return
	}
	response.OK(c, gin.H{"url": value})
}

func (h *OfficialAccountHandler) callback(c *gin.Context) {
	if _, err := h.service.CompleteBinding(c.Request.Context(), c.Query("code"), c.Query("state")); err != nil {
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusBadRequest, "<!doctype html><meta charset=utf-8><title>绑定失败</title><p>绑定链接已失效，请返回小程序重新发起。</p>")
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, "<!doctype html><meta charset=utf-8><title>绑定成功</title><p>公众号通知已绑定，请返回小程序。</p>")
}

func (h *OfficialAccountHandler) unbind(c *gin.Context) {
	if err := h.service.Unbind(c.Request.Context(), h.currentUserID(c)); err != nil {
		response.Fail(c, http.StatusServiceUnavailable, "official account unbind unavailable")
		return
	}
	response.OK(c, gin.H{"bound": false})
}
