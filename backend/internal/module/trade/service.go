package trade

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/im"
	"campus-circle/backend/internal/module/user"
	"campus-circle/backend/internal/pkg/media"
)

type UserDirectory interface {
	Profiles(context.Context, []uint) (map[uint]user.Profile, error)
}

type Service struct {
	repo          Repository
	users         UserDirectory
	reviews       review.Submitter
	conversations im.ConversationGateway
	search        SearchGateway
	media         media.Gateway
}

var (
	ErrCannotContactSelf        = errors.New("seller cannot contact self")
	ErrContactStateUnavailable  = errors.New("conversation state lookup is unavailable")
	ErrWriteContractUnavailable = errors.New("trade write contract is unavailable")
)

type stableProductConversationGateway interface {
	FindProductConversation(context.Context, uint, uint, uint) (im.ConversationContext, bool, error)
	EnsureProductConversationStable(context.Context, uint, uint, uint) (im.ConversationContext, error)
}

func NewService(repo Repository, users UserDirectory, reviews review.Submitter, conversations im.ConversationGateway, search SearchGateway, mediaGateway ...media.Gateway) *Service {
	var gateway media.Gateway
	if len(mediaGateway) > 0 {
		gateway = mediaGateway[0]
	}
	return &Service{repo: repo, users: users, reviews: reviews, conversations: conversations, search: search, media: gateway}
}

type ProductInput struct {
	SellerID                                        uint
	Title, Description, Category, Condition, Campus string
	Price, OriginalPrice                            int64
	ImageAssetIDs                                   []string
}

type productCreateContractRepository interface {
	FindProductByRequest(context.Context, uint, string) (Product, bool, error)
	CreateProductOrReplay(context.Context, *Product, *WriteOutbox) (bool, error)
}

type canonicalProductCreateRequest struct {
	Version         string   `json:"version"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Category        string   `json:"category"`
	Price           int64    `json:"price"`
	OriginalPrice   int64    `json:"originalPrice"`
	Condition       string   `json:"condition"`
	Campus          string   `json:"campus"`
	OrderedAssetIDs []string `json:"orderedAssetPublicIds"`
}

type productCreatedOutboxPayload struct {
	Version               string   `json:"version"`
	OrderedAssetPublicIDs []string `json:"orderedAssetPublicIds"`
	ReviewObjectType      string   `json:"reviewObjectType"`
	ReviewSubmitterID     uint     `json:"reviewSubmitterId"`
	ReviewRisk            string   `json:"reviewRisk"`
}

type ProductView struct {
	Product
	Seller user.Profile `json:"seller"`
}

type ProductViewerState struct {
	Favorited       bool `json:"favorited"`
	ContactedSeller bool `json:"contactedSeller"`
}

type ContactView struct {
	ConversationID string `json:"conversationId"`
	PeerUserID     string `json:"peerUserId"`
	ProductID      uint   `json:"productId"`
}

type Page struct {
	Items    []ProductView `json:"items"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
	Total    int64         `json:"total"`
}

func (s *Service) Create(ctx context.Context, input ProductInput, risk string) (Product, error) {
	if err := validateInput(input); err != nil {
		return Product{}, err
	}
	reservation := media.Reservation{}
	var err error
	if len(input.ImageAssetIDs) > 0 {
		if s.media == nil {
			return Product{}, errors.New("media pipeline is unavailable")
		}
		reservation, err = s.media.Reserve(ctx, input.SellerID, media.PurposeProductImage, input.ImageAssetIDs)
		if err != nil {
			return Product{}, err
		}
	}
	value := Product{
		SellerID: input.SellerID, Title: input.Title, Description: input.Description,
		Category: fallback(input.Category, "其他"), Price: input.Price, OriginalPrice: input.OriginalPrice,
		Condition: fallback(input.Condition, "未说明"), Campus: fallback(input.Campus, "校内"),
		Status: StatusSelling, ReviewStatus: review.StatusSubmitted,
	}
	for i, ref := range reservation.References {
		assetID := ref.InternalID
		value.Images = append(value.Images, ProductImage{AssetID: &assetID, URL: ref.URL, Sort: i})
	}
	if err := s.repo.Create(ctx, &value); err != nil {
		if s.media != nil && reservation.Token != "" {
			_ = s.media.Cancel(ctx, input.SellerID, reservation)
		}
		return Product{}, err
	}
	if len(reservation.References) > 0 {
		if err := s.media.Commit(ctx, input.SellerID, reservation, "product", uint64(value.ID)); err != nil {
			return Product{}, err
		}
	}
	if _, err := s.reviews.Submit(ctx, review.Submission{ObjectType: "product", ObjectID: value.ID, SubmitterID: input.SellerID, Risk: risk}); err != nil {
		return Product{}, err
	}
	return value, nil
}

