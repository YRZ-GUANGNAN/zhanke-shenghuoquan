package delivery

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"
)

const d4ProofMerchantUserID uint64 = 9_004_004_001

func TestD4ProofTransactionCASIdempotencyLeaseAndParticipantAgainstMySQL84(t *testing.T) {
	db := openD1MySQL(t)
	assertD1MySQL84(t, db)
	for _, table := range []any{&Order{}, &Proof{}} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required D-4 migration table missing for %T", table)
		}
	}
	cleanupD4ProofMySQL(t, db)
	t.Cleanup(func() { cleanupD4ProofMySQL(t, db) })

	now := time.Now().UTC().Truncate(time.Millisecond)
	riderID := uint64(9_004_004_002)
	order := Order{
		DeliveryOrderPublicID: "01J00000000000000000000401",
		MerchantUserID:        d4ProofMerchantUserID, RiderUserID: &riderID,
		PickupContactName: "Pickup", PickupContactPhone: "13100004001", PickupAddress: "Pickup A",
		RecipientName: "Recipient", RecipientPhone: "13100004002", DeliveryAddress: "Delivery B",
		Remark: "", DeliveryFeeCents: 150, RiderFeeCents: 100, PlatformCutCents: 50,
		Status: StatusToPickup, ReadyForClaimAt: &now, ClaimedAt: &now, ToPickupAt: &now,
		RowVersion: 3, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}

	repository := NewRepository(db)
	candidate := Proof{
		ProofPublicID:         "01J00000000000000000000402",
		DeliveryOrderPublicID: order.DeliveryOrderPublicID,
		ProofType:             ProofPickup, Slot: 0,
		AssetPublicID: "01J00000000000000000000403", UploaderUserID: riderID,
		Status: ProofBinding, OperationID: "01J00000000000000000000404",
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	proof, advanced, err := repository.CreateProofAndTransition(
		context.Background(), &candidate, 3, StatusToPickup, StatusPickedUp, now.Add(time.Second),
	)
	if err != nil || advanced.Status != StatusPickedUp || advanced.RowVersion != 4 ||
		proof.Status != ProofBinding || proof.ID == 0 {
		t.Fatalf("proof=%#v order=%#v err=%v", proof, advanced, err)
	}

	retryCandidate := candidate
	retryCandidate.ID = 0
	retryCandidate.ProofPublicID = "01J00000000000000000000405"
	retryCandidate.OperationID = "01J00000000000000000000406"
	idempotent, sameOrder, err := repository.CreateProofAndTransition(
		context.Background(), &retryCandidate, 3, StatusToPickup, StatusPickedUp, now.Add(2*time.Second),
	)
	if err != nil || idempotent.ID != proof.ID || sameOrder.RowVersion != advanced.RowVersion {
		t.Fatalf("idempotent=%#v order=%#v err=%v", idempotent, sameOrder, err)
	}
	conflict := retryCandidate
	conflict.AssetPublicID = "01J00000000000000000000407"
	if _, _, err := repository.CreateProofAndTransition(
		context.Background(), &conflict, 4, StatusToPickup, StatusPickedUp, now,
	); !errors.Is(err, ErrProofConflict) {
		t.Fatalf("different asset err=%v", err)
	}

	if _, err := repository.FindParticipantOrder(context.Background(), order.DeliveryOrderPublicID, d4ProofMerchantUserID); err != nil {
		t.Fatalf("merchant participant err=%v", err)
	}
	if _, err := repository.FindParticipantOrder(context.Background(), order.DeliveryOrderPublicID, riderID); err != nil {
		t.Fatalf("rider participant err=%v", err)
	}
	if _, err := repository.FindParticipantOrder(context.Background(), order.DeliveryOrderPublicID, riderID+99); !errors.Is(err, ErrForbiddenOrder) {
		t.Fatalf("nonparticipant err=%v", err)
	}

	lease, found, err := repository.ClaimProofBinding(context.Background(), proof.ProofPublicID, now.Add(3*time.Second), time.Minute)
	if err != nil || !found || lease.Proof.AttemptCount != 1 || lease.LeaseToken == "" {
		t.Fatalf("lease=%#v found=%v err=%v", lease, found, err)
	}
	if _, found, err := repository.ClaimProofBinding(context.Background(), proof.ProofPublicID, now.Add(30*time.Second), time.Minute); err != nil || found {
		t.Fatalf("unexpired lease was reclaimed found=%v err=%v", found, err)
	}
	recovered, found, err := repository.ClaimProofBinding(context.Background(), proof.ProofPublicID, now.Add(64*time.Second), time.Minute)
	if err != nil || !found || recovered.LeaseToken == lease.LeaseToken || recovered.Proof.AttemptCount != 2 {
		t.Fatalf("recovered=%#v found=%v err=%v", recovered, found, err)
	}
	if err := repository.CompleteProofBinding(context.Background(), lease, now.Add(65*time.Second)); !errors.Is(err, ErrProofLease) {
		t.Fatalf("stale lease completion err=%v", err)
	}
	if err := repository.CompleteProofBinding(context.Background(), recovered, now.Add(65*time.Second)); err != nil {
		t.Fatal(err)
	}
	active, err := repository.FindProof(context.Background(), order.DeliveryOrderPublicID, ProofPickup, 0)
	if err != nil || active.Status != ProofActive || active.ActivatedAt == nil {
		t.Fatalf("active=%#v err=%v", active, err)
	}

	var proofCount int64
	if err := db.Model(&Proof{}).Where("delivery_order_public_id = ?", order.DeliveryOrderPublicID).Count(&proofCount).Error; err != nil {
		t.Fatal(err)
	}
	if proofCount != 1 {
		t.Fatalf("proof count=%d want=1", proofCount)
	}
	t.Log("d4_proof_transaction=PASS cas=PASS idempotency=PASS participant_fence=PASS lease_recovery=PASS")
}

func cleanupD4ProofMySQL(t *testing.T, db *gorm.DB) {
	t.Helper()
	var orderIDs []string
	if err := db.Model(&Order{}).Where("merchant_user_id = ?", d4ProofMerchantUserID).
		Pluck("delivery_order_public_id", &orderIDs).Error; err != nil {
		t.Errorf("list D-4 proof orders: %v", err)
		return
	}
	if len(orderIDs) > 0 {
		if err := db.Where("delivery_order_public_id IN ?", orderIDs).Delete(&Proof{}).Error; err != nil {
			t.Errorf("cleanup D-4 proofs: %v", err)
		}
	}
	if err := db.Where("merchant_user_id = ?", d4ProofMerchantUserID).Delete(&Order{}).Error; err != nil {
		t.Errorf("cleanup D-4 orders: %v", err)
	}
}
