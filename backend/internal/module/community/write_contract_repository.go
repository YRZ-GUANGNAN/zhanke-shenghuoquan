package community

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrIdempotencyConflict  = errors.New("community idempotency key payload conflict")
	ErrWriteContractInvalid = errors.New("community write contract is invalid")
	ErrWriteOutboxLease     = errors.New("community write outbox lease lost")
	ErrWriteOutboxInvariant = errors.New("community write outbox invariant violated")
)

type WriteOutboxStatus string

const (
	WriteOutboxPending    WriteOutboxStatus = "pending"
	WriteOutboxProcessing WriteOutboxStatus = "processing"
	WriteOutboxRetryWait  WriteOutboxStatus = "retry_wait"
	WriteOutboxSucceeded  WriteOutboxStatus = "succeeded"
	WriteOutboxDeadletter WriteOutboxStatus = "deadletter"
)

const writeContractULIDAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

type WriteOutbox struct {
	ID               uint64            `gorm:"primaryKey"`
	EventPublicID    string            `gorm:"size:26;not null"`
	AggregateType    string            `gorm:"size:32;not null"`
	AggregateID      uint64            `gorm:"not null"`
	OwnerUserID      uint64            `gorm:"not null"`
	EventType        string            `gorm:"size:48;not null"`
	ReservationToken *string           `gorm:"size:128"`
	Payload          string            `gorm:"type:json;not null"`
	Status           WriteOutboxStatus `gorm:"size:24;not null"`
	AttemptCount     uint32            `gorm:"not null"`
	AvailableAt      time.Time         `gorm:"not null"`
	LeaseToken       *string           `gorm:"size:26"`
	LeaseExpiresAt   *time.Time
	ProcessedAt      *time.Time
	FailedAt         *time.Time
	LastError        string `gorm:"size:512;not null"`
	RowVersion       uint64 `gorm:"not null"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (WriteOutbox) TableName() string { return "community_write_outbox" }

type WriteOutboxDelivery struct {
	Event      WriteOutbox
	LeaseToken string
}

// WriteContractRepository is deliberately separate from the legacy Repository
// interface so existing in-memory repositories and no-key create callers keep
// their exact contract.
type WriteContractRepository interface {
	FindPostByRequest(context.Context, uint, string) (Post, bool, error)
	FindCommentByRequest(context.Context, uint, string) (Comment, bool, error)
	CreatePostOrReplay(context.Context, *Post, *WriteOutbox) (bool, error)
	CreateCommentOrReplay(context.Context, *Comment, *WriteOutbox) (bool, error)
	ClaimNextWriteOutbox(context.Context, time.Time, time.Duration) (WriteOutboxDelivery, bool, error)
	CompleteWriteOutbox(context.Context, WriteOutboxDelivery, time.Time) error
	FailWriteOutbox(context.Context, WriteOutboxDelivery, string, time.Time, uint32, time.Time) (WriteOutboxStatus, error)
	DeadletterWriteOutbox(context.Context, WriteOutboxDelivery, string, time.Time) error
}

var _ WriteContractRepository = (*GormRepository)(nil)

func (r *GormRepository) FindPostByRequest(ctx context.Context, ownerID uint, clientRequestID string) (Post, bool, error) {
	if ownerID == 0 || !validWriteContractULID(clientRequestID) {
		return Post{}, false, ErrWriteContractInvalid
	}
	var value Post
	err := r.db.WithContext(ctx).
		Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort ASC") }).
		Where("user_id = ? AND client_request_id = ?", ownerID, clientRequestID).
		First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Post{}, false, nil
	}
	return value, err == nil, err
}

func (r *GormRepository) FindCommentByRequest(ctx context.Context, ownerID uint, clientRequestID string) (Comment, bool, error) {
	if ownerID == 0 || !validWriteContractULID(clientRequestID) {
		return Comment{}, false, ErrWriteContractInvalid
	}
	var value Comment
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND client_request_id = ?", ownerID, clientRequestID).
		First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Comment{}, false, nil
	}
	return value, err == nil, err
}

func (r *GormRepository) CreatePostOrReplay(ctx context.Context, value *Post, event *WriteOutbox) (bool, error) {
	if value == nil || event == nil || !validCreateCoordinates(value.ClientRequestID, value.CreateRequestHash) {
		return false, ErrWriteContractInvalid
	}
	replayed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		images := append([]PostImage(nil), value.Images...)
		value.Images = nil
		create := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(value)
		if create.Error != nil {
			value.Images = images
			return create.Error
		}
		if create.RowsAffected == 0 {
			value.Images = images
			var existing Post
			if err := tx.Preload("Images", func(db *gorm.DB) *gorm.DB { return db.Order("sort ASC") }).
				Where("user_id = ? AND client_request_id = ?", value.UserID, *value.ClientRequestID).
				First(&existing).Error; err != nil {
				return err
			}
			if !bytes.Equal(existing.CreateRequestHash, value.CreateRequestHash) {
				return ErrIdempotencyConflict
			}
			*value = existing
			if err := loadCommunityWriteOutbox(tx, "post", uint64(existing.ID), event.EventType, event); err != nil {
				return err
			}
			replayed = true
			return nil
		}
		for index := range images {
			images[index].PostID = value.ID
			images[index].Sort = index
		}
		if len(images) > 0 {
			if err := tx.Create(&images).Error; err != nil {
				return err
			}
		}
		value.Images = images
		return insertCommunityWriteOutbox(tx, event, "post", uint64(value.ID), uint64(value.UserID))
	})
	return replayed, err
}

func (r *GormRepository) CreateCommentOrReplay(ctx context.Context, value *Comment, event *WriteOutbox) (bool, error) {
	if value == nil || event == nil || !validCreateCoordinates(value.ClientRequestID, value.CreateRequestHash) {
		return false, ErrWriteContractInvalid
	}
	replayed := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		create := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(value)
		if create.Error != nil {
			return create.Error
		}
		if create.RowsAffected == 0 {
			var existing Comment
			if err := tx.Where("user_id = ? AND client_request_id = ?", value.UserID, *value.ClientRequestID).
				First(&existing).Error; err != nil {
				return err
			}
			if !bytes.Equal(existing.CreateRequestHash, value.CreateRequestHash) {
				return ErrIdempotencyConflict
			}
			*value = existing
			if err := loadCommunityWriteOutbox(tx, "comment", uint64(existing.ID), event.EventType, event); err != nil {
				return err
			}
			replayed = true
			return nil
		}
		return insertCommunityWriteOutbox(tx, event, "comment", uint64(value.ID), uint64(value.UserID))
	})
	return replayed, err
}

func insertCommunityWriteOutbox(tx *gorm.DB, event *WriteOutbox, aggregateType string, aggregateID, ownerID uint64) error {
	now := time.Now().UTC()
	if event.EventPublicID == "" {
		publicID, err := newWriteContractULID(now, rand.Reader)
		if err != nil {
			return err
		}
		event.EventPublicID = publicID
	}
	if event.EventType == "" || !json.Valid([]byte(event.Payload)) {
		return ErrWriteContractInvalid
	}
	event.AggregateType = aggregateType
	event.AggregateID = aggregateID
	event.OwnerUserID = ownerID
	event.Status = WriteOutboxPending
	event.AttemptCount = 0
	event.AvailableAt = now
	event.LeaseToken = nil
	event.LeaseExpiresAt = nil
	event.ProcessedAt = nil
	event.FailedAt = nil
	event.LastError = ""
	event.RowVersion = 1
	event.CreatedAt = now
	event.UpdatedAt = now
	return tx.Create(event).Error
}

func loadCommunityWriteOutbox(tx *gorm.DB, aggregateType string, aggregateID uint64, eventType string, result *WriteOutbox) error {
	if eventType == "" {
		return ErrWriteContractInvalid
	}
	err := tx.Where("aggregate_type = ? AND aggregate_id = ? AND event_type = ?", aggregateType, aggregateID, eventType).
		First(result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrWriteOutboxInvariant
	}
	return err
}

func (r *GormRepository) ClaimNextWriteOutbox(ctx context.Context, now time.Time, leaseTTL time.Duration) (WriteOutboxDelivery, bool, error) {
	if leaseTTL <= 0 {
		return WriteOutboxDelivery{}, false, ErrWriteContractInvalid
	}
	var delivery WriteOutboxDelivery
	found := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var event WriteOutbox
		err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(`((status IN ? AND available_at <= ?) OR (status = ? AND lease_expires_at <= ?))`,
				[]WriteOutboxStatus{WriteOutboxPending, WriteOutboxRetryWait}, now, WriteOutboxProcessing, now).
			Order("id ASC").First(&event).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		leaseToken, err := newWriteContractULID(now, rand.Reader)
		if err != nil {
			return err
		}
		leaseExpiresAt := now.Add(leaseTTL)
		update := tx.Model(&WriteOutbox{}).
			Where("id = ? AND row_version = ? AND status = ?", event.ID, event.RowVersion, event.Status).
			Updates(map[string]any{
				"status": WriteOutboxProcessing, "attempt_count": gorm.Expr("attempt_count + 1"),
				"lease_token": leaseToken, "lease_expires_at": leaseExpiresAt,
				"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrWriteOutboxLease
		}
		event.Status = WriteOutboxProcessing
		event.AttemptCount++
		event.LeaseToken = &leaseToken
		event.LeaseExpiresAt = &leaseExpiresAt
		event.RowVersion++
		event.UpdatedAt = now
		delivery = WriteOutboxDelivery{Event: event, LeaseToken: leaseToken}
		found = true
		return nil
	})
	return delivery, found, err
}

func (r *GormRepository) CompleteWriteOutbox(ctx context.Context, delivery WriteOutboxDelivery, now time.Time) error {
	update := r.db.WithContext(ctx).Model(&WriteOutbox{}).
		Where("id = ? AND row_version = ? AND status = ? AND lease_token = ?",
			delivery.Event.ID, delivery.Event.RowVersion, WriteOutboxProcessing, delivery.LeaseToken).
		Updates(map[string]any{
			"status": WriteOutboxSucceeded, "processed_at": now, "lease_token": nil,
			"lease_expires_at": nil, "last_error": "", "row_version": gorm.Expr("row_version + 1"),
			"updated_at": now,
		})
	return normalizeWriteOutboxCAS(update)
}

func (r *GormRepository) FailWriteOutbox(ctx context.Context, delivery WriteOutboxDelivery, message string, retryAt time.Time, maxAttempts uint32, now time.Time) (WriteOutboxStatus, error) {
	if maxAttempts == 0 {
		return "", ErrWriteContractInvalid
	}
	message = truncateWriteOutboxError(message)
	status := WriteOutboxRetryWait
	updates := map[string]any{
		"status": status, "available_at": retryAt, "lease_token": nil, "lease_expires_at": nil,
		"last_error": message, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
	}
	if delivery.Event.AttemptCount >= maxAttempts {
		status = WriteOutboxDeadletter
		updates["status"] = status
		updates["failed_at"] = now
	}
	update := r.db.WithContext(ctx).Model(&WriteOutbox{}).
		Where("id = ? AND row_version = ? AND status = ? AND lease_token = ?",
			delivery.Event.ID, delivery.Event.RowVersion, WriteOutboxProcessing, delivery.LeaseToken).
		Updates(updates)
	if err := normalizeWriteOutboxCAS(update); err != nil {
		return "", err
	}
	return status, nil
}

func (r *GormRepository) DeadletterWriteOutbox(ctx context.Context, delivery WriteOutboxDelivery, message string, now time.Time) error {
	update := r.db.WithContext(ctx).Model(&WriteOutbox{}).
		Where("id = ? AND row_version = ? AND status = ? AND lease_token = ?",
			delivery.Event.ID, delivery.Event.RowVersion, WriteOutboxProcessing, delivery.LeaseToken).
		Updates(map[string]any{
			"status": WriteOutboxDeadletter, "failed_at": now, "lease_token": nil, "lease_expires_at": nil,
			"last_error": truncateWriteOutboxError(message), "row_version": gorm.Expr("row_version + 1"),
			"updated_at": now,
		})
	return normalizeWriteOutboxCAS(update)
}

func normalizeWriteOutboxCAS(result *gorm.DB) error {
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrWriteOutboxLease
	}
	return nil
}

func validCreateCoordinates(clientRequestID *string, requestHash []byte) bool {
	return clientRequestID != nil && validWriteContractULID(*clientRequestID) && len(requestHash) == 32
}

func isWriteContractDuplicate(err error) bool {
	var mysqlError *mysql.MySQLError
	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

func truncateWriteOutboxError(value string) string {
	if len(value) > 512 {
		return value[:512]
	}
	return value
}

func validWriteContractULID(value string) bool {
	if len(value) != 26 || value[0] > '7' {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune(writeContractULIDAlphabet, character) {
			return false
		}
	}
	return true
}

func newWriteContractULID(at time.Time, randomness io.Reader) (string, error) {
	if randomness == nil {
		return "", ErrWriteContractInvalid
	}
	milliseconds := at.UnixMilli()
	if milliseconds < 0 || milliseconds > (1<<48)-1 {
		return "", ErrWriteContractInvalid
	}
	raw := make([]byte, 16)
	for index := 5; index >= 0; index-- {
		raw[index] = byte(milliseconds)
		milliseconds >>= 8
	}
	if _, err := io.ReadFull(randomness, raw[6:]); err != nil {
		return "", fmt.Errorf("generate write outbox ULID: %w", err)
	}
	value := new(big.Int).SetBytes(raw)
	base := big.NewInt(32)
	remainder := new(big.Int)
	encoded := make([]byte, 26)
	for index := len(encoded) - 1; index >= 0; index-- {
		value.QuoRem(value, base, remainder)
		encoded[index] = writeContractULIDAlphabet[remainder.Int64()]
	}
	return string(encoded), nil
}