// CreateKeyed creates a product together with its cross-module work outbox.
// The legacy no-key Create path intentionally remains unchanged.
func (s *Service) CreateKeyed(ctx context.Context, input ProductInput, risk, clientRequestID string) (Product, bool, error) {
	if err := validateInput(input); err != nil {
		return Product{}, false, err
	}
	if !validWriteContractULID(clientRequestID) {
		return Product{}, false, ErrWriteContractInvalid
	}
	contractRepo, ok := s.repo.(productCreateContractRepository)
	if !ok {
		return Product{}, false, ErrWriteContractUnavailable
	}

	requestHash, err := canonicalProductCreateHash(input)
	if err != nil {
		return Product{}, false, err
	}
	existing, found, err := contractRepo.FindProductByRequest(ctx, input.SellerID, clientRequestID)
	if err != nil {
		return Product{}, false, err
	}
	if found {
		if !bytes.Equal(existing.CreateRequestHash, requestHash) {
			return Product{}, false, ErrIdempotencyConflict
		}
		return existing, true, nil
	}

	reservation := media.Reservation{}
	if len(input.ImageAssetIDs) > 0 {
		if s.media == nil {
			return Product{}, false, errors.New("media pipeline is unavailable")
		}
		reservation, err = s.media.Reserve(ctx, input.SellerID, media.PurposeProductImage, input.ImageAssetIDs)
		if err != nil {
			if errors.Is(err, media.ErrAlreadyBound) || errors.Is(err, media.ErrInvalidAsset) {
				if replay, replayErr, found := awaitProductReplay(ctx, contractRepo, input.SellerID, clientRequestID, requestHash); found {
					return replay, true, replayErr
				}
			}
			return Product{}, false, err
		}
	}
	cancelReservation := func() {
		if s.media != nil && reservation.Token != "" {
			_ = s.media.Cancel(ctx, input.SellerID, reservation)
		}
	}

	key := clientRequestID
	value := Product{
		SellerID: input.SellerID, ClientRequestID: &key, CreateRequestHash: requestHash,
		Title: input.Title, Description: input.Description,
		Category: fallback(input.Category, "其他"), Price: input.Price, OriginalPrice: input.OriginalPrice,
		Condition: fallback(input.Condition, "未说明"), Campus: fallback(input.Campus, "校内"),
		Status: StatusSelling, ReviewStatus: review.StatusSubmitted,
	}
	for index, ref := range reservation.References {
		assetID := ref.InternalID
		value.Images = append(value.Images, ProductImage{AssetID: &assetID, URL: ref.URL, Sort: index})
	}
	payload, err := json.Marshal(productCreatedOutboxPayload{
		Version: "trade-product-create:v1", OrderedAssetPublicIDs: append([]string(nil), input.ImageAssetIDs...),
		ReviewObjectType: "product", ReviewSubmitterID: input.SellerID, ReviewRisk: risk,
	})
	if err != nil {
		cancelReservation()
		return Product{}, false, err
	}
	event := WriteOutbox{EventType: "trade_product_created", Payload: string(payload)}
	if reservation.Token != "" {
		token := reservation.Token
		event.ReservationToken = &token
	}
	replayed, err := contractRepo.CreateProductOrReplay(ctx, &value, &event)
	if err != nil {
		cancelReservation()
		return Product{}, false, err
	}
	if replayed {
		cancelReservation()
	}
	return value, replayed, nil
}

