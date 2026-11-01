package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"campus-circle/backend/internal/module/delivery"
	"campus-circle/backend/internal/module/notification"
	"campus-circle/backend/internal/module/user"
	"gorm.io/gorm"
)

// Financial notifications are inbox-only until separately reviewed financial
// templates exist. An order-status template is not a financial authorization.
type financialNotificationReader interface {
	FindRiderEarning(context.Context, string) (delivery.RiderEarningRecord, error)
	FindRiderPayout(context.Context, string) (delivery.RiderPayoutRecord, error)
	FindRiderWithdrawal(context.Context, string) (delivery.RiderWithdrawal, error)
	FindRefund(context.Context, string) (delivery.Refund, error)
	FindPaymentAttemptOwned(context.Context, string, uint64) (delivery.PaymentAttempt, error)
}

type financialNotificationRecipients struct {
	funds financialNotificationReader
	users notificationUserReader
}

func financialAggregateType(eventType string) string {
	switch eventType {
	case delivery.EventRiderIncomeRecorded:
		return "rider_earning"
	case delivery.EventRiderPayoutCompleted:
		return "rider_payout"
	case delivery.EventRiderWithdrawalChanged:
		return "rider_withdrawal"
	case delivery.EventMerchantRefundChanged:
		return "delivery_refund"
	case delivery.EventMerchantWalletRecharged:
		return "merchant_recharge"
	default:
		return ""
	}
}

