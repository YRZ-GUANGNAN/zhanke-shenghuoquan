package miniappflow

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	coremedia "campus-circle/backend/internal/module/media"
	"github.com/gin-gonic/gin"
)

type fakeDisplayContent struct {
	value string
	err   error
}

func (f fakeDisplayContent) ContentURL(context.Context, coremedia.Purpose, string, uint16) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.value, nil
}

func TestDisplayContentRedirectsToSignedURL(t *testing.T) {
	router := displayTestRouter(fakeDisplayContent{value: "https://signed.example.test/final.webp?q-signature=short"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/display/dish/dish-1/0/content", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusFound {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if location := recorder.Header().Get("Location"); location != "https://signed.example.test/final.webp?q-signature=short" {
		t.Fatalf("location=%q", location)
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("redirect body must remain empty, got %q", recorder.Body.String())
	}
}

func TestDisplayContentFailClosedNotFound(t *testing.T) {
	for _, err := range []error{coremedia.ErrNotFound, coremedia.ErrMediaNotVisible, coremedia.ErrReviewProjectionUnavailable} {
		router := displayTestRouter(fakeDisplayContent{err: err})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/display/dish/dish-1/0/content", nil)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("err=%v status=%d body=%s", err, recorder.Code, recorder.Body.String())
		}
	}
}

func TestDisplayContentRejectsInvalidRouteParams(t *testing.T) {
	router := displayTestRouter(fakeDisplayContent{err: errors.New("must not call display service")})
	for _, path := range []string{"/display/invalid/dish-1/0/content", "/display/dish/dish-1/not-a-slot/content"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("path=%s status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
	}
}

func TestResolveContentURLReturnsSignedURLJSON(t *testing.T) {
	router := displayTestRouter(fakeDisplayContent{value: "https://signed.example.test/final.webp?q-signature=short"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/resolve/dish/dish-1/0", nil)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			URL              string `json:"url"`
			ExpiresInSeconds int64  `json:"expiresInSeconds"`
			FinalKey         string `json:"finalKey"`
			FinalVersionID   string `json:"finalVersionId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != 0 || envelope.Message != "ok" {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if envelope.Data.URL != "https://signed.example.test/final.webp?q-signature=short" {
		t.Fatalf("url=%q", envelope.Data.URL)
	}
	if envelope.Data.ExpiresInSeconds != int64(coremedia.MediaDisplayURLTTL.Seconds()) {
		t.Fatalf("ttl=%d", envelope.Data.ExpiresInSeconds)
	}
	if envelope.Data.FinalKey != "" || envelope.Data.FinalVersionID != "" {
		t.Fatalf("storage identity leaked: %+v", envelope.Data)
	}
}

func TestResolveContentURLFailClosedNotFound(t *testing.T) {
	for _, err := range []error{coremedia.ErrNotFound, coremedia.ErrMediaNotVisible, coremedia.ErrReviewProjectionUnavailable} {
		router := displayTestRouter(fakeDisplayContent{err: err})
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/resolve/dish/dish-1/0", nil)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("err=%v status=%d body=%s", err, recorder.Code, recorder.Body.String())
		}
	}
}

func TestResolveContentURLRejectsInvalidRouteParams(t *testing.T) {
	router := displayTestRouter(fakeDisplayContent{err: errors.New("must not call display service")})
	for _, path := range []string{"/resolve/invalid/dish-1/0", "/resolve/dish/dish-1/not-a-slot"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("path=%s status=%d body=%s", path, recorder.Code, recorder.Body.String())
		}
	}
}

func displayTestRouter(display displayContentService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/")
	NewHandler(nil, func(*gin.Context) uint64 { return 1 }, display).Register(group)
	return router
}
