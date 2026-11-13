package notification

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
)

var (
	ErrWecomConfiguration = errors.New("invalid WeCom group webhook configuration")
	ErrWecomPayload       = errors.New("invalid WeCom group payload")
)

type WecomSender interface {
	Enabled(string) bool
	Send(context.Context, WecomGroupSnapshot) (SendStatus, error)
}

type WeComGroupSender struct {
	urls   map[string]string
	client *http.Client
}

func NewWeComGroupSender(urls map[string]string) (*WeComGroupSender, error) {
	normalized := make(map[string]string, len(urls))
	for group, raw := range urls {
		if _, ok := wecomGroupRecipientID(group); !ok {
			return nil, ErrWecomConfiguration
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "qyapi.weixin.qq.com" ||
			parsed.Path != "/cgi-bin/webhook/send" || parsed.Query().Get("key") == "" ||
			parsed.User != nil || parsed.Fragment != "" {
			return nil, ErrWecomConfiguration
		}
		normalized[group] = raw
	}
	return &WeComGroupSender{urls: normalized, client: &http.Client{Timeout: 8 * time.Second}}, nil
}

func newWeComGroupSenderForTest(urls map[string]string, client *http.Client) *WeComGroupSender {
	return &WeComGroupSender{urls: urls, client: client}
}

func (s *WeComGroupSender) Enabled(group string) bool {
	return s != nil && s.client != nil && s.urls[group] != ""
}

func (s *WeComGroupSender) Send(ctx context.Context, message WecomGroupSnapshot) (SendStatus, error) {
	if s == nil || !s.Enabled(message.GroupKey) {
		return SendStatusBlocked, ErrWecomConfiguration
	}
	if message.Version != 1 || message.AggregatePublicID == "" || message.AggregateVersion == 0 ||
		message.OccurredAt.IsZero() || !validWecomGroupMessage(WecomGroupMessage{
		GroupKey: message.GroupKey, Title: message.Title, Summary: message.Summary,
	}) {
		return SendStatusBlocked, ErrWecomPayload
	}
	content := fmt.Sprintf(
		"### %s\n> 订单：%s\n> 状态：%s\n> 时间：%s",
		message.Title,
		message.AggregatePublicID,
		message.Summary,
		message.OccurredAt.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02 15:04"),
	)
	body, err := json.Marshal(map[string]any{
		"msgtype":  "markdown",
		"markdown": map[string]string{"content": content},
	})
	if err != nil {
		return SendStatusBlocked, ErrWecomPayload
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.urls[message.GroupKey], bytes.NewReader(body))
	if err != nil {
		return SendStatusBlocked, ErrWecomConfiguration
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return SendStatusUnknown, ErrWecomPayload
	}
	defer response.Body.Close()
	var result struct {
		ErrCode int `json:"errcode"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return SendStatusUnknown, ErrWecomPayload
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 && result.ErrCode == 0 {
		return SendStatusAccepted, nil
	}
	if response.StatusCode == http.StatusTooManyRequests || result.ErrCode == 45009 {
		return SendStatusRejected, ErrWecomPayload
	}
	return SendStatusRejected, ErrWecomPayload
}
