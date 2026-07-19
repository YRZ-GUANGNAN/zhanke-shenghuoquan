package identityapplication

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	FindCurrent(context.Context, uint64, Role) (Application, bool, error)
	ListCurrent(context.Context, uint64) ([]Application, error)
	SaveSubmission(context.Context, *Application, []string, EventType, uint64, SubmissionCommit) error
	ListAdmin(context.Context, AdminListFilter) ([]Application, int64, error)
	FindDetail(context.Context, string) (ApplicationDetail, error)
	Approve(context.Context, DecisionInput, ApprovalCommit) (Application, error)
	Reject(context.Context, DecisionInput) (Application, error)
}

// GrantFinder is the read-only boundary used by other domains to decide
// whether an approved rider or merchant identity is currently available.
// A missing row is represented by found=false; suspended and active grants
// are returned as ordinary persisted states.
type GrantFinder interface {
	FindGrant(context.Context, uint64, Role) (IdentityGrant, bool, error)
}

type SubmissionCommit func(context.Context, *gorm.DB) error
type ApprovalCommit func(context.Context, *gorm.DB, ApprovedIdentity) error

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) FindGrant(ctx context.Context, userID uint64, role Role) (IdentityGrant, bool, error) {
	if r == nil || r.db == nil || userID == 0 || !role.Valid() {
		return IdentityGrant{}, false, ErrInvalidInput
	}
	var value IdentityGrant
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND role = ?", userID, role).
		Take(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return IdentityGrant{}, false, nil
	}
	return value, err == nil, err
}

func (r *GormRepository) FindCurrent(ctx context.Context, userID uint64, role Role) (Application, bool, error) {
	var value Application
	err := r.withAssets(r.db.WithContext(ctx)).
		Where("user_id = ? AND role = ?", userID, role).
		Take(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Application{}, false, nil
	}
	return value, err == nil, err
}

func (r *GormRepository) ListCurrent(ctx context.Context, userID uint64) ([]Application, error) {
	values := make([]Application, 0, 2)
	err := r.withAssets(r.db.WithContext(ctx)).
		Where("user_id = ?", userID).
		Order("role ASC").
		Find(&values).Error
	return values, err
}

func (r *GormRepository) SaveSubmission(ctx context.Context, value *Application, assetIDs []string, eventType EventType, expectedRowVersion uint64, commit SubmissionCommit) error {
	if value == nil || value.UserID == 0 || !value.Role.Valid() || value.Status != StatusPending || commit == nil {
		return ErrInvalidInput
	}
	if value.ID == 0 && eventType != EventSubmitted {
		return ErrInvalidInput
	}
	if value.ID != 0 && eventType != EventResubmitted {
		return ErrInvalidInput
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if value.ID == 0 {
			if expectedRowVersion != 0 || value.RowVersion != 1 {
				return ErrVersionConflict
			}
			if err := tx.Omit("Assets").Create(value).Error; err != nil {
				return mapSubmissionWriteError(err)
			}
		} else {
			if expectedRowVersion == 0 || value.RowVersion != expectedRowVersion+1 {
				return ErrVersionConflict
			}
			result := tx.Model(&Application{}).
				Where("id = ? AND user_id = ? AND role = ? AND status = ? AND row_version = ?", value.ID, value.UserID, value.Role, StatusRejected, expectedRowVersion).
				Updates(map[string]any{
					"status":           StatusPending,
					"real_name":        value.RealName,
					"phone":            value.Phone,
					"merchant_name":    value.MerchantName,
					"merchant_address": value.MerchantAddress,
					"review_reason":    "",
					"reviewer_user_id": nil,
					"reviewed_at":      nil,
					"submitted_at":     value.SubmittedAt,
					"row_version":      value.RowVersion,
					"updated_at":       value.UpdatedAt,
				})
			if result.Error != nil {
				return mapSubmissionWriteError(result.Error)
			}
			if result.RowsAffected == 0 {
				return classifySubmissionConflict(tx, value.ID, expectedRowVersion)
			}
		}

		if err := tx.Where("application_id = ?", value.ID).Delete(&ApplicationAsset{}).Error; err != nil {
			return err
		}
		assets := make([]ApplicationAsset, len(assetIDs))
		for index, assetID := range assetIDs {
			assets[index] = ApplicationAsset{
				ApplicationID: value.ID,
				AssetPublicID: assetID,
				SortOrder:     uint16(index),
				CreatedAt:     value.UpdatedAt,
			}
		}
		if len(assets) > 0 {
			if err := tx.Create(&assets).Error; err != nil {
				return mapSubmissionWriteError(err)
			}
		}

		snapshot, err := json.Marshal(snapshotApplication(*value, assetIDs))
		if err != nil {
			return err
		}
		eventID, err := newULIDAt(value.UpdatedAt, rand.Reader)
		if err != nil {
			return err
		}
		event := ApplicationEvent{
			EventPublicID: eventID,
			ApplicationID: value.ID,
			ActorUserID:   value.UserID,
			EventType:     eventType,
			Status:        StatusPending,
			SnapshotJSON:  snapshot,
			CreatedAt:     value.UpdatedAt,
		}
		if err := tx.Create(&event).Error; err != nil {
			return mapSubmissionWriteError(err)
		}
		if err := commit(ctx, tx); err != nil {
			return err
		}
		value.Assets = assets
		return nil
	})
}

