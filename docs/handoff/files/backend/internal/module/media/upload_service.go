package media

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type UploadAuthorizer interface {
	IssueSinglePUT(context.Context, AuthorizationRequest) (UploadAuthorization, error)
}

type UploadTicketServiceConfig struct {
	Environment      string
	TicketTTL        time.Duration
	AuthorizationTTL time.Duration
	Quota            QuotaPolicy
	Now              func() time.Time
}

type UploadTicketService struct {
	repository UploadTicketRepository
	authorizer UploadAuthorizer
	config     UploadTicketServiceConfig
}

func NewUploadTicketService(repository UploadTicketRepository, authorizer UploadAuthorizer, config UploadTicketServiceConfig) (*UploadTicketService, error) {
	if repository == nil || authorizer == nil {
		return nil, ErrInvalidRecord
	}
	config.Environment = strings.ToLower(strings.TrimSpace(config.Environment))
	if config.Environment == "" || strings.ContainsAny(config.Environment, "/\\.") {
		return nil, ErrInvalidRecord
	}
	if config.TicketTTL <= 0 {
		config.TicketTTL = 30 * time.Minute
	}
	if config.AuthorizationTTL <= 0 || config.AuthorizationTTL > 2*time.Minute {
		config.AuthorizationTTL = 2 * time.Minute
	}
	if !config.Quota.valid() {
		config.Quota = DefaultQuotaPolicy()
	}
	if config.Now == nil {
		config.Now = func() time.Time { return time.Now().UTC() }
	}
	return &UploadTicketService{repository: repository, authorizer: authorizer, config: config}, nil
}

func (s *UploadTicketService) CreateTicket(ctx context.Context, input CreateUploadTicketInput) (UploadBundle, bool, error) {
	if err := validateUploadDeclaration(input); err != nil {
		return UploadBundle{}, false, err
	}
	now := s.config.Now().UTC()
	assetPublicID, err := NewULID()
	if err != nil {
		return UploadBundle{}, false, err
	}
	ticketID, err := NewULID()
	if err != nil {
		return UploadBundle{}, false, err
	}
	reservationID, err := NewULID()
	if err != nil {
		return UploadBundle{}, false, err
	}
	stagingKey, err := newStagingKey(s.config.Environment, input.OwnerID, assetPublicID, input.ExpectedContentType)
	if err != nil {
		return UploadBundle{}, false, err
	}
	expiresAt := now.Add(s.config.TicketTTL)
	bundle := UploadBundle{
		Asset: Asset{
			AssetPublicID: assetPublicID, OwnerID: input.OwnerID, Purpose: input.Purpose,
			StorageStage: StorageStageStaging, ReviewStatus: ReviewPending,
			TechnicalStatus: TechnicalPendingUpload, ProjectedDecision: ReviewPending,
			StagingKey: &stagingKey, IdempotencyKey: input.IdempotencyKey,
			UploadExpiresAt: &expiresAt, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
		},
		Ticket: UploadTicket{
			TicketID: ticketID, OwnerID: input.OwnerID, ExpectedSize: input.ExpectedSize,
			ExpectedMD5: input.ExpectedMD5, ExpectedContentType: input.ExpectedContentType,
			Status: TicketCreated, ExpiresAt: expiresAt, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
		},
		Reservation: QuotaReservation{
			ReservationID: reservationID, OwnerID: input.OwnerID, BytesReserved: input.ExpectedSize,
			CountReserved: 1, Status: ReservationActive, TicketID: ticketID,
			RowVersion: 1, CreatedAt: now, UpdatedAt: now,
		},
	}
	if err := bundle.Asset.ValidateNew(); err != nil {
		return UploadBundle{}, false, err
	}
	return s.repository.CreateUploadBundle(ctx, CreateUploadBundleParams{Bundle: bundle, Policy: s.config.Quota, Now: now})
}

