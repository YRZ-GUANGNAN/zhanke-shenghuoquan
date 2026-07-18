package trade

import (
	"context"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
)

type tradeSearchRepositoryStub struct {
	*tradeMemoryRepo
	values   []Product
	total    int64
	keyword  string
	viewerID uint
	page     int
	size     int
}

func (r *tradeSearchRepositoryStub) SearchProducts(_ context.Context, viewerID uint, keyword string, page, size int) ([]Product, int64, error) {
	r.viewerID, r.keyword, r.page, r.size = viewerID, keyword, page, size
	return r.values, r.total, nil
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
