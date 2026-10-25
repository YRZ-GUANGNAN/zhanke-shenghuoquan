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

func TestPaymentObservationRejectsAmountAndPayerMismatchWithoutDatabase(t *testing.T) {
	payer := "openid-merchant"
	attempt := PaymentAttempt{
		Provider: PaymentWechat, AmountCents: 150, PayerOpenID: &payer,
	}
	valid := PaymentConfirmation{AmountCents: 150, PayerOpenID: payer}
	if err := validatePaymentObservation(attempt, valid); err != nil {
		t.Fatalf("valid payment observation rejected: %v", err)
	}

	wrongAmount := valid
	wrongAmount.AmountCents++
	if !errors.Is(validatePaymentObservation(attempt, wrongAmount), ErrPaymentAmount) {
		t.Fatal("payment observation accepted a mismatched amount")
	}

	wrongPayer := valid
	wrongPayer.PayerOpenID = "openid-other"
	if !errors.Is(validatePaymentObservation(attempt, wrongPayer), ErrPaymentPayer) {
		t.Fatal("payment observation accepted a mismatched payer")
	}

	missingPayer := attempt
	missingPayer.PayerOpenID = nil
	if !errors.Is(validatePaymentObservation(missingPayer, valid), ErrPaymentPayer) {
		t.Fatal("payment observation trusted an attempt without a payer snapshot")
	}
}

func TestRefundObservationRejectsSourceNumberAndAmountMismatchWithoutDatabase(t *testing.T) {
	outTradeNo := "D3-source-order"
	wechatRefundID := "refund-wechat-1"
	refund := Refund{
		Source: SettlementWechat, OutTradeNo: &outTradeNo,
		AmountCents: 150, WechatRefundID: &wechatRefundID,
	}
	valid := RefundObservation{
		OutTradeNo: outTradeNo, WechatRefundID: wechatRefundID,
		AmountCents: 150, ProviderStatus: RefundProviderProcessing,
	}
	if err := validateRefundObservation(refund, valid); err != nil {
		t.Fatalf("valid refund observation rejected: %v", err)
	}

	cases := []struct {
		name   string
		refund Refund
		input  RefundObservation
	}{
		{name: "wrong source", refund: func() Refund { value := refund; value.Source = SettlementWallet; return value }(), input: valid},
		{name: "missing original transaction", refund: func() Refund { value := refund; value.OutTradeNo = nil; return value }(), input: valid},
		{name: "wrong original transaction", refund: refund, input: func() RefundObservation { value := valid; value.OutTradeNo = "D3-other"; return value }()},
		{name: "wrong amount", refund: refund, input: func() RefundObservation { value := valid; value.AmountCents++; return value }()},
		{name: "wrong refund id", refund: refund, input: func() RefundObservation { value := valid; value.WechatRefundID = "refund-wechat-2"; return value }()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !errors.Is(validateRefundObservation(tc.refund, tc.input), ErrRefundObservation) {
				t.Fatal("refund observation mismatch was accepted")
			}
		})
	}
}
