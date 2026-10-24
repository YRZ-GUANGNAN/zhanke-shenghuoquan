package deliverypayment

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"campus-circle/backend/internal/module/delivery"
	"github.com/gin-gonic/gin"
)

type terminalGuardProvider struct{ prepayCalls int }

func (p *terminalGuardProvider) Provider() delivery.PaymentProvider { return delivery.PaymentWechat }
func (p *terminalGuardProvider) CreatePrepay(context.Context, PrepayRequest) (PrepayResult, error) {
	p.prepayCalls++
	return PrepayResult{}, errors.New("must not be called")
}
func (*terminalGuardProvider) ClientParams(string) (*JSAPIParameter, error) {
	return nil, errors.New("must not be called")
}
func (*terminalGuardProvider) ParsePaymentNotification(*http.Request, []byte) (PaymentNotificationResult, error) {
	return PaymentNotificationResult{}, errors.New("unused")
}
func (*terminalGuardProvider) CreateRefund(context.Context, RefundRequest) (RefundProviderResult, error) {
	return RefundProviderResult{}, errors.New("unused")
}
func (*terminalGuardProvider) ParseRefundNotification(*http.Request, []byte) (RefundNotificationResult, error) {
	return RefundNotificationResult{}, errors.New("unused")
}

func TestIssuePrepayRejectsTerminalPaymentAttempts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, status := range []delivery.PaymentStatus{delivery.PaymentPaid, delivery.PaymentClosed, delivery.PaymentFailed} {
		t.Run(string(status), func(t *testing.T) {
			provider := &terminalGuardProvider{}
			handler := &Handler{provider: provider}
			value := delivery.PaymentStartResult{Attempt: delivery.PaymentAttempt{Status: status}}
			context, _ := gin.CreateTestContext(httptest.NewRecorder())
			if _, err := handler.issuePrepay(context, &value, "配送服务费"); !errors.Is(err, delivery.ErrPaymentNotPending) {
				t.Fatalf("status %s returned %v", status, err)
			}
			if provider.prepayCalls != 0 {
				t.Fatalf("status %s called provider %d times", status, provider.prepayCalls)
			}
		})
	}
}
