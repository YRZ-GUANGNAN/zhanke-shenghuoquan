package delivery

import (
	"testing"
	"time"
)

func TestRiderAvailabilitySnapshotValidation(t *testing.T) {
	now := time.Now().UTC()
	value := RiderAvailabilitySnapshot{RiderUserID: 1, State: RiderOffDuty, Scope: "campus", MaxInFlight: 3}
	if !value.Valid() {
		t.Fatal("missing row must have a valid read-only off-duty snapshot")
	}
	value.State = RiderOnDuty
	if value.Valid() {
		t.Fatal("version zero cannot announce on duty")
	}
	value.RowVersion, value.UpdatedAt, value.InFlight = 1, now, 5
	if !value.Valid() {
		t.Fatal("lowering the limit cannot make existing assigned work unreadable")
	}
	for _, max := range []uint32{0, 101} {
		value.MaxInFlight = max
		if value.Valid() {
			t.Fatal("capacity outside 1..100 accepted")
		}
	}
}

func TestRiderAvailabilityConfigValidation(t *testing.T) {
	now := time.Now().UTC()
	value := RiderBroadcastConfig{ConfigScope: "global", MaxInFlightPerRider: 3, RowVersion: 1, CreatedAt: now, UpdatedAt: now}
	if !value.Valid() {
		t.Fatal("default global config rejected")
	}
	value.ConfigScope = "other"
	if value.Valid() {
		t.Fatal("unknown scope accepted")
	}
}
