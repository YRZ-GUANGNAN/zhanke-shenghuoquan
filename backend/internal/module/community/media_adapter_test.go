package community

import (
	"context"
	"errors"
	"strings"
	"testing"

	coremedia "campus-circle/backend/internal/module/media"
	legacymedia "campus-circle/backend/internal/pkg/media"
)

const communityAdapterAssetID = "01J00000000000000000000001"

type communityBindingStub struct {
	reserveInput  coremedia.BindingReserveInput
	commitInput   coremedia.BindingCommitInput
	ensureInput   coremedia.BindingCommitInput
	reserveResult coremedia.BindingReservation
	reserveErr    error
	commitCalls   int
	ensureCalls   int
	cancelCalls   int
	revokeCalls   int
}

func (s *communityBindingStub) Reserve(_ context.Context, input coremedia.BindingReserveInput) (coremedia.BindingReservation, error) {
	s.reserveInput = input
	result := s.reserveResult
	if result.Token == "" {
		result.Token = input.Token
	}
	return result, s.reserveErr
}
func (s *communityBindingStub) Commit(_ context.Context, input coremedia.BindingCommitInput) error {
	s.commitCalls++
	s.commitInput = input
	return nil
}
func (s *communityBindingStub) EnsureCommitted(_ context.Context, input coremedia.BindingCommitInput) error {
	s.ensureCalls++
	s.ensureInput = input
	return nil
}
func (s *communityBindingStub) Cancel(context.Context, uint64, string) error {
	s.cancelCalls++
	return nil
}
func (s *communityBindingStub) Revoke(context.Context, coremedia.Purpose, string, []uint64) error {
	s.revokeCalls++
	return nil
}

type communityLegacyMediaStub struct {
	reservation  legacymedia.Reservation
	reserveCalls int
	commitCalls  int
	ensureCalls  int
	cancelCalls  int
	deleteCalls  int
}

func (s *communityLegacyMediaStub) Reserve(context.Context, uint, legacymedia.Purpose, []string) (legacymedia.Reservation, error) {
	s.reserveCalls++
	return s.reservation, nil
}
func (s *communityLegacyMediaStub) Commit(context.Context, uint, legacymedia.Reservation, string, uint64) error {
	s.commitCalls++
	return nil
}
func (s *communityLegacyMediaStub) EnsureCommitted(context.Context, uint, string, string, uint64, []string) error {
	s.ensureCalls++
	return nil
}
func (s *communityLegacyMediaStub) Cancel(context.Context, uint, legacymedia.Reservation) error {
	s.cancelCalls++
	return nil
}
func (s *communityLegacyMediaStub) DeleteReference(context.Context, string, uint64, []uint64) error {
	s.deleteCalls++
	return nil
}

func TestHybridMediaGatewayUsesCorePostBinding(t *testing.T) {
	core := &communityBindingStub{reserveResult: coremedia.BindingReservation{
		References: []coremedia.BindingAsset{{InternalID: 13, AssetPublicID: communityAdapterAssetID}},
	}}
	legacy := &communityLegacyMediaStub{}
	gateway, err := NewHybridMediaGateway(core, legacy, "https://api.example.com/")
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := gateway.Reserve(context.Background(), 2, legacymedia.PurposePostImage, []string{communityAdapterAssetID})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(reservation.Token, "m10_") || len(reservation.References) != 1 ||
		reservation.References[0].InternalID != 13 ||
		reservation.References[0].URL != "https://api.example.com/api/v1/media/assets/"+communityAdapterAssetID+"/content" {
		t.Fatalf("reservation=%+v", reservation)
	}
	if core.reserveInput.OwnerID != 2 || core.reserveInput.Purpose != coremedia.PurposePost || legacy.reserveCalls != 0 {
		t.Fatalf("core=%+v legacyCalls=%d", core.reserveInput, legacy.reserveCalls)
	}
}

func TestHybridMediaGatewayFallsBackForLegacyPostAssets(t *testing.T) {
	core := &communityBindingStub{reserveErr: coremedia.ErrBindingLegacySet}
	legacy := &communityLegacyMediaStub{reservation: legacymedia.Reservation{Token: "legacy-token"}}
	gateway, _ := NewHybridMediaGateway(core, legacy, "https://api.example.com")
	reservation, err := gateway.Reserve(context.Background(), 2, legacymedia.PurposePostImage, []string{"legacy-id"})
	if err != nil || reservation.Token != "legacy-token" || legacy.reserveCalls != 1 {
		t.Fatalf("reservation=%+v err=%v legacyCalls=%d", reservation, err, legacy.reserveCalls)
	}
}

func TestHybridMediaGatewayRoutesCoreLifecycleByToken(t *testing.T) {
	core := &communityBindingStub{}
	legacy := &communityLegacyMediaStub{}
	gateway, _ := NewHybridMediaGateway(core, legacy, "https://api.example.com")
	reservation := legacymedia.Reservation{
		Token:      "m10_" + communityAdapterAssetID,
		References: []legacymedia.Reference{{InternalID: 13, PublicID: communityAdapterAssetID}},
	}
	if err := gateway.Commit(context.Background(), 2, reservation, "post", 9); err != nil {
		t.Fatal(err)
	}
	if err := gateway.EnsureCommitted(context.Background(), 2, reservation.Token, "post", 9, []string{communityAdapterAssetID}); err != nil {
		t.Fatal(err)
	}
	if err := gateway.Cancel(context.Background(), 2, reservation); err != nil {
		t.Fatal(err)
	}
	if core.commitCalls != 1 || core.ensureCalls != 1 || core.cancelCalls != 1 ||
		core.commitInput.ReferenceType != coremedia.PurposePost || core.commitInput.ReferenceID != "9" ||
		legacy.commitCalls+legacy.ensureCalls+legacy.cancelCalls != 0 {
		t.Fatalf("core=%+v legacy=%+v", core, legacy)
	}
}

func TestHybridMediaGatewayMapsCoreBindingErrors(t *testing.T) {
	tests := []struct {
		err  error
		want error
	}{
		{err: coremedia.ErrBindingConflict, want: legacymedia.ErrAlreadyBound},
		{err: coremedia.ErrBindingInvalidAsset, want: legacymedia.ErrInvalidAsset},
		{err: coremedia.ErrBindingMismatch, want: legacymedia.ErrInvalidAsset},
	}
	for _, test := range tests {
		core := &communityBindingStub{reserveErr: test.err}
		legacy := &communityLegacyMediaStub{}
		gateway, _ := NewHybridMediaGateway(core, legacy, "https://api.example.com")
		_, err := gateway.Reserve(context.Background(), 2, legacymedia.PurposePostImage, []string{communityAdapterAssetID})
		if !errors.Is(err, test.want) || legacy.reserveCalls != 0 {
			t.Fatalf("err=%v want=%v legacyCalls=%d", err, test.want, legacy.reserveCalls)
		}
	}
}