func awaitProductReplay(ctx context.Context, repo productCreateContractRepository, ownerID uint, clientRequestID string, requestHash []byte) (Product, error, bool) {
	for attempt := 0; attempt < 20; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return Product{}, ctx.Err(), true
			case <-time.After(10 * time.Millisecond):
			}
		}
		existing, found, err := repo.FindProductByRequest(ctx, ownerID, clientRequestID)
		if err != nil {
			return Product{}, err, true
		}
		if !found {
			continue
		}
		if !bytes.Equal(existing.CreateRequestHash, requestHash) {
			return Product{}, ErrIdempotencyConflict, true
		}
		return existing, nil, true
	}
	return Product{}, nil, false
}

func canonicalProductCreateHash(input ProductInput) ([]byte, error) {
	encoded, err := json.Marshal(canonicalProductCreateRequest{
		Version: "trade-product-create:v1", Title: input.Title, Description: input.Description,
		Category: fallback(input.Category, "其他"), Price: input.Price, OriginalPrice: input.OriginalPrice,
		Condition: fallback(input.Condition, "未说明"), Campus: fallback(input.Campus, "校内"),
		OrderedAssetIDs: append([]string(nil), input.ImageAssetIDs...),
	})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(encoded)
	return digest[:], nil
}

func (s *Service) Update(ctx context.Context, id uint, input ProductInput, risk string) (Product, error) {
	if err := validateInput(input); err != nil {
		return Product{}, err
	}
	existing, err := s.repo.Find(ctx, id, false)
	if err != nil {
		return Product{}, err
	}
	if existing.SellerID != input.SellerID {
		return Product{}, ErrForbidden
	}
	if existing.Status == StatusSold {
		return Product{}, ErrProductSold
	}
	reservation := media.Reservation{}
	if len(input.ImageAssetIDs) > 0 {
		if s.media == nil {
			return Product{}, errors.New("media pipeline is unavailable")
		}
		reservation, err = s.media.Reserve(ctx, input.SellerID, media.PurposeProductImage, input.ImageAssetIDs)
		if err != nil {
			return Product{}, err
		}
	}
	existing.Title, existing.Description = input.Title, input.Description
	existing.Category, existing.Price, existing.OriginalPrice = fallback(input.Category, "其他"), input.Price, input.OriginalPrice
	existing.Condition, existing.Campus = fallback(input.Condition, "未说明"), fallback(input.Campus, "校内")
	images := make([]ProductImage, 0, len(reservation.References))
	keep := make([]uint64, 0, len(reservation.References))
	for i, ref := range reservation.References {
		assetID := ref.InternalID
		keep = append(keep, assetID)
		images = append(images, ProductImage{AssetID: &assetID, URL: ref.URL, Sort: i})
	}
	if err := s.repo.Update(ctx, &existing, images); err != nil {
		if s.media != nil && reservation.Token != "" {
			_ = s.media.Cancel(ctx, input.SellerID, reservation)
		}
		return Product{}, err
	}
	if len(reservation.References) > 0 {
		if err := s.media.Commit(ctx, input.SellerID, reservation, "product", uint64(id)); err != nil {
			return Product{}, err
		}
	}
	if s.media != nil {
		if err := s.media.DeleteReference(ctx, "product", uint64(id), keep); err != nil {
			return Product{}, err
		}
	}
	existing.Images, existing.ReviewStatus = images, review.StatusSubmitted
	if _, err := s.reviews.Submit(ctx, review.Submission{ObjectType: "product", ObjectID: id, SubmitterID: input.SellerID, Risk: risk}); err != nil {
		return Product{}, err
	}
	return existing, nil
}

