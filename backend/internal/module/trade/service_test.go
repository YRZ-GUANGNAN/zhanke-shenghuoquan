package trade

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
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

type tradeKeyedMemoryRepo struct {
	*tradeMemoryRepo
	productsByRequest map[string]uint
	outboxes          []WriteOutbox
}

func newTradeKeyedMemoryRepo() *tradeKeyedMemoryRepo {
	return &tradeKeyedMemoryRepo{tradeMemoryRepo: newTradeMemoryRepo(), productsByRequest: map[string]uint{}}
}

func (r *tradeKeyedMemoryRepo) requestCoordinate(ownerID uint, requestID string) string {
	return fmt.Sprintf("%d:%s", ownerID, requestID)
}

func (r *tradeKeyedMemoryRepo) FindProductByRequest(_ context.Context, ownerID uint, requestID string) (Product, bool, error) {
	id, ok := r.productsByRequest[r.requestCoordinate(ownerID, requestID)]
	if !ok {
		return Product{}, false, nil
	}
	return r.products[id], true, nil
}

func (r *tradeKeyedMemoryRepo) CreateProductOrReplay(_ context.Context, value *Product, event *WriteOutbox) (bool, error) {
	coordinate := r.requestCoordinate(value.SellerID, *value.ClientRequestID)
	if id, ok := r.productsByRequest[coordinate]; ok {
		existing := r.products[id]
		if !bytes.Equal(existing.CreateRequestHash, value.CreateRequestHash) {
			return false, ErrIdempotencyConflict
		}
		*value = existing
		return true, nil
	}
	value.ID = uint(len(r.products) + 1)
	r.products[value.ID] = *value
	r.productsByRequest[coordinate] = value.ID
	event.AggregateType = "product"
	event.AggregateID = uint64(value.ID)
	event.OwnerUserID = uint64(value.SellerID)
	r.outboxes = append(r.outboxes, *event)
	return false, nil
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
	v.ReviewStatus = review.StatusSubmitted
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
func (r *tradeMemoryRepo) IsFavorited(_ context.Context, id, userID uint) (bool, error) {
	return r.favorites[[2]uint{id, userID}], nil
}
func (r *tradeMemoryRepo) SetFavorite(_ context.Context, id, userID uint, desired, allow bool) (bool, error) {
	key := [2]uint{id, userID}
	if r.favorites[key] == desired {
		return desired, nil
	}
	if desired && !allow {
		return false, ErrProductSold
	}
	r.favorites[key] = desired
	if !desired {
		delete(r.favorites, key)
	}
	value := r.products[id]
	if desired {
		value.FavoriteCount++
	} else if value.FavoriteCount > 0 {
		value.FavoriteCount--
	}
	r.products[id] = value
	r.events = append(r.events, SearchOutbox{ProductID: id, Operation: SearchUpsert})
	return desired, nil
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

type tradeMediaFake struct {
	committed, deleted bool
	reserveCalls       int
	cancelCalls        int
}

type tradeMediaLifecycleFake struct {
	reservation    media.Reservation
	calls          []string
	reserveOwner   uint
	reserveProduct uint
	deleteKeep     []uint64
	onDelete       func()
	onPublish      func()
}

func (f *tradeMediaLifecycleFake) Reserve(context.Context, uint, media.Purpose, []string) (media.Reservation, error) {
	f.calls = append(f.calls, "reserve")
	return f.reservation, nil
}
func (f *tradeMediaLifecycleFake) ReserveForProduct(_ context.Context, ownerID, productID uint, _ media.Purpose, _ []string) (media.Reservation, error) {
	f.reserveOwner, f.reserveProduct = ownerID, productID
	f.calls = append(f.calls, fmt.Sprintf("reserve:%d", productID))
	return f.reservation, nil
}
func (f *tradeMediaLifecycleFake) Commit(_ context.Context, _ uint, _ media.Reservation, _ string, referenceID uint64) error {
	f.calls = append(f.calls, fmt.Sprintf("commit:%d", referenceID))
	return nil
}
func (f *tradeMediaLifecycleFake) Cancel(context.Context, uint, media.Reservation) error {
	f.calls = append(f.calls, "cancel")
	return nil
}
func (f *tradeMediaLifecycleFake) DeleteReference(_ context.Context, _ string, referenceID uint64, keep []uint64) error {
	f.deleteKeep = append([]uint64(nil), keep...)
	f.calls = append(f.calls, fmt.Sprintf("delete:%d", referenceID))
	if f.onDelete != nil {
		f.onDelete()
	}
	return nil
}
func (f *tradeMediaLifecycleFake) PublishProduct(_ context.Context, productID uint64) error {
	f.calls = append(f.calls, fmt.Sprintf("publish:%d", productID))
	if f.onPublish != nil {
		f.onPublish()
	}
	return nil
}

func (f *tradeMediaFake) Reserve(_ context.Context, _ uint, purpose media.Purpose, ids []string) (media.Reservation, error) {
	f.reserveCalls++
	if purpose != media.PurposeProductImage {
		return media.Reservation{}, errors.New("wrong purpose")
	}
	return media.Reservation{Token: "token", References: []media.Reference{{InternalID: 81, PublicID: ids[0], URL: "/api/v1/media/assets/product/content"}}}, nil
}
func (f *tradeMediaFake) Commit(context.Context, uint, media.Reservation, string, uint64) error {
	f.committed = true
	return nil
}
func (f *tradeMediaFake) Cancel(context.Context, uint, media.Reservation) error {
	f.cancelCalls++
	return nil
}
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

func TestProductApprovalPublishesMediaAfterParentApproval(t *testing.T) {
	repo := newTradeMemoryRepo(Product{ID: 42, SellerID: 7, Status: StatusSelling, ReviewStatus: review.StatusReviewing})
	gateway := &tradeMediaLifecycleFake{}
	gateway.onPublish = func() {
		if repo.products[42].ReviewStatus != review.StatusApproved {
			t.Fatal("media published before parent approval")
		}
	}
	service := NewService(repo, tradeUsers{}, tradeReviews{}, &tradeConversations{}, &tradeSearchFake{}, gateway)
	if err := service.SetReviewStatus(context.Background(), 42, review.StatusApproved); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gateway.calls, []string{"publish:42"}) || repo.products[42].ReviewStatus != review.StatusApproved {
		t.Fatalf("calls=%v product=%+v", gateway.calls, repo.products[42])
	}
}

func TestProductRejectionHidesMediaBeforeParentRejection(t *testing.T) {
	repo := newTradeMemoryRepo(Product{ID: 42, SellerID: 7, Status: StatusSelling, ReviewStatus: review.StatusReviewing})
	gateway := &tradeMediaLifecycleFake{}
	gateway.onDelete = func() {
		if repo.products[42].ReviewStatus != review.StatusReviewing {
			t.Fatal("parent rejected before media was hidden")
		}
	}
	service := NewService(repo, tradeUsers{}, tradeReviews{}, &tradeConversations{}, &tradeSearchFake{}, gateway)
	if err := service.SetReviewStatus(context.Background(), 42, review.StatusRejected); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gateway.calls, []string{"delete:42"}) || repo.products[42].ReviewStatus != review.StatusRejected {
		t.Fatalf("calls=%v product=%+v", gateway.calls, repo.products[42])
	}
}

