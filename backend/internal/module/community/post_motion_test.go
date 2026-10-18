package community

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	coremedia "campus-circle/backend/internal/module/media"
	"campus-circle/backend/internal/pkg/media"
)

func TestPostMotionRejectsMixedAndUnverifiedAssets(t *testing.T) {
	input := CreatePostInput{Motion: &CreatePostMotionInput{Kind: "video", AssetID: "clip", PosterAssetID: "cover"}}
	refs := []media.Reference{{InternalID: 1, PublicID: "cover", URL: "/cover", VerifiedMIME: "image/jpeg"}, {InternalID: 2, PublicID: "clip", URL: "/clip", VerifiedMIME: "video/mp4", Width: 720, Height: 1280, DurationMS: 10000}}
	var post Post
	if err := applyPostMedia(&post, input, refs); err != nil || post.Motion.DurationMS != 10000 || len(post.Images) != 1 || post.Images[0].URL != "/cover" {
		t.Fatalf("motion binding failed: %+v %v", post, err)
	}
	input.ImageAssetIDs = []string{"other"}
	if _, err := postAssetIDs(input); !errors.Is(err, ErrInvalidPostMotion) {
		t.Fatal("mixed media accepted")
	}
	input.ImageAssetIDs = nil
	input.Motion.Kind = "live_photo"
	if err := applyPostMedia(&Post{}, input, refs); !errors.Is(err, ErrInvalidPostMotion) {
		t.Fatal("long live photo accepted")
	}
	input.Motion.Kind = "video"
	refs[1].VerifiedMIME = "image/jpeg"
	if err := applyPostMedia(&Post{}, input, refs); !errors.Is(err, ErrInvalidPostMotion) {
		t.Fatal("image disguised as video")
	}
	refs[1].VerifiedMIME = "video/mp4"
	if err := applyPostMedia(&Post{}, CreatePostInput{}, refs); !errors.Is(err, ErrInvalidPostMotion) {
		t.Fatal("video entered ordinary image post")
	}
}

func TestPostMotionKeyedCreateBindsBothAssetsAndReplays(t *testing.T) {
	cover, clip := "01J00000000000000000000001", "01J00000000000000000000002"
	core := &communityBindingStub{reserveResult: coremedia.BindingReservation{References: []coremedia.BindingAsset{
		{InternalID: 11, AssetPublicID: cover, Slot: 0, VerifiedMIME: "image/jpeg"},
		{InternalID: 12, AssetPublicID: clip, Slot: 1, VerifiedMIME: "video/mp4", Width: 720, Height: 1280, DurationMS: 4000},
	}}}
	gateway, err := NewHybridMediaGateway(core, &communityLegacyMediaStub{}, "https://media.example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	repo := newCommunityKeyedMemoryRepo()
	s := NewService(repo, profileDirectory{}, &submissionRecorder{}, gateway)
	input := CreatePostInput{UserID: 7, Title: "校园片段", Content: "测试动态帖子", Motion: &CreatePostMotionInput{Kind: "video", AssetID: clip, PosterAssetID: cover}}
	first, err := s.CreatePostKeyed(context.Background(), input, communityTestRequestID, "")
	if err != nil || first.Value.Motion == nil || first.Replayed {
		t.Fatalf("create: %+v %v", first, err)
	}
	var event communityCreationPayload
	if err = json.Unmarshal([]byte(repo.postEvent.Payload), &event); err != nil {
		t.Fatal(err)
	}
	if len(event.OrderedAssetPublicIDs) != 2 || event.OrderedAssetPublicIDs[0] != cover || event.OrderedAssetPublicIDs[1] != clip {
		t.Fatal("video missing from durable media binding")
	}
	replayed, err := s.CreatePostKeyed(context.Background(), input, communityTestRequestID, "")
	if err != nil || !replayed.Replayed || replayed.Value.ID != first.Value.ID || len(repo.posts) != 1 {
		t.Fatal("same publish created duplicate post")
	}
	input.Motion = &CreatePostMotionInput{Kind: "live_photo", AssetID: clip, PosterAssetID: cover}
	if _, err := s.CreatePostKeyed(context.Background(), input, communityTestRequestID, ""); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("media kind change did not conflict: %v", err)
	}
	core.reserveResult.References[1].VerifiedMIME = "image/jpeg"
	if _, err := s.CreatePostKeyed(context.Background(), input, "01J00000000000000000000003", ""); !errors.Is(err, ErrInvalidPostMotion) || core.cancelCalls != 1 || len(repo.posts) != 1 {
		t.Fatalf("invalid motion retained reservation: %v", err)
	}
}
