package miniappflow

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	coremedia "campus-circle/backend/internal/module/media"
	mysqldriver "github.com/go-sql-driver/mysql"
)

type ticketCommands interface {
	CreateTicket(context.Context, coremedia.CreateUploadTicketInput) (coremedia.UploadBundle, bool, error)
	IssueAuthorization(context.Context, uint64, string, uint64) (coremedia.UploadAuthorization, error)
	MarkUploaded(context.Context, uint64, string, uint64) (coremedia.UploadBundle, error)
}

type uploadBundleReader interface {
	FindUploadBundle(context.Context, string, uint64) (coremedia.UploadBundle, error)
}

type completionCommands interface {
	CompleteUpload(context.Context, uint64, string) (coremedia.Asset, error)
}

type assetCommands interface {
	TransitionTechnical(context.Context, uint64, uint64, coremedia.TechnicalStatus, coremedia.TechnicalStatus) error
}

type projectionCommands interface {
	EnsureProjection(context.Context, string) (coremedia.ReviewProjection, bool, error)
}

type outboxAppender interface {
	AppendOutbox(context.Context, *coremedia.OutboxEvent) error
}

type consumerCommands interface {
	ConsumeBatch(context.Context, int) (coremedia.ConsumerBatchResult, error)
}

type Service struct {
	tickets     ticketCommands
	bundles     uploadBundleReader
	completion  completionCommands
	assets      assetCommands
	projections projectionCommands
	outbox      outboxAppender
	consumer    consumerCommands
	status      StatusRepository
	now         func() time.Time
}

