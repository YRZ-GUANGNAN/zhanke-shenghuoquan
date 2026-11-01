package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/notification"
)

type financialReaderStub struct {
	earning    delivery.RiderEarningRecord
	payout     delivery.RiderPayoutRecord
	withdrawal delivery.RiderWithdrawal
	refund     delivery.Refund
	payment    delivery.PaymentAttempt
	err        error
}

func (s financialReaderStub) FindRiderEarning(context.Context, string) (delivery.RiderEarningRecord, error) {
	return s.earning, s.err
}
func (s financialReaderStub) FindRiderPayout(context.Context, string) (delivery.RiderPayoutRecord, error) {
	return s.payout, s.err
}
func (s financialReaderStub) FindRiderWithdrawal(context.Context, string) (delivery.RiderWithdrawal, error) {
	return s.withdrawal, s.err
}
func (s financialReaderStub) FindRefund(context.Context, string) (delivery.Refund, error) {
	return s.refund, s.err
}
func (s financialReaderStub) FindPaymentAttemptOwned(context.Context, string, uint64) (delivery.PaymentAttempt, error) {
	return s.payment, s.err
}

func financialTestEvent(eventType string, payload any) notification.Event {
	raw, _ := json.Marshal(payload)
	return notification.Event{EventType: eventType, AggregateType: financialAggregateType(eventType),
		AggregatePublicID: notificationTestOrderID, AggregateVersion: 2, PayloadVersion: 1, Payload: raw}
}

func TestFinancialNotificationOwnerFactAndInAppOnly(t *testing.T) {
	id := notificationTestOrderID
	reader := financialReaderStub{
		earning:    delivery.RiderEarningRecord{EarningPublicID: id, DeliveryOrderPublicID: id, RiderUserID: 2, RiderFeeCents: 123, SettlementStatus: delivery.RiderEarningPending, RowVersion: 2},
		payout:     delivery.RiderPayoutRecord{PayoutPublicID: id, EarningPublicID: id, RiderUserID: 2, AmountCents: 123, PaidOfflineAt: time.Now(), RowVersion: 2},
		withdrawal: delivery.RiderWithdrawal{WithdrawPublicID: id, RiderUserID: 2, AmountCents: 123, Status: delivery.RiderWithdrawalProcessing, RowVersion: 2},
		refund:     delivery.Refund{RefundPublicID: id, DeliveryOrderPublicID: id, MerchantUserID: 2, AmountCents: 123, Status: delivery.RefundRetryPending, RowVersion: 2},
		payment:    delivery.PaymentAttempt{PaymentPublicID: id, MerchantUserID: 2, AmountCents: 123, Purpose: delivery.PaymentWalletRecharge, Status: delivery.PaymentPaid, RowVersion: 2},
	}
	cases := []struct {
		event       notification.Event
		title, path string
	}{
		{financialTestEvent(delivery.EventRiderIncomeRecorded, delivery.RiderIncomeNotificationPayload{EarningPublicID: id, OrderPublicID: id, RiderUserID: 2, AmountCents: 123, SettlementStatus: "pending"}), "配送收入已记录", "/pages/rider-wallet/index"},
		{financialTestEvent(delivery.EventRiderPayoutCompleted, delivery.RiderPayoutNotificationPayload{PayoutPublicID: id, EarningPublicID: id, RiderUserID: 2, AmountCents: 123, Status: "paid_offline"}), "后台已登记打款", "/pages/rider-wallet/index"},
		{financialTestEvent(delivery.EventRiderWithdrawalChanged, delivery.RiderWithdrawalNotificationPayload{WithdrawPublicID: id, RiderUserID: 2, AmountCents: 123, Status: "processing"}), "提现申请已受理", "/pages/rider-wallet/index"},
		{financialTestEvent(delivery.EventMerchantRefundChanged, delivery.MerchantRefundNotificationPayload{RefundPublicID: id, OrderPublicID: id, MerchantUserID: 2, AmountCents: 123, Status: "refund_pending"}), "退款处理中", "/pages/merchant-order-detail/index?orderId=" + id},
		{financialTestEvent(delivery.EventMerchantWalletRecharged, delivery.MerchantWalletNotificationPayload{PaymentPublicID: id, MerchantUserID: 2, AmountCents: 123, Status: "paid"}), "钱包充值已完成", "/pages/merchant-wallet/index"},
	}
	for _, tc := range cases {
		t.Run(tc.event.EventType, func(t *testing.T) {
			resolver := financialNotificationRecipients{funds: reader, users: notificationUsersStub{}}
			got, err := resolver.ResolveRecipients(context.Background(), tc.event)
			if err != nil || len(got) != 1 || got[0].UserID != 2 || got[0].Title != tc.title || got[0].TargetPath != tc.path || !strings.Contains(got[0].Summary, "¥1.23") {
				t.Fatalf("financial recipient: count=%d err=%v", len(got), err)
			}
			if got[0].AllowWechat || got[0].WechatTemplateKey != "" || strings.Contains(got[0].Summary, "到账") {
				t.Fatal("financial notification manufactured external delivery or到账")
			}
			for _, users := range []notificationUsersStub{{inactive: 2}, {optedOut: 2}} {
				resolver.users = users
				if recipients, err := resolver.ResolveRecipients(context.Background(), tc.event); err != nil || len(recipients) != 0 {
					t.Fatal("ignored account/preference guard")
				}
			}
			resolver.users = notificationUsersStub{}
			missing := reader
			missing.err = delivery.ErrNotFound
			resolver.funds = missing
			if got, err := resolver.ResolveRecipients(context.Background(), tc.event); err != nil || len(got) != 0 {
				t.Fatal("missing fact generated notification")
			}
			missing.err = errors.New("private-database-detail")
			resolver.funds = missing
			if _, err := resolver.ResolveRecipients(context.Background(), tc.event); err == nil || strings.Contains(err.Error(), "private-database-detail") {
				t.Fatal("lookup failure leaked or disappeared")
			}
			resolver.funds = reader
			future := tc.event
			future.AggregateVersion = 3
			if _, err := resolver.ResolveRecipients(context.Background(), future); !errors.Is(err, notification.ErrInvalidRecord) {
				t.Fatal("future version accepted")
			}
			wrong := tc.event
			wrong.AggregateType = "delivery_order"
			if _, err := resolver.ResolveRecipients(context.Background(), wrong); !errors.Is(err, notification.ErrInvalidRecord) {
				t.Fatal("aggregate alias accepted")
			}
			adapted := deliveryNotificationFact(delivery.NotificationEvent{EventType: tc.event.EventType, AggregatePublicID: id, AggregateVersion: 2, Payload: tc.event.Payload, OccurredAt: time.Now()})
			if adapted.AggregateType != tc.event.AggregateType {
				t.Fatal("adapter lost financial aggregate")
			}
		})
	}
}

