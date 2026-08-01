package deliverymerchant

import (
	"reflect"
	"testing"

	"campus-circle/backend/internal/module/delivery"
)

func TestMerchantStatusesForGroup(t *testing.T) {
	tests := []struct {
		name     string
		group    string
		expected []delivery.Status
		valid    bool
	}{
		{name: "backward compatible empty", group: "", valid: true},
		{name: "pending", group: "pending", expected: []delivery.Status{
			delivery.StatusPendingFeePayment,
			delivery.StatusPaymentFailed,
			delivery.StatusReadyForClaim,
		}, valid: true},
		{name: "delivering", group: "delivering", expected: []delivery.Status{
			delivery.StatusClaimed,
			delivery.StatusToPickup,
			delivery.StatusPickedUp,
			delivery.StatusDelivering,
		}, valid: true},
		{name: "completed", group: "completed", expected: []delivery.Status{
			delivery.StatusDelivered,
			delivery.StatusCancelled,
		}, valid: true},
		{name: "invalid", group: "unknown", valid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual, valid := merchantStatusesForGroup(test.group)
			if valid != test.valid || !reflect.DeepEqual(actual, test.expected) {
				t.Fatalf("merchantStatusesForGroup(%q) = (%v, %v), want (%v, %v)", test.group, actual, valid, test.expected, test.valid)
			}
		})
	}
}
