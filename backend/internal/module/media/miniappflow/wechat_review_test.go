package miniappflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
)

type signerStub struct {
	key       string
	versionID string
	ttl       time.Duration
	url       string
	err       error
}

func (s *signerStub) SignedGetURL(_ context.Context, key, versionID string, ttl time.Duration) (string, error) {
	s.key = key
	s.versionID = versionID
	s.ttl = ttl
	if s.err != nil {
		return "", s.err
	}
	return s.url, nil
}

func TestWechatAsyncReviewSubmitterSignsFinalObjectAndSubmitsAsyncCheck(t *testing.T) {
	assetID := testULID(t)
	signer := &signerStub{url: "https://cos.example.com/final.webp?sign=short"}
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/token":
			if r.URL.Query().Get("appid") != "wx-app" || r.URL.Query().Get("secret") != "secret" {
				t.Fatalf("unexpected token query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"access_token":"token-1","expires_in":7200}`))
		case "/wxa/media_check_async":
			if r.URL.Query().Get("access_token") != "token-1" {
				t.Fatalf("missing access token: %s", r.URL.RawQuery)
			}
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok","trace_id":"trace-real-1"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	submitter, err := NewWechatAsyncReviewSubmitter(WechatReviewSubmitterConfig{
		AppID: "wx-app", AppSecret: "secret", APIBase: server.URL,
		Client: server.Client(),
	}, signer)
	if err != nil {
		t.Fatal(err)
	}
	value, err := submitter.SubmitMediaReview(context.Background(), WechatReviewSubmitInput{
		OpenID: "openid-1", AssetPublicID: assetID, Purpose: coremedia.PurposeDish,
		FinalKey: "final/key.webp", FinalVersionID: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	expectedTaskID, err := WechatReviewTaskID("trace-real-1")
	if err != nil {
		t.Fatal(err)
	}
	if value.TraceID != "trace-real-1" || value.ReviewTaskID != expectedTaskID {
		t.Fatalf("unexpected submission: %#v", value)
	}
	if signer.key != "final/key.webp" || signer.versionID != "v1" || signer.ttl != WechatReviewURLTTL {
		t.Fatalf("unexpected signer input: %#v", signer)
	}
	if received["openid"] != "openid-1" || received["media_url"] != signer.url ||
		received["media_type"].(float64) != 2 || received["version"].(float64) != 2 || received["scene"].(float64) != 2 {
		t.Fatalf("unexpected wechat request payload: %#v", received)
	}
}

func TestWechatReviewURLTTLAllowsWechatHeadAndGetWindow(t *testing.T) {
	if WechatReviewURLTTL != 35*time.Minute {
		t.Fatalf("WechatReviewURLTTL=%s want=35m", WechatReviewURLTTL)
	}
}

func TestWechatFailurePayloadHashBindsAllInputs(t *testing.T) {
	base := WechatFailurePayloadHash("trace-1", wechatMediaDownloadFailureCode, 42, 1)
	if base == "" || base != WechatFailurePayloadHash("trace-1", wechatMediaDownloadFailureCode, 42, 1) {
		t.Fatal("failure payload hash must be deterministic")
	}
	values := []string{
		WechatFailurePayloadHash("trace-2", wechatMediaDownloadFailureCode, 42, 1),
		WechatFailurePayloadHash("trace-1", wechatMediaPlatformFailureCode, 42, 1),
		WechatFailurePayloadHash("trace-1", wechatMediaDownloadFailureCode, 43, 1),
		WechatFailurePayloadHash("trace-1", wechatMediaDownloadFailureCode, 42, 2),
	}
	for _, value := range values {
		if value == base {
			t.Fatal("failure payload hash did not bind every input")
		}
	}
}

func TestWechatMediaFailureCodesAreSealed(t *testing.T) {
	for _, value := range []string{wechatMediaDownloadFailureCode, wechatMediaPlatformFailureCode} {
		if !validWechatMediaFailureCode(value) {
			t.Fatalf("sealed code rejected: %q", value)
		}
	}
	for _, value := range []string{"", "-1008", "wechat_unknown_suggest", " wechat_media_download_error"} {
		if validWechatMediaFailureCode(value) {
			t.Fatalf("unsealed code accepted: %q", value)
		}
	}
}

func TestWechatReviewHelpersAreDeterministicAndFailClosed(t *testing.T) {
	first, err := WechatReviewTaskID("trace-a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := WechatReviewTaskID("trace-a")
	if err != nil {
		t.Fatal(err)
	}
	other, err := WechatReviewTaskID("trace-b")
	if err != nil {
		t.Fatal(err)
	}
	if first == 0 || first != second || first == other {
		t.Fatalf("unexpected task ids first=%d second=%d other=%d", first, second, other)
	}
	if _, err := WechatReviewTaskID(" "); !strings.Contains(err.Error(), ErrInvalidRequest.Error()) {
		t.Fatalf("expected invalid empty trace id, got %v", err)
	}

	decision, resultCode, err := WechatSuggestDecision("pass")
	if err != nil || decision != coremedia.ProjectionApproved || resultCode != "" {
		t.Fatalf("pass decision=%s result=%s err=%v", decision, resultCode, err)
	}
	decision, resultCode, err = WechatSuggestDecision("unknown")
	if err != nil || decision != coremedia.ProjectionReviewFailed || resultCode != "wechat_unknown_suggest" {
		t.Fatalf("unknown decision=%s result=%s err=%v", decision, resultCode, err)
	}
}

func TestVerifyWechatReviewSignature(t *testing.T) {
	token, timestamp, nonce := "callback-token-strong", "1710000000", "nonce-1"
	// Known-good value is generated via the same URL query style WeChat uses;
	// it protects the public callback route from unsigned external writes.
	signature := "1392ae9a4f769473d1d3177acd70323156c47feb"
	if !VerifyWechatReviewSignature(token, timestamp, nonce, signature) {
		t.Fatal("expected valid callback signature")
	}
	query := url.Values{"timestamp": {timestamp}, "nonce": {nonce}, "signature": {"bad"}}
	if VerifyWechatReviewSignature(token, query.Get("timestamp"), query.Get("nonce"), query.Get("signature")) {
		t.Fatal("expected invalid callback signature")
	}
}
