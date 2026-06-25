package takeout

import (
	"context"
	"time"
)

type PaymentRepository interface {
	CreatePaymentAttempt(context.Context, *PaymentAttempt) error
	FindPaymentAttempt(context.Context, string) (PaymentAttempt, error)
	FindPaymentAttemptByOutTradeNo(context.Context, string) (PaymentAttempt, error)
	FindLatestPaymentAttemptByOrder(context.Context, string, PaymentProvider) (PaymentAttempt, error)
	MarkPaymentAttemptPrepayIssued(context.Context, string, uint64, string, time.Time) (PaymentAttempt, error)
	CreatePaidPaymentAttemptAndMarkOrderPaid(context.Context, *PaymentAttempt, uint64) (PaymentAttempt, Order, error)
	RecordPaymentNotificationAndMarkPaid(context.Context, *PaymentNotification, string, string, string, uint64, time.Time) (PaymentAttempt, Order, bool, error)
}

var _ PaymentRepository = (*GormRepository)(nil)
