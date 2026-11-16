package studentverification

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	FindCurrent(context.Context, uint64) (Application, bool, error)
	SaveSubmission(context.Context, *Application, []string, EventType, uint64, SubmissionCommit) error
	ListAdmin(context.Context, AdminListFilter) ([]Application, int64, error)
	FindDetail(context.Context, string) (VerificationDetail, error)
	Approve(context.Context, DecisionInput) (Application, error)
	Reject(context.Context, DecisionInput) (Application, error)
	RequestCancel(context.Context, uint64, uint64, time.Time) (Application, error)
	ApproveCancel(context.Context, DecisionInput) (Application, error)
}

type SubmissionCommit func(context.Context, *gorm.DB) error

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) FindCurrent(ctx context.Context, userID uint64) (Application, bool, error) {
	if r == nil || r.db == nil || userID == 0 {
		return Application{}, false, ErrInvalidInput
	}
	var value Application
	err := r.withAssets(r.db.WithContext(ctx)).
		Where("user_id = ?", userID).
		Take(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Application{}, false, nil
	}
	return value, err == nil, err
}

func (r *GormRepository) SaveSubmission(ctx context.Context, value *Application, assetIDs []string, eventType EventType, expectedRowVersion uint64, commit SubmissionCommit) error {
	if value == nil || value.UserID == 0 || value.Status != StatusPending || commit == nil {
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
				Where("id = ? AND user_id = ? AND status IN ? AND row_version = ?",
					value.ID, value.UserID, []Status{StatusRejected, StatusCancelled}, expectedRowVersion).
				Updates(map[string]any{
					"status":              StatusPending,
					"real_name":           value.RealName,
					"student_no":          value.StudentNo,
					"college":             value.College,
					"review_reason":       "",
					"reviewer_user_id":    nil,
					"reviewed_at":         nil,
					"cancel_requested_at": nil,
					"submitted_at":        value.SubmittedAt,
					"row_version":         value.RowVersion,
					"updated_at":          value.UpdatedAt,
				})
			if result.Error != nil {
				return mapSubmissionWriteError(result.Error)
			}
			if result.RowsAffected == 0 {
				return classifySubmissionConflict(tx, value.ID, expectedRowVersion)
			}
		}

		if err := tx.Where("verification_id = ?", value.ID).Delete(&ApplicationAsset{}).Error; err != nil {
			return err
		}
		assets := make([]ApplicationAsset, len(assetIDs))
		for index, assetID := range assetIDs {
			assets[index] = ApplicationAsset{
				VerificationID: value.ID,
				AssetPublicID:  assetID,
				SortOrder:      uint16(index),
				CreatedAt:      value.UpdatedAt,
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
			EventPublicID:  eventID,
			VerificationID: value.ID,
			ActorUserID:    value.UserID,
			EventType:      eventType,
			Status:         StatusPending,
			SnapshotJSON:   snapshot,
			CreatedAt:      value.UpdatedAt,
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

func (r *GormRepository) Approve(ctx context.Context, input DecisionInput) (Application, error) {
	return r.decide(ctx, input, StatusApproved, EventApproved)
}

func (r *GormRepository) Reject(ctx context.Context, input DecisionInput) (Application, error) {
	if input.Reason == "" {
		return Application{}, ErrInvalidInput
	}
	return r.decide(ctx, input, StatusRejected, EventRejected)
}

func (r *GormRepository) decide(ctx context.Context, input DecisionInput, status Status, eventType EventType) (Application, error) {
	if r == nil || r.db == nil || input.ReviewerUserID == 0 || !validULID(input.ApplicationPublicID) ||
		input.ExpectedRowVersion == 0 || input.DecidedAt.IsZero() ||
		(status != StatusApproved && status != StatusRejected) ||
		(eventType != EventApproved && eventType != EventRejected) {
		return Application{}, ErrInvalidInput
	}
	var result Application
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.withAssets(tx.Clauses(clause.Locking{Strength: "UPDATE"})).
			Where("public_id = ?", input.ApplicationPublicID).
			Take(&result).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrApplicationNotFound
			}
			return err
		}
		if result.UserID == input.ReviewerUserID {
			return ErrSelfReview
		}
		if result.RowVersion != input.ExpectedRowVersion {
			return ErrDecisionConflict
		}
		if result.Status != StatusPending {
			return ErrDecisionConflict
		}
		previousStatus := result.Status

		nextVersion := result.RowVersion + 1
		update := tx.Model(&Application{}).
			Where("id = ? AND status = ? AND row_version = ?", result.ID, previousStatus, input.ExpectedRowVersion).
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
		applyDecision(&result, status, input.Reason, input.ReviewerUserID, input.DecidedAt, nextVersion)
		return r.appendEvent(tx, &result, input.ReviewerUserID, eventType, status, input.Reason, input.DecidedAt)
	})
	return result, err
}

func (r *GormRepository) RequestCancel(ctx context.Context, userID uint64, expectedRowVersion uint64, requestedAt time.Time) (Application, error) {
	if r == nil || r.db == nil || userID == 0 || expectedRowVersion == 0 || requestedAt.IsZero() {
		return Application{}, ErrInvalidInput
	}
	var result Application
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.withAssets(tx.Clauses(clause.Locking{Strength: "UPDATE"})).
			Where("user_id = ?", userID).
			Take(&result).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrApplicationNotFound
			}
			return err
		}
		if result.Status != StatusApproved {
			return ErrCancelNotAllowed
		}
		if result.CancelRequestedAt != nil {
			return nil
		}
		if result.RowVersion != expectedRowVersion {
			return ErrDecisionConflict
		}
		nextVersion := result.RowVersion + 1
		update := tx.Model(&Application{}).
			Where("id = ? AND status = ? AND row_version = ?", result.ID, StatusApproved, expectedRowVersion).
			Updates(map[string]any{
				"cancel_requested_at": requestedAt,
				"row_version":         nextVersion,
				"updated_at":          requestedAt,
			})
		if update.Error != nil {
			return mapDecisionWriteError(update.Error)
		}
		if update.RowsAffected != 1 {
			return ErrDecisionConflict
		}
		result.CancelRequestedAt = &requestedAt
		result.RowVersion = nextVersion
		result.UpdatedAt = requestedAt
		return r.appendEvent(tx, &result, userID, EventCancelRequested, StatusApproved, "", requestedAt)
	})
	return result, err
}

