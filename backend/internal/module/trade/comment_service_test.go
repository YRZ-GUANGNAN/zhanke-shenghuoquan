package trade

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
)

const productCommentTestRequestID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"

type productCommentMemoryRepo struct {
	comments       map[uint]ProductComment
	requests       map[string]uint
	nextID         uint
	recentCount    int64
	duplicate      bool
	lastCreated    ProductComment
	lastEvent      WriteOutbox
	createReplayed bool
}

func newProductCommentMemoryRepo(values ...ProductComment) *productCommentMemoryRepo {
	repo := &productCommentMemoryRepo{
		comments: make(map[uint]ProductComment), requests: make(map[string]uint), nextID: 1,
	}
	for _, value := range values {
		repo.comments[value.ID] = value
		if value.ID >= repo.nextID {
			repo.nextID = value.ID + 1
		}
		if value.ClientRequestID != "" {
			repo.requests[productCommentRequestKey(value.UserID, value.ClientRequestID)] = value.ID
		}
	}
	return repo
}

func (r *productCommentMemoryRepo) FindProductComment(_ context.Context, id uint) (ProductComment, error) {
	value, ok := r.comments[id]
	if !ok {
		return ProductComment{}, ErrNotFound
	}
	return value, nil
}

func (r *productCommentMemoryRepo) FindProductCommentByRequest(_ context.Context, userID uint, key string) (ProductComment, bool, error) {
	id, ok := r.requests[productCommentRequestKey(userID, key)]
	if !ok {
		return ProductComment{}, false, nil
	}
	return r.comments[id], true, nil
}

func (r *productCommentMemoryRepo) CreateProductCommentOrReplay(_ context.Context, value *ProductComment, event *WriteOutbox) (bool, error) {
	if id, ok := r.requests[productCommentRequestKey(value.UserID, value.ClientRequestID)]; ok {
		existing := r.comments[id]
		if !bytes.Equal(existing.RequestHash, value.RequestHash) {
			return false, ErrIdempotencyConflict
		}
		*value = existing
		return true, nil
	}
	value.ID = r.nextID
	r.nextID++
	r.comments[value.ID] = *value
	r.requests[productCommentRequestKey(value.UserID, value.ClientRequestID)] = value.ID
	r.lastCreated, r.lastEvent = *value, *event
	return r.createReplayed, nil
}

func (r *productCommentMemoryRepo) ListProductCommentThreads(_ context.Context, productID, viewerID uint, _, _ int) ([]ProductComment, int64, error) {
	values := make([]ProductComment, 0)
	for _, value := range r.comments {
		if value.ProductID == productID && value.ParentCommentID == 0 &&
			(value.ReviewStatus == review.StatusApproved || value.UserID == viewerID) {
			values = append(values, value)
		}
	}
	return values, int64(len(values)), nil
}

func (r *productCommentMemoryRepo) ListProductCommentReplies(_ context.Context, parentIDs []uint, viewerID uint) ([]ProductComment, error) {
	parents := make(map[uint]bool, len(parentIDs))
	for _, id := range parentIDs {
		parents[id] = true
	}
	values := make([]ProductComment, 0)
	for _, value := range r.comments {
		if parents[value.ParentCommentID] &&
			(value.ReviewStatus == review.StatusApproved || value.UserID == viewerID) {
			values = append(values, value)
		}
	}
	return values, nil
}

func (r *productCommentMemoryRepo) CountRecentProductComments(context.Context, uint, time.Time) (int64, error) {
	return r.recentCount, nil
}

func (r *productCommentMemoryRepo) HasRecentDuplicateProductComment(context.Context, uint, uint, []byte, time.Time) (bool, error) {
	return r.duplicate, nil
}

func (r *productCommentMemoryRepo) SetProductCommentReviewStatus(_ context.Context, id uint, status review.Status) error {
	value, ok := r.comments[id]
	if !ok {
		return ErrNotFound
	}
	value.ReviewStatus = status
	r.comments[id] = value
	return nil
}

func productCommentRequestKey(userID uint, key string) string {
	return strconv.FormatUint(uint64(userID), 10) + ":" + key
}

type productCommentProductRepo struct {
	products map[uint]Product
}

func (r productCommentProductRepo) Find(_ context.Context, id uint, publicOnly bool) (Product, error) {
	value, ok := r.products[id]
	if !ok || (publicOnly && value.ReviewStatus != review.StatusApproved) {
		return Product{}, ErrNotFound
	}
	return value, nil
}

