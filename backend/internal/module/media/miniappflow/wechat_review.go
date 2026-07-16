package miniappflow

import (
	"bytes"
	"context"
	"crypto/sha1" // #nosec G505 -- WeChat callback signature protocol mandates SHA-1.
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
)

const WechatReviewURLTTL = 35 * time.Minute

const (
	wechatMediaDownloadFailureCode = "wechat_media_download_error"
	wechatMediaPlatformFailureCode = "wechat_media_platform_error"
)

type WechatReviewSubmitInput struct {
	OpenID         string
	AssetPublicID  string
	Purpose        coremedia.Purpose
	FinalKey       string
	FinalVersionID string
}

type WechatReviewSubmission struct {
	TraceID      string
	ReviewTaskID uint64
}

type WechatReviewSubmitter interface {
	SubmitMediaReview(context.Context, WechatReviewSubmitInput) (WechatReviewSubmission, error)
}

type WechatReviewURLSigner interface {
	SignedGetURL(context.Context, string, string, time.Duration) (string, error)
}

type WechatReviewSubmitterConfig struct {
	AppID     string
	AppSecret string
	APIBase   string
	URLTTL    time.Duration
	Client    *http.Client
}

type WechatAsyncReviewSubmitter struct {
	appID, appSecret string
	apiBase          string
	urlTTL           time.Duration
	client           *http.Client
	signer           WechatReviewURLSigner
	mu               sync.Mutex
	token            string
	tokenExpiresAt   time.Time
	now              func() time.Time
}

func NewWechatAsyncReviewSubmitter(config WechatReviewSubmitterConfig, signer WechatReviewURLSigner) (*WechatAsyncReviewSubmitter, error) {
	config.AppID = strings.TrimSpace(config.AppID)
	config.AppSecret = strings.TrimSpace(config.AppSecret)
	config.APIBase = strings.TrimRight(strings.TrimSpace(config.APIBase), "/")
	if config.AppID == "" || config.AppSecret == "" || signer == nil {
		return nil, ErrInvalidRequest
	}
	if config.APIBase == "" {
		config.APIBase = "https://api.weixin.qq.com"
	}
	if config.URLTTL <= 0 {
		config.URLTTL = WechatReviewURLTTL
	}
	if config.Client == nil {
		config.Client = &http.Client{Timeout: 10 * time.Second}
	}
	return &WechatAsyncReviewSubmitter{
		appID: config.AppID, appSecret: config.AppSecret, apiBase: config.APIBase,
		urlTTL: config.URLTTL, client: config.Client, signer: signer,
		now: func() time.Time { return time.Now().UTC() },
	}, nil
}

