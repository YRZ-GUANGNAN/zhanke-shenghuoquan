package deliveryconsole

import (
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
)

func TestDeliveryConsoleErrorCodeIsRedacted(t *testing.T) {
	for _, test := range []struct {
		name string
		in   string
		want string
	}{
		{name: "known", in: "refund_closed", want: "refund_closed"},
		{name: "unknown provider text", in: "provider rejected: secret response", want: "provider_error"},
		{name: "empty", in: "", want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := safeDeliveryErrorCode(test.in); got != test.want {
				t.Fatalf("safeDeliveryErrorCode(%q)=%q want=%q", test.in, got, test.want)
			}
		})
	}
}

func TestDeliveryConsoleViewsExposeVerificationMetadataWithoutRawError(t *testing.T) {
	now := time.Now().UTC()
	payment := paymentView(delivery.PaymentAttempt{
		PaymentPublicID:    "01J0000000000000000000D6B1",
		MerchantUserID:     7,
		ProviderStatus:     "USERPAYING",
		NextVerifyAt:       &now,
		VerifyAttemptCount: 2,
		LastErrorCode:      "provider rejected: raw",
	})
	if payment.ProviderStatus != "USERPAYING" || payment.NextVerifyAt == nil ||
		payment.VerifyAttemptCount != 2 || payment.ErrorCode != "provider_error" {
		t.Fatalf("payment view=%#v", payment)
	}
	refund := refundView(delivery.Refund{
		RefundPublicID:     "01J0000000000000000000D6B2",
		ProviderStatus:     "PROCESSING",
		NextVerifyAt:       &now,
		VerifyAttemptCount: 3,
		LastErrorCode:      "refund_closed",
	})
	if refund.ProviderStatus != "PROCESSING" || refund.NextVerifyAt == nil ||
		refund.VerifyAttemptCount != 3 || refund.ErrorCode != "refund_closed" {
		t.Fatalf("refund view=%#v", refund)
	}
}
