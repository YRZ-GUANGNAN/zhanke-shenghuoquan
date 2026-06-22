package takeout

import (
	"testing"
	"time"
)

func TestRiderModels(t *testing.T) {
	now := time.Date(2026, 6, 22, 16, 0, 0, 0, time.UTC)
	rider := Rider{RiderUserID: 1, Status: RiderActive, RowVersion: 1, CreatedAt: now, UpdatedAt: now}
	if err := rider.ValidateNew(); err != nil {
		t.Fatalf("valid rider rejected: %v", err)
	}
	rider.Status = RiderResting
	if err := rider.ValidateNew(); err == nil {
		t.Fatal("new rider with non-active status accepted")
	}
	assignedAt := now
	fulfillment := Fulfillment{
		FulfillmentPublicID: "01J00000000000000000000301",
		OrderPublicID:       "01J00000000000000000000302",
		RiderUserID:         1,
		Status:              FulfillmentAssigned,
		AssignedAt:          &assignedAt,
		RowVersion:          1,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := fulfillment.ValidateNew(); err != nil {
		t.Fatalf("valid fulfillment rejected: %v", err)
	}
	fulfillment.Status = FulfillmentToStore
	if err := fulfillment.ValidateNew(); err == nil {
		t.Fatal("new fulfillment with non-assigned status accepted")
	}
	if !(RiderConfig{ConfigScope: "global", MaxInFlightPerRider: 1000, RowVersion: 1, UpdatedAt: now}).Valid() ||
		(RiderConfig{ConfigScope: "global", MaxInFlightPerRider: 0, RowVersion: 1, UpdatedAt: now}).Valid() {
		t.Fatal("rider config validity mismatch")
	}
}
