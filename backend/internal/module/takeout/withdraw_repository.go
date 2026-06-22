package takeout

import (
	"context"
	"time"
)

type WithdrawRepository interface {
	CreateWithdrawRequest(context.Context, *WithdrawRequest) error
	FindWithdrawByID(context.Context, string) (WithdrawRequest, error)
	ListWithdrawRequests(context.Context, WithdrawStatus, int, int) ([]WithdrawRequest, int64, error)
	MarkWithdrawPaid(context.Context, string, uint64, time.Time) (WithdrawRequest, error)
	RejectWithdraw(context.Context, string, uint64, string, time.Time) (WithdrawRequest, error)
}

var _ WithdrawRepository = (*GormRepository)(nil)
