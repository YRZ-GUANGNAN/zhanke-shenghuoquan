package im

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

type Credentials struct {
	SDKAppID string `json:"sdkAppId"`
	UserID   string `json:"userId"`
	UserSig  string `json:"userSig"`
}

type ConversationContext struct {
	ReferenceType  string `json:"referenceType"`
	RelationID     uint   `json:"relationId"`
	ConversationID string `json:"conversationId"`
	PeerUserID     string `json:"peerUserId"`
	ProductID      uint   `json:"productId"`
	BuyerID        uint   `json:"buyerId"`
	SellerID       uint   `json:"sellerId"`
}

type ConversationPage struct {
	Items    []ConversationContext `json:"items"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"pageSize"`
	Total    int64                 `json:"total"`
}

var ErrConversationNotFound = errors.New("conversation not found or access denied")

type ConversationGateway interface {
	EnsureProductConversation(context.Context, uint, uint, uint) (ConversationContext, error)
}

type Service struct {
	repo             Repository
	importer         AccountImporter
	sdkAppID, secret string
}

func NewService(repo Repository, importer AccountImporter, sdkAppID, secret string) *Service {
	return &Service{repo: repo, importer: importer, sdkAppID: sdkAppID, secret: secret}
}

func (s *Service) Credentials(ctx context.Context, userID uint) (Credentials, error) {
	if s.sdkAppID == "" || s.secret == "" || s.importer == nil {
		return Credentials{}, errors.New("Tencent IM is not configured")
	}
	mapping, err := s.ensureIMUser(ctx, userID)
	if err != nil {
		return Credentials{}, err
	}
	sig, err := generateUserSig(s.sdkAppID, s.secret, mapping.IMUserID, 86400)
	if err != nil {
		return Credentials{}, err
	}
	return Credentials{SDKAppID: s.sdkAppID, UserID: mapping.IMUserID, UserSig: sig}, nil
}

func (s *Service) ensureIMUser(ctx context.Context, userID uint) (UserMapping, error) {
	if userID == 0 {
		return UserMapping{}, errors.New("invalid IM user")
	}
	if s.importer == nil {
		return UserMapping{}, errors.New("Tencent IM is not configured")
	}
	mapping := UserMapping{UserID: userID, IMUserID: "campus_" + strconv.FormatUint(uint64(userID), 10)}
	if err := s.importer.ImportAccount(ctx, mapping.IMUserID); err != nil {
		return UserMapping{}, err
	}
	if err := s.repo.EnsureMapping(ctx, &mapping); err != nil {
		return UserMapping{}, err
	}
	return mapping, nil
}

// EnsureUser exposes the existing account-import boundary to application-layer
// adapters without coupling the IM domain to any business object.
func (s *Service) EnsureUser(ctx context.Context, userID uint) (string, error) {
	mapping, err := s.ensureIMUser(ctx, userID)
	if err != nil {
		return "", err
	}
	return mapping.IMUserID, nil
}

func (s *Service) EnsureProductConversation(ctx context.Context, productID, buyerID, sellerID uint) (ConversationContext, error) {
	if productID == 0 || buyerID == 0 || sellerID == 0 {
		return ConversationContext{}, errors.New("invalid conversation relation")
	}
	if buyerID == sellerID {
		return ConversationContext{}, errors.New("seller cannot contact self")
	}
	buyer, err := s.ensureIMUser(ctx, buyerID)
	if err != nil {
		return ConversationContext{}, err
	}
	seller, err := s.ensureIMUser(ctx, sellerID)
	if err != nil {
		return ConversationContext{}, err
	}
	_ = buyer
	value := Conversation{
		ConversationID: "C2C" + seller.IMUserID, BuyerID: buyerID, SellerID: sellerID,
		ReferenceType: "product", ReferenceID: productID,
	}
	if err := s.repo.EnsureConversation(ctx, &value); err != nil {
		return ConversationContext{}, err
	}
	return toContext(value, seller.IMUserID), nil
}

