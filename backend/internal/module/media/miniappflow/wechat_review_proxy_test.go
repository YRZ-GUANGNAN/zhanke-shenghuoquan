package miniappflow

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/middleware"
	coremedia "campus-circle/backend/internal/module/media"

	"github.com/gin-gonic/gin"
)

type reviewProxyStoreStub struct{}

func (reviewProxyStoreStub) HeadObject(context.Context, string, string) (coremedia.CompletionObject, error) {
	return coremedia.CompletionObject{}, coremedia.ErrObjectNotFound
}

func (reviewProxyStoreStub) OpenObject(context.Context, string, string) (io.ReadCloser, error) {
	return nil, coremedia.ErrObjectNotFound
}

func TestWechatReviewProxyConstructorRejectsUnsafeBaseURL(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		"", "http://api.example.test", "https://", "https://user@api.example.test",
		"https://api.example.test/base", "https://api.example.test?token=bad", "https://api.example.test?",
		"https://api.example.test#fragment", "https://api.example.test/%2f",
	} {
		if _, err := NewWechatReviewProxy(WechatReviewProxyConfig{
			PublicBaseURL: value,
			AppSecret:     "wechat-app-secret-at-least-sixteen",
		}, reviewProxyStoreStub{}); err == nil {
			t.Fatalf("unsafe base URL was accepted: %q", value)
		}
	}
	if _, err := NewWechatReviewProxy(WechatReviewProxyConfig{
		PublicBaseURL: "https://api.example.test",
		AppSecret:     "",
	}, reviewProxyStoreStub{}); err == nil {
		t.Fatal("empty app secret was accepted")
	}
	if _, err := NewWechatReviewProxy(WechatReviewProxyConfig{
		PublicBaseURL: "https://api.example.test",
		AppSecret:     "short",
	}, reviewProxyStoreStub{}); err != nil {
		t.Fatalf("non-empty app secret was rejected: %v", err)
	}
	var nilStore *recordingReviewProxyStore
	if _, err := NewWechatReviewProxy(WechatReviewProxyConfig{
		PublicBaseURL: "https://api.example.test",
		AppSecret:     "wechat-app-secret-at-least-sixteen",
	}, nilStore); err == nil {
		t.Fatal("typed nil store was accepted")
	}
}

