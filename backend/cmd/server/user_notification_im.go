package main

import (
	"context"
	"errors"
	"strconv"

	"campus-circle/backend/internal/module/im"
	"campus-circle/backend/internal/module/notification"
	"campus-circle/backend/internal/module/user"
	"gorm.io/gorm"
)

// notificationIMIdentityAdapter is the composition-root bridge between the
// Tencent account mapping/conversation domain and notification's narrow
// callback port. Every callback performs a fresh local user and block check.
type notificationIMIdentityAdapter struct {
	im    *im.Service
	users *user.Service
}

func (a notificationIMIdentityAdapter) ResolveIMAccount(ctx context.Context, account string) (uint64, error) {
	if a.im == nil || account == "" {
		return 0, notification.ErrIMCallbackRelation
	}
	id, err := a.im.ResolveIMAccount(ctx, account)
	if err != nil {
		return 0, notification.ErrIMCallbackRelation
	}
	return id, nil
}

func (a notificationIMIdentityAdapter) ResolveC2CRelation(ctx context.Context, senderID, recipientID uint64) (notification.IMNotificationRelation, error) {
	if a.im == nil || a.users == nil || senderID == 0 || recipientID == 0 || senderID == recipientID ||
		uint64(uint(senderID)) != senderID || uint64(uint(recipientID)) != recipientID {
		return notification.IMNotificationRelation{}, notification.ErrIMCallbackRelation
	}
	conversation, found, err := a.im.FindUserConversation(ctx, uint(senderID), uint(recipientID))
	if errors.Is(err, gorm.ErrRecordNotFound) || !found {
		return notification.IMNotificationRelation{}, notification.ErrIMCallbackRelation
	}
	if err != nil {
		return notification.IMNotificationRelation{}, err
	}
	sender, err := a.users.Get(ctx, uint(senderID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, user.ErrNotFound) {
			return notification.IMNotificationRelation{}, notification.ErrIMCallbackRelation
		}
		return notification.IMNotificationRelation{}, err
	}
	recipient, err := a.users.Get(ctx, uint(recipientID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, user.ErrNotFound) {
			return notification.IMNotificationRelation{}, notification.ErrIMCallbackRelation
		}
		return notification.IMNotificationRelation{}, err
	}
	if sender.Status != user.StatusActive || recipient.Status != user.StatusActive || sender.ClosedAt != nil || recipient.ClosedAt != nil || sender.DeletedAt.Valid || recipient.DeletedAt.Valid {
		return notification.IMNotificationRelation{}, notification.ErrIMCallbackRelation
	}
	if err := a.users.ValidatePrivateContact(ctx, uint(senderID), uint(recipientID)); err != nil {
		if errors.Is(err, user.ErrPrivateContactUnavailable) || errors.Is(err, user.ErrSettingsUnavailable) {
			return notification.IMNotificationRelation{}, notification.ErrIMCallbackRelation
		}
		return notification.IMNotificationRelation{}, err
	}
	targetPath := "/pages/chat/index?relationId=" + strconv.FormatUint(uint64(conversation.RelationID), 10) +
		"&conversationId=" + conversation.ConversationID + "&peerUserId=" + strconv.FormatUint(recipientID, 10)
	return notification.IMNotificationRelation{
		SenderUserID: senderID, RecipientUserID: recipientID,
		SenderActive: true, RecipientActive: true, Allowed: true,
		ConversationPublicID: conversation.ConversationID, TargetPath: targetPath,
	}, nil
}
