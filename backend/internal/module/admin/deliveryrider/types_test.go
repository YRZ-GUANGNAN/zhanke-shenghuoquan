package deliveryrider

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
)

func TestAssignedViewProjectsStageTimestampsAndKeepsMissingStagesNull(t *testing.T) {
	ready := time.Date(2026, 6, 29, 10, 1, 0, 0, time.UTC)
	claimed := ready.Add(time.Minute)
	value := delivery.Order{
		ReadyForClaimAt: &ready,
		ClaimedAt:       &claimed,
	}

	view := assignedView(value)
	if view.ReadyForClaimAt != &ready || view.ClaimedAt != &claimed {
		t.Fatalf("projected stage times=%#v", view)
	}
	if view.ToPickupAt != nil || view.PickedUpAt != nil || view.DeliveringAt != nil ||
		view.DeliveredAt != nil || view.PaymentFailedAt != nil || view.CancelledAt != nil {
		t.Fatalf("missing stages must remain nil: %#v", view)
	}

	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{
		`"toPickupAt":null`, `"pickedUpAt":null`, `"deliveringAt":null`,
		`"deliveredAt":null`, `"paymentFailedAt":null`, `"cancelledAt":null`,
	} {
		if !strings.Contains(string(encoded), field) {
			t.Fatalf("missing nullable field %s in %s", field, encoded)
		}
	}
}
