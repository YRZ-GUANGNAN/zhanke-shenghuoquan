package delivery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const PrivateConversationImageURLTTL = 60 * time.Second

type ConversationGroupPort interface {
	EnsureWorkGroup(context.Context, string, string, uint64, uint64) error
	RemoveWorkGroupMember(context.Context, string, uint64) error
	SetWorkGroupReadOnly(context.Context, string, bool) error
	DismissWorkGroup(context.Context, string) error
}

type ConversationMediaPort interface {
	ValidateOwnedApprovedConversationAsset(context.Context, uint64, string) error
	PublishConversationImage(context.Context, string, string, uint16) error
	ResolveConversationImage(context.Context, string, uint16, time.Duration) (string, error)
}

type ConversationServiceConfig struct {
	LeaseTTL      time.Duration
	RetryDelay    time.Duration
	MaxAttempts   uint32
	ImagesEnabled bool
	Now           func() time.Time
}

type ConversationService struct {
	repository ConversationRepository
	orders     *Service
	groups     ConversationGroupPort
	media      ConversationMediaPort
	config     ConversationServiceConfig
}

func NewConversationService(
	repository ConversationRepository,
	orders *Service,
	groups ConversationGroupPort,
	media ConversationMediaPort,
	config ConversationServiceConfig,
) (*ConversationService, error) {
	if repository == nil || orders == nil || groups == nil || media == nil {
		return nil, ErrInvalidRecord
	}
	if config.LeaseTTL <= 0 {
		config.LeaseTTL = 30 * time.Second
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 5 * time.Second
	}
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 5
	}
	if config.Now == nil {
		config.Now = func() time.Time { return time.Now().UTC() }
	}
	return &ConversationService{
		repository: repository, orders: orders, groups: groups, media: media, config: config,
	}, nil
}

func (s *ConversationService) EnsureForClaim(
	ctx context.Context,
	order Order,
) (Conversation, error) {
	if order.RiderUserID == nil || order.Status != StatusClaimed {
		return Conversation{}, ErrConversationForbidden
	}
	conversation, _, _, err := s.repository.EnsureConversationForCurrentAssignment(
		ctx, order.DeliveryOrderPublicID, *order.RiderUserID, s.config.Now().UTC(),
	)
	if err != nil {
		return Conversation{}, err
	}
	if conversation.Status == ConversationActive {
		return conversation, nil
	}
	if err := s.ProcessConversation(ctx, conversation.ConversationPublicID); err != nil {
		return conversation, err
	}
	return s.repositoryConversation(ctx, conversation.ConversationPublicID, order.MerchantUserID)
}

func (s *ConversationService) Current(
	ctx context.Context,
	userID uint64,
	orderID string,
) (Conversation, error) {
	orderID = strings.TrimSpace(orderID)
	if userID == 0 || !ValidULID(orderID) {
		return Conversation{}, ErrInvalidRecord
	}
	order, err := s.orders.FindOrder(ctx, orderID)
	if err != nil {
		return Conversation{}, ErrConversationNotFound
	}
	if userID != order.MerchantUserID && (order.RiderUserID == nil || *order.RiderUserID != userID) {
		return Conversation{}, ErrConversationForbidden
	}
	if order.RiderUserID == nil || order.Status == StatusReadyForClaim ||
		order.Status == StatusPendingFeePayment || order.Status == StatusPaymentFailed {
		return Conversation{}, ErrConversationNotFound
	}
	conversation, _, _, err := s.repository.EnsureConversationForCurrentAssignment(
		ctx, orderID, *order.RiderUserID, s.config.Now().UTC(),
	)
	if err != nil {
		return Conversation{}, err
	}
	if conversation.Status == ConversationProvisioning || conversation.Status == ConversationFailed {
		if err := s.ProcessConversation(ctx, conversation.ConversationPublicID); err != nil {
			return Conversation{}, ErrConversationUnavailable
		}
		conversation, err = s.repositoryConversation(ctx, conversation.ConversationPublicID, userID)
		if err != nil {
			return Conversation{}, err
		}
	}
	return s.syncReadOnly(ctx, conversation, order)
}

func (s *ConversationService) FindForParticipant(
	ctx context.Context,
	userID uint64,
	conversationID string,
) (Conversation, Order, error) {
	conversationID = strings.TrimSpace(conversationID)
	if userID == 0 || !ValidULID(conversationID) {
		return Conversation{}, Order{}, ErrInvalidRecord
	}
	conversation, order, err := s.repository.FindConversationForParticipant(ctx, conversationID, userID)
	if err != nil {
		return Conversation{}, Order{}, err
	}
	conversation, err = s.syncReadOnly(ctx, conversation, order)
	return conversation, order, err
}

func (s *ConversationService) ListForParticipant(
	ctx context.Context,
	userID uint64,
	page Page,
) ([]Conversation, int64, error) {
	if userID == 0 || !page.Valid() {
		return nil, 0, ErrInvalidRecord
	}
	return s.repository.ListConversationsForParticipant(ctx, userID, page)
}

