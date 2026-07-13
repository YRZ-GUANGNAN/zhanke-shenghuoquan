package trade

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResolveCreateRequestID(t *testing.T) {
	key := "01J00000000000000000000000"
	for _, test := range []struct {
		name, header, body string
		want               string
		wantErr            bool
	}{
		{name: "no key keeps legacy path"},
		{name: "header", header: key, want: key},
		{name: "body", body: key, want: key},
		{name: "matching header and body", header: key, body: key, want: key},
		{name: "mismatch", header: key, body: "01J00000000000000000000001", wantErr: true},
		{name: "invalid ULID", header: "not-a-ulid", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := resolveCreateRequestID(test.header, test.body)
			if (err != nil) != test.wantErr || got != test.want {
				t.Fatalf("got key=%q err=%v, want key=%q wantErr=%v", got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestWriteCreateResultFirstAndReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	firstRecorder := httptest.NewRecorder()
	firstContext, _ := gin.CreateTestContext(firstRecorder)
	writeCreateResult(firstContext, Product{ID: 7}, false)
	if firstRecorder.Code != http.StatusCreated || firstRecorder.Header().Get("Idempotency-Replayed") != "" {
		t.Fatalf("unexpected first response status=%d headers=%v", firstRecorder.Code, firstRecorder.Header())
	}

	replayRecorder := httptest.NewRecorder()
	replayContext, _ := gin.CreateTestContext(replayRecorder)
	writeCreateResult(replayContext, Product{ID: 7}, true)
	if replayRecorder.Code != http.StatusOK || replayRecorder.Header().Get("Idempotency-Replayed") != "true" {
		t.Fatalf("unexpected replay response status=%d headers=%v", replayRecorder.Code, replayRecorder.Header())
	}
}

func TestWriteMapsContactAvailabilityAndDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "contact state unavailable", err: ErrContactStateUnavailable, want: http.StatusServiceUnavailable},
		{name: "upstream deadline", err: context.DeadlineExceeded, want: http.StatusGatewayTimeout},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ginContext, _ := gin.CreateTestContext(recorder)
			write(ginContext, nil, test.err)
			if recorder.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", recorder.Code, test.want, recorder.Body.String())
			}
		})
	}
}
