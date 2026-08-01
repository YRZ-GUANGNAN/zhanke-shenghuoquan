package delivery

import "testing"

func TestValidStatusFilter(t *testing.T) {
	if !validStatusFilter(nil) {
		t.Fatal("empty filter must preserve the existing all-status API behavior")
	}
	if !validStatusFilter([]Status{StatusClaimed, StatusToPickup}) {
		t.Fatal("known delivery statuses must be accepted")
	}
	if validStatusFilter([]Status{StatusClaimed, Status("unknown")}) {
		t.Fatal("unknown delivery statuses must be rejected")
	}
}
