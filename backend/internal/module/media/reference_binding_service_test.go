package media

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

const (
	testBindingULID1  = "01J00000000000000000000001"
	testBindingULID2  = "01J00000000000000000000002"
	testLegacyMediaID = "0123456789abcdef0123456789abcdef"
)

type bindingRepositoryStub struct {
	reserved        []BindingAsset
	reserveErr      error
	commitErr       error
	exact           bool
	exactErr        error
	reserveCalls    int
	commitCalls     int
	cancelCalls     int
	publishCalls    int
	revokeCalls     int
	cancelOwner     uint64
	cancelToken     string
	publishedType   Purpose
	publishedID     string
	revokedType     Purpose
	revokedID       string
	revokedKeep     []uint64
	revocationFence string
}

func (r *bindingRepositoryStub) ReserveBinding(_ context.Context, _ BindingReserveInput) ([]BindingAsset, error) {
	r.reserveCalls++
	return append([]BindingAsset(nil), r.reserved...), r.reserveErr
}
func (r *bindingRepositoryStub) CommitBinding(_ context.Context, _ BindingCommitInput) error {
	r.commitCalls++
	return r.commitErr
}
func (r *bindingRepositoryStub) ExactCommittedBinding(_ context.Context, _ BindingCommitInput) (bool, error) {
	return r.exact, r.exactErr
}
func (r *bindingRepositoryStub) CancelBinding(_ context.Context, ownerID uint64, token string) error {
	r.cancelCalls++
	r.cancelOwner, r.cancelToken = ownerID, token
	return nil
}
func (r *bindingRepositoryStub) PublishBinding(_ context.Context, referenceType Purpose, referenceID string) error {
	r.publishCalls++
	r.publishedType, r.publishedID = referenceType, referenceID
	return nil
}
func (r *bindingRepositoryStub) RevokeBinding(_ context.Context, referenceType Purpose, referenceID string, keep []uint64, fence string) error {
	r.revokeCalls++
	r.revokedType, r.revokedID = referenceType, referenceID
	r.revokedKeep, r.revocationFence = append([]uint64(nil), keep...), fence
	return nil
}

