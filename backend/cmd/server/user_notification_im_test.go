package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"campus-circle/backend/internal/module/featureflags"
	"campus-circle/backend/internal/module/im"
	"campus-circle/backend/internal/module/notification"
	"campus-circle/backend/internal/module/user"
)

type imNotificationReaderFake struct {
	mappingErr, relationErr error
	found                   bool
	relation                im.ConversationContext
}

func (f imNotificationReaderFake) ResolveIMAccount(context.Context, string) (uint64, error) {
	return 11, f.mappingErr
}
func (f imNotificationReaderFake) FindUserConversation(context.Context, uint, uint) (im.ConversationContext, bool, error) {
	return f.relation, f.found, f.relationErr
}

type privateNotificationUsersFake struct {
	inactive                        uint
	blocked                         bool
	optedOut                        bool
	getErr, contactErr, settingsErr error
}

func (f privateNotificationUsersFake) Get(_ context.Context, id uint) (user.User, error) {
	status := user.StatusActive
	if id == f.inactive {
		status = user.StatusClosed
	}
	return user.User{ID: id, Status: status}, f.getErr
}
func (f privateNotificationUsersFake) Settings(context.Context, uint) (user.SettingsView, error) {
	return user.SettingsView{SocialNotice: !f.optedOut}, f.settingsErr
}
func (f privateNotificationUsersFake) ValidatePrivateContact(context.Context, uint, uint) error {
	if f.blocked {
		return user.ErrPrivateContactUnavailable
	}
	return f.contactErr
}

func privateNotificationIdentityFixture() notificationIMIdentityAdapter {
	return notificationIMIdentityAdapter{im: imNotificationReaderFake{found: true, relation: im.ConversationContext{RelationID: 4, ReferenceType: "user", BuyerID: 11, SellerID: 12}}, users: privateNotificationUsersFake{}}
}

func TestNotificationIMAdapterChecksRelationAndActiveContactState(t *testing.T) {
	featureflags.Configure(false)
	t.Cleanup(func() { featureflags.Configure(false) })
	for _, test := range []struct {
		name        string
		modify      func(*notificationIMIdentityAdapter)
		unavailable bool
	}{
		{"missing user relation", func(a *notificationIMIdentityAdapter) { a.im = imNotificationReaderFake{} }, true},
		{"product-only relation", func(a *notificationIMIdentityAdapter) {
			a.im = imNotificationReaderFake{found: true, relation: im.ConversationContext{RelationID: 4, ReferenceType: "product", BuyerID: 11, SellerID: 12}}
		}, true},
		{"wrong participants", func(a *notificationIMIdentityAdapter) {
			a.im = imNotificationReaderFake{found: true, relation: im.ConversationContext{RelationID: 4, ReferenceType: "user", BuyerID: 13, SellerID: 12}}
		}, true},
		{"inactive sender", func(a *notificationIMIdentityAdapter) { a.users = privateNotificationUsersFake{inactive: 11} }, true},
		{"inactive recipient", func(a *notificationIMIdentityAdapter) { a.users = privateNotificationUsersFake{inactive: 12} }, true},
		{"mutual block", func(a *notificationIMIdentityAdapter) { a.users = privateNotificationUsersFake{blocked: true} }, true},
		{"lookup outage found false", func(a *notificationIMIdentityAdapter) {
			a.im = imNotificationReaderFake{relationErr: errors.New("private db message")}
		}, false},
		{"account outage", func(a *notificationIMIdentityAdapter) {
			a.users = privateNotificationUsersFake{getErr: errors.New("db unavailable")}
		}, false},
		{"block lookup unavailable", func(a *notificationIMIdentityAdapter) {
			a.users = privateNotificationUsersFake{contactErr: user.ErrSettingsUnavailable}
		}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := privateNotificationIdentityFixture()
			test.modify(&a)
			_, err := a.ResolveC2CRelation(context.Background(), 11, 12)
			if err == nil || errors.Is(err, notification.ErrIMCallbackRelation) != test.unavailable {
				t.Fatalf("err=%v", err)
			}
		})
	}
	a := privateNotificationIdentityFixture()
	for _, ids := range [][2]uint64{{11, 12}, {12, 11}} {
		relation, err := a.ResolveC2CRelation(context.Background(), ids[0], ids[1])
		if err != nil || relation.ConversationPublicID != "im-pair:11:12" || relation.TargetPath != "/pages/secondhand-chat/index?relationId=4" || !relation.Allowed {
			t.Fatalf("relation=%+v err=%v", relation, err)
		}
	}
}