func TestWechatReviewProxyTokenBindsFinalVersionAndExpires(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	proxy, err := NewWechatReviewProxy(WechatReviewProxyConfig{
		PublicBaseURL: "https://api.example.test/",
		AppSecret:     "wechat-app-secret-at-least-sixteen",
	}, reviewProxyStoreStub{})
	if err != nil {
		t.Fatal(err)
	}
	proxy.now = func() time.Time { return now }
	key, versionID := "final/prod/product/2026/07/01K00000000000000000000001/asset.png", "version-1"
	value, err := proxy.SignedGetURL(context.Background(), key, versionID, WechatReviewURLTTL)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "https" || parsed.Host != "api.example.test" || parsed.Path != WechatReviewContentPath {
		t.Fatalf("unexpected proxy URL: %s", value)
	}
	token := parsed.Query().Get("token")
	if token == "" || strings.Contains(token, key) || strings.Contains(token, versionID) {
		t.Fatal("opaque token is missing or exposes object identity")
	}
	claims, err := proxy.openClaims(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Version != 1 || claims.ObjectKey != key || claims.VersionID != versionID ||
		claims.ExpiresAt != now.Add(WechatReviewURLTTL).Unix() {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	otherKey, err := NewWechatReviewProxy(WechatReviewProxyConfig{
		PublicBaseURL: "https://api.example.test",
		AppSecret:     "different-wechat-app-secret",
	}, reviewProxyStoreStub{})
	if err != nil {
		t.Fatal(err)
	}
	otherKey.now = func() time.Time { return now }
	if _, err := otherKey.openClaims(token); err == nil {
		t.Fatal("token was accepted with a different app secret")
	}

	other, err := proxy.SignedGetURL(context.Background(), key, versionID, WechatReviewURLTTL)
	if err != nil || other == value {
		t.Fatal("independent issuance must use a fresh random GCM nonce")
	}
}

func TestWechatReviewProxyRejectsTamperedExpiredAndInvalidIdentity(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	proxy, err := NewWechatReviewProxy(WechatReviewProxyConfig{
		PublicBaseURL: "https://api.example.test",
		AppSecret:     "wechat-app-secret-at-least-sixteen",
	}, reviewProxyStoreStub{})
	if err != nil {
		t.Fatal(err)
	}
	proxy.now = func() time.Time { return now }
	value, err := proxy.SignedGetURL(context.Background(), "final/prod/product/2026/07/id/image.png", "v1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(value)
	token := parsed.Query().Get("token")
	tampered := toggleTokenByte(token)
	if _, err := proxy.openClaims(tampered); err == nil {
		t.Fatal("tampered token was accepted")
	}
	proxy.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := proxy.openClaims(token); err == nil {
		t.Fatal("expired token was accepted")
	}
	proxy.now = func() time.Time { return now }
	farFuture, err := proxy.codec.seal(wechatReviewProxyClaims{
		Version: 1, ExpiresAt: now.Add(wechatReviewProxyMaxFuture + time.Second).Unix(),
		ObjectKey: "final/prod/item.png", VersionID: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := proxy.openClaims(farFuture); err == nil {
		t.Fatal("far-future token was accepted")
	}
	expires := strconv.FormatInt(now.Add(time.Minute).Unix(), 10)
	for _, raw := range []string{
		`{"version":1,"expiresAt":` + expires + `,"objectKey":"final/prod/item.png","versionId":"v1","extra":true}`,
		`{"version":1,"version":1,"expiresAt":` + expires + `,"objectKey":"final/prod/item.png","versionId":"v1"}`,
		`{"version":1,"expiresAt":` + expires + `,"objectKey":"final/prod/item.png","versionId":"v1"}{}`,
	} {
		if _, err := proxy.openClaims(sealRawReviewProxyToken(t, proxy.codec, []byte(raw))); err == nil {
			t.Fatalf("non-strict claims were accepted: %s", raw)
		}
	}
	for _, identity := range [][2]string{
		{"../secret.png", "v1"}, {"final/prod/item.exe", "v1"}, {"final/prod/item.png", ""},
		{"final/prod/a..b.png", "v1"},
		{"final/prod/item\n.png", "v1"}, {"final/prod/item.png", "v1\r\nheader"},
	} {
		if _, err := proxy.SignedGetURL(context.Background(), identity[0], identity[1], time.Minute); err == nil {
			t.Fatalf("invalid identity was accepted: %#v", identity)
		}
	}
	if _, err := proxy.SignedGetURL(context.Background(), "final/prod/item.png", "v1", WechatReviewURLTTL+time.Second); err == nil {
		t.Fatal("overlong TTL was accepted")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := proxy.SignedGetURL(cancelled, "final/prod/item.png", "v1", time.Minute); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled signer err=%v", err)
	}
}

type recordingReviewProxyStore struct {
	object    coremedia.CompletionObject
	payload   []byte
	headErr   error
	openErr   error
	openBody  io.ReadCloser
	headCalls int
	openCalls int
	headKey   string
	headVer   string
	openKey   string
	openVer   string
}

func (s *recordingReviewProxyStore) HeadObject(_ context.Context, key, version string) (coremedia.CompletionObject, error) {
	s.headCalls++
	s.headKey, s.headVer = key, version
	if s.headErr != nil {
		return coremedia.CompletionObject{}, s.headErr
	}
	return s.object, nil
}

func (s *recordingReviewProxyStore) OpenObject(_ context.Context, key, version string) (io.ReadCloser, error) {
	s.openCalls++
	s.openKey, s.openVer = key, version
	if s.openErr != nil {
		return nil, s.openErr
	}
	if s.openBody != nil {
		return s.openBody, nil
	}
	if s.payload == nil {
		return nil, nil
	}
	return io.NopCloser(bytes.NewReader(s.payload)), nil
}

func TestWechatReviewProxyUsesSameTokenForHeadAndGet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	payload := []byte("safe-png-test-payload")
	key, versionID := "final/prod/product/2026/07/id/image.png", "version-7"
	store := &recordingReviewProxyStore{
		payload: payload,
		object: coremedia.CompletionObject{
			Key: key, VersionID: versionID, ContentType: "image/png", Size: uint64(len(payload)),
		},
	}
	proxy, requestURI := newReviewProxyRequestURI(t, store, key, versionID)
	router := gin.New()
	proxy.Register(router.Group("/api/v1/miniapp/media"))

	head := httptest.NewRecorder()
	router.ServeHTTP(head, httptest.NewRequest(http.MethodHead, requestURI, nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 || store.headCalls != 1 || store.openCalls != 0 {
		t.Fatalf("HEAD status=%d body=%d head=%d open=%d", head.Code, head.Body.Len(), store.headCalls, store.openCalls)
	}

	get := httptest.NewRecorder()
	router.ServeHTTP(get, httptest.NewRequest(http.MethodGet, requestURI, nil))
	if get.Code != http.StatusOK || get.Body.String() != string(payload) || store.headCalls != 2 || store.openCalls != 1 {
		t.Fatalf("GET status=%d body=%q head=%d open=%d", get.Code, get.Body.String(), store.headCalls, store.openCalls)
	}
	if store.headKey != key || store.headVer != versionID || store.openKey != key || store.openVer != versionID {
		t.Fatal("proxy did not pin HEAD and GET to the token version")
	}
	for name, want := range map[string]string{
		"Content-Type": "image/png", "Content-Length": strconv.Itoa(len(payload)),
		"Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff",
	} {
		if get.Header().Get(name) != want || head.Header().Get(name) != want {
			t.Fatalf("header %s GET=%q HEAD=%q want=%q", name, get.Header().Get(name), head.Header().Get(name), want)
		}
	}
	rangeRequest := httptest.NewRequest(http.MethodGet, requestURI, nil)
	rangeRequest.Header.Set("Range", "bytes=0-2")
	rangeResponse := httptest.NewRecorder()
	router.ServeHTTP(rangeResponse, rangeRequest)
	if rangeResponse.Code != http.StatusOK || rangeResponse.Body.String() != string(payload) ||
		rangeResponse.Header().Get("Content-Range") != "" {
		t.Fatalf(
			"Range changed fixed full response: status=%d body=%q content-range=%q",
			rangeResponse.Code, rangeResponse.Body.String(), rangeResponse.Header().Get("Content-Range"),
		)
	}
}

func TestWechatReviewProxyRejectsBeforeStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, versionID := "final/prod/product/2026/07/id/image.png", "version-7"
	for _, test := range []struct {
		name   string
		mutate func(*WechatReviewProxy, string) string
	}{
		{name: "missing", mutate: func(_ *WechatReviewProxy, _ string) string { return WechatReviewContentPath }},
		{name: "extra", mutate: func(_ *WechatReviewProxy, uri string) string { return uri + "&extra=value" }},
		{name: "duplicate", mutate: func(_ *WechatReviewProxy, uri string) string {
			parsed, _ := url.Parse(uri)
			return uri + "&token=" + url.QueryEscape(parsed.Query().Get("token"))
		}},
		{name: "tampered", mutate: func(_ *WechatReviewProxy, uri string) string {
			parsed, _ := url.Parse(uri)
			return WechatReviewContentPath + "?token=" + url.QueryEscape(toggleTokenByte(parsed.Query().Get("token")))
		}},
		{name: "expired", mutate: func(proxy *WechatReviewProxy, uri string) string {
			now := proxy.now()
			proxy.now = func() time.Time { return now.Add(2 * time.Minute) }
			return uri
		}},
		{name: "oversized", mutate: func(_ *WechatReviewProxy, _ string) string {
			return WechatReviewContentPath + "?token=" + strings.Repeat("a", wechatReviewProxyMaxToken+1)
		}},
		{name: "malformed", mutate: func(_ *WechatReviewProxy, _ string) string {
			return WechatReviewContentPath + "?token=%zz"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := validRecordingReviewProxyStore(key, versionID)
			proxy, requestURI := newReviewProxyRequestURI(t, store, key, versionID)
			requestURI = test.mutate(proxy, requestURI)
			recorder := serveReviewProxyRequest(proxy, http.MethodGet, requestURI)
			if recorder.Code != http.StatusNotFound || recorder.Body.Len() != 0 {
				t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
			}
			if store.headCalls != 0 || store.openCalls != 0 {
				t.Fatalf("invalid token reached storage: head=%d open=%d", store.headCalls, store.openCalls)
			}
		})
	}
}

func TestWechatReviewProxyRejectsObjectIdentityMIMEAndSizeMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, versionID := "final/prod/product/2026/07/id/image.png", "version-7"
	for _, test := range []struct {
		name   string
		mutate func(*coremedia.CompletionObject)
	}{
		{name: "key", mutate: func(object *coremedia.CompletionObject) { object.Key = "final/prod/other.png" }},
		{name: "version", mutate: func(object *coremedia.CompletionObject) { object.VersionID = "version-8" }},
		{name: "mime", mutate: func(object *coremedia.CompletionObject) { object.ContentType = "application/octet-stream" }},
		{name: "mime parameters", mutate: func(object *coremedia.CompletionObject) { object.ContentType = "image/png; charset=binary" }},
		{name: "empty", mutate: func(object *coremedia.CompletionObject) { object.Size = 0 }},
		{name: "oversized", mutate: func(object *coremedia.CompletionObject) { object.Size = wechatReviewProxyMaxBytes + 1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := validRecordingReviewProxyStore(key, versionID)
			test.mutate(&store.object)
			proxy, requestURI := newReviewProxyRequestURI(t, store, key, versionID)
			recorder := serveReviewProxyRequest(proxy, http.MethodGet, requestURI)
			if recorder.Code != http.StatusNotFound || recorder.Body.Len() != 0 {
				t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
			}
			if store.headCalls != 1 || store.openCalls != 0 {
				t.Fatalf("head=%d open=%d", store.headCalls, store.openCalls)
			}
		})
	}
}

func TestWechatReviewProxyMapsNotFoundAndTemporaryStorageErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, versionID := "final/prod/product/2026/07/id/image.png", "version-7"
	temporary := errors.New("temporary storage failure with sensitive diagnostics")
	for _, test := range []struct {
		name       string
		headErr    error
		openErr    error
		nilPayload bool
		wantStatus int
		wantOpen   int
	}{
		{name: "head not found", headErr: coremedia.ErrObjectNotFound, wantStatus: http.StatusNotFound},
		{name: "head temporary", headErr: temporary, wantStatus: http.StatusServiceUnavailable},
		{name: "open not found", openErr: coremedia.ErrObjectNotFound, wantStatus: http.StatusNotFound, wantOpen: 1},
		{name: "open temporary", openErr: temporary, wantStatus: http.StatusServiceUnavailable, wantOpen: 1},
		{name: "nil body", nilPayload: true, wantStatus: http.StatusServiceUnavailable, wantOpen: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := validRecordingReviewProxyStore(key, versionID)
			store.headErr = test.headErr
			store.openErr = test.openErr
			if test.nilPayload {
				store.payload = nil
			}
			proxy, requestURI := newReviewProxyRequestURI(t, store, key, versionID)
			recorder := serveReviewProxyRequest(proxy, http.MethodGet, requestURI)
			if recorder.Code != test.wantStatus || recorder.Body.Len() != 0 {
				t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
			}
			if store.headCalls != 1 || store.openCalls != test.wantOpen {
				t.Fatalf("head=%d open=%d", store.headCalls, store.openCalls)
			}
		})
	}
}

type reviewProxyErrorReadCloser struct{ err error }

func (r *reviewProxyErrorReadCloser) Read([]byte) (int, error) { return 0, r.err }
func (r *reviewProxyErrorReadCloser) Close() error             { return nil }

type reviewProxyTypedNilReadCloser struct{}

func (*reviewProxyTypedNilReadCloser) Read([]byte) (int, error) { return 0, io.EOF }
func (*reviewProxyTypedNilReadCloser) Close() error             { return nil }

func TestWechatReviewProxyMapsFirstBodyReadFailureTo503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, versionID := "final/prod/product/2026/07/id/image.png", "version-7"
	store := validRecordingReviewProxyStore(key, versionID)
	store.openBody = &reviewProxyErrorReadCloser{err: errors.New("sensitive first read failure")}
	proxy, requestURI := newReviewProxyRequestURI(t, store, key, versionID)
	streamErrors := 0
	proxy.logStreamError = func() { streamErrors++ }

	recorder := serveReviewProxyRequest(proxy, http.MethodGet, requestURI)
	if recorder.Code != http.StatusServiceUnavailable || recorder.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	if streamErrors != 0 {
		t.Fatalf("pre-response read failure logged as committed stream error: %d", streamErrors)
	}
}

func TestWechatReviewProxyMapsTypedNilBodyTo503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, versionID := "final/prod/product/2026/07/id/image.png", "version-7"
	store := validRecordingReviewProxyStore(key, versionID)
	var body *reviewProxyTypedNilReadCloser
	store.openBody = body
	proxy, requestURI := newReviewProxyRequestURI(t, store, key, versionID)

	recorder := serveReviewProxyRequest(proxy, http.MethodGet, requestURI)
	if recorder.Code != http.StatusServiceUnavailable || recorder.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
}