func TestProductDeleteHidesMediaBeforeParentDelete(t *testing.T) {
	repo := newTradeMemoryRepo(Product{ID: 42, SellerID: 7, Status: StatusSelling, ReviewStatus: review.StatusApproved})
	gateway := &tradeMediaLifecycleFake{}
	gateway.onDelete = func() {
		if _, exists := repo.products[42]; !exists {
			t.Fatal("parent deleted before media was hidden")
		}
	}
	service := NewService(repo, tradeUsers{}, tradeReviews{}, &tradeConversations{}, &tradeSearchFake{}, gateway)
	if err := service.Delete(context.Background(), 42, 7); err != nil {
		t.Fatal(err)
	}
	if _, exists := repo.products[42]; exists || !reflect.DeepEqual(gateway.calls, []string{"delete:42"}) {
		t.Fatalf("calls=%v productExists=%v", gateway.calls, exists)
	}
}

func TestProductAdminTakedownHidesMediaBeforeParentTakedown(t *testing.T) {
	repo := newTradeMemoryRepo(Product{ID: 42, SellerID: 7, Status: StatusSelling, ReviewStatus: review.StatusApproved})
	gateway := &tradeMediaLifecycleFake{}
	gateway.onDelete = func() {
		if repo.products[42].ReviewStatus == ReviewStatusTakenDown {
			t.Fatal("parent taken down before media was hidden")
		}
	}
	service := NewService(repo, tradeUsers{}, tradeReviews{}, &tradeConversations{}, &tradeSearchFake{}, gateway)
	if err := service.AdminTakedown(context.Background(), 42, 9, "unsafe listing"); err != nil {
		t.Fatal(err)
	}
	if repo.products[42].ReviewStatus != ReviewStatusTakenDown || !reflect.DeepEqual(gateway.calls, []string{"delete:42"}) {
		t.Fatalf("calls=%v product=%+v", gateway.calls, repo.products[42])
	}
}

