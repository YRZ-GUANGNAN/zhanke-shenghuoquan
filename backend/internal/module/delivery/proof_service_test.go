package delivery

import (
	"context"
	"errors"
	"testing"
	"time"
)

type proofRepositoryStub struct {
	proof       Proof
	order       Order
	createErr   error
	participant bool
	assigned    bool
	lease       ProofBindingLease
	claim       bool
	completed   bool
	failed      bool
}

func (r *proofRepositoryStub) CreateProofAndTransition(_ context.Context, value *Proof, _ uint64, _, _ Status, _ time.Time) (Proof, Order, error) {
	if r.createErr != nil {
		return Proof{}, Order{}, r.createErr
	}
	if r.proof.ProofPublicID == "" {
		r.proof = *value
	}
	return r.proof, r.order, nil
}
func (r *proofRepositoryStub) FindProof(context.Context, string, ProofType, uint16) (Proof, error) {
	if r.proof.ProofPublicID == "" {
		return Proof{}, ErrNotFound
	}
	return r.proof, nil
}
func (r *proofRepositoryStub) ListProofs(context.Context, string) ([]Proof, error) {
	return []Proof{r.proof}, nil
}
func (r *proofRepositoryStub) FindParticipantOrder(context.Context, string, uint64) (Order, error) {
	if !r.participant {
		return Order{}, ErrForbiddenOrder
	}
	return r.order, nil
}
func (r *proofRepositoryStub) FindAssignedRiderOrder(context.Context, string, uint64) (Order, error) {
	if !r.assigned {
		return Order{}, ErrForbiddenOrder
	}
	return r.order, nil
}
func (r *proofRepositoryStub) ClaimProofBinding(context.Context, string, time.Time, time.Duration) (ProofBindingLease, bool, error) {
	return r.lease, r.claim, nil
}
func (r *proofRepositoryStub) ClaimNextProofBinding(context.Context, time.Time, time.Duration) (ProofBindingLease, bool, error) {
	return r.lease, r.claim, nil
}
func (r *proofRepositoryStub) CompleteProofBinding(context.Context, ProofBindingLease, time.Time) error {
	r.completed = true
	r.proof.Status = ProofActive
	return nil
}
func (r *proofRepositoryStub) FailProofBinding(context.Context, ProofBindingLease, time.Time, uint32, time.Time) (ProofStatus, error) {
	r.failed = true
	return ProofBinding, nil
}

type proofMediaStub struct {
	validateErr error
	publishErr  error
	url         string
}

func (m *proofMediaStub) ValidateOwnedApprovedAsset(context.Context, uint64, string) error {
	return m.validateErr
}
func (m *proofMediaStub) PublishProofReference(context.Context, string, string, uint16) error {
	return m.publishErr
}
func (m *proofMediaStub) ResolveProofURL(context.Context, string, uint16, time.Duration) (string, error) {
	return m.url, nil
}

