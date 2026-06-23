package trade

import (
	"context"
	"errors"
	"testing"
	"time"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/im"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/media"
)

type tradeMemoryRepo struct {
	products      map[uint]Product
	events        []SearchOutbox
	processed     []uint
	failed        []uint
	forceConflict bool
	favorites     map[[2]uint]bool
}

func newTradeMemoryRepo(values ...Product) *tradeMemoryRepo {
	r := &tradeMemoryRepo{products: map[uint]Product{}, favorites: map[[2]uint]bool{}}
	for _, v := range values {
		r.products[v.ID] = v
	}
	return r
}
func (r *tradeMemoryRepo) Create(_ context.Context, v *Product) error {
	v.ID = uint(len(r.products) + 1)
	r.products[v.ID] = *v
	return nil
}
func (r *tradeMemoryRepo) Update(_ context.Context, v *Product, _ []ProductImage) error {
	if r.forceConflict {
		return ErrConflict
	}
	r.products[v.ID] = *v
	return nil
}
func (r *tradeMemoryRepo) Delete(_ context.Context, id, seller uint) error {
	v, ok := r.products[id]
	if !ok {
		return ErrNotFound
	}
	if v.SellerID != seller {
		return ErrForbidden
	}
	if v.Status == StatusSold {
		return ErrProductSold
	}
	delete(r.products, id)
	return nil
}
func (r *tradeMemoryRepo) Find(_ context.Context, id uint, public bool) (Product, error) {
	v, ok := r.products[id]
	if !ok || v.DeletedAt.Valid || (public && v.ReviewStatus != review.StatusApproved) {
		return Product{}, ErrNotFound
	}
	return v, nil
}
func (r *tradeMemoryRepo) FindForIndex(_ context.Context, id uint) (Product, error) {
	v, ok := r.products[id]
	if !ok {
		return Product{}, ErrNotFound
	}
	return v, nil
}
func (r *tradeMemoryRepo) List(_ context.Context, q ListQuery, public bool) ([]Product, int64, error) {
	if q.Keyword != "" {
		return nil, 0, ErrSearchRequired
	}
	values := []Product{}
	for _, v := range r.products {
		if v.DeletedAt.Valid || (public && v.ReviewStatus != review.StatusApproved) || (q.Status != "" && v.Status != q.Status) {
			continue
		}
		values = append(values, v)
	}
	return values, int64(len(values)), nil
}
func (r *tradeMemoryRepo) ListByIDs(_ context.Context, ids []uint, q ListQuery) ([]Product, error) {
	values := []Product{}
	for _, id := range ids {
		v, ok := r.products[id]
		if ok && !v.DeletedAt.Valid && v.ReviewStatus == review.StatusApproved && (q.Status == "" || v.Status == q.Status) {
			values = append(values, v)
		}
	}
	return values, nil
}
func (r *tradeMemoryRepo) ListSeller(_ context.Context, id uint, _, _ int) ([]Product, int64, error) {
	values := []Product{}
	for _, v := range r.products {
		if v.SellerID == id && !v.DeletedAt.Valid {
			values = append(values, v)
		}
	}
	return values, int64(len(values)), nil
}
func (r *tradeMemoryRepo) SetStatus(_ context.Context, id, seller uint, from, to ProductStatus) error {
	if r.forceConflict {
		return ErrConflict
	}
	v := r.products[id]
	if v.SellerID != seller || v.Status != from || v.ReviewStatus != review.StatusApproved {
		return ErrConflict
	}
	v.Status = to
	r.products[id] = v
	return nil
}
func (r *tradeMemoryRepo) ToggleFavorite(_ context.Context, id, userID uint, allow bool) (bool, error) {
	key := [2]uint{id, userID}
	if r.favorites[key] {
		delete(r.favorites, key)
		return false, nil
	}
	if !allow {
		return false, ErrProductSold
	}
	r.favorites[key] = true
	return true, nil
}
func (r *tradeMemoryRepo) SetReviewStatus(_ context.Context, id uint, status review.Status) error {
	v := r.products[id]
	v.ReviewStatus = status
	r.products[id] = v
	return nil
}
func (r *tradeMemoryRepo) AdminTakedown(_ context.Context, id uint) error {
	v, ok := r.products[id]
	if !ok {
		return ErrNotFound
	}
	if v.ReviewStatus == ReviewStatusTakenDown {
		return nil
	}
	v.ReviewStatus = ReviewStatusTakenDown
	r.products[id] = v
	r.events = append(r.events, SearchOutbox{ProductID: id, Operation: SearchDelete})
	return nil
}
func (r *tradeMemoryRepo) QueueSearchRebuild(context.Context) error { return nil }
func (r *tradeMemoryRepo) PendingSearchEvents(_ context.Context, limit int) ([]SearchOutbox, error) {
	if len(r.events) < limit {
		limit = len(r.events)
	}
	return r.events[:limit], nil
}
func (r *tradeMemoryRepo) MarkSearchEventProcessed(_ context.Context, id uint) error {
	r.processed = append(r.processed, id)
	return nil
}
func (r *tradeMemoryRepo) MarkSearchEventFailed(_ context.Context, id uint, _ int, _ string) error {
	r.failed = append(r.failed, id)
	return nil
}
func (r *tradeMemoryRepo) Count(context.Context) (int64, int64, error) {
	return int64(len(r.products)), 0, nil
}