func TestProductUpdateUsesReserveForProductAndHidesRemovedBeforeParentUpdate(t *testing.T) {
	repo := newTradeMemoryRepo(Product{ID: 42, SellerID: 7, Title: "old", Status: StatusSelling, ReviewStatus: review.StatusApproved})
	gateway := &tradeMediaLifecycleFake{reservation: media.Reservation{
		Token: "m10_01J00000000000000000000001",
		References: []media.Reference{
			{InternalID: 11, PublicID: "01J00000000000000000000001", URL: "/api/v1/media/assets/1/content"},
			{InternalID: 12, PublicID: "01J00000000000000000000002", URL: "/api/v1/media/assets/2/content"},
		},
	}}
	gateway.onDelete = func() {
		if repo.products[42].Title != "old" {
			t.Fatal("parent updated before removed media was hidden")
		}
	}
	service := NewService(repo, tradeUsers{}, tradeReviews{}, &tradeConversations{}, &tradeSearchFake{}, gateway)
	value, err := service.Update(context.Background(), 42, ProductInput{
		SellerID: 7, Title: "new", Price: 10,
		ImageAssetIDs: []string{"01J00000000000000000000001", "01J00000000000000000000002"},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if gateway.reserveOwner != 7 || gateway.reserveProduct != 42 || !reflect.DeepEqual(gateway.deleteKeep, []uint64{11, 12}) ||
		!reflect.DeepEqual(gateway.calls, []string{"reserve:42", "delete:42", "commit:42"}) || value.ReviewStatus != review.StatusSubmitted ||
		repo.products[42].Title != "new" {
		t.Fatalf("gateway=%+v value=%+v product=%+v", gateway, value, repo.products[42])
	}
	for _, kept := range gateway.deleteKeep {
		if kept == 99 {
			t.Fatal("removed asset remained in keep set")
		}
	}
}

func TestCreateKeyedFirstReplayConflictAndStableOutboxPayload(t *testing.T) {
	repo := newTradeKeyedMemoryRepo()
	gateway := &tradeMediaFake{}
	service := NewService(repo, tradeUsers{}, tradeReviews{}, &tradeConversations{}, &tradeSearchFake{}, gateway)
	key, err := newWriteContractULID(time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC), zeroReader{})
	if err != nil {
		t.Fatal(err)
	}
	input := ProductInput{
		SellerID: 7, Title: "bike", Description: "used bike", Category: "travel",
		Price: 15000, OriginalPrice: 22000, Condition: "used", Campus: "main",
		ImageAssetIDs: []string{"01J00000000000000000000001", "01J00000000000000000000002"},
	}
	first, replayed, err := service.CreateKeyed(context.Background(), input, "wechat pass", key)
	if err != nil || replayed || first.ID == 0 {
		t.Fatalf("first keyed create failed: product=%+v replayed=%v err=%v", first, replayed, err)
	}
	if gateway.committed {
		t.Fatal("keyed create bypassed creation outbox with a synchronous media commit")
	}
	if gateway.reserveCalls != 1 || gateway.cancelCalls != 0 || len(repo.outboxes) != 1 {
		t.Fatalf("unexpected first-create effects reserve=%d cancel=%d outboxes=%d", gateway.reserveCalls, gateway.cancelCalls, len(repo.outboxes))
	}
	event := repo.outboxes[0]
	if event.EventType != "trade_product_created" || event.ReservationToken == nil || *event.ReservationToken != "token" {
		t.Fatalf("unexpected creation outbox coordinates: %+v", event)
	}
	var payload productCreatedOutboxPayload
	if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
		t.Fatalf("decode creation outbox payload: %v", err)
	}
	if payload.Version != "trade-product-create:v1" || payload.ReviewObjectType != "product" || payload.ReviewSubmitterID != 7 || payload.ReviewRisk != "wechat pass" ||
		len(payload.OrderedAssetPublicIDs) != 2 || payload.OrderedAssetPublicIDs[0] != input.ImageAssetIDs[0] || payload.OrderedAssetPublicIDs[1] != input.ImageAssetIDs[1] {
		t.Fatalf("unstable creation outbox payload: %+v", payload)
	}

	second, replayed, err := service.CreateKeyed(context.Background(), input, "different transient safety text", key)
	if err != nil || !replayed || second.ID != first.ID {
		t.Fatalf("same request did not replay: product=%+v replayed=%v err=%v", second, replayed, err)
	}
	if gateway.reserveCalls != 1 || len(repo.outboxes) != 1 {
		t.Fatalf("replay repeated side effects reserve=%d outboxes=%d", gateway.reserveCalls, len(repo.outboxes))
	}

	changed := input
	changed.Price++
	if _, _, err := service.CreateKeyed(context.Background(), changed, "wechat pass", key); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed payload did not conflict: %v", err)
	}
}

