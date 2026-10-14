package trade

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/searchmetrics"
	"github.com/gin-gonic/gin"
)

type tradeSearchRepositoryStub struct {
	*tradeMemoryRepo
	values   []Product
	total    int64
	keyword  string
	viewerID uint
	page     int
	size     int
	filters  SearchFilters
}

func (r *tradeSearchRepositoryStub) SearchProducts(_ context.Context, viewerID uint, keyword string, page, size int, options ...SearchFilters) ([]Product, int64, error) {
	r.viewerID, r.keyword, r.page, r.size = viewerID, keyword, page, size
	if len(options) > 0 {
		r.filters = options[0]
	}
	return r.values, r.total, nil
}

type filterSearchMetrics struct {
	searchmetrics.Metrics
	calls int
}

func (m *filterSearchMetrics) Confirm(_ context.Context, _ uint, keyword string) (string, error) {
	m.calls++
	return strings.TrimSpace(keyword), nil
}

func TestTradeSearchFilterHTTPContract(t *testing.T) {
	repository := &tradeSearchRepositoryStub{tradeMemoryRepo: newTradeMemoryRepo(), values: []Product{}}
	metrics := &filterSearchMetrics{}
	handler := NewSearchHandler(NewService(repository, tradeUsers{}, tradeReviews{}, nil, nil), nil, metrics)
	run := func(body string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest("POST", "/searches", strings.NewReader(body))
		ctx.Request.Header.Set("Content-Type", "application/json")
		user.SetCurrentUser(ctx, user.User{ID: 73})
		handler.search(ctx)
		return recorder
	}
	for _, body := range []string{`{"keyword":"教材","filters":{"minPrice":-1}}`, `{"keyword":"教材","filters":{"maxPrice":10.01}}`, `{"keyword":"教材","filters":{"sort":"unsupported"}}`} {
		if value := run(body); value.Code != 400 || metrics.calls != 0 {
			t.Fatalf("invalid request recorded history: %s", value.Body)
		}
	}
	response := run(`{"keyword":" 教材 ","page":2,"pageSize":5,"filters":{"sort":"price_asc","category":"书籍资料","minPrice":0,"maxPrice":1001}}`)
	if response.Code != 200 {
		t.Fatal(response.Body.String())
	}
	filters := repository.filters
	if repository.viewerID != 73 || repository.keyword != "教材" || repository.page != 2 || repository.size != 5 || filters.Sort != "price_asc" || filters.Category != "书籍资料" || filters.MinPrice == nil || *filters.MinPrice != 0 || filters.MaxPrice == nil || *filters.MaxPrice != 1001 {
		t.Fatalf("query=%+v", repository)
	}
	var payload struct {
		Data struct {
			Page
			FilterVersion int `json:"filterVersion"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil || payload.Data.FilterVersion != 1 || payload.Data.Page.Page != 2 || payload.Data.Items == nil {
		t.Fatalf("payload=%s err=%v", response.Body, err)
	}
	if old := run(`{"keyword":"教材"}`); old.Code != 200 || repository.filters != (SearchFilters{}) || repository.page != 1 || repository.size != 20 {
		t.Fatalf("old payload broke: %s", old.Body)
	}
}

func TestSearchProductsUsesDedicatedRepositoryAndEnriches(t *testing.T) {
	product := Product{ID: 7, SellerID: 3, Title: "自行车", Status: StatusSelling, ReviewStatus: review.StatusApproved}
	repository := &tradeSearchRepositoryStub{tradeMemoryRepo: newTradeMemoryRepo(product), values: []Product{product}, total: 1}
	service := NewService(repository, tradeUsers{}, tradeReviews{}, nil, nil)
	page, err := service.SearchProducts(context.Background(), 9, "  自行车  ", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if repository.viewerID != 9 || repository.keyword != "自行车" || repository.page != 1 || repository.size != 20 {
		t.Fatalf("query viewer=%d keyword=%q page=%d size=%d", repository.viewerID, repository.keyword, repository.page, repository.size)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != product.ID {
		t.Fatalf("page=%+v", page)
	}
}
