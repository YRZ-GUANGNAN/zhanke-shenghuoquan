package delivery

import (
	"context"
	"errors"
	"strings"
	"time"
)

const PrivateProofURLTTL = 60 * time.Second

type ProofServiceConfig struct {
	LeaseTTL    time.Duration
	RetryDelay  time.Duration
	MaxAttempts uint32
	Now         func() time.Time
}

type ProofService struct {
	repository ProofRepository
	media      MediaProofPort
	config     ProofServiceConfig
}

func NewProofService(repository ProofRepository, media MediaProofPort, config ProofServiceConfig) (*ProofService, error) {
	if repository == nil || media == nil {
		return nil, ErrInvalidProof
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
	return &ProofService{repository: repository, media: media, config: config}, nil
}

func (s *ProofService) AdvanceWithProof(ctx context.Context, input ProofTransitionInput) (Order, Proof, error) {
	input.OrderPublicID = strings.TrimSpace(input.OrderPublicID)
	input.AssetPublicID = strings.TrimSpace(input.AssetPublicID)
	if err := input.validate(); err != nil {
		return Order{}, Proof{}, err
	}
	if _, err := s.repository.FindAssignedRiderOrder(ctx, input.OrderPublicID, input.RiderUserID); err != nil {
		return Order{}, Proof{}, err
	}
	if err := s.media.ValidateOwnedApprovedAsset(ctx, input.RiderUserID, input.AssetPublicID); err != nil {
		return Order{}, Proof{}, ErrProofAssetRejected
	}
	proofID, err := NewULID()
	if err != nil {
		return Order{}, Proof{}, err
	}
	operationID, err := NewULID()
	if err != nil {
		return Order{}, Proof{}, err
	}
	now := s.config.Now().UTC()
	proof := Proof{
		ProofPublicID: proofID, DeliveryOrderPublicID: input.OrderPublicID,
		ProofType: input.ProofType, Slot: 0, AssetPublicID: input.AssetPublicID,
		UploaderUserID: input.RiderUserID, Status: ProofBinding,
		OperationID: operationID, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	from, to := proofTransition(input.ProofType)
	persisted, order, err := s.repository.CreateProofAndTransition(
		ctx, &proof, input.ExpectedRowVersion, from, to, now,
	)
	if err != nil {
		return Order{}, Proof{}, err
	}
	if persisted.Status == ProofActive {
		return order, persisted, nil
	}
	if persisted.Status == ProofDeadletter || persisted.Status == ProofRevoked {
		return order, persisted, ErrProofAssetRejected
	}
	if err := s.ProcessProof(ctx, persisted.ProofPublicID); err != nil {
		return order, persisted, err
	}
	persisted, err = s.repository.FindProof(ctx, input.OrderPublicID, input.ProofType, 0)
	return order, persisted, err
}

func proofTransition(proofType ProofType) (Status, Status) {
	if proofType == ProofPickup {
		return StatusToPickup, StatusPickedUp
	}
	return StatusDelivering, StatusDelivered
}

func (s *ProofService) ProcessProof(ctx context.Context, proofPublicID string) error {
	now := s.config.Now().UTC()
	lease, found, err := s.repository.ClaimProofBinding(ctx, strings.TrimSpace(proofPublicID), now, s.config.LeaseTTL)
	if err != nil {
		return err
	}
	if !found {
		return s.proofProcessState(ctx, proofPublicID)
	}
	if err := s.publishLease(ctx, lease); err != nil {
		return err
	}
	return nil
}

func (s *ProofService) ConsumeBatch(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	processed := 0
	for processed < limit {
		now := s.config.Now().UTC()
		lease, found, err := s.repository.ClaimNextProofBinding(ctx, now, s.config.LeaseTTL)
		if err != nil {
			return processed, err
		}
		if !found {
			break
		}
		processed++
		if err := s.publishLease(ctx, lease); err != nil && !errors.Is(err, ErrProofBindingPending) {
			return processed, err
		}
	}
	return processed, nil
}

func (s *ProofService) Run(ctx context.Context, interval time.Duration) error {
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

func (s *ProofService) publishLease(ctx context.Context, lease ProofBindingLease) error {
	err := s.media.PublishProofReference(
		ctx, lease.Proof.AssetPublicID, lease.Proof.ProofPublicID, lease.Proof.Slot,
	)
	now := s.config.Now().UTC()
	if err == nil {
		return s.repository.CompleteProofBinding(ctx, lease, now)
	}
	retryAt := now.Add(s.config.RetryDelay * time.Duration(1<<minProofAttempt(lease.Proof.AttemptCount-1, 8)))
	status, failErr := s.repository.FailProofBinding(ctx, lease, retryAt, s.config.MaxAttempts, now)
	if failErr != nil {
		return failErr
	}
	if status == ProofDeadletter {
		return ErrProofAssetRejected
	}
	return ErrProofBindingPending
}

func (s *ProofService) proofProcessState(ctx context.Context, proofPublicID string) error {
	if !ValidULID(strings.TrimSpace(proofPublicID)) {
		return ErrInvalidProof
	}
	// There is no public-id lookup in the private HTTP contract. A missing due
	// lease is treated as pending; the durable worker will recover it.
	return ErrProofBindingPending
}

func minProofAttempt(left, right uint32) uint32 {
	if left < right {
		return left
	}
	return right
}

func (s *ProofService) ListForParticipant(ctx context.Context, userID uint64, orderID string) ([]Proof, error) {
	orderID = strings.TrimSpace(orderID)
	if userID == 0 || !ValidULID(orderID) {
		return nil, ErrInvalidProof
	}
	if _, err := s.repository.FindParticipantOrder(ctx, orderID, userID); err != nil {
		return nil, err
	}
	return s.repository.ListProofs(ctx, orderID)
}

func (s *ProofService) ResolveForParticipant(
	ctx context.Context,
	userID uint64,
	orderID string,
	proofType ProofType,
	slot uint16,
) (string, error) {
	orderID = strings.TrimSpace(orderID)
	if userID == 0 || !ValidULID(orderID) || !proofType.Valid() || slot != 0 {
		return "", ErrInvalidProof
	}
	if _, err := s.repository.FindParticipantOrder(ctx, orderID, userID); err != nil {
		return "", err
	}
	proof, err := s.repository.FindProof(ctx, orderID, proofType, slot)
	if err != nil {
		return "", err
	}
	if proof.Status != ProofActive {
		return "", ErrProofNotActive
	}
	return s.media.ResolveProofURL(ctx, proof.ProofPublicID, proof.Slot, PrivateProofURLTTL)
}
