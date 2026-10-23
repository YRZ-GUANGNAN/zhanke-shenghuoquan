package deliveryintake

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"campus-circle/backend/internal/module/delivery"
	"github.com/gin-gonic/gin"
)

type fakeIntake struct {
	calls    int
	merchant uint64
	key      string
	envelope delivery.ExternalOrderEnvelope
	err      error
}

func (s *fakeIntake) Receive(_ context.Context, key string, _ delivery.SourcePlatform, envelope delivery.ExternalOrderEnvelope) (delivery.ExternalReceipt, error) {
	s.calls++
	s.key = key
	s.envelope = envelope
	return delivery.ExternalReceipt{ID: 9, Status: "pending", Duplicate: true}, s.err
}
func (s *fakeIntake) Receipt(_ context.Context, merchant, id uint64) (delivery.ExternalReceipt, error) {
	s.merchant = merchant
	return delivery.ExternalReceipt{ID: id, Status: "pending"}, s.err
}
func (s *fakeIntake) Retry(_ context.Context, merchant, id uint64) error {
	s.merchant = merchant
	return s.err
}

func testRouter(intake Intake, user uint64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(intake, func(*gin.Context) uint64 { return user })
	h.RegisterWebhook(r.Group("/callback"))
	h.RegisterMerchant(r.Group("/merchant"))
	return r
}
func send(r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("X-Provider-Signature", "unit-test-only")
	r.ServeHTTP(w, req)
	return w
}

func TestIntakeFailClosedAndExactEnvelope(t *testing.T) {
	if w := send(testRouter(nil, 1), "POST", "/callback/jd/opaque/events", `{}`); w.Code != 503 {
		t.Fatalf("unconfigured intake: %d", w.Code)
	}
	s := &fakeIntake{}
	r := testRouter(s, 7)
	for _, invalid := range []struct {
		path, body string
		status     int
	}{
		{"/callback/unknown/opaque/events", `{}`, 400},
		{"/callback/jd/opaque/events", "", 400},
		{"/callback/jd/opaque/events", strings.Repeat("a", 1024*1024+1), 413},
	} {
		w := send(r, "POST", invalid.path, invalid.body)
		if w.Code != invalid.status {
			t.Fatalf("invalid envelope: %d != %d", w.Code, invalid.status)
		}
	}
	if s.calls != 0 {
		t.Fatal("invalid input reached intake")
	}
	body := "{\n\"merchantUserId\":999,\"event\":\"created\"\n}"
	w := send(r, "POST", "/callback/taobao/server-owned-binding/events?sign=a%2Bb", body)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"pending"`) || !strings.Contains(w.Body.String(), `"duplicate":true`) {
		t.Fatalf("receipt: %s", w.Body.String())
	}
	if s.key != "server-owned-binding" || string(s.envelope.Payload) != body || s.envelope.RawQuery != "sign=a%2Bb" || s.envelope.Headers["X-Provider-Signature"][0] != "unit-test-only" || s.envelope.Method != "POST" {
		t.Fatal("signed envelope was changed")
	}
	if !s.envelope.ReceivedAt.IsZero() {
		t.Fatal("transport must not choose domain receive timestamp")
	}
	s.err = delivery.ErrExternalSignature
	if w := send(r, "POST", "/callback/jd/opaque/events", `{}`); w.Code != 401 {
		t.Fatal("invalid signature acknowledged")
	}
	s.err = delivery.ErrExternalEventConflict
	if w := send(r, "POST", "/callback/jd/opaque/events", `{}`); w.Code != 409 {
		t.Fatal("conflicting replay acknowledged")
	}
}

func TestReceiptMerchantScopeAndRetry(t *testing.T) {
	s := &fakeIntake{}
	r := testRouter(s, 7)
	if w := send(r, "GET", "/merchant/external-order-receipts/9?merchantUserId=999", ""); w.Code != 200 || s.merchant != 7 {
		t.Fatal("receipt did not use authenticated merchant")
	}
	if w := send(r, "POST", "/merchant/external-order-receipts/9/retry", `{"merchantUserId":999}`); w.Code != 200 || s.merchant != 7 {
		t.Fatal("retry changed scope")
	}
	s.err = delivery.ErrInvalidTransition
	if w := send(r, "POST", "/merchant/external-order-receipts/9/retry", `{}`); w.Code != 409 {
		t.Fatal("non-retryable state accepted")
	}
	if w := send(testRouter(s, 0), "GET", "/merchant/external-order-receipts/9", ""); w.Code != 401 {
		t.Fatal("anonymous receipt accepted")
	}
	if w := send(r, "GET", "/merchant/external-order-receipts/0", ""); w.Code != 400 {
		t.Fatal("invalid receipt accepted")
	}
}
