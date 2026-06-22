package media

import "testing"

func TestTechnicalStateMachineMatchesFrozenDesign(t *testing.T) {
	allowed := [][2]TechnicalStatus{
		{TechnicalPendingUpload, TechnicalUploaded},
		{TechnicalPendingUpload, TechnicalDeletePending},
		{TechnicalUploaded, TechnicalValidating},
		{TechnicalValidating, TechnicalPromoting},
		{TechnicalValidating, TechnicalDeletePending},
		{TechnicalPromoting, TechnicalFinalReady},
		{TechnicalPromoting, TechnicalDeletePending},
		{TechnicalFinalReady, TechnicalSubmitted},
		{TechnicalSubmitted, TechnicalReviewing},
		{TechnicalSubmitted, TechnicalReviewFailed},
		{TechnicalReviewing, TechnicalApproved},
		{TechnicalReviewing, TechnicalRejected},
		{TechnicalReviewing, TechnicalReviewFailed},
		{TechnicalApproved, TechnicalDeletePending},
		{TechnicalRejected, TechnicalDeletePending},
		{TechnicalReviewFailed, TechnicalDeletePending},
		{TechnicalDeletePending, TechnicalDeleted},
		{TechnicalDeletePending, TechnicalDeleteDeadletter},
		{TechnicalDeleteDeadletter, TechnicalDeleted},
	}
	for _, transition := range allowed {
		if !CanTransitionTechnical(transition[0], transition[1]) {
			t.Errorf("expected transition %s -> %s", transition[0], transition[1])
		}
	}

	for _, transition := range [][2]TechnicalStatus{
		{TechnicalPendingUpload, TechnicalApproved},
		{TechnicalApproved, TechnicalUploaded},
		{TechnicalRejected, TechnicalApproved},
		{TechnicalDeleted, TechnicalPendingUpload},
		{TechnicalDeleted, TechnicalDeleted},
	} {
		if CanTransitionTechnical(transition[0], transition[1]) {
			t.Errorf("accepted illegal transition %s -> %s", transition[0], transition[1])
		}
	}
}

func TestSupportingStateMachinesAreTerminalAndCASFriendly(t *testing.T) {
	if !CanTransitionStorage(StorageStageStaging, StorageStageFinal) || CanTransitionStorage(StorageStageFinal, StorageStageStaging) {
		t.Fatal("storage stage is not staging -> final only")
	}
	if !CanTransitionTicket(TicketCreated, TicketAuthorizationIssued) || CanTransitionTicket(TicketConsumed, TicketUploaded) {
		t.Fatal("ticket transition invariant failed")
	}
	if !CanTransitionReservation(ReservationActive, ReservationCommitted) || CanTransitionReservation(ReservationCommitted, ReservationReleased) {
		t.Fatal("reservation settlement is not terminal")
	}
	if !CanTransitionReference(ReferenceActive, ReferenceRevoking) || CanTransitionReference(ReferenceRevoked, ReferenceActive) {
		t.Fatal("reference revocation invariant failed")
	}
}