func TestWechatReviewProxyLogsFixedCategoryAfterCommittedShortRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, versionID := "final/prod/product/2026/07/id/image.png", "version-7"
	firstBlock := bytes.Repeat([]byte{0x5a}, 32<<10)
	payload := append(firstBlock, []byte("short-tail")...)
	store := &recordingReviewProxyStore{
		payload: payload,
		object: coremedia.CompletionObject{
			Key: key, VersionID: versionID, ContentType: "image/png", Size: uint64(len(payload) + 100),
		},
	}
	proxy, requestURI := newReviewProxyRequestURI(t, store, key, versionID)
	streamErrors := 0
	proxy.logStreamError = func() { streamErrors++ }

	recorder := serveReviewProxyRequest(proxy, http.MethodGet, requestURI)
	if recorder.Code != http.StatusOK || recorder.Body.Len() != len(payload) {
		t.Fatalf("status=%d body_length=%d want=%d", recorder.Code, recorder.Body.Len(), len(payload))
	}
	if streamErrors != 1 {
		t.Fatalf("stream error category calls=%d want=1", streamErrors)
	}
}

func TestWechatReviewProxyStreamsCompleteBodyPastFirstBlock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, versionID := "final/prod/product/2026/07/id/image.png", "version-7"
	payload := bytes.Repeat([]byte{0x6b}, (32<<10)+1)
	store := &recordingReviewProxyStore{
		payload: payload,
		object: coremedia.CompletionObject{
			Key: key, VersionID: versionID, ContentType: "image/png", Size: uint64(len(payload)),
		},
	}
	proxy, requestURI := newReviewProxyRequestURI(t, store, key, versionID)
	streamErrors := 0
	proxy.logStreamError = func() { streamErrors++ }

	recorder := serveReviewProxyRequest(proxy, http.MethodGet, requestURI)
	if recorder.Code != http.StatusOK || !bytes.Equal(recorder.Body.Bytes(), payload) {
		t.Fatalf("status=%d body_length=%d want=%d", recorder.Code, recorder.Body.Len(), len(payload))
	}
	if streamErrors != 0 {
		t.Fatalf("complete stream logged errors: %d", streamErrors)
	}
}

