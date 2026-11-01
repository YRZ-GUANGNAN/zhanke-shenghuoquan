package notification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type callbackIdentityFake struct {
	accounts map[string]uint64
	relation IMNotificationRelation
	err      error
}

func (f callbackIdentityFake) ResolveIMAccount(_ context.Context, account string) (uint64, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.accounts[account], nil
}
func (f callbackIdentityFake) ResolveC2CRelation(_ context.Context, sender, recipient uint64) (IMNotificationRelation, error) {
	if f.err != nil {
		return IMNotificationRelation{}, f.err
	}
	value := f.relation
	value.SenderUserID, value.RecipientUserID = sender, recipient
	return value, nil
}

type callbackEventSinkFake struct {
	receipt CallbackReceipt
	event   Event
	calls   int
	err     error
}

func (f *callbackEventSinkFake) RecordCallbackAndEvent(_ context.Context, receipt CallbackReceipt, event Event) (bool, error) {
	f.calls++
	f.receipt, f.event = receipt, event
	if f.err != nil {
		return false, f.err
	}
	return true, nil
}

func signedCallbackRequest(now time.Time, cfg IMCallbackConfig, body string) *http.Request {
	stamp := strconv.FormatInt(now.Unix(), 10)
	digest := sha256.Sum256([]byte(cfg.Token + stamp))
	query := url.Values{
		"SdkAppid":        {strconv.FormatUint(cfg.SDKAppID, 10)},
		"RequestTime":     {stamp},
		"Sign":            {hex.EncodeToString(digest[:])},
		"CallbackCommand": {IMCallbackC2CAfterSend},
	}
	request := httptest.NewRequest(http.MethodPost, "/notification/tencent-im/callback?"+query.Encode(), strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestIMCallbackHandlerRecordsOnlyEligibleC2CEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Unix(1793500000, 0).UTC()
	cfg := IMCallbackConfig{SDKAppID: 1400000000, Token: "local-callback-test-token", Now: func() time.Time { return now }}
	sink := &callbackEventSinkFake{}
	h := NewIMCallbackHandler(cfg, callbackIdentityFake{
		accounts: map[string]uint64{"sender": 11, "recipient": 12},
		relation: IMNotificationRelation{SenderActive: true, RecipientActive: true, Allowed: true, ConversationPublicID: "conv-12", TargetPath: "/pages/messages/index?relationId=12"},
	}, sink)
	router := gin.New()
	h.Register(router.Group("/notification/tencent-im"))
	body := c2cFixture
	out := httptest.NewRecorder()
	router.ServeHTTP(out, signedCallbackRequest(now, cfg, body))
	if out.Code != http.StatusOK || sink.calls != 1 {
		t.Fatalf("status=%d calls=%d body=%q", out.Code, sink.calls, out.Body.String())
	}
	if sink.receipt.SenderUserID != 11 || sink.receipt.RecipientUserID != 12 || sink.receipt.ConversationPublicID != "conv-12" || sink.event.EventType != EventPrivateMessageReceived {
		t.Fatalf("receipt=%+v event=%+v", sink.receipt, sink.event)
	}
	if strings.Contains(string(sink.event.Payload), "private-message-never-retained") || strings.Contains(string(sink.event.Payload), "MsgBody") {
		t.Fatalf("message body leaked into event: %s", sink.event.Payload)
	}
	var payload map[string]any
	if err := json.Unmarshal(sink.event.Payload, &payload); err != nil || payload["recipientUserId"] != float64(12) || payload["targetPath"] != "/pages/messages/index?relationId=12" {
		t.Fatalf("payload=%s err=%v", sink.event.Payload, err)
	}
}

func TestIMCallbackHandlerAcksGroupAndIneligibleC2CWithoutWriting(t *testing.T) {
	now := time.Unix(1793500000, 0).UTC()
	cfg := IMCallbackConfig{SDKAppID: 1400000000, Token: "local-callback-test-token", Now: func() time.Time { return now }}
	sink := &callbackEventSinkFake{}
	identity := callbackIdentityFake{accounts: map[string]uint64{"sender": 11, "recipient": 12}, relation: IMNotificationRelation{SenderActive: true, RecipientActive: true, Allowed: true, ConversationPublicID: "conv", TargetPath: "/pages/messages/index"}}
	h := NewIMCallbackHandler(cfg, identity, sink)
	router := gin.New()
	h.Register(router.Group("/notification/tencent-im"))

	group := `{"CallbackCommand":"Group.CallbackAfterSendMsg","GroupId":"delivery-group","From_Account":"sender","MsgSeq":42,"MsgTime":1793500000}`
	request := signedCallbackRequest(now, cfg, group)
	request.URL.RawQuery = strings.Replace(request.URL.RawQuery, url.QueryEscape(IMCallbackC2CAfterSend), url.QueryEscape(IMCallbackGroupAfterSend), 1)
	out := httptest.NewRecorder()
	router.ServeHTTP(out, request)
	if out.Code != http.StatusOK || sink.calls != 0 {
		t.Fatalf("group status=%d calls=%d", out.Code, sink.calls)
	}

	identity.relation.Allowed = false
	h.Identity = identity
	out = httptest.NewRecorder()
	router.ServeHTTP(out, signedCallbackRequest(now, cfg, c2cFixture))
	if out.Code != http.StatusOK || sink.calls != 0 {
		t.Fatalf("ineligible status=%d calls=%d", out.Code, sink.calls)
	}
}

func TestIMCallbackHandlerRejectsBadAuthAndOversizedBody(t *testing.T) {
	now := time.Unix(1793500000, 0).UTC()
	cfg := IMCallbackConfig{SDKAppID: 1400000000, Token: "local-callback-test-token", Now: func() time.Time { return now }}
	sink := &callbackEventSinkFake{}
	h := NewIMCallbackHandler(cfg, callbackIdentityFake{}, sink)
	router := gin.New()
	h.Register(router.Group("/notification/tencent-im"))

	bad := signedCallbackRequest(now, cfg, c2cFixture)
	bad.URL.RawQuery = strings.Replace(bad.URL.RawQuery, "Sign=", "Sign=0", 1)
	out := httptest.NewRecorder()
	router.ServeHTTP(out, bad)
	if out.Code != http.StatusUnauthorized || sink.calls != 0 {
		t.Fatalf("bad auth status=%d calls=%d", out.Code, sink.calls)
	}

	oversized := signedCallbackRequest(now, cfg, strings.Repeat("x", 100))
	h.MaxBodyBytes = 64
	out = httptest.NewRecorder()
	router.ServeHTTP(out, oversized)
	if out.Code != http.StatusRequestEntityTooLarge || sink.calls != 0 {
		t.Fatalf("oversized status=%d calls=%d", out.Code, sink.calls)
	}
}

func TestBuildPrivateMessageEventRequiresTrustedRelation(t *testing.T) {
	now := time.Unix(1793500000, 0).UTC()
	callback := IMCallback{SDKAppID: 1400000000, ConversationType: IMCallbackC2C, ProviderMessageID: "provider", MsgSequence: 4, OccurredAt: now}
	valid := IMNotificationRelation{SenderUserID: 11, RecipientUserID: 12, SenderActive: true, RecipientActive: true, Allowed: true, ConversationPublicID: "conv", TargetPath: "/pages/messages/index"}
	event, err := BuildPrivateMessageEvent(callback, valid)
	if err != nil || event.AggregateVersion == 0 || event.AggregateType != "im_conversation" || event.EventType != EventPrivateMessageReceived {
		t.Fatalf("event=%+v err=%v", event, err)
	}
	for _, mutate := range []func(*IMNotificationRelation){
		func(v *IMNotificationRelation) { v.Allowed = false },
		func(v *IMNotificationRelation) { v.Blocked = true },
		func(v *IMNotificationRelation) { v.TargetPath = "https://evil.example" },
		func(v *IMNotificationRelation) { v.ConversationPublicID = "" },
	} {
		copy := valid
		mutate(&copy)
		if _, err := BuildPrivateMessageEvent(callback, copy); err == nil {
			t.Fatalf("untrusted relation accepted: %+v", copy)
		}
	}
}
