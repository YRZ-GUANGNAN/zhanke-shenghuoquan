package searchmetrics

import (
	"strings"
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	display, normalized, err := Normalize("  Foo   BAR  ")
	if err != nil || display != "Foo BAR" || normalized != "foo bar" {
		t.Fatalf("display=%q normalized=%q err=%v", display, normalized, err)
	}
	for _, value := range []string{"   ", strings.Repeat("界", 51)} {
		if _, _, err := Normalize(value); err == nil {
			t.Fatalf("Normalize(%q) unexpectedly succeeded", value)
		}
	}
}

func TestRankHotUsesSevenDayWeightsAndDistinctUsers(t *testing.T) {
	now := time.Date(2026, 7, 19, 4, 0, 0, 0, time.UTC)
	rows := []eventRow{
		{UserID: 1, NormalizedKeyword: "奶茶", Keyword: "奶茶", CreatedAt: now},
		{UserID: 2, NormalizedKeyword: "奶茶", Keyword: "奶茶", CreatedAt: now.Add(-time.Hour)},
		{UserID: 2, NormalizedKeyword: "奶茶", Keyword: "奶茶", CreatedAt: now.Add(-24 * time.Hour)},
		{UserID: 1, NormalizedKeyword: "自行车", Keyword: "自行车", CreatedAt: now},
		{UserID: 2, NormalizedKeyword: "自行车", Keyword: "自行车", CreatedAt: now.Add(-time.Hour)},
		{UserID: 3, NormalizedKeyword: "自行车", Keyword: "自行车", CreatedAt: now.Add(-2 * time.Hour)},
		{UserID: 9, NormalizedKeyword: "单人刷词", Keyword: "单人刷词", CreatedAt: now},
		{UserID: 9, NormalizedKeyword: "单人刷词", Keyword: "单人刷词", CreatedAt: now.Add(-time.Minute)},
		{UserID: 9, NormalizedKeyword: "单人刷词", Keyword: "单人刷词", CreatedAt: now.Add(-2 * time.Minute)},
		{UserID: 1, NormalizedKeyword: "过期", Keyword: "过期", CreatedAt: now.Add(-7 * 24 * time.Hour)},
		{UserID: 2, NormalizedKeyword: "过期", Keyword: "过期", CreatedAt: now.Add(-7 * 24 * time.Hour)},
		{UserID: 3, NormalizedKeyword: "过期", Keyword: "过期", CreatedAt: now.Add(-7 * 24 * time.Hour)},
	}

	got := rankHot(rows, now, 10)
	if len(got) != 2 {
		t.Fatalf("hot=%+v, want two eligible keywords", got)
	}
	if got[0].Keyword != "自行车" || got[0].Score != 21 || got[0].Count != 3 {
		t.Fatalf("first=%+v", got[0])
	}
	if got[1].Keyword != "奶茶" || got[1].Score != 20 || got[1].Count != 3 {
		t.Fatalf("second=%+v", got[1])
	}
}
