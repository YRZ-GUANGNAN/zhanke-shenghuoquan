package miniappflow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	coremedia "campus-circle/backend/internal/module/media"
	"github.com/gin-gonic/gin"
)

func TestGenericResolveExplicitlyRejectsPrivatePurposes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	NewHandler(nil, func(*gin.Context) uint64 { return 1 }, &fakeDisplayService{}).Register(router.Group("/miniapp/media"))
	for _, purpose := range []coremedia.Purpose{coremedia.PurposeDeliveryProof, coremedia.PurposeDeliveryIM} {
		for _, endpoint := range []string{
			"/miniapp/media/resolve/" + string(purpose) + "/01J00000000000000000000001/0",
			"/miniapp/media/display/" + string(purpose) + "/01J00000000000000000000001/0/content",
		} {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, endpoint, nil))
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("%s status=%d body=%s", endpoint, recorder.Code, recorder.Body.String())
			}
		}
		if !purpose.Valid() || purpose.PubliclyResolvable() {
			t.Fatalf("%s must be valid for upload but private for generic resolve", purpose)
		}
	}
}

type fakeDisplayService struct{}

func (*fakeDisplayService) ContentURL(_ context.Context, _ coremedia.Purpose, _ string, _ uint16) (string, error) {
	return "https://should-not-be-called.example", nil
}
