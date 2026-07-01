package delivery

import "testing"

func TestConversationStateMachineMatrix(t *testing.T) {
	allowed := map[ConversationStatus][]ConversationStatus{
		ConversationProvisioning: {ConversationActive, ConversationFailed, ConversationRevoking, ConversationRevoked},
		ConversationFailed:       {ConversationActive, ConversationFailed, ConversationRevoking, ConversationRevoked},
		ConversationActive:       {ConversationReadOnly, ConversationRevoking},
		ConversationReadOnly:     {ConversationRevoking},
		ConversationRevoking:     {ConversationRevoked},
		ConversationRevoked:      {},
	}
	statuses := []ConversationStatus{
		ConversationProvisioning, ConversationActive, ConversationReadOnly,
		ConversationRevoking, ConversationRevoked, ConversationFailed,
	}
	for _, from := range statuses {
		for _, to := range statuses {
			want := containsConversationStatus(allowed[from], to)
			if got := CanTransitionConversation(from, to); got != want {
				t.Fatalf("%s -> %s got=%v want=%v", from, to, got, want)
			}
		}
	}
	if CanTransitionConversation("unknown", ConversationActive) ||
		CanTransitionConversation(ConversationActive, "unknown") {
		t.Fatal("unknown conversation status must fail closed")
	}
}

func TestConversationOrderLifecyclePolicy(t *testing.T) {
	for _, status := range []Status{StatusClaimed, StatusToPickup, StatusPickedUp, StatusDelivering} {
		if !orderAllowsConversationSend(status) {
			t.Fatalf("%s should allow conversation send", status)
		}
	}
	for _, status := range []Status{StatusDelivered, StatusCancelled} {
		if !orderConversationReadOnly(status) || orderAllowsConversationSend(status) {
			t.Fatalf("%s should be read-only", status)
		}
	}
	if orderAllowsConversationSend(StatusReadyForClaim) || orderConversationReadOnly(StatusReadyForClaim) {
		t.Fatal("ready_for_claim must have no conversation")
	}
}

func containsConversationStatus(values []ConversationStatus, target ConversationStatus) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
