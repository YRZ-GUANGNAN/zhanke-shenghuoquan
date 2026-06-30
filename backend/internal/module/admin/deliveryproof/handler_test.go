package deliveryproof

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"github.com/gin-gonic/gin"
)

type proofQueryStub struct {
	values []delivery.Proof
	url    string
	err    error
}

func (s proofQueryStub) ListForParticipant(context.Context, uint64, string) ([]delivery.Proof, error) {
	return s.values, s.err
}
func (s proofQueryStub) ResolveForParticipant(context.Context, uint64, string, delivery.ProofType, uint16) (string, error) {
	return s.url, s.err
}

func TestPrivateProofResolveUsesNoStoreAndDoesNotExposeAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	service := proofQueryStub{
		url: "https://signed.example/object?versionId=fixed",
		values: []delivery.Proof{{
			ProofPublicID: "01J00000000000000000000001",
			ProofType:     delivery.ProofPickup, Slot: 0, Status: delivery.ProofActive,
			AssetPublicID: "01J00000000000000000000099", CreatedAt: time.Now().UTC(),
		}},
	}
	NewHandler(service, func(*gin.Context) uint64 { return 7 }).Register(router.Group("/orders"))

	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/orders/01J00000000000000000000002/proofs", nil))
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), "asset") ||
		strings.Contains(list.Body.String(), "01J00000000000000000000099") {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	resolve := httptest.NewRecorder()
	router.ServeHTTP(resolve, httptest.NewRequest(
		http.MethodGet, "/orders/01J00000000000000000000002/proofs/pickup/0/resolve", nil,
	))
	if resolve.Code != http.StatusOK || resolve.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("resolve status=%d cache=%q body=%s", resolve.Code, resolve.Header().Get("Cache-Control"), resolve.Body.String())
	}
	var envelope struct {
		Data struct {
			URL              string `json:"url"`
			ExpiresInSeconds int64  `json:"expiresInSeconds"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resolve.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.URL != service.url || envelope.Data.ExpiresInSeconds != 60 {
		t.Fatalf("resolve=%#v", envelope.Data)
	}
}

func TestPrivateProofNonparticipantIsAlways404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, err := range []error{delivery.ErrForbiddenOrder, delivery.ErrNotFound, delivery.ErrProofNotActive} {
		router := gin.New()
		NewHandler(proofQueryStub{err: err}, func(*gin.Context) uint64 { return 99 }).Register(router.Group("/orders"))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(
			http.MethodGet, "/orders/01J00000000000000000000002/proofs/pickup/0/resolve", nil,
		))
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("err=%v status=%d body=%s", err, recorder.Code, recorder.Body.String())
		}
	}
}
