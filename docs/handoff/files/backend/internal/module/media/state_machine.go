package media

import "errors"

var ErrInvalidTransition = errors.New("invalid media state transition")

var technicalTransitions = map[TechnicalStatus]map[TechnicalStatus]struct{}{
	TechnicalPendingUpload:    setOf(TechnicalUploaded, TechnicalDeletePending),
	TechnicalUploaded:         setOf(TechnicalValidating),
	TechnicalValidating:       setOf(TechnicalPromoting, TechnicalDeletePending),
	TechnicalPromoting:        setOf(TechnicalFinalReady, TechnicalDeletePending),
	TechnicalFinalReady:       setOf(TechnicalSubmitted),
	TechnicalSubmitted:        setOf(TechnicalReviewing, TechnicalReviewFailed),
	TechnicalReviewing:        setOf(TechnicalApproved, TechnicalRejected, TechnicalReviewFailed),
	TechnicalApproved:         setOf(TechnicalDeletePending),
	TechnicalRejected:         setOf(TechnicalDeletePending),
	TechnicalReviewFailed:     setOf(TechnicalDeletePending),
	TechnicalDeletePending:    setOf(TechnicalDeleted, TechnicalDeleteDeadletter),
	TechnicalDeleteDeadletter: setOf(TechnicalDeleted),
	TechnicalDeleted:          {},
}

var referenceTransitions = map[ReferenceStatus]map[ReferenceStatus]struct{}{
	ReferencePending:  setOf(ReferenceActive, ReferenceRevoking, ReferenceRevoked),
	ReferenceActive:   setOf(ReferenceRevoking),
	ReferenceRevoking: setOf(ReferenceRevoked, ReferenceActive),
	ReferenceRevoked:  {},
}

var ticketTransitions = map[TicketStatus]map[TicketStatus]struct{}{
	TicketCreated:             setOf(TicketAuthorizationIssued, TicketExpired, TicketCancelled),
	TicketAuthorizationIssued: setOf(TicketUploaded, TicketExpired, TicketCancelled),
	TicketUploaded:            setOf(TicketConsumed, TicketExpired, TicketCancelled),
	TicketConsumed:            {},
	TicketExpired:             {},
	TicketCancelled:           {},
}

var reservationTransitions = map[ReservationStatus]map[ReservationStatus]struct{}{
	ReservationActive:    setOf(ReservationCommitted, ReservationReleased),
	ReservationCommitted: {},
	ReservationReleased:  {},
}

func CanTransitionTechnical(from, to TechnicalStatus) bool {
	return contains(technicalTransitions[from], to)
}
func CanTransitionReference(from, to ReferenceStatus) bool {
	return contains(referenceTransitions[from], to)
}
func CanTransitionTicket(from, to TicketStatus) bool { return contains(ticketTransitions[from], to) }
func CanTransitionReservation(from, to ReservationStatus) bool {
	return contains(reservationTransitions[from], to)
}

func CanTransitionStorage(from, to StorageStage) bool {
	return from == StorageStageStaging && to == StorageStageFinal
}

func CanTransitionReview(from, to ReviewStatus) bool {
	if from != ReviewPending {
		return false
	}
	return to == ReviewApproved || to == ReviewRejected || to == ReviewFailed
}

func setOf[T comparable](values ...T) map[T]struct{} {
	result := make(map[T]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func contains[T comparable](values map[T]struct{}, value T) bool {
	_, ok := values[value]
	return ok
}
