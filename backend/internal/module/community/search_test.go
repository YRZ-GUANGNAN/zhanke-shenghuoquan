package community

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNormalizeTags(t *testing.T) {
	got, err := normalizeTags([]string{" #求助 ", "求助", "#搭子", "攻略"})
	if err != nil || !reflect.DeepEqual(got, []string{"求助", "搭子", "攻略"}) {
		t.Fatalf("tags=%v err=%v", got, err)
	}
	for _, values := range [][]string{{""}, {strings.Repeat("标", 13)}, {"一", "二", "三", "四"}, {"坏\n标签"}} {
		if _, err := normalizeTags(values); err == nil {
			t.Fatalf("normalizeTags(%q) unexpectedly succeeded", values)
		}
	}
}

func TestPostCreateRequestHashKeepsLegacyV1AndSeparatesTags(t *testing.T) {
	legacy := CreatePostInput{Title: "标题", Content: "正文", Topic: "日常", ImageAssetIDs: []string{"asset"}}
	got, err := postCreateRequestHash(legacy)
	if err != nil {
		t.Fatal(err)
	}
	want, err := canonicalCommunityHash(struct {
		Version       string   `json:"version"`
		Title         string   `json:"title"`
		Content       string   `json:"content"`
		Topic         string   `json:"topic"`
		ImageAssetIDs []string `json:"imageAssetIds"`
	}{"community-post-create:v1", "标题", "正文", "日常", []string{"asset"}})
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("legacy hash changed: got=%x want=%x err=%v", got, want, err)
	}
	withTags := legacy
	withTags.Tags = []string{"求助"}
	tagged, err := postCreateRequestHash(withTags)
	if err != nil || reflect.DeepEqual(tagged, got) {
		t.Fatalf("tagged hash=%x legacy=%x err=%v", tagged, got, err)
	}
}

func TestRankHotTagsUsesUsageBeforeInteractions(t *testing.T) {
	now := time.Date(2026, 7, 19, 4, 0, 0, 0, time.UTC)
	posts := []Post{
		{Tags: []string{"求助", "搭子"}, CreatedAt: now, LikedCount: 1},
		{Tags: []string{"求助"}, CreatedAt: now.Add(-24 * time.Hour), CommentCount: 1},
		{Tags: []string{"搭子"}, CreatedAt: now, FavoriteCount: 100},
	}
	got := rankHotTags(posts, now, 12)
	if len(got) != 2 || got[0].Tag != "搭子" || got[0].UsageScore != 14 || got[1].Tag != "求助" || got[1].UsageScore != 13 {
		t.Fatalf("hot tags=%+v", got)
	}
}
