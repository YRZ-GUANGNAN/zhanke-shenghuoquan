package media

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"path"
	"strings"
	"time"

	_ "golang.org/x/image/webp"
)

type CompletionServiceConfig struct {
	Environment   string
	LeaseTTL      time.Duration
	ProcessingTTL time.Duration
	Now           func() time.Time
}

type CompletionService struct {
	repository CompletionRepository
	storage    CompletionObjectStore
	config     CompletionServiceConfig
}

func NewCompletionService(repository CompletionRepository, storage CompletionObjectStore, config CompletionServiceConfig) (*CompletionService, error) {
	if repository == nil || storage == nil {
		return nil, ErrInvalidRecord
	}
	config.Environment = strings.ToLower(strings.TrimSpace(config.Environment))
	if config.Environment == "" || strings.ContainsAny(config.Environment, "/\\.") {
		return nil, ErrInvalidRecord
	}
	if config.LeaseTTL <= 0 {
		config.LeaseTTL = 5 * time.Minute
	}
	if config.ProcessingTTL <= 0 {
		config.ProcessingTTL = time.Hour
	}
	if config.Now == nil {
		config.Now = func() time.Time { return time.Now().UTC() }
	}
	return &CompletionService{repository: repository, storage: storage, config: config}, nil
}

// CompleteUpload runs only the Module 4 technical pipeline. It does not create
// HTTP routes, references, review tasks, access URLs, or garbage-collection work.
func (s *CompletionService) CompleteUpload(ctx context.Context, ownerID uint64, ticketID string) (Asset, error) {
	if ownerID == 0 || !ValidULID(ticketID) {
		return Asset{}, ErrInvalidRecord
	}
	now := s.config.Now().UTC()
	work, err := s.repository.ClaimCompletion(ctx, ownerID, ticketID, now, s.config.LeaseTTL, s.config.ProcessingTTL)
	if err != nil {
		return Asset{}, err
	}
	if !work.ShouldProcess {
		return work.Bundle.Asset, nil
	}

	if work.Completion.Phase == CompletionValidating {
		validation, pinnedWork, err := s.validateStaging(ctx, work)
		if err != nil {
			if errors.Is(err, ErrCompletionValidation) || errors.Is(err, ErrObjectNotFound) {
				failed, failErr := s.repository.FailCompletionValidation(ctx, pinnedWorkOr(work, pinnedWork), completionErrorCode(err), s.config.Now().UTC())
				if failErr != nil {
					return Asset{}, errors.Join(err, failErr)
				}
				return failed.Asset, err
			}
			if retryErr := s.repository.RecordCompletionRetry(ctx, work.Completion.TicketID, work.LeaseToken, CompletionValidating, completionErrorCode(err), s.config.Now().UTC()); retryErr != nil {
				return Asset{}, errors.Join(err, retryErr)
			}
			return Asset{}, err
		}
		work, err = s.repository.MarkCompletionPromoting(ctx, pinnedWork, validation, s.config.Now().UTC())
		if err != nil {
			return Asset{}, err
		}
	}

	final, err := s.ensurePromoted(ctx, work)
	if err != nil {
		if retryErr := s.repository.RecordCompletionRetry(ctx, work.Completion.TicketID, work.LeaseToken, CompletionPromoting, completionErrorCode(err), s.config.Now().UTC()); retryErr != nil {
			return Asset{}, errors.Join(err, retryErr)
		}
		return Asset{}, err
	}
	bundle, err := s.repository.FinalizeCompletion(ctx, work, final, s.config.Now().UTC())
	if err != nil {
		// The deterministic final object is intentionally retained. A later call
		// reconciles it before attempting another CopyObject.
		_ = s.repository.RecordCompletionRetry(ctx, work.Completion.TicketID, work.LeaseToken, CompletionPromoting, "finalize_cas", s.config.Now().UTC())
		return Asset{}, err
	}
	return bundle.Asset, nil
}