func (s *ConversationService) PrepareImage(
	ctx context.Context,
	userID uint64,
	conversationID, assetPublicID string,
) (uint16, Conversation, error) {
	if !s.config.ImagesEnabled {
		return 0, Conversation{}, ErrConversationImageDenied
	}
	conversation, order, err := s.FindForParticipant(ctx, userID, conversationID)
	if err != nil {
		return 0, Conversation{}, err
	}
	if !orderAllowsConversationSend(order.Status) || conversation.Status != ConversationActive {
		return 0, Conversation{}, ErrConversationReadOnly
	}
	assetPublicID = strings.TrimSpace(assetPublicID)
	if !ValidULID(assetPublicID) {
		return 0, Conversation{}, ErrInvalidRecord
	}
	if err := s.media.ValidateOwnedApprovedConversationAsset(ctx, userID, assetPublicID); err != nil {
		return 0, Conversation{}, ErrConversationImageDenied
	}
	slot, updated, err := s.repository.AllocateConversationImageSlot(
		ctx, conversation.ID, conversation.RowVersion, s.config.Now().UTC(),
	)
	if err != nil {
		return 0, Conversation{}, err
	}
	if err := s.media.PublishConversationImage(
		ctx, assetPublicID, conversation.ConversationPublicID, slot,
	); err != nil {
		return 0, updated, fmt.Errorf("%w: %v", ErrConversationImagePending, err)
	}
	return slot, updated, nil
}

func (s *ConversationService) ResolveImage(
	ctx context.Context,
	userID uint64,
	conversationID string,
	slot uint16,
) (string, error) {
	if _, _, err := s.FindForParticipant(ctx, userID, conversationID); err != nil {
		return "", err
	}
	return s.media.ResolveConversationImage(
		ctx, strings.TrimSpace(conversationID), slot, PrivateConversationImageURLTTL,
	)
}

func (s *ConversationService) RevokeBeforeRelease(
	ctx context.Context,
	riderUserID uint64,
	orderID string,
) error {
	conversation, _, err := s.repository.FindCurrentConversationForRider(
		ctx, strings.TrimSpace(orderID), riderUserID,
	)
	if errors.Is(err, ErrConversationNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if conversation.Status == ConversationRevoked {
		return nil
	}
	if conversation.Status != ConversationRevoking {
		conversation, err = s.repository.BeginConversationRevocation(
			ctx, conversation.ID, conversation.Status, conversation.RowVersion, s.config.Now().UTC(),
		)
		if err != nil {
			return err
		}
	}
	if err := s.groups.RemoveWorkGroupMember(ctx, conversation.TencentGroupID, riderUserID); err != nil {
		return ErrConversationUnavailable
	}
	return s.repository.CompleteConversationRevocation(
		ctx, conversation.ID, conversation.RowVersion, s.config.Now().UTC(),
	)
}

func (s *ConversationService) ProcessConversation(ctx context.Context, conversationID string) error {
	now := s.config.Now().UTC()
	lease, found, err := s.repository.ClaimConversation(ctx, conversationID, now, s.config.LeaseTTL)
	if err != nil {
		return err
	}
	if !found {
		conversation, lookupErr := s.repository.FindConversation(ctx, conversationID)
		if lookupErr == nil && conversation.Status == ConversationActive {
			return nil
		}
		return ErrConversationUnavailable
	}
	name := "D-" + lease.Conversation.DeliveryOrderPublicID
	err = s.groups.EnsureWorkGroup(
		ctx, lease.Conversation.TencentGroupID, name,
		lease.Conversation.MerchantUserID, lease.Conversation.RiderUserID,
	)
	if err == nil {
		return s.repository.CompleteConversationProvision(ctx, lease, now)
	}
	retryAt := now.Add(s.config.RetryDelay * time.Duration(1<<minConversationAttempt(lease.Conversation.AttemptCount-1, 8)))
	if failErr := s.repository.FailConversationProvision(
		ctx, lease, err.Error(), retryAt, s.config.MaxAttempts, now,
	); failErr != nil {
		return failErr
	}
	return ErrConversationUnavailable
}

func (s *ConversationService) ConsumeBatch(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	processed := 0
	for processed < limit {
		now := s.config.Now().UTC()
		lease, found, err := s.repository.ClaimNextConversation(ctx, now, s.config.LeaseTTL)
		if err != nil {
			return processed, err
		}
		if !found {
			break
		}
		processed++
		name := "D-" + lease.Conversation.DeliveryOrderPublicID
		groupErr := s.groups.EnsureWorkGroup(
			ctx, lease.Conversation.TencentGroupID, name,
			lease.Conversation.MerchantUserID, lease.Conversation.RiderUserID,
		)
		if groupErr == nil {
			if err := s.repository.CompleteConversationProvision(ctx, lease, now); err != nil {
				return processed, err
			}
			continue
		}
		retryAt := now.Add(s.config.RetryDelay * time.Duration(1<<minConversationAttempt(lease.Conversation.AttemptCount-1, 8)))
		if err := s.repository.FailConversationProvision(
			ctx, lease, groupErr.Error(), retryAt, s.config.MaxAttempts, now,
		); err != nil {
			return processed, err
		}
	}
	return processed, nil
}

func (s *ConversationService) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := s.ConsumeBatch(ctx, 20); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *ConversationService) syncReadOnly(
	ctx context.Context,
	conversation Conversation,
	order Order,
) (Conversation, error) {
	if orderConversationReadOnly(order.Status) && conversation.Status == ConversationActive {
		// Messages travel directly through Tencent IM. Muting the remote Work
		// group is therefore the authoritative send barrier; the local status
		// is advanced only after that external barrier is in place.
		if err := s.groups.SetWorkGroupReadOnly(
			ctx, conversation.TencentGroupID, true,
		); err != nil {
			return Conversation{}, ErrConversationUnavailable
		}
		return s.repository.MarkConversationReadOnly(
			ctx, conversation.ID, conversation.RowVersion, s.config.Now().UTC(),
		)
	}
	return conversation, nil
}

func (s *ConversationService) repositoryConversation(
	ctx context.Context,
	conversationID string,
	participant uint64,
) (Conversation, error) {
	conversation, _, err := s.repository.FindConversationForParticipant(ctx, conversationID, participant)
	return conversation, err
}

func minConversationAttempt(left, right uint32) uint32 {
	if left < right {
		return left
	}
	return right
}