func TestNotificationIMAdapterDoesNotAcknowledgeTransientMappingErrors(t *testing.T) {
	a := privateNotificationIdentityFixture()
	a.im = imNotificationReaderFake{mappingErr: im.ErrNotificationIdentityUnavailable}
	if _, err := a.ResolveIMAccount(context.Background(), "unknown"); !errors.Is(err, notification.ErrIMCallbackRelation) {
		t.Fatal("unknown account not classified")
	}
	a.im = imNotificationReaderFake{mappingErr: errors.New("database failure")}
	if _, err := a.ResolveIMAccount(context.Background(), "known"); err == nil || errors.Is(err, notification.ErrIMCallbackRelation) {
		t.Fatal("database failure swallowed")
	}
}

func TestPrivateNotificationRecipientsRechecksEligibilityAndNeverCreatesWechat(t *testing.T) {
	featureflags.Configure(false)
	t.Cleanup(func() { featureflags.Configure(false) })
	a := privateNotificationIdentityFixture()
	relation, err := a.ResolveC2CRelation(context.Background(), 11, 12)
	if err != nil {
		t.Fatal(err)
	}
	event, err := notification.BuildPrivateMessageEvent(notification.IMCallback{SDKAppID: 1400000000, ConversationType: notification.IMCallbackC2C, ProviderMessageID: "provider", SenderAccount: "sender", RecipientAccount: "recipient", OccurredAt: time.Now()}, relation)
	if err != nil {
		t.Fatal(err)
	}
	r := privateNotificationRecipients{identity: a, users: privateNotificationUsersFake{}}
	recipients, err := r.ResolveRecipients(context.Background(), event)
	if err != nil || len(recipients) != 1 || recipients[0].UserID != 12 || recipients[0].Purpose != notification.PurposePrivateMessage || recipients[0].AllowWechat || recipients[0].WechatTemplateKey != "" {
		t.Fatalf("recipients=%+v err=%v", recipients, err)
	}
	for _, mutate := range []func(*privateNotificationRecipients){
		func(r *privateNotificationRecipients) { r.users = privateNotificationUsersFake{optedOut: true} },
		func(r *privateNotificationRecipients) {
			copy := a
			copy.users = privateNotificationUsersFake{blocked: true}
			r.identity = copy
		},
		func(r *privateNotificationRecipients) {
			copy := a
			copy.users = privateNotificationUsersFake{inactive: 12}
			r.identity = copy
		},
		func(r *privateNotificationRecipients) {
			copy := a
			copy.users = privateNotificationUsersFake{inactive: 11}
			r.identity = copy
		},
		func(r *privateNotificationRecipients) {
			copy := a
			copy.im = imNotificationReaderFake{}
			r.identity = copy
		},
	} {
		copy := r
		mutate(&copy)
		if got, err := copy.ResolveRecipients(context.Background(), event); err != nil || len(got) != 0 {
			t.Fatalf("ineligible recipients=%+v err=%v", got, err)
		}
	}
	r.users = privateNotificationUsersFake{settingsErr: errors.New("temporary preference failure")}
	if _, err := r.ResolveRecipients(context.Background(), event); err == nil {
		t.Fatal("temporary preference failure silently acknowledged")
	}
	r.users = privateNotificationUsersFake{}
}
