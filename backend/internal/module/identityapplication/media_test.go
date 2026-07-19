package identityapplication

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
)

const (
	proofAssetID1 = "01J00000000000000000000001"
	proofAssetID2 = "01J00000000000000000000002"
	proofAppID1   = "01J00000000000000000000003"
	proofAppID2   = "01J00000000000000000000004"
)

type applicationBindingStub struct {
	reserveInput coremedia.BindingReserveInput
	commitInput  coremedia.BindingCommitInput
	ensureInput  coremedia.BindingCommitInput
	cancelOwner  uint64
	cancelToken  string
	reserveErr   error
	commitErr    error
	cancelErr    error
	duplicateIDs bool
}

func (s *applicationBindingStub) Reserve(_ context.Context, input coremedia.BindingReserveInput) (coremedia.BindingReservation, error) {
	s.reserveInput = input
	if s.reserveErr != nil {
		return coremedia.BindingReservation{}, s.reserveErr
	}
	references := make([]coremedia.BindingAsset, len(input.OrderedAssetPublicIDs))
	for index, assetID := range input.OrderedAssetPublicIDs {
		internalID := uint64(index + 1)
		if s.duplicateIDs {
			internalID = 1
		}
		references[index] = coremedia.BindingAsset{InternalID: internalID, AssetPublicID: assetID, Slot: uint16(index)}
	}
	return coremedia.BindingReservation{Token: input.Token, References: references}, nil
}

func (s *applicationBindingStub) Commit(_ context.Context, input coremedia.BindingCommitInput) error {
	s.commitInput = input
	return s.commitErr
}

func (s *applicationBindingStub) EnsureCommitted(_ context.Context, input coremedia.BindingCommitInput) error {
	s.ensureInput = input
	return s.commitErr
}

func (s *applicationBindingStub) Cancel(_ context.Context, ownerID uint64, token string) error {
	s.cancelOwner, s.cancelToken = ownerID, token
	return s.cancelErr
}

type proofRepositoryStub struct {
	assets        map[string]coremedia.Asset
	snapshots     map[string][]coremedia.GatewaySnapshot
	references    map[string]coremedia.AssetReference
	assetCalls    []string
	snapshotCalls []string
	refCalls      []proofReferenceCall
}

type proofReferenceCall struct {
	assetID uint64
	kind    coremedia.Purpose
	parent  string
	slot    uint16
}

func (s *proofRepositoryStub) FindAsset(_ context.Context, publicID string) (coremedia.Asset, error) {
	s.assetCalls = append(s.assetCalls, publicID)
	value, ok := s.assets[publicID]
	if !ok {
		return coremedia.Asset{}, coremedia.ErrNotFound
	}
	return value, nil
}

func (s *proofRepositoryStub) FindGatewayAssetSnapshots(_ context.Context, publicID string) ([]coremedia.GatewaySnapshot, error) {
	s.snapshotCalls = append(s.snapshotCalls, publicID)
	values, ok := s.snapshots[publicID]
	if !ok {
		return nil, coremedia.ErrNotFound
	}
	return append([]coremedia.GatewaySnapshot(nil), values...), nil
}

func (s *proofRepositoryStub) FindReferenceByCoordinates(_ context.Context, assetID uint64, kind coremedia.Purpose, parent string, slot uint16) (coremedia.AssetReference, error) {
	s.refCalls = append(s.refCalls, proofReferenceCall{assetID: assetID, kind: kind, parent: parent, slot: slot})
	value, ok := s.references[proofReferenceKey(assetID, kind, parent, slot)]
	if !ok {
		return coremedia.AssetReference{}, coremedia.ErrNotFound
	}
	return value, nil
}

type proofSignerStub struct {
	calls []proofSignCall
	err   error
}

type proofSignCall struct {
	key       string
	versionID string
	ttl       time.Duration
}

func (s *proofSignerStub) SignedGetURL(_ context.Context, key, versionID string, ttl time.Duration) (string, error) {
	s.calls = append(s.calls, proofSignCall{key: key, versionID: versionID, ttl: ttl})
	if s.err != nil {
		return "", s.err
	}
	return "https://private.example/" + key + "?versionId=" + versionID, nil
}