func TestWechatReviewProxyRejectsUnsupportedMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, versionID := "final/prod/product/2026/07/id/image.png", "version-7"
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			store := validRecordingReviewProxyStore(key, versionID)
			proxy, requestURI := newReviewProxyRequestURI(t, store, key, versionID)
			recorder := serveReviewProxyRequest(proxy, method, requestURI)
			if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != "HEAD, GET" || recorder.Body.Len() != 0 {
				t.Fatalf("status=%d allow=%q body=%q", recorder.Code, recorder.Header().Get("Allow"), recorder.Body.String())
			}
			if store.headCalls != 0 || store.openCalls != 0 {
				t.Fatalf("unsupported method reached storage: head=%d open=%d", store.headCalls, store.openCalls)
			}
		})
	}
}

func TestWechatReviewProxyOptionsReachesHandlerThroughProductionCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, versionID := "final/prod/product/2026/07/id/image.png", "version-7"
	store := validRecordingReviewProxyStore(key, versionID)
	proxy, requestURI := newReviewProxyRequestURI(t, store, key, versionID)
	router := gin.New()
	router.Use(middleware.CORS(WechatReviewContentPath))
	proxy.Register(router.Group("/api/v1/miniapp/media"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodOptions, requestURI, nil))
	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != "HEAD, GET" {
		t.Fatalf("OPTIONS status=%d allow=%q", recorder.Code, recorder.Header().Get("Allow"))
	}
}

