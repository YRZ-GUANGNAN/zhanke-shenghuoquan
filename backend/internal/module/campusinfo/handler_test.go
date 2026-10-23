package campusinfo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPublishedGuidesReadWithoutCommunityOrLogin(t *testing.T) {
	h, err := NewHandler()
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	h.Register(router.Group("/api/v1"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/campus-info/guides", nil))
	var result struct {
		Data struct {
			Items []Guide `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK || len(result.Data.Items) != 5 {
		t.Fatalf("unexpected published content: %s", w.Body.String())
	}
	for _, guide := range result.Data.Items {
		if len(guide.Sections) < 3 || guide.VerifiedAt != "2026-10-13" {
			t.Fatalf("incomplete guide %s", guide.ID)
		}
		w = httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/campus-info/guides/"+guide.ID, nil))
		if w.Code != http.StatusOK {
			t.Fatalf("missing detail %s", guide.ID)
		}
	}
	for _, id := range []string{"missing", "1", "library-hours%3Ftoken%3Dsecret"} {
		w = httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/campus-info/guides/"+id, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("unknown id returned %d", w.Code)
		}
	}
}