func TestReferenceBindingReserveRejectsDuplicateAssetIDs(t *testing.T) {
	repo := &bindingRepositoryStub{}
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	service, err := NewReferenceBindingService(repo, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Reserve(context.Background(), BindingReserveInput{
		OwnerID: 7, Purpose: PurposeProduct,
		OrderedAssetPublicIDs: []string{testBindingULID1, testBindingULID1},
		Token:                 "m10_" + testBindingULID2, ExpiresAt: now.Add(time.Minute),
	})
	if !errors.Is(err, ErrBindingInvalidAsset) {
		t.Fatalf("err=%v want ErrBindingInvalidAsset", err)
	}
	if repo.reserveCalls != 0 {
		t.Fatal("invalid input reached repository")
	}
}

func TestReferenceBindingReservePreservesCallerOrder(t *testing.T) {
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	repo := &bindingRepositoryStub{reserved: []BindingAsset{
		{InternalID: 1, AssetPublicID: testBindingULID1},
		{InternalID: 2, AssetPublicID: testBindingULID2, Kept: true},
	}}
	service, _ := NewReferenceBindingService(repo, func() time.Time { return now })
	reservation, err := service.Reserve(context.Background(), BindingReserveInput{
		OwnerID: 7, Purpose: PurposeProduct,
		OrderedAssetPublicIDs: []string{testBindingULID2, testBindingULID1},
		Token:                 "m10_" + testBindingULID1, ExpiresAt: now.Add(time.Minute), ExistingReferenceID: "42",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(reservation.References) != 2 || reservation.References[0].AssetPublicID != testBindingULID2 ||
		reservation.References[0].Slot != 0 || !reservation.References[0].Kept || reservation.References[1].Slot != 1 {
		t.Fatalf("reservation=%+v", reservation)
	}
}

func TestReferenceBindingReservePropagatesLegacyClassification(t *testing.T) {
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	repo := &bindingRepositoryStub{reserveErr: ErrBindingLegacySet}
	service, _ := NewReferenceBindingService(repo, func() time.Time { return now })
	_, err := service.Reserve(context.Background(), BindingReserveInput{
		OwnerID: 7, Purpose: PurposeProduct, OrderedAssetPublicIDs: []string{testLegacyMediaID},
		Token: "m10_" + testBindingULID2, ExpiresAt: now.Add(time.Minute),
	})
	if !errors.Is(err, ErrBindingLegacySet) {
		t.Fatalf("err=%v want ErrBindingLegacySet", err)
	}
}

func TestReferenceBindingReserveRejectsInvalidLookupIDBeforeRepository(t *testing.T) {
	now := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	repo := &bindingRepositoryStub{}
	service, _ := NewReferenceBindingService(repo, func() time.Time { return now })
	_, err := service.Reserve(context.Background(), BindingReserveInput{
		OwnerID: 7, Purpose: PurposeProduct, OrderedAssetPublicIDs: []string{"not-an-asset"},
		Token: "m10_" + testBindingULID2, ExpiresAt: now.Add(time.Minute),
	})
	if !errors.Is(err, ErrBindingInvalidAsset) || repo.reserveCalls != 0 {
		t.Fatalf("err=%v reserveCalls=%d", err, repo.reserveCalls)
	}
}

func TestReferenceBindingEnsureCommittedAcceptsExactReplay(t *testing.T) {
	repo := &bindingRepositoryStub{commitErr: ErrBindingConflict, exact: true}
	service, _ := NewReferenceBindingService(repo, time.Now)
	err := service.EnsureCommitted(context.Background(), BindingCommitInput{
		OwnerID: 7, Token: "m10_" + testBindingULID2,
		ReferenceType: PurposeProduct, ReferenceID: "42",
		OrderedAssetPublicIDs: []string{testBindingULID1},
	})
	if err != nil || repo.commitCalls != 1 {
		t.Fatalf("err=%v commitCalls=%d", err, repo.commitCalls)
	}
}

func TestReferenceBindingEnsureCommittedRejectsMismatchedReplay(t *testing.T) {
	repo := &bindingRepositoryStub{commitErr: ErrBindingConflict, exact: false}
	service, _ := NewReferenceBindingService(repo, time.Now)
	err := service.EnsureCommitted(context.Background(), BindingCommitInput{
		OwnerID: 7, Token: "m10_" + testBindingULID2,
		ReferenceType: PurposeProduct, ReferenceID: "42",
		OrderedAssetPublicIDs: []string{testBindingULID1},
	})
	if !errors.Is(err, ErrBindingMismatch) {
		t.Fatalf("err=%v want ErrBindingMismatch", err)
	}
}

func TestReferenceBindingCancelPublishAndRevokeValidateCoordinates(t *testing.T) {
	repo := &bindingRepositoryStub{}
	service, _ := NewReferenceBindingService(repo, time.Now)
	if err := service.Cancel(context.Background(), 0, "m10_"+testBindingULID1); !errors.Is(err, ErrBindingInvalidAsset) {
		t.Fatalf("invalid cancel err=%v", err)
	}
	if repo.cancelCalls != 0 {
		t.Fatal("invalid cancel reached repository")
	}
	if err := service.Cancel(context.Background(), 7, "m10_"+testBindingULID1); err != nil {
		t.Fatal(err)
	}
	if err := service.Publish(context.Background(), PurposeProduct, "42"); err != nil {
		t.Fatal(err)
	}
	if err := service.Revoke(context.Background(), PurposeProduct, "42", []uint64{8, 9}); err != nil {
		t.Fatal(err)
	}
	if repo.cancelOwner != 7 || repo.cancelToken != "m10_"+testBindingULID1 ||
		repo.publishedType != PurposeProduct || repo.publishedID != "42" ||
		repo.revokedType != PurposeProduct || repo.revokedID != "42" ||
		!reflect.DeepEqual(repo.revokedKeep, []uint64{8, 9}) || !ValidULID(repo.revocationFence) {
		t.Fatalf("repo=%+v", repo)
	}
}
