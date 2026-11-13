package deliveryconsole

import (
	"testing"
	"time"
)

func TestShanghaiBusinessDayStart(t *testing.T) {
	for _, check := range []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "after midnight in Shanghai",
			now:  time.Date(2026, 11, 13, 0, 15, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60)),
			want: time.Date(2026, 11, 12, 16, 0, 0, 0, time.UTC),
		},
		{
			name: "previous UTC day is still the current Shanghai day",
			now:  time.Date(2026, 11, 12, 17, 30, 0, 0, time.UTC),
			want: time.Date(2026, 11, 12, 16, 0, 0, 0, time.UTC),
		},
	} {
		t.Run(check.name, func(t *testing.T) {
			if got := shanghaiBusinessDayStart(check.now); !got.Equal(check.want) {
				t.Fatalf("got=%s want=%s", got, check.want)
			}
		})
	}
}
