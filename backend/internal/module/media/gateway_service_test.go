package media

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type gatewayRepositoryStub struct {
	assets     []GatewaySnapshot
	references []GatewaySnapshot
	err        error
}

func (s *gatewayRepositoryStub) FindGatewayAssetSnapshots(context.Context, string) ([]GatewaySnapshot, error) {
	return s.assets, s.err
}

func (s *gatewayRepositoryStub) FindGatewayReferenceSnapshots(context.Context, Purpose, string) ([]GatewaySnapshot, error) {
	return s.references, s.err
}

func visibleGatewaySnapshot() GatewaySnapshot {
	now := time.Date(2026, 6, 21, 19, 0, 0, 0, time.UTC)
	return GatewaySnapshot{
		AssetPublicID: "01J00000000000000000000000", Purpose: PurposePost,
		StorageStage: StorageStageFinal, TechnicalStatus: TechnicalFinalReady,
		VerifiedMIME: "image/webp", ByteSize: 1024, Width: 800, Height: 600,
		FinalIdentityReady: true, ProjectionStatus: ProjectionApproved,
		AppliedDecisionVersion: 3, ProjectionDecidedAt: &now, ProjectionUpdatedAt: &now,
		HasReference: true, ReferenceType: PurposePost, ReferenceID: "post-public-1", ReferenceSlot: 1,
		ReferenceStatus: ReferenceActive, ParentVisibility: ParentVisible,
	}
}

func TestMediaGatewayVisibilityFailClosedMatrix(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*GatewaySnapshot)
		reason VisibilityReason
	}{
		{name: "visible", reason: VisibilityAllowed},
		{name: "not final", mutate: func(v *GatewaySnapshot) { v.TechnicalStatus = TechnicalPromoting }, reason: VisibilityNotReady},
		{name: "missing fixed identity", mutate: func(v *GatewaySnapshot) { v.FinalIdentityReady = false }, reason: VisibilityNotReady},
		{name: "delete requested", mutate: func(v *GatewaySnapshot) { v.DeleteRequested = true }, reason: VisibilityNotReady},
		{name: "not approved", mutate: func(v *GatewaySnapshot) { v.ProjectionStatus = ProjectionReviewing }, reason: VisibilityNotApproved},
		{name: "reference inactive", mutate: func(v *GatewaySnapshot) { v.ReferenceStatus = ReferenceRevoked }, reason: VisibilityReferenceInactive},
		{name: "parent hidden", mutate: func(v *GatewaySnapshot) { v.ParentVisibility = ParentHidden }, reason: VisibilityParentHidden},
		{name: "revocation fence", mutate: func(v *GatewaySnapshot) { v.RevocationFence = "01J00000000000000000000001" }, reason: VisibilityRevoked},
		{name: "purpose mismatch", mutate: func(v *GatewaySnapshot) { v.ReferenceType = PurposeProduct }, reason: VisibilityPurposeMismatch},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := visibleGatewaySnapshot()
			if test.mutate != nil {
				test.mutate(&value)
			}
			if got := evaluateVisibility(value); got != test.reason {
				t.Fatalf("got %s want %s", got, test.reason)
			}
		})
	}
}

func TestMediaGatewayBuildsOnlyApprovedViews(t *testing.T) {
	visible := visibleGatewaySnapshot()
	hidden := visible
	hidden.ReferenceSlot = 2
	hidden.ParentVisibility = ParentHidden
	repository := &gatewayRepositoryStub{assets: []GatewaySnapshot{visible, hidden}, references: []GatewaySnapshot{visible, hidden}}
	gateway, err := NewMediaGateway(repository)
	if err != nil {
		t.Fatal(err)
	}

	visibility, err := gateway.CheckVisibility(context.Background(), visible.AssetPublicID)
	if err != nil || !visibility.Visible || visibility.Reason != VisibilityAllowed {
		t.Fatalf("visibility=%#v err=%v", visibility, err)
	}
	metadata, err := gateway.GetVisibleMetadata(context.Background(), visible.AssetPublicID)
	if err != nil || metadata.MediaID != visible.AssetPublicID || metadata.ReviewStatus != ProjectionApproved {
		t.Fatalf("metadata=%#v err=%v", metadata, err)
	}
	view, err := gateway.BuildDisplayView(context.Background(), PurposePost, visible.ReferenceID, visible.ReferenceSlot)
	if err != nil || view.Media.MediaID != visible.AssetPublicID || view.Reference.ReferenceID != visible.ReferenceID {
		t.Fatalf("display=%#v err=%v", view, err)
	}
	if _, err := gateway.BuildDisplayView(context.Background(), PurposePost, hidden.ReferenceID, hidden.ReferenceSlot); !errors.Is(err, ErrMediaNotVisible) {
		t.Fatalf("hidden reference returned %v", err)
	}
	views, err := gateway.ListDisplayViews(context.Background(), PurposePost, visible.ReferenceID)
	if err != nil || len(views) != 1 || views[0].Reference.Slot != visible.ReferenceSlot {
		t.Fatalf("views=%#v err=%v", views, err)
	}
}

func TestMediaGatewayReviewProjectionAndDTOSecrecy(t *testing.T) {
	value := visibleGatewaySnapshot()
	value.ProjectionStatus = ProjectionReviewing
	value.AppliedDecisionVersion = 0
	repository := &gatewayRepositoryStub{assets: []GatewaySnapshot{value}}
	gateway, _ := NewMediaGateway(repository)
	projection, err := gateway.GetReviewProjection(context.Background(), value.AssetPublicID)
	if err != nil || projection.Status != ProjectionReviewing || projection.MediaID != value.AssetPublicID {
		t.Fatalf("projection=%#v err=%v", projection, err)
	}
	if _, err := gateway.GetVisibleMetadata(context.Background(), value.AssetPublicID); !errors.Is(err, ErrMediaNotVisible) {
		t.Fatalf("unapproved metadata returned %v", err)
	}

	raw, err := json.Marshal(MediaDisplayView{
		Media:     MediaMetadataView{MediaID: value.AssetPublicID, Purpose: PurposePost, MIMEType: "image/webp", ReviewStatus: ProjectionApproved},
		Reference: MediaReferenceView{ReferenceType: PurposePost, ReferenceID: "post-public-1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	serialized := strings.ToLower(string(raw))
	for _, forbidden := range []string{"staging", "bucket", "objectkey", "finalkey", "versionid", "secret", "ticket", "credential"} {
		if strings.Contains(serialized, forbidden) {
			t.Fatalf("DTO leaked forbidden field %q: %s", forbidden, serialized)
		}
	}
}