func TestCanonicalProductCreateHashUsesIntegerMoneyAndOrderedAssets(t *testing.T) {
	base := ProductInput{SellerID: 1, Title: "book", Price: 1234, OriginalPrice: 2345, ImageAssetIDs: []string{"asset-a", "asset-b"}}
	first, err := canonicalProductCreateHash(base)
	if err != nil {
		t.Fatal(err)
	}
	second, err := canonicalProductCreateHash(base)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("canonical hash is unstable: equal=%v err=%v", bytes.Equal(first, second), err)
	}
	reordered := base
	reordered.ImageAssetIDs = []string{"asset-b", "asset-a"}
	reorderedHash, _ := canonicalProductCreateHash(reordered)
	if bytes.Equal(first, reorderedHash) {
		t.Fatal("ordered asset public IDs were not part of canonical hash")
	}
	changedMoney := base
	changedMoney.Price++
	changedMoneyHash, _ := canonicalProductCreateHash(changedMoney)
	if bytes.Equal(first, changedMoneyHash) {
		t.Fatal("integer price cents were not part of canonical hash")
	}
}

type tradeConversations struct {
	calls     int
	findCalls int
	values    []im.ConversationContext
}

func (c *tradeConversations) EnsureProductConversation(_ context.Context, p, b, s uint) (im.ConversationContext, error) {
	c.calls++
	value := im.ConversationContext{RelationID: uint(len(c.values) + 1), ConversationID: "C2Ccampus_1", PeerUserID: "campus_1", ProductID: p, BuyerID: b, SellerID: s}
	c.values = append(c.values, value)
	return value, nil
}
func (c *tradeConversations) FindProductConversation(_ context.Context, productID, buyerID, sellerID uint) (im.ConversationContext, bool, error) {
	c.findCalls++
	if buyerID == sellerID {
		return im.ConversationContext{}, false, errors.New("seller cannot contact self")
	}
	for _, value := range c.values {
		if value.ProductID == productID && value.BuyerID == buyerID && value.SellerID == sellerID {
			return value, true, nil
		}
	}
	return im.ConversationContext{}, false, nil
}
func (c *tradeConversations) EnsureProductConversationStable(ctx context.Context, productID, buyerID, sellerID uint) (im.ConversationContext, error) {
	if value, ok, err := c.FindProductConversation(ctx, productID, buyerID, sellerID); err != nil || ok {
		return value, err
	}
	return c.EnsureProductConversation(ctx, productID, buyerID, sellerID)
}

func newTradeService(repo *tradeMemoryRepo, search SearchGateway, conversations *tradeConversations) *Service {
	return NewService(repo, tradeUsers{1: {ID: 1, Nickname: "seller"}}, tradeReviews{}, conversations, search)
}

func TestViewerStateForSellerSkipsSelfConversationLookup(t *testing.T) {
	repo := newTradeMemoryRepo(Product{
		ID:           1,
		SellerID:     1,
		Status:       StatusSelling,
		ReviewStatus: review.StatusApproved,
	})
	conversations := &tradeConversations{}
	service := newTradeService(repo, &tradeSearchFake{}, conversations)

	state, err := service.ViewerState(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("seller viewer state failed: %v", err)
	}
	if state.Favorited || state.ContactedSeller {
		t.Fatalf("unexpected seller viewer state: %+v", state)
	}
	if conversations.findCalls != 0 {
		t.Fatalf("seller viewer state reached self conversation lookup: calls=%d", conversations.findCalls)
	}
}

func TestProductStatusTransitions(t *testing.T) {
	tests := []struct {
		from, to ProductStatus
		want     bool
	}{
		{StatusSelling, StatusReserved, true},
		{StatusSelling, StatusSold, true},
		{StatusReserved, StatusSelling, true},
		{StatusReserved, StatusSold, true},
		{StatusSold, StatusSelling, true},
		{StatusSold, StatusReserved, false},
		{StatusSelling, StatusSelling, false},
	}
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
	if err := service.ChangeStatus(context.Background(), 1, 1, StatusReserved); err == nil {
		t.Fatal("sold product was reserved directly")
	}
	if err := service.ChangeStatus(context.Background(), 1, 1, StatusSelling); err != nil {
		t.Fatalf("seller could not relist sold product: %v", err)
	}
	repo.products[1] = Product{ID: 1, SellerID: 1, Status: StatusSelling, ReviewStatus: review.StatusApproved}
	repo.forceConflict = true
	if err := service.ChangeStatus(context.Background(), 1, 1, StatusReserved); !errors.Is(err, ErrConflict) {
		t.Fatalf("CAS conflict not surfaced: %v", err)
	}
}

