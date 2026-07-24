package adminauth

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrCredentialNotFound = errors.New("admin credential not found")

type Repository interface {
	FindCredential(context.Context, string) (Credential, error)
	UpsertCredential(context.Context, uint, string, string, time.Time) (Credential, error)
	RecordFailure(context.Context, uint64, uint16, *time.Time) error
	ResetFailures(context.Context, uint64) error
	CreateSession(context.Context, *WebSession) error
	FindSession(context.Context, []byte) (WebSession, error)
	TouchSession(context.Context, uint64, time.Time, time.Time) error
	RevokeSession(context.Context, uint64, time.Time) error
	RevokeCredentialSessions(context.Context, uint64, time.Time) error
	UpdatePasswordAndRevoke(context.Context, uint64, string, time.Time) error
	CountRecentFailures(context.Context, string, []byte, time.Time) (int64, error)
	CreateAudit(context.Context, AuditLog) error
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

func (r *GormRepository) FindCredential(ctx context.Context, username string) (Credential, error) {
	var value Credential
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Credential{}, ErrCredentialNotFound
	}
	return value, err
}

func (r *GormRepository) UpsertCredential(ctx context.Context, userID uint, username, passwordHash string, now time.Time) (Credential, error) {
	value := Credential{
		Username: username, UserID: userID, PasswordHash: passwordHash,
		PasswordChangedAt: now, RowVersion: 1, CreatedAt: now, UpdatedAt: now,
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing Credential
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("username = ?", username).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&value).Error
		}
		if err != nil {
			return err
		}
		value.ID = existing.ID
		value.RowVersion = existing.RowVersion + 1
		return tx.Model(&Credential{}).Where("id = ?", existing.ID).Updates(map[string]any{
			"user_id": userID, "password_hash": passwordHash, "failed_attempts": 0,
			"locked_until": nil, "password_changed_at": now,
			"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		}).Error
	})
	return value, err
}

func (r *GormRepository) RecordFailure(ctx context.Context, id uint64, attempts uint16, lockedUntil *time.Time) error {
	return r.db.WithContext(ctx).Model(&Credential{}).Where("id = ?", id).Updates(map[string]any{
		"failed_attempts": attempts, "locked_until": lockedUntil, "updated_at": time.Now().UTC(),
	}).Error
}

func (r *GormRepository) ResetFailures(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&Credential{}).Where("id = ?", id).
		Updates(map[string]any{"failed_attempts": 0, "locked_until": nil, "updated_at": time.Now().UTC()}).Error
}

func (r *GormRepository) CreateSession(ctx context.Context, value *WebSession) error {
	return r.db.WithContext(ctx).Create(value).Error
}

func (r *GormRepository) FindSession(ctx context.Context, tokenHash []byte) (WebSession, error) {
	var value WebSession
	err := r.db.WithContext(ctx).Where("token_hash = ?", tokenHash).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return WebSession{}, ErrInvalidSession
	}
	return value, err
}

func (r *GormRepository) TouchSession(ctx context.Context, id uint64, lastSeen, idleExpires time.Time) error {
	return r.db.WithContext(ctx).Model(&WebSession{}).Where("id = ? AND revoked_at IS NULL", id).
		Updates(map[string]any{"last_seen_at": lastSeen, "idle_expires_at": idleExpires, "updated_at": lastSeen}).Error
}

func (r *GormRepository) RevokeSession(ctx context.Context, id uint64, now time.Time) error {
	return r.db.WithContext(ctx).Model(&WebSession{}).Where("id = ? AND revoked_at IS NULL", id).
		Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error
}

func (r *GormRepository) RevokeCredentialSessions(ctx context.Context, credentialID uint64, now time.Time) error {
	return r.db.WithContext(ctx).Model(&WebSession{}).Where("credential_id = ? AND revoked_at IS NULL", credentialID).
		Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error
}

func (r *GormRepository) UpdatePasswordAndRevoke(ctx context.Context, credentialID uint64, passwordHash string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Credential{}).Where("id = ?", credentialID).Updates(map[string]any{
			"password_hash": passwordHash, "password_changed_at": now,
			"failed_attempts": 0, "locked_until": nil,
			"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&WebSession{}).Where("credential_id = ? AND revoked_at IS NULL", credentialID).
			Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error
	})
}

func (r *GormRepository) CountRecentFailures(ctx context.Context, username string, ipHash []byte, since time.Time) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&AuditLog{}).
		Where("username = ? AND event_type = 'login' AND outcome = 'failure' AND ip_hash = ? AND created_at >= ?", username, ipHash, since).
		Count(&count).Error
	return count, err
}

func (r *GormRepository) CreateAudit(ctx context.Context, value AuditLog) error {
	return r.db.WithContext(ctx).Create(&value).Error
}
