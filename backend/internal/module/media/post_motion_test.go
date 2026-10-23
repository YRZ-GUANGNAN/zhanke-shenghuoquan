package media

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strconv"
	"testing"
)

func motionBox(kind string, payload ...[]byte) []byte {
	value := make([]byte, 8)
	copy(value[4:], kind)
	for _, part := range payload {
		value = append(value, part...)
	}
	binary.BigEndian.PutUint32(value, uint32(len(value)))
	return value
}

// Structural fixture only; the production pipeline additionally requires COS
// to inspect the immutable object and confirm its codec, duration and size.
func motionMP4(duration uint32) []byte {
	movie, track, handler := make([]byte, 100), make([]byte, 84), make([]byte, 24)
	binary.BigEndian.PutUint32(movie[12:], 1000)
	binary.BigEndian.PutUint32(movie[16:], duration)
	binary.BigEndian.PutUint32(track[76:], 720<<16)
	binary.BigEndian.PutUint32(track[80:], 1280<<16)
	copy(handler[8:], "vide")
	return append(append(motionBox("ftyp", []byte("isom\x00\x00\x00\x00mp42")),
		motionBox("moov", motionBox("mvhd", movie), motionBox("trak", motionBox("tkhd", track), motionBox("mdia", motionBox("hdlr", handler))))...), motionBox("mdat", []byte{1, 2, 3, 4})...)
}

func TestPostMotionMP4StructureAndBounds(t *testing.T) {
	for _, duration := range []uint32{500, 5000, 60000} {
		value, err := inspectMP4(motionMP4(duration))
		if err != nil || value.Width != 720 || value.Height != 1280 || value.DurationMS != duration {
			t.Fatalf("duration %d: %+v %v", duration, value, err)
		}
	}
	for name, raw := range map[string][]byte{
		"empty": nil, "not-video": []byte("arbitrary uploaded data"), "truncated": motionMP4(1000)[:32],
		"too-long": motionMP4(60001), "zero-duration": motionMP4(0), "invalid-box": {0, 0, 0, 2, 'f', 't', 'y', 'p'},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := inspectMP4(raw); !errors.Is(err, ErrCompletionValidation) {
				t.Fatalf("accepted invalid media: %v", err)
			}
		})
	}
}

type motionStore struct {
	*completionStoreStub
	info                           VideoInfo
	inspectedKey, inspectedVersion string
}

func (s *motionStore) InspectVideo(_ context.Context, key, version string) (VideoInfo, error) {
	s.inspectedKey, s.inspectedVersion = key, version
	return s.info, nil
}
func (s *motionStore) PromoteObject(ctx context.Context, request PromoteObjectRequest) (CompletionObject, error) {
	value, err := s.completionStoreStub.PromoteObject(ctx, request)
	value.Metadata["media-duration-ms"] = strconv.FormatUint(uint64(request.VerifiedDurationMS), 10)
	return value, err
}

func TestPostMotionCompletionRequiresIndependentCodecInspection(t *testing.T) {
	for _, codec := range []string{"h264", "hevc", ""} {
		t.Run(codec, func(t *testing.T) {
			repo, store, service := newCompletionUnitFixture(t)
			raw := motionMP4(5000)
			digest := md5.Sum(raw)
			repo.work.Bundle.Ticket.ExpectedSize, repo.work.Bundle.Ticket.ExpectedContentType = uint64(len(raw)), "video/mp4"
			repo.work.Bundle.Ticket.ExpectedMD5 = base64.StdEncoding.EncodeToString(digest[:])
			store.stagingBytes = raw
			store.staging.Size, store.staging.ContentType, store.staging.ETag = uint64(len(raw)), "video/mp4", hexMD5(raw)
			videoStore := &motionStore{completionStoreStub: store, info: VideoInfo{Width: 1280, Height: 720, DurationMS: 5000, Codec: codec, Size: uint64(len(raw))}}
			service.storage = videoStore
			asset, err := service.CompleteUpload(context.Background(), 42, repo.work.Bundle.Ticket.TicketID)
			if codec != "h264" {
				if !errors.Is(err, ErrCompletionValidation) || store.promoteCalls != 0 {
					t.Fatalf("unsupported codec promoted: %v", err)
				}
				return
			}
			if err != nil || asset.DurationMS != 5000 || asset.Width != 1280 || asset.TechnicalStatus != TechnicalFinalReady {
				t.Fatalf("wrong trusted metadata: %+v %v", asset, err)
			}
			if videoStore.inspectedKey != store.staging.Key || videoStore.inspectedVersion != store.staging.VersionID {
				t.Fatal("inspected a different object version")
			}
			if _, err = service.CompleteUpload(context.Background(), 42, repo.work.Bundle.Ticket.TicketID); err != nil || store.promoteCalls != 1 {
				t.Fatalf("completion did not replay: %v", err)
			}
		})
	}
}

func TestPostVideoDeclarationCannotEnterOtherPurposes(t *testing.T) {
	input := CreateUploadTicketInput{OwnerID: 1, Purpose: PurposePost, ExpectedContentType: "video/mp4", ExpectedSize: 30 << 20, ExpectedMD5: "1B2M2Y8AsgTpgAmY7PhCfg==", IdempotencyKey: "video-upload"}
	if err := validateUploadDeclaration(input); err != nil {
		t.Fatal(err)
	}
	input.ExpectedSize++
	if err := validateUploadDeclaration(input); !errors.Is(err, ErrInvalidUploadDeclaration) {
		t.Fatal("oversized video accepted")
	}
	input.ExpectedSize = 100
	for _, purpose := range []Purpose{PurposeProduct, PurposeAvatar, PurposeProfileCover, PurposeDish, PurposeIdentityApplication} {
		input.Purpose = purpose
		if err := validateUploadDeclaration(input); !errors.Is(err, ErrInvalidUploadDeclaration) {
			t.Fatalf("video entered %s", purpose)
		}
	}
}
