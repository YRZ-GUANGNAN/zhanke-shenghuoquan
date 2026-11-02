package delivery

import (
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
