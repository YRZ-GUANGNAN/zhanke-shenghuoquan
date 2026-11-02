package delivery

import (
	"context"
	"errors"
	"time"
)

// RiderDailySettlementService is deliberately an explicit service. A deploy
// may invoke ReconcileBatch from its scheduler; constructing it does not start
// a background goroutine or mutate production data.
type RiderDailySettlementService struct {
	repository RiderDailySettlementRepository
	now        func() time.Time
	batchSize  uint64
}

func NewRiderDailySettlementService(repository RiderDailySettlementRepository, now func() time.Time) *RiderDailySettlementService {
	if now == nil {
		now = time.Now
	}
	return &RiderDailySettlementService{repository: repository, now: now, batchSize: 100}
}

func (s *RiderDailySettlementService) ReconcileBatch(ctx context.Context) (int, error) {
	if s == nil || s.repository == nil {
		return 0, ErrPayoutQueueUnavailable
	}
	now := s.now()
	if now.IsZero() {
		return 0, ErrInvalidRecord
	}
	count := 0
	var afterID uint64
	for {
		if err := ctx.Err(); err != nil {
			return count, err
		}
		values, err := s.repository.ListDueRiderEarnings(ctx, now, afterID, s.batchSize)
		if err != nil {
			return count, err
		}
		for _, earning := range values {
			if earning.ID <= afterID {
				return count, ErrInvalidRecord
			}
			afterID = earning.ID
			if _, _, err := s.repository.SettleRiderEarning(ctx, earning.EarningPublicID, now); err != nil {
				if isRiderPayoutSourceBlockError(err) || errors.Is(err, ErrRiderEarningNotDue) {
					continue // One blocked source cannot starve later valid earnings.
				}
				return count, err
			}
			count++
		}
		if uint64(len(values)) < s.batchSize {
			return count, nil
		}
	}
}
