package deliveryrider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/takeout"
	"github.com/gin-gonic/gin"
)

const withdrawalTestID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

type withdrawalRepositoryStub struct {
	rider, amount, version, operator uint64
	writes                           int
	reference, key                   string
	err                              error
}

func (s *withdrawalRepositoryStub) value() delivery.RiderWithdrawal {
	operator, key := uint64(99), "SECRET-PROCESS-KEY"
	return delivery.RiderWithdrawal{WithdrawPublicID: withdrawalTestID, RiderUserID: 77, AmountCents: 120, Status: delivery.RiderWithdrawalProcessing, RowVersion: 1, WalletPublicID: "SECRET-WALLET", IdempotencyKey: "SECRET-KEY", ProcessedBy: &operator, ProcessKey: &key, PaymentReference: "SECRET-PAYMENT"}
}
func (s *withdrawalRepositoryStub) CreateRiderWithdrawal(_ context.Context, rider, amount uint64, key string, _ time.Time) (delivery.RiderWithdrawal, error) {
	s.rider, s.amount, s.key = rider, amount, key
	s.writes++
	return s.value(), s.err
}
func (s *withdrawalRepositoryStub) ListRiderWithdrawalsByRider(_ context.Context, rider uint64, _ delivery.Page) ([]delivery.RiderWithdrawal, int64, error) {
	s.rider = rider
	return []delivery.RiderWithdrawal{s.value()}, 1, s.err
}
func (s *withdrawalRepositoryStub) ListRiderWithdrawals(context.Context, string, uint64, delivery.Page) ([]delivery.RiderWithdrawal, int64, error) {
	return []delivery.RiderWithdrawal{s.value()}, 1, s.err
}
func (s *withdrawalRepositoryStub) ApproveRiderWithdrawal(_ context.Context, _ string, version, operator uint64, key, reference string, _ time.Time) (delivery.RiderWithdrawal, error) {
	s.version, s.operator, s.key, s.reference = version, operator, key, reference
	s.writes++
	return s.value(), s.err
}
func (s *withdrawalRepositoryStub) RejectRiderWithdrawal(_ context.Context, _ string, version, operator uint64, key, reason string, _ time.Time) (delivery.RiderWithdrawal, error) {
	s.version, s.operator, s.key, s.reference = version, operator, key, reason
	s.writes++
	return s.value(), s.err
}

func withdrawalTestRouter(stub *withdrawalRepositoryStub, enabled bool, status takeout.RiderStatus) *gin.Engine {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil, nil, func(*gin.Context) uint64 { return 77 }).WithWithdrawals(stub)
	// Test-only opt-in. No production setter or environment flag can bypass the gate.
	handler.withdrawalWritesReady = enabled
	handler.identity = todayStatsIdentityStub{rider: takeout.Rider{RiderUserID: 77, Status: status}}
	router := gin.New()
	handler.Register(router.Group("/rider"))
	handler.RegisterAdmin(router.Group("/admin"))
	return router
}
func withdrawalRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	result := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(result, request)
	return result
}
func TestWithdrawalProductionWritesFailClosed(t *testing.T) {
	stub := &withdrawalRepositoryStub{}
	router := withdrawalTestRouter(stub, false, takeout.RiderActive)
	for _, path := range []string{"/rider/wallet/withdrawals", "/admin/rider-withdrawals/" + withdrawalTestID + "/approve", "/admin/rider-withdrawals/" + withdrawalTestID + "/reject"} {
		if result := withdrawalRequest(router, http.MethodPost, path, `{}`); result.Code != http.StatusForbidden {
			t.Fatalf("%s: %d", path, result.Code)
		}
	}
	if stub.writes != 0 {
		t.Fatal("production wrote funds before allocation safety was completed")
	}
}
func TestWithdrawalSessionOwnerAndDTO(t *testing.T) {
	stub := &withdrawalRepositoryStub{}
	router := withdrawalTestRouter(stub, true, takeout.RiderActive)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		body := fmt.Sprintf(`{"riderUserId":999,"amountCents":120,"idempotencyKey":%q}`, withdrawalTestID)
		result := withdrawalRequest(router, method, "/rider/wallet/withdrawals?riderUserId=999", body)
		if result.Code != http.StatusOK && result.Code != http.StatusCreated {
			t.Fatal(result.Code, result.Body.String())
		}
		if stub.rider != 77 {
			t.Fatal("client supplied rider bypassed session ownership")
		}
		for _, forbidden := range []string{"SECRET", "walletPublicId", "idempotencyKey", "processedBy", "processKey", "paymentReference"} {
			if strings.Contains(result.Body.String(), forbidden) {
				t.Fatalf("leaked %s", forbidden)
			}
		}
	}
}
func TestWithdrawalErrorMappingAndOperator(t *testing.T) {
	stub := &withdrawalRepositoryStub{}
	router := withdrawalTestRouter(stub, true, takeout.RiderActive)
	for _, test := range []struct {
		err    error
		status int
	}{
		{delivery.ErrWalletInsufficient, 422}, {delivery.ErrIdempotencyConflict, 409}, {delivery.ErrConcurrentUpdate, 409}, {delivery.ErrWithdrawalNotFound, 404}, {delivery.ErrInvalidRecord, 400}, {delivery.ErrWithdrawalPaymentRequired, 400},
	} {
		stub.err = test.err
		result := withdrawalRequest(router, http.MethodPost, "/rider/wallet/withdrawals", `{"amountCents":120,"idempotencyKey":"`+withdrawalTestID+`"}`)
		if result.Code != test.status {
			t.Fatal(test.err, result.Code, result.Body.String())
		}
	}
	stub.err = nil
	result := withdrawalRequest(router, http.MethodPost, "/admin/rider-withdrawals/"+withdrawalTestID+"/approve", `{"expectedRowVersion":2,"operatorUserId":999,"idempotencyKey":"`+withdrawalTestID+`","paymentReference":"BANK-123"}`)
	if result.Code != 200 || stub.operator != 77 || stub.version != 2 || stub.reference != "BANK-123" {
		t.Fatalf("operator/reference mismatch: %+v %s", stub, result.Body.String())
	}
}
