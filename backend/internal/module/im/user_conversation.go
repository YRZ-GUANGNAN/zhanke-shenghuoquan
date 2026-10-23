package im

import (
	"context"
	"errors"
)

// The unordered user pair has one stable relation. BuyerID/SellerID are storage
// slots for this type, not trading roles. Tencent C2C history stays peer-scoped.
func (s *Service) EnsureUserConversation(ctx context.Context, viewerID, peerID uint) (ConversationContext, error) {
	if viewerID == 0 || peerID == 0 || viewerID == peerID {
		return ConversationContext{}, errors.New("invalid user conversation")
	}
	first, second := viewerID, peerID
	if first > second {
		first, second = second, first
	}
	value, found, err := s.repo.FindConversation(ctx, first, second, "user", 0)
	if err != nil {
		return ConversationContext{}, err
	}
	if found {
		return s.contextForParticipant(ctx, value, viewerID)
	}
	if s.sdkAppID == "" || s.secret == "" {
		return ConversationContext{}, errors.New("Tencent IM is not configured")
	}
	if _, err := s.ensureIMUser(ctx, first); err != nil {
		return ConversationContext{}, err
	}
	secondMapping, err := s.ensureIMUser(ctx, second)
	if err != nil {
		return ConversationContext{}, err
	}
	value = Conversation{ConversationID: "C2C" + secondMapping.IMUserID, BuyerID: first, SellerID: second, ReferenceType: "user", ReferenceID: 0}
	if err := s.repo.EnsureConversationStable(ctx, &value); err != nil {
		return ConversationContext{}, err
	}
	return s.contextForParticipant(ctx, value, viewerID)
}
