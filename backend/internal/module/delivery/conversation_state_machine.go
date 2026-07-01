package delivery

var conversationTransitions = map[ConversationStatus]map[ConversationStatus]struct{}{
	ConversationProvisioning: conversationStatusSet(ConversationActive, ConversationFailed, ConversationRevoking, ConversationRevoked),
	ConversationFailed:       conversationStatusSet(ConversationActive, ConversationFailed, ConversationRevoking, ConversationRevoked),
	ConversationActive:       conversationStatusSet(ConversationReadOnly, ConversationRevoking),
	ConversationReadOnly:     conversationStatusSet(ConversationRevoking),
	ConversationRevoking:     conversationStatusSet(ConversationRevoked),
	ConversationRevoked:      {},
}

func CanTransitionConversation(from, to ConversationStatus) bool {
	if !from.Valid() || !to.Valid() {
		return false
	}
	_, ok := conversationTransitions[from][to]
	return ok
}

func conversationStatusSet(values ...ConversationStatus) map[ConversationStatus]struct{} {
	result := make(map[ConversationStatus]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
