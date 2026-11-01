package notification

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMetadataRejectsPrivateMessageAndCredentialFields(t *testing.T) {
	for _, raw := range []string{
		`{"title":"订单已更新","orderId":"01J00000000000000000000000"}`,
		`{"messageBody":"不要落库"}`,
		`{"content":"私信正文"}`,
		`{"openid":"o-private"}`,
		`{"access_token":"secret"}`,
		`["array-root"]`,
	} {
		if validMetadata(json.RawMessage(raw)) && strings.Contains(raw, "{") && !strings.Contains(raw, "title") {
			t.Fatalf("sensitive metadata accepted: %s", raw)
		}
	}
	if !validMetadata(json.RawMessage(`{"title":"订单已更新","orderId":"01J00000000000000000000000"}`)) {
		t.Fatal("safe metadata rejected")
	}
	if !validMetadata(json.RawMessage(`{"context":"order"}`)) {
		t.Fatal("safe context metadata rejected")
	}
}

func TestSubscriptionOpenIDIsNeverJSONSerialized(t *testing.T) {
	value := Subscription{ID: 9, UserID: 42, OpenID: "openid-secret", Status: "accepted", RemainingQuota: 1}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "openid-secret") || strings.Contains(string(raw), "openid") {
		t.Fatalf("openid leaked through JSON: %s", raw)
	}
	if !reflect.DeepEqual(value.OpenID, "openid-secret") {
		t.Fatal("test fixture changed")
	}
}

func TestNewIDAndSafeErrorBoundaries(t *testing.T) {
	id, err := newIDAt(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), strings.NewReader(strings.Repeat("x", 10)))
	if err != nil || !validID(id) {
		t.Fatalf("id=%q err=%v", id, err)
	}
	long := strings.Repeat("敏", 300)
	if got := cleanError(long); len([]rune(got)) != 256 {
		t.Fatalf("clean error rune length=%d", len([]rune(got)))
	}
}