type tradeSearchFake struct {
	ids     []uint
	total   int64
	queries []string
	upserts []uint
	deletes []uint
	fail    bool
}

func (s *tradeSearchFake) Configure(context.Context) error { return nil }
func (s *tradeSearchFake) Search(_ context.Context, keyword string, _ ListQuery) ([]uint, int64, error) {
	s.queries = append(s.queries, keyword)
	if s.fail {
		return nil, 0, ErrSearchUnavailable
	}
	return s.ids, s.total, nil
}
func (s *tradeSearchFake) Upsert(_ context.Context, v Product) error {
	s.upserts = append(s.upserts, v.ID)
	return nil
}
func (s *tradeSearchFake) Delete(_ context.Context, id uint) error {
	s.deletes = append(s.deletes, id)
	return nil
}
func (s *tradeSearchFake) Close() {}

type tradeUsers map[uint]user.Profile

func (u tradeUsers) Profiles(_ context.Context, ids []uint) (map[uint]user.Profile, error) {
	values := map[uint]user.Profile{}
	for _, id := range ids {
		values[id] = u[id]
	}
	return values, nil
}

type tradeReviews struct{}

func (tradeReviews) Submit(_ context.Context, v review.Submission) (review.Task, error) {
	return review.Task{ObjectType: v.ObjectType, ObjectID: v.ObjectID}, nil
}

type tradeMediaFake struct{ committed, deleted bool }

func (f *tradeMediaFake) Reserve(_ context.Context, _ uint, purpose media.Purpose, ids []string) (media.Reservation, error) {
	if purpose != media.PurposeProductImage {
		return media.Reservation{}, errors.New("wrong purpose")
	}
	return media.Reservation{Token: "token", References: []media.Reference{{InternalID: 81, PublicID: ids[0], URL: "/api/v1/media/assets/product/content"}}}, nil
}
func (f *tradeMediaFake) Commit(context.Context, uint, media.Reservation, string, uint64) error {
	f.committed = true
	return nil
}
func (f *tradeMediaFake) Cancel(context.Context, uint, media.Reservation) error { return nil }
func (f *tradeMediaFake) DeleteReference(context.Context, string, uint64, []uint64) error {
	f.deleted = true
	return nil
}