func (s *CompletionService) validateStaging(ctx context.Context, work CompletionWork) (CompletionValidation, CompletionWork, error) {
	if work.Bundle.Asset.StagingKey == nil || *work.Bundle.Asset.StagingKey == "" {
		return CompletionValidation{}, work, ErrCompletionInvariant
	}
	key := *work.Bundle.Asset.StagingKey
	var head CompletionObject
	var err error
	if work.Completion.StagingVersionID != nil {
		head, err = s.storage.HeadObject(ctx, key, *work.Completion.StagingVersionID)
	} else {
		head, err = s.storage.HeadObject(ctx, key, "")
		if err == nil {
			// All subsequent reads and CopyObject operations use this exact version.
			head, err = s.storage.HeadObject(ctx, key, head.VersionID)
		}
	}
	if err != nil {
		return CompletionValidation{}, work, err
	}
	if err := validateCompletionHead(work.Bundle.Ticket, head); err != nil {
		return CompletionValidation{}, work, err
	}
	pinned, err := s.repository.PinCompletionSource(ctx, work, head, s.config.Now().UTC())
	if err != nil {
		return CompletionValidation{}, work, err
	}
	body, err := s.storage.OpenObject(ctx, key, head.VersionID)
	if err != nil {
		return CompletionValidation{}, pinned, err
	}
	defer body.Close()
	raw, err := io.ReadAll(io.LimitReader(body, int64(work.Bundle.Ticket.ExpectedSize)+1))
	if err != nil {
		return CompletionValidation{}, pinned, err
	}
	validation, err := validateCompletionBytes(work.Bundle.Asset, work.Bundle.Ticket, head, raw, s.config.Environment)
	return validation, pinned, err
}

func validateCompletionHead(ticket UploadTicket, object CompletionObject) error {
	contentType, _, err := mime.ParseMediaType(strings.ToLower(strings.TrimSpace(object.ContentType)))
	if err != nil || object.Size != ticket.ExpectedSize || contentType != ticket.ExpectedContentType || object.VersionID == "" || object.ETag == "" || object.LastModified.IsZero() {
		return fmt.Errorf("%w: head declaration mismatch", ErrCompletionValidation)
	}
	expected, err := expectedMD5Hex(ticket.ExpectedMD5)
	if err != nil || normalizeETag(object.ETag) != expected {
		return fmt.Errorf("%w: etag mismatch", ErrCompletionValidation)
	}
	return nil
}

func validateCompletionBytes(asset Asset, ticket UploadTicket, head CompletionObject, raw []byte, environment string) (CompletionValidation, error) {
	if uint64(len(raw)) != ticket.ExpectedSize {
		return CompletionValidation{}, fmt.Errorf("%w: downloaded size mismatch", ErrCompletionValidation)
	}
	digest := md5.Sum(raw)
	expected, err := expectedMD5Hex(ticket.ExpectedMD5)
	if err != nil || hex.EncodeToString(digest[:]) != expected {
		return CompletionValidation{}, fmt.Errorf("%w: downloaded md5 mismatch", ErrCompletionValidation)
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return CompletionValidation{}, fmt.Errorf("%w: decode config", ErrCompletionValidation)
	}
	verifiedMIME, extension, err := strictImageContainer(raw, format)
	if err != nil || verifiedMIME != ticket.ExpectedContentType {
		return CompletionValidation{}, fmt.Errorf("%w: real format mismatch", ErrCompletionValidation)
	}
	maxEdge, maxPixels := uint64(8192), uint64(40_000_000)
	if asset.Purpose == PurposeAvatar {
		maxEdge, maxPixels = 4096, 16_000_000
	}
	if config.Width <= 0 || config.Height <= 0 || uint64(config.Width) > maxEdge || uint64(config.Height) > maxEdge || uint64(config.Width)*uint64(config.Height) > maxPixels {
		return CompletionValidation{}, fmt.Errorf("%w: image dimensions", ErrCompletionValidation)
	}
	if _, decodedFormat, err := image.Decode(bytes.NewReader(raw)); err != nil || decodedFormat != format {
		return CompletionValidation{}, fmt.Errorf("%w: full decode", ErrCompletionValidation)
	}
	trusted := sha256.Sum256(raw)
	trustedHex := hex.EncodeToString(trusted[:])
	created := asset.CreatedAt.UTC()
	if created.IsZero() {
		return CompletionValidation{}, ErrCompletionInvariant
	}
	finalKey := path.Join("final", environment, string(asset.Purpose), created.Format("2006"), created.Format("01"), asset.AssetPublicID, trustedHex+"."+extension)
	return CompletionValidation{
		StagingVersionID: head.VersionID, StagingETag: normalizeETag(head.ETag), StagingLastModified: head.LastModified,
		TrustedSHA256: trustedHex, VerifiedMIME: verifiedMIME, VerifiedSize: uint64(len(raw)),
		Width: uint32(config.Width), Height: uint32(config.Height), FinalKey: finalKey,
	}, nil
}

func strictImageContainer(raw []byte, format string) (string, string, error) {
	switch format {
	case "jpeg":
		if len(raw) < 4 || raw[0] != 0xff || raw[1] != 0xd8 || raw[len(raw)-2] != 0xff || raw[len(raw)-1] != 0xd9 {
			return "", "", ErrCompletionValidation
		}
		return "image/jpeg", "jpg", nil
	case "png":
		if !validExactPNG(raw) {
			return "", "", ErrCompletionValidation
		}
		return "image/png", "png", nil
	case "webp":
		if !validStaticWebP(raw) {
			return "", "", ErrCompletionValidation
		}
		return "image/webp", "webp", nil
	default:
		return "", "", ErrCompletionValidation
	}
}