func (s *UploadTicketService) IssueAuthorization(ctx context.Context, ownerID uint64, ticketID string, expectedVersion uint64) (UploadAuthorization, error) {
	if ownerID == 0 || !ValidULID(ticketID) || expectedVersion == 0 {
		return UploadAuthorization{}, ErrInvalidRecord
	}
	now := s.config.Now().UTC()
	bundle, err := s.repository.AuthorizeUploadTicket(ctx, ticketID, ownerID, expectedVersion, now)
	if err != nil {
		return UploadAuthorization{}, err
	}
	remaining := bundle.Ticket.ExpiresAt.Sub(now)
	ttl := min(s.config.AuthorizationTTL, remaining)
	if ttl <= 0 {
		_, _ = s.repository.CancelUploadTicket(ctx, ticketID, ownerID, bundle.Ticket.RowVersion, now)
		return UploadAuthorization{}, ErrTicketExpired
	}
	authorization, err := s.authorizer.IssueSinglePUT(ctx, AuthorizationRequest{
		TicketID: ticketID, ObjectKey: *bundle.Asset.StagingKey, Size: bundle.Ticket.ExpectedSize,
		MD5: bundle.Ticket.ExpectedMD5, ContentType: bundle.Ticket.ExpectedContentType, TTL: ttl,
	})
	if err != nil {
		_, cancelErr := s.repository.CancelUploadTicket(ctx, ticketID, ownerID, bundle.Ticket.RowVersion, now)
		return UploadAuthorization{}, errors.Join(err, cancelErr)
	}
	authorization.TicketID = ticketID
	authorization.AssetPublicID = bundle.Asset.AssetPublicID
	return authorization, nil
}

func (s *UploadTicketService) MarkUploaded(ctx context.Context, ownerID uint64, ticketID string, expectedVersion uint64) (UploadBundle, error) {
	if ownerID == 0 || !ValidULID(ticketID) || expectedVersion == 0 {
		return UploadBundle{}, ErrInvalidRecord
	}
	return s.repository.MarkUploadTicketUploaded(ctx, ticketID, ownerID, expectedVersion, s.config.Now().UTC())
}

func (s *UploadTicketService) Consume(ctx context.Context, ownerID uint64, ticketID string, expectedVersion uint64) (UploadBundle, error) {
	if ownerID == 0 || !ValidULID(ticketID) || expectedVersion == 0 {
		return UploadBundle{}, ErrInvalidRecord
	}
	return s.repository.ConsumeUploadTicket(ctx, ticketID, ownerID, expectedVersion, s.config.Now().UTC())
}

func (s *UploadTicketService) Cancel(ctx context.Context, ownerID uint64, ticketID string, expectedVersion uint64) (UploadBundle, error) {
	if ownerID == 0 || !ValidULID(ticketID) || expectedVersion == 0 {
		return UploadBundle{}, ErrInvalidRecord
	}
	return s.repository.CancelUploadTicket(ctx, ticketID, ownerID, expectedVersion, s.config.Now().UTC())
}

func (s *UploadTicketService) Expire(ctx context.Context, limit int) (int, error) {
	return s.repository.ExpireUploadTickets(ctx, s.config.Now().UTC(), limit)
}

func validateUploadDeclaration(input CreateUploadTicketInput) error {
	if input.OwnerID == 0 || !input.Purpose.Valid() || input.ExpectedSize == 0 || len(input.IdempotencyKey) == 0 || len(input.IdempotencyKey) > 96 {
		return ErrInvalidUploadDeclaration
	}
	maxSize := uint64(10 << 20)
	if input.Purpose == PurposeAvatar {
		maxSize = 5 << 20
	}
	if input.ExpectedSize > maxSize {
		return ErrInvalidUploadDeclaration
	}
	decodedMD5, err := base64.StdEncoding.DecodeString(input.ExpectedMD5)
	if err != nil || len(decodedMD5) != 16 || base64.StdEncoding.EncodeToString(decodedMD5) != input.ExpectedMD5 {
		return ErrInvalidUploadDeclaration
	}
	switch input.ExpectedContentType {
	case "image/jpeg", "image/png", "image/webp":
		return nil
	default:
		return ErrInvalidUploadDeclaration
	}
}

func newStagingKey(environment string, ownerID uint64, assetPublicID, contentType string) (string, error) {
	extension := map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}[contentType]
	if extension == "" || !ValidULID(assetPublicID) || ownerID == 0 {
		return "", ErrInvalidUploadDeclaration
	}
	ownerDigest := sha256.Sum256([]byte("media-owner:" + strconv.FormatUint(ownerID, 10)))
	ownerToken := hex.EncodeToString(ownerDigest[:])
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	key := fmt.Sprintf("staging/%s/%s/%s/%s/%s.%s", environment, ownerToken[:4], ownerToken[:26], assetPublicID, hex.EncodeToString(nonce), extension)
	if strings.Contains(key, "..") || strings.HasPrefix(key, "/") {
		return "", ErrInvalidUploadDeclaration
	}
	return key, nil
}