func (r *financialNotificationRecipients) ResolveRecipients(ctx context.Context, event notification.Event) ([]notification.Recipient, error) {
	if r == nil || r.funds == nil || r.users == nil || event.PayloadVersion != 1 ||
		!delivery.ValidULID(event.AggregatePublicID) || event.AggregateVersion == 0 ||
		financialAggregateType(event.EventType) == "" || financialAggregateType(event.EventType) != event.AggregateType {
		return nil, notification.ErrInvalidRecord
	}
	var owner, amount, version uint64
	var status, currentStatus, title, path string
	var err error
	purpose := notification.PurposePayment
	switch event.EventType {
	case delivery.EventRiderIncomeRecorded:
		var fact delivery.RiderIncomeNotificationPayload
		if json.Unmarshal(event.Payload, &fact) != nil || fact.EarningPublicID != event.AggregatePublicID || !delivery.ValidULID(fact.OrderPublicID) {
			return nil, notification.ErrInvalidRecord
		}
		var current delivery.RiderEarningRecord
		current, err = r.funds.FindRiderEarning(ctx, fact.EarningPublicID)
		if err == nil && (current.EarningPublicID != fact.EarningPublicID || current.DeliveryOrderPublicID != fact.OrderPublicID || current.RiderUserID != fact.RiderUserID || current.RiderFeeCents != fact.AmountCents) {
			return nil, notification.ErrInvalidRecord
		}
		owner, amount, version, status, currentStatus = fact.RiderUserID, fact.AmountCents, current.RowVersion, fact.SettlementStatus, string(current.SettlementStatus)
		// Recording an earning is neither a next-day release nor an external payment.
		title = map[string]string{"pending": "配送收入已记录", "settled": "配送收入已结算"}[status]
		purpose, path = notification.PurposeRiderIncome, "/pages/rider-wallet/index"
	case delivery.EventRiderPayoutCompleted:
		var fact delivery.RiderPayoutNotificationPayload
		if json.Unmarshal(event.Payload, &fact) != nil || fact.PayoutPublicID != event.AggregatePublicID || !delivery.ValidULID(fact.EarningPublicID) {
			return nil, notification.ErrInvalidRecord
		}
		var current delivery.RiderPayoutRecord
		current, err = r.funds.FindRiderPayout(ctx, fact.PayoutPublicID)
		if err == nil && (current.PayoutPublicID != fact.PayoutPublicID || current.EarningPublicID != fact.EarningPublicID || current.RiderUserID != fact.RiderUserID || current.AmountCents != fact.AmountCents || current.PaidOfflineAt.IsZero()) {
			return nil, notification.ErrInvalidRecord
		}
		owner, amount, version, status, currentStatus = fact.RiderUserID, fact.AmountCents, current.RowVersion, fact.Status, "paid_offline"
		if status == "paid_offline" {
			title = "后台已登记打款"
		}
		purpose, path = notification.PurposeWithdrawal, "/pages/rider-wallet/index"
	case delivery.EventRiderWithdrawalChanged:
		var fact delivery.RiderWithdrawalNotificationPayload
		if json.Unmarshal(event.Payload, &fact) != nil || fact.WithdrawPublicID != event.AggregatePublicID {
			return nil, notification.ErrInvalidRecord
		}
		var current delivery.RiderWithdrawal
		current, err = r.funds.FindRiderWithdrawal(ctx, fact.WithdrawPublicID)
		if err == nil && (current.WithdrawPublicID != fact.WithdrawPublicID || current.RiderUserID != fact.RiderUserID || current.AmountCents != fact.AmountCents) {
			return nil, notification.ErrInvalidRecord
		}
		owner, amount, version, status, currentStatus = fact.RiderUserID, fact.AmountCents, current.RowVersion, fact.Status, string(current.Status)
		title = map[string]string{"processing": "提现申请已受理", "paid": "后台已登记提现打款", "rejected": "提现申请未通过"}[status]
		purpose, path = notification.PurposeWithdrawal, "/pages/rider-wallet/index"
	case delivery.EventMerchantRefundChanged:
		var fact delivery.MerchantRefundNotificationPayload
		if json.Unmarshal(event.Payload, &fact) != nil || fact.RefundPublicID != event.AggregatePublicID || !delivery.ValidULID(fact.OrderPublicID) {
			return nil, notification.ErrInvalidRecord
		}
		var current delivery.Refund
		current, err = r.funds.FindRefund(ctx, fact.RefundPublicID)
		if err == nil && (current.RefundPublicID != fact.RefundPublicID || current.DeliveryOrderPublicID != fact.OrderPublicID || current.MerchantUserID != fact.MerchantUserID || current.AmountCents != fact.AmountCents) {
			return nil, notification.ErrInvalidRecord
		}
		owner, amount, version, status, currentStatus = fact.MerchantUserID, fact.AmountCents, current.RowVersion, fact.Status, string(current.Status)
		title = map[string]string{"pending": "退款申请已受理", "refund_pending": "退款处理中", "succeeded": "退款已完成", "failed": "退款未完成"}[status]
		path = "/pages/merchant-order-detail/index?orderId=" + fact.OrderPublicID
	case delivery.EventMerchantWalletRecharged:
		var fact delivery.MerchantWalletNotificationPayload
		if json.Unmarshal(event.Payload, &fact) != nil || fact.PaymentPublicID != event.AggregatePublicID {
			return nil, notification.ErrInvalidRecord
		}
		var current delivery.PaymentAttempt
		current, err = r.funds.FindPaymentAttemptOwned(ctx, fact.PaymentPublicID, fact.MerchantUserID)
		if err == nil && (current.PaymentPublicID != fact.PaymentPublicID || current.MerchantUserID != fact.MerchantUserID || current.AmountCents != fact.AmountCents || current.Purpose != delivery.PaymentWalletRecharge) {
			return nil, notification.ErrInvalidRecord
		}
		owner, amount, version, status, currentStatus = fact.MerchantUserID, fact.AmountCents, current.RowVersion, fact.Status, string(current.Status)
		if status == string(delivery.PaymentPaid) {
			title = "钱包充值已完成"
		}
		path = "/pages/merchant-wallet/index"
	}
	if errors.Is(err, delivery.ErrNotFound) || errors.Is(err, delivery.ErrWithdrawalNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("notification financial fact lookup failed")
	}
	if owner == 0 || uint64(uint(owner)) != owner || amount == 0 || version < event.AggregateVersion || title == "" {
		return nil, notification.ErrInvalidRecord
	}
	if status != currentStatus {
		return nil, nil // Never announce a superseded processing/pending result.
	}
	account, err := r.users.Get(ctx, uint(owner))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("notification account lookup failed")
	}
	if account.Status != user.StatusActive || account.ClosedAt != nil || account.DeletedAt.Valid {
		return nil, nil
	}
	settings, err := r.users.Settings(ctx, uint(owner))
	if err != nil {
		return nil, errors.New("notification preference lookup failed")
	}
	if !settings.OrderNotice {
		return nil, nil
	}
	return []notification.Recipient{{UserID: owner, Purpose: purpose, Title: title,
		Summary:       fmt.Sprintf("金额 ¥%d.%02d，请进入钱包或订单查看详情", amount/100, amount%100),
		AggregateType: event.AggregateType, AggregatePublicID: event.AggregatePublicID,
		AggregateVersion: event.AggregateVersion, TargetPath: path}}, nil
}