func TestApplicationMediaReserveCommitCancelUsePrivateCoordinates(t *testing.T) {
	now := time.Date(2026, 7, 19, 8, 0, 0, 0, time.UTC)
	bindings := &applicationBindingStub{}
	adapter, err := NewApplicationMedia(bindings, &proofRepositoryStub{}, &proofSignerStub{}, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	reservation, err := adapter.Reserve(context.Background(), 41, []string{proofAssetID2, proofAssetID1})
	if err != nil {
		t.Fatal(err)
	}
	if bindings.reserveInput.OwnerID != 41 || bindings.reserveInput.Purpose != coremedia.PurposeIdentityApplication ||
		!reflect.DeepEqual(bindings.reserveInput.OrderedAssetPublicIDs, []string{proofAssetID2, proofAssetID1}) ||
		bindings.reserveInput.ExpiresAt != now.Add(identityMediaReservationTTL) ||
		len(reservation.Token) != len("m10_")+26 || reservation.Token[:4] != "m10_" ||
		!reflect.DeepEqual(reservation.AssetPublicIDs, []string{proofAssetID2, proofAssetID1}) ||
		!reflect.DeepEqual(reservation.InternalAssetIDs, []uint64{1, 2}) {
		t.Fatalf("reserve input=%+v reservation=%+v", bindings.reserveInput, reservation)
	}

	if err := adapter.Commit(context.Background(), 41, reservation, proofAppID1); err != nil {
		t.Fatal(err)
	}
	if err := adapter.EnsureCommitted(context.Background(), 41, reservation, proofAppID1); err != nil {
		t.Fatal(err)
	}
	if bindings.ensureInput.OwnerID != 41 || bindings.ensureInput.Token != reservation.Token ||
		bindings.ensureInput.ReferenceType != coremedia.PurposeIdentityApplication || bindings.ensureInput.ReferenceID != proofAppID1 ||
		!reflect.DeepEqual(bindings.ensureInput.OrderedAssetPublicIDs, reservation.AssetPublicIDs) {
		t.Fatalf("ensure input=%+v", bindings.ensureInput)
	}
	if bindings.commitInput.OwnerID != 41 || bindings.commitInput.Token != reservation.Token ||
		bindings.commitInput.ReferenceType != coremedia.PurposeIdentityApplication || bindings.commitInput.ReferenceID != proofAppID1 ||
		!reflect.DeepEqual(bindings.commitInput.OrderedAssetPublicIDs, reservation.AssetPublicIDs) {
		t.Fatalf("commit input=%+v", bindings.commitInput)
	}

	if err := adapter.Cancel(context.Background(), 41, reservation); err != nil {
		t.Fatal(err)
	}
	if bindings.cancelOwner != 41 || bindings.cancelToken != reservation.Token {
		t.Fatalf("cancel owner=%d token=%q", bindings.cancelOwner, bindings.cancelToken)
	}

	if _, err := adapter.ReserveForApplication(context.Background(), 41, proofAppID1, []string{proofAssetID1}); err != nil {
		t.Fatal(err)
	}
	if bindings.reserveInput.ExistingReferenceID != proofAppID1 {
		t.Fatalf("existing reference=%q", bindings.reserveInput.ExistingReferenceID)
	}
	if err := adapter.EnsureCommittedInTransaction(context.Background(), nil, 41, reservation, proofAppID1); !errors.Is(err, ErrProofMediaUnavailable) {
		t.Fatalf("nil transaction err=%v", err)
	}
}

func TestApplicationMediaReservePropagatesOwnerMismatch(t *testing.T) {
	want := errors.New("owner mismatch")
	bindings := &applicationBindingStub{reserveErr: want}
	adapter, err := NewApplicationMedia(bindings, &proofRepositoryStub{}, &proofSignerStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Reserve(context.Background(), 92, []string{proofAssetID1}); !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
	if bindings.reserveInput.OwnerID != 92 {
		t.Fatalf("owner=%d", bindings.reserveInput.OwnerID)
	}
}

func TestApplicationMediaReserveRejectsDuplicateInternalAssetIDs(t *testing.T) {
	bindings := &applicationBindingStub{duplicateIDs: true}
	adapter, err := NewApplicationMedia(bindings, &proofRepositoryStub{}, &proofSignerStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Reserve(context.Background(), 41, []string{proofAssetID1, proofAssetID2}); !errors.Is(err, coremedia.ErrBindingMismatch) {
		t.Fatalf("err=%v", err)
	}
	if bindings.cancelOwner != 41 || bindings.cancelToken == "" {
		t.Fatalf("invalid reservation was not cancelled: owner=%d token=%q", bindings.cancelOwner, bindings.cancelToken)
	}
}

func TestResolveAdminProofsRequiresExactApplicationBindingAndSignsFiveMinutes(t *testing.T) {
	now := time.Date(2026, 7, 19, 8, 30, 0, 0, time.UTC)
	repository := readyProofRepository(proofAppID1, proofAssetID1, proofAssetID2)
	signer := &proofSignerStub{}
	adapter, err := NewApplicationMedia(&applicationBindingStub{}, repository, signer, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}

	proofs, err := adapter.ResolveAdminProofs(context.Background(), proofAppID1, []string{proofAssetID1, proofAssetID2})
	if err != nil {
		t.Fatal(err)
	}
	if len(proofs) != 2 || proofs[0].AssetID != proofAssetID1 || proofs[1].AssetID != proofAssetID2 ||
		proofs[0].ExpiresAt != now.Add(5*time.Minute) || proofs[1].ExpiresAt != now.Add(5*time.Minute) {
		t.Fatalf("proofs=%+v", proofs)
	}
	if !reflect.DeepEqual(repository.assetCalls, []string{proofAssetID1, proofAssetID2}) ||
		!reflect.DeepEqual(repository.snapshotCalls, []string{proofAssetID1, proofAssetID2}) || len(repository.refCalls) != 2 ||
		repository.refCalls[0].parent != proofAppID1 || repository.refCalls[0].slot != 0 ||
		repository.refCalls[1].parent != proofAppID1 || repository.refCalls[1].slot != 1 {
		t.Fatalf("assetCalls=%v refCalls=%+v", repository.assetCalls, repository.refCalls)
	}
	if len(signer.calls) != 2 || signer.calls[0].key != "final/"+proofAssetID1+".webp" ||
		signer.calls[1].key != "final/"+proofAssetID2+".webp" || signer.calls[0].ttl != 5*time.Minute || signer.calls[1].ttl != 5*time.Minute {
		t.Fatalf("signer calls=%+v", signer.calls)
	}
}

func TestResolveAdminProofsUsesAuthoritativeReviewProjection(t *testing.T) {
	repository := readyProofRepository(proofAppID1, proofAssetID1)
	if repository.assets[proofAssetID1].ReviewStatus != coremedia.ReviewPending {
		t.Fatal("fixture must keep the legacy asset review field pending")
	}
	adapter, err := NewApplicationMedia(&applicationBindingStub{}, repository, &proofSignerStub{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.ResolveAdminProofs(context.Background(), proofAppID1, []string{proofAssetID1}); err != nil {
		t.Fatalf("approved review projection must be authoritative: %v", err)
	}
}

func TestResolveAdminProofsRejectsCrossApplicationWrongOrderAndDuplicates(t *testing.T) {
	tests := []struct {
		name   string
		appID  string
		assets []string
	}{
		{name: "cross application", appID: proofAppID2, assets: []string{proofAssetID1}},
		{name: "wrong slot order", appID: proofAppID1, assets: []string{proofAssetID2, proofAssetID1}},
		{name: "duplicate", appID: proofAppID1, assets: []string{proofAssetID1, proofAssetID1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := readyProofRepository(proofAppID1, proofAssetID1, proofAssetID2)
			signer := &proofSignerStub{}
			adapter, err := NewApplicationMedia(&applicationBindingStub{}, repository, signer, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := adapter.ResolveAdminProofs(context.Background(), test.appID, test.assets); !errors.Is(err, ErrProofMediaUnavailable) {
				t.Fatalf("err=%v", err)
			}
			if len(signer.calls) != 0 {
				t.Fatalf("invalid proof set reached signer: %+v", signer.calls)
			}
		})
	}
}

func TestResolveAdminProofsRejectsUnsafeAssetOrReferenceState(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name   string
		mutate func(*coremedia.Asset, *coremedia.GatewaySnapshot, *coremedia.AssetReference)
	}{
		{name: "wrong purpose", mutate: func(a *coremedia.Asset, _ *coremedia.GatewaySnapshot, _ *coremedia.AssetReference) {
			a.Purpose = coremedia.PurposePost
		}},
		{name: "projection not approved", mutate: func(_ *coremedia.Asset, s *coremedia.GatewaySnapshot, _ *coremedia.AssetReference) {
			s.ProjectionStatus = coremedia.ProjectionPending
		}},
		{name: "snapshot wrong purpose", mutate: func(_ *coremedia.Asset, s *coremedia.GatewaySnapshot, _ *coremedia.AssetReference) {
			s.Purpose = coremedia.PurposePost
		}},
		{name: "snapshot not final", mutate: func(_ *coremedia.Asset, s *coremedia.GatewaySnapshot, _ *coremedia.AssetReference) {
			s.StorageStage = coremedia.StorageStageStaging
		}},
		{name: "snapshot not technically ready", mutate: func(_ *coremedia.Asset, s *coremedia.GatewaySnapshot, _ *coremedia.AssetReference) {
			s.TechnicalStatus = coremedia.TechnicalUploaded
		}},
		{name: "snapshot missing final identity", mutate: func(_ *coremedia.Asset, s *coremedia.GatewaySnapshot, _ *coremedia.AssetReference) {
			s.FinalIdentityReady = false
		}},
		{name: "snapshot deleted", mutate: func(_ *coremedia.Asset, s *coremedia.GatewaySnapshot, _ *coremedia.AssetReference) {
			s.DeleteRequested = true
		}},
		{name: "not final", mutate: func(a *coremedia.Asset, _ *coremedia.GatewaySnapshot, _ *coremedia.AssetReference) {
			a.StorageStage = coremedia.StorageStageStaging
		}},
		{name: "not technically ready", mutate: func(a *coremedia.Asset, _ *coremedia.GatewaySnapshot, _ *coremedia.AssetReference) {
			a.TechnicalStatus = coremedia.TechnicalUploaded
		}},
		{name: "missing final key", mutate: func(a *coremedia.Asset, _ *coremedia.GatewaySnapshot, _ *coremedia.AssetReference) { a.FinalKey = nil }},
		{name: "missing final version", mutate: func(a *coremedia.Asset, _ *coremedia.GatewaySnapshot, _ *coremedia.AssetReference) {
			a.FinalVersionID = nil
		}},
		{name: "delete requested", mutate: func(a *coremedia.Asset, _ *coremedia.GatewaySnapshot, _ *coremedia.AssetReference) {
			a.DeleteRequestedAt = &now
		}},
		{name: "deleted", mutate: func(a *coremedia.Asset, _ *coremedia.GatewaySnapshot, _ *coremedia.AssetReference) {
			a.DeletedAt = &now
		}},
		{name: "revoked reference", mutate: func(_ *coremedia.Asset, _ *coremedia.GatewaySnapshot, r *coremedia.AssetReference) {
			r.Status = coremedia.ReferenceRevoked
		}},
		{name: "visible reference", mutate: func(_ *coremedia.Asset, _ *coremedia.GatewaySnapshot, r *coremedia.AssetReference) {
			r.ParentVisibility = coremedia.ParentVisible
		}},
		{name: "revocation fenced", mutate: func(_ *coremedia.Asset, _ *coremedia.GatewaySnapshot, r *coremedia.AssetReference) {
			value := proofAssetID2
			r.RevocationFence = &value
		}},
		{name: "wrong reference type", mutate: func(_ *coremedia.Asset, _ *coremedia.GatewaySnapshot, r *coremedia.AssetReference) {
			r.ReferenceType = coremedia.PurposePost
		}},
		{name: "wrong reference id", mutate: func(_ *coremedia.Asset, _ *coremedia.GatewaySnapshot, r *coremedia.AssetReference) {
			r.ReferenceID = proofAppID2
		}},
		{name: "wrong reference asset", mutate: func(_ *coremedia.Asset, _ *coremedia.GatewaySnapshot, r *coremedia.AssetReference) { r.AssetID++ }},
		{name: "wrong reference slot", mutate: func(_ *coremedia.Asset, _ *coremedia.GatewaySnapshot, r *coremedia.AssetReference) { r.Slot++ }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := readyProofRepository(proofAppID1, proofAssetID1)
			asset := repository.assets[proofAssetID1]
			snapshot := repository.snapshots[proofAssetID1][0]
			key := proofReferenceKey(asset.ID, coremedia.PurposeIdentityApplication, proofAppID1, 0)
			reference := repository.references[key]
			test.mutate(&asset, &snapshot, &reference)
			repository.assets[proofAssetID1] = asset
			repository.snapshots[proofAssetID1] = []coremedia.GatewaySnapshot{snapshot}
			repository.references[key] = reference
			signer := &proofSignerStub{}
			adapter, err := NewApplicationMedia(&applicationBindingStub{}, repository, signer, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := adapter.ResolveAdminProofs(context.Background(), proofAppID1, []string{proofAssetID1}); !errors.Is(err, ErrProofMediaUnavailable) {
				t.Fatalf("err=%v", err)
			}
			if len(signer.calls) != 0 {
				t.Fatalf("unsafe proof reached signer: %+v", signer.calls)
			}
		})
	}
}

func readyProofRepository(applicationID string, assetIDs ...string) *proofRepositoryStub {
	repository := &proofRepositoryStub{
		assets:     make(map[string]coremedia.Asset, len(assetIDs)),
		snapshots:  make(map[string][]coremedia.GatewaySnapshot, len(assetIDs)),
		references: make(map[string]coremedia.AssetReference, len(assetIDs)),
	}
	for index, assetID := range assetIDs {
		finalKey, finalVersion := "final/"+assetID+".webp", "version-"+assetID
		asset := coremedia.Asset{
			ID: uint64(index + 71), AssetPublicID: assetID, OwnerID: 41, Purpose: coremedia.PurposeIdentityApplication,
			StorageStage: coremedia.StorageStageFinal, ReviewStatus: coremedia.ReviewPending,
			TechnicalStatus: coremedia.TechnicalFinalReady, FinalKey: &finalKey, FinalVersionID: &finalVersion,
		}
		snapshot := coremedia.GatewaySnapshot{
			AssetPublicID: assetID, Purpose: coremedia.PurposeIdentityApplication,
			StorageStage: coremedia.StorageStageFinal, TechnicalStatus: coremedia.TechnicalFinalReady,
			FinalIdentityReady: true, ProjectionStatus: coremedia.ProjectionApproved,
		}
		reference := coremedia.AssetReference{
			ID: uint64(index + 171), AssetID: asset.ID, ReferenceType: coremedia.PurposeIdentityApplication,
			ReferenceID: applicationID, Slot: uint16(index), Status: coremedia.ReferencePending,
			ParentVisibility: coremedia.ParentHidden,
		}
		repository.assets[assetID] = asset
		repository.snapshots[assetID] = []coremedia.GatewaySnapshot{snapshot}
		repository.references[proofReferenceKey(asset.ID, reference.ReferenceType, reference.ReferenceID, reference.Slot)] = reference
	}
	return repository
}

func proofReferenceKey(assetID uint64, kind coremedia.Purpose, parent string, slot uint16) string {
	return string(kind) + ":" + parent + ":" + time.Unix(int64(assetID), int64(slot)).UTC().Format(time.RFC3339Nano)
}
