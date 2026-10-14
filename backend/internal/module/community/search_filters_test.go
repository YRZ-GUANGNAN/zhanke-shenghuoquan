package community

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/searchmetrics"
	"github.com/gin-gonic/gin"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type filterSearchRepo struct {
	*communityMemoryRepo
	viewer     uint
	keyword    string
	page, size int
	filters    SearchFilters
}

func (r *filterSearchRepo) SearchPosts(_ context.Context, viewer uint, keyword string, page, size int, filters ...SearchFilters) ([]Post, int64, error) {
	r.viewer, r.keyword, r.page, r.size = viewer, keyword, page, size
	r.filters = filters[0]
	return []Post{}, 0, nil
}
func (*filterSearchRepo) ListRecentTagPosts(context.Context, time.Time) ([]Post, error) {
	return nil, nil
}

type filterSearchMetrics struct {
	searchmetrics.Metrics
	calls int
}

func (m *filterSearchMetrics) Confirm(_ context.Context, _ uint, keyword string) (string, error) {
	m.calls++
	return strings.TrimSpace(keyword), nil
}

func TestSearchFilterHTTPContractAndOldPayload(t *testing.T) {
	repository := &filterSearchRepo{communityMemoryRepo: &communityMemoryRepo{posts: map[uint]Post{}}}
	metrics := &filterSearchMetrics{}
	handler := NewSearchHandler(NewService(repository, profileDirectory{}, &submissionRecorder{}), nil, metrics)
	run := func(body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest("POST", "/searches", strings.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		user.SetCurrentUser(ctx, user.User{ID: 73})
		handler.search(ctx)
		return recorder
	}
	invalid := run(`{"keyword":"校园","filters":{"sort":"unsupported"}}`)
	if invalid.Code != 400 || metrics.calls != 0 {
		t.Fatalf("invalid request recorded history: %d %d", invalid.Code, metrics.calls)
	}
	response := run(`{"keyword":" 校园 ","page":2,"pageSize":5,"filters":{"sort":"comments","contentType":"image","withinDays":7}}`)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	want := SearchFilters{Sort: "comments", ContentType: "image", WithinDays: 7}
	if repository.viewer != 73 || repository.keyword != "校园" || repository.page != 2 || repository.size != 5 || !reflect.DeepEqual(repository.filters, want) {
		t.Fatalf("query=%+v", repository)
	}
	var payload struct {
		Data struct {
			Page[PostView]
			FilterVersion int `json:"filterVersion"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload.Data.FilterVersion != 1 || payload.Data.Page.Page != 2 || payload.Data.Items == nil {
		t.Fatalf("payload=%s err=%v", response.Body, err)
	}
	if old := run(`{"keyword":"校园"}`); old.Code != 200 || repository.filters != (SearchFilters{}) || repository.page != 1 || repository.size != 20 {
		t.Fatalf("old payload broke: %s", old.Body)
	}
}

func TestSearchFiltersValidation(t *testing.T) {
	for _, value := range []SearchFilters{{}, {Sort: "relevance", ContentType: "all"}, {Sort: "recent", ContentType: "image", WithinDays: 7}, {Sort: "likes", ContentType: "text", WithinDays: 1}, {Sort: "comments", WithinDays: 180}, {Sort: "favorites"}} {
		if err := validateSearchFilters(value); err != nil {
			t.Fatalf("valid filters %+v: %v", value, err)
		}
	}
	for _, value := range []SearchFilters{{Sort: "id DESC; DROP TABLE posts"}, {ContentType: "video"}, {WithinDays: -1}, {WithinDays: 2}} {
		if err := validateSearchFilters(value); err == nil {
			t.Fatalf("accepted invalid filters %+v", value)
		}
	}
}

func TestSearchFiltersSQLKeepsVisibilityAndFiltersBeforePagination(t *testing.T) {
	// DryRun + disabled initialization/ping: no database or network connection is used.
	db, err := gorm.Open(mysql.New(mysql.Config{DSN: "unused:unused@tcp(127.0.0.1:1)/unused", SkipInitializeWithVersion: true}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
	if err != nil {
		t.Fatal(err)
	}
	r := NewRepository(db)
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	filters := SearchFilters{Sort: "likes", ContentType: "image", WithinDays: 7}
	query := func() *gorm.DB { return r.searchPostsQuery(context.Background(), 73, "校园' OR 1=1", filters, now) }
	var posts []Post
	statement := query().Offset(20).Limit(20).Find(&posts).Statement
	sql := statement.SQL.String()
	for _, want := range []string{"review_status = ?", "user_blocks", "ub.blocked_id = posts.user_id", "EXISTS (SELECT 1 FROM post_images", "posts.created_at >= ?", "`posts`.`deleted_at` IS NULL", "ORDER BY liked_count DESC, created_at DESC, id DESC", "LIMIT ? OFFSET ?"} {
		if !strings.Contains(sql, want) {
			t.Fatalf("missing %q in %s", want, sql)
		}
	}
	if strings.Contains(sql, "校园") {
		t.Fatalf("keyword interpolated in SQL: %s", sql)
	}
	var total int64
	count := query().Count(&total).Statement.SQL.String()
	if !strings.Contains(count, "post_images") || !strings.Contains(count, "posts.created_at >= ?") || strings.Contains(count, "LIMIT") {
		t.Fatalf("count did not use full filtered scope: %s", count)
	}
	textOnly := r.searchPostsQuery(context.Background(), 73, "校园", SearchFilters{ContentType: "text"}, now).Find(&posts).Statement.SQL.String()
	if !strings.Contains(textOnly, "NOT EXISTS (SELECT 1 FROM post_images") || !strings.Contains(textOnly, "ORDER BY CASE") {
		t.Fatalf("text/relevance SQL=%s", textOnly)
	}
	for sortKey, column := range map[string]string{"recent": "created_at DESC, id DESC", "comments": "comment_count DESC", "favorites": "favorite_count DESC"} {
		ordered := r.searchPostsQuery(context.Background(), 73, "校园", SearchFilters{Sort: sortKey}, now).Find(&posts).Statement.SQL.String()
		if !strings.Contains(ordered, "ORDER BY "+column) {
			t.Fatalf("sort %s: %s", sortKey, ordered)
		}
	}
}
