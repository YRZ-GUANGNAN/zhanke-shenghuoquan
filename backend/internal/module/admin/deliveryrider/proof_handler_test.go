package deliveryrider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"campus-circle/backend/internal/module/delivery"
	"github.com/gin-gonic/gin"
)

type proofCommandStub struct {
	input delivery.ProofTransitionInput
}

func (s *proofCommandStub) AdvanceWithProof(_ context.Context, input delivery.ProofTransitionInput) (delivery.Order, delivery.Proof, error) {
	s.input = input
	return delivery.Order{
		DeliveryOrderPublicID: input.OrderPublicID, RiderUserID: &input.RiderUserID,
		Status: delivery.StatusPickedUp, RowVersion: input.ExpectedRowVersion + 1,
	}, delivery.Proof{ProofType: input.ProofType, AssetPublicID: input.AssetPublicID}, nil
}

func TestPickedUpRequiresProofAndDelegatesWithoutBypass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	commands := &proofCommandStub{}
	handler := NewHandler(nil, nil, nil, func(*gin.Context) uint64 { return 77 }, commands)
	router := gin.New()
	handler.Register(router.Group("/rider"))
	orderID := "01J00000000000000000000001"

	missing := httptest.NewRecorder()
	router.ServeHTTP(missing, httptest.NewRequest(
		http.MethodPatch, "/rider/orders/"+orderID+"/picked-up",
		strings.NewReader(`{"expectedRowVersion":3}`),
	))
	if missing.Code != http.StatusBadRequest || commands.input.OrderPublicID != "" {
		t.Fatalf("missing proof status=%d input=%#v", missing.Code, commands.input)
	}

	present := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPatch, "/rider/orders/"+orderID+"/picked-up",
		strings.NewReader(`{"expectedRowVersion":3,"proofAssetPublicId":"01J00000000000000000000002"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(present, request)
	if present.Code != http.StatusOK || commands.input.ProofType != delivery.ProofPickup ||
		commands.input.RiderUserID != 77 || commands.input.ExpectedRowVersion != 3 {
		t.Fatalf("present proof status=%d input=%#v body=%s", present.Code, commands.input, present.Body.String())
	}
}

func TestPickedUpFailsClosedWhenProofServiceUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil, nil, func(*gin.Context) uint64 { return 77 })
	router := gin.New()
	handler.Register(router.Group("/rider"))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodPatch, "/rider/orders/01J00000000000000000000001/picked-up",
		strings.NewReader(`{"expectedRowVersion":3,"proofAssetPublicId":"01J00000000000000000000002"}`),
	))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
