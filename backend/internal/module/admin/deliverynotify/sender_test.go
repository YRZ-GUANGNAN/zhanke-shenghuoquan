package deliverynotify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"campus-circle/backend/internal/module/delivery"
)

func TestWeComSenderUsesMinimumInformationAndChecksErrCode(t *testing.T) {
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer server.Close()
	sender := newSenderForTest(server.URL, server.Client())
	event := delivery.OutboxEvent{EventPublicID: "01J00000000000000000000801"}
	payload := delivery.RiderPoolOpenedPayload{
		OrderPublicID: "01J00000000000000000000802",
		RouteSummary:  "校园配送单", DeliveryFeeCents: 150,
	}
	if err := sender.Send(context.Background(), event, payload); err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, forbidden := range []string{"13800000000", "Recipient", "Room 101", "webhook/send?key"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("webhook leaked %q: %s", forbidden, text)
		}
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["msgtype"] != "markdown" || !strings.Contains(text, event.EventPublicID) ||
		!strings.Contains(text, payload.OrderPublicID) || !strings.Contains(text, "¥1.50") {
		t.Fatalf("unexpected webhook body: %s", text)
	}
}

func TestWeComSenderRejectsBusinessErrorAndHidesURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":93000,"errmsg":"rejected"}`))
	}))
	defer server.Close()
	sender := newSenderForTest(server.URL+"?key=SECRET-KEY", server.Client())
	err := sender.Send(context.Background(),
		delivery.OutboxEvent{EventPublicID: "01J00000000000000000000803"},
		delivery.RiderPoolOpenedPayload{
			OrderPublicID: "01J00000000000000000000804",
			RouteSummary:  "校园配送单", DeliveryFeeCents: 150,
		},
	)
	if !errors.Is(err, ErrWebhookRejected) {
		t.Fatalf("business error returned %v", err)
	}
	if strings.Contains(err.Error(), "SECRET-KEY") || strings.Contains(err.Error(), server.URL) {
		t.Fatalf("webhook error leaked URL/key: %v", err)
	}
}

func TestWeComConfigurationFailClosed(t *testing.T) {
	disabled, err := NewWeComSender("")
	if err != nil || disabled.Enabled() {
		t.Fatalf("empty webhook=%#v err=%v", disabled, err)
	}
	for _, invalid := range []string{
		"http://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=x",
		"https://example.com/cgi-bin/webhook/send?key=x",
		"https://qyapi.weixin.qq.com/cgi-bin/webhook/send",
	} {
		if _, err := NewWeComSender(invalid); !errors.Is(err, ErrWebhookConfiguration) {
			t.Fatalf("invalid URL %q returned %v", invalid, err)
		}
	}
}
