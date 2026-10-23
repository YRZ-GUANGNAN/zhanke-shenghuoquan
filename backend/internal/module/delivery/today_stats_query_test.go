package delivery

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestReadRiderTodayStatsBuildsScopedHalfOpenAggregation(t *testing.T) {
	// DryRun and disabled initialization/ping guarantee no database connection.
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN: "unused:unused@tcp(127.0.0.1:1)/unused", SkipInitializeWithVersion: true,
	}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 10, 18, 16, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 19, 16, 0, 0, 0, time.UTC)
	query := riderTodayStatsQuery(db, 77, start, end).Find(&RiderTodayStats{})
	if query.Error != nil {
		t.Fatal(query.Error)
	}
	sqlText := strings.ToLower(query.Statement.SQL.String())
	for _, fragment := range []string{
		"delivery_rider_earning_records", "rider_user_id = ?",
		"created_at >= ?", "created_at < ?", "rider_fee_cents > 0",
		"settlement_status in", "count(*)", "coalesce(sum(rider_fee_cents)",
	} {
		if !strings.Contains(sqlText, fragment) {
			t.Fatalf("aggregation SQL missing %q: %s", fragment, sqlText)
		}
	}
	wantArgs := []interface{}{uint64(77), start, end, RiderEarningPending, RiderEarningSettled, RiderEarningPaidOffline}
	if !reflect.DeepEqual(query.Statement.Vars, wantArgs) {
		t.Fatalf("query args=%#v want=%#v", query.Statement.Vars, wantArgs)
	}
	if strings.Contains(sqlText, "limit") || strings.Contains(sqlText, "wallet") || strings.Contains(sqlText, "updated_at") {
		t.Fatalf("aggregation must not use pagination, balance or payout time: %s", sqlText)
	}
}

func TestReadRiderTodayStatsRejectsInvalidScopeBeforeDatabase(t *testing.T) {
	repository := &GormRepository{}
	start := time.Date(2026, 10, 18, 16, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		rider      uint64
		start, end time.Time
	}{
		{0, start, start.Add(24 * time.Hour)}, {77, start, start},
		{77, start, start.Add(-time.Second)}, {77, time.Time{}, start},
	} {
		if _, err := repository.ReadRiderTodayStats(context.Background(), tc.rider, tc.start, tc.end); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("invalid scope=%+v err=%v", tc, err)
		}
	}
}
