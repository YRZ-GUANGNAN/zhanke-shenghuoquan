package notification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWeComGroupSenderUsesMetadataOnlyAndChecksProviderCode(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusBadRequest} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var body string
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				raw, _ := io.ReadAll(request.Body)
				body = string(raw)
				writer.Header().Set("Content-Type", "application/json")
				if status == http.StatusOK {
					_, _ = writer.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
					return
				}
				_, _ = writer.Write([]byte(`{"errcode":93000,"errmsg":"invalid webhook"}`))
			}))
			defer server.Close()
			sender := newWeComGroupSenderForTest(map[string]string{"merchant_ops": server.URL}, server.Client())
			result, err := sender.Send(context.Background(), WecomGroupSnapshot{
				Version: 1, GroupKey: "merchant_ops", Title: "商家订单 · 待接单",
				Summary: "配送单等待接单", AggregateType: "delivery_order",
				AggregatePublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", AggregateVersion: 2,
				OccurredAt: time.Date(2026, 11, 13, 9, 30, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60)),
			})
			if status == http.StatusOK {
				if err != nil || result != SendStatusAccepted {
					t.Fatalf("accepted result=%s err=%v", result, err)
				}
			} else if err == nil || result != SendStatusRejected {
				t.Fatalf("rejected result=%s err=%v", result, err)
			}
			for _, required := range []string{"商家订单", "01ARZ3NDEKTSV4RRFFQ69G5FAV", "配送单等待接单", "2026-11-13 09:30"} {
				if !strings.Contains(body, required) {
					t.Fatalf("WeCom payload missing %q: %s", required, body)
				}
			}
			for _, forbidden := range []string{"13800000000", "Room 101", "recipientName", "openid", "webhook/send"} {
				if strings.Contains(body, forbidden) {
					t.Fatalf("WeCom payload leaked %q: %s", forbidden, body)
				}
			}
			var envelope map[string]any
			if json.Unmarshal([]byte(body), &envelope) != nil || envelope["msgtype"] != "markdown" {
				t.Fatalf("invalid WeCom request: %s", body)
			}
		})
	}
}

func TestWeComGroupSenderConfigurationAndUnknownGroupFailClosed(t *testing.T) {
	if _, err := NewWeComGroupSender(map[string]string{"unknown": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=x"}); err == nil {
		t.Fatal("unknown WeCom group was accepted")
	}
	for _, raw := range []string{
		"http://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=x",
		"https://example.com/cgi-bin/webhook/send?key=x",
		"https://qyapi.weixin.qq.com/cgi-bin/webhook/send",
	} {
		if _, err := NewWeComGroupSender(map[string]string{"merchant_ops": raw}); err == nil {
			t.Fatalf("invalid webhook was accepted: %s", raw)
		}
	}
}
