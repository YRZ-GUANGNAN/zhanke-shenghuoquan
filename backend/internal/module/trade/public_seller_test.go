package trade

import (
	"context"
	"testing"

	"campus-circle/backend/internal/module/admin/review"
)

type publicSellerRepoStub struct {
	Repository
	products []Product
}

func (r publicSellerRepoStub) ListPublicSeller(context.Context, uint, int, int) ([]Product, int64, error) {
	return r.products, int64(len(r.products)), nil
}

func TestUserProductsReturnsEnrichedPublicSellerItems(t *testing.T) {
	repo := publicSellerRepoStub{products: []Product{{
		ID: 21, SellerID: 7, Title: "真实二手", Status: StatusSelling, ReviewStatus: review.StatusApproved,
	}}}
	service := NewService(repo, tradeUsers{7: {ID: 7, Nickname: "真实卖家"}}, tradeReviews{}, nil, nil)
	page, err := service.UserProducts(context.Background(), 7, 1, 20)
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].Seller.Nickname != "真实卖家" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
}
