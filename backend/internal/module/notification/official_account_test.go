package notification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type officialBindingRepositoryFake struct {
	binding OfficialAccountBinding
	err     error
}

func (f *officialBindingRepositoryFake) FindOfficialAccountBinding(context.Context, uint64, string) (OfficialAccountBinding, error) {
	if f.err != nil {
		return OfficialAccountBinding{}, f.err
	}
	if f.binding.UserID == 0 {
		return OfficialAccountBinding{}, ErrOfficialAccountNotBound
	}
	return f.binding, nil
}

func (f *officialBindingRepositoryFake) UpsertOfficialAccountBinding(_ context.Context, value OfficialAccountBinding, now time.Time) (OfficialAccountBinding, error) {
	value.RowVersion = 1
	value.BoundAt = now
	value.UpdatedAt = now
	f.binding = value
	return value, nil
}

func (f *officialBindingRepositoryFake) UpdateOfficialAccountFollowState(_ context.Context, _ uint64, _ string, state string, now time.Time) error {
	if f.binding.UserID == 0 {
		return ErrOfficialAccountNotBound
	}
	f.binding.FollowState = state
	f.binding.RowVersion++
	f.binding.UpdatedAt = now
	return nil
}

func (f *officialBindingRepositoryFake) UnbindOfficialAccount(context.Context, uint64, string) error {
	f.binding = OfficialAccountBinding{}
	return nil
}

func (f *officialBindingRepositoryFake) CountOfficialAccountBindings(context.Context, string, string) (int64, error) {
	if f.binding.UserID == 0 {
		return 0, nil
	}
	return 1, nil
}

func TestOfficialAccountOAuthBindingAndStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/sns/oauth2/access_token" {
			http.NotFound(writer, request)
			return
		}
		_, _ = writer.Write([]byte(`{"access_token":"oauth-token","openid":"official-openid","errcode":0}`))
	}))
	defer server.Close()
	repo := &officialBindingRepositoryFake{}
	service, err := NewOfficialAccountService(repo, OfficialAccountConfig{
		Enabled: true, AppID: "official-app", AppSecret: "official-secret",
		APIBase: server.URL, RedirectURL: "https://zhanduiwang.top/api/v1/notification/official-account/callback",
		StateSecret: strings.Repeat("s", 32), HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	oauthURL, err := service.OAuthURL(7)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(oauthURL)
	if err != nil || parsed.Host != "open.weixin.qq.com" || parsed.Query().Get("state") == "" {
		t.Fatalf("invalid OAuth URL: %s err=%v", oauthURL, err)
	}
	userID, err := service.CompleteBinding(context.Background(), "oauth-code", parsed.Query().Get("state"))
	if err != nil || userID != 7 || repo.binding.OpenID != "official-openid" {
		t.Fatalf("binding user=%d binding=%+v err=%v", userID, repo.binding, err)
	}
	status, err := service.Status(context.Background(), 7)
	if err != nil || !status.Enabled || !status.Bound || status.FollowState != "unknown" {
		t.Fatalf("status=%+v err=%v", status, err)
	}
	if _, err := service.CompleteBinding(context.Background(), "oauth-code", parsed.Query().Get("state")+"x"); err == nil {
		t.Fatal("tampered OAuth state was accepted")
	}
	if err := service.MarkFollowState(context.Background(), 7, "following"); err != nil || repo.binding.FollowState != "following" {
		t.Fatalf("follow state=%s err=%v", repo.binding.FollowState, err)
	}
}

type officialWorkerStoreFake struct {
	lease     DeliveryLease
	completed DeliveryStatus
	failed    bool
}

func (f *officialWorkerStoreFake) ClaimWechatOfficialDelivery(context.Context, time.Time, time.Duration) (DeliveryLease, bool, error) {
	return f.lease, true, nil
}

func (f *officialWorkerStoreFake) CompleteDelivery(_ context.Context, _ DeliveryLease, status DeliveryStatus, _ time.Time) error {
	f.completed = status
	return nil
}

func (f *officialWorkerStoreFake) FailDelivery(context.Context, DeliveryLease, string, string, time.Time, uint32, time.Time) (DeliveryStatus, error) {
	f.failed = true
	return DeliveryRetryWait, nil
}

func TestOfficialAccountWorkerSendsBoundTemplate(t *testing.T) {
	var sentBody string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/cgi-bin/token":
			_, _ = writer.Write([]byte(`{"access_token":"access-token","expires_in":7200,"errcode":0}`))
		case "/cgi-bin/message/template/send":
			raw, _ := io.ReadAll(request.Body)
			sentBody = string(raw)
			_, _ = writer.Write([]byte(`{"errcode":0,"errmsg":"ok","msgid":12345}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	sender, err := NewOfficialAccountSender(OfficialAccountSenderConfig{
		AppID: "official-app", AppSecret: "secret", MiniProgramAppID: "wx-mini",
		APIBase: server.URL, Enabled: true, HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(TemplateSnapshot{
		TemplateID: "official-template", Page: "/pages/merchant-order-detail/index?orderId=ORDER",
		Data: map[string]SubscribeTemplateValue{
			"thing1": {Value: "配送单等待接单"}, "time2": {Value: "2026-11-13 09:30"},
		},
	})
	now := time.Date(2026, 11, 13, 1, 30, 0, 0, time.UTC)
	token := "lease"
	lease := DeliveryLease{LeaseToken: token, Item: DeliveryItem{
		ID: 1, DeliveryPublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", EventPublicID: "01ARZ3NDEKTSV4RRFFQ69G5FAW",
		RecipientUserID: 7, Channel: ChannelWechatOfficial, Purpose: PurposeOrder,
		TemplateKey: "official_order", TemplateSnapshot: raw, PayloadVersion: 1,
		Status: DeliveryProcessing, AttemptCount: 1, RowVersion: 2, LeaseToken: &token,
	}}
	store := &officialWorkerStoreFake{lease: lease}
	marked := ""
	worker := NewOfficialAccountWorker(store, sender,
		func(context.Context, DeliveryItem, TemplateSnapshot) (uint64, error) { return 7, nil },
		func(context.Context, uint64) (string, error) { return "official-openid", nil },
	)
	worker.MarkFollow = func(_ context.Context, userID uint64, state string) error {
		if userID != 7 {
			t.Fatalf("unexpected user=%d", userID)
		}
		marked = state
		return nil
	}
	worker.Now = func() time.Time { return now }
	if found, err := worker.RunOnce(context.Background()); err != nil || !found || store.completed != DeliveryAccepted || marked != "following" {
		t.Fatalf("found=%v completed=%s marked=%s err=%v", found, store.completed, marked, err)
	}
	for _, required := range []string{"official-openid", "official-template", "wx-mini", "merchant-order-detail", "配送单等待接单"} {
		if !strings.Contains(sentBody, required) {
			t.Fatalf("official message missing %q: %s", required, sentBody)
		}
	}
}