func NewService(
	tickets ticketCommands,
	bundles uploadBundleReader,
	completion completionCommands,
	assets assetCommands,
	projections projectionCommands,
	outbox outboxAppender,
	consumer consumerCommands,
	status StatusRepository,
	now func() time.Time,
) (*Service, error) {
	if tickets == nil || bundles == nil || completion == nil || assets == nil || projections == nil || outbox == nil || consumer == nil || status == nil {
		return nil, ErrInvalidRequest
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Service{
		tickets: tickets, bundles: bundles, completion: completion, assets: assets,
		projections: projections, outbox: outbox, consumer: consumer, status: status, now: now,
	}, nil
}

func (s *Service) CreateUpload(ctx context.Context, input CreateUploadInput) (UploadSession, error) {
	if input.OwnerID == 0 {
		return UploadSession{}, ErrInvalidRequest
	}
	bundle, _, err := s.tickets.CreateTicket(ctx, coremedia.CreateUploadTicketInput{
		OwnerID: input.OwnerID, Purpose: input.Purpose, ExpectedSize: input.SizeBytes,
		ExpectedMD5: input.ContentMD5, ExpectedContentType: input.ContentType,
		IdempotencyKey: input.IdempotencyKey,
	})
	if err != nil {
		return UploadSession{}, err
	}
	switch bundle.Ticket.Status {
	case coremedia.TicketCreated:
	case coremedia.TicketExpired:
		return UploadSession{}, ErrUploadExpired
	case coremedia.TicketCancelled:
		return UploadSession{}, ErrUploadCancelled
	default:
		return UploadSession{}, ErrAuthorizationConsumed
	}
	authorization, err := s.tickets.IssueAuthorization(ctx, input.OwnerID, bundle.Ticket.TicketID, bundle.Ticket.RowVersion)
	if err != nil {
		return UploadSession{}, err
	}
	return UploadSession{
		TicketID: authorization.TicketID, AssetID: authorization.AssetPublicID,
		UploadURL: authorization.UploadURL,
		Headers: RequiredHeaders{
			ContentType:     authorization.RequiredHeaders.ContentType,
			ContentMD5:      authorization.RequiredHeaders.ContentMD5,
			ContentLength:   authorization.RequiredHeaders.ContentLength,
			ForbidOverwrite: authorization.RequiredHeaders.ForbidOverwrite,
		},
		ExpiresAt: authorization.ExpiresAt.UTC(), State: StateWaitingUpload,
		Message: "上传授权已签发，请在有效期内完成上传", Recovery: RecoveryNewUpload,
	}, nil
}

func (s *Service) CompleteUpload(ctx context.Context, ownerID uint64, ticketID string) (AssetStatus, error) {
	if ownerID == 0 || !coremedia.ValidULID(ticketID) {
		return AssetStatus{}, ErrInvalidRequest
	}
	bundle, err := s.bundles.FindUploadBundle(ctx, ticketID, ownerID)
	if err != nil {
		return AssetStatus{}, err
	}
	switch bundle.Ticket.Status {
	case coremedia.TicketAuthorizationIssued:
		bundle, err = s.tickets.MarkUploaded(ctx, ownerID, ticketID, bundle.Ticket.RowVersion)
		if err != nil {
			return AssetStatus{}, err
		}
	case coremedia.TicketUploaded, coremedia.TicketConsumed:
	case coremedia.TicketCreated:
		return AssetStatus{}, ErrUploadNotStarted
	case coremedia.TicketExpired:
		return AssetStatus{}, ErrUploadExpired
	case coremedia.TicketCancelled:
		return AssetStatus{}, ErrUploadCancelled
	default:
		return AssetStatus{}, ErrInvalidRequest
	}

	if bundle.Asset.TechnicalStatus == coremedia.TechnicalPendingUpload {
		if err := s.assets.TransitionTechnical(ctx, bundle.Asset.ID, bundle.Asset.RowVersion, coremedia.TechnicalPendingUpload, coremedia.TechnicalUploaded); err != nil {
			return AssetStatus{}, err
		}
	} else if bundle.Asset.TechnicalStatus != coremedia.TechnicalUploaded &&
		bundle.Asset.TechnicalStatus != coremedia.TechnicalValidating &&
		bundle.Asset.TechnicalStatus != coremedia.TechnicalPromoting &&
		bundle.Asset.TechnicalStatus != coremedia.TechnicalFinalReady {
		return AssetStatus{}, coremedia.ErrCompletionInvariant
	}

	asset, err := s.completion.CompleteUpload(ctx, ownerID, ticketID)
	if err != nil {
		return AssetStatus{}, err
	}
	if asset.TechnicalStatus != coremedia.TechnicalFinalReady || asset.StorageStage != coremedia.StorageStageFinal {
		return AssetStatus{}, coremedia.ErrCompletionInvariant
	}
	projection, _, err := s.projections.EnsureProjection(ctx, asset.AssetPublicID)
	if err != nil {
		return AssetStatus{}, err
	}
	if projection.Status == coremedia.ProjectionPending {
		if err := s.appendReviewSubmission(ctx, projection, asset.ID); err != nil {
			return AssetStatus{}, err
		}
		result, consumeErr := s.consumer.ConsumeBatch(ctx, 100)
		if consumeErr != nil || result.Deadletters > 0 {
			return AssetStatus{}, errors.Join(ErrSubmissionPending, consumeErr)
		}
	}
	return s.Status(ctx, ownerID, asset.AssetPublicID)
}

func (s *Service) Status(ctx context.Context, ownerID uint64, assetPublicID string) (AssetStatus, error) {
	fact, err := s.status.FindStatus(ctx, ownerID, assetPublicID)
	if err != nil {
		return AssetStatus{}, err
	}
	return projectStatus(fact), nil
}

func (s *Service) History(ctx context.Context, ownerID uint64, page, pageSize int) (History, error) {
	values, total, err := s.status.ListStatus(ctx, ownerID, page, pageSize)
	if err != nil {
		return History{}, err
	}
	items := make([]AssetStatus, 0, len(values))
	for _, value := range values {
		items = append(items, projectStatus(value))
	}
	return History{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *Service) appendReviewSubmission(ctx context.Context, projection coremedia.ReviewProjection, taskID uint64) error {
	payload, err := json.Marshal(coremedia.ConsumerEventPayload{
		ProjectionID: projection.ProjectionID, ReviewTaskID: taskID,
		ExpectedRowVersion: projection.RowVersion,
	})
	if err != nil {
		return err
	}
	eventID, err := coremedia.NewULID()
	if err != nil {
		return err
	}
	now := s.now().UTC()
	err = s.outbox.AppendOutbox(ctx, &coremedia.OutboxEvent{
		EventID: eventID, AggregateID: projection.ProjectionID, AggregateVersion: projection.RowVersion,
		Operation: coremedia.ConsumerOperationSubmit, Payload: coremedia.JSONDocument(payload),
		RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	})
	if duplicateKey(err) {
		return nil
	}
	return err
}

func duplicateKey(err error) bool {
	var mysqlError *mysqldriver.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

func projectStatus(fact StatusFact) AssetStatus {
	state, message, recovery := StateProcessing, "图片正在进行技术处理", RecoveryPoll
	canUse := false
	switch {
	case fact.TechnicalStatus == coremedia.TechnicalDeleted:
		state, message, recovery = StateDeleted, "图片已删除", RecoveryNewUpload
	case fact.TicketStatus == coremedia.TicketExpired:
		state, message, recovery = StateExpired, "上传票据已过期，请重新选择图片", RecoveryNewUpload
	case fact.TicketStatus == coremedia.TicketCancelled || fact.TechnicalStatus == coremedia.TechnicalDeleteDeadletter:
		state, message, recovery = StateUploadFailed, "上传未完成，请重新上传", RecoveryNewUpload
	case fact.TicketStatus == coremedia.TicketCreated || fact.TicketStatus == coremedia.TicketAuthorizationIssued:
		state, message, recovery = StateWaitingUpload, "等待确认 COS 上传；已完成直传可继续确认，否则请重新上传", RecoveryComplete
	case fact.Projection != nil && *fact.Projection == coremedia.ProjectionApproved:
		state, message, recovery, canUse = StateApproved, "图片审核通过，可以使用", RecoveryNone, true
	case fact.Projection != nil && *fact.Projection == coremedia.ProjectionRejected:
		state, message, recovery = StateRejected, "图片未通过审核，请更换后重试", RecoveryNewUpload
	case fact.Projection != nil && *fact.Projection == coremedia.ProjectionReviewFailed:
		state, message, recovery = StateReviewFailed, "图片审核异常，可稍后查询或重新上传", RecoveryPoll
	case fact.TechnicalStatus == coremedia.TechnicalFinalReady:
		state, message, recovery = StateReviewPending, "图片已上传，正在等待内容审核", RecoveryPoll
	}
	projection := "pending"
	if fact.Projection != nil {
		projection = string(*fact.Projection)
	}
	return AssetStatus{
		AssetID: fact.AssetPublicID, TicketID: fact.TicketID, Purpose: fact.Purpose, State: state,
		TechnicalStatus: string(fact.TechnicalStatus), ReviewStatus: projection,
		TicketStatus: string(fact.TicketStatus), Message: message, Recovery: recovery,
		CanUse: canUse, UpdatedAt: fact.UpdatedAt.UTC(),
	}
}
