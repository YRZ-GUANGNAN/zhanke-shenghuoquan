package mediaconsole

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type QueryRepository interface {
	Summary(context.Context) (Summary, error)
	ListAssets(context.Context, ListFilter) ([]AssetRow, int64, error)
	AssetDetail(context.Context, string) (AssetDetail, error)
	ListDecisions(context.Context, ListFilter) ([]DecisionRow, int64, error)
	ListEvents(context.Context, ListFilter) ([]EventRow, int64, error)
	ListGCJobs(context.Context, ListFilter) ([]GCJobRow, int64, error)
}

type GormQueryRepository struct{ db *gorm.DB }

func NewQueryRepository(db *gorm.DB) *GormQueryRepository { return &GormQueryRepository{db: db} }

func (r *GormQueryRepository) Summary(ctx context.Context) (Summary, error) {
	var value Summary
	var auxiliaryDeadletters int64
	queries := []struct {
		target *int64
		table  string
		where  string
		args   []any
	}{
		{&value.Assets, "media_assets", "", nil},
		{&value.PendingReview, "media_review_projections", "status IN ?", []any{[]string{"pending", "submitted", "reviewing"}}},
		{&value.ConsumerDeadletters, "media_event_inbox", "status = ?", []any{"deadletter"}},
		{&value.GCDeadletters, "media_gc_jobs", "status = ?", []any{"deadletter"}},
		{&auxiliaryDeadletters, "media_gc_auxiliary_jobs", "status = ?", []any{"deadletter"}},
	}
	for _, query := range queries {
		result := r.db.WithContext(ctx).Table(query.table)
		if query.where != "" {
			result = result.Where(query.where, query.args...)
		}
		if err := result.Count(query.target).Error; err != nil {
			return Summary{}, err
		}
	}
	value.GCDeadletters += auxiliaryDeadletters
	return value, nil
}

func (r *GormQueryRepository) ListAssets(ctx context.Context, filter ListFilter) ([]AssetRow, int64, error) {
	base := r.assetQuery(ctx, filter)
	var total int64
	if err := base.Session(&gorm.Session{}).Distinct("a.id").Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []AssetRow
	err := base.Select(`a.id AS db_asset_id, a.asset_public_id, a.owner_id, a.purpose, a.storage_stage,
		a.technical_status, COALESCE(p.status, 'unavailable') AS projection_status, p.review_task_id,
		t.ticket_id, t.status AS ticket_status, c.phase AS completion_phase,
		COALESCE(ref.reference_count, 0) AS reference_count, COALESCE(ref.blocking_references, 0) AS blocking_references,
		a.verified_mime, a.byte_size, a.width, a.height, a.created_at, a.updated_at,
		a.delete_requested_at, a.deleted_at`).
		Order("a.created_at DESC, a.asset_public_id DESC").
		Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Scan(&values).Error
	return values, total, err
}

func (r *GormQueryRepository) assetQuery(ctx context.Context, filter ListFilter) *gorm.DB {
	query := r.db.WithContext(ctx).Table("media_assets AS a").
		Joins("LEFT JOIN media_review_projections AS p ON p.asset_id = a.id").
		Joins("LEFT JOIN media_upload_tickets AS t ON t.asset_id = a.id").
		Joins("LEFT JOIN media_upload_completions AS c ON c.asset_id = a.id").
		Joins(`LEFT JOIN (
			SELECT asset_id, COUNT(*) AS reference_count,
			SUM(CASE WHEN status <> 'revoked' THEN 1 ELSE 0 END) AS blocking_references
			FROM media_asset_references GROUP BY asset_id
		) AS ref ON ref.asset_id = a.id`)
	if filter.AssetID != "" {
		query = query.Where("a.asset_public_id = ?", filter.AssetID)
	}
	if filter.Status != "" {
		query = query.Where("a.technical_status = ?", filter.Status)
	}
	if filter.ReviewStatus != "" {
		query = query.Where("p.status = ?", filter.ReviewStatus)
	}
	if filter.Purpose != "" {
		query = query.Where("a.purpose = ?", filter.Purpose)
	}
	if filter.TaskID != nil {
		query = query.Where("p.review_task_id = ?", *filter.TaskID)
	}
	return applyTimeFilter(query, "a.created_at", filter)
}

