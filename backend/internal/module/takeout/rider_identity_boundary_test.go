package takeout

import (
	"os"
	"strings"
	"testing"
)

func TestRiderIdentityNarrowServiceDoesNotReactivateLegacyOrderRuntime(t *testing.T) {
	for _, file := range []string{"rider_identity.go", "rider_identity_repository.go"} {
		body, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		source := string(body)
		for _, forbidden := range []string{
			"RiderOrderOps", "OrderService", "PaymentService", "CatalogService",
			"ClaimOrder(", "CompleteFulfillment(", "TransitionFulfillment",
		} {
			if strings.Contains(source, forbidden) {
				t.Errorf("%s references legacy runtime capability %s", file, forbidden)
			}
		}
	}
}