func (r *GormRepository) Approve(ctx context.Context, input DecisionInput, commit ApprovalCommit) (Application, error) {
	if commit == nil || input.Reason != "" {
		return Application{}, ErrInvalidInput
	}
	return r.decide(ctx, input, StatusApproved, EventApproved, commit)
}

func (r *GormRepository) Reject(ctx context.Context, input DecisionInput) (Application, error) {
	if input.Reason == "" {
		return Application{}, ErrInvalidInput
	}
	return r.decide(ctx, input, StatusRejected, EventRejected, nil)
}

func (r *GormRepository) decide(
	ctx context.Context,
	input DecisionInput,
	status Status,
	eventType EventType,
	commit ApprovalCommit,
) (Application, error) {
	if r == nil || r.db == nil || input.ReviewerUserID == 0 || !validULID(input.ApplicationPublicID) ||
		input.ExpectedRowVersion == 0 || input.DecidedAt.IsZero() ||
		(status != StatusApproved && status != StatusRejected) ||
		(eventType != EventApproved && eventType != EventRejected) {
		return Application{}, ErrInvalidInput
	}
	var result Application
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.withAssets(tx.Clauses(clause.Locking{Strength: "UPDATE"})).
			Where("application_public_id = ?", input.ApplicationPublicID).
			Take(&result).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrApplicationNotFound
			}
			return err
		}
		if result.UserID == input.ReviewerUserID {
			return ErrSelfReview
		}
		if result.Status != StatusPending || result.RowVersion != input.ExpectedRowVersion {
			return ErrDecisionConflict
		}

		if status == StatusApproved {
			grant := IdentityGrant{
				UserID: result.UserID, Role: result.Role, Status: GrantActive,
				SourceApplicationID: &result.ID, GrantedAt: input.DecidedAt,
				RowVersion: 1, CreatedAt: input.DecidedAt, UpdatedAt: input.DecidedAt,
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "user_id"}, {Name: "role"}},
				DoUpdates: clause.Assignments(map[string]any{
					"status": GrantActive, "source_application_id": result.ID,
					"granted_at": input.DecidedAt, "updated_at": input.DecidedAt,
					"row_version": gorm.Expr("row_version + 1"),
				}),
			}).Create(&grant).Error; err != nil {
				return mapDecisionWriteError(err)
			}
			approved := ApprovedIdentity{
				UserID: result.UserID, Role: result.Role,
				RealName: result.RealName, Phone: result.Phone,
			}
			if err := commit(ctx, tx, approved); err != nil {
				return err
			}
		}

		nextVersion := result.RowVersion + 1
		update := tx.Model(&Application{}).
			Where("id = ? AND status = ? AND row_version = ?", result.ID, StatusPending, input.ExpectedRowVersion).
			Updates(map[string]any{
				"status": status, "review_reason": input.Reason,
				"reviewer_user_id": input.ReviewerUserID, "reviewed_at": input.DecidedAt,
				"row_version": nextVersion, "updated_at": input.DecidedAt,
			})
		if update.Error != nil {
			return mapDecisionWriteError(update.Error)
		}
		if update.RowsAffected != 1 {
			return ErrDecisionConflict
		}
		result.Status = status
		result.ReviewReason = input.Reason
		reviewerID := input.ReviewerUserID
		result.ReviewerUserID = &reviewerID
		reviewedAt := input.DecidedAt
		result.ReviewedAt = &reviewedAt
		result.RowVersion = nextVersion
		result.UpdatedAt = input.DecidedAt

		snapshot, err := json.Marshal(snapshotApplication(result, applicationAssetPublicIDs(result.Assets)))
		if err != nil {
			return err
		}
		eventID, err := newULIDAt(input.DecidedAt, rand.Reader)
		if err != nil {
			return err
		}
		event := ApplicationEvent{
			EventPublicID: eventID, ApplicationID: result.ID,
			ActorUserID: input.ReviewerUserID, EventType: eventType,
			Status: status, Reason: input.Reason, SnapshotJSON: snapshot,
			CreatedAt: input.DecidedAt,
		}
		return mapDecisionWriteError(tx.Create(&event).Error)
	})
	return result, err
}