func (r *GormRepository) ApproveCancel(ctx context.Context, input DecisionInput) (Application, error) {
	if r == nil || r.db == nil || input.ReviewerUserID == 0 || !validULID(input.ApplicationPublicID) ||
		input.ExpectedRowVersion == 0 || input.DecidedAt.IsZero() {
		return Application{}, ErrInvalidInput
	}
	var result Application
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.withAssets(tx.Clauses(clause.Locking{Strength: "UPDATE"})).
			Where("public_id = ?", input.ApplicationPublicID).
			Take(&result).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrApplicationNotFound
			}
			return err
		}
		if result.UserID == input.ReviewerUserID {
			return ErrSelfReview
		}
		if result.Status != StatusApproved || result.CancelRequestedAt == nil {
			return ErrDecisionConflict
		}
		if result.RowVersion != input.ExpectedRowVersion {
			return ErrDecisionConflict
		}
		nextVersion := result.RowVersion + 1
		update := tx.Model(&Application{}).
			Where("id = ? AND status = ? AND row_version = ?", result.ID, StatusApproved, input.ExpectedRowVersion).
			Updates(map[string]any{
				"status": StatusCancelled, "review_reason": input.Reason,
				"reviewer_user_id": input.ReviewerUserID, "reviewed_at": input.DecidedAt,
				"row_version": nextVersion, "updated_at": input.DecidedAt,
			})
		if update.Error != nil {
			return mapDecisionWriteError(update.Error)
		}
		if update.RowsAffected != 1 {
			return ErrDecisionConflict
		}
		applyDecision(&result, StatusCancelled, input.Reason, input.ReviewerUserID, input.DecidedAt, nextVersion)
		return r.appendEvent(tx, &result, input.ReviewerUserID, EventCancelApproved, StatusCancelled, input.Reason, input.DecidedAt)
	})
	return result, err
}

func applyDecision(value *Application, status Status, reason string, reviewerUserID uint64, decidedAt time.Time, nextVersion uint64) {
	value.Status = status
	value.ReviewReason = reason
	reviewerID := reviewerUserID
	value.ReviewerUserID = &reviewerID
	reviewedAt := decidedAt
	value.ReviewedAt = &reviewedAt
	value.RowVersion = nextVersion
	value.UpdatedAt = decidedAt
}

func (r *GormRepository) appendEvent(tx *gorm.DB, value *Application, actorUserID uint64, eventType EventType, status Status, reason string, at time.Time) error {
	snapshot, err := json.Marshal(snapshotApplication(*value, applicationAssetPublicIDs(value.Assets)))
	if err != nil {
		return err
	}
	eventID, err := newULIDAt(at, rand.Reader)
	if err != nil {
		return err
	}
	event := ApplicationEvent{
		EventPublicID:  eventID,
		VerificationID: value.ID,
		ActorUserID:    actorUserID,
		EventType:      eventType,
		Status:         status,
		Reason:         reason,
		SnapshotJSON:   snapshot,
		CreatedAt:      at,
	}
	return mapDecisionWriteError(tx.Create(&event).Error)
}

func snapshotApplication(value Application, assetIDs []string) verificationSnapshot {
	var reviewerUserID *string
	if value.ReviewerUserID != nil {
		formatted := strconv.FormatUint(*value.ReviewerUserID, 10)
		reviewerUserID = &formatted
	}
	return verificationSnapshot{
		ApplicationPublicID: value.ApplicationPublicID,
		UserID:              strconv.FormatUint(value.UserID, 10),
		Status:              value.Status,
		RealName:            value.RealName,
		StudentNo:           value.StudentNo,
		College:             value.College,
		AssetPublicIDs:      append([]string(nil), assetIDs...),
		ReviewReason:        value.ReviewReason,
		ReviewerUserID:      reviewerUserID,
		ReviewedAt:          value.ReviewedAt,
		CancelRequestedAt:   value.CancelRequestedAt,
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

func (r *GormRepository) FindDetail(ctx context.Context, applicationPublicID string) (VerificationDetail, error) {
	var value Application
	err := r.withAssets(r.db.WithContext(ctx)).
		Where("public_id = ?", applicationPublicID).
		Take(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return VerificationDetail{}, ErrApplicationNotFound
	}
	if err != nil {
		return VerificationDetail{}, err
	}
	var events []ApplicationEvent
	if err := r.db.WithContext(ctx).
		Where("verification_id = ?", value.ID).
		Order("created_at ASC, id ASC").
		Find(&events).Error; err != nil {
		return VerificationDetail{}, err
	}
	eventViews := make([]VerificationEventView, len(events))
	for index := range events {
		eventViews[index] = VerificationEventView{
			EventPublicID: events[index].EventPublicID,
			ActorUserID:   strconv.FormatUint(events[index].ActorUserID, 10),
			EventType:     events[index].EventType,
			Status:        events[index].Status,
			Reason:        events[index].Reason,
			CreatedAt:     events[index].CreatedAt,
		}
	}
	return VerificationDetail{VerificationView: toVerificationView(value), Events: eventViews}, nil
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
	case StatusRejected, StatusCancelled:
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