func TestProductImageBindingDoesNotChangeTransactionState(t *testing.T) {
	repo := newTradeMemoryRepo()
	gateway := &tradeMediaFake{}
	service := NewService(repo, tradeUsers{}, tradeReviews{}, &tradeConversations{}, &tradeSearchFake{}, gateway)
	product, err := service.Create(context.Background(), ProductInput{SellerID: 4, Title: "product", Price: 100, ImageAssetIDs: []string{"asset"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if product.Status != StatusSelling || product.ReviewStatus != review.StatusSubmitted {
		t.Fatalf("transaction state changed: %+v", product)
	}
	if !gateway.committed || len(product.Images) != 1 || product.Images[0].AssetID == nil || *product.Images[0].AssetID != 81 {
		t.Fatalf("media not bound: %+v", product.Images)
	}
	if err := service.Delete(context.Background(), product.ID, 4); err != nil || !gateway.deleted {
		t.Fatalf("media cleanup not requested: %v", err)
	}
}

type tradeConversations struct{ calls int }

func (c *tradeConversations) EnsureProductConversation(_ context.Context, p, b, s uint) (im.ConversationContext, error) {
	c.calls++
	return im.ConversationContext{ProductID: p, BuyerID: b, SellerID: s}, nil
}

func newTradeService(repo *tradeMemoryRepo, search SearchGateway, conversations *tradeConversations) *Service {
	return NewService(repo, tradeUsers{1: {ID: 1, Nickname: "seller"}}, tradeReviews{}, conversations, search)
}

func TestProductStatusTransitions(t *testing.T) {
	tests := []struct {
		from, to ProductStatus
		want     bool
	}{{StatusSelling, StatusReserved, true}, {StatusSelling, StatusSold, true}, {StatusReserved, StatusSelling, true}, {StatusReserved, StatusSold, true}, {StatusSold, StatusSelling, false}, {StatusSelling, StatusSelling, false}}
	for _, tt := range tests {
		if got := validTransition(tt.from, tt.to); got != tt.want {
			t.Errorf("%s -> %s = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestSearchUsesMeilisearchThenDatabaseTruth(t *testing.T) {
	repo := newTradeMemoryRepo(Product{ID: 1, SellerID: 1, Status: StatusSelling, ReviewStatus: review.StatusApproved, Title: "approved"}, Product{ID: 2, SellerID: 1, Status: StatusSelling, ReviewStatus: review.StatusRejected, Title: "hidden"})
	search := &tradeSearchFake{ids: []uint{2, 1}, total: 2}
	service := newTradeService(repo, search, &tradeConversations{})
	page, err := service.List(context.Background(), ListQuery{Keyword: "book", Status: StatusSelling, Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(search.queries) != 1 || search.queries[0] != "book" {
		t.Fatal("Meilisearch was not called")
	}
	if len(page.Items) != 1 || page.Items[0].ID != 1 {
		t.Fatalf("database truth filter failed: %+v", page.Items)
	}
}

func TestStatusCASOwnershipAndSoldTerminal(t *testing.T) {
	repo := newTradeMemoryRepo(Product{ID: 1, SellerID: 1, Status: StatusSelling, ReviewStatus: review.StatusApproved})
	service := newTradeService(repo, &tradeSearchFake{}, &tradeConversations{})
	if err := service.ChangeStatus(context.Background(), 1, 2, StatusReserved); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-seller changed status: %v", err)
	}
	if err := service.ChangeStatus(context.Background(), 1, 1, StatusReserved); err != nil {
		t.Fatal(err)
	}
	if err := service.ChangeStatus(context.Background(), 1, 1, StatusSold); err != nil {
		t.Fatal(err)
	}
	if err := service.ChangeStatus(context.Background(), 1, 1, StatusSelling); err == nil {
		t.Fatal("sold terminal was reopened")
	}
	repo.products[1] = Product{ID: 1, SellerID: 1, Status: StatusSelling, ReviewStatus: review.StatusApproved}
	repo.forceConflict = true
	if err := service.ChangeStatus(context.Background(), 1, 1, StatusReserved); !errors.Is(err, ErrConflict) {
		t.Fatalf("CAS conflict not surfaced: %v", err)
	}
}

func TestSoldProductBlocksMutationFavoriteAndContact(t *testing.T) {
	repo := newTradeMemoryRepo(Product{ID: 1, SellerID: 1, Status: StatusSold, ReviewStatus: review.StatusApproved})
	conversations := &tradeConversations{}
	service := newTradeService(repo, &tradeSearchFake{}, conversations)
	input := ProductInput{SellerID: 1, Title: "x", Price: 1}
	if _, err := service.Update(context.Background(), 1, input, ""); !errors.Is(err, ErrProductSold) {
		t.Fatalf("sold edit allowed: %v", err)
	}
	if err := service.Delete(context.Background(), 1, 1); !errors.Is(err, ErrProductSold) {
		t.Fatalf("sold delete allowed: %v", err)
	}
	if _, err := service.ToggleFavorite(context.Background(), 1, 2); !errors.Is(err, ErrProductSold) {
		t.Fatalf("sold favorite allowed: %v", err)
	}
	if _, err := service.ContactSeller(context.Background(), 1, 2); !errors.Is(err, ErrProductSold) {
		t.Fatalf("sold contact allowed: %v", err)
	}
	if conversations.calls != 0 {
		t.Fatal("sold product reached IM gateway")
	}
}

func TestSearchOutboxAlwaysConvergesToDatabaseTruth(t *testing.T) {
	deleted := Product{ID: 4, SellerID: 1, Status: StatusSelling, ReviewStatus: review.StatusApproved}
	deleted.DeletedAt.Valid = true
	repo := newTradeMemoryRepo(Product{ID: 1, SellerID: 1, Status: StatusSelling, ReviewStatus: review.StatusApproved}, Product{ID: 2, SellerID: 1, Status: StatusSelling, ReviewStatus: review.StatusRejected}, deleted)
	repo.events = []SearchOutbox{{ID: 11, ProductID: 1, Operation: SearchDelete}, {ID: 12, ProductID: 2, Operation: SearchUpsert}, {ID: 13, ProductID: 3, Operation: SearchUpsert}, {ID: 14, ProductID: 4, Operation: SearchUpsert}}
	search := &tradeSearchFake{}
	service := newTradeService(repo, search, &tradeConversations{})
	processed, err := service.ProcessSearchBatch(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if processed != 4 || len(search.upserts) != 1 || search.upserts[0] != 1 || len(search.deletes) != 3 {
		t.Fatalf("unexpected convergence: processed=%d upsert=%v delete=%v", processed, search.upserts, search.deletes)
	}
}

func TestSearchWorkerStopsOnContext(t *testing.T) {
	repo := newTradeMemoryRepo()
	service := newTradeService(repo, &tradeSearchFake{}, &tradeConversations{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.RunSearchWorker(ctx, time.Millisecond); !errors.Is(err, context.Canceled) {
		t.Fatalf("worker did not stop: %v", err)
	}
}