func TestFinancialNotificationSuppressesStaleAndMismatchedFacts(t *testing.T) {
	id := notificationTestOrderID
	fact := delivery.RiderWithdrawalNotificationPayload{WithdrawPublicID: id, RiderUserID: 2, AmountCents: 123, Status: "processing"}
	event := financialTestEvent(delivery.EventRiderWithdrawalChanged, fact)
	current := delivery.RiderWithdrawal{WithdrawPublicID: id, RiderUserID: 2, AmountCents: 123, Status: delivery.RiderWithdrawalPaid, RowVersion: 3}
	resolver := financialNotificationRecipients{funds: financialReaderStub{withdrawal: current}, users: notificationUsersStub{}}
	if got, err := resolver.ResolveRecipients(context.Background(), event); err != nil || len(got) != 0 {
		t.Fatal("stale processing notice emitted after payment")
	}
	for _, mutate := range []func(*delivery.RiderWithdrawal){
		func(v *delivery.RiderWithdrawal) { v.RiderUserID = 3 },
		func(v *delivery.RiderWithdrawal) { v.AmountCents++ },
		func(v *delivery.RiderWithdrawal) { v.WithdrawPublicID = "different" },
	} {
		copy := current
		mutate(&copy)
		resolver.funds = financialReaderStub{withdrawal: copy}
		if _, err := resolver.ResolveRecipients(context.Background(), event); !errors.Is(err, notification.ErrInvalidRecord) {
			t.Fatal("wrong recipient/amount/ID accepted")
		}
	}
	resolver.funds = financialReaderStub{withdrawal: current}
	fact.Status = "paid"
	got, err := resolver.ResolveRecipients(context.Background(), financialTestEvent(delivery.EventRiderWithdrawalChanged, fact))
	if err != nil || len(got) != 1 || got[0].Title != "后台已登记提现打款" {
		t.Fatal("manual payment confirmation misrepresented")
	}
}
