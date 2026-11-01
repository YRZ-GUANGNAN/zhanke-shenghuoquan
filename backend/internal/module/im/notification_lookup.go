package im

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var ErrNotificationIdentityUnavailable = errors.New("IM notification identity unavailable")

// ResolveIMAccount is a read-only provider-account lookup used by trusted
// callback adapters. It never imports or creates an account, so an unknown
// callback account cannot create a local user as a side effect.
func (s *Service) ResolveIMAccount(ctx context.Context, imUserID string) (uint64, error) {
	if s == nil || s.repo == nil || imUserID == "" {
		return 0, ErrNotificationIdentityUnavailable
	}
	lookup, ok := s.repo.(accountMappingLookup)
	if !ok {
		return 0, ErrNotificationIdentityUnavailable
	}
	mapping, err := lookup.FindMappingByIMUserID(ctx, imUserID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, ErrNotificationIdentityUnavailable
	}
	if err != nil || mapping.UserID == 0 {
		return 0, ErrNotificationIdentityUnavailable
	}
	return uint64(mapping.UserID), nil
}

// FindUserConversation is a read-only lookup for the stable C2C user
// relation. It does not import Tencent accounts or refresh relation state.
func (s *Service) FindUserConversation(ctx context.Context, senderID, recipientID uint) (ConversationContext, bool, error) {
	if s == nil || s.repo == nil || senderID == 0 || recipientID == 0 || senderID == recipientID {
		return ConversationContext{}, false, ErrNotificationIdentityUnavailable
	}
	first, second := senderID, recipientID
	if first > second {
		first, second = second, first
	}
	value, found, err := s.repo.FindConversation(ctx, first, second, "user", 0)
	if err != nil || !found {
		return ConversationContext{}, found, err
	}
	result, err := s.contextForParticipant(ctx, value, senderID)
	return result, true, err
}
