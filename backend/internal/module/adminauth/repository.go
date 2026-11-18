package adminauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrCredentialNotFound = errors.New("admin account not found")
var ErrUsernameTaken = errors.New("admin username already taken")
var ErrLastActiveSuperAdmin = errors.New("cannot disable or demote the last active superadmin")

type Repository interface {
	FindAccountByUsername(context.Context, string) (Account, error)
	FindAccountByID(context.Context, uint64) (Account, error)
	ListAccounts(context.Context) ([]Account, error)
	CreateAccount(context.Context, Account) (Account, error)
	UpdateAccount(context.Context, uint64, string, Role, time.Time) (Account, error)
	SetAccountStatus(context.Context, uint64, Status, time.Time) (Account, error)
	ResetAccountPassword(context.Context, uint64, string, time.Time) error
	RecordFailure(context.Context, uint64, uint16, *time.Time) error
	ResetFailures(context.Context, uint64) error
	CreateSession(context.Context, *WebSession) error
	FindSession(context.Context, []byte) (WebSession, error)
	TouchSession(context.Context, uint64, time.Time, time.Time) error
	RevokeSession(context.Context, uint64, time.Time) error
	RevokeCredentialSessions(context.Context, uint64, time.Time) error
	CountRecentFailures(context.Context, string, []byte, time.Time) (int64, error)
	CreateAudit(context.Context, AuditLog) error
}

type GormRepository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

// EnsureSchema 校验独立管理员账号所需列已由迁移 069 建立；缺失时拒绝启动，
// 避免线上以旧结构运行后出现难以定位的登录/审计失败。
func EnsureSchema(db *gorm.DB) error {
	if db == nil {
		return errors.New("administrator account schema requires a database")
	}
	migrator := db.Migrator()
	if !migrator.HasTable(&Account{}) || !migrator.HasTable(&WebSession{}) || !migrator.HasTable(&AuditLog{}) {
		return errors.New("administrator authentication tables missing; apply migrations 035 and 069")
	}
	for _, column := range []string{"role", "status", "display_name", "legacy_user_id"} {
		if !migrator.HasColumn(&Account{}, column) {
			return fmt.Errorf("administrator account column %s missing; apply migration 069", column)
		}
	}
	if !migrator.HasColumn(&AuditLog{}, "actor_credential_id") {
		return errors.New("administrator audit actor column missing; apply migration 069")
	}
	return nil
}

func (r *GormRepository) FindAccountByUsername(ctx context.Context, username string) (Account, error) {
	var value Account
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Account{}, ErrCredentialNotFound
	}
	return value, err
}

func (r *GormRepository) FindAccountByID(ctx context.Context, id uint64) (Account, error) {
	var value Account
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&value).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Account{}, ErrCredentialNotFound
	}
	return value, err
}

func (r *GormRepository) ListAccounts(ctx context.Context) ([]Account, error) {
	values := make([]Account, 0)
	err := r.db.WithContext(ctx).Order("id").Find(&values).Error
	return values, err
}

func (r *GormRepository) CreateAccount(ctx context.Context, value Account) (Account, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing Account
		err := tx.Where("username = ?", value.Username).First(&existing).Error
		if err == nil {
			return ErrUsernameTaken
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(&value).Error
	})
	return value, err
}

// lockActiveSuperAdmins 在事务内锁住所有启用中的超级管理员行，用于最后一名超管保护。
func lockActiveSuperAdmins(tx *gorm.DB) (int, error) {
	ids := make([]uint64, 0, 4)
	err := tx.Model(&Account{}).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("role = ? AND status = ?", RoleSuperAdmin, StatusActive).Pluck("id", &ids).Error
	return len(ids), err
}

func (r *GormRepository) UpdateAccount(ctx context.Context, id uint64, displayName string, role Role, now time.Time) (Account, error) {
	var result Account
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current Account
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCredentialNotFound
			}
			return err
		}
		if current.Role == RoleSuperAdmin && role != RoleSuperAdmin && current.Status == StatusActive {
			count, err := lockActiveSuperAdmins(tx)
			if err != nil {
				return err
			}
			if count <= 1 {
				return ErrLastActiveSuperAdmin
			}
		}
		if err := tx.Model(&Account{}).Where("id = ?", id).Updates(map[string]any{
			"display_name": displayName, "role": role,
			"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		}).Error; err != nil {
			return err
		}
		result = current
		result.DisplayName, result.Role = displayName, role
		result.RowVersion++
		result.UpdatedAt = now
		return nil
	})
	return result, err
}

func (r *GormRepository) SetAccountStatus(ctx context.Context, id uint64, status Status, now time.Time) (Account, error) {
	var result Account
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current Account
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCredentialNotFound
			}
			return err
		}
		if current.Status == status {
			result = current
			return nil
		}
		if current.Role == RoleSuperAdmin && current.Status == StatusActive && status == StatusDisabled {
			count, err := lockActiveSuperAdmins(tx)
			if err != nil {
				return err
			}
			if count <= 1 {
				return ErrLastActiveSuperAdmin
			}
		}
		if err := tx.Model(&Account{}).Where("id = ?", id).Updates(map[string]any{
			"status": status, "row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		}).Error; err != nil {
			return err
		}
		if status == StatusDisabled {
			if err := tx.Model(&WebSession{}).Where("credential_id = ? AND revoked_at IS NULL", id).
				Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error; err != nil {
				return err
			}
		}
		result = current
		result.Status = status
		result.RowVersion++
		result.UpdatedAt = now
		return nil
	})
	return result, err
}

func (r *GormRepository) ResetAccountPassword(ctx context.Context, id uint64, passwordHash string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Account{}).Where("id = ?", id).Updates(map[string]any{
			"password_hash": passwordHash, "password_changed_at": now,
			"failed_attempts": 0, "locked_until": nil,
			"row_version": gorm.Expr("row_version + 1"), "updated_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Model(&WebSession{}).Where("credential_id = ? AND revoked_at IS NULL", id).
			Updates(map[string]any{"revoked_at": now, "updated_at": now}).Error
	})
}

func (r *GormRepository) RecordFailure(ctx context.Context, id uint64, attempts uint16, lockedUntil *time.Time) error {
	return r.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).Updates(map[string]any{
		"failed_attempts": attempts, "locked_until": lockedUntil, "updated_at": time.Now().UTC(),
	}).Error
}

func (r *GormRepository) ResetFailures(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Model(&Account{}).Where("id = ?", id).
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
