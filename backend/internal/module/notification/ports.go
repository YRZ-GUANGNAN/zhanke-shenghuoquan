package notification

import "context"

// RecipientPort supplies cross-domain recipient facts to the notification
// composition root. The notification domain does not import user, delivery or
// IM repositories directly.
type RecipientPort interface {
	CanReceive(ctx context.Context, userID uint64, purpose NotificationPurpose) (bool, error)
}

type AccountPort interface {
	IsActive(ctx context.Context, userID uint64) (bool, error)
}

type ConversationPort interface {
	CanNotify(ctx context.Context, senderUserID, recipientUserID uint64, conversationID string) (bool, error)
}

type PreferencePort interface {
	Allows(ctx context.Context, userID uint64, purpose NotificationPurpose) (bool, error)
}