func validateInput(input ProductInput) error {
	if input.Title == "" || input.Price < 0 || input.OriginalPrice < 0 {
		return errors.New("title and valid price are required")
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id, sellerID uint) error {
	value, err := s.repo.Find(ctx, id, false)
	if err != nil {
		return err
	}
	if value.SellerID != sellerID {
		return ErrForbidden
	}
	if value.Status == StatusSold {
		return ErrProductSold
	}
	if err := s.repo.Delete(ctx, id, sellerID); err != nil {
		return err
	}
	if s.media != nil {
		return s.media.DeleteReference(ctx, "product", uint64(id), nil)
	}
	return nil
}

func (s *Service) List(ctx context.Context, q ListQuery) (Page, error) {
	q.Page, q.PageSize = normalize(q.Page, q.PageSize)
	if q.Status == "" {
		q.Status = StatusSelling
	}
	if !validStatus(q.Status) {
		return Page{}, errors.New("invalid product status")
	}
	var values []Product
	var total int64
	var err error
	if q.Keyword != "" {
		if s.search == nil {
			return Page{}, ErrSearchUnavailable
		}
		var ids []uint
		ids, total, err = s.search.Search(ctx, q.Keyword, q)
		if err == nil {
			values, err = s.repo.ListByIDs(ctx, ids, q)
		}
	} else {
		values, total, err = s.repo.List(ctx, q, true)
	}
	if err != nil {
		return Page{}, err
	}
	views, err := s.enrich(ctx, values)
	return Page{Items: views, Page: q.Page, PageSize: q.PageSize, Total: total}, err
}

func (s *Service) MyProducts(ctx context.Context, sellerID uint, page, size int) (Page, error) {
	page, size = normalize(page, size)
	values, total, err := s.repo.ListSeller(ctx, sellerID, page, size)
	if err != nil {
		return Page{}, err
	}
	views, err := s.enrich(ctx, values)
	return Page{Items: views, Page: page, PageSize: size, Total: total}, err
}

func (s *Service) Product(ctx context.Context, id uint) (ProductView, error) {
	value, err := s.repo.Find(ctx, id, true)
	if err != nil {
		return ProductView{}, err
	}
	views, err := s.enrich(ctx, []Product{value})
	if err != nil {
		return ProductView{}, err
	}
	return views[0], nil
}

func (s *Service) ChangeStatus(ctx context.Context, id, sellerID uint, next ProductStatus) error {
	value, err := s.repo.Find(ctx, id, false)
	if err != nil {
		return err
	}
	if value.SellerID != sellerID {
		return ErrForbidden
	}
	if value.ReviewStatus != review.StatusApproved {
		return ErrConflict
	}
	if !validTransition(value.Status, next) {
		return errors.New("invalid product status transition")
	}
	return s.repo.SetStatus(ctx, id, sellerID, value.Status, next)
}

func validStatus(value ProductStatus) bool {
	return value == StatusSelling || value == StatusReserved || value == StatusSold
}

func validTransition(from, to ProductStatus) bool {
	switch from {
	case StatusSelling:
		return to == StatusReserved || to == StatusSold
	case StatusReserved:
		return to == StatusSelling || to == StatusSold
	case StatusSold:
		return false
	default:
		return false
	}
}

func (s *Service) ToggleFavorite(ctx context.Context, id, userID uint) (bool, error) {
	value, err := s.repo.Find(ctx, id, true)
	if err != nil {
		return false, err
	}
	return s.repo.ToggleFavorite(ctx, id, userID, value.Status != StatusSold)
}

func (s *Service) ViewerState(ctx context.Context, id, userID uint) (ProductViewerState, error) {
	value, err := s.repo.Find(ctx, id, true)
	if err != nil {
		return ProductViewerState{}, err
	}
	favorited, err := s.repo.IsFavorited(ctx, id, userID)
	if err != nil {
		return ProductViewerState{}, err
	}
	stable, ok := s.conversations.(stableProductConversationGateway)
	if !ok {
		return ProductViewerState{}, ErrContactStateUnavailable
	}
	_, contacted, err := stable.FindProductConversation(ctx, id, userID, value.SellerID)
	if err != nil {
		return ProductViewerState{}, err
	}
	return ProductViewerState{Favorited: favorited, ContactedSeller: contacted}, nil
}

func (s *Service) SetFavorite(ctx context.Context, id, userID uint, desired bool) (bool, error) {
	value, err := s.repo.Find(ctx, id, true)
	if err != nil {
		return false, err
	}
	return s.repo.SetFavorite(ctx, id, userID, desired, value.Status != StatusSold)
}

func (s *Service) ContactSeller(ctx context.Context, id, buyerID uint) (im.ConversationContext, error) {
	value, err := s.repo.Find(ctx, id, true)
	if err != nil {
		return im.ConversationContext{}, err
	}
	if value.Status == StatusSold {
		return im.ConversationContext{}, ErrProductSold
	}
	return s.conversations.EnsureProductConversation(ctx, id, buyerID, value.SellerID)
}

func (s *Service) EnsureSellerContact(ctx context.Context, id, buyerID uint) (ContactView, error) {
	value, err := s.repo.Find(ctx, id, true)
	if err != nil {
		return ContactView{}, err
	}
	if value.Status == StatusSold {
		return ContactView{}, ErrProductSold
	}
	if value.SellerID == buyerID {
		return ContactView{}, ErrCannotContactSelf
	}

	stable, ok := s.conversations.(stableProductConversationGateway)
	if !ok {
		return ContactView{}, ErrContactStateUnavailable
	}
	created, err := stable.EnsureProductConversationStable(ctx, id, buyerID, value.SellerID)
	if err != nil {
		return ContactView{}, err
	}
	return sanitizeContact(created), nil
}

func sanitizeContact(value im.ConversationContext) ContactView {
	return ContactView{
		ConversationID: value.ConversationID,
		PeerUserID:     value.PeerUserID, ProductID: value.ProductID,
	}
}

func (s *Service) AdminList(ctx context.Context, page, size int) (Page, error) {
	page, size = normalize(page, size)
	values, total, err := s.repo.List(ctx, ListQuery{Page: page, PageSize: size}, false)
	if err != nil {
		return Page{}, err
	}
	views, err := s.enrich(ctx, values)
	return Page{Items: views, Page: page, PageSize: size, Total: total}, err
}

func (s *Service) AdminTakedown(ctx context.Context, productID, operatorID uint, reason string) error {
	if productID == 0 || operatorID == 0 || strings.TrimSpace(reason) == "" {
		return errors.New("product id, operator and takedown reason are required")
	}
	return s.repo.AdminTakedown(ctx, productID)
}

func (s *Service) Stats(ctx context.Context) (int64, int64, error) { return s.repo.Count(ctx) }

func (s *Service) enrich(ctx context.Context, values []Product) ([]ProductView, error) {
	ids := make([]uint, 0, len(values))
	for _, value := range values {
		ids = append(ids, value.SellerID)
	}
	profiles, err := s.users.Profiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	views := make([]ProductView, 0, len(values))
	for _, value := range values {
		views = append(views, ProductView{Product: value, Seller: profiles[value.SellerID]})
	}
	return views, nil
}

func (s *Service) ObjectType() string { return "product" }

func (s *Service) SetReviewStatus(ctx context.Context, id uint, status review.Status) error {
	if err := s.repo.SetReviewStatus(ctx, id, status); err != nil {
		return err
	}
	if status == review.StatusRejected && s.media != nil {
		return s.media.DeleteReference(ctx, "product", uint64(id), nil)
	}
	return nil
}

func (s *Service) ProcessSearchBatch(ctx context.Context, limit int) (int, error) {
	if s.search == nil {
		return 0, ErrSearchUnavailable
	}
	events, err := s.repo.PendingSearchEvents(ctx, limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, event := range events {
		value, findErr := s.repo.FindForIndex(ctx, event.ProductID)
		operationErr := findErr
		if errors.Is(findErr, ErrNotFound) {
			operationErr = s.search.Delete(ctx, event.ProductID)
		} else if findErr == nil {
			if value.DeletedAt.Valid || value.ReviewStatus != review.StatusApproved {
				operationErr = s.search.Delete(ctx, event.ProductID)
			} else {
				operationErr = s.search.Upsert(ctx, value)
			}
		}
		if operationErr != nil {
			_ = s.repo.MarkSearchEventFailed(ctx, event.ID, event.Attempts+1, operationErr.Error())
			continue
		}
		if err := s.repo.MarkSearchEventProcessed(ctx, event.ID); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (s *Service) RunSearchWorker(ctx context.Context, interval time.Duration) error {
	if err := s.repo.QueueSearchRebuild(ctx); err != nil {
		return fmt.Errorf("queue search rebuild: %w", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := s.ProcessSearchBatch(ctx, 100); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func fallback(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func normalize(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}
