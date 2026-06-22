package media

import "testing"

func TestTakeoutPurposesAreValidAndFitFrozenApplicationSize(t *testing.T) {
	for _, purpose := range []Purpose{PurposeDish, PurposeMerchantLogo} {
		if !purpose.Valid() {
			t.Errorf("takeout purpose %q is invalid", purpose)
		}
		if len(purpose) > 16 {
			t.Errorf("takeout purpose %q exceeds size:16", purpose)
		}
	}
	if len(PurposeMerchantLogo) != 13 {
		t.Fatalf("merchant_logo length=%d want=13", len(PurposeMerchantLogo))
	}
	for _, invalid := range []Purpose{"", "merchant", "dish_image", "merchant-logo"} {
		if invalid.Valid() {
			t.Errorf("unexpected valid purpose %q", invalid)
		}
	}
}
