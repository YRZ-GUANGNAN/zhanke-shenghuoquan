package deliverynotify

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

	"campus-circle/backend/internal/module/delivery"
)

var (
	ErrWebhookConfiguration = errors.New("invalid rider webhook configuration")
	ErrWebhookRejected      = errors.New("rider webhook rejected request")
	ErrWebhookPayload       = errors.New("invalid rider webhook payload")
)

type Sender interface {
	Enabled() bool
	Send(context.Context, delivery.OutboxEvent, delivery.RiderPoolOpenedPayload) error
}

type WeComSender struct {
	webhookURL string
	client     *http.Client
}

func NewWeComSender(webhookURL string) (*WeComSender, error) {
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return &WeComSender{client: &http.Client{Timeout: 5 * time.Second}}, nil
	}
	parsed, err := url.Parse(webhookURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() != "qyapi.weixin.qq.com" ||
		parsed.Path != "/cgi-bin/webhook/send" || parsed.Query().Get("key") == "" {
		return nil, ErrWebhookConfiguration
	}
	return &WeComSender{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 5 * time.Second},
	}, nil
}

func newSenderForTest(webhookURL string, client *http.Client) *WeComSender {
	return &WeComSender{webhookURL: webhookURL, client: client}
}

func (s *WeComSender) Enabled() bool { return s.webhookURL != "" }

func (s *WeComSender) Send(
	ctx context.Context,
	event delivery.OutboxEvent,
	payload delivery.RiderPoolOpenedPayload,
) error {
	if !s.Enabled() {
		return nil
	}
	if !delivery.ValidULID(event.EventPublicID) || !delivery.ValidULID(payload.OrderPublicID) ||
		payload.DeliveryFeeCents == 0 || strings.TrimSpace(payload.RouteSummary) == "" {
		return ErrWebhookPayload
	}
	content := fmt.Sprintf(
		"### 新配送单可抢\n> 订单：%s\n> 路线：%s\n> 配送费：¥%d.%02d\n> 事件：%s",
		payload.OrderPublicID, payload.RouteSummary,
		payload.DeliveryFeeCents/100, payload.DeliveryFeeCents%100, event.EventPublicID,
	)
	body, err := json.Marshal(map[string]any{
		"msgtype":  "markdown",
		"markdown": map[string]string{"content": content},
	})
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return ErrWebhookConfiguration
	}
	request.Header.Set("Content-Type", "application/json")
	result, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("%w: transport", ErrWebhookRejected)
	}
	defer result.Body.Close()
	var response struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(result.Body).Decode(&response); err != nil {
		return fmt.Errorf("%w: invalid response", ErrWebhookRejected)
	}
	if result.StatusCode < 200 || result.StatusCode >= 300 || response.ErrCode != 0 {
		return fmt.Errorf("%w: http=%d errcode=%d", ErrWebhookRejected, result.StatusCode, response.ErrCode)
	}
	return nil
}
