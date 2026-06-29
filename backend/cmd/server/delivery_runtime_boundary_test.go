package main

import (
	"os"
	"strings"
	"testing"
)

func TestDeliveryRuntimeDoesNotConstructLegacyTakeoutServices(t *testing.T) {
	body, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	start := strings.Index(source, "func newDeliveryRuntime(")
	if start < 0 {
		t.Fatal("delivery runtime composition function not found")
	}
	end := strings.Index(source[start:], "func registerDeliveryRoutes(")
	if end < 0 {
		t.Fatal("delivery runtime composition functions not found")
	}
	block := source[start : start+end]
	for _, forbidden := range []string{
		"NewRiderOrderOps", "NewRiderService", "NewOrderService",
		"NewPaymentService", "NewCatalogService", "RegisterTarget",
	} {
		if strings.Contains(block, forbidden) {
			t.Errorf("delivery runtime constructs legacy capability %s", forbidden)
		}
	}
	if !strings.Contains(block, "NewRiderIdentityService") ||
		!strings.Contains(block, "if !cfg.EnableDelivery") {
		t.Fatal("delivery runtime lacks identity-only adapter or fail-closed gate")
	}
}