func TestWechatReviewProxyResponsesDoNotLeakSensitiveValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key, versionID := "final/prod/product/2026/07/private-image.png", "sensitive-version-7"
	storageText := "storage diagnostics must remain private"
	store := validRecordingReviewProxyStore(key, versionID)
	store.headErr = errors.New(storageText)
	proxy, requestURI := newReviewProxyRequestURI(t, store, key, versionID)
	parsed, err := url.Parse(requestURI)
	if err != nil {
		t.Fatal(err)
	}
	token := parsed.Query().Get("token")
	recorder := serveReviewProxyRequest(proxy, http.MethodGet, requestURI)
	if recorder.Code != http.StatusServiceUnavailable || recorder.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", recorder.Code, recorder.Body.String())
	}
	visible := recorder.Body.String() + recorder.Header().Get("Content-Type") + recorder.Header().Get("Location")
	for _, secret := range []string{token, key, versionID, storageText, "wechat-app-secret-at-least-sixteen"} {
		if secret != "" && strings.Contains(visible, secret) {
			t.Fatalf("response leaked sensitive value %q", secret)
		}
	}
}

func newReviewProxyRequestURI(
	t *testing.T,
	store WechatReviewContentStore,
	key, versionID string,
) (*WechatReviewProxy, string) {
	t.Helper()
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	proxy, err := NewWechatReviewProxy(WechatReviewProxyConfig{
		PublicBaseURL: "https://api.example.test",
		AppSecret:     "wechat-app-secret-at-least-sixteen",
	}, store)
	if err != nil {
		t.Fatal(err)
	}
	proxy.now = func() time.Time { return now }
	value, err := proxy.SignedGetURL(context.Background(), key, versionID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatal(err)
	}
	return proxy, parsed.RequestURI()
}

func validRecordingReviewProxyStore(key, versionID string) *recordingReviewProxyStore {
	payload := []byte("safe-png-test-payload")
	return &recordingReviewProxyStore{
		payload: payload,
		object: coremedia.CompletionObject{
			Key: key, VersionID: versionID, ContentType: "image/png", Size: uint64(len(payload)),
		},
	}
}

func serveReviewProxyRequest(proxy *WechatReviewProxy, method, requestURI string) *httptest.ResponseRecorder {
	router := gin.New()
	proxy.Register(router.Group("/api/v1/miniapp/media"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(method, requestURI, nil))
	return recorder
}

func toggleTokenByte(token string) string {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) == 0 {
		return "invalid-tampered-token"
	}
	raw[len(raw)/2] ^= 0x01
	return base64.RawURLEncoding.EncodeToString(raw)
}

func sealRawReviewProxyToken(t *testing.T, codec *wechatReviewProxyCodec, plaintext []byte) string {
	t.Helper()
	nonce := make([]byte, codec.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	sealed := codec.aead.Seal(nil, nonce, plaintext, []byte(wechatReviewProxyAAD))
	raw := append(nonce, sealed...)
	return base64.RawURLEncoding.EncodeToString(raw)
}
