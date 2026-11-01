package notification

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"
)

func callbackEventRows(t *testing.T) (CallbackReceipt, Event) {
	t.Helper()
	endpoint, cfg := callbackFixture(time.Unix(1793500000, 0))
	callback, err := ParseIMCallback(endpoint, []byte(c2cFixture), cfg)
	if err != nil {
		t.Fatal(err)
	}
	relation := IMNotificationRelation{SenderUserID: 11, RecipientUserID: 12, SenderActive: true, RecipientActive: true, Allowed: true, ConversationPublicID: "im-pair:11:12", TargetPath: "/pages/secondhand-chat/index?relationId=4"}
	event, err := BuildPrivateMessageEvent(callback, relation)
	if err != nil {
		t.Fatal(err)
	}
	return CallbackReceipt{SDKAppID: callback.SDKAppID, ProviderMessageID: callback.ProviderMessageID,
		SenderAccount: callback.SenderAccount, RecipientAccount: callback.RecipientAccount,
		SenderUserID: 11, RecipientUserID: 12, CallbackHash: callback.CallbackHash,
		ConversationType: IMCallbackC2C, ConversationPublicID: relation.ConversationPublicID,
		OccurredAt: callback.OccurredAt, CreatedAt: callback.OccurredAt}, event
}

func TestIMCallbackRepositoryAtomicIdempotencyAndRollback(t *testing.T) {
	receipt, event := callbackEventRows(t)
	for _, failTable := range []string{"", "im_notification_receipts", "delivery_notification_events"} {
		t.Run("insert_"+failTable, func(t *testing.T) {
			eventRepo, state := newEventSQLRepository(t, testLease(t))
			state.tables = make(map[string][]map[string]driver.Value)
			state.failInsertTable = failTable
			repo := NewRepository(eventRepo.db)
			inserted, err := repo.RecordCallbackAndEvent(context.Background(), receipt, event)
			if failTable != "" {
				if err == nil || inserted || len(state.tables["im_notification_receipts"]) != 0 || len(state.tables["delivery_notification_events"]) != 0 {
					t.Fatal("failed transaction retained a receipt/event or reported success")
				}
				state.failInsertTable = ""
				inserted, err = repo.RecordCallbackAndEvent(context.Background(), receipt, event)
			}
			if err != nil || !inserted || len(state.tables["im_notification_receipts"]) != 1 || len(state.tables["delivery_notification_events"]) != 1 {
				t.Fatalf("valid transaction inserted=%v err=%v", inserted, err)
			}
			if inserted, err = repo.RecordCallbackAndEvent(context.Background(), receipt, event); err != nil || inserted || len(state.tables["delivery_notification_events"]) != 1 {
				t.Fatalf("duplicate was not idempotent inserted=%v err=%v", inserted, err)
			}
		})
	}
}

func TestIMCallbackRepositoryRejectsMismatchedReceiptBeforeTransaction(t *testing.T) {
	validReceipt, validEvent := callbackEventRows(t)
	for _, mutate := range []func(*CallbackReceipt, *Event){
		func(r *CallbackReceipt, _ *Event) { r.SenderUserID++ },
		func(r *CallbackReceipt, _ *Event) { r.RecipientUserID++ },
		func(r *CallbackReceipt, _ *Event) { r.SenderAccount = "another-account" },
		func(r *CallbackReceipt, _ *Event) { r.ProviderMessageID = "another-message" },
		func(r *CallbackReceipt, _ *Event) { r.CallbackHash = "bad" },
		func(r *CallbackReceipt, _ *Event) { r.Status = "processed" },
		func(_ *CallbackReceipt, e *Event) { e.AggregatePublicID = "wrong-pair" },
		func(_ *CallbackReceipt, e *Event) { e.OccurredAt = e.OccurredAt.Add(time.Second) },
		func(_ *CallbackReceipt, e *Event) { e.EventType = "OrderStatusChanged" },
	} {
		r, e := validReceipt, validEvent
		mutate(&r, &e)
		repo, state := newEventSQLRepository(t, testLease(t))
		before := len(state.queries)
		if _, err := NewRepository(repo.db).RecordCallbackAndEvent(context.Background(), r, e); !errors.Is(err, ErrInvalidRecord) || len(state.queries) != before {
			t.Fatalf("invalid coordinate reached SQL: err=%v", err)
		}
	}
}