func (s *WechatAsyncReviewSubmitter) SubmitMediaReview(ctx context.Context, input WechatReviewSubmitInput) (WechatReviewSubmission, error) {
	openID := strings.TrimSpace(input.OpenID)
	if openID == "" || !coremedia.ValidULID(input.AssetPublicID) || strings.TrimSpace(input.FinalKey) == "" ||
		strings.TrimSpace(input.FinalVersionID) == "" || !input.Purpose.Valid() {
		return WechatReviewSubmission{}, ErrInvalidRequest
	}
	mediaURL, err := s.signer.SignedGetURL(ctx, input.FinalKey, input.FinalVersionID, s.urlTTL)
	if err != nil {
		return WechatReviewSubmission{}, err
	}
	token, err := s.accessToken(ctx)
	if err != nil {
		return WechatReviewSubmission{}, err
	}
	payload, err := json.Marshal(map[string]any{
		"media_url":  mediaURL,
		"media_type": 2,
		"version":    2,
		"scene":      2,
		"openid":     openID,
	})
	if err != nil {
		return WechatReviewSubmission{}, err
	}
	endpoint := s.apiBase + "/wxa/media_check_async?access_token=" + url.QueryEscape(token)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return WechatReviewSubmission{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return WechatReviewSubmission{}, err
	}
	defer response.Body.Close()
	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		TraceID string `json:"trace_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return WechatReviewSubmission{}, err
	}
	if result.ErrCode != 0 || strings.TrimSpace(result.TraceID) == "" {
		return WechatReviewSubmission{}, fmt.Errorf("wechat media check %d: %s", result.ErrCode, result.ErrMsg)
	}
	reviewTaskID, err := WechatReviewTaskID(result.TraceID)
	if err != nil {
		return WechatReviewSubmission{}, err
	}
	return WechatReviewSubmission{TraceID: strings.TrimSpace(result.TraceID), ReviewTaskID: reviewTaskID}, nil
}

func (s *WechatAsyncReviewSubmitter) accessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.token != "" && s.now().Before(s.tokenExpiresAt) {
		return s.token, nil
	}
	endpoint := s.apiBase + "/cgi-bin/token?grant_type=client_credential&appid=" + url.QueryEscape(s.appID) + "&secret=" + url.QueryEscape(s.appSecret)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	response, err := s.client.Do(request)
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
	if strings.TrimSpace(result.AccessToken) == "" {
		return "", fmt.Errorf("wechat token %d: %s", result.ErrCode, result.ErrMsg)
	}
	ttl := time.Duration(result.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = 2 * time.Hour
	}
	if ttl > 5*time.Minute {
		ttl -= 5 * time.Minute
	}
	s.token = strings.TrimSpace(result.AccessToken)
	s.tokenExpiresAt = s.now().Add(ttl)
	return s.token, nil
}

func WechatReviewTaskID(traceID string) (uint64, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return 0, ErrInvalidRequest
	}
	sum := sha256.Sum256([]byte(traceID))
	value := binary.BigEndian.Uint64(sum[:8])
	if value == 0 {
		value = binary.BigEndian.Uint64(sum[8:16])
	}
	if value == 0 {
		return 0, ErrInvalidRequest
	}
	return value, nil
}

func WechatDecisionPayloadHash(traceID, suggest string, label int, reviewTaskID, decisionVersion uint64, decision coremedia.ProjectionStatus) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("wechat:%s:%s:%d:%d:%d:%s",
		strings.TrimSpace(traceID), strings.ToLower(strings.TrimSpace(suggest)), label, reviewTaskID, decisionVersion, decision,
	)))
	return hex.EncodeToString(sum[:])
}

func validWechatMediaFailureCode(value string) bool {
	switch value {
	case wechatMediaDownloadFailureCode, wechatMediaPlatformFailureCode:
		return true
	default:
		return false
	}
}

func WechatFailurePayloadHash(traceID, failureCode string, reviewTaskID, decisionVersion uint64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf(
		"wechat-failure:%s:%s:%d:%d:%s",
		strings.TrimSpace(traceID), failureCode, reviewTaskID, decisionVersion,
		coremedia.ProjectionReviewFailed,
	)))
	return hex.EncodeToString(sum[:])
}

func WechatSuggestDecision(suggest string) (coremedia.ProjectionStatus, string, error) {
	switch strings.ToLower(strings.TrimSpace(suggest)) {
	case "pass":
		return coremedia.ProjectionApproved, "", nil
	case "risky", "review":
		return coremedia.ProjectionRejected, "", nil
	case "":
		return "", "", ErrInvalidRequest
	default:
		return coremedia.ProjectionReviewFailed, "wechat_unknown_suggest", nil
	}
}

func VerifyWechatReviewSignature(token, timestamp, nonce, signature string) bool {
	if strings.TrimSpace(token) == "" || strings.TrimSpace(timestamp) == "" ||
		strings.TrimSpace(nonce) == "" || strings.TrimSpace(signature) == "" {
		return false
	}
	values := []string{token, timestamp, nonce}
	sort.Strings(values)
	sum := sha1.Sum([]byte(strings.Join(values, "")))
	return hex.EncodeToString(sum[:]) == strings.ToLower(strings.TrimSpace(signature))
}

var _ WechatReviewSubmitter = (*WechatAsyncReviewSubmitter)(nil)