func snapshotApplication(value Application, assetIDs []string) applicationSnapshot {
	var reviewerUserID *string
	if value.ReviewerUserID != nil {
		formatted := strconv.FormatUint(*value.ReviewerUserID, 10)
		reviewerUserID = &formatted
	}
	return applicationSnapshot{
		ApplicationPublicID: value.ApplicationPublicID,
		UserID:              strconv.FormatUint(value.UserID, 10),
		Role:                value.Role,
		Status:              value.Status,
		RealName:            value.RealName,
		Phone:               value.Phone,
		MerchantName:        value.MerchantName,
		MerchantAddress:     value.MerchantAddress,
		AssetPublicIDs:      append([]string(nil), assetIDs...),
		ReviewReason:        value.ReviewReason,
		ReviewerUserID:      reviewerUserID,
		ReviewedAt:          value.ReviewedAt,
		RowVersion:          value.RowVersion,
	}
}

func applicationAssetPublicIDs(assets []ApplicationAsset) []string {
	values := make([]string, len(assets))
	for index := range assets {
		values[index] = assets[index].AssetPublicID
	}
	return values
}

func (r *GormRepository) ListAdmin(ctx context.Context, filter AdminListFilter) ([]Application, int64, error) {
	page, pageSize := filter.Page, filter.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	query := r.db.WithContext(ctx).Model(&Application{})
	if filter.Role != "" {
		query = query.Where("role = ?", filter.Role)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	values := make([]Application, 0)
	err := r.withAssets(query).
		Order("submitted_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&values).Error
	return values, total, err
}

func (r *GormRepository) FindDetail(ctx context.Context, applicationPublicID string) (ApplicationDetail, error) {
	var value Application
	err := r.withAssets(r.db.WithContext(ctx)).
		Where("application_public_id = ?", applicationPublicID).
		Take(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ApplicationDetail{}, ErrApplicationNotFound
	}
	if err != nil {
		return ApplicationDetail{}, err
	}
	var events []ApplicationEvent
	if err := r.db.WithContext(ctx).
		Where("application_id = ?", value.ID).
		Order("created_at ASC, id ASC").
		Find(&events).Error; err != nil {
		return ApplicationDetail{}, err
	}
	eventViews := make([]ApplicationEventView, len(events))
	for index := range events {
		eventViews[index] = ApplicationEventView{
			EventPublicID: events[index].EventPublicID,
			ActorUserID:   strconv.FormatUint(events[index].ActorUserID, 10),
			EventType:     events[index].EventType,
			Status:        events[index].Status,
			Reason:        events[index].Reason,
			CreatedAt:     events[index].CreatedAt,
		}
	}
	return ApplicationDetail{ApplicationView: toApplicationView(value), Events: eventViews}, nil
}

func (r *GormRepository) withAssets(query *gorm.DB) *gorm.DB {
	return query.Preload("Assets", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC, id ASC")
	})
}

func classifySubmissionConflict(tx *gorm.DB, id, expectedRowVersion uint64) error {
	var current Application
	err := tx.Select("id", "status", "row_version").Where("id = ?", id).Take(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrApplicationNotFound
	}
	if err != nil {
		return err
	}
	switch current.Status {
	case StatusPending:
		return ErrApplicationPending
	case StatusApproved:
		return ErrApplicationApproved
	case StatusRejected:
		if current.RowVersion != expectedRowVersion {
			return ErrVersionConflict
		}
	}
	return ErrVersionConflict
}

func mapSubmissionWriteError(err error) error {
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return ErrVersionConflict
	}
	if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
		return ErrVersionConflict
	}
	return err
}

func mapDecisionWriteError(err error) error {
	if err == nil {
		return nil
	}
	var mysqlErr *mysqlDriver.MySQLError
	if errors.As(err, &mysqlErr) && (mysqlErr.Number == 1062 || mysqlErr.Number == 1213 || mysqlErr.Number == 1205) {
		return errors.Join(ErrDecisionConflict, err)
	}
	return err
}

var _ Repository = (*GormRepository)(nil)
var _ GrantFinder = (*GormRepository)(nil)
