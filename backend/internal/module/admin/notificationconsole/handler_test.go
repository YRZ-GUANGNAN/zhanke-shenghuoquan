package notificationconsole

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"campus-circle/backend/internal/module/notification"
)

type testRepository struct{}

func (testRepository) ChannelStats(context.Context, time.Time) ([]ChannelStat, error) {
	return []ChannelStat{
		{Channel: notification.ChannelWechatSubscribe, Status: notification.DeliveryAccepted, Count: 8},
		{Channel: notification.ChannelWechatOfficial, Status: notification.DeliveryRetryWait, Count: 2},
		{Channel: notification.ChannelWecom, Status: notification.DeliveryDeadletter, Count: 1},
	}, nil
}

func (testRepository) OfficialAccountCounts(context.Context, string) (OfficialAccountCounts, error) {
	return OfficialAccountCounts{Unknown: 3, Following: 4, NotFollowing: 1}, nil
}

func (testRepository) RecentDeliveries(context.Context, notification.DeliveryStatus, int, int) ([]DeliveryRecord, int64, error) {
	next := time.Date(2026, 3, 5, 10, 30, 0, 0, time.UTC)
	return []DeliveryRecord{{
		DeliveryPublicID: "01JTESTDELIVERY0000000001", RecipientUserID: 42,
		Channel: notification.ChannelWechatOfficial, Purpose: "order_status",
		Status: notification.DeliveryRetryWait, AttemptCount: 3,
		LastErrorCode: "errcode_43101", LastError: "user refuse to accept the msg",
		NextAttemptAt: &next, CreatedAt: next,
	}}, 1, nil
}

func TestNotificationChannelOverviewIsAggregateAndSecretFree(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(testRepository{}, Config{
		UserNotifications: true, SubscribeEnabled: true, NotificationSending: true, NotificationAudience: "all",
		OfficialEnabled: true, OfficialAppID: "official-app",
		OfficialTemplateReady: true, WecomMerchantConfigured: true, WecomRiderConfigured: false,
	}, nil, func() time.Time { return time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC) })
	handler.Register(router.Group("/admin/delivery"))
	request := httptest.NewRequest(http.MethodGet, "/admin/delivery/notification-channels", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Data Overview `json:"data"`
	}
	if json.Unmarshal(response.Body.Bytes(), &body) != nil {
		t.Fatal("invalid response JSON")
	}
	if len(body.Data.Channels) != 4 || !body.Data.Channels[0].Enabled ||
		body.Data.Channels[1].Accepted != 8 ||
		body.Data.Channels[2].RetryWait != 2 || body.Data.Channels[3].Deadletter != 1 ||
		body.Data.Official.Unknown != 3 || !body.Data.Wecom.MerchantConfigured ||
		body.Data.Wecom.RiderConfigured || !body.Data.SendingEnabled ||
		body.Data.Audience != "all" || !body.Data.CheckedAt.Equal(time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected overview: %+v", body.Data)
	}
	for _, forbidden := range []string{"official-app", "secret", "openid", "webhook", "key="} {
		if containsFold(response.Body.String(), forbidden) {
			t.Fatalf("admin overview leaked %q: %s", forbidden, response.Body.String())
		}
	}
}

func TestNotificationChannelTestRejectsUnknownGroupAndUnconfiguredSender(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewHandler(testRepository{}, Config{}, nil, time.Now)
	handler.Register(router.Group("/admin/delivery"))

	invalid := httptest.NewRequest(http.MethodPost, "/admin/delivery/notification-channels/wecom/test",
		bytes.NewBufferString(`{"group":"all_users"}`))
	invalid.Header.Set("Content-Type", "application/json")
	invalidResponse := httptest.NewRecorder()
	router.ServeHTTP(invalidResponse, invalid)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid group status=%d body=%s", invalidResponse.Code, invalidResponse.Body.String())
	}

	unconfigured := httptest.NewRequest(http.MethodPost, "/admin/delivery/notification-channels/wecom/test",
		bytes.NewBufferString(`{"group":"merchant_ops"}`))
	unconfigured.Header.Set("Content-Type", "application/json")
	unconfiguredResponse := httptest.NewRecorder()
	router.ServeHTTP(unconfiguredResponse, unconfigured)
	if unconfiguredResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured sender status=%d body=%s", unconfiguredResponse.Code, unconfiguredResponse.Body.String())
	}
}

func containsFold(value, target string) bool {
	return len(target) > 0 && (len(value) >= len(target)) &&
		func() bool {
			for i := 0; i+len(target) <= len(value); i++ {
				if equalFoldASCII(value[i:i+len(target)], target) {
					return true
				}
			}
			return false
		}()
}

func equalFoldASCII(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		a, b := left[i], right[i]
		if a >= 'A' && a <= 'Z' {
			a += 'a' - 'A'
		}
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}
