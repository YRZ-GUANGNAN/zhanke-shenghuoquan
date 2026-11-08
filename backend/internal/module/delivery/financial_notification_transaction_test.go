package delivery

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Notification rollback/replay runs against isolated MySQL alongside source
// allocations and claims; do not retain the obsolete wallet-only SQL mock path.
func TestRiderWithdrawalLegacyApproveCannotBypassClaim(t *testing.T) {
	repo := NewRepository(nil)
	_, err := repo.ApproveRiderWithdrawal(context.Background(), "01ARZ3NDEKTSV4RRFFQ69G5FAV", 1, 91, "01ARZ3NDEKTSV4RRFFQ69G5FAW", "test-reference", time.Now())
	if !errors.Is(err, ErrWithdrawalNotClaimed) {
		t.Fatalf("legacy approval bypass: %v", err)
	}
}

func TestRiderWithdrawalMissingSchemaFailsClosed(t *testing.T) {
	repo := NewRepository(nil)
	_, err := repo.CreateRiderWithdrawal(context.Background(), 61, 200, "01ARZ3NDEKTSV4RRFFQ69G5FAW", time.Now())
	if !errors.Is(err, ErrWithdrawalAllocationUnavailable) {
		t.Fatalf("missing schema accepted: %v", err)
	}
}