func (r *GormQueryRepository) AssetDetail(ctx context.Context, assetPublicID string) (AssetDetail, error) {
	assets, total, err := r.ListAssets(ctx, ListFilter{Page: 1, PageSize: 1, AssetID: assetPublicID})
	if err != nil {
		return AssetDetail{}, err
	}
	if total != 1 {
		return AssetDetail{}, ErrNotFound
	}
	detail := AssetDetail{Asset: assets[0], References: []ReferenceRow{}, Decisions: []DecisionRow{}, Events: []EventRow{}, GCJobs: []GCJobRow{}}
	db := r.db.WithContext(ctx)
	if err := db.Table("media_asset_references").Select(`id, reference_type, reference_id, slot, status,
		parent_visibility, aggregate_version, fence_expires_at, updated_at`).
		Where("asset_id = ?", detail.Asset.DBAssetID).Order("reference_type, reference_id, slot").Scan(&detail.References).Error; err != nil {
		return AssetDetail{}, err
	}
	var ticket TicketRow
	if err := db.Table("media_upload_tickets").Select(`ticket_id, status, expected_size, expected_content_type,
		credential_issue_count, expires_at, created_at, updated_at`).Where("asset_id = ?", detail.Asset.DBAssetID).Take(&ticket).Error; err == nil {
		detail.Ticket = &ticket
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return AssetDetail{}, err
	}
	var completion CompletionRow
	if err := db.Table("media_upload_completions").Select(`ticket_id, phase, validation_attempts, promote_attempts,
		last_error_code, completed_at, updated_at`).Where("asset_id = ?", detail.Asset.DBAssetID).Take(&completion).Error; err == nil {
		detail.Completion = &completion
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return AssetDetail{}, err
	}
	var projection ProjectionRow
	if err := db.Table("media_review_projections").Select(`projection_id, review_task_id, status,
		applied_decision_version, failure_code, submitted_at, reviewing_at, decided_at, updated_at`).
		Where("asset_id = ?", detail.Asset.DBAssetID).Take(&projection).Error; err == nil {
		detail.Projection = &projection
		if err := db.Table("media_review_decisions AS d").Select(`? AS asset_public_id, d.projection_id, d.review_task_id,
			d.decision_version, d.decision, d.payload_hash, d.result_code, d.received_at`, assetPublicID).
			Where("d.projection_id = ?", projection.ProjectionID).Order("d.decision_version DESC").Scan(&detail.Decisions).Error; err != nil {
			return AssetDetail{}, err
		}
		eventFilter := ListFilter{Page: 1, PageSize: 100, AssetID: projection.ProjectionID}
		detail.Events, _, err = r.ListEvents(ctx, eventFilter)
		if err != nil {
			return AssetDetail{}, err
		}
		var checkpoint CheckpointRow
		if err := db.Table("media_consumer_checkpoints").Select(`aggregate_id, last_event_id, last_aggregate_version,
			last_operation, active_event_id, lease_expires_at, updated_at`).Where("aggregate_id = ?", projection.ProjectionID).Take(&checkpoint).Error; err == nil {
			detail.Checkpoint = &checkpoint
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return AssetDetail{}, err
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return AssetDetail{}, err
	}
	detail.GCJobs, _, err = r.ListGCJobs(ctx, ListFilter{Page: 1, PageSize: 100, AssetID: assetPublicID})
	return detail, err
}

func (r *GormQueryRepository) ListDecisions(ctx context.Context, filter ListFilter) ([]DecisionRow, int64, error) {
	query := r.db.WithContext(ctx).Table("media_review_decisions AS d").
		Joins("JOIN media_review_projections AS p ON p.projection_id = d.projection_id").
		Joins("JOIN media_assets AS a ON a.id = p.asset_id")
	if filter.AssetID != "" {
		query = query.Where("a.asset_public_id = ?", filter.AssetID)
	}
	if filter.Status != "" {
		query = query.Where("d.decision = ?", filter.Status)
	}
	if filter.TaskID != nil {
		query = query.Where("d.review_task_id = ?", *filter.TaskID)
	}
	query = applyTimeFilter(query, "d.received_at", filter)
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []DecisionRow
	err := query.Select(`a.asset_public_id, d.projection_id, d.review_task_id, d.decision_version,
		d.decision, d.payload_hash, d.result_code, d.received_at`).Order("d.received_at DESC, d.id DESC").
		Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Scan(&values).Error
	return values, total, err
}

func (r *GormQueryRepository) ListEvents(ctx context.Context, filter ListFilter) ([]EventRow, int64, error) {
	query := r.db.WithContext(ctx).Table("media_outbox AS o").
		Joins("LEFT JOIN media_event_inbox AS i ON i.event_id = o.event_id").
		Joins("LEFT JOIN media_review_projections AS p ON p.projection_id = o.aggregate_id").
		Joins("LEFT JOIN media_assets AS a ON a.id = p.asset_id")
	if filter.AssetID != "" {
		// Asset detail calls by projection aggregate ID; console filters call by
		// public asset ID. Both are exact ULIDs and remain index-backed.
		query = query.Where("a.asset_public_id = ? OR o.aggregate_id = ?", filter.AssetID, filter.AssetID)
	}
	if filter.Status != "" {
		if filter.Status == "unconsumed" {
			query = query.Where("i.event_id IS NULL")
		} else {
			query = query.Where("i.status = ?", filter.Status)
		}
	}
	if filter.Operation != "" {
		query = query.Where("o.operation = ?", filter.Operation)
	}
	query = applyTimeFilter(query, "o.created_at", filter)
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []EventRow
	err := query.Select(`o.event_id, o.aggregate_id, o.aggregate_version, o.operation,
		COALESCE(i.status, 'unconsumed') AS consumer_status, COALESCE(i.attempt_count, 0) AS attempt_count,
		COALESCE(i.last_error_code, '') AS last_error_code, i.next_attempt_at,
		COALESCE(i.processed_at, o.processed_at) AS processed_at, o.created_at`).
		Order("o.created_at DESC, o.event_id DESC").Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).Scan(&values).Error
	return values, total, err
}

