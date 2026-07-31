package trade

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"campus-circle/backend/internal/module/admin/review"
	"campus-circle/backend/internal/module/user"
)

var (
	ErrInvalidProductComment     = errors.New("invalid product comment")
	ErrProductCommentDuplicate   = errors.New("duplicate product comment")
	ErrProductCommentRateLimited = errors.New("product comment rate limited")
)

type ProductCommentProductRepository interface {
	Find(context.Context, uint, bool) (Product, error)
}

type ProductCommentHistoryRepository interface {
	FindProductForCommentHistory(context.Context, uint) (Product, error)
}

type ProductCommentService struct {
	repo     ProductCommentRepository
	products ProductCommentProductRepository
	users    UserDirectory
	now      func() time.Time
}

func NewProductCommentService(repo ProductCommentRepository, products ProductCommentProductRepository, users UserDirectory) *ProductCommentService {
	return &ProductCommentService{repo: repo, products: products, users: users, now: time.Now}
}

type canonicalProductCommentRequest struct {
	Version         string `json:"version"`
	ProductID       uint   `json:"productId"`
	ParentCommentID uint   `json:"parentCommentId"`
	ReplyToUserID   uint   `json:"replyToUserId"`
	Content         string `json:"content"`
}

type productCommentCreatedOutboxPayload struct {
	Version           string `json:"version"`
	ReviewObjectType  string `json:"reviewObjectType"`
	ReviewSubmitterID uint   `json:"reviewSubmitterId"`
	ReviewRisk        string `json:"reviewRisk"`
}

func (s *ProductCommentService) Create(ctx context.Context, userID, productID uint, input ProductCommentInput, risk string) (ProductCommentView, bool, error) {
	return s.create(ctx, userID, productID, 0, input, risk)
}

func (s *ProductCommentService) Reply(ctx context.Context, userID, productID, targetCommentID uint, input ProductCommentInput, risk string) (ProductCommentView, bool, error) {
	return s.create(ctx, userID, productID, targetCommentID, input, risk)
}

func (s *ProductCommentService) create(ctx context.Context, userID, productID, targetCommentID uint, input ProductCommentInput, risk string) (ProductCommentView, bool, error) {
	content := strings.TrimSpace(input.Content)
	if userID == 0 || productID == 0 || !validWriteContractULID(input.ClientRequestID) ||
		content == "" || !utf8.ValidString(content) || utf8.RuneCountInString(content) > 500 {
		return ProductCommentView{}, false, ErrInvalidProductComment
	}

	parentID, replyToUserID, err := s.replyCoordinates(ctx, userID, productID, targetCommentID)
	if err != nil {
		return ProductCommentView{}, false, err
	}
	requestHash, err := productCommentRequestHash(productID, parentID, replyToUserID, content)
	if err != nil {
		return ProductCommentView{}, false, err
	}
	if existing, found, findErr := s.repo.FindProductCommentByRequest(ctx, userID, input.ClientRequestID); findErr != nil {
		return ProductCommentView{}, false, findErr
	} else if found {
		if !bytes.Equal(existing.RequestHash, requestHash) {
			return ProductCommentView{}, false, ErrIdempotencyConflict
		}
		product, productErr := s.products.Find(ctx, existing.ProductID, false)
		if productErr != nil {
			return ProductCommentView{}, false, productErr
		}
		view, viewErr := s.enrichComments(ctx, product, []ProductComment{existing}, nil)
		if viewErr != nil {
			return ProductCommentView{}, false, viewErr
		}
		return view[0], true, nil
	}

	product, err := s.products.Find(ctx, productID, false)
	if err != nil {
		return ProductCommentView{}, false, err
	}
	if product.ReviewStatus != review.StatusApproved ||
		(product.Status != StatusSelling && product.Status != StatusReserved) {
		return ProductCommentView{}, false, ErrConflict
	}
	now := s.now().UTC()
	since := now.Add(-time.Minute)
	recent, err := s.repo.CountRecentProductComments(ctx, userID, since)
	if err != nil {
		return ProductCommentView{}, false, err
	}
	if recent >= 5 {
		return ProductCommentView{}, false, ErrProductCommentRateLimited
	}
	duplicate, err := s.repo.HasRecentDuplicateProductComment(ctx, userID, productID, requestHash, since)
	if err != nil {
		return ProductCommentView{}, false, err
	}
	if duplicate {
		return ProductCommentView{}, false, ErrProductCommentDuplicate
	}
	value := ProductComment{
		ProductID: productID, UserID: userID, ParentCommentID: parentID, ReplyToUserID: replyToUserID,
		Content: content, ReviewStatus: review.StatusSubmitted, ClientRequestID: input.ClientRequestID,
		RequestHash: requestHash, CreatedAt: now, UpdatedAt: now,
	}
	payload, err := json.Marshal(productCommentCreatedOutboxPayload{
		Version: "trade-product-comment:v1", ReviewObjectType: ProductCommentAggregateType,
		ReviewSubmitterID: userID, ReviewRisk: risk,
	})
	if err != nil {
		return ProductCommentView{}, false, err
	}
	replayed, err := s.repo.CreateProductCommentOrReplay(ctx, &value, &WriteOutbox{
		EventType: ProductCommentCreatedEventType, Payload: string(payload),
	})
	if err != nil {
		return ProductCommentView{}, false, err
	}
	views, err := s.enrichComments(ctx, product, []ProductComment{value}, nil)
	if err != nil {
		return ProductCommentView{}, false, err
	}
	return views[0], replayed, nil
}