func (r productCommentProductRepo) FindProductForCommentHistory(_ context.Context, id uint) (Product, error) {
	value, ok := r.products[id]
	if !ok {
		return Product{}, ErrNotFound
	}
	return value, nil
}

type productCommentUsers struct{}

func (productCommentUsers) Profiles(_ context.Context, ids []uint) (map[uint]user.Profile, error) {
	values := make(map[uint]user.Profile, len(ids))
	for _, id := range ids {
		values[id] = user.Profile{ID: id, Nickname: "user"}
	}
	return values, nil
}

func newProductCommentTestService(repo *productCommentMemoryRepo, product Product) *ProductCommentService {
	service := NewProductCommentService(repo, productCommentProductRepo{
		products: map[uint]Product{product.ID: product},
	}, productCommentUsers{})
	service.now = func() time.Time { return time.Date(2026, 7, 29, 8, 0, 0, 0, time.UTC) }
	return service
}

func TestProductCommentServiceCreatesTopLevelAndReturnsSellerLabel(t *testing.T) {
	repo := newProductCommentMemoryRepo()
	product := Product{ID: 9, SellerID: 21, Status: StatusSelling, ReviewStatus: review.StatusApproved}
	service := newProductCommentTestService(repo, product)

	view, replayed, err := service.Create(context.Background(), 21, product.ID, ProductCommentInput{
		Content: "  卖家补充说明  ", ClientRequestID: productCommentTestRequestID,
	}, "wechat pass")
	if err != nil || replayed {
		t.Fatalf("create top-level comment: replayed=%v err=%v", replayed, err)
	}
	if view.Content != "卖家补充说明" || !view.IsSeller || view.ParentCommentID != 0 || view.ReplyTo != nil {
		t.Fatalf("unexpected view: %#v", view)
	}
	if repo.lastEvent.EventType != ProductCommentCreatedEventType || repo.lastCreated.ReviewStatus != review.StatusSubmitted {
		t.Fatalf("unexpected write contract: comment=%#v event=%#v", repo.lastCreated, repo.lastEvent)
	}
}

func TestProductCommentServiceInfersNestedReplyCoordinates(t *testing.T) {
	root := ProductComment{ID: 10, ProductID: 9, UserID: 31, ReviewStatus: review.StatusApproved}
	reply := ProductComment{ID: 11, ProductID: 9, UserID: 32, ParentCommentID: 10, ReplyToUserID: 31, ReviewStatus: review.StatusApproved}
	repo := newProductCommentMemoryRepo(root, reply)
	service := newProductCommentTestService(repo, Product{ID: 9, SellerID: 21, Status: StatusReserved, ReviewStatus: review.StatusApproved})

	view, _, err := service.Reply(context.Background(), 33, 9, reply.ID, ProductCommentInput{
		Content: "回复二级内容", ClientRequestID: productCommentTestRequestID,
	}, "wechat pass")
	if err != nil {
		t.Fatalf("reply nested comment: %v", err)
	}
	if view.ParentCommentID != root.ID || view.ReplyToUserID != reply.UserID || view.ReplyTo == nil || view.ReplyTo.ID != reply.UserID {
		t.Fatalf("server coordinates were not inferred: %#v", view)
	}
}

func TestProductCommentServiceRejectsUnavailableProductsAndCrossProductReplies(t *testing.T) {
	for _, status := range []ProductStatus{StatusSold, ""} {
		repo := newProductCommentMemoryRepo()
		service := newProductCommentTestService(repo, Product{ID: 9, SellerID: 21, Status: status, ReviewStatus: review.StatusApproved})
		_, _, err := service.Create(context.Background(), 31, 9, ProductCommentInput{
			Content: "hello", ClientRequestID: productCommentTestRequestID,
		}, "wechat pass")
		if !errors.Is(err, ErrConflict) {
			t.Fatalf("status %q: expected conflict, got %v", status, err)
		}
	}

	target := ProductComment{ID: 10, ProductID: 99, UserID: 31, ReviewStatus: review.StatusApproved}
	repo := newProductCommentMemoryRepo(target)
	service := newProductCommentTestService(repo, Product{ID: 9, Status: StatusSelling, ReviewStatus: review.StatusApproved})
	_, _, err := service.Reply(context.Background(), 32, 9, target.ID, ProductCommentInput{
		Content: "hello", ClientRequestID: productCommentTestRequestID,
	}, "wechat pass")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected cross-product reply to be hidden, got %v", err)
	}
}

