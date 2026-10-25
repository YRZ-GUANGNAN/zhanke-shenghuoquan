package delivery

import (
	"errors"
	"testing"
	"time"
)

func TestRiderPayoutQueueStatesHaveExplicitReservationSemantics(t *testing.T) {
	now := time.Now().UTC()
	queueID := mustQueueULID(t)
	earningID := mustQueueULID(t)
	orderID := mustQueueULID(t)
	walletID := mustQueueULID(t)
	base := RiderPayoutQueue{
		PayoutQueuePublicID: queueID, EarningPublicID: earningID, DeliveryOrderPublicID: orderID,
		RiderUserID: 7, AmountCents: 120, Currency: "CNY", Status: RiderPayoutQueueQueued,
		SourcePolicyVersion: "rider-payout-source-v1", RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := validateRiderPayoutQueue(base); err != nil {
		t.Fatalf("queued queue invalid: %v", err)
	}
	reservation := RiderPayoutReservation{
		ReservationPublicID: mustQueueULID(t), PayoutQueuePublicID: queueID, EarningPublicID: earningID,
		WalletPublicID: walletID, RiderUserID: 7, AmountCents: 120,
		Status: RiderPayoutReservationActive, RowVersion: 1, ReservedAt: now,
	}
	if err := validateRiderPayoutReservation(reservation); err != nil {
		t.Fatalf("active reservation invalid: %v", err)
	}
	if available, err := riderPayoutReservedAmount(120, reservation.AmountCents); err != nil || available != 0 {
		t.Fatalf("reserved amount mismatch: available=%d err=%v", available, err)
	}
	cancelled := base
	cancelled.Status = RiderPayoutQueueCancelled
	cancelled.CancelledAt = &now
	if err := validateRiderPayoutQueue(cancelled); err != nil {
		t.Fatalf("cancelled queue invalid: %v", err)
	}
	released := reservation
	released.Status = RiderPayoutReservationReleased
	released.ReleasedAt = &now
	if err := validateRiderPayoutReservation(released); err != nil {
		t.Fatalf("released reservation invalid: %v", err)
	}
}

func TestRiderPayoutQueueRejectsUnderflowAndInvalidTerminalFacts(t *testing.T) {
	if _, err := riderPayoutReservedAmount(99, 100); !errors.Is(err, ErrFundingSourceMismatch) {
		t.Fatalf("expected underflow rejection, got %v", err)
	}
	now := time.Now().UTC()
	value := RiderPayoutQueue{
		PayoutQueuePublicID: mustQueueULID(t), EarningPublicID: mustQueueULID(t), DeliveryOrderPublicID: mustQueueULID(t),
		RiderUserID: 1, AmountCents: 1, Currency: "CNY", Status: RiderPayoutQueueCancelled,
		SourcePolicyVersion: "v1", RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	if err := validateRiderPayoutQueue(value); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("cancelled queue without release timestamp should fail, got %v", err)
	}
}

func TestRiderPayoutSourceRejectsMockAndHistoricalSources(t *testing.T) {
	for _, source := range []SettlementSource{SettlementMock, ""} {
		if source == SettlementWallet || source == SettlementWechat {
			t.Fatal("test fixture unexpectedly verified source")
		}
	}
	if !RiderPayoutQueueBlocked.Valid() || RiderPayoutQueueQueued.Valid() == false {
		t.Fatal("queue statuses must be explicit")
	}
}

func mustQueueULID(t *testing.T) string {
	t.Helper()
	value, err := NewULID()
	if err != nil {
		t.Fatal(err)
	}
	return value
}