func (s *ProductCommentService) replyCoordinates(ctx context.Context, viewerID, productID, targetCommentID uint) (uint, uint, error) {
	if targetCommentID == 0 {
		return 0, 0, nil
	}
	target, err := s.repo.FindProductComment(ctx, targetCommentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, 0, ErrNotFound
		}
		return 0, 0, err
	}
	if target.ProductID != productID ||
		(target.ReviewStatus != review.StatusApproved && target.UserID != viewerID) {
		return 0, 0, ErrNotFound
	}
	parentID := target.ID
	if target.ParentCommentID != 0 {
		parentID = target.ParentCommentID
	}
	parent, err := s.repo.FindProductComment(ctx, parentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return 0, 0, ErrNotFound
		}
		return 0, 0, err
	}
	if parent.ProductID != productID || parent.ParentCommentID != 0 ||
		(parent.ReviewStatus != review.StatusApproved && parent.UserID != viewerID) {
		return 0, 0, ErrNotFound
	}
	return parentID, target.UserID, nil
}

func productCommentRequestHash(productID, parentCommentID, replyToUserID uint, content string) ([]byte, error) {
	payload, err := json.Marshal(canonicalProductCommentRequest{
		Version: "trade-product-comment:v1", ProductID: productID,
		ParentCommentID: parentCommentID, ReplyToUserID: replyToUserID, Content: content,
	})
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(payload)
	return sum[:], nil
}

func (s *ProductCommentService) List(ctx context.Context, viewerID, productID uint, page, pageSize int) (ProductCommentPage, error) {
	product, err := s.findProductForCommentHistory(ctx, productID)
	if err != nil {
		return ProductCommentPage{}, err
	}
	normalizedPage, normalizedSize := normalize(page, pageSize)
	threads, total, err := s.repo.ListProductCommentThreads(ctx, productID, viewerID, normalizedPage, normalizedSize)
	if err != nil {
		return ProductCommentPage{}, err
	}
	parentIDs := make([]uint, 0, len(threads))
	for _, thread := range threads {
		parentIDs = append(parentIDs, thread.ID)
	}
	replies, err := s.repo.ListProductCommentReplies(ctx, parentIDs, viewerID)
	if err != nil {
		return ProductCommentPage{}, err
	}
	views, err := s.enrichComments(ctx, product, threads, replies)
	return ProductCommentPage{Items: views, Page: normalizedPage, PageSize: normalizedSize, Total: total}, err
}

func (s *ProductCommentService) findProductForCommentHistory(ctx context.Context, productID uint) (Product, error) {
	if repo, ok := s.products.(ProductCommentHistoryRepository); ok {
		return repo.FindProductForCommentHistory(ctx, productID)
	}
	return s.products.Find(ctx, productID, false)
}

func (s *ProductCommentService) enrichComments(ctx context.Context, product Product, roots, replies []ProductComment) ([]ProductCommentView, error) {
	ids := []uint{product.SellerID}
	for _, value := range append(append([]ProductComment{}, roots...), replies...) {
		ids = append(ids, value.UserID)
		if value.ReplyToUserID != 0 {
			ids = append(ids, value.ReplyToUserID)
		}
	}
	profiles, err := s.users.Profiles(ctx, ids)
	if err != nil {
		return nil, err
	}
	replyViews := make(map[uint][]ProductCommentView)
	for _, value := range replies {
		view := productCommentToView(value, product.SellerID, profiles)
		replyViews[value.ParentCommentID] = append(replyViews[value.ParentCommentID], view)
	}
	views := make([]ProductCommentView, 0, len(roots))
	for _, value := range roots {
		view := productCommentToView(value, product.SellerID, profiles)
		view.Replies = replyViews[value.ID]
		if view.Replies == nil {
			view.Replies = []ProductCommentView{}
		}
		views = append(views, view)
	}
	return views, nil
}

func productCommentToView(value ProductComment, sellerID uint, profiles map[uint]user.Profile) ProductCommentView {
	author := profiles[value.UserID]
	if author.ID == 0 {
		author.ID = value.UserID
	}
	view := ProductCommentView{
		ID: value.ID, ProductID: value.ProductID, ParentCommentID: value.ParentCommentID,
		ReplyToUserID: value.ReplyToUserID, Content: value.Content, ReviewStatus: value.ReviewStatus,
		Author: author, IsSeller: value.UserID == sellerID, Replies: []ProductCommentView{}, CreatedAt: value.CreatedAt,
	}
	if value.ReplyToUserID != 0 {
		replyTo := profiles[value.ReplyToUserID]
		if replyTo.ID == 0 {
			replyTo.ID = value.ReplyToUserID
		}
		view.ReplyTo = &replyTo
	}
	return view
}
