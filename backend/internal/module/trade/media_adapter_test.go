package trade

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
	legacymedia "campus-circle/backend/internal/pkg/media"
)

const (
	adapterAssetID1 = "01J00000000000000000000001"
	adapterAssetID2 = "01J00000000000000000000002"
)

type bindingGatewayStub struct {
	reserveInput  coremedia.BindingReserveInput
	commitInput   coremedia.BindingCommitInput
	ensureInput   coremedia.BindingCommitInput
	cancelOwner   uint64
	cancelToken   string
	publishType   coremedia.Purpose
	publishID     string
	revokeType    coremedia.Purpose
	revokeID      string
	revokeKeep    []uint64
	reserveResult coremedia.BindingReservation
	reserveErr    error
	commitErr     error
	ensureErr     error
	cancelErr     error
	publishErr    error
	revokeErr     error
	reserveCalls  int
	commitCalls   int
	ensureCalls   int
	cancelCalls   int
	publishCalls  int
	revokeCalls   int
}

func (s *bindingGatewayStub) Reserve(_ context.Context, input coremedia.BindingReserveInput) (coremedia.BindingReservation, error) {
	s.reserveCalls++
	s.reserveInput = input
	result := s.reserveResult
	if result.Token == "" {
		result.Token = input.Token
	}
	return result, s.reserveErr
}
func (s *bindingGatewayStub) Commit(_ context.Context, input coremedia.BindingCommitInput) error {
	s.commitCalls++
	s.commitInput = input
	return s.commitErr
}
func (s *bindingGatewayStub) EnsureCommitted(_ context.Context, input coremedia.BindingCommitInput) error {
	s.ensureCalls++
	s.ensureInput = input
	return s.ensureErr
}
func (s *bindingGatewayStub) Cancel(_ context.Context, ownerID uint64, token string) error {
	s.cancelCalls++
	s.cancelOwner, s.cancelToken = ownerID, token
	return s.cancelErr
}
func (s *bindingGatewayStub) Publish(_ context.Context, referenceType coremedia.Purpose, referenceID string) error {
	s.publishCalls++
	s.publishType, s.publishID = referenceType, referenceID
	return s.publishErr
}
func (s *bindingGatewayStub) Revoke(_ context.Context, referenceType coremedia.Purpose, referenceID string, keep []uint64) error {
	s.revokeCalls++
	s.revokeType, s.revokeID, s.revokeKeep = referenceType, referenceID, append([]uint64(nil), keep...)
	return s.revokeErr
}

type legacyGatewayStub struct {
	reservation          legacymedia.Reservation
	reserveErr           error
	commitErr            error
	ensureErr            error
	cancelErr            error
	deleteErr            error
	reserveCalls         int
	commitCalls          int
	ensureCalls          int
	cancelCalls          int
	deleteCalls          int
	lastEnsureToken      string
	lastEnsureOrderedIDs []string
}

func (s *legacyGatewayStub) Reserve(context.Context, uint, legacymedia.Purpose, []string) (legacymedia.Reservation, error) {
	s.reserveCalls++
	return s.reservation, s.reserveErr
}
func (s *legacyGatewayStub) Commit(context.Context, uint, legacymedia.Reservation, string, uint64) error {
	s.commitCalls++
	return s.commitErr
}
func (s *legacyGatewayStub) EnsureCommitted(_ context.Context, _ uint, token, _ string, _ uint64, orderedIDs []string) error {
	s.ensureCalls++
	s.lastEnsureToken = token
	s.lastEnsureOrderedIDs = append([]string(nil), orderedIDs...)
	return s.ensureErr
}
func (s *legacyGatewayStub) Cancel(context.Context, uint, legacymedia.Reservation) error {
	s.cancelCalls++
	return s.cancelErr
}
func (s *legacyGatewayStub) DeleteReference(context.Context, string, uint64, []uint64) error {
	s.deleteCalls++
	return s.deleteErr
}

