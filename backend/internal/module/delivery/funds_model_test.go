package delivery

import (
	"errors"
	"testing"
	"time"
)

func TestPaymentAttemptValidationRejectsCrossPurposeReferences(t *testing.T) {
	now := time.Now().UTC()
	paymentID, _ := NewULID()
	requestID, _ := NewULID()
	orderID, _ := NewULID()
	base := PaymentAttempt{
		PaymentPublicID: paymentID, MerchantUserID: 1, Purpose: PaymentOrderFee,
		DeliveryOrderPublicID: &orderID, Provider: PaymentWechat, Status: PaymentCreated,
		AmountCents: 150, Currency: "CNY", ClientRequestID: requestID,
		OutTradeNo: "D3-test", RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := validatePaymentAttempt(base); err != nil {
		t.Fatalf("valid order fee attempt rejected: %v", err)
	}
	recharge := base
	recharge.Purpose = PaymentWalletRecharge
	if !errors.Is(validatePaymentAttempt(recharge), ErrInvalidRecord) {
		t.Fatal("wallet recharge accepted an order reference")
	}
	orderFee := base
	orderFee.DeliveryOrderPublicID = nil
	if !errors.Is(validatePaymentAttempt(orderFee), ErrInvalidRecord) {
		t.Fatal("order fee accepted a missing order reference")
	}
}
