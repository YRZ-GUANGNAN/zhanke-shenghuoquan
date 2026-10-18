package community

import (
	"campus-circle/backend/internal/pkg/media"
	"errors"
	"strings"
)

var ErrInvalidPostMotion = errors.New("invalid post media: use images or one video/live-photo with an approved cover")

func postAssetIDs(input CreatePostInput) ([]string, error) {
	if input.Motion == nil {
		return normalizedAssetIDs(input.ImageAssetIDs), nil
	}
	m := input.Motion
	if len(input.ImageAssetIDs) != 0 || (m.Kind != "video" && m.Kind != "live_photo") ||
		strings.TrimSpace(m.AssetID) == "" || strings.TrimSpace(m.PosterAssetID) == "" || m.AssetID == m.PosterAssetID {
		return nil, ErrInvalidPostMotion
	}
	return []string{m.PosterAssetID, m.AssetID}, nil
}

func applyPostMedia(post *Post, input CreatePostInput, refs []media.Reference) error {
	if input.Motion != nil {
		m := input.Motion
		if len(refs) != 2 || refs[0].PublicID != m.PosterAssetID || refs[1].PublicID != m.AssetID ||
			!strings.HasPrefix(refs[0].VerifiedMIME, "image/") || refs[1].VerifiedMIME != "video/mp4" ||
			refs[1].DurationMS < 500 || refs[1].DurationMS > 60000 || refs[1].Width == 0 || refs[1].Height == 0 ||
			(m.Kind == "live_photo" && refs[1].DurationMS > 5000) {
			return ErrInvalidPostMotion
		}
		id := refs[0].InternalID
		post.Images = []PostImage{{AssetID: &id, URL: refs[0].URL, Sort: 0}}
		post.Motion = &PostMotion{Kind: m.Kind, URL: refs[1].URL, PosterURL: refs[0].URL,
			Width: refs[1].Width, Height: refs[1].Height, DurationMS: refs[1].DurationMS}
		return nil
	}
	for i, ref := range refs {
		// Older image-only gateways omit MIME. Verified videos must never enter images.
		if ref.VerifiedMIME != "" && !strings.HasPrefix(ref.VerifiedMIME, "image/") {
			return ErrInvalidPostMotion
		}
		id := ref.InternalID
		post.Images = append(post.Images, PostImage{AssetID: &id, URL: ref.URL, Sort: i})
	}
	return nil
}
