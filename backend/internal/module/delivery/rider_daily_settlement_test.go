package delivery

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestNextRiderSettlementAtUsesBeijingMidnight(t *testing.T) {
	for _, test := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "beijing morning", input: "2026-11-02T03:15:00Z", want: "2026-11-02T16:00:00Z"},
		{name: "utc before midnight", input: "2026-11-02T15:59:59Z", want: "2026-11-02T16:00:00Z"},
		{name: "utc after midnight", input: "2026-11-02T16:00:00Z", want: "2026-11-03T16:00:00Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input, err := time.Parse(time.RFC3339, test.input)
			if err != nil {
				t.Fatal(err)
			}
			want, err := time.Parse(time.RFC3339, test.want)
			if err != nil {
				t.Fatal(err)
			}
			if got := nextRiderSettlementAt(input); !got.Equal(want) {
				t.Fatalf("settleAt=%s want=%s", got.Format(time.RFC3339), want.Format(time.RFC3339))
			}
		})
	}
}

type dailySettlementStub struct {
	rows    []RiderEarningRecord
	cursors []uint64
	calls   []string
	clock   time.Time
	fail    error
}

func (s *dailySettlementStub) ListDueRiderEarnings(_ context.Context, now time.Time, afterID, limit uint64) ([]RiderEarningRecord, error) {
	s.cursors = append(s.cursors, afterID)
	s.clock = now
	var result []RiderEarningRecord
	for _, row := range s.rows {
		if row.ID > afterID && uint64(len(result)) < limit {
			result = append(result, row)
		}
	}
	return result, nil
}
func (s *dailySettlementStub) SettleRiderEarning(_ context.Context, id string, now time.Time) (RiderEarningRecord, RiderLedger, error) {
	if !now.Equal(s.clock) {
		return RiderEarningRecord{}, RiderLedger{}, ErrInvalidRecord
	}
	s.calls = append(s.calls, id)
	if s.fail != nil {
		return RiderEarningRecord{}, RiderLedger{}, s.fail
	}
	if id != "valid" {
		return RiderEarningRecord{}, RiderLedger{}, ErrPayoutQueueSourceBlocked
	}
	return RiderEarningRecord{}, RiderLedger{}, nil
}

func TestRiderDailyBatchDoesNotStarveBehindBlockedSources(t *testing.T) {
	s := &dailySettlementStub{}
	for i := uint64(1); i <= 100; i++ {
		s.rows = append(s.rows, RiderEarningRecord{ID: i, EarningPublicID: fmt.Sprint(i)})
	}
	s.rows = append(s.rows, RiderEarningRecord{ID: 101, EarningPublicID: "valid"})
	now := time.Date(2026, 11, 2, 16, 0, 0, 0, time.UTC)
	count, err := NewRiderDailySettlementService(s, func() time.Time { return now }).ReconcileBatch(context.Background())
	if err != nil || count != 1 || len(s.calls) != 101 || len(s.cursors) != 2 || s.cursors[1] != 100 {
		t.Fatalf("count=%d calls=%d cursors=%v err=%v", count, len(s.calls), s.cursors, err)
	}
}

func TestRiderDailyBatchStopsOnDatabaseFailureOrCancellation(t *testing.T) {
	want := errors.New("database unavailable")
	s := &dailySettlementStub{rows: []RiderEarningRecord{{ID: 1, EarningPublicID: "valid"}}, fail: want}
	service := NewRiderDailySettlementService(s, time.Now)
	if count, err := service.ReconcileBatch(context.Background()); count != 0 || !errors.Is(err, want) {
		t.Fatalf("count=%d err=%v", count, err)
	}
	s.calls, s.cursors = nil, nil
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.ReconcileBatch(ctx); !errors.Is(err, context.Canceled) || len(s.calls) != 0 || len(s.cursors) != 0 {
		t.Fatalf("cancelled batch made repository calls: %v", err)
	}
}