func validExactPNG(raw []byte) bool {
	signature := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	if len(raw) < len(signature)+12 || !bytes.Equal(raw[:8], signature) {
		return false
	}
	offset := 8
	for offset+12 <= len(raw) {
		length := int(binary.BigEndian.Uint32(raw[offset : offset+4]))
		if length < 0 || offset+12+length > len(raw) {
			return false
		}
		chunkType := string(raw[offset+4 : offset+8])
		offset += 12 + length
		if chunkType == "IEND" {
			return length == 0 && offset == len(raw)
		}
	}
	return false
}

func validStaticWebP(raw []byte) bool {
	if len(raw) < 12 || string(raw[:4]) != "RIFF" || string(raw[8:12]) != "WEBP" || int(binary.LittleEndian.Uint32(raw[4:8]))+8 != len(raw) {
		return false
	}
	for offset := 12; offset+8 <= len(raw); {
		chunkType := string(raw[offset : offset+4])
		length := int(binary.LittleEndian.Uint32(raw[offset+4 : offset+8]))
		offset += 8
		if length < 0 || offset+length > len(raw) || chunkType == "ANIM" || chunkType == "ANMF" {
			return false
		}
		offset += length
		if length%2 == 1 {
			offset++
		}
		if offset == len(raw) {
			return true
		}
	}
	return false
}

func (s *CompletionService) ensurePromoted(ctx context.Context, work CompletionWork) (CompletionObject, error) {
	c := work.Completion
	if c.FinalKey == nil || c.StagingVersionID == nil || c.StagingETag == nil || c.TrustedSHA256 == nil || c.VerifiedMIME == nil || c.VerifiedSize == nil {
		return CompletionObject{}, ErrCompletionInvariant
	}
	if existing, err := s.storage.HeadObject(ctx, *c.FinalKey, ""); err == nil {
		if err := verifyFinalObject(existing, work); err != nil {
			return CompletionObject{}, err
		}
		return existing, nil
	} else if !errors.Is(err, ErrObjectNotFound) {
		return CompletionObject{}, err
	}
	final, err := s.storage.PromoteObject(ctx, PromoteObjectRequest{
		SourceKey: *work.Bundle.Asset.StagingKey, SourceVersionID: *c.StagingVersionID, SourceETag: *c.StagingETag,
		DestinationKey: *c.FinalKey, AssetPublicID: work.Bundle.Asset.AssetPublicID, TicketID: c.TicketID,
		TrustedSHA256: *c.TrustedSHA256, VerifiedMIME: *c.VerifiedMIME, VerifiedSize: *c.VerifiedSize,
	})
	if errors.Is(err, ErrObjectAlreadyExists) {
		final, err = s.storage.HeadObject(ctx, *c.FinalKey, "")
	}
	if err != nil {
		return CompletionObject{}, err
	}
	if err := verifyFinalObject(final, work); err != nil {
		return CompletionObject{}, err
	}
	return final, nil
}

func verifyFinalObject(object CompletionObject, work CompletionWork) error {
	c := work.Completion
	if c.FinalKey == nil || c.VerifiedSize == nil || c.VerifiedMIME == nil || c.TrustedSHA256 == nil || c.StagingVersionID == nil || c.StagingETag == nil ||
		object.Key != *c.FinalKey || object.VersionID == "" || object.Size != *c.VerifiedSize || object.ContentType != *c.VerifiedMIME ||
		normalizeETag(object.ETag) != normalizeETag(*c.StagingETag) ||
		object.Metadata["media-asset-id"] != work.Bundle.Asset.AssetPublicID || object.Metadata["media-ticket-id"] != c.TicketID ||
		object.Metadata["media-sha256"] != *c.TrustedSHA256 || object.Metadata["media-source-version"] != *c.StagingVersionID {
		return fmt.Errorf("%w: final object identity mismatch", ErrCompletionInvariant)
	}
	return nil
}

func expectedMD5Hex(value string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) != md5.Size {
		return "", ErrInvalidUploadDeclaration
	}
	return hex.EncodeToString(decoded), nil
}

func completionErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrObjectNotFound):
		return "staging_not_found"
	case errors.Is(err, ErrCompletionValidation):
		return "validation_failed"
	case errors.Is(err, ErrObjectAlreadyExists):
		return "final_exists_conflict"
	case errors.Is(err, ErrConcurrentUpdate):
		return "cas_conflict"
	default:
		return "transient_storage_error"
	}
}

func pinnedWorkOr(original, pinned CompletionWork) CompletionWork {
	if pinned.Completion.TicketID != "" {
		return pinned
	}
	return original
}
