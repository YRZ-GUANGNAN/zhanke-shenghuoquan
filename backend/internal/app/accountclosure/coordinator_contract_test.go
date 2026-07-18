package accountclosure

import (
	"os"
	"strings"
	"testing"
)

func TestCoordinatorKeepsAtomicBlockerAndAnonymizationContract(t *testing.T) {
	bytes, err := os.ReadFile("coordinator.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(bytes)
	fragments := []string{
		"Transaction(func(tx *gorm.DB)", "clause.Locking{Strength: \"UPDATE\"}",
		"delivery_orders", "delivery_payment_attempts", "delivery_fee_settlements", "delivery_refunds",
		"delivery_merchant_wallets", "delivery_rider_wallets", "delivery_rider_earning_records",
		"takeout_orders", "takeout_payment_attempts", "takeout_withdraw_requests", "takeout_rider_earning_records",
		"账号已注销", "token_version", "community_search_histories", "trade_search_histories",
		"navigation_search_histories", "user_follows", "status <> ?", "takeout_merchants", "takeout_riders",
	}
	for _, fragment := range fragments {
		if !strings.Contains(source, fragment) {
			t.Errorf("missing account closure contract fragment %q", fragment)
		}
	}
}
