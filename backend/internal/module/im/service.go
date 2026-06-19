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
	mapping := UserMapping{UserID: userID, IMUserID: "campus_" + strconv.FormatUint(uint64(userID), 10)}
	if err := s.importer.ImportAccount(ctx, mapping.IMUserID); err != nil {
		return UserMapping{}, err
	}
	if err := s.repo.EnsureMapping(ctx, &mapping); err != nil {
		return UserMapping{}, err
	}
	return mapping, nil
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

func (s *Service) Conversations(ctx context.Context, userID uint, page, size int) (ConversationPage, error) {
	page, size = normalize(page, size)
	values, total, err := s.repo.ListConversations(ctx, userID, page, size)
	if err != nil {
		return ConversationPage{}, err
	}
	result := make([]ConversationContext, 0, len(values))
	for _, value := range values {
		peerID := value.SellerID
		if userID == value.SellerID {
			peerID = value.BuyerID
		}
		mapping, err := s.repo.FindMapping(ctx, peerID)
		if err != nil {
			return ConversationPage{}, fmt.Errorf("find IM peer mapping: %w", err)
		}
		value.ConversationID = "C2C" + mapping.IMUserID
		result = append(result, toContext(value, mapping.IMUserID))
	}
	return ConversationPage{Items: result, Page: page, PageSize: size, Total: total}, nil
}

func toContext(value Conversation, peerUserID string) ConversationContext {
	return ConversationContext{
		RelationID: value.ID, ConversationID: "C2C" + peerUserID, PeerUserID: peerUserID,
		ProductID: value.ReferenceID, BuyerID: value.BuyerID, SellerID: value.SellerID,
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