func TestProofServiceRejectsUnapprovedAssetBeforeDeliveryTransaction(t *testing.T) {
	repository := &proofRepositoryStub{assigned: true}
	service, err := NewProofService(repository, &proofMediaStub{validateErr: errors.New("not approved")}, ProofServiceConfig{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = service.AdvanceWithProof(context.Background(), ProofTransitionInput{
		OrderPublicID: "01J00000000000000000000001", RiderUserID: 9,
		ExpectedRowVersion: 3, ProofType: ProofPickup,
		AssetPublicID: "01J00000000000000000000002",
	})
	if !errors.Is(err, ErrProofAssetRejected) || repository.proof.ProofPublicID != "" {
		t.Fatalf("err=%v repository=%#v", err, repository)
	}
}

func TestProofServicePersistsThenPublishesWithLease(t *testing.T) {
	now := time.Date(2026, 6, 30, 1, 2, 3, 0, time.UTC)
	repository := &proofRepositoryStub{
		order: Order{DeliveryOrderPublicID: "01J00000000000000000000001", Status: StatusPickedUp, RowVersion: 4},
		claim: true, assigned: true,
	}
	media := &proofMediaStub{}
	service, err := NewProofService(repository, media, ProofServiceConfig{Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	// The repository returns the candidate, then the claim fixture is populated
	// with the same durable fact to emulate the binding worker.
	repository.lease = ProofBindingLease{
		Proof: Proof{
			ID: 1, ProofPublicID: "01J00000000000000000000003",
			DeliveryOrderPublicID: "01J00000000000000000000001",
			ProofType:             ProofPickup, AssetPublicID: "01J00000000000000000000002",
			UploaderUserID: 9, Status: ProofBinding, OperationID: "01J00000000000000000000004",
			RowVersion: 2, AttemptCount: 1,
		},
		LeaseToken: "01J00000000000000000000005",
	}
	order, _, err := service.AdvanceWithProof(context.Background(), ProofTransitionInput{
		OrderPublicID: "01J00000000000000000000001", RiderUserID: 9,
		ExpectedRowVersion: 3, ProofType: ProofPickup,
		AssetPublicID: "01J00000000000000000000002",
	})
	if err != nil || order.Status != StatusPickedUp || !repository.completed || repository.failed {
		t.Fatalf("order=%#v err=%v completed=%v failed=%v", order, err, repository.completed, repository.failed)
	}
}

func TestProofServiceBindingFailureRemainsDurable(t *testing.T) {
	repository := &proofRepositoryStub{
		order: Order{DeliveryOrderPublicID: "01J00000000000000000000001", Status: StatusDelivered, RowVersion: 8},
		proof: Proof{
			ProofPublicID:         "01J00000000000000000000003",
			DeliveryOrderPublicID: "01J00000000000000000000001",
			ProofType:             ProofDelivery, AssetPublicID: "01J00000000000000000000002",
			UploaderUserID: 9, Status: ProofBinding,
		},
		lease: ProofBindingLease{
			Proof:      Proof{ID: 1, Status: ProofBinding, RowVersion: 2, AttemptCount: 1},
			LeaseToken: "01J00000000000000000000005",
		},
		claim: true, assigned: true,
	}
	service, err := NewProofService(repository, &proofMediaStub{publishErr: errors.New("temporary")}, ProofServiceConfig{})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = service.AdvanceWithProof(context.Background(), ProofTransitionInput{
		OrderPublicID: "01J00000000000000000000001", RiderUserID: 9,
		ExpectedRowVersion: 7, ProofType: ProofDelivery,
		AssetPublicID: "01J00000000000000000000002",
	})
	if !errors.Is(err, ErrProofBindingPending) || !repository.failed || repository.completed {
		t.Fatalf("err=%v completed=%v failed=%v", err, repository.completed, repository.failed)
	}
}

func TestProofParticipantFenceAndPrivateTTL(t *testing.T) {
	proof := Proof{
		ProofPublicID:         "01J00000000000000000000003",
		DeliveryOrderPublicID: "01J00000000000000000000001",
		ProofType:             ProofPickup, Slot: 0, Status: ProofActive,
	}
	repository := &proofRepositoryStub{proof: proof, participant: false}
	media := &proofMediaStub{url: "https://signed.example/private"}
	service, err := NewProofService(repository, media, ProofServiceConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveForParticipant(context.Background(), 7, proof.DeliveryOrderPublicID, ProofPickup, 0); !errors.Is(err, ErrForbiddenOrder) {
		t.Fatalf("nonparticipant err=%v", err)
	}
	repository.participant = true
	url, err := service.ResolveForParticipant(context.Background(), 7, proof.DeliveryOrderPublicID, ProofPickup, 0)
	if err != nil || url != media.url {
		t.Fatalf("url=%q err=%v", url, err)
	}
}

func TestProofModelAndTransitionMatrix(t *testing.T) {
	if !ProofPickup.Valid() || !ProofDelivery.Valid() || ProofType("other").Valid() {
		t.Fatal("proof type validation mismatch")
	}
	if !ProofDeadletter.Terminal() || !ProofRevoked.Terminal() || ProofActive.Terminal() {
		t.Fatal("proof terminal states mismatch")
	}
	if !proofTransitionMatches(ProofPickup, StatusToPickup, StatusPickedUp) ||
		!proofTransitionMatches(ProofDelivery, StatusDelivering, StatusDelivered) ||
		proofTransitionMatches(ProofPickup, StatusDelivering, StatusDelivered) {
		t.Fatal("proof transition matrix mismatch")
	}
	if !CanTransitionProof(ProofBinding, ProofActive) ||
		!CanTransitionProof(ProofBinding, ProofDeadletter) ||
		!CanTransitionProof(ProofActive, ProofRevoked) ||
		CanTransitionProof(ProofDeadletter, ProofActive) {
		t.Fatal("proof status transition matrix mismatch")
	}
}