func TestHybridMediaGatewayCoreReservePreservesOrderAndStableURL(t *testing.T) {
	core := &bindingGatewayStub{reserveResult: coremedia.BindingReservation{References: []coremedia.BindingAsset{
		{InternalID: 12, AssetPublicID: adapterAssetID2},
		{InternalID: 11, AssetPublicID: adapterAssetID1},
	}}}
	legacy := &legacyGatewayStub{}
	gateway, err := NewHybridMediaGateway(core, legacy, "https://api.example.com/")
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := gateway.Reserve(context.Background(), 7, legacymedia.PurposeProductImage, []string{adapterAssetID2, adapterAssetID1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(reservation.Token, "m10_") || len(reservation.References) != 2 ||
		reservation.References[0].InternalID != 12 || reservation.References[0].PublicID != adapterAssetID2 ||
		reservation.References[0].URL != "https://api.example.com/api/v1/media/assets/"+adapterAssetID2+"/content" || legacy.reserveCalls != 0 {
		t.Fatalf("reservation=%+v legacyCalls=%d", reservation, legacy.reserveCalls)
	}
	if core.reserveInput.Purpose != coremedia.PurposeProduct || core.reserveInput.OwnerID != 7 ||
		!core.reserveInput.ExpiresAt.After(time.Now()) {
		t.Fatalf("core input=%+v", core.reserveInput)
	}
}

func TestHybridMediaGatewayFallsBackOnlyForLegacyClassification(t *testing.T) {
	legacyReservation := legacymedia.Reservation{Token: "legacy-token", References: []legacymedia.Reference{{InternalID: 31, PublicID: "legacy-id"}}}
	t.Run("legacy only", func(t *testing.T) {
		core := &bindingGatewayStub{reserveErr: coremedia.ErrBindingLegacySet}
		legacy := &legacyGatewayStub{reservation: legacyReservation}
		gateway, _ := NewHybridMediaGateway(core, legacy, "https://api.example.com")
		reservation, err := gateway.Reserve(context.Background(), 7, legacymedia.PurposeProductImage, []string{"legacy-id"})
		if err != nil || reservation.Token != "legacy-token" || legacy.reserveCalls != 1 {
			t.Fatalf("reservation=%+v err=%v legacyCalls=%d", reservation, err, legacy.reserveCalls)
		}
	})
	t.Run("mixed fail closed", func(t *testing.T) {
		core := &bindingGatewayStub{reserveErr: coremedia.ErrBindingInvalidAsset}
		legacy := &legacyGatewayStub{reservation: legacyReservation}
		gateway, _ := NewHybridMediaGateway(core, legacy, "https://api.example.com")
		_, err := gateway.Reserve(context.Background(), 7, legacymedia.PurposeProductImage, []string{adapterAssetID1, "legacy-id"})
		if !errors.Is(err, ErrMediaInvalid) || legacy.reserveCalls != 0 {
			t.Fatalf("err=%v legacyCalls=%d", err, legacy.reserveCalls)
		}
	})
}

func TestHybridMediaGatewayRoutesCommitEnsureAndCancelByToken(t *testing.T) {
	core := &bindingGatewayStub{}
	legacy := &legacyGatewayStub{}
	gateway, _ := NewHybridMediaGateway(core, legacy, "https://api.example.com")
	coreReservation := legacymedia.Reservation{Token: "m10_" + adapterAssetID1, References: []legacymedia.Reference{{InternalID: 11, PublicID: adapterAssetID2}}}
	if err := gateway.Commit(context.Background(), 7, coreReservation, "product", 42); err != nil {
		t.Fatal(err)
	}
	if err := gateway.EnsureCommitted(context.Background(), 7, coreReservation.Token, "product", 42, []string{adapterAssetID2}); err != nil {
		t.Fatal(err)
	}
	if err := gateway.Cancel(context.Background(), 7, coreReservation); err != nil {
		t.Fatal(err)
	}
	if core.commitCalls != 1 || core.ensureCalls != 1 || core.cancelCalls != 1 || legacy.commitCalls+legacy.ensureCalls+legacy.cancelCalls != 0 ||
		core.commitInput.ReferenceID != "42" || core.cancelToken != coreReservation.Token {
		t.Fatalf("core=%+v legacy=%+v", core, legacy)
	}

	legacyReservation := legacymedia.Reservation{Token: "legacy-token", References: []legacymedia.Reference{{InternalID: 31, PublicID: "legacy-id"}}}
	if err := gateway.Commit(context.Background(), 7, legacyReservation, "product", 42); err != nil {
		t.Fatal(err)
	}
	if err := gateway.EnsureCommitted(context.Background(), 7, legacyReservation.Token, "product", 42, []string{"legacy-id"}); err != nil {
		t.Fatal(err)
	}
	if err := gateway.Cancel(context.Background(), 7, legacyReservation); err != nil {
		t.Fatal(err)
	}
	if legacy.commitCalls != 1 || legacy.ensureCalls != 1 || legacy.cancelCalls != 1 {
		t.Fatalf("legacy=%+v", legacy)
	}
}

func TestHybridMediaGatewayReserveForProductAndLifecycle(t *testing.T) {
	core := &bindingGatewayStub{reserveResult: coremedia.BindingReservation{References: []coremedia.BindingAsset{
		{InternalID: 11, AssetPublicID: adapterAssetID1, Kept: true},
		{InternalID: 12, AssetPublicID: adapterAssetID2},
	}}}
	legacy := &legacyGatewayStub{}
	gateway, _ := NewHybridMediaGateway(core, legacy, "https://api.example.com")
	reservation, err := gateway.ReserveForProduct(context.Background(), 7, 42, legacymedia.PurposeProductImage, []string{adapterAssetID1, adapterAssetID2})
	if err != nil || core.reserveInput.ExistingReferenceID != "42" || len(reservation.References) != 2 {
		t.Fatalf("reservation=%+v input=%+v err=%v", reservation, core.reserveInput, err)
	}
	if err := gateway.PublishProduct(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	if err := gateway.DeleteReference(context.Background(), "product", 42, []uint64{11, 12}); err != nil {
		t.Fatal(err)
	}
	if core.publishCalls != 1 || core.publishType != coremedia.PurposeProduct || core.publishID != "42" ||
		core.revokeCalls != 1 || legacy.deleteCalls != 1 || len(core.revokeKeep) != 2 {
		t.Fatalf("core=%+v legacy=%+v", core, legacy)
	}
}

func TestHybridMediaGatewayMapsCoreErrors(t *testing.T) {
	tests := []struct {
		name    string
		coreErr error
		want    error
	}{
		{name: "invalid", coreErr: coremedia.ErrBindingInvalidAsset, want: ErrMediaInvalid},
		{name: "mismatch", coreErr: coremedia.ErrBindingMismatch, want: ErrMediaInvalid},
		{name: "conflict", coreErr: coremedia.ErrBindingConflict, want: ErrMediaConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			core := &bindingGatewayStub{reserveErr: test.coreErr}
			legacy := &legacyGatewayStub{}
			gateway, _ := NewHybridMediaGateway(core, legacy, "https://api.example.com")
			_, err := gateway.Reserve(context.Background(), 7, legacymedia.PurposeProductImage, []string{adapterAssetID1})
			if !errors.Is(err, test.want) || legacy.reserveCalls != 0 {
				t.Fatalf("err=%v want=%v legacyCalls=%d", err, test.want, legacy.reserveCalls)
			}
		})
	}
}
