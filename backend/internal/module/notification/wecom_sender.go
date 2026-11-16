package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
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
	mu     sync.RWMutex
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
		if err := ValidateWeComGroupURL(raw); err != nil {
			return nil, err
		}
		normalized[group] = raw
	}
	return &WeComGroupSender{urls: normalized, client: &http.Client{Timeout: 8 * time.Second}}, nil
}

// ValidateWeComGroupURL enforces the exact WeCom group robot webhook shape.
func ValidateWeComGroupURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "qyapi.weixin.qq.com" ||
		parsed.Path != "/cgi-bin/webhook/send" || parsed.Query().Get("key") == "" ||
		parsed.User != nil || parsed.Fragment != "" {
		return ErrWecomConfiguration
	}
	return nil
}

// SetGroupURL updates a single group webhook at runtime; empty disables it.
func (s *WeComGroupSender) SetGroupURL(group, raw string) error {
	if _, ok := wecomGroupRecipientID(group); !ok {
		return ErrWecomConfiguration
	}
	raw = strings.TrimSpace(raw)
	if err := ValidateWeComGroupURL(raw); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.urls == nil {
		s.urls = make(map[string]string, 2)
	}
	if raw == "" {
		delete(s.urls, group)
	} else {
		s.urls[group] = raw
	}
	return nil
}

// GroupURL returns the configured webhook for a group (empty when disabled).
func (s *WeComGroupSender) GroupURL(group string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.urls[group]
}

func newWeComGroupSenderForTest(urls map[string]string, client *http.Client) *WeComGroupSender {
	return &WeComGroupSender{urls: urls, client: client}
}

func (s *WeComGroupSender) Enabled(group string) bool {
	if s == nil || s.client == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.urls[group] != ""
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
	var body []byte
	var err error
	if message.Card != nil {
		body, err = marshalWecomTemplateCard(*message.Card, message.OccurredAt)
	} else {
		content := fmt.Sprintf(
			"### %s\n> 订单：%s\n> 状态：%s\n> 时间：%s",
			message.Title,
			message.AggregatePublicID,
			message.Summary,
			message.OccurredAt.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02 15:04"),
		)
		body, err = json.Marshal(map[string]any{
			"msgtype":  "markdown",
			"markdown": map[string]string{"content": content},
		})
	}
	if err != nil {
		return SendStatusBlocked, ErrWecomPayload
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.GroupURL(message.GroupKey), bytes.NewReader(body))
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

var (
	wecomMiniAppIDPattern       = regexp.MustCompile(`^wx[0-9A-Za-z]{16}$`)
	wecomMiniAppPagePathPattern = regexp.MustCompile(`^pages/[A-Za-z0-9_-]+/index(\?[A-Za-z0-9_=&%.\-]*)?$`)
)

// marshalWecomTemplateCard builds a text_notice template card payload. Lengths
// are clamped to the values accepted by the group robot API.
func marshalWecomTemplateCard(card WecomTemplateCard, occurredAt time.Time) ([]byte, error) {
	clamp := func(value string, limit int) string {
		runes := []rune(strings.TrimSpace(value))
		if len(runes) > limit {
			return string(runes[:limit]) + "…"
		}
		return string(runes)
	}
	card.SourceDesc = clamp(card.SourceDesc, 20)
	card.Title = clamp(card.Title, 26)
	card.Desc = clamp(card.Desc, 40)
	card.EmphasisTitle = clamp(card.EmphasisTitle, 16)
	card.EmphasisDesc = clamp(card.EmphasisDesc, 8)
	items := make([]map[string]string, 0, len(card.Items))
	for index, item := range card.Items {
		if index >= 6 {
			break
		}
		items = append(items, map[string]string{
			"keyname": clamp(item.Key, 12),
			"value":   clamp(item.Value, 30),
		})
	}
	if len(items) == 0 {
		items = append(items, map[string]string{
			"keyname": "时间",
			"value":   occurredAt.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02 15:04"),
		})
	}
	actionURL := strings.TrimSpace(card.ActionURL)
	if actionURL == "" {
		actionURL = "https://zhanduiwang.top/"
	}
	cardAction := map[string]any{"type": 1, "url": actionURL}
	// 小程序跳转：appid+pagepath 合法时优先，桌面端用 URL 兜底。
	if appID := strings.TrimSpace(card.ActionAppID); wecomMiniAppIDPattern.MatchString(appID) {
		if pagePath := strings.TrimSpace(card.ActionPagePath); wecomMiniAppPagePathPattern.MatchString(pagePath) {
			cardAction = map[string]any{
				"type":     2,
				"url":      actionURL,
				"appid":    appID,
				"pagepath": pagePath,
			}
		}
	}
	template := map[string]any{
		"card_type": "text_notice",
		"source": map[string]any{
			"desc":       card.SourceDesc,
			"desc_color": 0,
		},
		"main_title": map[string]any{
			"title": card.Title,
			"desc":  card.Desc,
		},
		"horizontal_content_list": items,
		"card_action":             cardAction,
	}
	if card.EmphasisTitle != "" {
		template["emphasis_content"] = map[string]any{
			"title": card.EmphasisTitle,
			"desc":  card.EmphasisDesc,
		}
	}
	return json.Marshal(map[string]any{
		"msgtype":       "template_card",
		"template_card": template,
	})
}
