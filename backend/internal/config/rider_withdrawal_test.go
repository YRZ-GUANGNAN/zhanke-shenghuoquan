package config

import "testing"

func TestRiderWithdrawalRequiresDeliveryAndExcludesAutomaticQueue(t *testing.T) {
	cfg := validConfig()
	cfg.EnableDeliveryRiderWithdrawal = true
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.EnableDeliveryRiderPayoutQueue = true
	if err := cfg.Validate(); err == nil {
		t.Fatal("simultaneous withdrawal and queue accepted")
	}
	cfg.EnableDeliveryRiderPayoutQueue = false
	cfg.EnableDelivery = false
	if err := cfg.Validate(); err == nil {
		t.Fatal("withdrawal without delivery accepted")
	}
}