func TestProductCommentServiceEnforcesContentRateAndDuplicateRules(t *testing.T) {
	product := Product{ID: 9, Status: StatusSelling, ReviewStatus: review.StatusApproved}
	tests := []struct {
		name    string
		content string
		setup   func(*productCommentMemoryRepo)
		want    error
	}{
		{name: "empty", content: " \n ", want: ErrInvalidProductComment},
		{name: "too long", content: string(bytes.Repeat([]byte("界"), 501)), want: ErrInvalidProductComment},
		{name: "rate", content: "hello", setup: func(r *productCommentMemoryRepo) { r.recentCount = 5 }, want: ErrProductCommentRateLimited},
		{name: "duplicate", content: "hello", setup: func(r *productCommentMemoryRepo) { r.duplicate = true }, want: ErrProductCommentDuplicate},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := newProductCommentMemoryRepo()
			if test.setup != nil {
				test.setup(repo)
			}
			service := newProductCommentTestService(repo, product)
			_, _, err := service.Create(context.Background(), 31, 9, ProductCommentInput{
				Content: test.content, ClientRequestID: productCommentTestRequestID,
			}, "wechat pass")
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestProductCommentServiceReplaysSameRequestAndConflictsOnChangedContent(t *testing.T) {
	repo := newProductCommentMemoryRepo()
	service := newProductCommentTestService(repo, Product{ID: 9, Status: StatusSelling, ReviewStatus: review.StatusApproved})
	first, replayed, err := service.Create(context.Background(), 31, 9, ProductCommentInput{
		Content: "hello", ClientRequestID: productCommentTestRequestID,
	}, "wechat pass")
	if err != nil || replayed {
		t.Fatalf("first create: replayed=%v err=%v", replayed, err)
	}
	second, replayed, err := service.Create(context.Background(), 31, 9, ProductCommentInput{
		Content: " hello ", ClientRequestID: productCommentTestRequestID,
	}, "wechat pass")
	if err != nil || !replayed || second.ID != first.ID {
		t.Fatalf("replay: first=%d second=%d replayed=%v err=%v", first.ID, second.ID, replayed, err)
	}
	_, _, err = service.Create(context.Background(), 31, 9, ProductCommentInput{
		Content: "changed", ClientRequestID: productCommentTestRequestID,
	}, "wechat pass")
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected payload conflict, got %v", err)
	}
}

func TestProductCommentServiceListsOnlyRepositoryVisibleTree(t *testing.T) {
	root := ProductComment{ID: 10, ProductID: 9, UserID: 31, Content: "root", ReviewStatus: review.StatusApproved}
	reply := ProductComment{ID: 11, ProductID: 9, UserID: 32, ParentCommentID: 10, ReplyToUserID: 31, Content: "reply", ReviewStatus: review.StatusApproved}
	hidden := ProductComment{ID: 12, ProductID: 9, UserID: 99, Content: "hidden", ReviewStatus: review.StatusSubmitted}
	repo := newProductCommentMemoryRepo(root, reply, hidden)
	service := newProductCommentTestService(repo, Product{ID: 9, SellerID: 32, Status: StatusSold, ReviewStatus: review.StatusApproved})

	page, err := service.List(context.Background(), 31, 9, 1, 20)
	if err != nil || page.Total != 1 || len(page.Items) != 1 || len(page.Items[0].Replies) != 1 {
		t.Fatalf("unexpected page: %#v err=%v", page, err)
	}
	if !page.Items[0].Replies[0].IsSeller {
		t.Fatalf("seller reply label was not derived from product seller")
	}
}

func TestProductCommentServiceKeepsApprovedHistoryAfterTakedown(t *testing.T) {
	root := ProductComment{
		ID: 10, ProductID: 9, UserID: 31, Content: "历史留言",
		ReviewStatus: review.StatusApproved,
	}
	repo := newProductCommentMemoryRepo(root)
	service := newProductCommentTestService(repo, Product{
		ID: 9, SellerID: 32, Status: StatusSelling, ReviewStatus: ReviewStatusTakenDown,
	})

	page, err := service.List(context.Background(), 44, 9, 1, 20)
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].Content != "历史留言" {
		t.Fatalf("takedown history unavailable: page=%#v err=%v", page, err)
	}
	_, _, err = service.Create(context.Background(), 44, 9, ProductCommentInput{
		Content: "不应新增", ClientRequestID: productCommentTestRequestID,
	}, "wechat pass")
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("takedown product accepted a new comment: %v", err)
	}
}
