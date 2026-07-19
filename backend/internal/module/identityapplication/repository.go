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
)

type Repository interface {
	FindCurrent(context.Context, uint64, Role) (Application, bool, error)
	ListCurrent(context.Context, uint64) ([]Application, error)
	SaveSubmission(context.Context, *Application, []string, EventType, uint64, SubmissionCommit) error
	ListAdmin(context.Context, AdminListFilter) ([]Application, int64, error)
	FindDetail(context.Context, string) (ApplicationDetail, error)
}

type SubmissionCommit func(context.Context, *gorm.DB) error

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

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

		snapshot, err := json.Marshal(applicationSnapshot{
			ApplicationPublicID: value.ApplicationPublicID,
			UserID:              value.UserID,
			Role:                value.Role,
			Status:              value.Status,
			RealName:            value.RealName,
			Phone:               value.Phone,
			MerchantName:        value.MerchantName,
			MerchantAddress:     value.MerchantAddress,
			AssetPublicIDs:      append([]string(nil), assetIDs...),
			RowVersion:          value.RowVersion,
		})
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

var _ Repository = (*GormRepository)(nil)