func TestSoldProductGuards(t *testing.T) {
	repo := newTradeMemoryRepo(Product{
		ID: 1, SellerID: 1, Status: StatusSold, ReviewStatus: review.StatusApproved,
	})
	service := newTradeService(repo, &tradeSearchFake{}, &tradeConversations{})

	if err := service.ChangeStatus(context.Background(), 1, 2, StatusSelling); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-seller relisted sold product: %v", err)
	}
	if err := service.ChangeStatus(context.Background(), 1, 1, StatusReserved); err == nil {
		t.Fatal("sold product changed directly to reserved")
	}

	repo.products[1] = Product{
		ID: 1, SellerID: 1, Status: StatusSold, ReviewStatus: review.StatusRejected,
	}
	if err := service.ChangeStatus(context.Background(), 1, 1, StatusSelling); !errors.Is(err, ErrConflict) {
		t.Fatalf("unapproved sold product relisted: %v", err)
	}

	repo.products[1] = Product{
		ID: 1, SellerID: 1, Status: StatusSold, ReviewStatus: review.StatusApproved,
	}
	if err := service.ChangeStatus(context.Background(), 1, 1, StatusSelling); err != nil {
		t.Fatalf("approved seller relist failed: %v", err)
	}
	if got := repo.products[1].Status; got != StatusSelling {
		t.Fatalf("relisted status=%q want=%q", got, StatusSelling)
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

func TestProductTargetFavoriteViewerStateAndStableContact(t *testing.T) {
	repo := newTradeMemoryRepo(Product{ID: 1, SellerID: 1, Status: StatusSelling, ReviewStatus: review.StatusApproved})
	conversations := &tradeConversations{}
	service := newTradeService(repo, &tradeSearchFake{}, conversations)

	for i := 0; i < 2; i++ {
		favorited, err := service.SetFavorite(context.Background(), 1, 2, true)
		if err != nil || !favorited {
			t.Fatalf("set favorite failed: value=%v err=%v", favorited, err)
		}
	}
	if repo.products[1].FavoriteCount != 1 || len(repo.events) != 1 {
		t.Fatalf("favorite no-op drifted: count=%d events=%d", repo.products[1].FavoriteCount, len(repo.events))
	}
	state, err := service.ViewerState(context.Background(), 1, 2)
	if err != nil || !state.Favorited || state.ContactedSeller {
		t.Fatalf("unexpected initial viewer state: %+v err=%v", state, err)
	}
	first, err := service.EnsureSellerContact(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.EnsureSellerContact(context.Background(), 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if conversations.calls != 1 || first != second ||
		first.RelationID == 0 || first.ProductID != 1 ||
		first.BuyerID != 2 || first.SellerID != 1 ||
		first.ConversationID != "C2C"+first.PeerUserID || first.PeerUserID == "" {
		t.Fatalf("contact was not stable: calls=%d first=%+v second=%+v", conversations.calls, first, second)
	}
	state, err = service.ViewerState(context.Background(), 1, 2)
	if err != nil || !state.ContactedSeller {
		t.Fatalf("contact viewer state missing: %+v err=%v", state, err)
	}
	if _, err := service.EnsureSellerContact(context.Background(), 1, 1); !errors.Is(err, ErrCannotContactSelf) {
		t.Fatalf("seller self-contact accepted: %v", err)
	}
}

func TestSoldProductExistingFavoritePUTIsNoOp(t *testing.T) {
	repo := newTradeMemoryRepo(Product{ID: 1, SellerID: 1, Status: StatusSold, ReviewStatus: review.StatusApproved, FavoriteCount: 1})
	repo.favorites[[2]uint{1, 2}] = true
	service := newTradeService(repo, &tradeSearchFake{}, &tradeConversations{})

	favorited, err := service.SetFavorite(context.Background(), 1, 2, true)
	if err != nil || !favorited {
		t.Fatalf("existing favorite replay on sold product failed: favorited=%v err=%v", favorited, err)
	}
	if repo.products[1].FavoriteCount != 1 || len(repo.events) != 0 {
		t.Fatalf("sold replay drifted count/outbox: count=%d events=%d", repo.products[1].FavoriteCount, len(repo.events))
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
