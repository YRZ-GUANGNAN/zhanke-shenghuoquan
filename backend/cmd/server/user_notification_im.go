package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	"campus-circle/backend/internal/module/featureflags"
	"campus-circle/backend/internal/module/im"
	"campus-circle/backend/internal/module/notification"
	"campus-circle/backend/internal/module/user"
	"gorm.io/gorm"
)

type notificationIMReader interface {
	ResolveIMAccount(context.Context, string) (uint64, error)
	FindUserConversation(context.Context, uint, uint) (im.ConversationContext, bool, error)
}

type notificationPrivateUserReader interface {
	Get(context.Context, uint) (user.User, error)
	Settings(context.Context, uint) (user.SettingsView, error)
	ValidatePrivateContact(context.Context, uint, uint) error
}

// notificationIMIdentityAdapter is the composition-root bridge between the
// Tencent account mapping/conversation domain and notification's narrow
// callback port. Every callback performs a fresh local user and block check.
type notificationIMIdentityAdapter struct {
	im    notificationIMReader
	users notificationPrivateUserReader
}

func (a notificationIMIdentityAdapter) ResolveIMAccount(ctx context.Context, account string) (uint64, error) {
	if a.im == nil {
		return 0, errors.New("notification IM identity lookup unavailable")
	}
	if account == "" {
		return 0, notification.ErrIMCallbackRelation
	}
	id, err := a.im.ResolveIMAccount(ctx, account)
	if errors.Is(err, im.ErrNotificationIdentityUnavailable) || errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, notification.ErrIMCallbackRelation
	}
	if err != nil {
		return 0, errors.New("notification IM identity lookup failed")
	}
	return id, nil
}

func (a notificationIMIdentityAdapter) ResolveC2CRelation(ctx context.Context, senderID, recipientID uint64) (notification.IMNotificationRelation, error) {
	if a.im == nil || a.users == nil || senderID == 0 || recipientID == 0 || senderID == recipientID ||
		uint64(uint(senderID)) != senderID || uint64(uint(recipientID)) != recipientID {
		return notification.IMNotificationRelation{}, notification.ErrIMCallbackRelation
	}
	if !featureflags.Can(featureflags.KeyMessages) {
		return notification.IMNotificationRelation{}, notification.ErrIMCallbackRelation
	}
	conversation, found, err := a.im.FindUserConversation(ctx, uint(senderID), uint(recipientID))
	if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, im.ErrNotificationIdentityUnavailable) {
		return notification.IMNotificationRelation{}, notification.ErrIMCallbackRelation
	}
	if err != nil {
		return notification.IMNotificationRelation{}, errors.New("notification IM relation lookup failed")
	}
	if !found || conversation.RelationID == 0 || conversation.ReferenceType != "user" ||
		!((conversation.BuyerID == uint(senderID) && conversation.SellerID == uint(recipientID)) ||
			(conversation.BuyerID == uint(recipientID) && conversation.SellerID == uint(senderID))) {
		// Product-only and delivery GROUP relationships must never be silently
		// promoted into a direct user relationship, even if messaging is enabled.
		return notification.IMNotificationRelation{}, notification.ErrIMCallbackRelation
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
	if uint64(sender.ID) != senderID || uint64(recipient.ID) != recipientID || sender.Status != user.StatusActive || recipient.Status != user.StatusActive || sender.ClosedAt != nil || recipient.ClosedAt != nil || sender.DeletedAt.Valid || recipient.DeletedAt.Valid {
		return notification.IMNotificationRelation{}, notification.ErrIMCallbackRelation
	}
	if err := a.users.ValidatePrivateContact(ctx, uint(senderID), uint(recipientID)); err != nil {
		if errors.Is(err, user.ErrPrivateContactUnavailable) {
			return notification.IMNotificationRelation{}, notification.ErrIMCallbackRelation
		}
		return notification.IMNotificationRelation{}, err
	}
	targetPath := "/pages/secondhand-chat/index?relationId=" + strconv.FormatUint(uint64(conversation.RelationID), 10)
	return notification.IMNotificationRelation{
		SenderUserID: senderID, RecipientUserID: recipientID,
		SenderActive: true, RecipientActive: true, Allowed: true,
		ConversationPublicID: notificationIMPair(senderID, recipientID), TargetPath: targetPath,
	}, nil
}

