package notification

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func callbackFixture(at time.Time) (string, IMCallbackConfig) {
	cfg := IMCallbackConfig{SDKAppID: 1400000000, Token: "local-callback-test-token", Now: func() time.Time { return at }}
	stamp := strconv.FormatInt(at.Unix(), 10)
	digest := sha256.Sum256([]byte(cfg.Token + stamp))
	q := url.Values{"SdkAppid": {"1400000000"}, "RequestTime": {stamp}, "Sign": {hex.EncodeToString(digest[:])}}
	return "https://local.example/callback?" + q.Encode(), cfg
}

const c2cFixture = `{"CallbackCommand":"C2C.CallbackAfterSendMsg","From_Account":"sender","To_Account":"recipient","MsgKey":"provider-key","MsgSeq":1,"MsgTime":1793500000,"SendMsgResult":0,"UnreadMsgNum":3,"MsgBody":[{"MsgType":"TIMTextElem","MsgContent":{"Text":"private-message-never-retained"}}]}`

func TestIMCallbackAuthenticationAndTimeBoundary(t *testing.T) {
	now := time.Unix(1793500000, 0)
	endpoint, cfg := callbackFixture(now)
	got, err := ParseIMCallback(endpoint, []byte(c2cFixture), cfg)
	if err != nil || got.ProviderMessageID != "provider-key" || got.UnreadMessageCount != 3 {
		t.Fatalf("metadata=%+v err=%v", got, err)
	}
	for _, delta := range []time.Duration{-61 * time.Second, 61 * time.Second} {
		local := cfg
		local.Now = func() time.Time { return now.Add(delta) }
		if _, err := ParseIMCallback(endpoint, []byte(c2cFixture), local); !errors.Is(err, ErrIMCallbackAuth) {
			t.Fatalf("stale request accepted: %v", err)
		}
	}
	for _, delta := range []time.Duration{-time.Minute, time.Minute} {
		local := cfg
		local.Now = func() time.Time { return now.Add(delta) }
		if _, err := ParseIMCallback(endpoint, []byte(c2cFixture), local); err != nil {
			t.Fatalf("boundary rejected: %v", err)
		}
	}
	for _, invalid := range []string{strings.Replace(endpoint, "1400000000", "1400000001", 1), endpoint + "bad", strings.Replace(endpoint, "Sign=", "Sign=0", 1)} {
		if _, err := ParseIMCallback(invalid, []byte(c2cFixture), cfg); err == nil {
			t.Fatal("invalid authentication accepted")
		}
	}
	cfg.Token = ""
	if _, err := ParseIMCallback(endpoint, []byte(c2cFixture), cfg); !errors.Is(err, ErrIMCallbackAuth) {
		t.Fatal("missing token accepted")
	}
}

func TestIMCallbackRejectsAmbiguousJSONAndSequenceOnlyC2C(t *testing.T) {
	endpoint, cfg := callbackFixture(time.Unix(1793500000, 0))
	cases := []string{
		strings.TrimSuffix(c2cFixture, "}") + `,"MsgBody":[]}`,
		strings.Replace(c2cFixture, `"Text":"private-message-never-retained"`, `"Text":"a","Text":"b"`, 1),
		c2cFixture + ` {}`,
		strings.Replace(c2cFixture, `"MsgKey":"provider-key",`, "", 1),
		strings.Replace(c2cFixture, `"SendMsgResult":0`, `"SendMsgResult":1`, 1),
		strings.TrimSuffix(c2cFixture, "}") + `,"extra":` + strings.Repeat("[", 14) + `0` + strings.Repeat("]", 14) + `}`,
	}
	for i, body := range cases {
		if _, err := ParseIMCallback(endpoint, []byte(body), cfg); err == nil {
			t.Fatalf("invalid case %d accepted", i)
		}
	}
}

func TestIMCallbackDistinguishesGroupCoordinates(t *testing.T) {
	endpoint, cfg := callbackFixture(time.Unix(1793500000, 0))
	body := `{"CallbackCommand":"Group.CallbackAfterSendMsg","GroupId":"delivery-group","From_Account":"sender","MsgSeq":42,"MsgTime":1793500000}`
	got, err := ParseIMCallback(endpoint, []byte(body), cfg)
	if err != nil || got.ConversationType != IMCallbackGroup || got.ProviderMessageID != "delivery-group:42" || got.RecipientAccount != "" {
		t.Fatalf("group metadata=%+v err=%v", got, err)
	}
	oversized := strings.Replace(body, "delivery-group", strings.Repeat("x", 116), 1)
	if _, err := ParseIMCallback(endpoint, []byte(oversized), cfg); err == nil {
		t.Fatal("oversized group accepted")
	}
}