// FindProductConversation performs a read-only lookup by the immutable product
// relation. It never imports Tencent accounts or refreshes relation timestamps.
func (s *Service) FindProductConversation(ctx context.Context, productID, buyerID, sellerID uint) (ConversationContext, bool, error) {
	if productID == 0 || buyerID == 0 || sellerID == 0 {
		return ConversationContext{}, false, errors.New("invalid conversation relation")
	}
	if buyerID == sellerID {
		return ConversationContext{}, false, errors.New("seller cannot contact self")
	}
	value, found, err := s.repo.FindConversation(ctx, buyerID, sellerID, "product", productID)
	if err != nil {
		return ConversationContext{}, false, err
	}
	if !found {
		return ConversationContext{}, false, nil
	}
	seller, err := s.repo.FindMapping(ctx, sellerID)
	if err != nil {
		return ConversationContext{}, false, fmt.Errorf("find IM peer mapping: %w", err)
	}
	return toContext(value, seller.IMUserID), true, nil
}

// EnsureProductConversationStable creates the immutable product relation once.
// Replays return the canonical row without refreshing timestamps or importing
// Tencent accounts again.
func (s *Service) EnsureProductConversationStable(ctx context.Context, productID, buyerID, sellerID uint) (ConversationContext, error) {
	value, found, err := s.FindProductConversation(ctx, productID, buyerID, sellerID)
	if err != nil {
		return ConversationContext{}, err
	}
	if found {
		return value, nil
	}
	if _, err := s.ensureIMUser(ctx, buyerID); err != nil {
		return ConversationContext{}, err
	}
	seller, err := s.ensureIMUser(ctx, sellerID)
	if err != nil {
		return ConversationContext{}, err
	}
	relation := Conversation{
		ConversationID: "C2C" + seller.IMUserID, BuyerID: buyerID, SellerID: sellerID,
		ReferenceType: "product", ReferenceID: productID,
	}
	if err := s.repo.EnsureConversationStable(ctx, &relation); err != nil {
		return ConversationContext{}, err
	}
	return toContext(relation, seller.IMUserID), nil
}

func (s *Service) Conversation(ctx context.Context, relationID, userID uint) (ConversationContext, error) {
	value, found, err := s.repo.FindConversationForParticipant(ctx, relationID, userID)
	if err != nil {
		return ConversationContext{}, err
	}
	if !found {
		return ConversationContext{}, ErrConversationNotFound
	}
	return s.contextForParticipant(ctx, value, userID)
}

func (s *Service) contextForParticipant(ctx context.Context, value Conversation, userID uint) (ConversationContext, error) {
	var peerID uint
	switch userID {
	case value.BuyerID:
		peerID = value.SellerID
	case value.SellerID:
		peerID = value.BuyerID
	default:
		return ConversationContext{}, ErrConversationNotFound
	}
	mapping, err := s.repo.FindMapping(ctx, peerID)
	if err != nil {
		return ConversationContext{}, fmt.Errorf("find IM peer mapping: %w", err)
	}
	return toContext(value, mapping.IMUserID), nil
}

func (s *Service) Conversations(ctx context.Context, userID uint, page, size int, includeProducts ...bool) (ConversationPage, error) {
	page, size = normalize(page, size)
	values, total, err := s.repo.ListConversations(ctx, userID, page, size, includeProducts...)
	if err != nil {
		return ConversationPage{}, err
	}
	result := make([]ConversationContext, 0, len(values))
	for _, value := range values {
		item, err := s.contextForParticipant(ctx, value, userID)
		if err != nil {
			return ConversationPage{}, err
		}
		result = append(result, item)
	}
	return ConversationPage{Items: result, Page: page, PageSize: size, Total: total}, nil
}

func toContext(value Conversation, peerUserID string) ConversationContext {
	productID := uint(0)
	if value.ReferenceType == "product" {
		productID = value.ReferenceID
	}
	return ConversationContext{
		ReferenceType: value.ReferenceType,
		RelationID:    value.ID, ConversationID: "C2C" + peerUserID, PeerUserID: peerUserID,
		ProductID: productID, BuyerID: value.BuyerID, SellerID: value.SellerID,
	}
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