func notificationIMPair(senderID, recipientID uint64) string {
	first, second := senderID, recipientID
	if first > second {
		first, second = second, first
	}
	return "im-pair:" + strconv.FormatUint(first, 10) + ":" + strconv.FormatUint(second, 10)
}

type privateNotificationRecipients struct {
	identity notification.IMCallbackIdentityResolver
	users    notificationPrivateUserReader
}

// ResolveRecipients re-checks current account, contact/block, relationship,
// feature and preference facts when the event is expanded, not just when the
// callback was accepted. No private-message WeChat tasks are produced.
func (r *privateNotificationRecipients) ResolveRecipients(ctx context.Context, event notification.Event) ([]notification.Recipient, error) {
	if r == nil || r.identity == nil || r.users == nil || event.EventType != notification.EventPrivateMessageReceived || event.AggregateType != "im_conversation" || event.PayloadVersion != 1 || event.AggregateVersion == 0 {
		return nil, notification.ErrInvalidRecord
	}
	var fact struct {
		SenderUserID       uint64 `json:"senderUserId"`
		RecipientUserID    uint64 `json:"recipientUserId"`
		ProviderMessageID  string `json:"providerMessageId"`
		MessageSequence    uint32 `json:"messageSequence,omitempty"`
		UnreadMessageCount int64  `json:"unreadMessageCount,omitempty"`
		TargetPath         string `json:"targetPath"`
		OccurredAt         string `json:"occurredAt"`
	}
	decoder := json.NewDecoder(strings.NewReader(string(event.Payload)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&fact) != nil || decoder.Decode(new(any)) != io.EOF || fact.SenderUserID == 0 || fact.RecipientUserID == 0 || fact.SenderUserID == fact.RecipientUserID || fact.ProviderMessageID == "" || fact.UnreadMessageCount < 0 || event.AggregatePublicID != notificationIMPair(fact.SenderUserID, fact.RecipientUserID) {
		return nil, notification.ErrInvalidRecord
	}
	if !featureflags.Can(featureflags.KeyMessages) {
		return nil, nil
	}
	relation, err := r.identity.ResolveC2CRelation(ctx, fact.SenderUserID, fact.RecipientUserID)
	if errors.Is(err, notification.ErrIMCallbackRelation) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("notification private relation lookup failed")
	}
	if !relation.Allowed || relation.Blocked || !relation.SenderActive || !relation.RecipientActive || relation.SenderUserID != fact.SenderUserID || relation.RecipientUserID != fact.RecipientUserID || relation.ConversationPublicID != event.AggregatePublicID || relation.TargetPath != fact.TargetPath || !validPrivateNotificationTarget(relation.TargetPath) {
		return nil, nil
	}
	settings, err := r.users.Settings(ctx, uint(fact.RecipientUserID))
	if err != nil {
		return nil, errors.New("notification private preference lookup failed")
	}
	if !settings.SocialNotice {
		return nil, nil
	}
	return []notification.Recipient{{UserID: fact.RecipientUserID, Purpose: notification.PurposePrivateMessage,
		Title: "收到新私信", Summary: "请进入会话查看消息", AggregateType: event.AggregateType,
		AggregatePublicID: event.AggregatePublicID, AggregateVersion: event.AggregateVersion,
		TargetPath: relation.TargetPath, AllowWechat: false}}, nil
}

func validPrivateNotificationTarget(path string) bool {
	const prefix = "/pages/secondhand-chat/index?relationId="
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	value := strings.TrimPrefix(path, prefix)
	id, err := strconv.ParseUint(value, 10, 64)
	return err == nil && id > 0 && value == strconv.FormatUint(id, 10)
}