func (r *GormQueryRepository) ListGCJobs(ctx context.Context, filter ListFilter) ([]GCJobRow, int64, error) {
	objectWhere, objectArgs := gcWhere("j", "a", filter, false)
	auxWhere, auxArgs := gcWhere("j", "a", filter, true)
	objectSQL := `SELECT 'object' AS kind, j.job_id, a.asset_public_id, j.object_stage, j.status,
		j.attempt_count, j.last_error_code, j.provider_request_id, j.deleted_at, j.created_at, j.updated_at
		FROM media_gc_jobs j JOIN media_assets a ON a.id = j.asset_id` + objectWhere
	auxSQL := `SELECT 'auxiliary' AS kind, j.job_id, a.asset_public_id, '' AS object_stage, j.status,
		j.attempt_count, j.last_error_code, '' AS provider_request_id, j.purged_at AS deleted_at, j.created_at, j.updated_at
		FROM media_gc_auxiliary_jobs j JOIN media_assets a ON a.id = j.asset_id` + auxWhere
	parts, args := []string{}, []any{}
	if filter.Kind == "" || filter.Kind == "object" {
		parts, args = append(parts, objectSQL), append(args, objectArgs...)
	}
	if filter.Kind == "" || filter.Kind == "auxiliary" {
		parts, args = append(parts, auxSQL), append(args, auxArgs...)
	}
	if len(parts) == 0 {
		return []GCJobRow{}, 0, nil
	}
	union := strings.Join(parts, " UNION ALL ")
	var total int64
	if err := r.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM ("+union+") AS gc", args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []GCJobRow
	queryArgs := append(append([]any{}, args...), filter.PageSize, (filter.Page-1)*filter.PageSize)
	err := r.db.WithContext(ctx).Raw("SELECT * FROM ("+union+") AS gc ORDER BY created_at DESC, job_id DESC LIMIT ? OFFSET ?", queryArgs...).Scan(&values).Error
	return values, total, err
}

func gcWhere(jobAlias, assetAlias string, filter ListFilter, auxiliary bool) (string, []any) {
	conditions, args := []string{"1=1"}, []any{}
	if filter.AssetID != "" {
		conditions, args = append(conditions, assetAlias+".asset_public_id = ?"), append(args, filter.AssetID)
	}
	if filter.Status != "" {
		conditions, args = append(conditions, jobAlias+".status = ?"), append(args, filter.Status)
	}
	if filter.From != nil {
		conditions, args = append(conditions, jobAlias+".created_at >= ?"), append(args, *filter.From)
	}
	if filter.To != nil {
		conditions, args = append(conditions, jobAlias+".created_at < ?"), append(args, *filter.To)
	}
	if !auxiliary && filter.Purpose != "" { // object stage reuses the purpose filter slot internally
		conditions, args = append(conditions, jobAlias+".object_stage = ?"), append(args, filter.Purpose)
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

func applyTimeFilter(query *gorm.DB, column string, filter ListFilter) *gorm.DB {
	if filter.From != nil {
		query = query.Where(fmt.Sprintf("%s >= ?", column), *filter.From)
	}
	if filter.To != nil {
		query = query.Where(fmt.Sprintf("%s < ?", column), *filter.To)
	}
	return query
}

var _ QueryRepository = (*GormQueryRepository)(nil)
