package delivery

import (
	"context"
	"time"
)

// RiderPayoutQueueService only reconciles internal facts. It never calls a
// provider and never marks a payout as externally successful.
type RiderPayoutQueueService struct {
	repository RiderPayoutQueueRepository
	now        func() time.Time
	batchSize  int
}

func (s *RiderPayoutQueueService) Run(ctx context.Context, interval time.Duration) error {
	if s == nil || s.repository == nil {
		return ErrPayoutQueueUnavailable
	}
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := s.ReconcileBatch(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func NewRiderPayoutQueueService(repository RiderPayoutQueueRepository, now func() time.Time) *RiderPayoutQueueService {
	if now == nil {
		now = time.Now
	}
	return &RiderPayoutQueueService{repository: repository, now: now, batchSize: 100}
}

func (s *RiderPayoutQueueService) ReconcileBatch(ctx context.Context) (int, error) {
	if s == nil || s.repository == nil {
		return 0, ErrPayoutQueueUnavailable
	}
	count := 0
	cursor := RiderPayoutCandidateCursor{Limit: s.batchSize}
	for {
		values, err := s.repository.ListRiderPayoutCandidates(ctx, cursor)
		if err != nil {
			return count, err
		}
		if len(values) == 0 {
			return count, nil
		}
		for _, earning := range values {
			if _, err := s.repository.ReconcileRiderPayoutQueue(ctx, earning.EarningPublicID, s.now()); err != nil {
				return count, err
			}
			count++
			cursor.AfterID = earning.ID
		}
		if len(values) < cursor.Limit {
			return count, nil
		}
	}
}
